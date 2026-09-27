package nepnho

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mobile/services/core/internal/aiharness/trinho"
)

func TestMatchesChiDungDuongTriNho(t *testing.T) {
	for _, p := range []string{"/me/nep/tri-nho", "/me/nep/tri-nho/", "/me/nep/tri-nho/cai-dat", "/me/nep/su-kien"} {
		if !Matches(p) {
			t.Errorf("not matched: %s", p)
		}
	}
	for _, p := range []string{"/me/nep/media", "/me/nep/ai-invocations", "/me/nep/tri-nho/x", "/me/nep", "/contexts/x/tri-nho"} {
		if Matches(p) {
			t.Errorf("took %s from its owner", p)
		}
	}
}

// Every listed route is registered, and every registered pattern matches.
func TestRoutesDeuDangKy(t *testing.T) {
	h := NewHandler(nil, nil)
	for _, r := range Routes() {
		method, path, _ := strings.Cut(r, " ")
		req := httptest.NewRequest(method, path, nil)
		_, pattern := h.mux.Handler(req)
		if pattern != r {
			t.Errorf("%s registered as %q", r, pattern)
		}
		if !Matches(path) {
			t.Errorf("%s not matched", r)
		}
	}
}

// Shape is refused before anyone is authenticated or any row is read (the
// handler has no pool here: reaching the database would panic).
func TestHinhSaiBiTuChoiTruocXacThuc(t *testing.T) {
	k, err := Moi(nil, nil, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(nil, k)
	for _, c := range []struct {
		method, path, ctype, body string
		want                      int
	}{
		{"PUT", "/me/nep/tri-nho/cai-dat", "text/plain", `{"nho":true}`, 415},
		{"PUT", "/me/nep/tri-nho/cai-dat", "application/json", `{"nho":true}`, 400},
		{"PUT", "/me/nep/tri-nho/cai-dat", "application/json", `{"nho":false,"cong_bo_ban":1}`, 400},
		{"PUT", "/me/nep/tri-nho/cai-dat", "application/json", `{"nho":true,"cong_bo_ban":1,"ghi_chu":"x"}`, 400},
		{"PUT", "/me/nep/tri-nho/cai-dat", "application/json", `{}`, 400},
		{"POST", "/me/nep/su-kien", "application/json", `{"su_kien":[]}`, 400},
		{"POST", "/me/nep/su-kien", "application/json", `{"su_kien":[{"loai":"doc_chat","luc":"2026-09-25T00:00:00Z"}]}`, 400},
		{"POST", "/me/nep/su-kien", "application/json", `{"su_kien":[{"loai":"mo_dia_diem","luc":"2026-09-25T00:00:00Z"}]}`, 400},
		{"POST", "/me/nep/su-kien", "application/json", `{"su_kien":[{"loai":"mo_dia_diem","luc":"2026-09-25T00:00:00Z","dia_diem_id":"p1","ghi_chu":"quán của Lan"}]}`, 400},
		{"POST", "/me/nep/su-kien", "application/json", `{"su_kien":[{"loai":"tao_keo","luc":"2026-09-25T00:00:00Z","danh_muc":"cafe"}]}`, 400},
	} {
		req := httptest.NewRequest(c.method, c.path, strings.NewReader(c.body))
		req.Header.Set("Content-Type", c.ctype)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != c.want {
			t.Errorf("%s %s %s: %d, want %d", c.method, c.path, c.body, w.Code, c.want)
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Error("memory answers must not be cached")
		}
	}
	// No bearer: 401 without a database.
	for _, r := range Routes() {
		method, path, _ := strings.Cut(r, " ")
		body := ""
		switch r {
		case "PUT /me/nep/tri-nho/cai-dat":
			body = `{"nho":false}`
		case "POST /me/nep/su-kien":
			body = `{"su_kien":[{"loai":"tao_keo","luc":"2026-09-25T00:00:00Z"}]}`
		}
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s without a session: %d", r, w.Code)
		}
	}
}

// Each kind carries exactly its own typed reference, as the table's CHECK.
func TestSuKienHinhDong(t *testing.T) {
	luc := time.Date(2026, 9, 25, 7, 0, 0, 0, time.UTC)
	ok := []SuKien{
		{Loai: trinho.MoDiaDiem, Luc: luc, DiaDiemID: "p_123"},
		{Loai: trinho.CheckIn, Luc: luc, DiaDiemID: "0b8f1c9e-aaaa"},
		{Loai: trinho.ChonDiemDen, Luc: luc, DiemDenID: "da-lat"},
		{Loai: trinho.LocDanhMuc, Luc: luc, DanhMuc: "cafe"},
		{Loai: trinho.ChonPhuongTien, Luc: luc, PhuongTien: "motorbike"},
		{Loai: trinho.ChonThoiLuong, Luc: luc, ThoiLuongPhut: 90},
		{Loai: trinho.TaoKeo, Luc: luc},
	}
	for _, s := range ok {
		if err := s.Kiem(); err != nil {
			t.Errorf("%s refused: %v", s.Loai, err)
		}
	}
	bad := []SuKien{
		{Loai: "doc_chat", Luc: luc},
		{Loai: trinho.MoDiaDiem, DiaDiemID: "p1"},
		{Loai: trinho.MoDiaDiem, Luc: luc},
		{Loai: trinho.MoDiaDiem, Luc: luc, DiaDiemID: "quán cà phê"},
		{Loai: trinho.MoDiaDiem, Luc: luc, DiaDiemID: "p1", DanhMuc: "cafe"},
		{Loai: trinho.LocDanhMuc, Luc: luc, DanhMuc: "Cà phê"},
		{Loai: trinho.ChonPhuongTien, Luc: luc, PhuongTien: "bus"},
		{Loai: trinho.ChonThoiLuong, Luc: luc, ThoiLuongPhut: 2},
		{Loai: trinho.TaoKeo, Luc: luc, DiemDenID: "da-lat"},
	}
	for _, s := range bad {
		if err := s.Kiem(); err == nil {
			t.Errorf("%+v accepted", s)
		}
	}
}

// A host without the memory key serves every memory route as 503, reading
// nothing.
func TestKhongKhoaThi503(t *testing.T) {
	h := NewHandler(nil, nil)
	for _, r := range Routes() {
		method, path, _ := strings.Cut(r, " ")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(method, path, nil))
		if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "nep_memory_unavailable") {
			t.Errorf("%s without a key: %d %s", r, w.Code, w.Body)
		}
	}
}
