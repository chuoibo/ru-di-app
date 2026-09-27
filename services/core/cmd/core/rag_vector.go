package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"google.golang.org/adk/v2/model"

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
const ragVectorUsage = "usage: core rag v-build <place|manual> [--auto] | v-eval <id> | v-promote <id> | v-rollback <place|manual> | " +
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
	return d.e.NhungTaiLieu(ctx, in)
}

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
		var b nap.BaoCaoChiMuc
		cm := nap.ChiMuc{Nap: n}
		_, err := jobs.MotLuot(ctx, pool, jobs.DinhKy{Ten: cm.DinhKy().Ten, Nhip: time.Minute, Chay: func(ctx context.Context, tx pgxTx) error {
			var err error
			b, err = cm.MotLuot(ctx, tx)
			return err
		}})
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
func ragEnrich(ctx context.Context, getenv func(string) string, pool nap.CSDL, tranGoi int, stdout, stderr io.Writer) int {
	cfg, err := nap.MacDinh()
	if err != nil {
		return ragOut(stdout, stderr, nil, err)
	}
	m, err := llm.GeminiFromEnv(ctx, getenv)
	if err != nil {
		return ragOut(stdout, stderr, nil, err)
	}
	return ragOut(stdout, stderr, nil, enrichWith(ctx, pool, m, cfg, tranGoi, stdout))
}

func enrichWith(ctx context.Context, pool nap.CSDL, m model.LLM, cfg nap.CauHinh, tranGoi int, stdout io.Writer) error {
	var rep nap.BaoCaoDung
	docs, _, err := nap.Nap{Cfg: cfg}.ChuanBiQuan(ctx, pool, &rep)
	if err != nil {
		return err
	}
	var need []nap.HoSoQuan
	for _, d := range docs {
		if !d.TT.Co {
			need = append(need, d.HoSo)
		}
	}
	done, hong, b := nap.ChayLamGiau(ctx, m, cfg, tranGoi, need)
	if err := nap.GhiLamGiau(ctx, pool, done); err != nil {
		return err
	}
	for id, e := range hong {
		if err := nap.GhiDLQ(ctx, pool, nap.CorpusQuan, id, nap.ChangLamGiau, e); err != nil {
			return err
		}
	}
	printJSON(stdout, b)
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
// writer. The indexer pass every minute and on every message of the lane
// 'rag' (MOBILE_AMQP_URL set), and the alias reconciler every minute, each
// under its own advisory lock.
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
	cm := nap.ChiMuc{Nap: n, Logger: logger}
	periodic := []jobs.DinhKy{cm.DinhKy(), n.DinhKyDoiChieu(logger)}
	var side sync.WaitGroup
	side.Add(1)
	go func() { defer side.Done(); _ = jobs.ChayDinhKy(ctx, pool, logger, periodic) }()
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
