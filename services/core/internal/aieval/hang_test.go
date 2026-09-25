package aieval

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"mobile/services/core/internal/aiharness"
	"mobile/services/core/internal/aiharness/cau"
	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/obs"
	"mobile/services/core/internal/aiharness/prompts"
)

// The constants are the engine's own, and an empty tool list reads as [],
// not null, to whoever parses it.
func TestHang(t *testing.T) {
	h := DocHang()
	if h.MoHinh != llm.Model || h.MaxModelCallsPerTurn != llm.MaxModelCallsPerTurn || h.NepMaxChu != aiharness.NepMaxChu || h.PromptVersionNep != prompts.VersionNep() {
		t.Fatalf("hằng: %+v", h)
	}
	if len(h.Ma) != len(cau.Tat()) || len(h.TrangThai) != 2 || h.TrangThai[0] != string(cau.DangDoc) {
		t.Fatalf("tập đóng: %v %v", h.Ma, h.TrangThai)
	}
	raw, _ := json.Marshal(h)
	if !strings.Contains(string(raw), `"cong_cu":{"nep":[]}`) {
		t.Fatalf("công cụ: %s", raw)
	}
	if _, chay := CongCuDuocPhep(obs.BotNhom); chay {
		t.Fatal("bot nhóm chưa lên engine ở lát này")
	}
}

// The seed is the turn the worker would build, with a repeatable id and
// marker in the engine's own shapes.
func TestGieo(t *testing.T) {
	b, _, _ := napBo(t)
	var c Ca
	for _, x := range b.Ca {
		if x.CaID == CaDongNhat {
			c = x
		}
	}
	g1, err := GieoCa(c, 1)
	if err != nil {
		t.Fatal(err)
	}
	g1b, _ := GieoCa(c, 1)
	g2, _ := GieoCa(c, 2)
	if !obs.ID(g1.Turn.InvocationID).Valid() || g1.Turn.InvocationID != g1b.Turn.InvocationID || g1.Turn.InvocationID == g2.Turn.InvocationID {
		t.Fatalf("id: %s %s %s", g1.Turn.InvocationID, g1b.Turn.InvocationID, g2.Turn.InvocationID)
	}
	if !regexp.MustCompile(`^[0-9a-f]{12}$`).MatchString(g1.MaKiem) || g1.MaKiem != g2.MaKiem {
		t.Fatalf("mã kiểm: %s", g1.MaKiem)
	}
	muon := time.Date(2026, 9, 25, 7, 5, 0, 0, time.UTC)
	if !g1.Turn.Luc.Equal(muon) || g1.Turn.Luc.Location() != time.UTC {
		t.Fatalf("Luc: %v", g1.Turn.Luc)
	}
	tr := g1.Turn
	if tr.Bot != obs.BotNep || tr.Lenh != obs.LenhHoi || tr.LanThu != 1 || tr.LoiNho != c.DauVao.LoiNho || tr.PhieuNep == nil ||
		tr.PhieuNep.TieuDe != c.DauVao.Phieu.TieuDe || tr.PhieuNep.Nhip == nil || *tr.PhieuNep.Nhip.ConNgay != 2 || len(tr.LuotNep) != 2 || tr.GiuLuot != nil {
		t.Fatalf("turn: %+v", tr)
	}
	if _, err := GieoCa(c, 0); err == nil {
		t.Fatal("lap 0 được nhận")
	}
}

// The recorder keeps every event in order, timed by its clock.
func TestGhiLai(t *testing.T) {
	t0 := time.Date(2026, 9, 25, 7, 5, 0, 0, time.UTC)
	now := t0
	g := NewGhiLai(func() time.Time { return now })
	g.TrangThai(cau.DangDoc, 0)
	now = now.Add(15 * time.Millisecond)
	g.Phan(0, aiharness.PhanText, json.RawMessage(`{"a":1}`))
	g.Delta(0, "chữ")
	g.LamLai()
	ds := g.SuKien()
	var nhan []string
	for _, d := range ds {
		nhan = append(nhan, d.Nhan())
	}
	if strings.Join(nhan, ",") != "trang_thai:dang_doc:0,phan:text,delta,lam_lai" || ds[0].Ms != 0 || ds[1].Ms != 15 || ds[2].Chu != "chữ" || string(ds[1].JSON) != `{"a":1}` {
		t.Fatalf("%+v", ds)
	}
}

// Invariant 10, on the source: the eval package and the eval binary hold no
// way to build a model client or read a key. Nothing here calls the genai or
// ADK constructors or the engine's env constructors, names the key
// variables, or reads the environment at all; so `kich-ban` cannot open a
// connection whatever GEMINI_API_KEY holds. Slice 18's `that` mode will need
// a named exception here, reviewed with it.
func TestKhongDungClientGenai(t *testing.T) {
	fset := token.NewFileSet()
	var loi []string
	quet := map[string]int{}
	for _, dir := range []string{".", filepath.Join("..", "..", "cmd", "rudi-eval")} {
		paths, _ := filepath.Glob(filepath.Join(dir, "*.go"))
		for _, p := range paths {
			if strings.HasSuffix(p, "_test.go") {
				continue
			}
			src, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			loi = append(loi, dungClient(fset, p, src)...)
			quet[dir]++
		}
	}
	if quet["."] < 5 || quet[filepath.Join("..", "..", "cmd", "rudi-eval")] < 1 {
		t.Fatalf("quét quá ít file: %v", quet)
	}
	for _, l := range loi {
		t.Error(l)
	}
	// Canary: the scan is red on each way in.
	for name, src := range map[string]string{
		"env.go":    "package x\nimport \"mobile/services/core/internal/aiharness/llm\"\nfunc f() { llm.GeminiFromEnv(nil, nil) }\n",
		"new.go":    "package x\nimport \"mobile/services/core/internal/aiharness/llm\"\nfunc f() { llm.NewGemini(nil, \"\", \"\") }\n",
		"engine.go": "package x\nimport \"mobile/services/core/internal/aiharness\"\nfunc f() { aiharness.FromEnv(nil, nil, nil) }\n",
		"genai.go":  "package x\nimport g \"google.golang.org/genai\"\nfunc f() { g.NewClient(nil, nil) }\n",
		"adk.go":    "package x\nimport \"google.golang.org/adk/model/gemini\"\n",
		"getenv.go": "package x\nimport \"os\"\nfunc f() { _ = os.Getenv(\"X\") }\n",
		"key.go":    "package x\nconst k = \"GEMINI_API_KEY\"\n",
		"keyref.go": "package x\nimport \"mobile/services/core/internal/aiharness/llm\"\nvar k = llm.EnvAPIKey\n",
	} {
		if len(dungClient(fset, name, []byte(src))) == 0 {
			t.Errorf("%s: không bị bắt", name)
		}
	}
}

var (
	goiCam = map[string]map[string]bool{
		"mobile/services/core/internal/aiharness/llm": {"NewGemini": true, "GeminiFromEnv": true, "EnvAPIKey": true, "EnvBaseURL": true},
		"mobile/services/core/internal/aiharness":     {"FromEnv": true},
		"google.golang.org/genai":                     {"NewClient": true},
		"os":                                          {"Getenv": true, "LookupEnv": true, "Environ": true},
	}
	importCam = []string{"google.golang.org/adk/model/gemini"}
	chuoiCam  = regexp.MustCompile(`GEMINI_API_KEY|GOOGLE_API_KEY|MOBILE_GEMINI_BASE_URL|GOOGLE_GEMINI_BASE_URL`)
)

func dungClient(fset *token.FileSet, name string, src []byte) []string {
	f, err := parser.ParseFile(fset, name, src, parser.SkipObjectResolution)
	if err != nil {
		return []string{name + ": " + err.Error()}
	}
	var out []string
	local := map[string]string{}
	for _, imp := range f.Imports {
		p, _ := strconv.Unquote(imp.Path.Value)
		for _, c := range importCam {
			if p == c {
				out = append(out, name+" imports "+p)
			}
		}
		if goiCam[p] != nil {
			n := filepath.Base(p)
			if imp.Name != nil {
				n = imp.Name.Name
			}
			local[n] = p
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.SelectorExpr:
			if id, ok := v.X.(*ast.Ident); ok && local[id.Name] != "" && goiCam[local[id.Name]][v.Sel.Name] {
				out = append(out, name+" uses "+local[id.Name]+"."+v.Sel.Name)
			}
		case *ast.BasicLit:
			if v.Kind == token.STRING && chuoiCam.MatchString(v.Value) {
				out = append(out, name+" names "+v.Value)
			}
		}
		return true
	})
	return out
}
