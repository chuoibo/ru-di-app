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
	"os/signal"
	"syscall"
	"time"

	"mobile/services/core/internal/db"
	"mobile/services/core/internal/ingest"
	"mobile/services/core/internal/media/storage"
)

func main() {
	os.Exit(run(os.Args[1:], os.Getenv, os.Stdout, os.Stderr))
}

func run(args []string, getenv func(string) string, stdout, stderr *os.File) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: rudi-ingest migrate|land|apply|photos|pull|sync|purge-dev [flags]")
		return 2
	}
	// SIGTERM/SIGINT cancel the context: a running round finishes its current
	// statement, its transaction rolls back, and the next start resumes from
	// the cursor that matches what is committed.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
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
		root := set.String("frames-root", "", "thư mục khung hình của nguồn (bỏ trống = đọc MinIO nguồn)")
		province := set.Int("province", 0, "chỉ nạp một tỉnh (0 = tất cả)")
		perPlace := set.Int("per-place", 3, "số ảnh tối đa mỗi địa điểm")
		if err := set.Parse(args[1:]); err != nil {
			return 2
		}
		if *batch == "" {
			fmt.Fprintln(stderr, "--batch là bắt buộc")
			return 2
		}
		var source ingest.FrameSource = ingest.DirFrames{Root: *root}
		if *root == "" {
			s3, err := s3Frames(getenv)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 2
			}
			source = s3
		}
		// The same root the API serves bytes from. Written anywhere else, the
		// rows would point at objects no reader can find.
		store, err := storage.New()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		opt := ingest.PhotoOptions{BatchID: *batch, Source: source, PerPlace: *perPlace}
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

	case "pull", "sync":
		set := flag.NewFlagSet(args[0], flag.ContinueOnError)
		every := set.Duration("every", 0, "sync: chạy lặp mỗi khoảng này (0 = một vòng rồi thoát)")
		perPlace := set.Int("per-place", 3, "số ảnh tối đa mỗi địa điểm")
		maxRows := set.Int("batch-rows", 2000, "số dòng tối đa mỗi đợt")
		if err := set.Parse(args[1:]); err != nil {
			return 2
		}
		srcDSN := getenv("VNLOCAL_PG_DSN")
		if srcDSN == "" {
			fmt.Fprintln(stderr, "VNLOCAL_PG_DSN is required")
			return 1
		}
		src, err := db.Open(ctx, srcDSN)
		if err != nil {
			fmt.Fprintln(stderr, "feed database:", err)
			return 1
		}
		defer src.Close()
		feed := ingest.PGFeed{Pool: src}
		opt := ingest.SyncOptions{Pull: ingest.PullOptions{MaxRows: *maxRows}, PerPlace: *perPlace}

		if args[0] == "pull" {
			// Pull and apply only; photographs are `sync`'s job.
			report, err := ingest.SyncOnce(ctx, pool, feed, nil, nil, opt)
			if err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			fmt.Fprintln(stdout, report)
			return 0
		}

		frames, err := s3Frames(getenv)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		store, err := storage.New()
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		opt.Photos = true
		for {
			started := time.Now()
			report, err := ingest.SyncOnce(ctx, pool, feed, frames, store, opt)
			stamp := started.UTC().Format(time.RFC3339)
			if err != nil {
				// A daemon logs and waits for the next round: the feed's
				// machine rebooting is not a reason to stop syncing forever.
				fmt.Fprintf(stderr, "%s lỗi sau %s: %v\n", stamp, time.Since(started).Round(time.Second), err)
				if *every == 0 {
					return 1
				}
			} else {
				fmt.Fprintf(stdout, "%s %s · %s\n", stamp, report, time.Since(started).Round(time.Second))
			}
			if *every == 0 {
				return 0
			}
			select {
			case <-ctx.Done():
				return 0
			case <-time.After(*every):
			}
		}

	case "purge-dev":
		set := flag.NewFlagSet("purge-dev", flag.ContinueOnError)
		commit := set.Bool("apply", false, "ghi thật; mặc định chỉ chạy thử rồi hoàn tác")
		if err := set.Parse(args[1:]); err != nil {
			return 2
		}
		// Always run for real inside a transaction, then keep or roll back: a
		// dry run that counted with SELECTs could disagree with what the
		// DELETEs would actually do.
		tx, err := pool.Begin(ctx)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		defer tx.Rollback(ctx)
		report, err := ingest.PurgeDevCatalogue(ctx, tx)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		mode := "chạy thử, đã hoàn tác (thêm --apply để ghi)"
		if *commit {
			if err := tx.Commit(ctx); err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			mode = "đã ghi"
		}
		fmt.Fprintf(stdout, "%s\n  địa điểm seed %d · ảnh %d (xếp hàng xoá file %d)\n"+
			"  bookmark %d · chặng bỏ liên kết %d · kỷ niệm bỏ liên kết %d · điểm đến cũ %d\n",
			mode, report.SeedPlaces, report.Photos, report.QueuedObjects,
			report.SavedPlaces, report.OutingStops, report.Memories, report.Destinations)
		return 0
	}
	fmt.Fprintf(stderr, "lệnh lạ: %s\n", args[0])
	return 2
}

// s3Frames reads the feed's frame bucket with the credentials the vnlocal
// machine issued (read-only on that bucket).
func s3Frames(getenv func(string) string) (ingest.S3Frames, error) {
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
		return frames, fmt.Errorf("VNLOCAL_S3_ENDPOINT, VNLOCAL_S3_ACCESS_KEY and VNLOCAL_S3_SECRET are required")
	}
	return frames, nil
}
