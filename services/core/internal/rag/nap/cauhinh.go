package nap

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
)

// cauHinhJSON is the pipeline's configuration, committed in the repo: every
// number a build depends on is a reviewed diff, not an environment value
// (research sdlc-production §E1). Its sha256 is part of every version's
// fingerprint.
//
//go:embed cauhinh.json
var cauHinhJSON []byte

// CheDoThua is how the sparse leg is produced.
type CheDoThua string

const (
	// ThuaBM25 is Milvus's BM25 function over the chunk text, twice: once
	// with diacritics kept, once folded (vectordb's two analyzers): a
	// retrieval scoring function computed by Milvus, no client vector.
	ThuaBM25 CheDoThua = "bm25"
	// ThuaMILCO is a learned sparse vector from the MILCO encoder. Its
	// licence is unconfirmed (research milco.md, Kiểm chứng #11): it stays
	// off until a person has read the model cards and set milco_bat.
	ThuaMILCO CheDoThua = "milco"
)

// CauHinh is cauhinh.json.
type CauHinh struct {
	PhienBan string `json:"phien_ban"`
	Dense    struct {
		Model string `json:"model"`
		Dims  int    `json:"dims"`
		// CoChe is how the task reaches the model: «prefix-v1» writes it into
		// the text (Gemini Developer API, research gemini-embedding-2 Kiểm
		// chứng #2); it is part of the cache key.
		CoChe string `json:"co_che"`
		Lo    int    `json:"lo"`
	} `json:"dense"`
	Thua struct {
		CheDo      CheDoThua `json:"che_do"`
		MilcoBat   bool      `json:"milco_bat"`
		MilcoModel string    `json:"milco_model"`
		MilcoRev   string    `json:"milco_rev"`
		PruneK     int       `json:"prune_k"`
	} `json:"thua"`
	// LuocDo names the revision of vectordb's collection schema (fields,
	// analyzers, index parameters) this configuration was written for; the
	// schema itself is declared once, in vectordb, and napkho refuses to
	// build a collection when the two differ.
	LuocDo  string            `json:"luoc_do"`
	Chunker map[Corpus]string `json:"chunker"`
	LamGiau struct {
		Model    string `json:"model"`
		Lo       int    `json:"lo"`
		SongSong int    `json:"song_song"`
		HanGiay  int    `json:"han_giay"`
	} `json:"lam_giau"`
	Trung struct {
		CosineToiThieu float64 `json:"cosine_toi_thieu"`
		KhoangCachM    float64 `json:"khoang_cach_m"`
	} `json:"trung"`
	// Hop is the fusion: the RRF constant (vectordb's, checked by napkho)
	// and the legs' weights. Retrieval serves with these same weights
	// (cmd/core wires them into hybrid), so the gate measures what serves.
	Hop struct {
		RRFK    int     `json:"rrf_k"`
		TrongSo TrongSo `json:"trong_so"`
	} `json:"hop"`
	Cong   NguongCong `json:"cong"`
	TuDong struct {
		TyLeDoiToiDa float64 `json:"ty_le_doi_toi_da"`
	} `json:"tu_dong"`
	GiuBan struct {
		ToiDa int `json:"toi_da"`
		Ngay  int `json:"ngay"`
	} `json:"giu_ban"`
}

// NguongCong are the promotion gate's thresholds (research sdlc-production
// §C4; design 04 §8.3). Violation has no tolerance.
type NguongCong struct {
	Recall10       float64 `json:"recall_10"`
	NDCG10         float64 `json:"ndcg_10"`
	MRR10          float64 `json:"mrr_10"`
	Violation10    float64 `json:"violation_10"`
	KhongDauGap    float64 `json:"khong_dau_gap"`
	KhongKemActive float64 `json:"khong_kem_active"`
}

// MacDinh returns the committed configuration, checked.
func MacDinh() (CauHinh, error) { return DocCauHinh(cauHinhJSON) }

// DocCauHinh reads a configuration strictly: unknown keys, a missing number,
// MILCO chosen without its licence flag, or a violation tolerance above zero
// are refused.
func DocCauHinh(raw []byte) (CauHinh, error) {
	var c CauHinh
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&c); err != nil || dec.More() {
		return CauHinh{}, fmt.Errorf("%w: %v", ErrCauHinh, err)
	}
	return c, c.Kiem()
}

// Kiem checks the configuration's structure.
func (c CauHinh) Kiem() error {
	bad := func(what string) error { return fmt.Errorf("%w: %s", ErrCauHinh, what) }
	switch {
	case c.PhienBan == "":
		return bad("phien_ban")
	case c.Dense.Model == "" || c.Dense.CoChe == "":
		return bad("dense model")
	case c.Dense.Dims <= 0 || c.Dense.Dims > 32768:
		return bad("dense dims")
	case c.Dense.Lo <= 0 || c.Dense.Lo > 100:
		return bad("dense batch")
	case c.Chunker[CorpusQuan] == "" || c.Chunker[CorpusSoTay] == "":
		return bad("chunker")
	case c.LamGiau.Model == "" || c.LamGiau.Lo <= 0 || c.LamGiau.Lo > 20 || c.LamGiau.SongSong <= 0 || c.LamGiau.HanGiay <= 0:
		return bad("lam_giau")
	case c.Trung.CosineToiThieu <= 0 || c.Trung.CosineToiThieu > 1 || c.Trung.KhoangCachM <= 0 || c.Trung.KhoangCachM > 80:
		return bad("trung: cosine in (0,1], distance in (0,80] m")
	case c.LuocDo == "":
		return bad("luoc_do")
	case c.Hop.RRFK <= 0:
		return bad("hop rrf_k")
	case !trongSoHopLe(c.Hop.TrongSo):
		return bad("hop trong_so: weights are finite, non-negative, and not all zero")
	case c.Cong.Violation10 != 0:
		return bad("violation@10 has no tolerance")
	case c.Cong.Recall10 <= 0 || c.Cong.NDCG10 <= 0 || c.Cong.MRR10 <= 0 || c.Cong.KhongDauGap < 0 || c.Cong.KhongKemActive < 0:
		return bad("cong")
	case c.TuDong.TyLeDoiToiDa < 0 || c.TuDong.TyLeDoiToiDa > 0.05:
		return bad("tu_dong: auto-promotion is capped at 5% changed documents")
	case c.GiuBan.ToiDa < 2 || c.GiuBan.Ngay <= 0:
		return bad("giu_ban")
	}
	switch c.Thua.CheDo {
	case ThuaBM25:
	case ThuaMILCO:
		if !c.Thua.MilcoBat {
			return bad("MILCO chosen while milco_bat is false (licence unconfirmed)")
		}
		if c.Thua.MilcoModel == "" || c.Thua.MilcoRev == "" || c.Thua.PruneK <= 0 {
			return bad("MILCO needs a model, a pinned revision and prune_k")
		}
	default:
		return bad("thua che_do")
	}
	if math.IsNaN(c.Trung.CosineToiThieu) {
		return bad("trung")
	}
	return nil
}

// VanTay is the configuration's fingerprint: 12 hex of the sha256 of its
// canonical JSON. A version records it, and auto-promotion requires the
// parent to carry the same one.
func (c CauHinh) VanTay() string {
	raw, _ := json.Marshal(c)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])[:12]
}

// SparseRev names the sparse leg as a version records it: the analyzer for
// BM25, the pinned model revision and prune for MILCO.
func (c CauHinh) SparseRev() string {
	if c.Thua.CheDo == ThuaMILCO {
		return fmt.Sprintf("milco:%s@%s:k%d", c.Thua.MilcoModel, c.Thua.MilcoRev, c.Thua.PruneK)
	}
	return "bm25:" + c.LuocDo
}

func trongSoHopLe(w TrongSo) bool {
	sum := 0.0
	for _, x := range []float64{w.Dense, w.BM25, w.BM25KhongDau, w.MILCO} {
		if !(x >= 0) || math.IsInf(x, 0) {
			return false
		}
		sum += x
	}
	return sum > 0
}
