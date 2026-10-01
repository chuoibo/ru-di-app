package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"

	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/nhung"
	"mobile/services/core/internal/db"
	"mobile/services/core/internal/jobs"
	"mobile/services/core/internal/rag"
	"mobile/services/core/internal/rag/nap"
	"mobile/services/core/internal/vectordb"
	"mobile/services/core/internal/vectordb/napkho"
)

// ragVectorUsage is the vector index's half of `core rag` (package rag/nap):
// every command prints one JSON object of ids, states and counts, never a
// word of the catalogue.
const ragVectorUsage = "usage: core rag v-build <place|manual> [--auto] | v-embed-batch place | v-eval <id> | v-promote <id> | v-rollback <place|manual> | " +
	"v-status | v-enrich --tran-goi N | v-review list [--all] | v-review approve|reject <place_id> <ban> | v-dlq ls|retry | v-index | v-reconcile"

// EnvRagDense chooses the dense encoder of the vector pipeline: unset or
// "gemini" is the engine's embedding door (aiharness/nhung, which refuses a
// model or dimensionality the committed configuration does not name);
// "stub" is the deterministic encoder, for a local stack with no key.
const EnvRagDense = "MOBILE_RAG_DENSE"

type ragVectorCommand struct {
	name    string
	corpus  nap.Corpus
	id      int64
	auto    bool
	all     bool
	verdict string
	placeID string
	ban     string
	tranGoi int
}

func parseRagVector(args []string) (ragVectorCommand, error) {
	bad := errors.New(ragVectorUsage)
	if len(args) == 0 {
		return ragVectorCommand{}, bad
	}
	c := ragVectorCommand{name: args[0]}
	corpus := func(s string) error {
		k, err := nap.KiemCorpus(s)
		c.corpus = k
		return err
	}
	switch c.name {
	case "v-build":
		if len(args) < 2 || len(args) > 3 || corpus(args[1]) != nil || (len(args) == 3 && args[2] != "--auto") {
			return c, bad
		}
		c.auto = len(args) == 3
	case "v-eval", "v-promote":
		if len(args) != 2 {
			return c, bad
		}
		id, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil || id <= 0 {
			return c, bad
		}
		c.id = id
	case "v-rollback":
		if len(args) != 2 || corpus(args[1]) != nil {
			return c, bad
		}
	case "v-embed-batch":
		// Places only: the manual is a few dozen chunks, online is fine.
		if len(args) != 2 || args[1] != string(nap.CorpusQuan) {
			return c, bad
		}
		c.corpus = nap.CorpusQuan
	case "v-status", "v-index", "v-reconcile":
		if len(args) != 1 {
			return c, bad
		}
	case "v-enrich":
		// The ceiling is mandatory and has no default: every real model call
		// is a number a person chose (design 06 §7).
		if len(args) != 3 || args[1] != "--tran-goi" {
			return c, bad
		}
		n, err := strconv.Atoi(args[2])
		if err != nil || n <= 0 || n > 2000 {
			return c, bad
		}
		c.tranGoi = n
	case "v-review":
		switch {
		case len(args) == 2 && args[1] == "list":
			c.verdict = "list"
		case len(args) == 3 && args[1] == "list" && args[2] == "--all":
			c.verdict, c.all = "list", true
		case len(args) == 4 && (args[1] == "approve" || args[1] == "reject") && args[2] != "" && args[3] != "":
			// The verdict names the version the reviewer read (MucDuyet.Ban):
			// an enrichment replaced since is refused, never approved unseen.
			c.verdict, c.placeID, c.ban = args[1], args[2], args[3]
		default:
			return c, bad
		}
	case "v-dlq":
		if len(args) != 2 || (args[1] != "ls" && args[1] != "retry") {
			return c, bad
		}
		c.verdict = args[1]
	default:
		return c, bad
	}
	return c, nil
}

// napDense adapts the engine's embedding door to the pipeline. It passes the
// title and the text as they are: gemini-embedding-2's task prefix is
// written once, inside aiharness/nhung (DinhDang), for documents and
// queries alike, and never here (a second prefix would embed
// «title: none | text: title: … | text: …»; TestNapDenseMotTienTo pins the
// wire body).
type napDense struct{ e nhung.Nhung }

func (d napDense) Model() string { return d.e.Model() }
func (d napDense) Dims() int     { return d.e.Dims() }
func (d napDense) SoGoi() int64  { return d.e.SoGoi() }

func (d napDense) NhungTaiLieu(ctx context.Context, docs []nap.TaiLieu) ([][]float32, error) {
	in := make([]nhung.TaiLieuVao, len(docs))
	for i, t := range docs {
		in[i] = nhung.TaiLieuVao{TieuDe: t.TieuDe, NoiDung: t.Chu}
	}
	// The ingest (not a turn: a turn has its own counted retries) waits
	// out the provider's rate limit: a 429 is retried with a growing pause,
	// so one burst of sentence embeddings does not fail a whole build.
	for lan := 0; ; lan++ {
		vecs, err := d.e.NhungTaiLieu(ctx, in)
		var api genai.APIError
		if err == nil || !errors.As(err, &api) || api.Code != 429 || lan >= len(napChoLai) {
			return vecs, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(napChoLai[lan]):
		}
	}
}

// napChoLai are the pauses before the ingest retries a rate-limited
// embedding call (then it gives up and the error stands).
var napChoLai = []time.Duration{5 * time.Second, 15 * time.Second, 45 * time.Second, 90 * time.Second}

func (d napDense) NhungCauHoi(ctx context.Context, qs []string) ([][]float32, error) {
	return d.e.Nhung(ctx, qs, nhung.CauHoi)
}

type pgxTx = pgx.Tx

type ragEncoder interface {
	nap.NhungTaiLieu
	nap.NhungCauHoi
}

func ragDense(ctx context.Context, getenv func(string) string, cfg nap.CauHinh) (ragEncoder, error) {
	switch getenv(EnvRagDense) {
	case "stub":
		return nap.StubDense{N: cfg.Dense.Dims}, nil
	case "", "gemini":
		g, err := nhung.FromEnv(ctx, getenv)
		if err != nil {
			return nil, err
		}
		d := napDense{e: g}
		if d.Model() != cfg.Dense.Model || d.Dims() != cfg.Dense.Dims {
			return nil, fmt.Errorf("the embedding door serves %s/%d, the committed configuration asks %s/%d",
				d.Model(), d.Dims(), cfg.Dense.Model, cfg.Dense.Dims)
		}
		return d, nil
	}
	return nil, fmt.Errorf("%s must be gemini or stub", EnvRagDense)
}

// ragVectorDeps opens what a command needs: the configuration, Milvus, the
// encoder (only for the commands that embed).
func ragVectorDeps(ctx context.Context, getenv func(string) string, needEncoder bool) (nap.Nap, ragEncoder, func(), error) {
	cfg, err := nap.MacDinh()
	if err != nil {
		return nap.Nap{}, nil, nil, err
	}
	if err := napkho.KiemKhop(cfg); err != nil {
		return nap.Nap{}, nil, nil, err
	}
	k, err := vectordb.FromEnv(getenv)
	if err != nil {
		return nap.Nap{}, nil, nil, err
	}
	kho, err := napkho.Mo(ctx, k)
	if err != nil {
		return nap.Nap{}, nil, nil, errors.New("rag: Milvus is unreachable")
	}
	closeFn := func() { _ = kho.Dong(context.Background()) }
	n := nap.Nap{Kho: kho, Cfg: cfg}
	var enc ragEncoder
	if needEncoder {
		if enc, err = ragDense(ctx, getenv, cfg); err != nil {
			closeFn()
			return nap.Nap{}, nil, nil, err
		}
		n.Dense = enc
	} else {
		n.Dense = nap.StubDense{N: cfg.Dense.Dims}
	}
	return n, enc, closeFn, nil
}

func runRagVector(args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	c, err := parseRagVector(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer cancel()
	pool, err := db.Open(ctx, getenv(db.EnvDatabaseURL))
	if err != nil {
		fmt.Fprintln(stderr, "rag: invalid database configuration")
		return 1
	}
	defer pool.Close()
	if ok, err := nap.Installed(ctx, pool); err != nil || !ok {
		fmt.Fprintln(stderr, "rag: the vector ingestion schema is missing; run `core migrate-rag-vector` first")
		return 1
	}
	// Commands that touch only Postgres open no Milvus connection.
	switch c.name {
	case "v-review":
		return ragReview(ctx, pool, c, stdout, stderr)
	case "v-dlq":
		if c.verdict == "retry" {
			n, err := nap.ThuLaiDLQ(ctx, pool)
			return ragOut(stdout, stderr, map[string]int{"tro_lai": n}, err)
		}
		var rows []map[string]any
		r, err := pool.Query(ctx, `SELECT corpus, chang, ma_loi, count(*), max(so_lan) FROM rag_ingest_dlq GROUP BY 1,2,3 ORDER BY 1,2,3`)
		if err == nil {
			for r.Next() {
				var corpus, chang, ma string
				var n, max int
				if err = r.Scan(&corpus, &chang, &ma, &n, &max); err != nil {
					break
				}
				rows = append(rows, map[string]any{"corpus": corpus, "chang": chang, "ma_loi": ma, "so_doc": n, "so_lan_toi_da": max})
			}
			r.Close()
		}
		return ragOut(stdout, stderr, rows, err)
	case "v-enrich":
		return ragEnrich(ctx, getenv, pool, c.tranGoi, stdout, stderr)
	case "v-embed-batch":
		return ragEmbedBatch(ctx, getenv, pool, stdout, stderr)
	}
	needEnc := c.name == "v-build" || c.name == "v-eval" || c.name == "v-index"
	n, enc, closeFn, err := ragVectorDeps(ctx, getenv, needEnc)
	if err != nil {
		fmt.Fprintln(stderr, "rag:", err)
		return 1
	}
	defer closeFn()
	golden := func() (nap.TapVang, error) { return nap.DocVang(rag.VangDiaDiem()) }
	switch c.name {
	case "v-build":
		if c.auto {
			v, err := golden()
			if err != nil {
				return ragOut(stdout, stderr, nil, err)
			}
			kq, err := n.DungTuDong(ctx, pool, c.corpus, enc, v)
			return ragOut(stdout, stderr, kq, err)
		}
		rep, err := n.Dung(ctx, pool, c.corpus)
		return ragOut(stdout, stderr, rep, err)
	case "v-eval":
		v, err := golden()
		if err != nil {
			return ragOut(stdout, stderr, nil, err)
		}
		k, err := n.DanhGia(ctx, pool, c.id, enc, v)
		if err == nil && !k.Dat {
			printJSON(stdout, k)
			fmt.Fprintln(stderr, "rag: the gate refused the version; it is marked failed")
			return 1
		}
		return ragOut(stdout, stderr, k, err)
	case "v-promote":
		// Promote re-checks the verdict against the golden file in force.
		v, err := golden()
		if err != nil {
			return ragOut(stdout, stderr, nil, err)
		}
		p, err := n.Promote(ctx, pool, c.id, v.Sha)
		return ragOut(stdout, stderr, map[string]any{"active": p.ID, "collection": p.Collection, "parent": p.ParentID}, err)
	case "v-rollback":
		from, to, err := n.Rollback(ctx, pool, c.corpus)
		return ragOut(stdout, stderr, map[string]int64{"retired": from, "active": to}, err)
	case "v-status":
		st, err := n.DocTrangThai(ctx, pool)
		return ragOut(stdout, stderr, st, err)
	case "v-index":
		// One pass by hand: the same budget as the rag-indexer's (at most
		// NguongOnline vectors paid online), no batch door.
		cm := nap.ChiMuc{Nap: n, Pool: pool, HanMuc: nap.NewHanMuc(nap.TranOnlineGio)}
		_, b, err := cm.Luot(ctx, pool)
		return ragOut(stdout, stderr, b, err)
	case "v-reconcile":
		var b nap.BaoCaoDoiChieu
		_, err := jobs.MotLuot(ctx, pool, jobs.DinhKy{Ten: "rag.nap.doi_chieu", Nhip: time.Minute, Chay: func(ctx context.Context, tx pgxTx) error {
			var err error
			b, err = n.DoiChieu(ctx, tx, nil)
			return err
		}})
		return ragOut(stdout, stderr, b, err)
	}
	fmt.Fprintln(stderr, ragVectorUsage)
	return 2
}

func ragOut(stdout, stderr io.Writer, v any, err error) int {
	if err != nil {
		fmt.Fprintln(stderr, "rag:", err)
		return 1
	}
	printJSON(stdout, v)
	return 0
}

func ragReview(ctx context.Context, pool nap.CSDL, c ragVectorCommand, stdout, stderr io.Writer) int {
	switch c.verdict {
	case "list":
		q, err := nap.HangDuyet(ctx, pool, c.all, 500)
		return ragOut(stdout, stderr, q, err)
	case "approve":
		return ragOut(stdout, stderr, map[string]string{"place_id": c.placeID, "ban": c.ban, "review": nap.ReviewReviewed},
			nap.Duyet(ctx, pool, c.placeID, c.ban, nap.ReviewReviewed))
	case "reject":
		return ragOut(stdout, stderr, map[string]string{"place_id": c.placeID, "ban": c.ban, "review": nap.ReviewRejected},
			nap.Duyet(ctx, pool, c.placeID, c.ban, nap.ReviewRejected))
	}
	return 2
}

// ragEnrich runs the enrichment over every place whose stored enrichment is
// missing or stale, through the real model (GEMINI_API_KEY), at most
// tranGoi calls, counted by llm.Dem. No key: it refuses; there is no stub
// for a command that writes production rows.
// EnvRagEnrichSongSong overrides, for one enrichment run, how many batches
// go to the model at once (the committed lam_giau.song_song otherwise).
// agy-proxy runs at most 8 per client (vnlocal HANDOFF-KET-NOI.md §5.6); a
// 9th would only queue there.
const EnvRagEnrichSongSong = "MOBILE_RAG_ENRICH_SONG_SONG"

func ragEnrich(ctx context.Context, getenv func(string) string, pool nap.CSDL, tranGoi int, stdout, stderr io.Writer) int {
	cfg, err := nap.MacDinh()
	if err != nil {
		return ragOut(stdout, stderr, nil, err)
	}
	if raw := strings.TrimSpace(getenv(EnvRagEnrichSongSong)); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 8 {
			return ragOut(stdout, stderr, nil, fmt.Errorf("%s must be 1..8", EnvRagEnrichSongSong))
		}
		cfg.LamGiau.SongSong = n
	}
	m, err := llm.GeminiFromEnv(ctx, getenv)
	if err != nil {
		return ragOut(stdout, stderr, nil, err)
	}
	return ragOut(stdout, stderr, nil, enrichWith(ctx, pool, m, cfg, tranGoi, stdout))
}

func enrichWith(ctx context.Context, pool nap.CSDL, m model.LLM, cfg nap.CauHinh, tranGoi int, stdout io.Writer) error {
	// The enrichment reads each place's profile, never its chunks: the
	// places are exactly those the build gate counts as lacking one.
	need, err := nap.CanLamGiau(ctx, pool)
	if err != nil {
		return err
	}
	// Checkpointed: the places go in rounds (every worker a few batches),
	// and each round's results and failures are written before the next
	// starts. A run cut short keeps what it finished; the next run selects
	// only places still lacking a current enrichment, so it resumes. The
	// ceiling counts across rounds.
	round := cfg.LamGiau.Lo * cfg.LamGiau.SongSong * 4
	var tong nap.BaoCaoLamGiau
	tong.Quan = len(need)
	con := tranGoi
	for start := 0; start < len(need) && con > 0; start += round {
		if err := ctx.Err(); err != nil {
			break
		}
		part := need[start:min(start+round, len(need))]
		done, hong, b := nap.ChayLamGiau(ctx, m, cfg, con, part)
		if err := nap.GhiLamGiau(ctx, pool, done); err != nil {
			return err
		}
		for id, e := range hong {
			if err := nap.GhiDLQ(ctx, pool, nap.CorpusQuan, id, nap.ChangLamGiau, e); err != nil {
				return err
			}
		}
		con -= b.SoGoi
		tong.SoGoi += b.SoGoi
		tong.Xong += b.Xong
		tong.Hong += b.Hong
		tong.HetTran += b.HetTran
		tong.CanDuyet += b.CanDuyet
		tong.ChenLenh += b.ChenLenh
		tong.MonBo += b.MonBo
		fmt.Fprintf(stdout, "{\"dot\":%d,\"da_xu_ly\":%d,\"tong\":%d,\"xong\":%d,\"hong\":%d,\"so_goi\":%d}\n",
			start/round+1, min(start+round, len(need)), len(need), tong.Xong, tong.Hong, tong.SoGoi)
	}
	printJSON(stdout, tong)
	return nil
}

// migrateRagVector is `core migrate-rag-vector`: the ingestion schema, after
// `core migrate-chat` (the outbox's version 2) and `core migrate-rag`.
func migrateRagVector(getenv func(string) string, stdout, stderr io.Writer) int {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, err := db.Open(ctx, getenv(db.EnvDatabaseURL))
	if err != nil {
		fmt.Fprintln(stderr, "rag vector migration: invalid database configuration")
		return 1
	}
	defer pool.Close()
	if err := nap.Migrate(ctx, pool); err != nil {
		fmt.Fprintln(stderr, "rag vector migration failed:", err)
		return 1
	}
	fmt.Fprintln(stdout, "Đã áp dụng migration nạp chỉ mục vector (rag/nap). Chưa có phiên bản nào: chạy `core rag v-build place`.")
	return 0
}

// ragIndexer is `core rag-indexer`: the vector index's only long-running
// writer. The indexer pass on every notification of rag_dirty (at least
// every 20 s) and on every message of the lane 'rag' (MOBILE_AMQP_URL set),
// the batch embedding door's turn every two minutes, and the alias
// reconciler every minute, each under its own advisory lock.
func ragIndexer(getenv func(string) string, stderr io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	logger := slog.New(slog.NewJSONHandler(stderr, nil))
	refuse := func(err error) int {
		logger.Error("refusing to start", "error", err.Error())
		return 1
	}
	pool, err := openPool(ctx, getenv(db.EnvDatabaseURL), 6)
	if err != nil {
		return refuse(err)
	}
	defer pool.Close()
	if ok, err := nap.Installed(ctx, pool); err != nil || !ok {
		return refuse(errors.New("the vector ingestion schema is missing: run `core migrate-rag-vector`"))
	}
	n, _, closeFn, err := ragVectorDeps(ctx, getenv, true)
	if err != nil {
		return refuse(err)
	}
	defer closeFn()
	gs := &nap.GiamSat{}
	cm := nap.ChiMuc{Nap: n, Logger: logger, Pool: pool, HanMuc: nap.NewHanMuc(nap.TranOnlineGio), GiamSat: gs}
	if lo, err := nhung.LoFromEnv(ctx, getenv); err == nil {
		cm.Lo = napLo{l: lo}
	} else {
		// Without the batch door a bulk change is embedded online, within
		// the hour's budget, NguongOnline at a time.
		logger.Warn("rag indexer: no batch embedding door; bulk changes wait for the online budget", "code", nap.MaLoi(err))
	}
	periodic := []jobs.DinhKy{n.DinhKyDoiChieu(logger)}
	var side sync.WaitGroup
	side.Add(1)
	go func() { defer side.Done(); _ = jobs.ChayDinhKy(ctx, pool, logger, periodic) }()
	side.Add(1)
	go func() { defer side.Done(); cm.Nghe(pool, logger).Run(ctx) }()
	side.Add(1)
	go func() { defer side.Done(); docSLO(ctx, pool, gs, logger) }()
	srv := &http.Server{Addr: ragIndexerListen(getenv), Handler: sloHandler(gs), ReadHeaderTimeout: 3 * time.Second}
	side.Add(1)
	go func() {
		defer side.Done()
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("rag indexer: health port failed", "error", err.Error())
		}
	}()
	go func() {
		<-ctx.Done()
		closing, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(closing)
	}()
	if cm.Lo != nil {
		side.Add(1)
		go func() {
			defer side.Done()
			t := time.NewTicker(nap.ChuKyLo)
			defer t.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-t.C:
				}
				turn, cancel := context.WithTimeout(ctx, 5*time.Minute)
				rep, err := cm.LuotLo(turn, pool)
				cancel()
				switch {
				case err != nil && ctx.Err() == nil:
					logger.Warn("rag indexer: batch embedding turn failed", "code", nap.MaLoi(err))
				case rep.Job != "" || rep.Gui > 0:
					logger.Info("rag indexer: batch embedding turn", "report", rep)
				}
			}
		}()
	}
	if url := getenv(EnvAMQPURL); url != "" {
		if err := jobs.CheckURL(url); err != nil {
			return refuse(err)
		}
		topology, _ := jobs.NewTopology(amqpNamespace)
		ket := &jobs.Ket{URL: url, Topology: topology, Pool: pool, Queues: []string{"rag"}, Concurrency: 1,
			Handler: cm.XuLyTin(pool), Logger: logger}
		side.Add(1)
		go func() { defer side.Done(); ket.Run(ctx) }()
	}
	logger.Info("rag indexer started", "lane", getenv(EnvAMQPURL) != "")
	side.Wait()
	return 0
}

// napLo adapts the engine's batch embedding door to the pipeline.
type napLo struct{ l *nhung.Lo }

func (a napLo) Model() string { return a.l.Model() }
func (a napLo) Dims() int     { return a.l.Dims() }

func (a napLo) GuiLo(ctx context.Context, ten string, docs []nap.LoVao) (string, error) {
	in := make([]nhung.TaiLieuLo, len(docs))
	for i, d := range docs {
		in[i] = nhung.TaiLieuLo{Khoa: d.Khoa, TieuDe: d.TieuDe, NoiDung: d.Chu}
	}
	return a.l.Gui(ctx, ten, in)
}

func (a napLo) XemLo(ctx context.Context, job string) (nap.KetQuaLo, error) {
	kq, err := a.l.Xem(ctx, job)
	if err != nil {
		return nap.KetQuaLo{}, err
	}
	return nap.KetQuaLo{Xong: kq.TrangThai == nhung.LoXong, Hong: kq.TrangThai == nhung.LoHong,
		Vecs: kq.Vecs, LoiDong: kq.LoiDong, Loi: kq.Loi}, nil
}

// ragEmbedBatch is `core rag v-embed-batch place`: every place chunk whose
// vector the cache lacks goes to the Gemini Batch API (half the online
// price, ADR-0049 §2.7), and the vectors land in rag_embedding_cache, where
// the next `v-build` finds them. It refuses while a place lacks a current
// enrichment: the enrichment writes the chunk's context line, so embedding
// before it would pay for vectors of text that is about to change. The run
// polls every 30 s until the job ends or the command's hour does; a job left
// running is polled by the next run, never submitted twice.
func ragEmbedBatch(ctx context.Context, getenv func(string) string, pool nap.CSDL, stdout, stderr io.Writer) int {
	cfg, err := nap.MacDinh()
	if err != nil {
		return ragOut(stdout, stderr, nil, err)
	}
	enc, err := ragDense(ctx, getenv, cfg)
	if err != nil {
		return ragOut(stdout, stderr, nil, err)
	}
	lo, err := nhung.LoFromEnv(ctx, getenv)
	if err != nil {
		return ragOut(stdout, stderr, nil, err)
	}
	n := nap.Nap{Cfg: cfg, Dense: enc}
	var rep nap.BaoCaoDung
	docs, _, err := n.ChuanBiQuan(ctx, pool, &rep)
	if err != nil {
		return ragOut(stdout, stderr, nil, err)
	}
	// Places without a current enrichment are not in docs (ChuanBiQuan
	// leaves them out): their text is not final, embedding it now would be
	// paid twice. They are embedded online by the indexer once it arrives.
	var rows []nap.Hang
	for _, d := range docs {
		rows = append(rows, d.Rows...)
	}
	kq, err := n.NhungQuaLo(ctx, pool, napLo{l: lo}, rows, 30*time.Second, nap.ToiDaLo)
	return ragOut(stdout, stderr, map[string]any{"quan": len(docs), "qua_dai": rep.QuaDai, "cho_lam_giau": rep.ThieuLamGiau, "lo": kq}, err)
}

// EnvRagIndexerListen is the rag-indexer's health port (loopback by
// default; nothing outside the container needs it).
const EnvRagIndexerListen = "MOBILE_RAG_INDEXER_LISTEN"

func ragIndexerListen(getenv func(string) string) string {
	if a := getenv(EnvRagIndexerListen); a != "" {
		return a
	}
	return "127.0.0.1:8091"
}

// docSLO reads the index's freshness every 15 s into gs, and logs a WARN
// slo_vi_pham at most once a minute while the SLO is broken.
func docSLO(ctx context.Context, pool *pgxpool.Pool, gs *nap.GiamSat, logger *slog.Logger) {
	var warned time.Time
	t := time.NewTicker(15 * time.Second)
	defer t.Stop()
	for {
		read, cancel := context.WithTimeout(ctx, 10*time.Second)
		d, err := nap.DocDoTuoi(read, pool)
		cancel()
		if ctx.Err() != nil {
			return
		}
		now := time.Now()
		k := gs.Ghi(now, d, err)
		if !k.Dat && now.Sub(warned) >= time.Minute {
			logger.Warn("slo_vi_pham", "vi_pham", k.ViPham, "tu_giay", k.TuGiay, "do_tuoi", k.DoTuoi, "loi", k.Loi)
			warned = now
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// sloHandler answers /livez (the indexer loop passed within 90 s; it
// passes at least every 20 s) and /slo (the last freshness reading: 503
// when the SLO has been broken for nap.SLOKeoDai, the reading failed, or the
// loop is not alive). Counts and ages only.
func sloHandler(gs *nap.GiamSat) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /livez", func(w http.ResponseWriter, _ *http.Request) {
		if !gs.Song(time.Now(), 90*time.Second) {
			http.Error(w, "loop not alive", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("GET /slo", func(w http.ResponseWriter, _ *http.Request) {
		k := gs.Doc()
		w.Header().Set("Content-Type", "application/json")
		if k == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"dat":false,"loi":"chua_doc"}`))
			return
		}
		out := *k
		if !gs.Song(time.Now(), 90*time.Second) {
			out.Dat, out.Loi = false, "vong_khong_chay"
		}
		if !out.Dat {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		_ = json.NewEncoder(w).Encode(out)
	})
	return mux
}

// ragIndexerHealthcheck is `core rag-indexer-healthcheck`, the container's
// probe: /slo on the indexer's own port. Unhealthy means the index is
// staler than the SLO allows (Docker shows it; it does not restart for it).
func ragIndexerHealthcheck(getenv func(string) string, stderr io.Writer) int {
	host, port, err := net.SplitHostPort(ragIndexerListen(getenv))
	if err != nil {
		fmt.Fprintf(stderr, "rag-indexer-healthcheck: %v\n", err)
		return 1
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	resp, err := (&http.Client{Timeout: 3 * time.Second}).Get("http://" + net.JoinHostPort(host, port) + "/slo")
	if err != nil {
		fmt.Fprintf(stderr, "rag-indexer-healthcheck: %v\n", err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		fmt.Fprintf(stderr, "rag-indexer-healthcheck: status %d %s\n", resp.StatusCode, body)
		return 1
	}
	return 0
}
