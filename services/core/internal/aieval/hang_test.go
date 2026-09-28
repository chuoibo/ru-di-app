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
	// Nếp's tools are the permission table's, read from it: the memory and
	// own-outing tools, never a group tool, never set_reminder.
	raw, _ := json.Marshal(h)
	nep := strings.Join(h.CongCu["nep"], ",")
	if !strings.Contains(nep, "search_places") || !strings.Contains(nep, "what_you_remember") ||
		strings.Contains(nep, "group_snapshot") || strings.Contains(nep, "set_reminder") || len(h.CongCu) != 2 {
		t.Fatalf("công cụ: %s", raw)
	}
	// The group (slice 9): its own tools, never one of scope me.
	nhom, chay := CongCuDuocPhep(obs.BotNhom)
	s := strings.Join(nhom, ",")
	if !chay || !strings.Contains(s, "group_snapshot") || strings.Contains(s, "recall_memory") || strings.Contains(s, "explain_screen") {
		t.Fatalf("công cụ nhóm: %v", nhom)
	}
	if h.NhomMaxChu != aiharness.NhomMaxChu || h.PromptVersionNhom != prompts.VersionNhom() || h.PromptVersionDoi != prompts.VersionDoi() || h.PromptVersionDoi == h.PromptVersionNhom {
		t.Fatalf("hằng nhóm: %+v", h)
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
// way to build a model client or read a key, with ONE named exception:
// cmd/rudi-eval/nha_cung_cap.go, the provider door of `that` and `ghi`
// (design 06 §6.1, slice 18). Nothing else calls the genai, ADK, embedder or
// reranker constructors or the engine's env constructors, names the key
// variables, or reads the environment at all; and inside the exception the
// constructors are called only from dungNhaCungCap, which main reaches only
// for `that` and `ghi` (TestKichBanPhatLaiKhongDungClient counts it at run
// time). So `kich-ban` and `phat-lai` cannot open a connection whatever
// GEMINI_API_KEY holds.
func TestKhongDungClientGenai(t *testing.T) {
	fset := token.NewFileSet()
	var loi []string
	quet := map[string]int{}
	cmdDir := filepath.Join("..", "..", "cmd", "rudi-eval")
	ngoaiLe := filepath.Join(cmdDir, "nha_cung_cap.go")
	for _, dir := range []string{".", "giagemini", cmdDir} {
		paths, _ := filepath.Glob(filepath.Join(dir, "*.go"))
		for _, p := range paths {
			if strings.HasSuffix(p, "_test.go") || p == ngoaiLe {
				continue
			}
			src, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			if dir == "giagemini" {
				// The loopback stand-in builds no client and reads no
				// environment either; it may name nothing forbidden.
				loi = append(loi, dungClient(fset, p, src)...)
				continue
			}
			loi = append(loi, dungClient(fset, p, src)...)
			quet[dir]++
		}
	}
	if quet["."] < 5 || quet[cmdDir] < 2 {
		t.Fatalf("quét quá ít file: %v", quet)
	}
	for _, l := range loi {
		t.Error(l)
	}
	// The exception: every forbidden use sits inside dungNhaCungCap or
	// coPhuTuMoiTruong (the estimate reads whether a reranker is set) or is
	// the file's getenv variable.
	src, err := os.ReadFile(ngoaiLe)
	if err != nil {
		t.Fatalf("ngoại lệ có tên phải tồn tại: %v", err)
	}
	f, err := parser.ParseFile(fset, ngoaiLe, src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range f.Decls {
		ten := ""
		switch v := d.(type) {
		case *ast.FuncDecl:
			ten = v.Name.Name
		case *ast.GenDecl:
			ten = "var/const"
		}
		start, end := fset.Position(d.Pos()).Offset, fset.Position(d.End()).Offset
		for _, l := range dungClient(fset, "x.go", append([]byte("package x\n"), vungImport(f, fset, src)+string(src[start:end])...)) {
			switch {
			case ten == "dungNhaCungCap", ten == "coPhuTuMoiTruong":
			case ten == "var/const" && (strings.Contains(l, "os.Getenv") || strings.Contains(l, "names")):
			default:
				t.Errorf("nha_cung_cap.go: %s ngoài dungNhaCungCap (%s)", l, ten)
			}
		}
	}
	// Canary: the scan is red on each way in.
	for name, src := range map[string]string{
		"env.go":    "package x\nimport \"mobile/services/core/internal/aiharness/llm\"\nfunc f() { llm.GeminiFromEnv(nil, nil) }\n",
		"new.go":    "package x\nimport \"mobile/services/core/internal/aiharness/llm\"\nfunc f() { llm.NewGemini(nil, \"\", \"\") }\n",
		"nhung.go":  "package x\nimport \"mobile/services/core/internal/aiharness/nhung\"\nfunc f() { nhung.NewGemini(nil, \"\", \"\") }\n",
		"rerank.go": "package x\nimport \"mobile/services/core/internal/rerank\"\nfunc f() { rerank.TuEnv(nil) }\n",
		"engine.go": "package x\nimport \"mobile/services/core/internal/aiharness\"\nfunc f() { aiharness.FromEnv(nil, nil, nil) }\n",
		"genai.go":  "package x\nimport g \"google.golang.org/genai\"\nfunc f() { g.NewClient(nil, nil) }\n",
		"adk.go":    "package x\nimport \"google.golang.org/adk/v2/model/gemini\"\n",
		"getenv.go": "package x\nimport \"os\"\nfunc f() { _ = os.Getenv(\"X\") }\n",
		"key.go":    "package x\nconst k = \"GEMINI_API_KEY\"\n",
		"keyref.go": "package x\nimport \"mobile/services/core/internal/aiharness/llm\"\nvar k = llm.EnvAPIKey\n",
	} {
		if len(dungClient(fset, name, []byte(src))) == 0 {
			t.Errorf("%s: không bị bắt", name)
		}
	}
}

// vungImport is f's import block, so one declaration can be scanned with
// the names it resolves.
func vungImport(f *ast.File, fset *token.FileSet, src []byte) string {
	var b strings.Builder
	for _, d := range f.Decls {
		if g, ok := d.(*ast.GenDecl); ok && g.Tok == token.IMPORT {
			b.Write(src[fset.Position(g.Pos()).Offset:fset.Position(g.End()).Offset])
			b.WriteString("\n")
		}
	}
	return b.String()
}

var (
	goiCam = map[string]map[string]bool{
		"mobile/services/core/internal/aiharness/llm":   {"NewGemini": true, "GeminiFromEnv": true, "EnvAPIKey": true, "EnvBaseURL": true},
		"mobile/services/core/internal/aiharness":       {"FromEnv": true},
		"mobile/services/core/internal/aiharness/nhung": {"NewGemini": true, "FromEnv": true},
		"mobile/services/core/internal/rerank":          {"TuEnv": true, "Moi": true, "EnvURL": true},
		"google.golang.org/genai":                       {"NewClient": true},
		"os":                                            {"Getenv": true, "LookupEnv": true, "Environ": true},
	}
	importCam = []string{"google.golang.org/adk/v2/model/gemini"}
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
