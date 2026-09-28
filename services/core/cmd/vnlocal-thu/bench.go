package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"mobile/services/core/internal/agyproxy"
)

// benchKinds are the prompt shapes the bench can send. Each is a fixed input,
// so every model answers the same question.
var benchKinds = map[string]agyproxy.RunRequest{
	"ngan": {
		SystemInstruction: "Trả lời một câu ngắn.",
		Input:             "Thủ đô của Việt Nam là gì?",
	},
	"json": {
		Input:      "Liệt kê đúng 3 món đặc sản miền Tây, mỗi món kèm tỉnh.",
		JSONSchema: json.RawMessage(`{"type":"object","properties":{"mon":{"type":"array","items":{"type":"object","properties":{"ten":{"type":"string"},"tinh":{"type":"string"}},"required":["ten","tinh"]}}},"required":["mon"]}`),
	},
	"dai": {
		SystemInstruction: "Bạn là trợ lý du lịch. Viết tiếng Việt, khoảng 200 chữ.",
		Input:             "Gợi ý lịch trình một buổi tối ở Sài Gòn cho hai người bạn thích đồ ăn đường phố và cà phê yên tĩnh.",
	},
	"search": {
		Input:        "Giá xăng RON95-III kỳ điều chỉnh gần nhất ở Việt Nam là bao nhiêu?",
		GoogleSearch: true,
	},
}

type benchSample struct {
	model, kind string
	took        time.Duration
	out, think  float64
	err         error
}

func bench(ctx context.Context, args []string, getenv func(string) string, out io.Writer) int {
	set := flag.NewFlagSet("bench", flag.ContinueOnError)
	set.SetOutput(out)
	models := set.String("models", "gemini-3.5-flash-lite,"+agyproxy.ModelLow+","+agyproxy.ModelMedium, "các model, phân cách dấu phẩy")
	n := set.Int("n", 5, "số lượt mỗi model × kiểu")
	kinds := set.String("kieu", "ngan,json,dai", "kiểu prompt: ngan, json, dai, search")
	parallel := set.Int("song-song", 1, "số request chạy cùng lúc (proxy cho tối đa 8 mỗi client)")
	if err := set.Parse(args); err != nil {
		return 2
	}
	c, err := agyproxy.FromEnv(getenv)
	if err != nil {
		fmt.Fprintln(out, err)
		return 1
	}
	modelList := strings.Split(*models, ",")
	kindList := strings.Split(*kinds, ",")
	for _, k := range kindList {
		if _, ok := benchKinds[k]; !ok {
			fmt.Fprintf(out, "kiểu lạ: %s\n", k)
			return 2
		}
	}
	today := "Hôm nay là " + time.Now().Format("02/01/2006") + ". "

	// Round-major order: each round asks every model every kind once, so a
	// slow minute on the shared proxy lands on all models alike.
	type job struct{ model, kind string }
	var jobs []job
	for round := 0; round < *n; round++ {
		for _, k := range kindList {
			for _, m := range modelList {
				jobs = append(jobs, job{m, k})
			}
		}
	}
	fmt.Fprintf(out, "%d request · %d model × %d kiểu × %d lượt · song song %d\n",
		len(jobs), len(modelList), len(kindList), *n, *parallel)

	samples := make([]benchSample, len(jobs))
	next := make(chan int)
	var wg sync.WaitGroup
	var mu sync.Mutex
	done := 0
	started := time.Now()
	for w := 0; w < max(1, *parallel); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range next {
				j := jobs[i]
				req := benchKinds[j.kind]
				req.Model = j.model
				req.SystemInstruction = today + req.SystemInstruction
				t0 := time.Now()
				r, err := c.Run(ctx, req)
				s := benchSample{model: j.model, kind: j.kind, took: time.Since(t0), err: err}
				if err == nil {
					s.out, _ = r.Usage["output_tokens"].(float64)
					s.think, _ = r.Usage["thinking_tokens"].(float64)
					total, _ := r.Usage["total_tokens"].(float64)
					// A retired model answers 200 with a notice as text and zero
					// usage (seen: gemini-3.5-flash). That is not an answer.
					if total == 0 {
						s.err = errors.New("200 nhưng usage = 0: " + short(r.Text))
					}
				}
				samples[i] = s
				mu.Lock()
				done++
				mark := "."
				if s.err != nil {
					mark = "x"
				}
				fmt.Fprint(out, mark)
				if done%50 == 0 {
					fmt.Fprintf(out, " %d\n", done)
				}
				mu.Unlock()
			}
		}()
	}
	for i := range jobs {
		select {
		case next <- i:
		case <-ctx.Done():
		}
	}
	close(next)
	wg.Wait()
	fmt.Fprintf(out, "\nxong sau %s\n\n", time.Since(started).Round(time.Second))

	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', tabwriter.AlignRight)
	fmt.Fprintln(tw, "kiểu\tmodel\tok/n\tp50\tp90\tmin\tmax\tTB\ttok ra\ttok nghĩ\t")
	errs := map[string]int{}
	for _, k := range kindList {
		for _, m := range modelList {
			var took []time.Duration
			var outTok, thinkTok float64
			total := 0
			for _, s := range samples {
				if s.model != m || s.kind != k {
					continue
				}
				total++
				if s.err != nil {
					errs[m+" · "+k+" · "+short(s.err.Error())]++
					continue
				}
				took = append(took, s.took)
				outTok += s.out
				thinkTok += s.think
			}
			row := fmt.Sprintf("%s\t%s\t%d/%d\t", k, m, len(took), total)
			if len(took) == 0 {
				fmt.Fprintln(tw, row+"-\t-\t-\t-\t-\t-\t-\t")
				continue
			}
			sort.Slice(took, func(a, b int) bool { return took[a] < took[b] })
			var sum time.Duration
			for _, d := range took {
				sum += d
			}
			ok := float64(len(took))
			fmt.Fprintf(tw, "%s%s\t%s\t%s\t%s\t%s\t%.0f\t%.0f\t\n", row,
				sec(pct(took, 50)), sec(pct(took, 90)), sec(took[0]), sec(took[len(took)-1]),
				sec(sum/time.Duration(len(took))), outTok/ok, thinkTok/ok)
		}
	}
	tw.Flush()
	if len(errs) > 0 {
		fmt.Fprintln(out, "\nlỗi:")
		for e, count := range errs {
			fmt.Fprintf(out, "  %d× %s\n", count, e)
		}
		return 1
	}
	return 0
}

// pct is the nearest-rank percentile of sorted durations.
func pct(sorted []time.Duration, p int) time.Duration {
	i := (p*len(sorted)+99)/100 - 1
	return sorted[max(0, min(i, len(sorted)-1))]
}

func sec(d time.Duration) string { return fmt.Sprintf("%.2fs", d.Seconds()) }
