//go:build postgres

package metrics_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/aiharness/metrics"
	"mobile/services/core/internal/aiharness/obs"
	"mobile/services/core/internal/chatassist"
	"mobile/services/core/internal/jobs"
	"mobile/services/core/internal/testdb"
)

func uuid() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	s := hex.EncodeToString(b[:])
	return s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:]
}

// setup builds a private schema with the tables chatassist's migration
// references, runs jobs.Migrate, chatassist.Migrate then metrics.Migrate (twice: it must be
// idempotent), exactly the order `core migrate-chat` uses.
func setup(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	base := testdb.Pool(t)
	schema := "aiharness_test_" + strings.ReplaceAll(uuid(), "-", "")
	ident := pgx.Identifier{schema}.Sanitize()
	if _, err := base.Exec(ctx, "CREATE SCHEMA "+ident); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = base.Exec(context.Background(), "DROP SCHEMA "+ident+" CASCADE") })
	cfg := base.Config().Copy()
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	for _, table := range []string{"people", "contexts", "memberships", "account_sessions", "messages", "outings"} {
		if _, err = pool.Exec(ctx, fmt.Sprintf("CREATE TABLE %s (LIKE public.%s INCLUDING ALL)", table, table)); err != nil {
			t.Fatal(err)
		}
	}
	if installed, err := metrics.Installed(ctx, pool); err != nil || installed {
		t.Fatalf("trước migrate: installed=%v err=%v", installed, err)
	}
	if err = jobs.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	if err = chatassist.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err = metrics.Migrate(ctx, pool); err != nil {
			t.Fatalf("lần %d: %v", i+1, err)
		}
	}
	if installed, err := metrics.Installed(ctx, pool); err != nil || !installed {
		t.Fatalf("sau migrate: installed=%v err=%v", installed, err)
	}
	return pool
}

func invocation(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	ctx := context.Background()
	person, id := uuid(), uuid()
	if _, err := pool.Exec(ctx, `INSERT INTO people(id,display_name) VALUES($1,'Synthetic metrics person')`, person); err != nil {
		t.Fatal(err)
	}
	_, err := pool.Exec(ctx, `INSERT INTO chat_ai_invocations(id,scope,context_id,person_id,membership_id,session_digest,logical_id,input_digest,command,prompt,boi_canh,share_expires_at,status) VALUES($1,'me',NULL,$2,NULL,sha256('s'::bytea),$3,sha256('i'::bytea),'hoi','synthetic','{"phieu":null,"luot":[]}',clock_timestamp()+interval '15 minutes','queued')`, id, person, uuid())
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func record(id string) obs.TurnRecord {
	return obs.TurnRecord{InvocationID: obs.ID(id), LanThu: 1, Bot: obs.BotNep, Lenh: obs.LenhHoi, Guard: obs.GuardProceed,
		OutGuard: obs.OutNone, KetThuc: obs.KetThucXong, LoiMoHinh: obs.LoiKhong, PromptVersion: "0123456789ab",
		Buoc: 1, SoGoiMoHinh: 1, TokensIn: 800, TokensOut: 40, MsTong: 900,
		NhanGuard: "sach", YDinh: "find_places", SoYDinh: 1, Tien: "none", Huong: "truy_hoi_mot_buoc", Duong: obs.DuongTruyHoi,
		CongCu: obs.CacCongCu{"search_places", "get_place"}, VongSua: 1, KetKiem: obs.KiemDat, SoXepLai: 1}
}

func TestGhiDocXoaVaCascade(t *testing.T) {
	pool := setup(t)
	ctx := context.Background()
	id := invocation(t, pool)
	if err := metrics.Ghi(ctx, pool, record(id)); err != nil {
		t.Fatal(err)
	}
	// The same attempt twice is one row.
	if err := metrics.Ghi(ctx, pool, record(id)); err != nil {
		t.Fatal(err)
	}
	failed := record(id)
	failed.LanThu, failed.KetThuc, failed.Code, failed.Guard = 2, obs.KetThucThatBai, "nep_khong_cham_tien", obs.GuardRefused
	if err := metrics.Ghi(ctx, pool, failed); err != nil {
		t.Fatal(err)
	}
	var n int
	var code *string
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM ai_turn_metrics WHERE invocation_id=$1`, id).Scan(&n); err != nil || n != 2 {
		t.Fatalf("%d hàng, %v", n, err)
	}
	if err := pool.QueryRow(ctx, `SELECT code FROM ai_turn_metrics WHERE invocation_id=$1 AND lan_thu=1`, id).Scan(&code); err != nil || code != nil {
		t.Fatalf("mã của lượt thành công phải NULL: %v %v", code, err)
	}
	// An invalid record never reaches SQL, and the database refuses text on
	// its own too.
	bad := record(id)
	bad.Bot = "Tối nay đi đâu"
	if err := metrics.Ghi(ctx, pool, bad); err == nil {
		t.Fatal("bản ghi có chữ được ghi")
	}
	for _, col := range []string{"bot", "guard", "out_guard", "ket_thuc", "code", "loi_mo_hinh", "lenh"} {
		_, err := pool.Exec(ctx, `INSERT INTO ai_turn_metrics(invocation_id,lan_thu,bot,lenh,guard,out_guard,ket_thuc,code,loi_mo_hinh,prompt_version,buoc,so_goi_model,so_cong_cu,tokens_in,tokens_out,tokens_cached,tokens_thoughts,luot_bo,phieu_bo,ngay_mo_ho,ky_tu_an,khong_dau,ms_trang_thai_dau,ms_tien_xu_ly,ms_mo_hinh,ms_tong) SELECT $1,9,'nep','hoi','proceed','none','xong',NULL,'none','0123456789ab',0,0,0,0,0,0,0,0,0,0,0,false,0,0,0,0`, id)
		if err != nil {
			t.Fatalf("hàng đồng nhất bị từ chối: %v", err)
		}
		_, err = pool.Exec(ctx, `UPDATE ai_turn_metrics SET `+col+`='Tối nay đi đâu? gọi cho Minh nhé' WHERE invocation_id=$1 AND lan_thu=9`, id)
		if err == nil {
			t.Errorf("cột %s nhận chữ tự do", col)
		}
		_, _ = pool.Exec(ctx, `DELETE FROM ai_turn_metrics WHERE invocation_id=$1 AND lan_thu=9`, id)
	}
	// Thirty days, then gone; a fresh row stays.
	if _, err := pool.Exec(ctx, `UPDATE ai_turn_metrics SET created_at=clock_timestamp()-interval '31 days' WHERE invocation_id=$1 AND lan_thu=2`, id); err != nil {
		t.Fatal(err)
	}
	if gone, err := metrics.Xoa(ctx, pool); err != nil || gone != 1 {
		t.Fatalf("xoá %d, %v", gone, err)
	}
	// The row goes with its invocation.
	if _, err := pool.Exec(ctx, `DELETE FROM chat_ai_invocations WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM ai_turn_metrics WHERE invocation_id=$1`, id).Scan(&n); err != nil || n != 0 {
		t.Fatalf("còn %d hàng sau khi xoá lời gọi", n)
	}
}

// The live schema holds no free text: every text column carries a CHECK, and
// no column is json, bytea or varchar. Read from the catalogue, not the file.
func TestBangThatKhongChuaChuTuDo(t *testing.T) {
	pool := setup(t)
	ctx := context.Background()
	rows, err := pool.Query(ctx, `SELECT c.column_name, c.data_type,
		EXISTS(SELECT 1 FROM pg_constraint k JOIN pg_attribute a ON a.attrelid=k.conrelid AND a.attnum = ANY(k.conkey)
		       WHERE k.conrelid=to_regclass('ai_turn_metrics') AND k.contype='c' AND a.attname=c.column_name)
		FROM information_schema.columns c WHERE c.table_name='ai_turn_metrics' AND c.table_schema=current_schema() ORDER BY c.ordinal_position`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var name, typ string
		var checked bool
		if err := rows.Scan(&name, &typ, &checked); err != nil {
			t.Fatal(err)
		}
		n++
		switch typ {
		case "uuid", "smallint", "integer", "boolean", "timestamp with time zone":
		case "text", "character", "ARRAY":
			if !checked {
				t.Errorf("cột chữ %s không có CHECK", name)
			}
		default:
			t.Errorf("cột %s có kiểu %s", name, typ)
		}
	}
	if n != len(obs.Columns())+1 {
		t.Fatalf("%d cột, muốn %d", n, len(obs.Columns())+1)
	}
}

// A changed version 2 is refused too, and a row carrying a tool name
// outside the registry is refused by the table itself.
func TestChecksumLechV2VaMangDong(t *testing.T) {
	pool := setup(t)
	ctx := context.Background()
	id := invocation(t, pool)
	if _, err := pool.Exec(ctx, `INSERT INTO ai_turn_metrics(invocation_id,lan_thu,bot,lenh,guard,out_guard,ket_thuc,loi_mo_hinh,prompt_version,buoc,so_goi_model,so_cong_cu,tokens_in,tokens_out,tokens_cached,tokens_thoughts,luot_bo,phieu_bo,ngay_mo_ho,ky_tu_an,khong_dau,ms_trang_thai_dau,ms_tien_xu_ly,ms_mo_hinh,ms_tong,cong_cu)
		VALUES($1,1,'nep','hoi','proceed','none','xong','none','0123456789ab',0,0,0,0,0,0,0,0,0,0,0,false,0,0,0,0,ARRAY['search_places','Tối nay đi đâu'])`, id); err == nil {
		t.Fatal("cong_cu nhận một chuỗi ngoài danh sách")
	}
	if _, err := pool.Exec(ctx, `UPDATE aiharness_schema_migrations SET digest='khac' WHERE version=2`); err != nil {
		t.Fatal(err)
	}
	if err := metrics.Migrate(ctx, pool); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("checksum v2 lệch không bị từ chối: %v", err)
	}
}

// Version 3 in the live table: nhay_cam is refused, the other labels are
// not, and a changed version 3 is refused like the others.
func TestV3KhongNhanNhayCam(t *testing.T) {
	pool := setup(t)
	ctx := context.Background()
	id := invocation(t, pool)
	r := record(id)
	if err := metrics.Ghi(ctx, pool, r); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE ai_turn_metrics SET nhan_guard='nhay_cam' WHERE invocation_id=$1`, id); err == nil {
		t.Fatal("the table took nhay_cam")
	}
	if _, err := pool.Exec(ctx, `UPDATE ai_turn_metrics SET nhan_guard='ngoai_pham_vi' WHERE invocation_id=$1`, id); err != nil {
		t.Fatalf("ngoai_pham_vi refused: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE aiharness_schema_migrations SET digest='khac' WHERE version=3`); err != nil {
		t.Fatal(err)
	}
	if err := metrics.Migrate(ctx, pool); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("checksum v3 lệch không bị từ chối: %v", err)
	}
}

// Version 4 in the live table: rows a version-3 binary wrote for a
// nhay_cam turn (no label, the router's other columns kept) are rewritten
// to the clean turn they cannot be told from, the direct path's to the
// canonical small-talk labels; a router failure's row is left as it is; and
// the table refuses such a row from then on.
func TestV4ViVetNhayCamCu(t *testing.T) {
	pool := setup(t)
	ctx := context.Background()
	thang, tien, hong := invocation(t, pool), invocation(t, pool), invocation(t, pool)
	for _, id := range []string{thang, tien, hong} {
		if err := metrics.Ghi(ctx, pool, record(id)); err != nil {
			t.Fatal(err)
		}
	}
	// Back to what version 3 allowed, then the old rows.
	if _, err := pool.Exec(ctx, `ALTER TABLE ai_turn_metrics DROP CONSTRAINT ai_turn_metrics_nhan_nhay_cam_check`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE ai_turn_metrics SET nhan_guard='', huong='tac_tu', y_dinh='find_places', so_y_dinh=2, tien='none', duong='thang', cong_cu='{}' WHERE invocation_id=$1`, thang); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE ai_turn_metrics SET nhan_guard='', huong='tac_tu', y_dinh='find_places', so_y_dinh=1, tien='money_action', duong='tu_choi_tien', cong_cu='{}' WHERE invocation_id=$1`, tien); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE ai_turn_metrics SET nhan_guard='', huong='', y_dinh='', so_y_dinh=0, tien='', duong='', cong_cu='{}' WHERE invocation_id=$1`, hong); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM aiharness_schema_migrations WHERE version=4`); err != nil {
		t.Fatal(err)
	}
	if err := metrics.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	doc := func(id string) string {
		var s string
		if err := pool.QueryRow(ctx, `SELECT concat_ws('|', nhan_guard, huong, y_dinh, so_y_dinh, tien, duong) FROM ai_turn_metrics WHERE invocation_id=$1`, id).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	for id, want := range map[string]string{
		thang: "sach|tra_loi_thang|smalltalk|1|none|thang",
		tien:  "sach|tac_tu|find_places|1|money_action|tu_choi_tien",
		hong:  "|||0||",
	} {
		if got := doc(id); got != want {
			t.Errorf("row %s = %s, want %s", id, got, want)
		}
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM ai_turn_metrics WHERE nhan_guard='' AND huong<>''`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("%d rows still tell a nhay_cam turn: %v", n, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE ai_turn_metrics SET nhan_guard='' WHERE invocation_id=$1`, thang); err == nil {
		t.Fatal("the table took router columns without a label")
	}
}

// A changed version 1 is refused, never reapplied.
func TestChecksumLech(t *testing.T) {
	pool := setup(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `UPDATE aiharness_schema_migrations SET digest='khac' WHERE version=1`); err != nil {
		t.Fatal(err)
	}
	if err := metrics.Migrate(ctx, pool); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("checksum lệch không bị từ chối: %v", err)
	}
}
