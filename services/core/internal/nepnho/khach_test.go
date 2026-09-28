package nepnho

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const tokenThu = "0123456789abcdef0123456789abcdef-sidecar"

type yeuCau struct {
	path string
	auth string
	body map[string]any
}

// sidecarThu answers each path with the given status and body, and records
// what Go sent.
func sidecarThu(t *testing.T, tra map[string]struct {
	status int
	body   string
}) (*KhachHTTP, *[]yeuCau) {
	t.Helper()
	var got []yeuCau
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var b map[string]any
		_ = json.Unmarshal(raw, &b)
		got = append(got, yeuCau{path: r.URL.Path, auth: r.Header.Get("Authorization"), body: b})
		a, ok := tra[r.URL.Path]
		if !ok {
			w.WriteHeader(500)
			return
		}
		if a.status == -1 {
			time.Sleep(1500 * time.Millisecond)
			a.status = 200
		}
		w.WriteHeader(a.status)
		_, _ = io.WriteString(w, a.body)
	}))
	t.Cleanup(srv.Close)
	k, err := MoiKhachHTTP(srv.URL, tokenThu, 0.3)
	if err != nil {
		t.Fatal(err)
	}
	return k, &got
}

type tl = struct {
	status int
	body   string
}

// The wire contract with services/ai-infer: paths, bearer token, and the
// exact fields of each body.
func TestKhachHopDongDay(t *testing.T) {
	k, got := sidecarThu(t, map[string]tl{
		"/v1/memory/add":        {200, `{"added":[{"id":"0b8f1c9e-aaaa-4bbb-8ccc-dddddddddd01","text":"Người dùng thích cà phê yên tĩnh","loai":"thich_danh_muc"}],"refused":1}`},
		"/v1/memory/search":     {200, `{"items":[{"id":"a1","text":"x","score":0.8,"loai":"phuong_tien"}]}`},
		"/v1/memory/list":       {200, `{"items":[{"id":"a1","text":"x","score":null,"loai":null}],"count":1}`},
		"/v1/memory/delete":     {200, `{"deleted":1,"remaining":0}`},
		"/v1/memory/delete_all": {200, `{"deleted":2,"remaining":0}`},
		"/v1/memory/purge_user": {200, `{"deleted":3,"remaining":0,"compaction_s":1.2}`},
	})
	ctx := context.Background()
	ms, refused, err := k.Them(ctx, "u1", "Mình thích cà phê yên tĩnh")
	if err != nil || len(ms) != 1 || ms[0].ID != "0b8f1c9e-aaaa-4bbb-8ccc-dddddddddd01" || ms[0].Loai != "thich_danh_muc" || refused != 1 {
		t.Fatalf("Them %+v %d %v", ms, refused, err)
	}
	if items, err := k.Tim(ctx, "u1", "cà phê", 5); err != nil || len(items) != 1 || items[0].Score != 0.8 {
		t.Fatalf("Tim %+v %v", items, err)
	}
	if items, err := k.LietKe(ctx, "u1"); err != nil || len(items) != 1 || items[0].Loai != "" {
		t.Fatalf("LietKe %+v %v", items, err)
	}
	if n, err := k.Xoa(ctx, "u1", "a1"); err != nil || n != 1 {
		t.Fatalf("Xoa %d %v", n, err)
	}
	if n, err := k.XoaHet(ctx, "u1"); err != nil || n != 2 {
		t.Fatalf("XoaHet %d %v", n, err)
	}
	if n, err := k.XoaSach(ctx, "u1"); err != nil || n != 3 {
		t.Fatalf("XoaSach %d %v", n, err)
	}
	if len(*got) != 6 {
		t.Fatalf("%d calls", len(*got))
	}
	for _, y := range *got {
		if y.auth != "Bearer "+tokenThu {
			t.Errorf("%s without the bearer token", y.path)
		}
		if y.body["user_id"] != "u1" {
			t.Errorf("%s without the owner", y.path)
		}
	}
	// add carries exactly the owner and the person's own words, role user:
	// the sidecar's AddReq has no other field.
	add := (*got)[0].body
	if len(add) != 2 {
		t.Fatalf("add body has fields beyond user_id and messages: %v", add)
	}
	msgs := add["messages"].([]any)
	if len(msgs) != 1 || msgs[0].(map[string]any)["role"] != "user" || msgs[0].(map[string]any)["content"] != "Mình thích cà phê yên tĩnh" {
		t.Fatalf("messages %v", msgs)
	}
	if s := (*got)[1].body; s["top_k"] != float64(5) || s["threshold"] != 0.3 || s["query"] != "cà phê" {
		t.Fatalf("search body %v", s)
	}
	if l := (*got)[2].body; l["limit"] != float64(LietKeToiDa) {
		t.Fatalf("list body %v", l)
	}
	if d := (*got)[3].body; d["memory_id"] != "a1" {
		t.Fatalf("delete body %v", d)
	}
	for _, i := range []int{4, 5} {
		if b := (*got)[i].body; len(b) != 1 {
			t.Fatalf("%s body %v", (*got)[i].path, b)
		}
	}
}

// Answers outside the contract are errors, never a partial success.
func TestKhachTuChoiTraLoiSai(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		ten  string
		path string
		a    tl
		goi  func(k *KhachHTTP) error
		want error
	}{
		{"add khong refused", "/v1/memory/add", tl{200, `{"added":[]}`}, func(k *KhachHTTP) error {
			_, _, err := k.Them(ctx, "u", "x")
			return err
		}, ErrDichVuSai},
		{"add muc khong id", "/v1/memory/add", tl{200, `{"added":[{"text":"x"}],"refused":0}`}, func(k *KhachHTTP) error {
			_, _, err := k.Them(ctx, "u", "x")
			return err
		}, ErrDichVuSai},
		{"add qua nhieu", "/v1/memory/add", tl{200, `{"added":[` + strings.Repeat(`{"id":"a"},`, MaxThemMotLan) + `{"id":"b"}],"refused":0}`}, func(k *KhachHTTP) error {
			_, _, err := k.Them(ctx, "u", "x")
			return err
		}, ErrDichVuSai},
		{"search qua k", "/v1/memory/search", tl{200, `{"items":[{"id":"a"},{"id":"b"}]}`}, func(k *KhachHTTP) error {
			_, err := k.Tim(ctx, "u", "q", 1)
			return err
		}, ErrDichVuSai},
		{"search khong id", "/v1/memory/search", tl{200, `{"items":[{"text":"x"}]}`}, func(k *KhachHTTP) error {
			_, err := k.Tim(ctx, "u", "q", 3)
			return err
		}, ErrDichVuSai},
		{"search 503", "/v1/memory/search", tl{503, `{"detail":"memory disabled"}`}, func(k *KhachHTTP) error {
			_, err := k.Tim(ctx, "u", "q", 3)
			return err
		}, ErrDichVuVang},
		{"search 401", "/v1/memory/search", tl{401, `{}`}, func(k *KhachHTTP) error {
			_, err := k.Tim(ctx, "u", "q", 3)
			return err
		}, ErrDichVuSai},
		{"search cham hon han", "/v1/memory/search", tl{-1, `{"items":[]}`}, func(k *KhachHTTP) error {
			_, err := k.Tim(ctx, "u", "q", 3)
			return err
		}, ErrDichVuVang},
		{"list dem lech", "/v1/memory/list", tl{200, `{"items":[{"id":"a"}],"count":2}`}, func(k *KhachHTTP) error {
			_, err := k.LietKe(ctx, "u")
			return err
		}, ErrDichVuSai},
		{"delete con sot", "/v1/memory/delete", tl{200, `{"deleted":1,"remaining":1}`}, func(k *KhachHTTP) error {
			_, err := k.Xoa(ctx, "u", "a")
			return err
		}, ErrConHang},
		{"delete khong xong", "/v1/memory/delete", tl{500, `{"detail":"delete incomplete","remaining":1}`}, func(k *KhachHTTP) error {
			_, err := k.Xoa(ctx, "u", "a")
			return err
		}, ErrConHang},
		{"delete 500 khac", "/v1/memory/delete", tl{500, `{"detail":"owner check failed"}`}, func(k *KhachHTTP) error {
			_, err := k.Xoa(ctx, "u", "a")
			return err
		}, ErrDichVuVang},
		{"delete_all khong xong", "/v1/memory/delete_all", tl{500, `{"detail":"delete incomplete","remaining":3}`}, func(k *KhachHTTP) error {
			_, err := k.XoaHet(ctx, "u")
			return err
		}, ErrConHang},
		{"delete_all thieu remaining", "/v1/memory/delete_all", tl{200, `{"deleted":1}`}, func(k *KhachHTTP) error {
			_, err := k.XoaHet(ctx, "u")
			return err
		}, ErrDichVuSai},
		{"purge con sot", "/v1/memory/purge_user", tl{200, `{"deleted":1,"remaining":2}`}, func(k *KhachHTTP) error {
			_, err := k.XoaSach(ctx, "u")
			return err
		}, ErrConHang},
		{"purge thieu remaining", "/v1/memory/purge_user", tl{200, `{"deleted":1}`}, func(k *KhachHTTP) error {
			_, err := k.XoaSach(ctx, "u")
			return err
		}, ErrDichVuSai},
	}
	for _, c := range cases {
		k, _ := sidecarThu(t, map[string]tl{c.path: c.a})
		if err := c.goi(k); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", c.ten, err, c.want)
		}
	}
	// A memory already gone is 0 deleted, not an error.
	k, _ := sidecarThu(t, map[string]tl{"/v1/memory/delete": {404, `{"detail":"not found"}`}})
	if n, err := k.Xoa(ctx, "u", "a"); err != nil || n != 0 {
		t.Fatalf("404 delete = %d %v", n, err)
	}
	// Nothing listening.
	dead, _ := MoiKhachHTTP("http://127.0.0.1:1", tokenThu, 0.3)
	if _, err := dead.Tim(ctx, "u", "q", 3); !errors.Is(err, ErrDichVuVang) || strings.Contains(err.Error(), "connection refused") {
		t.Fatalf("dead sidecar: %v", err)
	}
}

func TestMoiKhachHTTPKiemCauHinh(t *testing.T) {
	for _, c := range []struct {
		goc, token string
		nguong     float64
	}{{"127.0.0.1:8090", tokenThu, 0.3}, {"http://x", "short", 0.3}, {"http://x", tokenThu, 1.5}} {
		if _, err := MoiKhachHTTP(c.goc, c.token, c.nguong); err == nil {
			t.Errorf("accepted %+v", c)
		}
	}
}
