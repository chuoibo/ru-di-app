package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/adk/v2/model"
	"google.golang.org/genai"

	"mobile/services/core/internal/agyproxy"
	"mobile/services/core/internal/aiharness/cautruc"
	"mobile/services/core/internal/aiharness/docbill"
	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/motluot"
)

// anh answers the one open question before the brain's image steps move to
// Go (ADR-0051): does agy-proxy take inline images with a response schema
// on the Gemini wire, how large a body, and which model reads a bill right.
// Fixtures come from scripts/sinh_anh_gia_ai.py (invented, outside Git).
//
//	vnlocal-thu anh [-thu-muc DIR] [-models a,b]
func anh(ctx context.Context, args []string, getenv func(string) string, out io.Writer) int {
	fs := flag.NewFlagSet("anh", flag.ContinueOnError)
	fs.SetOutput(out)
	home, _ := os.UserHomeDir()
	dir := fs.String("thu-muc", filepath.Join(home, ".cache/rudi-bang-chung/agy/anh"), "fixtures from scripts/sinh_anh_gia_ai.py")
	models := fs.String("models", llm.Model, "models to compare, comma-separated")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	y, err := motluot.TuEnv(ctx, getenv, 4)
	if err != nil || y == nil {
		fmt.Fprintf(out, "ĐỎ   không dựng được model: %v (cần %s và %s)\n", err, llm.EnvAgyURL, llm.EnvAgyKey)
		return 1
	}
	doc := func(name string) ([]byte, error) { return os.ReadFile(filepath.Join(*dir, name)) }
	var failed int
	check := func(name string, f func() (string, error)) {
		started := time.Now()
		detail, err := f()
		took := time.Since(started).Round(time.Millisecond)
		if err != nil {
			failed++
			fmt.Fprintf(out, "ĐỎ   %-34s %v (%s)\n", name, err, took)
			return
		}
		fmt.Fprintf(out, "XANH %-34s %s (%s)\n", name, detail, took)
	}

	check("agy liệt kê model", func() (string, error) {
		c, err := agyproxy.FromEnv(getenv)
		if err != nil {
			return "", err
		}
		names, err := c.Models(ctx)
		return strings.Join(names, ","), err
	})
	for _, m := range strings.Split(*models, ",") {
		m = strings.TrimSpace(m)
		goi := func(req *model.LLMRequest) (map[string]any, error) {
			req.Model = m
			text, err := y.Luot(3).Goi(ctx, req)
			if err != nil {
				return nil, err
			}
			return docbill.DocTraLoi(text)
		}
		check(m+" · hoá đơn", func() (string, error) {
			png, err := doc("hoa_don.png")
			if err != nil {
				return "", err
			}
			r, err := goi(docbill.YeuCau("image/png", png))
			if err != nil {
				return "", err
			}
			items, _ := r["items"].([]any)
			detail := fmt.Sprintf("type=%v total=%v items=%d conf=%v", r["document_type"], r["total_text"], len(items), r["confidence"])
			if r["document_type"] != docbill.LoaiHoaDon || !strings.Contains(fmt.Sprint(r["total_text"]), "165") || len(items) != 3 {
				return "", errors.New("đọc sai: " + detail)
			}
			return detail + " " + compact(items), nil
		})
		check(m+" · thực đơn", func() (string, error) {
			png, err := doc("thuc_don.png")
			if err != nil {
				return "", err
			}
			r, err := goi(docbill.YeuCau("image/png", png))
			if err != nil {
				return "", err
			}
			if r["document_type"] != docbill.LoaiThucDon {
				return "", fmt.Errorf("type=%v, phải là price_list", r["document_type"])
			}
			return fmt.Sprintf("type=%v", r["document_type"]), nil
		})
		check(m+" · ba ảnh một request", func() (string, error) {
			parts := []*genai.Part{{Text: "Mô tả mỗi ảnh một câu, theo đúng mã ảnh."}}
			for i := 1; i <= 3; i++ {
				jpg, err := doc(fmt.Sprintf("canh_%d.jpg", i))
				if err != nil {
					return "", err
				}
				parts = append(parts, &genai.Part{Text: fmt.Sprintf("Mã ảnh: a%d", i)}, cautruc.Anh("image/jpeg", jpg))
			}
			schema := &genai.Schema{Type: genai.TypeObject, Required: []string{"anh"}, Properties: map[string]*genai.Schema{
				"anh": {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeObject, Required: []string{"id", "mo_ta"}, Properties: map[string]*genai.Schema{
					"id": {Type: genai.TypeString}, "mo_ta": {Type: genai.TypeString},
				}}},
			}}
			r, err := goi(cautruc.YeuCauPhan(llm.BuocDocAnh, "Bạn mô tả ảnh.", parts, schema, 1024))
			if err != nil {
				return "", err
			}
			list, _ := r["anh"].([]any)
			if len(list) != 3 {
				return "", fmt.Errorf("%d mô tả, cần 3: %s", len(list), compact(r))
			}
			return compact(list), nil
		})
	}
	check("body ~15 MB inline", func() (string, error) {
		png, err := doc("nhieu_15mb.png")
		if err != nil {
			return "", err
		}
		schema := &genai.Schema{Type: genai.TypeObject, Required: []string{"mo_ta"}, Properties: map[string]*genai.Schema{"mo_ta": {Type: genai.TypeString}}}
		req := cautruc.YeuCauPhan(llm.BuocDocAnh, "Mô tả ảnh trong năm từ.", []*genai.Part{cautruc.Anh("image/png", png)}, schema, 256)
		text, err := y.Luot(1).Goi(ctx, req)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%d byte ảnh → %s", len(png), strings.TrimSpace(text)), nil
	})
	if failed > 0 {
		fmt.Fprintf(out, "%d mục đỏ\n", failed)
		return 1
	}
	return 0
}

func compact(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
