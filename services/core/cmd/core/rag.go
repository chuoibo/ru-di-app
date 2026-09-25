package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	"mobile/services/core/internal/db"
	"mobile/services/core/internal/rag"
)

// ragUsage is `core rag` without a valid subcommand.
const ragUsage = "usage: core rag build | eval <version> | promote <version> | rollback | status | tombstone <doc_id> <unsafe|takedown|closed|source_deleted>"

// ragCommand is one parsed `core rag` call.
type ragCommand struct {
	name    string
	version int64
	docID   string
	reason  string
}

// parseRag reads the arguments before any database is opened, so a typo
// costs no connection and says what was wrong.
func parseRag(args []string) (ragCommand, error) {
	if len(args) == 0 {
		return ragCommand{}, errors.New(ragUsage)
	}
	c := ragCommand{name: args[0]}
	switch c.name {
	case "build", "rollback", "status":
		if len(args) != 1 {
			return ragCommand{}, errors.New(ragUsage)
		}
	case "eval", "promote":
		if len(args) != 2 {
			return ragCommand{}, errors.New(ragUsage)
		}
		v, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil || v <= 0 {
			return ragCommand{}, fmt.Errorf("version must be a positive integer; %s", ragUsage)
		}
		c.version = v
	case "tombstone":
		if len(args) != 3 {
			return ragCommand{}, errors.New(ragUsage)
		}
		c.docID, c.reason = args[1], args[2]
	default:
		return ragCommand{}, errors.New(ragUsage)
	}
	return c, nil
}

// runRag is `core rag`: the retrieval index's lifecycle, run by an operator,
// never by a request (design 04 §2.4). Every command prints one JSON object
// of ids, states and counts; none prints a word of the catalogue.
func runRag(args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	c, err := parseRag(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	pool, err := db.Open(ctx, getenv(db.EnvDatabaseURL))
	if err != nil {
		fmt.Fprintln(stderr, "rag: invalid database configuration")
		return 1
	}
	defer pool.Close()
	ok, err := rag.Installed(ctx, pool)
	if err != nil {
		fmt.Fprintln(stderr, "rag:", err)
		return 1
	}
	if !ok && c.name != "status" {
		fmt.Fprintln(stderr, "rag: the retrieval schema is missing; run `core migrate-rag` first")
		return 1
	}
	var out any
	switch c.name {
	case "build":
		out, err = rag.Build(ctx, pool)
	case "eval":
		var g rag.DanhGia
		g, err = rag.Evaluate(ctx, pool, c.version)
		out = g
		if err == nil && !g.Dat {
			printJSON(stdout, g)
			fmt.Fprintln(stderr, "rag: evaluation failed; the version is marked failed")
			return 1
		}
	case "promote":
		err = rag.Promote(ctx, pool, c.version)
		out = map[string]int64{"active": c.version}
	case "rollback":
		var from, to int64
		from, to, err = rag.Rollback(ctx, pool)
		out = map[string]int64{"retired": from, "active": to}
	case "status":
		out, err = rag.Status(ctx, pool)
	case "tombstone":
		err = rag.Tombstone(ctx, pool, c.docID, c.reason)
		out = map[string]string{"doc_id": c.docID, "reason": c.reason}
	}
	if err != nil {
		fmt.Fprintln(stderr, "rag:", err)
		return 1
	}
	printJSON(stdout, out)
	return 0
}

func printJSON(w io.Writer, v any) {
	raw, _ := json.Marshal(v)
	fmt.Fprintln(w, string(raw))
}

// migrateRag is `core migrate-rag`: the retrieval schema, with its own
// version table. `serve` never requires it -- the public search falls back
// to live rows without it -- and nothing else migrates it.
func migrateRag(getenv func(string) string, stdout, stderr io.Writer) int {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, err := db.Open(ctx, getenv(db.EnvDatabaseURL))
	if err != nil {
		fmt.Fprintln(stderr, "rag migration: invalid database configuration")
		return 1
	}
	defer pool.Close()
	if err := rag.Migrate(ctx, pool); err != nil {
		fmt.Fprintln(stderr, "rag migration failed:", err)
		return 1
	}
	fmt.Fprintln(stdout, "Đã áp dụng migration chỉ mục truy hồi (rag, từ vựng). Chưa có phiên bản nào được dựng: chạy `core rag build`.")
	return 0
}
