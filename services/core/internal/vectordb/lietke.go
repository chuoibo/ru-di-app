package vectordb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/milvus-io/milvus/client/v3/column"
	"github.com/milvus-io/milvus/client/v3/entity"
	"github.com/milvus-io/milvus/client/v3/milvusclient"
)

// The ingest's reads and attribute writes (rag/nap through vectordb/napkho):
// listings for reconciliation, filtered listings for the gate's probes, and
// an attribute-only update. All at Strong consistency: each is asked right
// after a write, to check it.

// KhoaHang is a row's keys as reconciliation reads them.
type KhoaHang struct {
	ID          string
	DocID       string
	ContentHash string
	EmbedModel  string
	// Dau is the writer's fingerprint stored in FMoRong (KhoaTheoDoc only).
	Dau string
}

var cotKhoa = []string{FID, FDocID, FContentHash, FEmbedModel}

func docKhoa(rs milvusclient.ResultSet) ([]KhoaHang, error) {
	cols := make([]column.Column, len(cotKhoa))
	for i, name := range cotKhoa {
		if cols[i] = rs.GetColumn(name); cols[i] == nil {
			return nil, fmt.Errorf("vectordb: %s missing from the answer", name)
		}
	}
	out := make([]KhoaHang, 0, cols[0].Len())
	for i := 0; i < cols[0].Len(); i++ {
		var v [4]string
		for j := range cols {
			s, err := cols[j].GetAsString(i)
			if err != nil {
				return nil, err
			}
			v[j] = s
		}
		out = append(out, KhoaHang{ID: v[0], DocID: v[1], ContentHash: v[2], EmbedModel: v[3]})
	}
	return out, nil
}

func sapKhoa(out []KhoaHang) []KhoaHang {
	slices.SortFunc(out, func(a, b KhoaHang) int {
		switch {
		case a.ID < b.ID:
			return -1
		case a.ID > b.ID:
			return 1
		}
		return 0
	})
	return out
}

// LietKe lists every row's keys of name.
func (m *Milvus) LietKe(ctx context.Context, name string) ([]KhoaHang, error) {
	it, err := m.cli.QueryIterator(ctx, milvusclient.NewQueryIteratorOption(name).WithBatchSize(1000).
		WithOutputFields(cotKhoa...).WithConsistencyLevel(entity.ClStrong))
	if err != nil {
		return nil, err
	}
	var out []KhoaHang
	for {
		rs, err := it.Next(ctx)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		got, err := docKhoa(rs)
		if err != nil {
			return nil, err
		}
		out = append(out, got...)
	}
	return sapKhoa(out), nil
}

// MaxLoc bounds one filtered listing: Milvus's offset+limit ceiling. A
// filter admitting more is an error, never a silent truncation.
const MaxLoc = 16384

// LocDiaDiem lists the keys of every place row l admits (the same
// expression a search filters with, so the gate probes exactly what
// retrieval serves).
func (m *Milvus) LocDiaDiem(ctx context.Context, name string, l LocCung) ([]KhoaHang, error) {
	expr, params := l.BieuThuc()
	opt := milvusclient.NewQueryOption(name).WithFilter(expr).WithOutputFields(cotKhoa...).
		WithLimit(MaxLoc).WithConsistencyLevel(entity.ClStrong)
	for k, v := range params {
		opt = opt.WithTemplateParam(k, v)
	}
	rs, err := m.cli.Query(ctx, opt)
	if err != nil {
		return nil, err
	}
	out, err := docKhoa(rs)
	if err != nil {
		return nil, err
	}
	if len(out) >= MaxLoc {
		return nil, fmt.Errorf("vectordb: a filter admits %d rows or more", MaxLoc)
	}
	return sapKhoa(out), nil
}

// IDCuaDoc lists the row ids of document docID in name (one for a place
// since rd.v4; an older collection may hold several chunks of it).
func (m *Milvus) IDCuaDoc(ctx context.Context, name, docID string) ([]string, error) {
	rs, err := m.cli.Query(ctx, milvusclient.NewQueryOption(name).WithFilter(FDocID+" == {d}").
		WithTemplateParam("d", docID).WithOutputFields(FID).WithLimit(1024).WithConsistencyLevel(entity.ClStrong))
	if err != nil {
		return nil, err
	}
	col := rs.GetColumn(FID)
	if col == nil {
		return nil, nil
	}
	out := make([]string, 0, col.Len())
	for i := 0; i < col.Len(); i++ {
		s, err := col.GetAsString(i)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	slices.Sort(out)
	return out, nil
}

// CapNhatThuocTinh rewrites the hard-constraint attributes of document
// docID's rows in name, in place (a partial upsert of those columns only:
// vectors, text and hashes stay). It is how an attribute change reaches a
// collection built with another configuration, whose vectors the current
// pipeline cannot rebuild. The categories are rewritten too where the
// collection has them (rd.v4 on; an rd.v3 collection has none). It returns
// how many rows it rewrote; a document with no row in name rewrites none.
func (m *Milvus) CapNhatThuocTinh(ctx context.Context, name, docID string, t ThuocTinh, giaMaxVND int64) (int, error) {
	ids, err := m.IDCuaDoc(ctx, name, docID)
	if err != nil || len(ids) == 0 {
		return 0, err
	}
	t.GiaMaxVND = giaMaxVND
	hs := make([]CapNhatHang, len(ids))
	for i, id := range ids {
		hs[i] = CapNhatHang{ID: id, ThuocTinh: t}
	}
	return m.CapNhatThuocTinhLo(ctx, name, hs)
}

// CapNhatHang is one row's new attributes for CapNhatThuocTinhLo; MoRong,
// when set, replaces the row's FMoRong dict as well.
type CapNhatHang struct {
	ID        string
	ThuocTinh ThuocTinh
	MoRong    []byte
}

// CapNhatThuocTinhLo rewrites the hard-constraint attributes (and FMoRong
// where given) of the rows hs name, in one partial upsert: vectors, text and
// hashes stay. Every id must already be in name -- a partial upsert of a
// missing key would have to insert a row without a vector, and Milvus
// refuses the whole call -- so the ingest passes only ids it has just read
// back (KhoaTheoDoc). MoRong is all or nothing: every row carries one or
// none does. It returns how many rows it rewrote.
func (m *Milvus) CapNhatThuocTinhLo(ctx context.Context, name string, hs []CapNhatHang) (int, error) {
	n := len(hs)
	if n == 0 {
		return 0, nil
	}
	coDanhMuc, err := m.coTruong(ctx, name, FDanhMuc)
	if err != nil {
		return 0, err
	}
	ids, dests, slots := make([]string, n), make([]string, n), make([][]int16, n)
	pmin, pmax := make([]int64, n), make([]int64, n)
	alg, diet, dm := make([][]string, n), make([][]string, n), make([][]string, n)
	tomb := make([]bool, n)
	var moRong [][]byte
	for i, h := range hs {
		t := h.ThuocTinh
		if dm[i], err = danhMucLuu(t); err != nil {
			return 0, err
		}
		ids[i], dests[i], slots[i] = h.ID, t.DiemDen, notNil(t.OSlots)
		pmin[i], pmax[i] = t.GiaMinVND, t.GiaMaxVND
		alg[i], diet[i], tomb[i] = notNil(t.DiUng), notNil(t.AnKieng), t.GoBo
		if (len(h.MoRong) > 0) != (len(hs[0].MoRong) > 0) {
			return 0, fmt.Errorf("vectordb: %s given for some rows of a batch, not all", FMoRong)
		}
		if len(h.MoRong) > 0 {
			if !json.Valid(h.MoRong) || h.MoRong[0] != '{' {
				return 0, fmt.Errorf("vectordb: %s of %s must be a JSON object", FMoRong, h.ID)
			}
			moRong = append(moRong, h.MoRong)
		}
	}
	opt := milvusclient.NewColumnBasedInsertOption(name).
		WithVarcharColumn(FID, ids).
		WithVarcharColumn(FDestination, dests).
		WithColumns(column.NewColumnInt16Array(FOpenSlots, slots)).
		WithInt64Column(FPriceMin, pmin).
		WithInt64Column(FPriceMax, pmax).
		WithColumns(column.NewColumnVarCharArray(FAllergens, alg), column.NewColumnVarCharArray(FDiets, diet)).
		WithBoolColumn(FTombstoned, tomb).
		WithPartialUpdate(true)
	if coDanhMuc {
		opt = opt.WithColumns(column.NewColumnVarCharArray(FDanhMuc, dm))
	}
	if moRong != nil {
		opt = opt.WithColumns(column.NewColumnJSONBytes(FMoRong, moRong))
	}
	if _, err = m.cli.Upsert(ctx, opt); err != nil {
		return 0, err
	}
	return n, nil
}

// KhoaTheoDoc reads the keys of every row of the documents docIDs in place
// collection name, at strong consistency, with the fingerprint each row's
// FMoRong carries (MoRongDau; "" when it has none): what the indexer
// compares to know whether a row needs a new vector, only its attributes,
// or nothing. One query for the whole batch.
func (m *Milvus) KhoaTheoDoc(ctx context.Context, name string, docIDs []string) ([]KhoaHang, error) {
	if len(docIDs) == 0 {
		return nil, nil
	}
	if len(docIDs) > MaxLoc/8 {
		return nil, fmt.Errorf("vectordb: %d documents in one key read, at most %d", len(docIDs), MaxLoc/8)
	}
	rs, err := m.cli.Query(ctx, milvusclient.NewQueryOption(name).WithFilter(FDocID+" in {ds}").
		WithTemplateParam("ds", docIDs).WithOutputFields(append(slices.Clone(cotKhoa), FMoRong)...).
		WithLimit(MaxLoc).WithConsistencyLevel(entity.ClStrong))
	if err != nil {
		return nil, err
	}
	out, err := docKhoa(rs)
	if err != nil || len(out) == 0 {
		return out, err
	}
	mr := rs.GetColumn(FMoRong)
	if mr == nil {
		return nil, fmt.Errorf("vectordb: %s missing from the answer", FMoRong)
	}
	for i := range out {
		v, err := mr.Get(i)
		if err != nil {
			return nil, err
		}
		if b, ok := v.([]byte); ok && len(b) > 0 {
			var d map[string]json.RawMessage
			if json.Unmarshal(b, &d) == nil {
				_ = json.Unmarshal(d[MoRongDau], &out[i].Dau)
			}
		}
	}
	return sapKhoa(out), nil
}

// coTruong reports whether collection name has field f.
func (m *Milvus) coTruong(ctx context.Context, name, f string) (bool, error) {
	c, err := m.cli.DescribeCollection(ctx, milvusclient.NewDescribeCollectionOption(name))
	if err != nil {
		return false, err
	}
	for _, x := range c.Schema.Fields {
		if x.Name == f {
			return true, nil
		}
	}
	return false, nil
}

// XoaDoc deletes every row of document docID from name, whatever ids they
// carry: how a place leaves a collection built by any configuration (an
// rd.v3 collection holds it as several chunks, an rd.v4 one as one row).
func (m *Milvus) XoaDoc(ctx context.Context, name, docID string) error {
	_, err := m.cli.Delete(ctx, milvusclient.NewDeleteOption(name).WithExpr(FDocID+" == {d}").WithTemplateParam("d", docID))
	return err
}

// DocThuocTinh reads back the attributes stored on document docID's rows of
// place collection name (rd.v4; tests and the ingest's own checks): one
// ThuocTinh per row id.
func (m *Milvus) DocThuocTinh(ctx context.Context, name, docID string) (map[string]ThuocTinh, error) {
	rs, err := m.cli.Query(ctx, milvusclient.NewQueryOption(name).WithFilter(FDocID+" == {d}").
		WithTemplateParam("d", docID).
		WithOutputFields(FID, FDestination, FOpenSlots, FPriceMin, FPriceMax, FAllergens, FDiets, FDanhMuc, FTombstoned).
		WithLimit(1024).WithConsistencyLevel(entity.ClStrong))
	if err != nil {
		return nil, err
	}
	out := map[string]ThuocTinh{}
	ids := rs.GetColumn(FID)
	if ids == nil {
		return out, nil
	}
	for i := 0; i < ids.Len(); i++ {
		id, _ := ids.GetAsString(i)
		var t ThuocTinh
		t.DiemDen, _ = rs.GetColumn(FDestination).GetAsString(i)
		t.GiaMinVND, _ = rs.GetColumn(FPriceMin).GetAsInt64(i)
		t.GiaMaxVND, _ = rs.GetColumn(FPriceMax).GetAsInt64(i)
		t.GoBo, _ = rs.GetColumn(FTombstoned).GetAsBool(i)
		if v, err := rs.GetColumn(FOpenSlots).Get(i); err == nil {
			t.OSlots, _ = v.([]int16)
		}
		if v, err := rs.GetColumn(FAllergens).Get(i); err == nil {
			t.DiUng, _ = v.([]string)
		}
		if v, err := rs.GetColumn(FDiets).Get(i); err == nil {
			t.AnKieng, _ = v.([]string)
		}
		if v, err := rs.GetColumn(FDanhMuc).Get(i); err == nil {
			t.DanhMuc, _ = v.([]string)
		}
		out[id] = t
	}
	return out, nil
}

// CapNhatMoRong replaces the FMoRong dict of the given rows of place
// collection name, touching no other field (partial update): the way a
// field that arrives after the schema is added without a new revision.
func (m *Milvus) CapNhatMoRong(ctx context.Context, name string, ids []string, moRong []byte) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	if !json.Valid(moRong) || len(moRong) == 0 || moRong[0] != '{' {
		return 0, fmt.Errorf("vectordb: %s must be a JSON object", FMoRong)
	}
	vals := make([][]byte, len(ids))
	for i := range vals {
		vals[i] = moRong
	}
	_, err := m.cli.Upsert(ctx, milvusclient.NewColumnBasedInsertOption(name).
		WithVarcharColumn(FID, ids).
		WithColumns(column.NewColumnJSONBytes(FMoRong, vals)).
		WithPartialUpdate(true))
	if err != nil {
		return 0, err
	}
	return len(ids), nil
}
