package main

import (
	"context"
	"errors"
	"testing"

	"mobile/services/core/internal/aidoc"
	"mobile/services/core/internal/aiharness/nhung"
	"mobile/services/core/internal/vectordb"
)

// The places retriever follows the configuration: no Milvus named is the
// lexical index; a Milvus named without its credentials is refused at
// start, never silently served lexically.
func TestQuanRetrieverTheoCauHinh(t *testing.T) {
	doc := aidoc.Moi(nil, 0)
	r, err := quanRetriever(context.Background(), func(string) string { return "" }, doc, nhung.Stub{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := r.(aidoc.Lexical); !ok {
		t.Fatalf("no Milvus: %T, want the lexical index", r)
	}
	_, err = quanRetriever(context.Background(), func(k string) string {
		if k == vectordb.EnvAddr {
			return "127.0.0.1:19530"
		}
		return ""
	}, doc, nhung.Stub{})
	if !errors.Is(err, vectordb.ErrChuaCauHinh) {
		t.Fatalf("a Milvus without credentials: %v", err)
	}
}
