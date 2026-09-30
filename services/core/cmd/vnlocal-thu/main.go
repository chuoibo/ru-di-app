// Command vnlocal-thu checks this machine's access to the vnlocal machine
// (vnlocal HANDOFF-KET-NOI.md) and sends a few sample requests to agy-proxy.
//
//	vnlocal-thu ket-noi   Postgres (feed + app DB), MinIO, agy-proxy auth
//	vnlocal-thu agy       one run, JSON schema, Google Search, chat, one agent loop
//	vnlocal-thu bench [-models a,b] [-n 5] [-kieu ngan,json,dai,search] [-song-song 1]
//	                      latency per model × prompt kind, models interleaved per round
//	vnlocal-thu anh [-thu-muc DIR] [-models a,b]
//	                      inline images + response schema through the Go model door
//
// Secrets come from the environment only and are never printed.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"mobile/services/core/internal/agyproxy"
	"mobile/services/core/internal/db"
	"mobile/services/core/internal/ingest"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Getenv, os.Stdout))
}

func run(ctx context.Context, args []string, getenv func(string) string, out io.Writer) int {
	if len(args) >= 1 && args[0] == "bench" {
		return bench(ctx, args[1:], getenv, out)
	}
	if len(args) >= 1 && args[0] == "anh" {
		return anh(ctx, args[1:], getenv, out)
	}
	if len(args) != 1 {
		fmt.Fprintln(out, "dùng: vnlocal-thu ket-noi | agy | bench | anh")
		return 2
	}
	var failed int
	check := func(name string, f func() (string, error)) {
		started := time.Now()
		detail, err := f()
		took := time.Since(started).Round(time.Millisecond)
		if err != nil {
			failed++
			fmt.Fprintf(out, "ĐỎ   %-26s %v (%s)\n", name, err, took)
			return
		}
		fmt.Fprintf(out, "XANH %-26s %s (%s)\n", name, detail, took)
	}
	switch args[0] {
	case "ket-noi":
		ketNoi(ctx, getenv, check)
	case "agy":
		agy(ctx, getenv, check)
	default:
		fmt.Fprintf(out, "lệnh lạ: %s\n", args[0])
		return 2
	}
	if failed > 0 {
		fmt.Fprintf(out, "%d mục đỏ\n", failed)
		return 1
	}
	return 0
}

func ketNoi(ctx context.Context, getenv func(string) string, check func(string, func() (string, error))) {
	var frameKey string
	check("postgres nguồn (vnlocal)", func() (string, error) {
		dsn := getenv("VNLOCAL_PG_DSN")
		if dsn == "" {
			return "", errors.New("thiếu VNLOCAL_PG_DSN")
		}
		pool, err := db.Open(ctx, dsn)
		if err != nil {
			return "", err
		}
		defer pool.Close()
		var who string
		var places int
		var synced time.Time
		err = pool.QueryRow(ctx, `SELECT current_user, count(*), max(synced_at) FROM places`).Scan(&who, &places, &synced)
		if err != nil {
			return "", err
		}
		// A frame the feed says is in MinIO, for the bucket check below.
		_ = pool.QueryRow(ctx, `SELECT storage_key FROM place_frames WHERE co_trong_minio LIMIT 1`).Scan(&frameKey)
		// The role must stay read-only: a write that succeeds is a finding.
		if _, err := pool.Exec(ctx, `CREATE TEMP TABLE vnlocal_thu_ghi (x int)`); err == nil {
			return "", errors.New("user " + who + " GHI ĐƯỢC — nguồn phải chỉ đọc")
		}
		return fmt.Sprintf("user %s · %d địa điểm · synced_at mới nhất %s · chỉ đọc", who, places, synced.UTC().Format(time.RFC3339)), nil
	})
	check("postgres app (rudi)", func() (string, error) {
		dsn := getenv("MOBILE_DATABASE_URL")
		if dsn == "" {
			return "", errors.New("thiếu MOBILE_DATABASE_URL")
		}
		pool, err := db.Open(ctx, dsn)
		if err != nil {
			return "", err
		}
		defer pool.Close()
		var who, database string
		var readOnly string
		if err := pool.QueryRow(ctx, `SELECT current_user, current_database(), current_setting('transaction_read_only')`).Scan(&who, &database, &readOnly); err != nil {
			return "", err
		}
		if readOnly == "on" {
			return "", errors.New("DB app đang read-only")
		}
		return fmt.Sprintf("user %s · db %s · ghi được", who, database), nil
	})
	check("minio có khoá", func() (string, error) {
		if frameKey == "" {
			return "", errors.New("không lấy được storage_key mẫu từ place_frames")
		}
		frames := ingest.S3Frames{
			Endpoint:  getenv("VNLOCAL_S3_ENDPOINT"),
			Bucket:    getenv("VNLOCAL_S3_FRAMES_BUCKET"),
			AccessKey: getenv("VNLOCAL_S3_ACCESS_KEY"),
			SecretKey: getenv("VNLOCAL_S3_SECRET"),
		}
		if frames.Bucket == "" {
			frames.Bucket = "vnlocal-frames"
		}
		if frames.Endpoint == "" || frames.AccessKey == "" || frames.SecretKey == "" {
			return "", errors.New("thiếu VNLOCAL_S3_ENDPOINT / _ACCESS_KEY / _SECRET")
		}
		raw, err := frames.Read(ctx, frameKey)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("bucket %s · đọc 1 ảnh %d byte (%s)", frames.Bucket, len(raw), http.DetectContentType(raw)), nil
	})
	check("minio ẩn danh bị chặn", func() (string, error) {
		endpoint := strings.TrimRight(getenv("VNLOCAL_S3_ENDPOINT"), "/")
		bucket := getenv("VNLOCAL_S3_FRAMES_BUCKET")
		if bucket == "" {
			bucket = "vnlocal-frames"
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"/"+bucket+"/"+frameKey, nil)
		if err != nil {
			return "", err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", err
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			return "", fmt.Errorf("mong 403, nhận %d", resp.StatusCode)
		}
		return "403 như handoff nói", nil
	})
	agyAuth(ctx, getenv, check)
}

func agyAuth(ctx context.Context, getenv func(string) string, check func(string, func() (string, error))) {
	check("agy /health", func() (string, error) {
		c, err := agyproxy.FromEnv(getenv)
		if err != nil {
			return "", err
		}
		return "ok", c.Health(ctx)
	})
	check("agy khoá đúng", func() (string, error) {
		c, err := agyproxy.FromEnv(getenv)
		if err != nil {
			return "", err
		}
		ids, err := c.Models(ctx)
		if err != nil {
			return "", err
		}
		return strings.Join(ids, ", "), nil
	})
	check("agy khoá sai bị 401", func() (string, error) {
		c, err := agyproxy.New(getenv(agyproxy.EnvURL), "vnl_sai")
		if err != nil {
			return "", err
		}
		_, err = c.Models(ctx)
		var e *agyproxy.Error
		if errors.As(err, &e) && e.Unauthorized() {
			return "401", nil
		}
		return "", fmt.Errorf("mong 401, nhận %v", err)
	})
}

func agy(ctx context.Context, getenv func(string) string, check func(string, func() (string, error))) {
	c, err := agyproxy.FromEnv(getenv)
	if err != nil {
		check("agy cấu hình", func() (string, error) { return "", err })
		return
	}
	today := "Hôm nay là " + time.Now().Format("02/01/2006") + "."

	check("run một lượt (low)", func() (string, error) {
		r, err := c.Run(ctx, agyproxy.RunRequest{
			Model:             agyproxy.ModelLow,
			SystemInstruction: "Trả lời một câu ngắn. " + today,
			Input:             "Thủ đô của Việt Nam là gì?",
		})
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%q · run %s · usage %v", short(r.Text), r.RunID, r.Usage), nil
	})
	check("run JSON schema", func() (string, error) {
		schema := json.RawMessage(`{"type":"object","properties":{"mon":{"type":"array","items":{"type":"object","properties":{"ten":{"type":"string"},"vung":{"type":"string"}},"required":["ten","vung"]}}},"required":["mon"]}`)
		r, err := c.Run(ctx, agyproxy.RunRequest{
			Model:      agyproxy.ModelLow,
			Input:      "Liệt kê đúng 3 món đặc sản miền Tây.",
			JSONSchema: schema,
		})
		if err != nil {
			return "", err
		}
		var got struct {
			Mon []struct{ Ten, Vung string } `json:"mon"`
		}
		if err := json.Unmarshal(r.JSON, &got); err != nil {
			return "", fmt.Errorf("json không khớp schema: %w (%s)", err, short(string(r.JSON)))
		}
		if len(got.Mon) == 0 {
			return "", errors.New("json rỗng: " + short(string(r.JSON)))
		}
		return short(string(r.JSON)), nil
	})
	check("run Google Search", func() (string, error) {
		r, err := c.Run(ctx, agyproxy.RunRequest{
			Model:             agyproxy.ModelMedium,
			SystemInstruction: today,
			Input:             "Giá xăng RON95-III kỳ điều chỉnh gần nhất ở Việt Nam là bao nhiêu?",
			GoogleSearch:      true,
		})
		if err != nil {
			return "", err
		}
		if r.Grounding == nil || len(r.Grounding.Queries) == 0 {
			return "", errors.New("không tra web (grounding.queries rỗng): " + short(r.Text))
		}
		return fmt.Sprintf("%d truy vấn · %d nguồn · %q", len(r.Grounding.Queries), len(r.Grounding.Sources), short(r.Text)), nil
	})
	check("chat kiểu OpenAI", func() (string, error) {
		zero := 0.0
		r, err := c.Chat(ctx, agyproxy.ChatRequest{
			Model: agyproxy.ModelLow,
			Messages: []agyproxy.Message{
				{Role: "system", Content: "Trả lời bằng đúng một từ."},
				{Role: "user", Content: "Màu của lá cây thường là gì?"},
			},
			Temperature: &zero,
			// Thinking tokens count against max_tokens: a small cap spends it
			// all on thinking and returns "" with finish_reason length.
			MaxTokens: 2000,
		})
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(r.Content) == "" {
			return "", fmt.Errorf("nội dung rỗng · finish %s · usage %v", r.FinishReason, r.Usage)
		}
		return fmt.Sprintf("%q · finish %s", short(r.Content), r.FinishReason), nil
	})
	check("agent gọi tool", func() (string, error) {
		req := agyproxy.AgentRequest{
			Model: agyproxy.ModelMedium,
			Input: "Đà Lạt và Sa Pa hôm nay nơi nào lạnh hơn? Dùng tool thoi_tiet cho từng nơi.",
			Tools: []agyproxy.Tool{{
				Name:        "thoi_tiet",
				Description: "Nhiệt độ hiện tại của một thành phố, độ C",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"tp":{"type":"string"}},"required":["tp"]}`),
			}},
		}
		// Canned tool: the check is the loop, not the weather.
		fake := map[string]float64{"Đà Lạt": 17, "Sa Pa": 12}
		var calls []string
		for step := 0; step < 4; step++ {
			s, err := c.Agent(ctx, req)
			if err != nil {
				return "", fmt.Errorf("bước %d: %w", step+1, err)
			}
			if s.Done {
				if len(calls) == 0 {
					return "", errors.New("xong mà không gọi tool: " + short(s.Text))
				}
				return fmt.Sprintf("%d bước · gọi %s · %q", step+1, strings.Join(calls, ", "), short(s.Text)), nil
			}
			var results []agyproxy.FunctionResult
			for _, fc := range s.FunctionCalls {
				tp, _ := fc.Args["tp"].(string)
				calls = append(calls, fc.Name+"("+tp+")")
				temp, ok := fake[tp]
				if !ok {
					temp = 20
				}
				results = append(results, agyproxy.FunctionResult{Name: fc.Name, Response: map[string]any{"tp": tp, "nhiet_do_c": temp}})
			}
			if req, err = agyproxy.Continue(req, s, results); err != nil {
				return "", err
			}
		}
		return "", errors.New("quá 4 bước chưa xong")
	})
}

func short(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 140 {
		return string(r[:140]) + "…"
	}
	return s
}
