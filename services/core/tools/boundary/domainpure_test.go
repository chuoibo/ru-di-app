// Package boundary holds structural tests over the core module's own source.
package boundary

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// domainAllowed is everything internal/domain may import: arithmetic, text and
// time values. It is the Go spelling of tests/test_import_boundary.py and of
// its reason: a balance stays recomputable from the ledger only while the
// domain cannot reach a database, the network, the file system or the process
// environment. Reading the clock is a separate rule for the money linter
// (ADR-0029 §2.5); importing time for its value types is allowed here.
var domainAllowed = map[string]bool{
	"bytes": true, "cmp": true, "errors": true, "fmt": true, "maps": true,
	"math": true, "math/big": true, "regexp": true, "slices": true, "sort": true,
	"strconv": true, "strings": true, "time": true, "unicode": true,
	"unicode/utf8": true,
	// unicodedata.normalize / re — Python domain stdlib; neither reaches I/O.
	"golang.org/x/text/unicode/norm": true,
}

const domainPrefix = "mobile/services/core/internal/domain/"

// forbiddenImports parses one Go file's imports and returns each one outside
// the allowlist and outside other domain packages, as "file: path".
func forbiddenImports(fset *token.FileSet, name string, src any) ([]string, error) {
	file, err := parser.ParseFile(fset, name, src, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return nil, err
		}
		if domainAllowed[path] || strings.HasPrefix(path, domainPrefix) {
			continue
		}
		out = append(out, name+": "+path)
	}
	return out, nil
}

// walkDomain checks every shipped Go file under root. Test files may import
// what they need to load goldens, and testdata is not code.
func walkDomain(root string) (files int, violations []string, err error) {
	fset := token.NewFileSet()
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && entry.Name() == "testdata" {
			return filepath.SkipDir
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		found, err := forbiddenImports(fset, path, nil)
		if err != nil {
			return err
		}
		files++
		violations = append(violations, found...)
		return nil
	})
	sort.Strings(violations)
	return files, violations, err
}

func TestDomainImportsOnlyPureStandardLibrary(t *testing.T) {
	files, violations, err := walkDomain(filepath.Join("..", "..", "internal", "domain"))
	if err != nil {
		t.Fatal(err)
	}
	if files == 0 {
		t.Fatal("no domain source found under internal/domain: this test measured nothing")
	}
	if len(violations) > 0 {
		t.Fatalf("internal/domain may import only the pure standard library and other domain packages:\n%s",
			strings.Join(violations, "\n"))
	}
}

// The walk itself must find a leak wherever a shipped domain file hides it,
// and only there.
func TestTheWalkFindsALeakInAShippedFileOnly(t *testing.T) {
	root := t.TempDir()
	write := func(rel, src string) {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("money/money.go", "package money\n\nimport \"math/big\"\n\nvar _ = big.NewRat\n")
	write("money/money_test.go", "package money\n\nimport _ \"os\"\n")
	write("money/testdata/fixture.go", "package fixture\n\nimport _ \"net\"\n")
	write("ledger/deep/ledger.go", "package deep\n\nimport (\n\t\"strings\"\n\t_ \"os\"\n)\n\nvar _ = strings.ToLower\n")
	files, violations, err := walkDomain(root)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "ledger", "deep", "ledger.go") + ": os"
	if files != 2 || len(violations) != 1 || violations[0] != want {
		t.Fatalf("files %d, violations %v; want 2 files and only %q", files, violations, want)
	}
}

func TestTheCheckerRefusesWhatTheDomainMustNotReach(t *testing.T) {
	cases := map[string]bool{
		`package p; import "strings"`:                                          false,
		`package p; import "math/big"`:                                         false,
		`package p; import "mobile/services/core/internal/domain/permissions"`: false,
		`package p; import "regexp"`:                                           false,
		`package p; import "golang.org/x/text/unicode/norm"`:                   false,
		`package p; import "net/http"`:                                         true,
		`package p; import "os"`:                                               true,
		`package p; import "database/sql"`:                                     true,
		`package p; import _ "embed"`:                                          true,
		`package p; import "github.com/jackc/pgx/v5"`:                          true,
		`package p; import "mobile/services/core/internal/db"`:                 true,
		`package p; import "mobile/services/core/internal/domainx"`:            true,
		`package p; import ( "sort"; h "net/http" )`:                           true,
	}
	for src, forbidden := range cases {
		found, err := forbiddenImports(token.NewFileSet(), "case.go", src)
		if err != nil {
			t.Fatal(err)
		}
		if (len(found) > 0) != forbidden {
			t.Fatalf("%s: found %v, want forbidden=%v", src, found, forbidden)
		}
	}
}
