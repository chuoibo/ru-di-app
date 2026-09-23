// Command rudi-ingest loads a delivered place catalogue into the database.
//
// Two steps, run separately on purpose. `land` verifies a delivery and stores
// it word for word; `apply` reads that back and projects it into the
// catalogue. Keeping them apart is what lets a corrected mapping rule be
// replayed without asking the other side for the file again.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"mobile/services/core/internal/db"
	"mobile/services/core/internal/ingest"
	"mobile/services/core/internal/media/storage"
)

func main() {
	os.Exit(run(os.Args[1:], os.Getenv, os.Stdout, os.Stderr))
}

func run(args []string, getenv func(string) string, stdout, stderr *os.File) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: rudi-ingest migrate|land|apply [flags]")
		return 2
	}
	ctx := context.Background()
	dsn := getenv(db.EnvDatabaseURL)
	if dsn == "" {
		fmt.Fprintf(stderr, "%s is required\n", db.EnvDatabaseURL)
		return 1
	}
	pool, err := db.Open(ctx, dsn)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	defer pool.Close()

	switch args[0] {
	case "migrate":
		if err := ingest.Migrate(ctx, pool); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		provinces, err := ingest.SeedProvinces(ctx, pool)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		destinations, err := ingest.SeedProvinceDestinations(ctx, pool)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintf(stdout, "migrated · %d tỉnh · %d điểm đến cấp tỉnh\n",
			provinces, destinations)
		return 0

	case "land":
		set := flag.NewFlagSet("land", flag.ContinueOnError)
		manifest := set.String("manifest", "", "đường dẫn tới manifest json")
		if err := set.Parse(args[1:]); err != nil {
			return 2
		}
		if *manifest == "" {
			fmt.Fprintln(stderr, "--manifest là bắt buộc")
			return 2
		}
		m, err := ingest.ReadManifest(*manifest)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		result, err := ingest.Land(ctx, pool, m)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if result.AlreadyLanded {
			fmt.Fprintf(stdout, "đợt %s đã hạ rồi, không ghi gì\n", result.BatchID)
			return 0
		}
		fmt.Fprintf(stdout, "hạ %d dòng · từ chối %d · khai quá %d\n",
			result.Landed, result.RejectedTotal(), len(result.Overclaimed))
		for _, name := range result.DriftNames() {
			fmt.Fprintf(stdout, "  TRÔI %s: %d dòng\n", name, result.Drift[name])
		}
		return 0

	case "apply":
		set := flag.NewFlagSet("apply", flag.ContinueOnError)
		batch := set.String("batch", "", "mã đợt")
		if err := set.Parse(args[1:]); err != nil {
			return 2
		}
		if *batch == "" {
			fmt.Fprintln(stderr, "--batch là bắt buộc")
			return 2
		}
		result, err := ingest.Apply(ctx, pool, *batch)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintf(stdout, "thêm %d · cập nhật %d · bỏ vì cũ hơn %d · từ chối %d · bài %d\n",
			result.Inserted, result.Updated, result.Stale,
			result.RejectedTotal(), result.Posts)
		for _, reason := range result.Reasons() {
			fmt.Fprintf(stdout, "  %s: %d\n", reason, result.Rejected[reason])
		}
		return 0

	case "photos":
		set := flag.NewFlagSet("photos", flag.ContinueOnError)
		batch := set.String("batch", "", "mã đợt đã hạ")
		root := set.String("frames-root", "", "thư mục khung hình của nguồn")
		province := set.Int("province", 0, "chỉ nạp một tỉnh (0 = tất cả)")
		perPlace := set.Int("per-place", 3, "số ảnh tối đa mỗi địa điểm")
		if err := set.Parse(args[1:]); err != nil {
			return 2
		}
		if *batch == "" || *root == "" {
			fmt.Fprintln(stderr, "--batch và --frames-root là bắt buộc")
			return 2
		}
		// The same root the API serves bytes from. Written anywhere else, the
		// rows would point at objects no reader can find.
		store, err := storage.New()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		opt := ingest.PhotoOptions{BatchID: *batch, FramesRoot: *root, PerPlace: *perPlace}
		if *province > 0 {
			code := int16(*province)
			opt.ProvinceCode = &code
		}
		result, err := ingest.ImportPhotos(ctx, pool, store, opt)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintf(stdout, "địa điểm %d · ảnh mới %d · đã có %d\n",
			result.Places, result.Written, result.Existing)
		for reason, count := range result.Skipped {
			fmt.Fprintf(stdout, "  bỏ qua %s: %d\n", reason, count)
		}
		return 0
	}
	fmt.Fprintf(stderr, "lệnh lạ: %s\n", args[0])
	return 2
}
