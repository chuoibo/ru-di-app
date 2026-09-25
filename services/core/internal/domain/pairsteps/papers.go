package pairsteps

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"mobile/services/core/internal/domain/pairnotebook"
	"mobile/services/core/internal/domain/pairpaper"
)

// SummaryStop is PaperSummaryStop.
type SummaryStop struct {
	Gio  string
	Viec string
}

// PaperSummary is one row of PaperListResponse.
type PaperSummary struct {
	ID         string
	State      string
	Version    int
	Tuan       pairpaper.Date
	Ngay       *pairpaper.Date
	ExpiresAt  time.Time
	ChangDau   *SummaryStop
	DongGiuDau *string
}

// Command is PaperCommandResponse.
type Command struct {
	ID       string
	State    string
	Version  int
	OutingID *string
}

// StopInput is PaperStopInput after validation; PlaceID is a catalogue id
// (1..80 characters, a slug such as «p-lau-ga-la-e») or nil.
type StopInput struct {
	Gio     string
	Viec    string
	PlaceID *string
	CanKiem bool
}

// ContentInput is PaperContentInput after validation.
type ContentInput struct {
	Ngay  pairpaper.Date
	Chang []StopInput
}

// Reply is PaperResponseRequest: Kind "dong_y", or a counter-proposal with its
// content and ly_do (nil when absent).
type Reply struct {
	Kind    string
	Content ContentInput
	LyDo    *string
}

// NoiDungLuu is _noi_dung_luu: the request's content as the row stores it.
func NoiDungLuu(content ContentInput) pairpaper.Content {
	out := pairpaper.Content{Ngay: content.Ngay.ISOFormat(), Chang: make([]pairpaper.Stop, len(content.Chang))}
	for i, stop := range content.Chang {
		out.Chang[i] = pairpaper.Stop{Gio: stop.Gio, Viec: stop.Viec, PlaceID: stop.PlaceID, CanKiem: stop.CanKiem}
	}
	return out
}

func wireCommand(paper *Paper, state string, outingID *string) Command {
	if outingID == nil {
		outingID = paper.OutingID
	}
	return Command{ID: paper.ID, State: state, Version: paper.CurrentVersion, OutingID: outingID}
}

// versionNumbered is `next((v for v in paper.versions if v.version == n), None)`.
func versionNumbered(paper *Paper, number int64) *Version {
	for i := range paper.Versions {
		if int64(paper.Versions[i].Version) == number {
			return &paper.Versions[i]
		}
	}
	return nil
}

// ListPapers is list_pair_papers: every sheet this person may see, deadline
// applied.
func ListPapers(s Store, actor Actor, contextID string, now time.Time) ([]PaperSummary, error) {
	if _, err := pairContextOr404(s, actor, contextID); err != nil {
		return nil, err
	}
	if err := requirePairPermission("view_pair_notebook", actor, fact{"is_group_member", true}); err != nil {
		return nil, err
	}
	papers, err := s.ListPairPapers(contextID)
	if err != nil {
		return nil, err
	}
	out := []PaperSummary{}
	for i := range papers {
		paper := &papers[i]
		state := pairpaper.HieuLuc(PaperDict(paper), now)
		if !chiChuThay(paper, state, actor.ID) {
			continue
		}
		var kept *string
		if len(paper.Keeps) > 0 {
			line := paper.Keeps[0].Line
			kept = &line
		}
		out = append(out, PaperSummary{
			ID:         paper.ID,
			State:      state,
			Version:    paper.CurrentVersion,
			Tuan:       paper.Tuan,
			Ngay:       NgayCua(paper),
			ExpiresAt:  paper.ExpiresAt,
			ChangDau:   ChangDau(paper),
			DongGiuDau: kept,
		})
	}
	return out, nil
}

// DraftPaper is draft_pair_paper: the notebook locked (and created), the door,
// one open sheet at a time, then Nep's template for this week.
func DraftPaper(s Store, actor Actor, contextID string, now time.Time) (Command, error) {
	roster, err := pairRosterOr404(s, actor, contextID)
	if err != nil {
		return Command{}, err
	}
	notebook, err := lockedNotebook(s, contextID, now)
	if err != nil {
		return Command{}, err
	}
	if err := requirePairPermission("draft_pair_paper", actor,
		fact{"is_group_member", true},
		fact{"cycle_active_or_temporary", isActive(notebook) || notebook.CycleID == nil},
	); err != nil {
		return Command{}, err
	}
	papers, err := s.ListPairPapers(contextID)
	if err != nil {
		return Command{}, err
	}
	for i := range papers {
		if pairpaper.IsOpen(pairpaper.HieuLuc(PaperDict(&papers[i]), now)) {
			return Command{}, refusal(409, "paper_wrong_state", "Đang có một tờ mở. Xong tờ này đã.")
		}
	}
	// ADR-0034 §2.5: a ceiling per person per week.
	tuanNay := pairpaper.TuanCua(now)
	mine := 0
	for i := range papers {
		if papers[i].DraftOwnerID == actor.ID && papers[i].Tuan.Compare(tuanNay) == 0 {
			mine++
		}
	}
	if mine >= pairpaper.ToMoiNguoiMoiTuan {
		return Command{}, refusal(409, "paper_week_quota", fmt.Sprintf("Tuần này bạn đã phác %d tờ rồi. Tuần sau phác tiếp nhé.", pairpaper.ToMoiNguoiMoiTuan))
	}
	// What this cycle already agreed, and the catalogue around the place it
	// chose -- read before the write, in Python's order.
	lichSu, err := lichSuChuKy(papers, notebook)
	if err != nil {
		return Command{}, err
	}
	var choCu *PlaceRef
	for _, nd := range lichSu {
		if id := nd.Chang[0].PlaceID; id != nil && *id != "" {
			if choCu, err = s.GetPlace(*id); err != nil {
				return Command{}, err
			}
			break
		}
	}
	var ungVien []pairpaper.PlaceRow
	var choCuRow *pairpaper.PlaceRow
	if choCu != nil {
		rows, err := s.ListPlaces(choCu.DestinationID, choCu.Category)
		if err != nil {
			return Command{}, err
		}
		for _, row := range rows {
			ungVien = append(ungVien, row.row())
		}
		r := choCu.row()
		choCuRow = &r
	}
	ngay := pairpaper.NgayDeXuat(now)
	phac, err := pairpaper.PhacToGiay(pairpaper.Routine{Ngay: &ngay, Gio: khungGio, Viec: khungViec}, len(notebook.Constraints) > 0, now)
	if err != nil {
		return Command{}, err
	}
	boxes := make([]string, len(notebook.Constraints))
	for i, c := range notebook.Constraints {
		boxes[i] = c.Content
	}
	phac = pairpaper.LamGiauPhac(phac, lichSu, choCuRow, ungVien, boxes)
	// ADR-0034 §2.2: the tastes of whoever shared theirs, and nobody else's.
	gu, err := guChoNep(s, notebook, roster, now)
	if err != nil {
		return Command{}, err
	}
	if len(gu) > 0 {
		loai := pairpaper.LoaiTheoGu(gu)
		dau := phac.Content.Chang[0]
		var ungVienGu []pairpaper.PlaceRow
		if loai != "" && choCu != nil && (dau.PlaceID == nil || *dau.PlaceID == "") {
			rows, err := s.ListPlaces(choCu.DestinationID, loai)
			if err != nil {
				return Command{}, err
			}
			for _, row := range rows {
				ungVienGu = append(ungVienGu, row.row())
			}
		}
		daDi := []string{}
		for _, nd := range lichSu {
			for _, c := range nd.Chang {
				if c.PlaceID != nil && *c.PlaceID != "" {
					daDi = append(daDi, *c.PlaceID)
				}
			}
		}
		phac = pairpaper.LamGiauTheoGu(phac, gu, ungVienGu, daDi, boxes)
	}
	var lyDo *string
	if phac.LyDo != "" {
		lyDo = &phac.LyDo
	}
	paper, err := s.CreatePairPaper(PaperDraft{
		ContextID:    contextID,
		CycleID:      notebook.CycleID,
		DraftOwnerID: actor.ID,
		Tuan:         pairpaper.TuanCua(now),
		ExpiresAt:    pairpaper.HanTuan(now),
		Content:      phac.Content,
		LyDo:         lyDo,
		Nguon:        phac.Nguon,
		AuthorType:   "human",
		Now:          now,
	})
	if err != nil {
		return Command{}, err
	}
	return wireCommand(&paper, paper.State, nil), nil
}

// chiChuThay is _chi_chu_thay: a sheet nobody ever sent is its owner's draft
// whatever its state -- skipping the week on it, discarding it or letting its
// week run out does not hand it to the other person (QA 24/09).
func chiChuThay(paper *Paper, state, actorID string) bool {
	if paper.DraftOwnerID == actorID {
		return true
	}
	if state == "nhap" {
		return false
	}
	for _, v := range paper.Versions {
		if v.SentAt != nil {
			return true
		}
	}
	return false
}

// lichSuToiDa is _LICH_SU_TOI_DA.
const lichSuToiDa = 4

// lichSuChuKy is _lich_su_chu_ky: the agreed contents of the notebook's
// ACTIVE cycle, newest first (ListPairPapers' order), at most lichSuToiDa. A
// sheet whose stored content does not read is skipped.
func lichSuChuKy(papers []Paper, notebook *Notebook) ([]pairpaper.Content, error) {
	if notebook == nil || notebook.CycleID == nil || !isActive(notebook) {
		return nil, nil
	}
	var out []pairpaper.Content
	for i := range papers {
		paper := &papers[i]
		if paper.CycleID == nil || *paper.CycleID != *notebook.CycleID || paper.IsTemporary ||
			(paper.State != "chot" && paper.State != "da_di" && paper.State != "da_giu") {
			continue
		}
		var current *Version
		for j := range paper.Versions {
			if paper.Versions[j].Version == paper.CurrentVersion {
				current = &paper.Versions[j]
				break
			}
		}
		if current == nil {
			continue
		}
		wire, err := NoiDungWire(current.Content)
		var refused *Refusal
		if errors.As(err, &refused) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if len(wire.Chang) == 0 {
			continue
		}
		content := pairpaper.Content{Ngay: wire.Ngay.ISOFormat()}
		for _, stop := range wire.Chang {
			content.Chang = append(content.Chang, pairpaper.Stop{Gio: stop.Gio, Viec: stop.Viec, PlaceID: stop.PlaceID})
		}
		out = append(out, content)
		if len(out) == lichSuToiDa {
			break
		}
	}
	return out, nil
}

// readablePaperOr404 is _readable_paper_or_404.
func readablePaperOr404(s Store, actor Actor, paperID string) (*Paper, []string, error) {
	paper, err := s.GetPairPaper(paperID)
	if err != nil {
		return nil, nil, err
	}
	if paper == nil {
		return nil, nil, refusal(404, "paper_not_found", "Không có tờ giấy này.")
	}
	members, err := pairContextOr404(s, actor, paper.ContextID)
	if err != nil {
		return nil, nil, err
	}
	if err := requirePairPermission("view_pair_paper", actor,
		fact{"may_view_paper", chiChuThay(paper, paper.State, actor.ID)},
	); err != nil {
		return nil, nil, err
	}
	return paper, members, nil
}

// ReadPaper is pair_paper (GET /papers/{paper_id}).
func ReadPaper(s Store, actor Actor, paperID string, now time.Time) (PaperView, error) {
	paper, _, err := readablePaperOr404(s, actor, paperID)
	if err != nil {
		return PaperView{}, err
	}
	return WirePaper(paper, actor.ID, now)
}

// lockedPaper is _locked_paper: read for permission, then the locked row.
func lockedPaper(s Store, actor Actor, paperID string) (*Paper, error) {
	paper, _, err := readablePaperOr404(s, actor, paperID)
	if err != nil {
		return nil, err
	}
	locked, err := s.LockPairPaper(paper.ID)
	if err != nil {
		return nil, err
	}
	if locked == nil {
		return nil, refusal(404, "paper_not_found", "Không có tờ giấy này.")
	}
	return locked, nil
}

// chuyen is ApiService._chuyen: a pair_paper refusal as a 409 in the person's
// words.
func chuyen(paper *Paper, suKien string, now time.Time, facts pairpaper.Facts) (pairpaper.Paper, error) {
	after, err := pairpaper.Chuyen(PaperDict(paper), suKien, now, facts)
	if refused, ok := err.(*pairpaper.PaperError); ok {
		return pairpaper.Paper{}, refusal(409, refused.Code, paperErrorDetails[refused.Code])
	}
	return after, err
}

// EditDraft is edit_pair_draft.
func EditDraft(s Store, actor Actor, paperID string, content ContentInput, lyDo *string, now time.Time) (Command, error) {
	paper, err := lockedPaper(s, actor, paperID)
	if err != nil {
		return Command{}, err
	}
	if err := requirePairPermission("edit_pair_draft", actor, fact{"is_draft_owner", paper.DraftOwnerID == actor.ID}); err != nil {
		return Command{}, err
	}
	if pairpaper.HieuLuc(PaperDict(paper), now) != "nhap" {
		return Command{}, refusal(409, "paper_wrong_state", "Tờ này đã gửi, sửa thì gửi bản mới.")
	}
	if err := s.UpdatePairDraft(paper.ID, NoiDungLuu(content), stripOrNone(lyDo)); err != nil {
		return Command{}, err
	}
	return wireCommand(paper, paper.State, nil), nil
}

// SendPaper is send_pair_paper: sending writes the sender's own agreement.
func SendPaper(s Store, actor Actor, paperID string, version int64, now time.Time) (Command, error) {
	paper, err := lockedPaper(s, actor, paperID)
	if err != nil {
		return Command{}, err
	}
	if err := requirePairPermission("send_pair_paper", actor,
		fact{"is_draft_owner", paper.DraftOwnerID == actor.ID},
		fact{"version_current", version == int64(paper.CurrentVersion)},
	); err != nil {
		return Command{}, err
	}
	after, err := chuyen(paper, "gui", now, pairpaper.Facts{})
	if err != nil {
		return Command{}, err
	}
	sender := actor.ID
	if err := s.MarkVersionSent(paper.ID, paper.CurrentVersion, &sender, now); err != nil {
		return Command{}, err
	}
	if err := s.AddPaperResponse(paper.ID, paper.CurrentVersion, actor.ID, "dong_y", now); err != nil {
		return Command{}, err
	}
	if err := s.SetPaperState(paper.ID, after.State, now, nil, nil); err != nil {
		return Command{}, err
	}
	return wireCommand(paper, after.State, nil), nil
}

// MarkViewed is mark_pair_paper_viewed.
func MarkViewed(s Store, actor Actor, paperID string, version int64, now time.Time) error {
	paper, err := lockedPaper(s, actor, paperID)
	if err != nil {
		return err
	}
	row := versionNumbered(paper, version)
	if row == nil {
		return refusal(404, "paper_not_found", "Không có phiên bản này.")
	}
	if err := requirePairPermission("view_pair_paper_as_recipient", actor,
		fact{"is_not_version_sender", row.SentBy == nil || *row.SentBy != actor.ID},
	); err != nil {
		return err
	}
	if err := s.MarkPaperViewed(paper.ID, row.Version, actor.ID, now); err != nil {
		return err
	}
	if version == int64(paper.CurrentVersion) && paper.State == "da_gui" {
		after, err := chuyen(paper, "xem", now, pairpaper.Facts{})
		if err != nil {
			return err
		}
		return s.SetPaperState(paper.ID, after.State, now, nil, nil)
	}
	return nil
}

// RespondPaper is respond_pair_paper: «ừ» or a counter-proposal.
func RespondPaper(s Store, actor Actor, paperID string, version int64, reply Reply, now time.Time) (Command, error) {
	paper, err := lockedPaper(s, actor, paperID)
	if err != nil {
		return Command{}, err
	}
	row := versionNumbered(paper, version)
	if row == nil {
		return Command{}, refusal(404, "paper_not_found", "Không có phiên bản này.")
	}
	if err := requirePairPermission("respond_pair_paper", actor,
		fact{"is_not_version_sender", row.SentBy == nil || *row.SentBy != actor.ID},
		fact{"version_current", version == int64(paper.CurrentVersion)},
	); err != nil {
		return Command{}, err
	}
	if reply.Kind == "dong_y" {
		return dongY(s, paper, row.Version, actor, now)
	}
	return deNghiSua(s, paper, row.Version, reply, actor, now)
}

// dongY is _dong_y: a repeated yes reads the sheet back; two yeses on one
// version make the outing.
func dongY(s Store, paper *Paper, version int, actor Actor, now time.Time) (Command, error) {
	if err := s.AddPaperResponse(paper.ID, version, actor.ID, "dong_y", now); err != nil {
		if code, ok := conflictCode(err); !ok || code != "paper_already_agreed" {
			return Command{}, err
		}
		again, err := s.GetPairPaper(paper.ID)
		if err != nil {
			return Command{}, err
		}
		if again == nil {
			return Command{}, refusal(404, "paper_not_found", "Không có tờ giấy này.")
		}
		return wireCommand(again, pairpaper.HieuLuc(PaperDict(again), now), nil), nil
	}
	afterRows, err := s.GetPairPaper(paper.ID)
	if err != nil {
		return Command{}, err
	}
	if afterRows == nil {
		return Command{}, &Invariant{Reason: "get_pair_paper found nothing after add_paper_response"}
	}
	rows := make([]pairpaper.Row, len(afterRows.Responses))
	for i, response := range afterRows.Responses {
		rows[i] = pairpaper.Row{Version: response.Version, PersonID: response.PersonID, Kind: response.Kind}
	}
	after, err := chuyen(paper, "dong_y", now, pairpaper.Facts{DuDongY: pairpaper.DaDuDongY(rows, version)})
	if err != nil {
		return Command{}, err
	}
	if after.State != "chot" {
		if err := s.SetPaperState(paper.ID, after.State, now, nil, nil); err != nil {
			return Command{}, err
		}
		return wireCommand(paper, after.State, nil), nil
	}
	outingID, err := chot(s, afterRows, version, actor, now)
	if err != nil {
		return Command{}, err
	}
	if err := s.SetPaperState(paper.ID, "chot", now, nil, nil); err != nil {
		return Command{}, err
	}
	return wireCommand(paper, "chot", &outingID), nil
}

// OutingTitle is the title _chot gives the outing: what was agreed -- the
// catalogue place's name, or the first stop's line -- then « · dd/mm». Python
// cuts the name at 190 code points (`ten[:190]`), so the title stays inside
// OutingCreateRequest's 200.
func OutingTitle(ten string, ngay pairpaper.Date) string {
	runes := []rune(ten)
	if len(runes) > 190 {
		runes = runes[:190]
	}
	return string(runes) + " · " + twoDigits(ngay.Day) + "/" + twoDigits(ngay.Month)
}

func twoDigits(value int) string {
	if value < 10 {
		return "0" + string(rune('0'+value))
	}
	return string(rune('0'+value/10)) + string(rune('0'+value%10))
}

// chot is _chot: the agreed sheet becomes one outing, or the one it already
// has.
func chot(s Store, paper *Paper, version int, actor Actor, now time.Time) (string, error) {
	existing, err := s.GetPaperOuting(paper.ID)
	if err != nil {
		return "", err
	}
	if existing != nil {
		return *existing, nil
	}
	current := versionNumbered(paper, int64(version))
	if current == nil {
		return "", refusal(409, "paper_wrong_state", "Tờ giấy này không đọc được.")
	}
	content, err := NoiDungWire(current.Content)
	if err != nil {
		return "", err
	}
	// The places the sheet names, read before anything is written: a key the
	// catalogue no longer knows keeps its line and drops its id, because the
	// outing's timeline refuses unknown places (QA 23/09).
	places := make([]*PlaceRef, len(content.Chang))
	for i, stop := range content.Chang {
		if stop.PlaceID == nil {
			continue
		}
		if places[i], err = s.GetPlace(*stop.PlaceID); err != nil {
			return "", err
		}
	}
	ten := content.Chang[0].Viec
	if places[0] != nil {
		ten = places[0].Name
	}
	outingID, err := s.CreateOuting(OutingDraft{
		ContextID:          paper.ContextID,
		CreatedByID:        actor.ID,
		Title:              OutingTitle(ten, content.Ngay),
		StartsOn:           content.Ngay,
		EndsOn:             content.Ngay,
		Headcount:          2,
		BudgetPerPersonVND: 0,
		Now:                now,
	})
	if err != nil {
		return "", err
	}
	if err := s.LinkPaperOuting(paper.ID, version, outingID, now); err != nil {
		code, ok := conflictCode(err)
		if !ok {
			return "", err
		}
		already, err := s.GetPaperOuting(paper.ID)
		if err != nil {
			return "", err
		}
		if already == nil {
			return "", refusal(409, lower(code), "Tờ này đã có buổi đi rồi.")
		}
		return *already, nil
	}
	// The agreed stops become the outing's timeline, in the same transaction.
	stops := make([]OutingStopDraft, len(content.Chang))
	for i, stop := range content.Chang {
		minute, err := gioThanhPhut(stop.Gio)
		if err != nil {
			return "", err
		}
		stops[i] = OutingStopDraft{MinuteOfDay: minute, Label: stop.Viec}
		if places[i] != nil {
			name, id := places[i].Name, places[i].ID
			stops[i].PlaceName, stops[i].PlaceID = &name, &id
		}
	}
	if err := s.ReplaceOutingStops(outingID, stops); err != nil {
		return "", err
	}
	return outingID, nil
}

// gioThanhPhut is _minute_of_day for a stop the schema already held to HH:MM.
func gioThanhPhut(gio string) (int64, error) {
	parts := strings.Split(gio, ":")
	if len(parts) != 2 {
		return 0, &Invariant{Reason: "a stored stop's gio is not HH:MM: " + gio}
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, err
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, err
	}
	return int64(hour*60 + minute), nil
}

// deNghiSua is _de_nghi_sua: a counter-proposal is a new version its author
// has agreed to.
func deNghiSua(s Store, paper *Paper, version int, reply Reply, actor Actor, now time.Time) (Command, error) {
	after, err := chuyen(paper, "de_nghi_sua", now, pairpaper.Facts{})
	if err != nil {
		return Command{}, err
	}
	next := paper.CurrentVersion + 1
	if err := s.AddPaperResponse(paper.ID, version, actor.ID, "de_nghi_sua", now); err != nil {
		return Command{}, err
	}
	if err := s.AddPaperVersion(VersionDraft{
		PaperID:    paper.ID,
		Version:    next,
		Content:    NoiDungLuu(reply.Content),
		LyDo:       stripOrNone(reply.LyDo),
		Nguon:      pairpaper.Nguon{Scope: "chung", Dung: []string{"nguoi"}, Luc: pairpaper.ISOFormat(now)},
		AuthorType: "human",
		SentAt:     now,
		SentBy:     actor.ID,
		Now:        now,
	}); err != nil {
		return Command{}, err
	}
	if err := s.AddPaperResponse(paper.ID, next, actor.ID, "dong_y", now); err != nil {
		return Command{}, err
	}
	if err := s.SetPaperState(paper.ID, after.State, now, &next, nil); err != nil {
		return Command{}, err
	}
	return Command{ID: paper.ID, State: after.State, Version: next, OutingID: paper.OutingID}, nil
}

// WithdrawPaper is withdraw_pair_paper: the door first, then the version.
func WithdrawPaper(s Store, actor Actor, paperID string, version int64, now time.Time) (Command, error) {
	paper, err := lockedPaper(s, actor, paperID)
	if err != nil {
		return Command{}, err
	}
	versions := make([]pairpaper.Version, len(paper.Versions))
	for i, row := range paper.Versions {
		versions[i] = pairpaper.Version{Version: row.Version, AuthorType: row.AuthorType, SentBy: row.SentBy}
	}
	views := make([]pairpaper.Row, len(paper.Views))
	for i, row := range paper.Views {
		views[i] = pairpaper.Row{Version: row.Version, PersonID: row.PersonID}
	}
	responses := make([]pairpaper.Row, len(paper.Responses))
	for i, row := range paper.Responses {
		responses[i] = pairpaper.Row{Version: row.Version, PersonID: row.PersonID, Kind: row.Kind}
	}
	withdrawable := pairpaper.CoTheRut(PaperDict(paper), versions, views, responses, actor.ID)
	if err := requirePairPermission("withdraw_pair_paper", actor,
		fact{"is_group_member", true},
		fact{"paper_unseen_unanswered", withdrawable},
	); err != nil {
		return Command{}, err
	}
	if version != int64(paper.CurrentVersion) {
		return Command{}, refusal(409, "paper_version_stale", "Tờ giấy đã sang phiên bản mới.")
	}
	after, err := chuyen(paper, "rut", now, pairpaper.Facts{CoTheRut: withdrawable})
	if err != nil {
		return Command{}, err
	}
	if err := s.SetPaperState(paper.ID, after.State, now, nil, nil); err != nil {
		return Command{}, err
	}
	return wireCommand(paper, after.State, nil), nil
}

// SkipWeek is skip_pair_week.
func SkipWeek(s Store, actor Actor, paperID string, now time.Time) (Command, error) {
	paper, err := lockedPaper(s, actor, paperID)
	if err != nil {
		return Command{}, err
	}
	if err := requirePairPermission("skip_pair_week", actor, fact{"is_group_member", true}); err != nil {
		return Command{}, err
	}
	after, err := chuyen(paper, "nghi_tuan", now, pairpaper.Facts{})
	if err != nil {
		return Command{}, err
	}
	if err := s.SetPaperState(paper.ID, after.State, now, nil, nil); err != nil {
		return Command{}, err
	}
	return wireCommand(paper, after.State, nil), nil
}

// RecordDone is record_pair_outing_done: only from the day itself, with the
// recorder's name.
func RecordDone(s Store, actor Actor, paperID string, now time.Time) (Command, error) {
	paper, err := lockedPaper(s, actor, paperID)
	if err != nil {
		return Command{}, err
	}
	if err := requirePairPermission("record_pair_outing_done", actor, fact{"is_group_member", true}); err != nil {
		return Command{}, err
	}
	if !CoTheGhiDaDi(paper, now) {
		return Command{}, refusal(409, "paper_wrong_state", "Chưa tới ngày đi.")
	}
	after, err := chuyen(paper, "da_di", now, pairpaper.Facts{NguoiGhi: true})
	if err != nil {
		return Command{}, err
	}
	recorder := actor.ID
	if err := s.SetPaperState(paper.ID, after.State, now, nil, &recorder); err != nil {
		return Command{}, err
	}
	return wireCommand(paper, after.State, nil), nil
}

// KeepLine is keep_pair_paper_line. line is the request's, already stripped
// by its validator.
func KeepLine(s Store, actor Actor, paperID, line string, now time.Time) (Keep, error) {
	paper, err := lockedPaper(s, actor, paperID)
	if err != nil {
		return Keep{}, err
	}
	if err := requirePairPermission("keep_pair_paper_line", actor, fact{"is_group_member", true}); err != nil {
		return Keep{}, err
	}
	after, err := chuyen(paper, "giu", now, pairpaper.Facts{})
	if err != nil {
		return Keep{}, err
	}
	keep, err := s.AddPaperKeep(paper.ID, actor.ID, line, now)
	if err != nil {
		return Keep{}, err
	}
	if err := s.SetPaperState(paper.ID, after.State, now, nil, nil); err != nil {
		return Keep{}, err
	}
	return keep, nil
}

// guChoNep is _gu_cho_nep: nothing outside «Một đôi», nothing for a person
// who has not shared, and interests read only for those who have.
func guChoNep(s Store, notebook *Notebook, roster []Member, now time.Time) ([]pairpaper.GuMuc, error) {
	members := []string{}
	names := map[string]string{}
	for _, row := range roster {
		members = append(members, row.PersonID)
		names[row.PersonID] = row.DisplayName
	}
	participants := Participants(notebook, members)
	consents := ConsentsOf(notebook)
	if !pairnotebook.CanBatDoi(consents, participants, &now) {
		return nil, nil
	}
	chia := []string{}
	for _, person := range participants {
		if slices.Contains(pairnotebook.GrantedBy(consents, person, &now), "chia_gu") {
			chia = append(chia, person)
		}
	}
	if len(chia) == 0 {
		return nil, nil
	}
	tags, err := s.InterestsByPerson(append([]string{}, chia...))
	if err != nil {
		return nil, err
	}
	distinct := map[string]bool{}
	for _, person := range participants {
		distinct[person] = true
	}
	return pairpaper.GuChoNep(chia, tags, names, len(chia) == len(distinct) && len(distinct) == 2), nil
}
