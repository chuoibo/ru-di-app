package routes

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
	"time"

	"google.golang.org/genai"

	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/motluot"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/pyjson"
)

// A small opaque PNG: the sanitiser re-encodes it as JPEG.
func anhThu(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 24, 16))
	for x := 0; x < 24; x++ {
		for y := 0; y < 16; y++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 10), G: uint8(y * 10), B: 90, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func maTuChoi(t *testing.T, err error) (int, string, string) {
	t.Helper()
	var r *endpoint.Refusal
	if !errors.As(err, &r) {
		t.Fatalf("not a refusal: %v", err)
	}
	return r.Problem.Status, r.Problem.Code, r.Problem.Detail
}

const billDoc = `{"document_type":"receipt","items":[{"name":"Cơm gà","quantity_text":"2","unit_price_text":"45.000","line_total_text":"90.000"},{"name":"Trà đá","quantity_text":"x3","unit_price_text":null,"line_total_text":"15.000"}],"total_text":"105.000 đ","confidence":0.97}`

// The upload is checked before any model, in Python's order, and a keyless
// process refuses only after the checks: the same answers a keyless stack
// has always given.
func TestDocHoaDonKiemTruocKhiGoiModel(t *testing.T) {
	anh := anhThu(t)
	for _, c := range []struct {
		name, mime string
		body       []byte
		status     int
		code       string
	}{
		{"rỗng", "image/png", nil, 422, "receipt_unreadable"},
		{"loại lạ", "image/heic", anh, 415, "unsupported_image_type"},
		{"quá 8 MB", "image/png", bytes.Repeat([]byte{1}, maxScanBytes+1), 413, "image_too_large"},
		{"không phải ảnh", "image/png", []byte("not an image at all"), 415, "unsupported_image_type"},
		{"không khoá", "image/png", anh, 503, "receipt_reader_not_configured"},
	} {
		_, err := docHoaDon(context.Background(), nil, c.body, c.mime)
		if status, code, _ := maTuChoi(t, err); status != c.status || code != c.code {
			t.Errorf("%s: %d %s, muốn %d %s", c.name, status, code, c.status, c.code)
		}
	}
}

// The model reads the re-encoded pixels under their real type, never the
// uploader's claim; its reading goes through domain/receipt.
func TestDocHoaDonGuiAnhDaLamSach(t *testing.T) {
	anh := anhThu(t)
	stub := llm.NewStub(llm.Buoc{Text: billDoc})
	may := motluot.Moi(stub, 1)
	r, err := docHoaDon(context.Background(), may, anh, "image/webp")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Items) != 2 || r.ItemsTotalVND != 105000 || *r.TotalVND != 105000 || !*r.TotalsAgree || r.NeedsReview || *r.Items[1].UnitPriceVND != 5000 {
		t.Fatalf("reading: %+v", r)
	}
	req := strings.Join(strings.Fields(string(stub.YeuCau()[0])), "")
	if strings.Contains(req, "image/webp") || !strings.Contains(req, `"mimeType":"image/jpeg"`) {
		t.Fatalf("the model was not handed the sanitised JPEG: %s", req)
	}
	if strings.Contains(req, base64.StdEncoding.EncodeToString(anh)) {
		t.Fatal("the model got the upload's own bytes")
	}
	body := wireReceiptScan(r)
	raw, _ := pyjson.Dumps(body)
	if strings.Contains(string(raw), "confidence") || !strings.HasPrefix(string(raw), `{"items": [{"name": "C\u01a1m g\u00e0", "quantity": 2, "unit_price_vnd": 45000, "line_total_vnd": 90000}`) {
		t.Fatalf("wire: %s", raw)
	}
}

func TestDocHoaDonTuChoiDongCua(t *testing.T) {
	anh := anhThu(t)
	for _, c := range []struct {
		name   string
		buoc   llm.Buoc
		status int
		code   string
		detail string
	}{
		{"thực đơn", llm.Buoc{Text: `{"document_type":"price_list","items":[],"total_text":null,"confidence":0.99}`}, 422, "not_a_receipt", "thực đơn"},
		{"không phải bill", llm.Buoc{Text: `{"document_type":"other","items":[],"total_text":null,"confidence":0.99}`}, 422, "not_a_receipt", "không phải hoá đơn"},
		{"mờ", llm.Buoc{Text: `{"document_type":"receipt","items":[],"total_text":null,"confidence":0.2}`}, 422, "receipt_too_blurry", ""},
		{"số lạ", llm.Buoc{Text: `{"document_type":"receipt","items":[{"name":"A","line_total_text":"một trăm","unit_price_text":null}],"total_text":null,"confidence":0.99}`}, 422, "receipt_unreadable", ""},
		{"không phải JSON", llm.Buoc{Text: "tôi không đọc được"}, 502, "receipt_reader_unavailable", ""},
		{"nhà cung cấp hỏng", llm.Buoc{Loi: genai.APIError{Code: 500}}, 502, "receipt_reader_unavailable", ""},
	} {
		may := motluot.Moi(llm.NewStub(c.buoc), 1).WithWait(func(int) time.Duration { return 0 })
		_, err := docHoaDon(context.Background(), may, anh, "image/png")
		status, code, detail := maTuChoi(t, err)
		if status != c.status || code != c.code || !strings.Contains(detail, c.detail) {
			t.Errorf("%s: %d %s %q", c.name, status, code, detail)
		}
	}
}

func TestDocManHinh(t *testing.T) {
	anh := anhThu(t)
	if _, err := docManHinh(context.Background(), nil, anh, "image/png"); err == nil {
		t.Fatal("keyless read")
	} else if status, code, _ := maTuChoi(t, err); status != 503 || code != "screenshot_reader_not_configured" {
		t.Fatalf("keyless: %d %s", status, code)
	}
	if _, err := docManHinh(context.Background(), nil, nil, "image/png"); err == nil {
		t.Fatal("empty read")
	} else if status, code, _ := maTuChoi(t, err); status != 422 || code != "screenshot_unreadable" {
		t.Fatalf("empty: %d %s", status, code)
	}
	ok := llm.Buoc{Text: `{"source":"banking","merchant":" Cửa hàng bánh ","total_text":"250.000 VND","occurred_on":"2026-09-15"}`}
	r, err := docManHinh(context.Background(), motluot.Moi(llm.NewStub(ok), 1), anh, "image/png")
	if err != nil || r.Source != "banking" || r.Merchant != "Cửa hàng bánh" || r.TotalVND != 250000 || *r.OccurredOn != "2026-09-15" {
		t.Fatalf("%+v %v", r, err)
	}
	for _, c := range []struct {
		answer, code string
		status       int
	}{
		{`{"source":"other","merchant":"x","total_text":"1","occurred_on":null}`, "not_a_transaction", 422},
		{`{"source":"banking","merchant":"x","total_text":"1k","occurred_on":null,"payer":"Minh"}`, "screenshot_model_named_a_person", 422},
		{`{"source":"banking","merchant":"x","total_text":250000,"occurred_on":null}`, "screenshot_unreadable", 422},
		{`{"source":["banking"],"merchant":"x","total_text":"1k","occurred_on":null}`, "screenshot_reader_unavailable", 502},
	} {
		_, err := docManHinh(context.Background(), motluot.Moi(llm.NewStub(llm.Buoc{Text: c.answer}), 1), anh, "image/png")
		if status, code, _ := maTuChoi(t, err); status != c.status || code != c.code {
			t.Errorf("%s: %d %s", c.answer, status, code)
		}
	}
}

func TestReadChatExpense(t *testing.T) {
	if _, err := readChatExpense(context.Background(), nil, "tao trả 300k"); err == nil {
		t.Fatal("keyless read")
	} else if status, code, _ := maTuChoi(t, err); status != 503 || code != "chat_reader_not_configured" {
		t.Fatalf("keyless: %d %s", status, code)
	}
	stub := llm.NewStub(llm.Buoc{Text: `{"is_expense":true,"title":" Tiền nước ","amount_text":"300k"}`})
	r, err := readChatExpense(context.Background(), motluot.Moi(stub, 1), "Tao trả 300k tiền nước <3")
	if err != nil || !r.isExpense || r.title != "Tiền nước" || r.amount != 300000 || !r.needsReview {
		t.Fatalf("%+v %v", r, err)
	}
	if req := string(stub.YeuCau()[0]); !strings.Contains(req, `SUPPLIED MESSAGE (JSON):\n{\"message_text\":\"Tao trả 300k tiền nước <3\"}`) {
		t.Fatalf("the message did not reach the model as data: %s", req)
	}
	for _, c := range []struct {
		answer, code string
		status       int
	}{
		{`{"is_expense":false}`, "", 0},
		{`{"is_expense":true,"title":"x","amount_text":"5k","paid_by":"Minh"}`, "chat_expense_model_named_a_person", 422},
		{`{"is_expense":true,"title":"x","amount_text":180000.0}`, "chat_expense_unreadable", 422},
		{`nope`, "chat_reader_unavailable", 502},
	} {
		r, err := readChatExpense(context.Background(), motluot.Moi(llm.NewStub(llm.Buoc{Text: c.answer}), 1), "x")
		if c.status == 0 {
			if err != nil || r.isExpense {
				t.Errorf("%s: %+v %v", c.answer, r, err)
			}
			continue
		}
		if status, code, _ := maTuChoi(t, err); status != c.status || code != c.code {
			t.Errorf("%s: %d %s", c.answer, status, code)
		}
	}
}
