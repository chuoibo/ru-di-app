package tools

import (
	"context"
	"time"

	"mobile/services/core/internal/aiharness/trinho"
	"mobile/services/core/internal/aiharness/truyhoi"
)

// The data ports the tools read through. Each read port is implemented over
// Postgres in internal/aidoc (outside the engine, which holds no database code), inside a ReadOnly transaction under the
// database semaphore; tests pass in-memory fakes. Every method is scoped by
// an identity the TURN carries (BoiCanh), never by one a model argument
// names: no tool schema has a person or group argument.

// DocCho reads the shared catalogue.
type DocCho interface {
	// Quan returns the evidence of the places with these ids, in the order
	// asked, skipping ids the catalogue does not have.
	Quan(ctx context.Context, ids []string) ([]truyhoi.BangChung, error)
	// DiemDen returns every destination the app covers.
	DiemDen(ctx context.Context) ([]truyhoi.BangChung, error)
	// KhuVuc returns the known areas inside one destination.
	KhuVuc(ctx context.Context, diemDenID string) ([]truyhoi.BangChung, error)
}

// DocNhom reads the group the question was asked in.
type DocNhom interface {
	// ChuyenDi returns at most k outings of the group: those not yet over at
	// ngay (sapToi), soonest first, or those over before it, latest first.
	ChuyenDi(ctx context.Context, nhomID string, ngay time.Time, sapToi bool, k int) ([]truyhoi.BangChung, error)
	// SoThanhVien is how many active members the group has.
	SoThanhVien(ctx context.Context, nhomID string) (int, error)
}

// DocCaNhan reads the asking person's own data.
type DocCaNhan interface {
	// ChuyenDiSapToi returns at most k outings, not yet over at ngay, of the
	// groups the person is an active member of, soonest first.
	ChuyenDiSapToi(ctx context.Context, nguoi string, ngay time.Time, k int) ([]truyhoi.BangChung, error)
}

// NguonDuLieu is every port a turn's tools may use. A nil port makes its
// tools answer loi_nguon; nothing falls back to another source.
type NguonDuLieu struct {
	Quan   truyhoi.Retriever
	Cho    DocCho
	Nhom   DocNhom
	CaNhan DocCaNhan
	TriNho trinho.TriNho
}
