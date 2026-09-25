package aigate

import (
	"testing"

	"golang.org/x/tools/go/packages"
)

// The eval kit never reaches the production binary (design 06 §1): cmd/core
// does not link internal/aieval, whoever would import it. The eval's recording
// Sink, its canary markers and its scripted model belong to cmd/rudi-eval,
// which only the `eval` build tag builds and the core image never does.
// scripts/eval_kich_ban.sh checks the same with `go list -deps`; this is the
// half that runs in plain `go test ./...`.
func TestCoreBinaryDoesNotLinkEval(t *testing.T) {
	const aieval = module + "/internal/aieval"
	core := doThiPhuThuoc(t, nil, module+"/cmd/core")
	if !core[module+"/internal/aiharness"] {
		t.Fatalf("aiharness không có trong đồ thị của core; phép duyệt hỏng (%d gói)", len(core))
	}
	if core[aieval] {
		t.Fatalf("cmd/core liên kết %s", aieval)
	}
	// Canary: the same walk over the eval binary, built with its tag, does
	// see the package -- so the absence above is a finding, not blindness.
	eval := doThiPhuThuoc(t, []string{"-tags=eval"}, module+"/cmd/rudi-eval")
	if !eval[aieval] {
		t.Fatalf("không thấy %s trong đồ thị của cmd/rudi-eval: phép duyệt mù (%d gói)", aieval, len(eval))
	}
}

// doThiPhuThuoc is every package pattern links, built with flags.
func doThiPhuThuoc(t *testing.T, flags []string, pattern string) map[string]bool {
	t.Helper()
	pkgs, err := packages.Load(&packages.Config{Mode: packages.NeedName | packages.NeedImports | packages.NeedDeps, Dir: "../..", BuildFlags: flags}, pattern)
	if err != nil {
		t.Fatal(err)
	}
	if packages.PrintErrors(pkgs) > 0 {
		t.Fatalf("nạp %s lỗi", pattern)
	}
	seen := map[string]bool{}
	var walk func(p *packages.Package)
	walk = func(p *packages.Package) {
		if seen[p.PkgPath] {
			return
		}
		seen[p.PkgPath] = true
		for _, imp := range p.Imports {
			walk(imp)
		}
	}
	for _, p := range pkgs {
		walk(p)
	}
	return seen
}
