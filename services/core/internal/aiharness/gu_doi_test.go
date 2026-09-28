package aiharness

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/obs"
	"mobile/services/core/internal/aiharness/testkit"
	"mobile/services/core/internal/gudoi"
)

// A couple's shared taste through the engine (ADR-0048): the model decides
// to call gu_hai_ban (no keyword of the message does), the taste enters the
// ledger as evidence the verifier judges against, and the result names whose
// taste was read for the worker's re-check and the card. A chat of two among
// friends never declares the tool, and its port is never read.

func kichGu(cau string) []llm.Buoc {
	return kichGuKiem(cau, kiemHoTro(1, "ho_tro"))
}

// kichGuKiem is kichGu with the verifier's output given: an answer with no
// taste in the ledger cites nothing.
func kichGuKiem(cau string, kiem llm.Buoc) []llm.Buoc {
	return []llm.Buoc{
		ru{huong: "tac_tu", yDinh: []string{"find_places"}}.buoc(),
		goiCC("gu_hai_ban", map[string]any{}),
		dung(false, cau), kiem,
	}
}

// khaiBaoGu reports whether any request declared gu_hai_ban as a function
// the model may call (the history may still hold a call the model made).
func khaiBaoGu(m moTa) bool {
	for _, y := range m.stub.YeuCau() {
		var r struct {
			Config struct {
				Tools []struct {
					FunctionDeclarations []struct{ Name string } `json:"functionDeclarations"`
				} `json:"tools"`
			} `json:"config"`
		}
		if json.Unmarshal(y, &r) != nil {
			continue
		}
		for _, tl := range r.Config.Tools {
			for _, d := range tl.FunctionDeclarations {
				if d.Name == "gu_hai_ban" {
					return true
				}
			}
		}
	}
	return false
}

func TestDoiDungGuQuaCongCu(t *testing.T) {
	w := moiTheGioi(t)
	port := &testkit.GuDoi{Gu: []gudoi.Gu{{NguoiID: nguoiLan, The: []string{"cafe", "outdoor"}}}}
	turn := luotHaiNguoi(true)
	turn.LoiNho = "@Rủ Đi tối nay hai đứa đi đâu?"
	m := chayNhom(t, w, nhomOpts{gu: port}, turn, kichGu("Lan thích cafe nên hai bạn ghé một quán cafe yên tĩnh nhé.")...)
	if m.err != nil {
		t.Fatalf("%v %+v", m.err, m.res.Record)
	}
	if !reflect.DeepEqual(m.res.GuDung, []NguoiGu{{ID: nguoiLan, Nhan: "Lan"}}) {
		t.Fatalf("GuDung %+v", m.res.GuDung)
	}
	if !khaiBaoGu(m) || !reflect.DeepEqual(port.Hoi, []string{phongNhom}) {
		t.Fatalf("declared %v, port read %v", khaiBaoGu(m), port.Hoi)
	}
	if !strings.Contains(strings.Join(func() []string {
		var s []string
		for _, c := range m.res.Record.CongCu {
			s = append(s, string(c))
		}
		return s
	}(), ","), "gu_hai_ban") || m.res.Record.Bot != obs.BotDoi || m.res.Record.Valid() != nil {
		t.Fatalf("record %+v", m.res.Record)
	}
	// The taste reached the answer step as evidence under an alias, the
	// person's id never.
	var sau string
	for _, y := range m.stub.YeuCau()[2:] {
		sau += string(y)
	}
	if !strings.Contains(sau, "d1") || strings.Contains(sau, nguoiLan) {
		t.Fatal("the taste is not evidence under its alias, or a person id reached the model")
	}
	// Nothing in the log carries the taste.
	if strings.Contains(m.log.String(), "Cafe") || strings.Contains(m.log.String(), "outdoor") {
		t.Fatalf("the log carries the taste: %s", m.log.String())
	}
}

// Nobody shared under the chat's wording: the tool answers an empty list,
// and the answer names nobody's taste.
func TestDoiGuRongKhongTenAi(t *testing.T) {
	w := moiTheGioi(t)
	port := &testkit.GuDoi{}
	m := chayNhom(t, w, nhomOpts{gu: port}, luotHaiNguoi(true), kichGuKiem("Mình chưa biết gu hai bạn, thử một quán cafe yên tĩnh nhé.", kiemDat())...)
	if m.err != nil || len(m.res.GuDung) != 0 || port.SoLanHoi() != 1 {
		t.Fatalf("%v %+v %d", m.err, m.res.GuDung, port.SoLanHoi())
	}
}

// A chat of two among friends (Doi false) never offers gu_hai_ban: not in
// any declaration, and a call the model makes anyway is refused before the
// port is read.
func TestDamBanKhongCoGuHaiBan(t *testing.T) {
	w := moiTheGioi(t)
	port := &testkit.GuDoi{Gu: []gudoi.Gu{{NguoiID: nguoiLan, The: []string{"cafe"}}}}
	m := chayNhom(t, w, nhomOpts{gu: port}, luotHaiNguoi(false), kichGuKiem("Hai bạn ghé một quán cafe yên tĩnh nhé.", kiemDat())...)
	if khaiBaoGu(m) || port.SoLanHoi() != 0 || len(m.res.GuDung) != 0 {
		t.Fatalf("declared %v, port read %d, GuDung %+v (err %v)", khaiBaoGu(m), port.SoLanHoi(), m.res.GuDung, m.err)
	}
	// A group, likewise.
	m = chayNhom(t, w, nhomOpts{gu: port}, luotNhomCoBan(), kichTacTu()...)
	if khaiBaoGu(m) || port.SoLanHoi() != 0 {
		t.Fatal("a group declared or read the couple's taste")
	}
}
