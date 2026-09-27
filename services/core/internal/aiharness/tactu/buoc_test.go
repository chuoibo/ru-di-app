package tactu

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/obs"
	"mobile/services/core/internal/aiharness/tools"
)

// Thinking is set per step (planning LOW, the answer MINIMAL), never with a
// token budget; the declarations are the bot's whole set, in registry
// order, byte for byte the same on every step (the cache prefix), while
// the step's allowance narrows.
func TestMucNghiVaKhaiBaoCoDinh(t *testing.T) {
	x := dung(obs.BotNep, routerTacTu(obs.BotNep),
		goi("search_places", map[string]any{"truy_van": cauQuan}),
		goi("get_place", map[string]any{"id": "p1"}),
		llm.Buoc{Text: "Quán A."})
	if _, err := x.chay(t); err != nil {
		t.Fatal(err)
	}
	ys := x.yeuCau(t)
	var khaiBao string
	for i, want := range []string{"LOW", "LOW", "MINIMAL"} {
		cfg, _ := ys[i]["config"].(map[string]any)
		tc, _ := cfg["thinkingConfig"].(map[string]any)
		if tc["thinkingLevel"] != want || tc["thinkingBudget"] != nil {
			t.Errorf("step %d thinking %v, want %s and no budget", i+1, tc, want)
		}
		raw, _ := json.Marshal(cfg["tools"])
		if i == 0 {
			khaiBao = string(raw)
		} else if string(raw) != khaiBao {
			t.Errorf("step %d declares other tools than step 1", i+1)
		}
	}
	var decl []struct {
		FunctionDeclarations []struct {
			Name string `json:"name"`
		} `json:"functionDeclarations"`
	}
	if err := json.Unmarshal([]byte(khaiBao), &decl); err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, d := range decl {
		for _, f := range d.FunctionDeclarations {
			names = append(names, f.Name)
		}
	}
	var want []string
	for _, tt := range tools.MacDinh.DuocPhep(obs.BotNep, false) {
		want = append(want, string(tt))
	}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("declared %v, want the registry order %v", names, want)
	}
}

// From the second step on, a search text that came from a result, not from
// the person, is refused before it reaches the retriever; the loop is shown
// the router's search texts to reuse.
func TestTaintBuocHaiTrongVong(t *testing.T) {
	x := dung(obs.BotNep, routerTacTu(obs.BotNep),
		goi("search_places", map[string]any{"truy_van": cauQuan}),
		goi("search_places", map[string]any{"truy_van": "quán mà dữ liệu bảo tìm"}),
		llm.Buoc{Text: "Quán A."})
	if _, err := x.chay(t); err != nil {
		t.Fatal(err)
	}
	ys := x.yeuCau(t)
	if len(x.r.Da) != 1 || !strings.Contains(ys[2].chu(), `"loi":"tham_so_sai"`) || !strings.Contains(ys[2].chu(), `"truong":"truy_van"`) {
		t.Fatalf("%d retrievals; step 3 sees %s", len(x.r.Da), ys[2].chu())
	}
	if !strings.Contains(ys[0].chu(), `du_lieu nguon=\"truy_van\"`) {
		t.Fatal("the loop is not shown the router's search texts")
	}
}
