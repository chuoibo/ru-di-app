package nap

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"

	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/domain/promptsafety"
	"mobile/services/core/internal/domain/tuvung"
)

// Enrichment (S5): a quarantined model reads the SafeDeep'd text of up to
// cfg.LamGiau.Lo places per call and answers only closed ids, a few short
// dish names, its own injection label and a confidence enum
// (research sdlc-production §B4: dual-LLM pattern; no tools, enum output).
// Nothing here reads the text itself: the vocabularies below contribute
// their ids and labels to the prompt and to the response schema, never their
// phrase lists.

//go:embed lamgiau_prompt.txt
var lamGiauPrompt string

// Extractor names this extractor in place_enrichments.
const Extractor = "place-enrich"

// KhongRo is the enum value for «the text does not say».
const KhongRo = "khong_ro"

// Bounds of the enrichment output.
const (
	MaxMonChinh    = 5
	MaxRuneMon     = 40
	MaxKhiChatQuan = 4
	// MaxRuneNguCanh bounds a context line (contextual retrieval: one short
	// sentence per chunk, written by the offline enrichment call, prepended
	// to the chunk's text for both the dense and the BM25 legs).
	MaxRuneNguCanh = 160
)

// TinCay is the model's confidence.
type TinCay string

const (
	TinCayCao  TinCay = "cao"
	TinCayVua  TinCay = "vua"
	TinCayThap TinCay = "thap"
)

var tinCays = []string{string(TinCayCao), string(TinCayVua), string(TinCayThap)}

// Review states of an enrichment (design 04 §3).
const (
	ReviewAuto     = "auto"
	ReviewReviewed = "reviewed"
	ReviewRejected = "rejected"
)

// KetQuaLamGiau is one place's checked enrichment output.
type KetQuaLamGiau struct {
	DiUng   []string `json:"di_ung"`
	DiUngRo bool     `json:"di_ung_ro"`
	// AnKieng is what the model says the place declares; it reaches a hard
	// filter only once a person has reviewed it (design 04 §6.3).
	AnKieng   []string `json:"an_kieng"`
	AnKiengRo bool     `json:"an_kieng_ro"`
	KhiChat   []string `json:"khi_chat"`
	MonChinh  []string `json:"mon_chinh"`
	// ChenLenh is the model's own label: the third-party text tried to
	// instruct it. The place's output is then quarantined (see ApDung).
	ChenLenh bool   `json:"chen_lenh"`
	TinCay   TinCay `json:"tin_cay"`
	// MonBo counts dish strings refused by TextSafe or the length bound.
	MonBo int `json:"mon_bo"`
	// NguCanhHoSo, NguCanhTraiNghiem and NguCanhMonAn are the context lines
	// the prompt still asks for (rd.v3's contextual retrieval). rd.v4 embeds
	// one row per place with no context line (owner 2026-09-29), so nothing
	// reads them; they stay in the answer's shape because the stored
	// enrichments carry them and are not re-run.
	NguCanhHoSo       string `json:"ngu_canh_ho_so,omitempty"`
	NguCanhTraiNghiem string `json:"ngu_canh_trai_nghiem,omitempty"`
	NguCanhMonAn      string `json:"ngu_canh_mon_an,omitempty"`
	// NguCanhBo counts context lines refused by TextSafe or the bound.
	NguCanhBo int `json:"ngu_canh_bo,omitempty"`
}

// LamGiau is one row of place_enrichments.
type LamGiau struct {
	PlaceID       string
	NguonHash     [32]byte
	Model         string
	PromptVersion string
	KetQua        KetQuaLamGiau
	CanDuyet      bool
	Review        string
	// Ngoai marks an enrichment vnlocal produced (ingest's place_lam_giau,
	// lam-giau@1): vnlocal re-runs a place whenever its input changes, so
	// the row is current by construction and carries no RuDi source hash.
	// Never reviewed here: its allergen-free claim is never certain.
	Ngoai bool
}

// HuongDanLamGiau is the rendered system instruction: the committed prompt
// plus the closed code lists with their labels.
func HuongDanLamGiau() string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(lamGiauPrompt))
	list := func(title string, v *tuvung.TuVung) {
		b.WriteString("\n\n" + title + ":")
		for _, m := range v.Muc() {
			b.WriteString("\n- " + m.ID + ": " + m.Nhan)
		}
	}
	list("Mã di_ung", tuvung.DiUng)
	list("Mã an_kieng", tuvung.AnKieng)
	list("Mã khi_chat", tuvung.KhiChat)
	return b.String()
}

// PromptVersion is 12 hex of the sha256 of the rendered instruction and the
// response schema: a change to either is a new prompt version, and every
// enrichment row records the one it came from.
func PromptVersion() string {
	schema, _ := json.Marshal(LuocDoLamGiau(2))
	sum := sha256.Sum256(append([]byte(HuongDanLamGiau()+"\x00"), schema...))
	return hex.EncodeToString(sum[:])[:12]
}

func biDanh(i int) string { return "p" + strconv.Itoa(i+1) }

func i64(n int64) *int64 { return &n }

// LuocDoLamGiau is the response schema for a batch of n places, the one
// source of truth for the output's shape.
func LuocDoLamGiau(n int) *genai.Schema {
	aliases := make([]string, n)
	for i := range aliases {
		aliases[i] = biDanh(i)
	}
	enumArr := func(ids []string, max int) *genai.Schema {
		return &genai.Schema{Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString, Enum: ids}, MaxItems: i64(int64(max))}
	}
	diUng := append(append([]string(nil), tuvung.DiUng.IDs()...), KhongRo)
	anKieng := append(append([]string(nil), tuvung.AnKieng.IDs()...), KhongRo)
	item := &genai.Schema{
		Type: genai.TypeObject,
		Properties: map[string]*genai.Schema{
			"bi_danh":              {Type: genai.TypeString, Enum: aliases},
			"di_ung":               enumArr(diUng, len(diUng)),
			"an_kieng":             enumArr(anKieng, len(anKieng)),
			"khi_chat":             enumArr(tuvung.KhiChat.IDs(), MaxKhiChatQuan),
			"mon_chinh":            {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString, MaxLength: i64(MaxRuneMon)}, MaxItems: i64(MaxMonChinh)},
			"chen_lenh":            {Type: genai.TypeBoolean},
			"tin_cay":              {Type: genai.TypeString, Enum: tinCays},
			"ngu_canh_ho_so":       {Type: genai.TypeString, MaxLength: i64(MaxRuneNguCanh)},
			"ngu_canh_trai_nghiem": {Type: genai.TypeString, MaxLength: i64(MaxRuneNguCanh)},
			"ngu_canh_mon_an":      {Type: genai.TypeString, MaxLength: i64(MaxRuneNguCanh)},
		},
		PropertyOrdering: []string{"bi_danh", "di_ung", "an_kieng", "khi_chat", "mon_chinh", "chen_lenh", "tin_cay", "ngu_canh_ho_so", "ngu_canh_trai_nghiem", "ngu_canh_mon_an"},
		Required:         []string{"bi_danh", "di_ung", "an_kieng", "khi_chat", "mon_chinh", "chen_lenh", "tin_cay", "ngu_canh_ho_so", "ngu_canh_trai_nghiem", "ngu_canh_mon_an"},
	}
	return &genai.Schema{
		Type:       genai.TypeObject,
		Properties: map[string]*genai.Schema{"quan": {Type: genai.TypeArray, Items: item, MinItems: i64(int64(n)), MaxItems: i64(int64(n))}},
		Required:   []string{"quan"},
	}
}

var fullwidth = strings.NewReplacer("<", "＜", ">", "＞")

// BocQuan lays one place's safe text into its datamarked block: '<' and '>'
// become their fullwidth forms, so the data can neither close its block nor
// open a tag of ours (the same rule as aiharness/prompts.BocDuLieu, which
// package rag may not import).
func BocQuan(alias string, h HoSoQuan) string {
	body := h.HoSo
	if h.TraiNghiem != "" {
		body += "\nTrải nghiệm:\n" + h.TraiNghiem
	}
	if h.MonAn != "" {
		body += "\nMón ăn:\n" + h.MonAn
	}
	return `<du_lieu nguon="quan" bi_danh="` + alias + `">` + "\n" + fullwidth.Replace(body) + "\n</du_lieu>"
}

// YeuCauLamGiau builds the request for one batch.
func YeuCauLamGiau(batch []HoSoQuan) *model.LLMRequest {
	var parts []string
	for i, h := range batch {
		parts = append(parts, BocQuan(biDanh(i), h))
	}
	zero := float32(0)
	return &model.LLMRequest{
		Model:    llm.Model,
		Contents: []*genai.Content{genai.NewContentFromText(strings.Join(parts, "\n\n"), genai.RoleUser)},
		Config: &genai.GenerateContentConfig{
			SystemInstruction: genai.NewContentFromText(HuongDanLamGiau(), genai.RoleUser),
			Temperature:       &zero,
			ResponseMIMEType:  "application/json",
			ResponseSchema:    LuocDoLamGiau(len(batch)),
		},
	}
}

// ErrCauTrucLamGiau wraps every structural refusal of a model answer.
var ErrCauTrucLamGiau = errors.New("nap: enrichment output refused")

type thoMuc struct {
	BiDanh   *string  `json:"bi_danh"`
	DiUng    []string `json:"di_ung"`
	AnKieng  []string `json:"an_kieng"`
	KhiChat  []string `json:"khi_chat"`
	MonChinh []string `json:"mon_chinh"`
	ChenLenh *bool    `json:"chen_lenh"`
	TinCay   *string  `json:"tin_cay"`
	NguCanhH *string  `json:"ngu_canh_ho_so"`
	NguCanhT *string  `json:"ngu_canh_trai_nghiem"`
	NguCanhM *string  `json:"ngu_canh_mon_an"`
}

// DocTraLoiTungQuan reads a batch answer strictly, place by place: unknown
// keys, a missing field, an alias missing, repeated or unknown, or a count
// that is not the batch's refuse the whole answer (err); a place whose own
// item breaks a rule (a value outside its enum, a repeat inside a list,
// khong_ro beside another id, too many atmospheres or dishes) is refused
// alone (loi[i]), and the rest of the batch stands. Measured 2026-09-29 on
// the real catalogue: refusing the whole batch for one item lost 90 of 503
// batches (1,800 places).
func DocTraLoiTungQuan(raw []byte, n int) ([]KetQuaLamGiau, []error, error) {
	var t struct {
		Quan []thoMuc `json:"quan"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&t); err != nil || dec.More() {
		return nil, nil, fmt.Errorf("%w: json: %v", ErrCauTrucLamGiau, err)
	}
	if len(t.Quan) != n {
		return nil, nil, fmt.Errorf("%w: %d items for %d places", ErrCauTrucLamGiau, len(t.Quan), n)
	}
	out := make([]KetQuaLamGiau, n)
	loi := make([]error, n)
	seen := make([]bool, n)
	for _, m := range t.Quan {
		if m.BiDanh == nil || m.DiUng == nil || m.AnKieng == nil || m.KhiChat == nil || m.MonChinh == nil || m.ChenLenh == nil || m.TinCay == nil ||
			m.NguCanhH == nil || m.NguCanhT == nil || m.NguCanhM == nil {
			return nil, nil, fmt.Errorf("%w: a required field is missing", ErrCauTrucLamGiau)
		}
		i := -1
		for j := 0; j < n; j++ {
			if *m.BiDanh == biDanh(j) {
				i = j
			}
		}
		if i < 0 || seen[i] {
			return nil, nil, fmt.Errorf("%w: alias %q unknown or repeated", ErrCauTrucLamGiau, *m.BiDanh)
		}
		seen[i] = true
		out[i], loi[i] = docMuc(m)
	}
	return out, loi, nil
}

// DocTraLoi reads a batch answer strictly: unknown keys, a missing field,
// an alias missing, repeated or unknown, a value outside its enum, a repeat
// inside a list, or khong_ro beside another id refuse the whole answer. Dish
// names failing TextSafe or the length bound are dropped and counted.
func DocTraLoi(raw []byte, n int) ([]KetQuaLamGiau, error) {
	out, loi, err := DocTraLoiTungQuan(raw, n)
	if err != nil {
		return nil, err
	}
	for _, e := range loi {
		if e != nil {
			return nil, e
		}
	}
	return out, nil
}

// docMuc checks one place's item.
func docMuc(m thoMuc) (KetQuaLamGiau, error) {
	var k KetQuaLamGiau
	var err error
	if k.DiUng, k.DiUngRo, err = docDanhSachRo(m.DiUng, tuvung.DiUng); err != nil {
		return KetQuaLamGiau{}, err
	}
	if k.AnKieng, k.AnKiengRo, err = docDanhSachRo(m.AnKieng, tuvung.AnKieng); err != nil {
		return KetQuaLamGiau{}, err
	}
	if len(m.KhiChat) > MaxKhiChatQuan {
		return KetQuaLamGiau{}, fmt.Errorf("%w: too many khi_chat", ErrCauTrucLamGiau)
	}
	if k.KhiChat, err = docDanhSach(m.KhiChat, tuvung.KhiChat); err != nil {
		return KetQuaLamGiau{}, err
	}
	if len(m.MonChinh) > MaxMonChinh {
		return KetQuaLamGiau{}, fmt.Errorf("%w: too many mon_chinh", ErrCauTrucLamGiau)
	}
	for _, s := range m.MonChinh {
		s = nfc(strings.TrimSpace(s))
		if s == "" || utf8.RuneCountInString(s) > MaxRuneMon || !promptsafety.TextSafe(s, MaxRuneMon) {
			k.MonBo++
			continue
		}
		k.MonChinh = append(k.MonChinh, s)
	}
	for _, x := range []struct {
		in  string
		out *string
	}{{*m.NguCanhH, &k.NguCanhHoSo}, {*m.NguCanhT, &k.NguCanhTraiNghiem}, {*m.NguCanhM, &k.NguCanhMonAn}} {
		line := nfc(strings.Join(strings.Fields(x.in), " "))
		switch {
		case line == "":
		case utf8.RuneCountInString(line) > MaxRuneNguCanh || !promptsafety.TextSafe(line, MaxRuneNguCanh):
			k.NguCanhBo++
		default:
			*x.out = line
		}
	}
	k.ChenLenh = *m.ChenLenh
	switch TinCay(*m.TinCay) {
	case TinCayCao, TinCayVua, TinCayThap:
		k.TinCay = TinCay(*m.TinCay)
	default:
		return KetQuaLamGiau{}, fmt.Errorf("%w: tin_cay %q", ErrCauTrucLamGiau, *m.TinCay)
	}
	return k, nil
}

func docDanhSach(xs []string, v *tuvung.TuVung) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	for _, x := range xs {
		if !v.Co(x) || seen[x] {
			return nil, fmt.Errorf("%w: %s value %q unknown or repeated", ErrCauTrucLamGiau, v.Ten(), x)
		}
		seen[x] = true
		out = append(out, x)
	}
	return out, nil
}

func docDanhSachRo(xs []string, v *tuvung.TuVung) ([]string, bool, error) {
	for _, x := range xs {
		if x == KhongRo {
			if len(xs) != 1 {
				return nil, false, fmt.Errorf("%w: khong_ro beside another %s id", ErrCauTrucLamGiau, v.Ten())
			}
			return nil, false, nil
		}
	}
	out, err := docDanhSach(xs, v)
	return out, true, err
}

// CanDuyet is the review policy (research sdlc-production §C2 S5r): seed and
// curated rows are always reviewed; so is every enrichment whose approval
// would move a hard filter -- any allergen assertion the model calls
// certain (a list, empty or not: «no allergen» is the permissive direction,
// and ApDung honours certainty only once reviewed), any diet -- and every
// one that is not highly confident or carries the model's injection label;
// and a deterministic ~5% sample of the rest, to estimate the extractor's
// precision. An enrichment that says the allergens are unknown and names no
// diet moves no hard filter whatever the verdict, so it is only sampled.
func CanDuyet(nguon string, h [32]byte, placeID string, k KetQuaLamGiau) bool {
	if nguon == "seed" || nguon == "curated" {
		return true
	}
	if k.DiUngRo || len(k.DiUng) > 0 || len(k.AnKieng) > 0 || k.TinCay != TinCayCao || k.ChenLenh {
		return true
	}
	sum := sha256.Sum256(append([]byte(placeID+"\x00"), h[:]...))
	return sum[0] < 13 // 13/256 ≈ 5.1%
}

// ThuocTinh are the retrieval attributes a place's enrichment yields.
type ThuocTinh struct {
	// DanhMuc are the place's categories as vnlocal classified them
	// (DocDanhMuc), set by the caller beside ApDung; empty until classified.
	DanhMuc []string
	// DiUng only ever adds exclusions. DiUngRo is true only when a current
	// enrichment a PERSON reviewed says what the place serves; otherwise
	// retrieval treats the place as possibly containing any allergen, and
	// it is out for anyone with an allergy (docs/architecture/03 §8.4).
	DiUng   []string
	DiUngRo bool
	// AnKieng reaches the hard filter: reviewed rows only, closed over
	// tuvung.DoiKieng. AnKiengGoiY only ranks.
	AnKieng     []string
	AnKiengGoiY []string
	KhiChat     []string
	MonChinh    []string
	// Co: a current enrichment was applied. Cu: only a stale one exists
	// (its allergens still exclude). CachLy: the injection label held
	// everything but its allergens back.
	Co, Cu, CachLy bool
}

// ApDung turns a place's stored enrichment (nil when none) into attributes,
// given the place's current source hash. The only direction a stale,
// rejected, missing or injection-flagged enrichment can move a place is
// toward being excluded: allergens it names are kept, allergen certainty is
// lost, and everything that could make a place match is dropped. The same
// holds for an enrichment nobody reviewed: its allergen certainty -- the
// claim that lets a place pass an allergy filter -- counts only once a
// person approved it; an automatic «di_ung: []» never does.
func ApDung(lg *LamGiau, nguonHash [32]byte) ThuocTinh {
	var t ThuocTinh
	if lg == nil {
		return t
	}
	if lg.Review == ReviewRejected {
		return t
	}
	t.DiUng = append([]string(nil), lg.KetQua.DiUng...)
	if !lg.Ngoai && lg.NguonHash != nguonHash {
		t.Cu = true
		return t
	}
	if lg.KetQua.ChenLenh && lg.Review != ReviewReviewed {
		t.CachLy = true
		return t
	}
	t.Co = true
	if lg.Review == ReviewReviewed {
		t.DiUngRo = lg.KetQua.DiUngRo
		t.AnKieng = tuvung.DoiKieng(lg.KetQua.AnKieng)
	} else {
		t.AnKiengGoiY = tuvung.DoiKieng(lg.KetQua.AnKieng)
	}
	t.KhiChat = append([]string(nil), lg.KetQua.KhiChat...)
	t.MonChinh = append([]string(nil), lg.KetQua.MonChinh...)
	return t
}

// BaoCaoLamGiau counts one enrichment run: no text, no ids.
type BaoCaoLamGiau struct {
	Quan     int `json:"quan"`
	SoGoi    int `json:"so_goi"`
	Xong     int `json:"xong"`
	Hong     int `json:"hong"`
	HetTran  int `json:"het_tran"`
	CanDuyet int `json:"can_duyet"`
	ChenLenh int `json:"chen_lenh"`
	MonBo    int `json:"mon_bo"`
}

// ChayLamGiau runs enrichment over places in batches, cfg.LamGiau.SongSong
// at once, each call bounded by cfg.LamGiau.HanGiay, every call and retry
// counted by one llm.Dem holding at most tranGoi calls: past it the rest
// are left for the next run (HetTran), never called. A batch whose answer
// is refused is reported in hong by place id; the stored enrichment, if
// any, stays.
func ChayLamGiau(ctx context.Context, m model.LLM, cfg CauHinh, tranGoi int, places []HoSoQuan) ([]LamGiau, map[string]error, BaoCaoLamGiau) {
	rep := BaoCaoLamGiau{Quan: len(places)}
	hong := map[string]error{}
	if len(places) == 0 {
		return nil, hong, rep
	}
	dem := llm.NewDem(m, tranGoi, nil)
	var batches [][]HoSoQuan
	for i := 0; i < len(places); i += cfg.LamGiau.Lo {
		batches = append(batches, places[i:min(i+cfg.LamGiau.Lo, len(places))])
	}
	version := PromptVersion()
	var mu sync.Mutex
	var out []LamGiau
	jobs := make(chan []HoSoQuan)
	var wg sync.WaitGroup
	for w := 0; w < cfg.LamGiau.SongSong; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for batch := range jobs {
				res, loi, err := motLo(ctx, dem, time.Duration(cfg.LamGiau.HanGiay)*time.Second, batch)
				mu.Lock()
				if err != nil {
					for _, h := range batch {
						hong[h.ID] = err
					}
					if errors.Is(err, llm.ErrHetNganSach) {
						rep.HetTran += len(batch)
					} else {
						rep.Hong += len(batch)
					}
					mu.Unlock()
					continue
				}
				for i, h := range batch {
					if loi[i] != nil {
						// This place alone: the rest of the batch stands.
						hong[h.ID] = loi[i]
						rep.Hong++
						continue
					}
					k := res[i]
					lg := LamGiau{PlaceID: h.ID, NguonHash: h.NguonHash, Model: m.Name(), PromptVersion: version, KetQua: k,
						CanDuyet: CanDuyet(h.Nguon, h.NguonHash, h.ID, k), Review: ReviewAuto}
					out = append(out, lg)
					rep.Xong++
					rep.MonBo += k.MonBo
					if lg.CanDuyet {
						rep.CanDuyet++
					}
					if k.ChenLenh {
						rep.ChenLenh++
					}
				}
				mu.Unlock()
			}
		}()
	}
	for _, b := range batches {
		jobs <- b
	}
	close(jobs)
	wg.Wait()
	rep.SoGoi = dem.SoGoi()
	sort.Slice(out, func(i, j int) bool { return out[i].PlaceID < out[j].PlaceID })
	return out, hong, rep
}

func motLo(ctx context.Context, m model.LLM, han time.Duration, batch []HoSoQuan) ([]KetQuaLamGiau, []error, error) {
	ctx, cancel := context.WithTimeout(ctx, han)
	defer cancel()
	var text strings.Builder
	for resp, err := range m.GenerateContent(ctx, YeuCauLamGiau(batch), false) {
		if err != nil {
			return nil, nil, err
		}
		if resp != nil && resp.Content != nil {
			for _, p := range resp.Content.Parts {
				if p != nil {
					text.WriteString(p.Text)
				}
			}
		}
	}
	return DocTraLoiTungQuan([]byte(text.String()), len(batch))
}
