package nepnho

// CachXoa is what account deletion does to one Go-owned column that names a
// person.
type CachXoa string

const (
	// Xoa: no row names the person once people.deleted_at is set and the
	// tai_khoan deletion completed (trigger nep_xoa_nguoi, then the memory
	// lane for what the sidecar holds).
	Xoa CachXoa = "xoa"
	// Chua: not erased by account deletion yet, and not nepnho's to erase
	// (one writer per table). Named here so the gap is on a list with its
	// owner, not silent; each is on the open list of ADR-0043.
	Chua CachXoa = "chua"
)

// CotNguoi is one column of a Go-owned table that holds a person id.
type CotNguoi struct {
	Bang, Cot string
	Cach      CachXoa
	LyDo      string
}

// CotNguoiGo answers, for every column of every table a Go package creates
// that names a person, what account deletion does to it. The PostgreSQL gate
// (dangky_postgres_test.go) enumerates those columns from the live catalogue
// after every Go migration ran, and goes red on a column missing here or a
// row here with no column: a new table cannot add a person column without
// answering.
var CotNguoiGo = []CotNguoi{
	{"nep_cai_dat", "person_id", Xoa, "consent row; deleted by the trigger"},
	{"nep_su_kien", "person_id", Xoa, "typed events; deleted by the trigger"},
	{"nep_quen", "person_id", Xoa, "tombstones; deleted by the trigger"},
	{"nep_su_that", "person_id", Xoa, "receipts; hidden by the trigger, deleted by the tai_khoan deletion once Milvus counts zero"},
	{"nep_xoa", "person_id", Xoa, "the deletion receipts: once the account deletion completes, every row of the person trades person_id for a keyed hash (nguoi_bam), the proof the erasure ran without naming the person"},
	{"chat_ai_invocations", "person_id", Chua, "chatassist: job rows keep the caller for the room's history; question text is purged by chatassist's own 15-minute and 30-day passes"},
	{"chat_plan_promotions", "created_by_id", Chua, "chatassist: who promoted a plan card; no text of the person"},
	{"chat_shared_drafts", "created_by", Chua, "chatassist: who started a shared sheet"},
	{"chat_v2_devices", "person_id", Chua, "chatv2 (lab): device keys of the person (chat_v2_members reaches the person only through a device); the E2EE rollout owns their erasure"},
	{"chat_v2_events", "actor_id", Chua, "chatv2 (lab): who caused a room event; ids only, no plaintext"},
}
