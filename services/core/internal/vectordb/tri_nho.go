package vectordb

import (
	"context"
	"errors"
	"fmt"
	"unicode"
	"unicode/utf8"

	"github.com/milvus-io/milvus/client/v3/entity"
	"github.com/milvus-io/milvus/client/v3/milvusclient"

	"mobile/services/core/internal/aiharness/nhung"
)

// ChuSoHuu is the person a memory belongs to. It is its own type so no call
// can pass an empty or unchecked string where an owner is meant: the
// partition key is not a security boundary, the filter on it is, and that
// filter is written here once (research milvus.md §5, multi-tenancy).
type ChuSoHuu struct{ id string }

// ErrChuSoHuu: an empty, too long or control-character owner id.
var ErrChuSoHuu = errors.New("vectordb: memory owner id empty or malformed")

// MoiChuSoHuu checks an owner id's structure.
func MoiChuSoHuu(id string) (ChuSoHuu, error) {
	if id == "" || len(id) > MaxOwnerLen || !utf8.ValidString(id) {
		return ChuSoHuu{}, ErrChuSoHuu
	}
	for _, r := range id {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return ChuSoHuu{}, ErrChuSoHuu
		}
	}
	return ChuSoHuu{id: id}, nil
}

// String is the id.
func (c ChuSoHuu) String() string { return c.id }

// HangTriNho is one memory vector. No text: the fact's words live in
// PostgreSQL, and «forget» deletes them there.
type HangTriNho struct {
	ID       string
	Loai     string
	TaoLuc   int64
	Dense    []float32
	PhienBan int64
}

// SoTriNho is Nếp's memory index for one server: every method takes the
// owner and every read, count and delete carries «owner_id == {o}». It is
// reachable from Nếp's path only; the group assistant's closure never
// reaches it (aigate).
type SoTriNho struct {
	m *Milvus
}

// TriNho is the memory store of m.
func (m *Milvus) TriNho() *SoTriNho { return &SoTriNho{m: m} }

func (k *SoTriNho) ten() string { return k.m.Alias(KhoTriNho) }

func (c ChuSoHuu) kiem() error {
	if c.id == "" {
		return ErrChuSoHuu
	}
	return nil
}

// Ghi upserts the owner's memory vectors.
func (k *SoTriNho) Ghi(ctx context.Context, o ChuSoHuu, rows []HangTriNho) error {
	if err := o.kiem(); err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	n := len(rows)
	ids, owners, kinds := make([]string, n), make([]string, n), make([]string, n)
	at, ver := make([]int64, n), make([]int64, n)
	dense := make([][]float32, n)
	for i, r := range rows {
		if err := kiemDense(r.Dense); err != nil {
			return err
		}
		ids[i], owners[i], kinds[i], at[i], ver[i] = r.ID, o.id, r.Loai, r.TaoLuc, r.PhienBan
		dense[i] = r.Dense
	}
	_, err := k.m.cli.Upsert(ctx, milvusclient.NewColumnBasedInsertOption(k.ten()).
		WithVarcharColumn(FID, ids).WithVarcharColumn(FOwner, owners).WithVarcharColumn(FKind, kinds).
		WithInt64Column(FCreatedAt, at).
		WithFloatVectorColumn(FDense, nhung.Dims, dense).
		WithInt64Column(FIndexVersion, ver))
	return err
}

const locChu = FOwner + " == {o}"

// Tim returns the owner's k nearest memories to the query vector.
func (k *SoTriNho) Tim(ctx context.Context, o ChuSoHuu, q []float32, n int) ([]Trung, error) {
	if err := o.kiem(); err != nil {
		return nil, err
	}
	if err := kiemDense(q); err != nil {
		return nil, err
	}
	rs, err := k.m.cli.Search(ctx, milvusclient.NewSearchOption(k.ten(), n, []entity.Vector{entity.FloatVector(q)}).
		WithANNSField(FDense).WithAnnParam(k.m.chiMucDense().thamSoTim()).
		WithFilter(locChu).WithTemplateParam("o", o.id).
		WithConsistencyLevel(entity.ClStrong).WithOutputFields(FIndexVersion))
	if err != nil {
		return nil, err
	}
	return trungTu(rs)
}

// Xoa deletes the owner's memories with these ids; another person's id in
// the list matches nothing.
func (k *SoTriNho) Xoa(ctx context.Context, o ChuSoHuu, ids []string) error {
	if err := o.kiem(); err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	_, err := k.m.cli.Delete(ctx, milvusclient.NewDeleteOption(k.ten()).
		WithExpr(locChu+" and "+FID+" in {ids}").WithTemplateParam("o", o.id).WithTemplateParam("ids", ids))
	return err
}

// XoaHet deletes every memory of the owner (account deletion).
func (k *SoTriNho) XoaHet(ctx context.Context, o ChuSoHuu) error {
	if err := o.kiem(); err != nil {
		return err
	}
	_, err := k.m.cli.Delete(ctx, milvusclient.NewDeleteOption(k.ten()).WithExpr(locChu).WithTemplateParam("o", o.id))
	return err
}

// Dem counts the owner's memory rows (Strong): the deletion saga counts
// after it deletes, and a receipt is written only on zero.
func (k *SoTriNho) Dem(ctx context.Context, o ChuSoHuu) (int64, error) {
	if err := o.kiem(); err != nil {
		return 0, err
	}
	return k.m.Dem(ctx, k.ten(), locChu, map[string]any{"o": o.id})
}

// NenVaCho compacts the memory collection the alias serves.
func (k *SoTriNho) NenVaCho(ctx context.Context) error {
	name, err := k.m.DangPhucVu(ctx, KhoTriNho)
	if err != nil {
		return fmt.Errorf("vectordb: memory alias: %w", err)
	}
	return k.m.NenVaCho(ctx, name, compactTimeout)
}

// aliasTriNho is the memory collection's alias ("" for any other Kho).
func aliasTriNho(k Kho) string {
	if k == KhoTriNho {
		return "nep_memories"
	}
	return ""
}

// laTriNho reports whether k is the memory collection.
func laTriNho(k Kho) bool { return k == KhoTriNho }
