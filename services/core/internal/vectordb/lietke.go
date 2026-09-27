package vectordb

import (
	"context"
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

// IDCuaDoc lists the chunk ids of document docID in name.
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
// pipeline cannot rebuild. It returns how many rows it rewrote; a document
// with no row in name rewrites none.
func (m *Milvus) CapNhatThuocTinh(ctx context.Context, name, docID string, t ThuocTinh, giaMaxVND int64) (int, error) {
	ids, err := m.IDCuaDoc(ctx, name, docID)
	if err != nil || len(ids) == 0 {
		return 0, err
	}
	n := len(ids)
	dests, slots := make([]string, n), make([][]int16, n)
	pmin, pmax := make([]int64, n), make([]int64, n)
	alg, diet := make([][]string, n), make([][]string, n)
	tomb := make([]bool, n)
	for i := range ids {
		dests[i], slots[i] = t.DiemDen, notNil(t.OSlots)
		pmin[i], pmax[i] = t.GiaMinVND, giaMaxVND
		alg[i], diet[i], tomb[i] = notNil(t.DiUng), notNil(t.AnKieng), t.GoBo
	}
	_, err = m.cli.Upsert(ctx, milvusclient.NewColumnBasedInsertOption(name).
		WithVarcharColumn(FID, ids).
		WithVarcharColumn(FDestination, dests).
		WithColumns(column.NewColumnInt16Array(FOpenSlots, slots)).
		WithInt64Column(FPriceMin, pmin).
		WithInt64Column(FPriceMax, pmax).
		WithColumns(column.NewColumnVarCharArray(FAllergens, alg), column.NewColumnVarCharArray(FDiets, diet)).
		WithBoolColumn(FTombstoned, tomb).
		WithPartialUpdate(true))
	if err != nil {
		return 0, err
	}
	return n, nil
}

// DocThuocTinh reads back the attributes stored on document docID's rows of
// name (tests and the ingest's own checks): one ThuocTinh per chunk id.
func (m *Milvus) DocThuocTinh(ctx context.Context, name, docID string) (map[string]ThuocTinh, error) {
	rs, err := m.cli.Query(ctx, milvusclient.NewQueryOption(name).WithFilter(FDocID+" == {d}").
		WithTemplateParam("d", docID).
		WithOutputFields(FID, FDestination, FOpenSlots, FPriceMin, FAllergens, FDiets, FTombstoned).
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
		out[id] = t
	}
	return out, nil
}
