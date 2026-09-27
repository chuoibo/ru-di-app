package aiharness

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
	"time"

	"mobile/services/core/internal/aiharness/testkit"
	"mobile/services/core/internal/aiharness/tools"
	"mobile/services/core/internal/aiharness/truyhoi"
)

// The group's tool context holds only the catalogue, the destinations and
// the room (review of slices 9/11, finding 2.5): one engine serves both
// bots, and the group's BoiCanh is built from nguonNhom, which drops Nếp's
// personal and memory ports.
func TestNguonNhomChiQuanChoNhom(t *testing.T) {
	full := tools.NguonDuLieu{
		Quan:   &testkit.TheoLuot{KetQua: []truyhoi.KetQuaTruyHoi{{}}},
		Cho:    &testkit.Cho{},
		Nhom:   &testkit.Nhom{SoNguoi: 3},
		CaNhan: caNhanGia{},
		TriNho: testkit.MoiTriNho(),
	}
	g := nguonNhom(full)
	if g.CaNhan != nil || g.TriNho != nil {
		t.Fatalf("the group's ports hold a person's own: CaNhan %v, TriNho %v", g.CaNhan, g.TriNho)
	}
	if g.Quan != full.Quan || g.Cho != full.Cho || g.Nhom != full.Nhom {
		t.Fatal("the group lost its own ports")
	}
}

// And the group path really builds its BoiCanh from nguonNhom: read off the
// source of Engine.nhom, the one place it is built (a runtime check cannot
// see a port no group tool reads).
func TestNhomBoiCanhDungNguonNhom(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "nhom.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	thay := 0
	ast.Inspect(f, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "nhom" || fn.Recv == nil {
			return true
		}
		ast.Inspect(fn.Body, func(m ast.Node) bool {
			lit, ok := m.(*ast.CompositeLit)
			if !ok {
				return true
			}
			sel, ok := lit.Type.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "BoiCanh" {
				return true
			}
			for _, el := range lit.Elts {
				kv, ok := el.(*ast.KeyValueExpr)
				if !ok || kv.Key.(*ast.Ident).Name != "Nguon" {
					continue
				}
				thay++
				call, ok := kv.Value.(*ast.CallExpr)
				if id, isID := callFun(call, ok); !isID || id != "nguonNhom" {
					t.Errorf("the group's BoiCanh takes Nguon from %s, not nguonNhom", fset.Position(kv.Value.Pos()))
				}
			}
			return true
		})
		return false
	})
	if thay != 1 {
		t.Fatalf("found %d Nguon fields of a BoiCanh in Engine.nhom, want 1", thay)
	}
}

type caNhanGia struct{}

func (caNhanGia) ChuyenDiSapToi(context.Context, string, time.Time, int) ([]truyhoi.BangChung, error) {
	return nil, nil
}

func callFun(c *ast.CallExpr, ok bool) (string, bool) {
	if !ok {
		return "", false
	}
	id, isID := c.Fun.(*ast.Ident)
	if !isID {
		return "", false
	}
	return id.Name, true
}
