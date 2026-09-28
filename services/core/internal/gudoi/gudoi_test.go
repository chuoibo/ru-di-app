package gudoi

import (
	"reflect"
	"testing"
	"time"

	"mobile/services/core/internal/domain/pairnotebook"
)

const (
	an   = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	binh = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
)

var (
	now   = mocChat.Add(48 * time.Hour)
	truoc = mocChat.Add(-time.Hour)
	sau   = mocChat.Add(time.Hour)
)

func t0(t time.Time) *time.Time { return &t }

// doi is a live «Một đôi» both said yes to on one completed proposal.
func doi() []pairnotebook.Consent {
	xong := t0(truoc)
	return []pairnotebook.Consent{
		{PersonID: an, Purpose: "bat_doi", GrantedAt: t0(truoc), ProposalID: "p-doi", ProposalCompletedAt: xong},
		{PersonID: binh, Purpose: "bat_doi", GrantedAt: t0(truoc), ProposalID: "p-doi", ProposalCompletedAt: xong},
	}
}

// chiaGu is person's own `chia_gu`, completed as it is filed.
func chiaGu(person string, luc time.Time, thuHoi *time.Time) pairnotebook.Consent {
	return pairnotebook.Consent{PersonID: person, Purpose: "chia_gu", GrantedAt: t0(luc), RevokedAt: thuHoi, ProposalID: "p-gu-" + person + luc.String(), ProposalCompletedAt: t0(luc)}
}

var gu = map[string][]string{an: {"cafe", "outdoor", "la-la"}, binh: {"outdoor", "cafe", "karaoke"}}

func TestChiNguoiDongYMoiDuocDung(t *testing.T) {
	ai := []string{an, binh}
	for _, c := range []struct {
		ten   string
		them  []pairnotebook.Consent
		doi   bool
		want  []string
		chung []string
	}{
		{"chỉ An, đồng ý mới", []pairnotebook.Consent{chiaGu(an, sau, nil)}, true, []string{an}, []string{}},
		{"cả hai, đồng ý mới", []pairnotebook.Consent{chiaGu(an, sau, nil), chiaGu(binh, sau, nil)}, true, []string{an, binh}, []string{"cafe", "outdoor"}},
		{"đồng ý cũ (trước mốc)", []pairnotebook.Consent{chiaGu(an, truoc, nil), chiaGu(binh, truoc, nil)}, true, []string{}, []string{}},
		{"đồng ý đúng mốc", []pairnotebook.Consent{chiaGu(an, mocChat, nil)}, true, []string{an}, []string{}},
		{"đã thu hồi", []pairnotebook.Consent{chiaGu(an, sau, t0(sau.Add(time.Minute)))}, true, []string{}, []string{}},
		{"cũ thu hồi, bật lại", []pairnotebook.Consent{chiaGu(an, truoc, t0(sau)), chiaGu(an, sau.Add(time.Minute), nil)}, true, []string{an}, []string{}},
		{"không phải cặp đôi", []pairnotebook.Consent{chiaGu(an, sau, nil), chiaGu(binh, sau, nil)}, false, []string{}, []string{}},
	} {
		consents := c.them
		if c.doi {
			consents = append(doi(), c.them...)
		}
		got := NguoiDuocDung(consents, ai, now)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: %v, muốn %v", c.ten, got, c.want)
		}
		gs := GuChoChat(got, gu)
		if ch := Chung(gs); !reflect.DeepEqual(ch, c.chung) {
			t.Errorf("%s: chung %v, muốn %v", c.ten, ch, c.chung)
		}
		for _, g := range gs {
			if !reflect.DeepEqual(got, c.want) || (g.NguoiID != an && g.NguoiID != binh) {
				t.Errorf("%s: gu của người ngoài %v", c.ten, g)
			}
		}
	}
}

func TestGuTheoThuTuTuVung(t *testing.T) {
	gs := GuChoChat([]string{an, binh}, gu)
	if len(gs) != 2 || !reflect.DeepEqual(gs[0].The, []string{"cafe", "outdoor"}) || !reflect.DeepEqual(gs[1].The, []string{"cafe", "outdoor", "karaoke"}) {
		t.Fatalf("%+v", gs)
	}
	// A person eligible but with no known tag is left out; somebody not
	// eligible is never read, whatever the map holds.
	if gs := GuChoChat([]string{an}, map[string][]string{an: {"la"}, binh: {"cafe"}}); len(gs) != 0 {
		t.Fatalf("%+v", gs)
	}
	if Nhan("cafe") != "Cafe" || Nhan("la") != "" {
		t.Fatal("nhãn")
	}
}

func TestTrangThaiCua(t *testing.T) {
	for _, c := range []struct {
		ten  string
		them []pairnotebook.Consent
		want TrangThai
	}{
		{"tắt", nil, Tat},
		{"bật mới", []pairnotebook.Consent{chiaGu(an, sau, nil)}, Bat},
		{"bật cũ", []pairnotebook.Consent{chiaGu(an, truoc, nil)}, CanBatLai},
		{"bật lại", []pairnotebook.Consent{chiaGu(an, truoc, t0(sau)), chiaGu(an, sau.Add(time.Minute), nil)}, Bat},
		{"cũ đã thu hồi", []pairnotebook.Consent{chiaGu(an, truoc, t0(sau))}, Tat},
	} {
		if got := TrangThaiCua(append(doi(), c.them...), an, now); got != c.want {
			t.Errorf("%s: %s, muốn %s", c.ten, got, c.want)
		}
	}
}

// The instant is fixed: midnight starting 29 September 2026 in Vietnam.
func TestMocChat(t *testing.T) {
	vn := MocChat().In(time.FixedZone("ICT", 7*3600))
	if y, m, d := vn.Date(); y != 2026 || m != time.September || d != 29 || vn.Hour() != 0 || vn.Minute() != 0 || vn.Second() != 0 || vn.Nanosecond() != 0 {
		t.Fatal(vn)
	}
	if ChoChat(nil) || ChoChat(t0(truoc)) || !ChoChat(t0(mocChat)) {
		t.Fatal("ChoChat")
	}
}
