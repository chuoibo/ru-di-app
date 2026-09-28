package aiharness

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The engine's boundary (design 01 §7): the engine reads and writes no
// database. Only aiharness/metrics, the one writer of ai_turn_metrics, holds
// SQL or a database driver; nothing else in aiharness imports pgx, the
// repository, the db package or chatassist (which imports the engine, never
// the other way), and the engine proper does not import metrics -- the worker
// writes the row, the engine only fills it in.
var (
	camMoiNoi = []string{"github.com/jackc/pgx", "mobile/services/core/internal/repo", "mobile/services/core/internal/db", "mobile/services/core/internal/chatassist", "mobile/services/core/internal/service", "mobile/services/core/internal/brain"}
	sqlChu    = regexp.MustCompile(`(?is)\b(select\b.+\bfrom|insert\s+into|update\s+\w+\s+set|delete\s+from|create\s+table)\b`)
)

func TestRanhGioiEngine(t *testing.T) {
	fset := token.NewFileSet()
	files := 0
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == "testdata" {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		files++
		laMetrics := strings.HasPrefix(filepath.ToSlash(path), "metrics/")
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			p, _ := strconv.Unquote(imp.Path.Value)
			for _, cam := range camMoiNoi {
				if (p == cam || strings.HasPrefix(p, cam+"/")) && !(laMetrics && strings.HasPrefix(p, "github.com/jackc/pgx")) {
					t.Errorf("%s imports %s", path, p)
				}
			}
			if p == "mobile/services/core/internal/aiharness/metrics" {
				t.Errorf("%s imports the metrics writer: the engine does not write", path)
			}
		}
		if laMetrics {
			return nil
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				if s, err := strconv.Unquote(lit.Value); err == nil && sqlChu.MatchString(s) {
					t.Errorf("%s holds SQL: %q", path, s)
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if files < 10 {
		t.Fatalf("only %d files scanned", files)
	}
	// Canary: the SQL pattern sees SQL, and not the words of a prompt.
	if !sqlChu.MatchString("SELECT body FROM messages") || !sqlChu.MatchString("insert into ai_turn_metrics(x)") || sqlChu.MatchString("Never create, change, split, settle or remind about money") {
		t.Fatal("the SQL pattern is broken")
	}
}
