// Package nap is the production ingestion pipeline of the vector index:
// the place catalogue and the app manual, from PostgreSQL into versioned
// Milvus collections (research sdlc-production §C; design 04 §6).
//
//	places, rag_tombstones, place_enrichments
//	  ──trigger──▶ rag_danh_dau: rag_dirty + NOTIFY rag_dirty  (change capture)
//	  │                        │ indexer pass (LISTEN, 2 s gather, 20 s
//	  │                        │ safety poll; also the lane 'rag'); writes
//	  │                        │ only what differs (hash + fingerprint)
//	  ▼                        ▼
//	S1 source hash → S2 SafeDeep → S5 LLM enrichment (quarantined, review
//	queue) → S6 dedupe (embedding cosine + haversine) → S7 chunks with
//	deterministic ids → S8 dense (cached) + sparse (Milvus BM25 function, or
//	MILCO behind a flag) → S9 upsert into a NEW physical collection
//	(rd_places__v<N>, rd_manual__v<N>) → S10 reconciliation → S11 offline
//	eval gate → S12 promote by alias swap, rollback by alias swap.
//
// One schema, one writer (docs/architecture/03 §8.4): the collections this
// package builds are the ones retrieval reads (package hybrid, through
// vectordb), with vectordb's schema -- fields, analyzers, indexes -- and
// vectordb's alias names. The Milvus adapter of KhoVector is
// vectordb/napkho, over vectordb's one client constructor. The attributes
// retrieval re-checks every hit against are the ones this package derives
// from place_enrichments (ApDung), read by package thuoctinh.
//
// What decides what (docs/architecture/03-ai-engine-hop-dong.md §8, «Luật
// không heuristic»): no word list reads a place's text. Allergens, diets,
// atmospheres and main dishes come from the enrichment model's structured
// output over the place's SafeDeep'd text, in closed id enums with
// «khong_ro» allowed. Go only checks structure (enum membership, lengths),
// applies the review policy, and applies ontology rules over ids
// (tuvung.DoiKieng: vegan implies vegetarian). Dedupe asks the embedding
// space and the map, never a string rule. Ranking inside Milvus (BM25 as a
// scoring function, dense cosine, RRF) is retrieval, not understanding.
//
// Writers: this package is the only writer of its tables (rag_dirty,
// rag_vector_versions, place_enrichments, rag_embedding_cache,
// rag_embed_batches, rag_trung, rag_ingest_dlq) and of every Milvus
// collection whose
// name starts with rd_. It never names a nep_* table or collection
// (aigate/rag_gate_test.go). It imports no engine package: the dense
// encoder, the vector store and the model arrive through the interfaces
// declared here (NhungTaiLieu, KhoVector, model.LLM from aiharness/llm).
package nap

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Corpus is what a collection indexes.
type Corpus string

const (
	// CorpusQuan is the place catalogue.
	CorpusQuan Corpus = "place"
	// CorpusSoTay is the app manual (huongdan).
	CorpusSoTay Corpus = "manual"
)

// Corpora is every corpus, in a stable order.
var Corpora = []Corpus{CorpusQuan, CorpusSoTay}

// KiemCorpus refuses an unknown corpus.
func KiemCorpus(c string) (Corpus, error) {
	for _, k := range Corpora {
		if string(k) == c {
			return k, nil
		}
	}
	return "", fmt.Errorf("%w: corpus %q", ErrCauHinh, c)
}

// Errors of the pipeline.
var (
	ErrCauHinh     = errors.New("nap: invalid configuration")
	ErrTrangThai   = errors.New("nap: the version is not in the state this step needs")
	ErrDoiSoat     = errors.New("nap: reconciliation failed")
	ErrCong        = errors.New("nap: the evaluation gate refused the version")
	ErrAlias       = errors.New("nap: the alias does not point where Postgres says")
	ErrKhongCoCha  = errors.New("nap: the active version has no version to roll back to")
	ErrKhongActive = errors.New("nap: no active version")
	ErrTenTrung    = errors.New("nap: a physical collection carries an alias name")
	ErrVector      = errors.New("nap: an embedding came back malformed")
)

// Alias is the serving name of a corpus: what retrieval reads (vectordb's
// KhoDiaDiem and KhoHuongDan aliases; napkho checks they agree). Milvus looks
// a name up as a collection before it looks it up as an alias (research
// sdlc-production §B1), so no physical collection may ever carry it.
func Alias(c Corpus) string {
	if c == CorpusQuan {
		return "rd_places"
	}
	return "rd_manual"
}

// AliasShadow is the alias a candidate is shadowed under.
func AliasShadow(c Corpus) string { return Alias(c) + "_shadow" }

// TenCollection is the physical collection of version id: «alias__vN», the
// same scheme as vectordb.TenVatLy and rag_vector_versions.milvus_collection.
func TenCollection(c Corpus, id int64) string {
	return Alias(c) + "__v" + strconv.FormatInt(id, 10)
}

var tenVatLy = regexp.MustCompile(`^rd_(places|manual)__v[1-9][0-9]*$`)

// LaTenVatLy reports whether name is a physical collection of this package.
func LaTenVatLy(name string) bool { return tenVatLy.MatchString(name) }

// KiemTenCollection refuses a physical collection name equal to any alias,
// and any name outside this package's pattern.
func KiemTenCollection(name string) error {
	for _, c := range Corpora {
		if name == Alias(c) || name == AliasShadow(c) {
			return fmt.Errorf("%w: %s", ErrTenTrung, name)
		}
	}
	if !LaTenVatLy(name) {
		return fmt.Errorf("%w: %s is not rd_(places|manual)__v<N>", ErrTenTrung, name)
	}
	return nil
}

// KiemKhongTrungAlias lists the names among collections that equal an alias.
// Retrieval would silently read that collection and ignore every alias swap.
func KiemKhongTrungAlias(collections []string) []string {
	var out []string
	for _, n := range collections {
		for _, c := range Corpora {
			if n == Alias(c) || n == AliasShadow(c) {
				out = append(out, n)
			}
		}
	}
	return out
}

// dongTrang trims and drops empty strings.
func dongTrang(xs []string) []string {
	var out []string
	for _, x := range xs {
		if x = strings.TrimSpace(x); x != "" {
			out = append(out, x)
		}
	}
	return out
}
