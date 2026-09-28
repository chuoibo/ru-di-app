// Package aidoc implements the engine's read ports over PostgreSQL: the
// lexical places retriever (truyhoi.Retriever over rag.Retrieve) and the
// tools' catalogue, group, own-outing and couple's-taste readers
// (tools.DocCho, DocNhom, DocCaNhan, DocDoi).
//
// Every read runs in its own READ ONLY transaction (the database refuses a
// write, SQLSTATE 25006) and under a semaphore that bounds how many
// connections the engine's tools hold at once, so a turn's parallel tool
// calls cannot starve the request path of the pool.
//
// Nothing here reads the person's words. The retriever maps the contract's
// request field by field onto rag's: the hard constraints arrive as ids, an
// instant and an integer the MODEL extracted; the query text goes to rag's
// ranking only (BM25/trigram terms are retrieval scoring, not
// understanding). No word list of domain/tuvung and no destination resolver
// runs on the query (TestKhongDocChu).
package aidoc

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"mobile/services/core/internal/aiharness/tools"
	"mobile/services/core/internal/aiharness/truyhoi"
	"mobile/services/core/internal/domain/areas"
	"mobile/services/core/internal/domain/giomo"
	"mobile/services/core/internal/domain/pairnotebook"
	"mobile/services/core/internal/gudoi"
	"mobile/services/core/internal/rag"
	"mobile/services/core/internal/repo"
	"mobile/services/core/internal/thuoctinh"
)

// Beginner opens a transaction with options: a pool.
type Beginner interface {
	BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error)
}

// ChiDoc runs reads in ReadOnly transactions under a semaphore.
type ChiDoc struct {
	db  Beginner
	sem chan struct{}
}

// DongThoiMacDinh is how many tool reads may hold a connection at once.
const DongThoiMacDinh = 4

// Moi wraps db; dongThoi <= 0 is DongThoiMacDinh.
func Moi(db Beginner, dongThoi int) *ChiDoc {
	if dongThoi <= 0 {
		dongThoi = DongThoiMacDinh
	}
	return &ChiDoc{db: db, sem: make(chan struct{}, dongThoi)}
}

// Doc runs fn in a READ ONLY, REPEATABLE READ transaction, waiting for a
// semaphore slot first (a done context stops the wait). The transaction is
// always rolled back: a read has nothing to commit.
func (c *ChiDoc) Doc(ctx context.Context, fn func(pgx.Tx) error) error {
	select {
	case c.sem <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-c.sem }()
	tx, err := c.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	return fn(tx)
}

// BangChungQuan is one place as evidence: the fields an answer may quote.
func BangChungQuan(p repo.Place) truyhoi.BangChung {
	t := map[string]string{"ten": p.Name, "loai": p.Category, "diem_den": p.DestinationID}
	if p.Address != nil {
		t["dia_chi"] = *p.Address
	}
	if p.PriceMinVND != nil {
		t["gia_min_vnd"] = strconv.FormatInt(*p.PriceMinVND, 10)
	}
	if p.PriceMaxVND != nil {
		t["gia_max_vnd"] = strconv.FormatInt(*p.PriceMaxVND, 10)
	}
	if p.OpenHours != nil {
		t["gio"] = *p.OpenHours
	}
	return truyhoi.BangChung{ID: p.ID, Nguon: truyhoi.Places, Truong: t}
}

// Lexical is the places retriever over rag's lexical index (full text,
// trigram, soft lists, RRF), or its live-row fallback.
type Lexical struct{ C *ChiDoc }

var _ truyhoi.Retriever = Lexical{}

// YeuCauRag maps the contract's request onto rag's, field by field: the
// destination, allergens, diets and budget as the model extracted them, the
// open instant as its minute of the week on Vietnam's clock, the soft lists,
// the query text for ranking. The area preference has no rag counterpart
// and ranks nothing here.
func YeuCauRag(y truyhoi.YeuCau) rag.YeuCau {
	r := rag.YeuCau{
		DiemDen:  y.Cung.DiemDenID,
		Cau:      y.Cau,
		DiUng:    append([]string(nil), y.Cung.DiUng...),
		AnKieng:  append([]string(nil), y.Cung.AnKieng...),
		NganSach: y.Cung.NganSachVND,
		LoaiCho:  append([]string(nil), y.Mem.LoaiCho...),
		KhiChat:  append([]string(nil), y.Mem.KhiChat...),
		K:        y.K,
	}
	if y.Cung.MoLuc != nil {
		m := giomo.PhutCuaTuan(*y.Cung.MoLuc)
		r.Luc = &m
	}
	if k := y.Cung.MoTrong; k != nil {
		// rag's window is [tu, den) in minutes of the week; den past the
		// week's end wraps (giomo.Khung), so a window crossing Sunday night
		// stays one window.
		tu := giomo.PhutCuaTuan(k.Tu)
		r.Khung = &[2]int{tu, tu + int(k.Den.Sub(k.Tu)/time.Minute)}
	}
	return r
}

// GiuChuaRo applies the engine's rule for unknown attributes
// (docs/architecture/03 §8.4) to a lexical hit's flags: under a hard
// budget a place with an unknown price is out, under a hard open instant or
// window a place with unknown hours is out. rag.Retrieve keeps them flagged
// (design 04 §4a, what POST /places/search still serves); every constraint
// the engine passes is hard, so the engine's path drops them -- the same
// answer the hybrid index gives.
func GiuChuaRo(y truyhoi.YeuCau, co []string) bool {
	for _, c := range co {
		switch {
		case c == rag.CoGiaChuaRo && y.Cung.NganSachVND != nil:
			return false
		case c == rag.CoGioChuaRo && (y.Cung.MoLuc != nil || y.Cung.MoTrong != nil):
			return false
		}
	}
	return true
}

// ChuaRo are the evidence flags of a live place's unknown attributes:
// "gio_chua_ro" when its hours are missing or unreadable, "gia_chua_ro"
// when its price is. Kept and said when no hard constraint asks about them.
func ChuaRo(p repo.Place) []string {
	var out []string
	if p.OpenHours == nil {
		out = append(out, rag.CoGioChuaRo)
	} else if _, ok := giomo.Doc(*p.OpenHours); !ok {
		out = append(out, rag.CoGioChuaRo)
	}
	if p.PriceMinVND == nil {
		out = append(out, rag.CoGiaChuaRo)
	}
	return out
}

// Tim answers a places request. Hard constraints are rag's filters, never
// relaxed; fewer results is the answer, and an unknown price or hours under
// a hard constraint on it is out (GiuChuaRo). Every result is flagged
// lexical_only: this adapter has no dense or sparse vectors.
func (l Lexical) Tim(ctx context.Context, y truyhoi.YeuCau) (truyhoi.KetQuaTruyHoi, error) {
	if err := y.Kiem(); err != nil {
		return truyhoi.KetQuaTruyHoi{}, err
	}
	if y.Nguon != truyhoi.Places {
		return truyhoi.KetQuaTruyHoi{}, truyhoi.ErrYeuCau
	}
	var out truyhoi.KetQuaTruyHoi
	err := l.C.Doc(ctx, func(tx pgx.Tx) error {
		kq, err := rag.Kho{Q: tx}.Retrieve(ctx, YeuCauRag(y))
		if err != nil {
			return err
		}
		ids := make([]string, len(kq.Quan))
		for i, h := range kq.Quan {
			ids[i] = h.ID
		}
		rows, err := repo.Repository{Q: tx}.PlacesByID(ctx, ids)
		if err != nil {
			return err
		}
		byID := map[string]repo.Place{}
		for _, p := range rows {
			byID[p.ID] = p
		}
		ban := ""
		if kq.PhienBan != 0 {
			ban = strconv.FormatInt(kq.PhienBan, 10)
		}
		for _, h := range kq.Quan {
			p, ok := byID[h.ID]
			if !ok || !GiuChuaRo(y, h.Co) {
				continue
			}
			b := BangChungQuan(p)
			b.Diem = float64(h.Diem)
			b.PhienBanChiMuc = ban
			if co := ChuaRo(p); len(co) > 0 {
				b.Truong["chua_ro"] = strings.Join(co, ",")
			}
			out.BangChung = append(out.BangChung, b)
		}
		return nil
	})
	if err != nil {
		return truyhoi.KetQuaTruyHoi{}, err
	}
	out.Degraded = []truyhoi.CoSuyGiam{truyhoi.LexicalOnly}
	return out, nil
}

// ThuocTinhSong is the hybrid retriever's re-check read (hybrid.DocSong):
// the live rows of hit ids and the attributes the ingest derives for them
// (thuoctinh.Doc: places, place_enrichments, rag_tombstones), in a READ
// ONLY transaction under the tools' semaphore.
type ThuocTinhSong struct{ C *ChiDoc }

// DocSong reads the live rows of ids.
func (s ThuocTinhSong) DocSong(ctx context.Context, ids []string) (map[string]thuoctinh.Hang, error) {
	var out map[string]thuoctinh.Hang
	err := s.C.Doc(ctx, func(tx pgx.Tx) error {
		var err error
		out, err = thuoctinh.Doc(ctx, tx, ids)
		return err
	})
	return out, err
}

// Doc implements the tools' read ports.
type Doc struct{ C *ChiDoc }

var (
	_ tools.DocCho    = Doc{}
	_ tools.DocNhom   = Doc{}
	_ tools.DocCaNhan = Doc{}
)

// Quan returns the places with these ids, in the order asked, as the index
// would show them: a place whose naming fields are unsafe
// (rag.DungHoSo), or taken down, is not returned.
func (d Doc) Quan(ctx context.Context, ids []string) ([]truyhoi.BangChung, error) {
	var out []truyhoi.BangChung
	err := d.C.Doc(ctx, func(tx pgx.Tx) error {
		rows, err := repo.Repository{Q: tx}.PlacesByID(ctx, ids)
		if err != nil {
			return err
		}
		ha, err := daHa(ctx, tx, ids)
		if err != nil {
			return err
		}
		byID := map[string]repo.Place{}
		for _, p := range rows {
			byID[p.ID] = p
		}
		for _, id := range ids {
			p, ok := byID[id]
			if !ok || ha[id] {
				continue
			}
			if _, r := rag.DungHoSo(p); r.Bo {
				continue
			}
			out = append(out, BangChungQuan(p))
		}
		return nil
	})
	return out, err
}

// daHa are the ids among ids that the index tombstoned; none when the
// index schema is not installed.
func daHa(ctx context.Context, tx pgx.Tx, ids []string) (map[string]bool, error) {
	out := map[string]bool{}
	ok, err := rag.Installed(ctx, tx)
	if err != nil || !ok {
		return out, err
	}
	rows, err := tx.Query(ctx, `SELECT doc_id FROM rag_tombstones WHERE corpus = 'place' AND doc_id = ANY($1::text[])`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// DiemDen returns every destination: the closed list the router picks a
// destination id from. A catalogue holds curated cities and the province
// destinations the ingest seeds side by side (rag.PhamViCua), so «Hà Nội»
// and «Thành phố Hà Nội» both appear; each name says which it is, from the
// geography alone -- a province is «toàn tỉnh/thành», a curated city names
// the province destination it lies in (rag.TinhCua) -- and either choice
// retrieves the places filed under the other that lie in it.
func (d Doc) DiemDen(ctx context.Context) ([]truyhoi.BangChung, error) {
	var out []truyhoi.BangChung
	err := d.C.Doc(ctx, func(tx pgx.Tx) error {
		ds, err := repo.Repository{Q: tx}.ListDestinations(ctx)
		if err != nil {
			return err
		}
		all := rag.DiemDenTuRepo(ds)
		ten := map[string]string{}
		for _, x := range ds {
			ten[x.ID] = x.Name
		}
		for _, x := range ds {
			name := x.Name
			if rag.LaTinh(x.ID) {
				name += " (toàn tỉnh/thành)"
			} else if p := rag.TinhCua(all, x.ID); p != "" {
				name += " (thuộc " + ten[p] + ")"
			}
			out = append(out, truyhoi.BangChung{ID: x.ID, Truong: map[string]string{"ten": name}})
		}
		return nil
	})
	return out, err
}

// KhuVuc returns the areas whose centre lies inside the destination's
// bounding box, in the catalogue's order.
func (d Doc) KhuVuc(ctx context.Context, diemDenID string) ([]truyhoi.BangChung, error) {
	var out []truyhoi.BangChung
	err := d.C.Doc(ctx, func(tx pgx.Tx) error {
		x, err := repo.Repository{Q: tx}.GetDestination(ctx, diemDenID)
		if err != nil || x == nil {
			return err
		}
		for _, a := range areas.All() {
			if a.Lat >= x.BBoxSouth && a.Lat <= x.BBoxNorth && a.Lng >= x.BBoxWest && a.Lng <= x.BBoxEast {
				out = append(out, truyhoi.BangChung{ID: a.ID, Truong: map[string]string{"ten": a.Label}})
			}
		}
		return nil
	})
	return out, err
}

// The outing fields a tool shows: title, dates, headcount. No money.
const cotChuyenDi = `o.id::text, o.title, o.starts_on, o.ends_on, o.headcount`

func docChuyenDi(rows pgx.Rows) ([]truyhoi.BangChung, error) {
	defer rows.Close()
	var out []truyhoi.BangChung
	for rows.Next() {
		var id, title string
		var tu, den time.Time
		var n int64
		if err := rows.Scan(&id, &title, &tu, &den, &n); err != nil {
			return nil, err
		}
		out = append(out, truyhoi.BangChung{ID: id, Nguon: truyhoi.GroupHistory, Truong: map[string]string{
			"tieu_de": title, "tu_ngay": tu.Format(time.DateOnly), "den_ngay": den.Format(time.DateOnly), "so_nguoi": strconv.FormatInt(n, 10),
		}})
	}
	return out, rows.Err()
}

// ErrID: an identity that is not a UUID never reaches SQL.
var ErrID = errors.New("aidoc: id is not a uuid")

func laUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, r := range s {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
				return false
			}
		}
	}
	return true
}

// ChuyenDi returns the group's outings (DocNhom).
func (d Doc) ChuyenDi(ctx context.Context, nhomID string, ngay time.Time, sapToi bool, k int) ([]truyhoi.BangChung, error) {
	if !laUUID(nhomID) {
		return nil, ErrID
	}
	q := `SELECT ` + cotChuyenDi + ` FROM outings o WHERE o.context_id = $1::uuid AND o.ends_on >= $2::date ORDER BY o.starts_on, o.id LIMIT $3`
	if !sapToi {
		q = `SELECT ` + cotChuyenDi + ` FROM outings o WHERE o.context_id = $1::uuid AND o.ends_on < $2::date ORDER BY o.starts_on DESC, o.id DESC LIMIT $3`
	}
	var out []truyhoi.BangChung
	err := d.C.Doc(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, q, nhomID, ngay.Format(time.DateOnly), k)
		if err != nil {
			return err
		}
		out, err = docChuyenDi(rows)
		return err
	})
	return out, err
}

// SoThanhVien counts the group's active members (DocNhom).
func (d Doc) SoThanhVien(ctx context.Context, nhomID string) (int, error) {
	if !laUUID(nhomID) {
		return 0, ErrID
	}
	var n int
	err := d.C.Doc(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT count(*) FROM memberships WHERE context_id = $1::uuid AND state = 'active' AND left_at IS NULL`, nhomID).Scan(&n)
	})
	return n, err
}

var _ tools.DocDoi = Doc{}

// The couple's taste reads (ADR-0048), in one READ ONLY transaction: the
// room's live notebook cycle, its two participants, their `bat_doi` and
// `chia_gu` consent rows, and -- only for the people gudoi.NguoiDuocDung
// keeps -- their closed-vocabulary tags. No read names a person's name, a
// budget, a message or the notebook's shared constraints; aigate's gate on
// Nếp's path pins that (TestGuDoiDocDungCot).
const (
	cauChuKyDoi = `SELECT c.id::text FROM pair_notebooks n JOIN pair_notebook_cycles c ON c.notebook_id = n.id WHERE n.context_id = $1::uuid AND c.state <> 'closed' ORDER BY c.created_at DESC LIMIT 1`
	cauNguoiDoi = `SELECT person_id::text FROM pair_cycle_participants WHERE cycle_id = $1::uuid ORDER BY created_at, person_id`
	cauDongYDoi = `SELECT k.person_id::text, k.proposal_id::text, p.purpose, k.granted_at, k.revoked_at, p.expires_at, p.completed_at FROM pair_consents k JOIN pair_consent_proposals p ON p.id = k.proposal_id WHERE p.cycle_id = $1::uuid AND p.purpose IN ('bat_doi', 'chia_gu')`
	cauGuDoi    = `SELECT person_id::text, tag FROM person_interests WHERE person_id = ANY($1::uuid[]) ORDER BY person_id, tag`
)

// GuDoi returns the shared taste of room phong's couple (DocDoi): nothing
// when the room has no live notebook cycle or its two are not a couple;
// otherwise the tags of each participant whose `chia_gu` covers the chat
// (granted at or after gudoi.MocChat, still live), asked at every call,
// never cached. A revoked consent or a broken «Một đôi» is seen at the
// next call.
func (d Doc) GuDoi(ctx context.Context, phong string) ([]gudoi.Gu, error) {
	if !laUUID(phong) {
		return nil, ErrID
	}
	out := []gudoi.Gu{}
	err := d.C.Doc(ctx, func(tx pgx.Tx) error {
		var chuKy string
		err := tx.QueryRow(ctx, cauChuKyDoi, phong).Scan(&chuKy)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		rows, err := tx.Query(ctx, cauNguoiDoi, chuKy)
		if err != nil {
			return err
		}
		nguoi, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			return err
		}
		rows, err = tx.Query(ctx, cauDongYDoi, chuKy)
		if err != nil {
			return err
		}
		var dongY []pairnotebook.Consent
		for rows.Next() {
			var c pairnotebook.Consent
			var het time.Time
			if err := rows.Scan(&c.PersonID, &c.ProposalID, &c.Purpose, &c.GrantedAt, &c.RevokedAt, &het, &c.ProposalCompletedAt); err != nil {
				rows.Close()
				return err
			}
			c.ProposalExpiresAt = &het
			dongY = append(dongY, c)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		duocDung := gudoi.NguoiDuocDung(dongY, nguoi, time.Now().UTC())
		if len(duocDung) == 0 {
			return nil
		}
		rows, err = tx.Query(ctx, cauGuDoi, duocDung)
		if err != nil {
			return err
		}
		the := map[string][]string{}
		for rows.Next() {
			var ai, tag string
			if err := rows.Scan(&ai, &tag); err != nil {
				rows.Close()
				return err
			}
			the[ai] = append(the[ai], tag)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		out = gudoi.GuChoChat(duocDung, the)
		return nil
	})
	return out, err
}

// ChuyenDiSapToi returns the person's upcoming outings across the groups
// they are an active member of (DocCaNhan).
func (d Doc) ChuyenDiSapToi(ctx context.Context, nguoi string, ngay time.Time, k int) ([]truyhoi.BangChung, error) {
	if !laUUID(nguoi) {
		return nil, ErrID
	}
	var out []truyhoi.BangChung
	err := d.C.Doc(ctx, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT `+cotChuyenDi+`
		   FROM outings o JOIN memberships m ON m.context_id = o.context_id
		  WHERE m.person_id = $1::uuid AND m.state = 'active' AND m.left_at IS NULL AND o.ends_on >= $2::date
		  ORDER BY o.starts_on, o.id LIMIT $3`, nguoi, ngay.Format(time.DateOnly), k)
		if err != nil {
			return err
		}
		out, err = docChuyenDi(rows)
		return err
	})
	return out, err
}
