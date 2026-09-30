package vectordb

import (
	"context"

	"mobile/services/core/internal/aiharness/nhung"
)

// ThuaTruyVan is a query's sparse leg: the text Milvus's BM25 function
// reads through FText's analyzer (FSparse is the only sparse field, rd.v4).
type ThuaTruyVan struct {
	Text string
}

// Thua is the sparse port the retriever builds a query's leg with. BM25 is
// its one adapter; the port stays so a leg that cannot be built degrades
// the search (truyhoi.NoSparse) instead of failing it.
type Thua interface {
	TruyVan(ctx context.Context, cau string) (ThuaTruyVan, error)
}

// BM25 is Milvus's built-in BM25 over FText: no model, no client vector.
type BM25 struct{}

// TruyVan passes the NFC query text to Milvus's analyzer.
func (BM25) TruyVan(_ context.Context, cau string) (ThuaTruyVan, error) {
	t := nhung.ChuanNFC(cau)
	if t == "" {
		return ThuaTruyVan{}, nhung.ErrRong
	}
	return ThuaTruyVan{Text: t}, nil
}
