//go:build postgres

package repo

// The world of the W8 oracle (pair_repo_oracle_postgres_test.go): fourteen
// pair conversations and a group, written as literal SQL through
// pair_world_postgres_test.go's helpers so the same statements seed both
// sides. Each conversation holds one situation a notebook or a sheet can be
// in, because uq_pair_papers_open_per_context allows one open sheet each:
//
//	ab  active cycle; lap_so and doc_chat complete, bat_doi half granted (the
//	    other half an unanswered row), an expired doc_chat offer with a live
//	    and a revoked grant; two constraints; a sheet sent back at v2 (da_gui),
//	    one agreed with its outing (chot), one kept twice (da_giu), one
//	    withdrawn
//	cd  active cycle, both people a couple, a human draft (nhap)
//	ae  pending cycle, lap_so offered by one, a Nếp draft (nhap, v1 sent)
//	be  no notebook
//	bc  a notebook whose only cycle is closed, a cancelled sheet
//	de  a Nếp sheet one person agreed to (dong_y)
//	ce  a sheet opened by its recipient (da_xem)
//	eg  a sheet whose week is over (da_gui past its deadline)
//	fa  one person left the conversation; a recorded outing (da_di)
//	cg  bat_doi offered, one of the two already a couple elsewhere
//	bg  a notebook that never opened a cycle
//	ag  the other person only invited
//	fg  closed temporary sheets whose stored JSON takes every shape dict() reads
//	group  a group context
//
// Ties are deliberate: two responses and two kept lines on one created_at.

const (
	kindPaper    = 0x94
	kindKeep     = 0x95
	kindResponse = 0x96
)

// pairNow is a Wednesday, noon in Vietnam: that week's Monday is 2030-09-16,
// its Saturday 2030-09-21, and it ends 2030-09-22T17:00:00Z.
const pairNow = "2030-09-18T05:00:00.654321Z"

// pairWeekEnd is han_tuan(pairNow).
const pairWeekEnd = "2030-09-22T17:00:00Z"

type pairRepoWorld struct {
	pairWorld
	an, binh, chi, dung, em, phuong, la, giang string

	ab, cd, ae, be, bc, de, ce, eg, fa, cg, bg, ag, fg, group string

	nbAB, nbCD, nbAE, nbBC, nbDE, nbBG                     string
	cyAB, cyCD, cyAE, cyBC, cyDE, cyCE, cyEG, cyFA, cyCG   string
	prLapSoAB, prDocChatAB, prBatDoiAB, prOldAB, prLapSoAE string
	prBatDoiCD, prBatDoiCG                                 string

	pAB1, pAB2, pAB3, pAB4, pCD1, pAE1, pBC1, pDE1, pCE1, pEG1, pFA1 string
	odd                                                              []string

	oAB, oFree string

	missingPaper string
}

const (
	stopDinner = `{"gio": "19:00", "viec": "Ăn tối (dữ liệu mẫu)", "place_id": null, "can_kiem": true}`
)

func pairContent(day string, stops ...string) string {
	if len(stops) == 0 {
		stops = []string{stopDinner}
	}
	out := `{"ngay": "` + day + `", "chang": [`
	for i, s := range stops {
		if i > 0 {
			out += ", "
		}
		out += s
	}
	return out + `]}`
}

func (w *pairRepoWorld) paper(n int, contextID string, cycle any, owner, state string, current int, tuan, createdAt,
	expiresAt string, extra ...any) string {
	id := fid(kindPaper, n)
	w.insert("pair_papers", append([]any{"id", id, "context_id", contextID, "context_kind", "pair", "cycle_id", cycle,
		"is_temporary", cycle == nil, "draft_owner_id", owner, "state", state, "current_version", current, "tuan", tuan,
		"created_at", createdAt, "expires_at", expiresAt}, extra...)...)
	return id
}

func (w *pairRepoWorld) version(paper string, v int, content string, lyDo any, nguon, author string, sentAt, sentBy any,
	createdAt string) {
	w.insert("pair_paper_versions", "paper_id", paper, "version", v, "content", content, "ly_do", lyDo, "nguon", nguon,
		"author_type", author, "sent_at", sentAt, "sent_by", sentBy, "created_at", createdAt)
}

func (w *pairRepoWorld) view(paper string, v int, person, seenAt string) {
	w.insert("pair_paper_views", "paper_id", paper, "version", v, "person_id", person, "seen_at", seenAt)
}

func (w *pairRepoWorld) response(n int, paper string, v int, person, kind, createdAt string) {
	w.insert("pair_paper_responses", "id", fid(kindResponse, n), "paper_id", paper, "version", v, "person_id", person,
		"kind", kind, "created_at", createdAt)
}

func (w *pairRepoWorld) keep(n int, paper, person, line, createdAt string) {
	w.insert("pair_paper_keeps", "id", fid(kindKeep, n), "paper_id", paper, "person_id", person, "line", line,
		"created_at", createdAt)
}

func (w *pairRepoWorld) outing(n int, contextID, creator, title, day string) string {
	id := fid(kindOuting, n)
	w.insert("outings", "id", id, "context_id", contextID, "created_by_id", creator, "title", title, "starts_on", day,
		"ends_on", day, "headcount", 2, "budget_per_person_vnd", 0, "created_at", stdCreated)
	return id
}

func (w *pairRepoWorld) sentPaper(n int, contextID, cycle, sender, state, tuan, createdAt, expiresAt, day string,
	extra ...any) string {
	id := w.paper(n, contextID, cycle, sender, state, 1, tuan, createdAt, expiresAt, extra...)
	w.version(id, 1, pairContent(day), nil, `{}`, "human", createdAt, sender, createdAt)
	return id
}

func (w *pairRepoWorld) pair(n int, x, y string, stateY string) string {
	id := w.pairContext(n, x, y)
	w.member(0x100+2*n, id, x, "active", stdCreated)
	w.member(0x101+2*n, id, y, stateY, stdCreated)
	return id
}

func (w *pairRepoWorld) activeCycle(n int, notebook, createdAt string, people ...string) string {
	id := w.cycle(n, notebook, "active", createdAt, 1)
	for _, p := range people {
		w.participant(id, p, createdAt)
	}
	return id
}

func newPairRepoWorld() *pairRepoWorld {
	w := &pairRepoWorld{}
	w.an = w.person(0x21, "An (dữ liệu mẫu)")
	w.binh = w.person(0x22, "Bình (dữ liệu mẫu)")
	w.chi = w.person(0x23, "Chi (dữ liệu mẫu)")
	w.dung = w.person(0x24, "Dũng (dữ liệu mẫu)")
	w.em = w.person(0x25, "")
	w.phuong = w.person(0x26, "Phương (dữ liệu mẫu)")
	w.la = w.person(0x27, "Người lạ (dữ liệu mẫu)")
	w.giang = w.person(0x28, "Giang (dữ liệu mẫu)")
	w.missingContext = fid(kindContext, 0xff)
	w.missingPerson = fid(kindPerson, 0xff)
	w.missingPaper = fid(kindPaper, 0xff)

	w.ab = w.pair(0x41, w.an, w.binh, "active")
	w.cd = w.pair(0x42, w.chi, w.dung, "active")
	w.ae = w.pair(0x43, w.an, w.em, "active")
	w.be = w.pair(0x44, w.binh, w.em, "active")
	w.bc = w.pair(0x45, w.binh, w.chi, "active")
	w.de = w.pair(0x46, w.dung, w.em, "active")
	w.ce = w.pair(0x47, w.chi, w.em, "active")
	w.eg = w.pair(0x48, w.em, w.giang, "active")
	w.fa = w.pair(0x49, w.an, w.phuong, "left")
	w.cg = w.pair(0x4a, w.chi, w.giang, "active")
	w.bg = w.pair(0x4b, w.binh, w.giang, "active")
	w.ag = w.pair(0x4c, w.an, w.giang, "invited")
	w.fg = w.pair(0x4d, w.phuong, w.giang, "active")
	w.group = w.context(0x4e, w.an)
	for n, p := range []string{w.an, w.binh, w.chi} {
		w.member(0x1f0+n, w.group, p, "active", stdCreated)
	}

	// ab
	w.nbAB = w.notebook(0x11, w.ab)
	w.cyAB = w.activeCycle(0x11, w.nbAB, "2030-09-01T00:00:00Z", w.an, w.binh)
	w.prLapSoAB = w.proposal(0x11, w.cyAB, "lap_so", w.an, "2030-09-01T00:00:00Z", "2030-09-08T00:00:00Z", "2030-09-01T01:00:00Z")
	w.consent(0x11, w.prLapSoAB, w.an, "2030-09-01T00:00:00Z", nil, "2030-09-01T00:00:00Z")
	w.consent(0x12, w.prLapSoAB, w.binh, "2030-09-01T01:00:00Z", nil, "2030-09-01T01:00:00Z")
	w.prDocChatAB = w.proposal(0x12, w.cyAB, "doc_chat", w.binh, "2030-09-15T00:00:00Z", "2030-09-22T00:00:00Z", "2030-09-15T02:00:00Z")
	w.consent(0x13, w.prDocChatAB, w.binh, "2030-09-15T00:00:00Z", nil, "2030-09-15T00:00:00Z")
	w.consent(0x14, w.prDocChatAB, w.an, "2030-09-15T02:00:00Z", nil, "2030-09-15T02:00:00Z")
	w.prBatDoiAB = w.proposal(0x13, w.cyAB, "bat_doi", w.an, "2030-09-16T00:00:00Z", "2030-09-23T00:00:00Z", nil)
	w.consent(0x15, w.prBatDoiAB, w.an, "2030-09-16T00:00:00Z", nil, "2030-09-16T00:00:00Z")
	w.consent(0x16, w.prBatDoiAB, w.binh, nil, nil, "2030-09-16T00:00:00Z")
	w.prOldAB = w.proposal(0x14, w.cyAB, "doc_chat", w.an, "2030-09-02T00:00:00Z", "2030-09-09T00:00:00Z", nil)
	w.consent(0x17, w.prOldAB, w.an, "2030-09-02T00:00:00Z", nil, "2030-09-02T00:00:00Z")
	w.consent(0x18, w.prOldAB, w.binh, "2030-09-03T00:00:00Z", "2030-09-04T00:00:00Z", "2030-09-03T00:00:00Z")
	w.constraint(w.cyAB, w.an, "khong_an_duoc", "Hành 🧅 (dữ liệu mẫu)", 1, "2030-09-02T00:00:00Z")
	w.constraint(w.cyAB, w.binh, "dung", "Đừng hát (dữ liệu mẫu)", 2, "2030-09-03T00:00:00.5Z")

	w.pAB1 = w.paper(0x11, w.ab, w.cyAB, w.an, "da_gui", 2, "2030-09-16", "2030-09-16T02:00:00Z", pairWeekEnd)
	w.version(w.pAB1, 1, pairContent("2030-09-21"), nil, `{}`, "human", "2030-09-16T03:00:00Z", w.an, "2030-09-16T02:00:00Z")
	w.version(w.pAB1, 2, pairContent("2030-09-20",
		`{"gio": "18:30", "viec": "Lẩu \"ngon\" (dữ liệu mẫu)", "place_id": "e0000099-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "can_kiem": false}`,
		`{"gio": "21:00", "viec": "Cà phê", "place_id": null, "can_kiem": true}`),
		"Thứ Bảy bận (dữ liệu mẫu)", `{"scope": "chung", "dung": ["nguoi"], "luc": "2030-09-17T01:00:00+00:00"}`,
		"human", "2030-09-17T01:00:00Z", w.binh, "2030-09-17T01:00:00Z")
	w.view(w.pAB1, 1, w.binh, "2030-09-16T04:00:00Z")
	w.response(0x11, w.pAB1, 1, w.an, "dong_y", "2030-09-16T03:00:00Z")
	w.response(0x13, w.pAB1, 1, w.binh, "de_nghi_sua", "2030-09-17T01:00:00Z")
	w.response(0x12, w.pAB1, 2, w.binh, "dong_y", "2030-09-17T01:00:00Z")

	w.pAB2 = w.sentPaper(0x12, w.ab, w.cyAB, w.binh, "chot", "2030-09-09", "2030-09-09T02:00:00Z", "2030-09-15T17:00:00Z", "2030-09-14")
	w.response(0x14, w.pAB2, 1, w.binh, "dong_y", "2030-09-09T02:00:00Z")
	w.response(0x15, w.pAB2, 1, w.an, "dong_y", "2030-09-10T00:00:00Z")
	w.oAB = w.outing(0x41, w.ab, w.an, "Tờ lời rủ 14/09", "2030-09-14")
	w.insert("pair_paper_outings", "paper_id", w.pAB2, "version", 1, "outing_id", w.oAB, "linked_at", "2030-09-10T00:00:00Z")

	w.pAB3 = w.sentPaper(0x13, w.ab, w.cyAB, w.an, "da_giu", "2030-09-02", "2030-09-02T02:00:00Z", "2030-09-08T17:00:00Z",
		"2030-09-07", "done_recorded_by_id", w.binh, "done_recorded_at", "2030-09-07T12:00:00Z")
	w.keep(0x12, w.pAB3, w.an, "Mưa to mà vui (dữ liệu mẫu)", "2030-09-07T13:00:00Z")
	w.keep(0x11, w.pAB3, w.binh, "Lần sau mang áo mưa", "2030-09-07T13:00:00Z")
	w.pAB4 = w.sentPaper(0x14, w.ab, w.cyAB, w.binh, "rut", "2030-08-26", "2030-08-26T02:00:00Z", "2030-09-01T17:00:00Z", "2030-08-31")

	// cd
	w.nbCD = w.notebook(0x12, w.cd)
	w.cyCD = w.activeCycle(0x12, w.nbCD, "2030-08-01T00:00:00Z", w.chi, w.dung)
	w.prBatDoiCD = w.proposal(0x15, w.cyCD, "bat_doi", w.chi, "2030-08-02T00:00:00Z", "2030-08-09T00:00:00Z", "2030-08-02T01:00:00Z")
	w.consent(0x19, w.prBatDoiCD, w.chi, "2030-08-02T00:00:00Z", nil, "2030-08-02T00:00:00Z")
	w.consent(0x1a, w.prBatDoiCD, w.dung, "2030-08-02T01:00:00Z", nil, "2030-08-02T01:00:00Z")
	for _, p := range []string{w.chi, w.dung} {
		w.insert("active_couple_members", "person_id", p, "cycle_id", w.cyCD, "since", "2030-08-02T01:00:00Z")
	}
	w.pCD1 = w.paper(0x21, w.cd, w.cyCD, w.chi, "nhap", 1, "2030-09-16", "2030-09-17T00:00:00Z", pairWeekEnd)
	w.version(w.pCD1, 1, pairContent("2030-09-21"), nil, `{"scope": "chung", "dung": ["routine"], "luc": "2030-09-17T00:00:00+00:00"}`,
		"human", nil, nil, "2030-09-17T00:00:00Z")

	// ae
	w.nbAE = w.notebook(0x13, w.ae)
	w.cyAE = w.cycle(0x13, w.nbAE, "pending", "2030-09-17T00:00:00Z", 1)
	w.participant(w.cyAE, w.an, "2030-09-17T00:00:00Z")
	w.participant(w.cyAE, w.em, "2030-09-17T00:00:00Z")
	w.prLapSoAE = w.proposal(0x16, w.cyAE, "lap_so", w.an, "2030-09-17T00:00:00Z", "2030-09-24T00:00:00Z", nil)
	w.consent(0x1b, w.prLapSoAE, w.an, "2030-09-17T00:00:00Z", nil, "2030-09-17T00:00:00Z")
	w.pAE1 = w.paper(0x31, w.ae, w.cyAE, w.an, "nhap", 1, "2030-09-16", "2030-09-17T01:00:00Z", pairWeekEnd)
	w.version(w.pAE1, 1, pairContent("2030-09-21"), "Nếp thấy hai bạn hay đi tối thứ Bảy (dữ liệu mẫu)",
		`{"scope": "chung", "dung": ["chat"], "luc": "2030-09-17T01:00:00+00:00"}`, "nep", "2030-09-17T01:00:00Z", nil,
		"2030-09-17T01:00:00Z")

	// bc
	w.nbBC = w.notebook(0x14, w.bc)
	w.cyBC = w.cycle(0x14, w.nbBC, "closed", "2030-06-01T00:00:00Z", 1)
	w.participant(w.cyBC, w.binh, "2030-06-01T00:00:00Z")
	w.participant(w.cyBC, w.chi, "2030-06-01T00:00:00Z")
	w.pBC1 = w.sentPaper(0x41, w.bc, w.cyBC, w.binh, "huy", "2030-05-27", "2030-05-27T00:00:00Z", "2030-06-01T17:00:00Z", "2030-06-01")

	// de
	w.nbDE = w.notebook(0x15, w.de)
	w.cyDE = w.activeCycle(0x15, w.nbDE, "2030-09-01T00:00:00Z", w.dung, w.em)
	w.pDE1 = w.paper(0x51, w.de, w.cyDE, w.dung, "dong_y", 1, "2030-09-16", "2030-09-16T00:00:00Z", pairWeekEnd)
	w.version(w.pDE1, 1, pairContent("2030-09-21"), "Tối thứ Bảy (dữ liệu mẫu)", `{"scope": "chung", "dung": ["routine"]}`,
		"nep", "2030-09-16T00:00:00Z", nil, "2030-09-16T00:00:00Z")
	w.response(0x21, w.pDE1, 1, w.dung, "dong_y", "2030-09-16T05:00:00Z")
	w.oFree = w.outing(0x42, w.de, w.dung, "Buổi tự do (dữ liệu mẫu)", "2030-09-21")

	// ce
	w.cyCE = w.activeCycle(0x16, w.notebook(0x16, w.ce), "2030-09-01T00:00:00Z", w.chi, w.em)
	w.pCE1 = w.sentPaper(0x61, w.ce, w.cyCE, w.chi, "da_xem", "2030-09-16", "2030-09-16T06:00:00Z", pairWeekEnd, "2030-09-20")
	w.view(w.pCE1, 1, w.em, "2030-09-17T00:00:00Z")
	w.response(0x31, w.pCE1, 1, w.chi, "dong_y", "2030-09-16T06:00:00Z")

	// eg
	w.cyEG = w.activeCycle(0x17, w.notebook(0x17, w.eg), "2030-09-01T00:00:00Z", w.em, w.giang)
	w.pEG1 = w.sentPaper(0x71, w.eg, w.cyEG, w.giang, "da_gui", "2030-09-09", "2030-09-09T00:00:00Z", "2030-09-15T17:00:00Z", "2030-09-14")
	w.response(0x41, w.pEG1, 1, w.giang, "dong_y", "2030-09-09T00:00:00Z")

	// fa
	w.cyFA = w.activeCycle(0x18, w.notebook(0x18, w.fa), "2030-07-01T00:00:00Z", w.an, w.phuong)
	w.constraint(w.cyFA, w.an, "dung", "Đừng đến muộn (dữ liệu mẫu)", 1, "2030-07-02T00:00:00Z")
	w.pFA1 = w.sentPaper(0x81, w.fa, w.cyFA, w.an, "da_di", "2030-09-09", "2030-09-09T00:00:00Z", "2030-09-15T17:00:00Z",
		"2030-09-13", "done_recorded_by_id", w.an, "done_recorded_at", "2030-09-14T12:00:00Z")

	// cg: giang is the cycle's first participant.
	w.cyCG = w.cycle(0x19, w.notebook(0x19, w.cg), "active", "2030-09-01T00:00:00Z", 1)
	w.participant(w.cyCG, w.giang, "2030-09-01T00:00:00Z")
	w.participant(w.cyCG, w.chi, "2030-09-01T00:00:00.000001Z")
	w.prBatDoiCG = w.proposal(0x17, w.cyCG, "bat_doi", w.giang, "2030-09-17T00:00:00Z", "2030-09-24T00:00:00Z", nil)
	w.consent(0x1c, w.prBatDoiCG, w.giang, "2030-09-17T00:00:00Z", nil, "2030-09-17T00:00:00Z")

	// bg
	w.nbBG = w.notebook(0x1a, w.bg)

	// fg: every stored JSON shape, one closed temporary sheet each.
	oddVersion := func(n int, created string, versions ...[2]string) {
		id := w.paper(0x90+n, w.fg, nil, w.phuong, "bo", 1, "2030-08-12", created, "2030-08-18T17:00:00Z")
		for v, pair := range versions {
			w.version(id, v+1, pair[0], nil, pair[1], "human", nil, nil, created)
		}
		w.odd = append(w.odd, id)
	}
	oddVersion(1, "2030-08-12T00:00:09Z",
		[2]string{`[["ngay", "2030-09-21"], "xy", {"a": 1, "b": 2}, ["ngay", "2030-09-22"]]`, `0`},
		[2]string{`{}`, `null`}, [2]string{`""`, `false`}, [2]string{`[]`, `0.0`},
		[2]string{`{"z": [1, 2.5, {"k": null}], "a": "Ơ 🙂"}`, `["ab", ["x", {"y": 1}]]`})
	oddVersion(2, "2030-08-12T00:00:08Z", [2]string{`true`, `{}`})
	oddVersion(3, "2030-08-12T00:00:07Z", [2]string{`"ab"`, `{}`})
	oddVersion(4, "2030-08-12T00:00:06Z", [2]string{`[5]`, `{}`})
	oddVersion(5, "2030-08-12T00:00:05Z", [2]string{`[["a", 1, 2]]`, `{}`})
	oddVersion(6, "2030-08-12T00:00:04Z", [2]string{`[[["k"], 1]]`, `{}`})
	oddVersion(7, "2030-08-12T00:00:03Z", [2]string{`[{"a": 1}]`, `{}`})
	oddVersion(8, "2030-08-12T00:00:02Z", [2]string{`{"ngay": "x"}`, `7`})
	oddVersion(9, "2030-08-12T00:00:01Z", [2]string{`[null]`, `{}`})
	oddVersion(10, "2030-08-12T00:00:00Z", [2]string{`-1e-400`, `"ư"`})
	return w
}
