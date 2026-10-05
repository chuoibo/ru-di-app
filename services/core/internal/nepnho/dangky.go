package nepnho

// CachXoa is what account deletion does to one Go-owned column that names a
// person.
type CachXoa string

const (
	// Xoa: no row names the person once people.deleted_at is set and the
	// tai_khoan deletion completed (trigger nep_xoa_nguoi, then the memory
	// lane for what the sidecar holds).
	Xoa CachXoa = "xoa"
	// Chua: not erased by nepnho and not nepnho's to erase (one writer per
	// table): either not erased by account deletion yet, or erased by the
	// owning package's own trigger, which this gate does not count (the reason
	// says which, and names the owner's test when one exists). Named here so
	// the gap is on a list with its owner, not silent; each is on the open
	// list of ADR-0043.
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
	// internal/dieuchinh (ADR-0056): amendments of a collection batch are money
	// records, kept like the obligations they amend (ADR-0023 keeps
	// collection_* rows); a person who ends their account stays named on the
	// ledger the group still settles, exactly as collection_obligations does.
	{"collection_amendments", "proposed_by_id", Chua, "dieuchinh: who proposed an amendment; money audit, kept like collection_obligations (ADR-0023)"},
	{"collection_amendment_lines", "sender_id", Chua, "dieuchinh: an amended pair's sender; money record, kept like collection_obligations (ADR-0023)"},
	{"collection_amendment_lines", "recipient_id", Chua, "dieuchinh: an amended pair's recipient; money record, kept like collection_obligations (ADR-0023)"},
	{"collection_amendment_parties", "person_id", Chua, "dieuchinh: who had to accept; money audit, kept (ADR-0023)"},
	{"collection_amendment_decisions", "person_id", Chua, "dieuchinh: who accepted or refused; money audit, kept (ADR-0023)"},
	{"collection_amendment_links", "sender_id", Chua, "dieuchinh: whose review link; the link itself is a token digest, kept with the money record (ADR-0023)"},
	{"chat_ai_invocations", "person_id", Chua, "chatassist: job rows keep the caller for the room's history; question text is purged by chatassist's own 15-minute and 30-day passes"},
	{"chat_plan_promotions", "created_by_id", Chua, "chatassist: who promoted a plan card; no text of the person"},
	{"chat_shared_drafts", "created_by", Chua, "chatassist: who started a shared sheet"},
	{"chat_v2_devices", "person_id", Chua, "chatv2 (ADR-0057): public device keys of the person, kept because the room logs reference them; account deletion revokes every device (trigger chat_v2_account_deleted), which erases their key packages and pending Welcomes and clears ready of their rooms; no private key is ever stored"},
	{"push_devices", "person_id", Xoa, "push (ADR-0057 §7): installations and their Expo tokens; deleted by the trigger push_account_deleted"},
	{"push_outbox", "person_id", Xoa, "push: pending wakes, no content; deleted by the trigger push_account_deleted"},
	{"chat_v2_events", "actor_id", Chua, "chatv2 (lab): who caused a room event; ids only, no plaintext"},
	// internal/community (ADR-0040): its trigger community_erase (schema.sql)
	// deletes these rows, or nulls the audit actor, when people.deleted_at is
	// set. No community test counts them after a deletion yet.
	{"community_moderators", "person_id", Chua, "community: moderator grant; deleted by community's own trigger community_erase, not counted by any test yet"},
	{"community_media", "owner_id", Chua, "community: uploaded media rows (the storage GC is queued by community_media_gc); deleted by community_erase, not counted by any test yet"},
	{"community_comment_drafts", "author_id", Chua, "community: comments awaiting moderation, with their text; deleted by community_erase, not counted by any test yet"},
	{"community_follows", "person_id", Chua, "community: who follows whom (a person target is text, erased by the same trigger); deleted by community_erase, not counted by any test yet"},
	{"community_preferences", "person_id", Chua, "community: feed preferences; deleted by community_erase, not counted by any test yet"},
	{"community_keeps", "person_id", Chua, "community: saved posts; deleted by community_erase, not counted by any test yet"},
	{"community_feeds", "person_id", Chua, "community: ranked feed snapshots; deleted by community_erase, not counted by any test yet"},
	{"community_feedback", "person_id", Chua, "community: feed feedback; deleted by community_erase, not counted by any test yet"},
	{"community_interactions", "person_id", Chua, "community: interaction signals for ranking; deleted by community_erase, not counted by any test yet"},
	{"community_audit", "actor_id", Chua, "community: moderation audit trail; the actor is set NULL by community_erase (the FK is ON DELETE SET NULL), not counted by any test yet"},
	{"community_notifications", "person_id", Chua, "community: notifications; deleted by community_erase, not counted by any test yet"},
	{"community_notifications", "actor_id", Chua, "community: who mentioned the recipient (QA UI-147); set NULL by community_erase as rewritten in notification_actor_erasure.sql, counted by community's TestPostgresCommunityErasureForgetsWhoMentioned"},
	{"community_idempotency", "person_id", Chua, "community: idempotency keys; deleted by community_erase, not counted by any test yet"},
	{"community_limits", "person_id", Chua, "community: rate-limit counters; deleted by community_erase, not counted by any test yet"},
	// internal/diary (ADR-0039).
	{"outing_diaries", "owner_id", Chua, "diary: memory books (versions and photos cascade); deleted by diary's own trigger diary_erase_for_account, counted by diary's TestPostgresAccountErasurePurgesBooksVersionsAndJobs"},
	{"outing_diary_jobs", "owner_id", Chua, "diary: AI jobs holding the excerpts sent for a book; deleted by diary_erase_for_account, counted by the same diary test"},
	{"outing_endings", "ended_by", Chua, "diary: who closed an outing (the FK has no ON DELETE); not erased by account deletion yet: an id only, no text of the person"},
	{"managed_accounts", "person_id", Chua, "accountauth: encrypted login and recovery credentials; erased by erase_managed_account_credentials, counted by TestPostgresAccountErasureAndDiscoveryCompatibility"},
	{"account_challenges", "person_id", Chua, "accountauth: bound sensitive-change proofs; erased by the same trigger, counted by the same accountauth test"},
}
