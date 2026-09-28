package traloi

import (
	"reflect"
	"strconv"
	"strings"
	"testing"

	"mobile/services/core/internal/aiharness/cau"
	"mobile/services/core/internal/aiharness/kiemchung"
	"mobile/services/core/internal/aiharness/obs"
	"mobile/services/core/internal/aiharness/tools"
	"mobile/services/core/internal/aiharness/truyhoi"
)

func TestTachCauVanXuoi(t *testing.T) {
	got := TachCauVanXuoi("Quán mở 7.30 tới 22.00! Giá 150.000đ... Còn gì nữa?\nThứ hai:  đi sớm")
	want := []string{"Quán mở 7.30 tới 22.00!", "Giá 150.000đ...", "Còn gì nữa?", "Thứ hai: đi sớm"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tách %q", got)
	}
	if got := TachCauVanXuoi("  \n "); len(got) != 0 {
		t.Fatalf("chữ rỗng thành %q", got)
	}
	var dai []string
	for i := 0; i < kiemchung.MaxMenhDe+3; i++ {
		dai = append(dai, "Câu "+strconv.Itoa(i)+".")
	}
	got = TachCauVanXuoi(strings.Join(dai, " "))
	if len(got) != kiemchung.MaxMenhDe || !strings.HasSuffix(got[len(got)-1], "Câu "+strconv.Itoa(kiemchung.MaxMenhDe+2)+".") {
		t.Fatalf("quá trần: %d câu, câu cuối %q", len(got), got[len(got)-1])
	}
}

func TestGhepVanXuoi(t *testing.T) {
	sc := tools.MoiSoCai(obs.BotNep)
	sc.Ghi(tools.SearchPlaces, []truyhoi.BangChung{{ID: "id-a", Nguon: truyhoi.Places, Truong: map[string]string{"ten": "Quán Gió"}}})
	a, _ := sc.BiDanh("id-a")
	chu, la, hong := GhepVanXuoi("Thử [["+"p:"+a+"]] nhé.\nHoặc [[p:p9]] và [[p:", sc)
	if chu != "Thử Quán Gió nhé.\nHoặc "+cau.MotCho+" và" || la != 1 || hong != 1 {
		t.Fatalf("ghép %q la=%d hong=%d", chu, la, hong)
	}
}
