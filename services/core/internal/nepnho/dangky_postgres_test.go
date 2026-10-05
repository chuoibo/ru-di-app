//go:build postgres

package nepnho

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/accountauth"
	"mobile/services/core/internal/aiharness/metrics"
	"mobile/services/core/internal/aiharness/trinho"
	"mobile/services/core/internal/avatarfeed"
	"mobile/services/core/internal/chatassist"
	"mobile/services/core/internal/chatlegacychange"
	"mobile/services/core/internal/chatv2"
	"mobile/services/core/internal/community"
	"mobile/services/core/internal/diary"
	"mobile/services/core/internal/jobs"
	"mobile/services/core/internal/rag"
)

// sqlGo is every migration the Go binary embeds (the files `core
// migrate-chat`, `migrate-rag`, `migrate-diaries` and `migrate-community`
// run).
func sqlGo() []string {
	sqls := append(chatassist.SchemaFiles(), accountauth.SchemaSQL(), jobs.SchemaSQL(), chatlegacychange.SchemaSQL(), chatv2.SchemaSQL(),
		metrics.SchemaSQL(), metrics.SchemaV2SQL(), metrics.SchemaV3SQL(), metrics.SchemaV4SQL(), SchemaSQL())
	sqls = append(sqls, avatarfeed.SchemaFiles()...)
	sqls = append(sqls, diary.SchemaFiles()...)
	sqls = append(sqls, community.SchemaFiles()...)
	return append(sqls, rag.SchemaFiles()...)
}

var (
	bangSQL  = regexp.MustCompile(`(?i)^\s*(?:CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?|ALTER\s+TABLE\s+(?:IF\s+EXISTS\s+)?)([a-z_][a-z0-9_]*)`)
	cotSQL   = regexp.MustCompile(`(?i)^\s*(?:ADD\s+COLUMN\s+(?:IF\s+NOT\s+EXISTS\s+)?)?([a-z_][a-z0-9_]*)\s+uuid\b(.*)$`)
	tenNguoi = regexp.MustCompile(`^(person_id|.*_person_id|created_by|created_by_id|sender_id|actor_id)$`)
)

// cotNguoiSQL reads, from the SQL the binary embeds, every uuid column of a
// Go table that names a person: a foreign key to people, or a name like one.
// Read from the text, not from a live catalogue, so this test migrates no
// other package's schema into the shared test database (measured: doing so
// changes what the chatassist, rag and cmd/core tests running beside it see).
func cotNguoiSQL() []string {
	seen := map[string]bool{}
	for _, s := range sqlGo() {
		bang := ""
		for _, line := range strings.Split(s, "\n") {
			line = strings.SplitN(line, "--", 2)[0]
			if m := bangSQL.FindStringSubmatch(line); m != nil {
				bang = strings.ToLower(m[1])
				continue
			}
			m := cotSQL.FindStringSubmatch(line)
			if m == nil || bang == "" {
				continue
			}
			cot := strings.ToLower(m[1])
			if strings.Contains(strings.ToLower(m[2]), "references people") || tenNguoi.MatchString(cot) {
				seen[bang+"."+cot] = true
			}
		}
	}
	var out []string
	for c := range seen {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

// cotNguoiThat reads the same from the live catalogue, for nepnho's own
// tables (which this package migrated): the text parse is checked against
// the database where the database is ours to read.
func cotNguoiThat(t *testing.T, pool *pgxpool.Pool) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), `
	  SELECT c.relname, a.attname
	    FROM pg_attribute a JOIN pg_class c ON c.oid=a.attrelid
	   WHERE c.relnamespace=current_schema()::regnamespace AND c.relkind='r' AND c.relname LIKE 'nep\_%' AND a.attnum>0 AND NOT a.attisdropped
	     AND (EXISTS (SELECT 1 FROM pg_constraint k WHERE k.contype='f' AND k.conrelid=c.oid AND k.confrelid='people'::regclass AND a.attnum = ANY(k.conkey))
	          OR (a.atttypid='uuid'::regtype AND a.attname ~ '^(person_id|.*_person_id|created_by|created_by_id|sender_id|actor_id)$'))`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var b, c string
		if err := rows.Scan(&b, &c); err != nil {
			t.Fatal(err)
		}
		out = append(out, b+"."+c)
	}
	sort.Strings(out)
	return out
}

// Every column of every Go-owned table that names a person has an answer in
// CotNguoiGo, and every answer names a live column: a new Go table cannot add
// a person column without saying what account deletion does to it.
//
// Then an account is deleted with memory on, facts written, one forgotten,
// events recorded: after the trigger and the memory lane, every column
// answered Xoa holds 0 rows of that person, and the sidecar holds nothing.
// The columns answered Chua belong to other packages' erasure work and are
// listed by name in the log, not silently passed.
func TestXoaTaiKhoanKhongConHangNaoCuaNguoi(t *testing.T) {
	pool := moiPool(t)
	that := cotNguoiSQL()
	if len(that) < 8 {
		t.Fatalf("read %d person columns from the Go SQL; the parse slipped: %v", len(that), that)
	}
	var cuaNep []string
	for _, c := range that {
		if strings.HasPrefix(c, "nep_") {
			cuaNep = append(cuaNep, c)
		}
	}
	if live := cotNguoiThat(t, pool); strings.Join(live, ",") != strings.Join(cuaNep, ",") {
		t.Fatalf("nepnho's person columns: catalogue %v, SQL text %v", live, cuaNep)
	}
	var khai []string
	cach := map[string]CachXoa{}
	for _, c := range CotNguoiGo {
		khai = append(khai, c.Bang+"."+c.Cot)
		cach[c.Bang+"."+c.Cot] = c.Cach
		if c.LyDo == "" {
			t.Errorf("%s.%s has no reason", c.Bang, c.Cot)
		}
	}
	sort.Strings(khai)
	if strings.Join(that, ",") != strings.Join(khai, ",") {
		t.Fatalf("person columns of Go tables:\n live %v\n listed %v", that, khai)
	}
	for _, c := range that {
		if strings.HasPrefix(c, "nep_") && cach[c] != Xoa {
			t.Errorf("%s is nepnho's own and must be deleted, not %s", c, cach[c])
		}
	}

	gia := moiKhoGia()
	k, err := Moi(pool, gia, khoaQuenThu)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	p := moiNguoi(t, pool)
	if err := k.Bat(ctx, p, CongBoBan); err != nil {
		t.Fatal(err)
	}
	f, err := k.Ghi(ctx, p, suThatMoi("Thích bún chả"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := k.Ghi(ctx, p, suThatMoi("Né quán đông")); err != nil {
		t.Fatal(err)
	}
	if n, err := k.Quen(ctx, p, trinho.QuenGi{ID: f.ID}); n != 1 || err != nil {
		t.Fatalf("Quen %d %v", n, err)
	}
	k.now = func() time.Time { return time.Now().Add(20 * time.Second) }
	if _, err := k.GhiSuKien(ctx, p, []SuKien{{Loai: trinho.CheckIn, Luc: time.Now(), DiaDiemID: "p9"}}); err != nil {
		t.Fatal(err)
	}
	// Before: the person is in nep_cai_dat, nep_su_that, nep_quen,
	// nep_su_kien and nep_xoa (the canary: a count of 0 here proves nothing).
	before := 0
	for _, c := range cuaNep {
		if cach[c] == Xoa {
			before += demCot(t, pool, c, p)
		}
	}
	if before < 5 {
		t.Fatalf("only %d rows name the person before deletion: the scenario is too thin", before)
	}
	if _, err := pool.Exec(ctx, `UPDATE people SET deleted_at=now() WHERE id=$1`, p); err != nil {
		t.Fatal(err)
	}
	var viec string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM nep_xoa WHERE person_id=$1 AND pham_vi='tai_khoan'`, p).Scan(&viec); err != nil {
		t.Fatal(err)
	}
	if err := k.XuLyTin(ctx, jobs.Message{V: 1, Ref: viec}); err != nil {
		t.Fatal(err)
	}
	for _, c := range that {
		if !strings.HasPrefix(c, "nep_") {
			t.Logf("%s: %s, owned by another package's erasure", c, cach[c])
			continue
		}
		n := demCot(t, pool, c, p)
		switch cach[c] {
		case Xoa:
			if n != 0 {
				t.Errorf("%s still holds %d rows of the deleted person", c, n)
			}
		default:
			t.Logf("%s: %s (%d rows of this person), owned by another package's erasure", c, cach[c], n)
		}
	}
	if left := gia.owners(); len(left) != 0 {
		t.Fatalf("the sidecar still holds memories of %v", left)
	}
}

func demCot(t *testing.T, pool *pgxpool.Pool, col, p string) int {
	t.Helper()
	parts := strings.SplitN(col, ".", 2)
	var n int
	if err := pool.QueryRow(context.Background(), fmt.Sprintf(`SELECT count(*) FROM %s WHERE %s=$1`, parts[0], parts[1]), p).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
