package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mobile/services/core/internal/aiharness/docanh"
	"mobile/services/core/internal/aiharness/docbill"
	"mobile/services/core/internal/aiharness/dockhoan"
	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/motluot"
	"mobile/services/core/internal/domain/chatexpense"
	"mobile/services/core/internal/domain/receipt"
	"mobile/services/core/internal/domain/screenshot"
	"mobile/services/core/internal/media/sanitize"
)

// tinhNang sends each ported one-shot AI step (ADR-0051) one real request
// through the Go model door (agy-proxy when AGY_PROXY_URL is set), on
// invented data only, and checks what the Go domain code makes of the
// answer: the same packages the routes call, in the same order. Images come
// from scripts/sinh_anh_gia_ai.py, outside Git.
//
//	vnlocal-thu tinh-nang [-thu-muc DIR] [-chi ten,ten]
func tinhNang(ctx context.Context, args []string, getenv func(string) string, out io.Writer) int {
	fs := flag.NewFlagSet("tinh-nang", flag.ContinueOnError)
	fs.SetOutput(out)
	home, _ := os.UserHomeDir()
	dir := fs.String("thu-muc", filepath.Join(home, ".cache/rudi-bang-chung/agy/anh"), "fixtures from scripts/sinh_anh_gia_ai.py")
	chi := fs.String("chi", "", "only these checks, comma-separated")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	may, err := motluot.TuEnv(ctx, getenv, 4)
	if err != nil || may == nil {
		fmt.Fprintf(out, "ĐỎ   không dựng được model: %v (cần %s và %s)\n", err, llm.EnvAgyURL, llm.EnvAgyKey)
		return 1
	}
	chon := map[string]bool{}
	for _, c := range strings.Split(*chi, ",") {
		if c = strings.TrimSpace(c); c != "" {
			chon[c] = true
		}
	}
	var failed int
	check := func(name string, f func(l *motluot.Luot) (string, error)) {
		if len(chon) > 0 && !chon[name] {
			return
		}
		l := may.Luot(3)
		started := time.Now()
		detail, err := f(l)
		took := time.Since(started).Round(time.Millisecond)
		if err != nil {
			failed++
			fmt.Fprintf(out, "ĐỎ   %-22s %v (%s, %d lời gọi)\n", name, err, took, l.SoGoi())
			return
		}
		fmt.Fprintf(out, "XANH %-22s %s (%s, %d lời gọi, %d token vào)\n", name, detail, took, l.SoGoi(), l.Token().In)
	}
	anh := func(name string) (sanitize.Sanitized, error) {
		raw, err := os.ReadFile(filepath.Join(*dir, name))
		if err != nil {
			return sanitize.Sanitized{}, err
		}
		return sanitize.Sanitize(raw)
	}

	check("hoa-don", func(l *motluot.Luot) (string, error) {
		s, err := anh("hoa_don.png")
		if err != nil {
			return "", err
		}
		raw, err := docbill.Doc(ctx, l, s.ContentType, s.Data)
		if err != nil {
			return "", err
		}
		r, err := receipt.ReadScannedDocument(raw)
		if err != nil {
			return "", err
		}
		if len(r.Items) != 3 || r.ItemsTotalVND != 165000 || r.TotalVND == nil || *r.TotalVND != 165000 || !*r.TotalsAgree {
			return "", fmt.Errorf("đọc sai: %d món, tổng dòng %d", len(r.Items), r.ItemsTotalVND)
		}
		return fmt.Sprintf("3 món, tổng 165.000đ khớp, cần xem lại=%v", r.NeedsReview), nil
	})
	check("thuc-don", func(l *motluot.Luot) (string, error) {
		s, err := anh("thuc_don.png")
		if err != nil {
			return "", err
		}
		raw, err := docbill.Doc(ctx, l, s.ContentType, s.Data)
		if err != nil {
			return "", err
		}
		_, err = receipt.ReadScannedDocument(raw)
		var refused *receipt.Error
		if !errors.As(err, &refused) || refused.Code != "NOT_A_RECEIPT_PRICE_LIST" {
			return "", fmt.Errorf("phải từ chối là thực đơn, nhận %v", err)
		}
		return "từ chối NOT_A_RECEIPT_PRICE_LIST", nil
	})
	check("chuyen-khoan", func(l *motluot.Luot) (string, error) {
		s, err := anh("chuyen_khoan.png")
		if err != nil {
			return "", err
		}
		raw, err := docanh.Doc(ctx, l, s.ContentType, s.Data)
		if err != nil {
			return "", err
		}
		r, err := screenshot.Read(raw)
		if err != nil {
			return "", err
		}
		if r.Source != "banking" || r.TotalVND != 250000 {
			return "", fmt.Errorf("đọc sai: %s %d", r.Source, r.TotalVND)
		}
		day := "không ngày"
		if r.OccurredOn != nil {
			day = *r.OccurredOn
		}
		return fmt.Sprintf("%s · %s · 250.000đ · %s", r.Source, r.Merchant, day), nil
	})
	check("khoan-chi", func(l *motluot.Luot) (string, error) {
		raw, err := dockhoan.Doc(ctx, l, "Tao trả 300k tiền nước hôm qua nhé")
		if err != nil {
			return "", err
		}
		r, err := chatexpense.Read(raw)
		if err != nil {
			return "", err
		}
		if !r.IsExpense || r.AmountVND != 300000 {
			return "", fmt.Errorf("đọc sai: %+v", r)
		}
		return fmt.Sprintf("«%s» 300.000đ", r.Title), nil
	})
	check("khong-phai-khoan", func(l *motluot.Luot) (string, error) {
		raw, err := dockhoan.Doc(ctx, l, "Tối nay mọi người rảnh không, đi cà phê nhé")
		if err != nil {
			return "", err
		}
		r, err := chatexpense.Read(raw)
		if err != nil {
			return "", err
		}
		if r.IsExpense {
			return "", fmt.Errorf("một lời rủ bị đọc thành khoản chi: %+v", r)
		}
		return "không phải khoản chi", nil
	})
	if failed > 0 {
		fmt.Fprintf(out, "%d mục đỏ\n", failed)
		return 1
	}
	return 0
}
