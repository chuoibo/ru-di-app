package achievementv1

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"mobile/services/core/internal/aiharness/goiy"
	"mobile/services/core/internal/aiharness/motluot"
	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/domain/achievement"
	"mobile/services/core/internal/pyjson"
	"mobile/services/core/internal/repo"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var uuidPath = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type Handler struct {
	Pool *pgxpool.Pool
	Mode string
	// ai is the process's model for Nếp's line on the journey choices
	// (ADR-0052); nil keeps the Go fallback.
	ai *motluot.May
}

func New(pool *pgxpool.Pool, mode string) *Handler { return &Handler{Pool: pool, Mode: mode} }

// WithAI gives the handler the process's model.
func (h *Handler) WithAI(may *motluot.May) *Handler {
	h.ai = may
	return h
}

// RouteIDs names the Go-only public extension for ownership and gate checks.
func RouteIDs() []string {
	return []string{
		"GET /me/achievement-routes",
		"POST /me/achievement-runs",
		"POST /me/achievement-runs/{run_id}/finish",
		"POST /me/achievement-suggestions",
		"PATCH /me/achievement-display",
		"POST /me/achievement-seen",
		"GET /people/{person_id}/achievements",
	}
}

// Matches reserves only new achievement paths; the Python parity manifest
// continues to own every legacy route.
func Matches(path string) bool {
	switch path {
	case "/me/achievement-routes", "/me/achievement-runs", "/me/achievement-suggestions", "/me/achievement-display", "/me/achievement-seen":
		return true
	}
	if strings.HasPrefix(path, "/me/achievement-runs/") && strings.HasSuffix(path, "/finish") {
		parts := strings.Split(path, "/")
		return len(parts) == 5 && uuidPath.MatchString(parts[3])
	}
	if strings.HasPrefix(path, "/people/") && strings.HasSuffix(path, "/achievements") {
		parts := strings.Split(path, "/")
		return len(parts) == 4 && uuidPath.MatchString(parts[2])
	}
	return false
}

type apiError struct {
	Code   string `json:"code"`
	Detail string `json:"detail"`
}

func answer(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func refuse(w http.ResponseWriter, status int, code, detail string) {
	answer(w, status, apiError{Code: code, Detail: detail})
}

func decodeRequest(r *http.Request, out any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(r.Body, maxBody))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return errors.New("trailing JSON")
	}
	return nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !Matches(r.URL.Path) {
		http.NotFound(w, r)
		return
	}
	if h == nil || h.Pool == nil {
		refuse(w, 503, "achievement_unavailable", "Achievement storage is unavailable")
		return
	}
	// The body is received before any transaction opens, under its own
	// deadline, and the whole request has one (audit 2026-10-05,
	// PER-ACHIEVE-01): a slow client used to hold a pool connection inside an
	// open transaction while it sent.
	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()
	r = r.WithContext(ctx)
	if err := bufferBody(w, r); err != nil {
		refuse(w, 400, "invalid_request", "The request body did not arrive")
		return
	}
	tx, err := h.Pool.Begin(ctx)
	if err != nil {
		refuse(w, 503, "achievement_unavailable", "Achievement storage is unavailable")
		return
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	personID, err := h.actor(ctx, tx, r.Header)
	if err != nil {
		var denied *routeError
		if errors.As(err, &denied) {
			refuse(w, denied.Status, denied.Code, denied.Detail)
		} else {
			refuse(w, 503, "achievement_unavailable", "Achievement storage is unavailable")
		}
		return
	}
	store := Store{Q: tx}
	status, body, fault := h.handle(ctx, r, store, personID)
	if fault != nil {
		var refusal *routeError
		if errors.As(fault, &refusal) {
			refuse(w, refusal.Status, refusal.Code, refusal.Detail)
		} else {
			refuse(w, 500, "achievement_unavailable", "Could not read achievements")
		}
		return
	}
	if err := tx.Commit(ctx); err != nil {
		refuse(w, 503, "achievement_unavailable", "Achievement storage is unavailable")
		return
	}
	if ask, ok := body.(suggestionAsk); ok {
		// The model is asked after the read transaction committed: its seat
		// wait and its call no longer hold a pool connection.
		ids, source, line := suggestions(ctx, h.ai, ask.facts, ask.choices, ask.selected, ask.history)
		body = map[string]any{"candidate_ids": ids, "source": source, "line": line}
	}
	answer(w, status, body)
}

// requestTimeout bounds one request: the body, the reads and Nếp's call
// (suggestionTimeout) with room to answer.
const requestTimeout = 30 * time.Second

// bodyReadTimeout bounds receiving a body; maxBody is decodeRequest's cap.
const (
	bodyReadTimeout = 10 * time.Second
	maxBody         = 8192
)

// bufferBody reads the body (one byte past the cap, so decodeRequest still
// sees an oversized body as one) before the transaction opens.
func bufferBody(w http.ResponseWriter, r *http.Request) error {
	if r.Body == nil || r.Body == http.NoBody {
		return nil
	}
	rc := http.NewResponseController(w)
	deadlineSet := rc.SetReadDeadline(time.Now().Add(bodyReadTimeout)) == nil
	data, err := io.ReadAll(io.LimitReader(r.Body, maxBody+1))
	if err != nil {
		return err
	}
	if deadlineSet {
		_ = rc.SetReadDeadline(time.Time{})
	}
	_ = r.Body.Close()
	r.Body = io.NopCloser(bytes.NewReader(data))
	return nil
}

// suggestionAsk is what POST /me/achievement-suggestions read under the
// transaction; the model call runs on it after the commit.
type suggestionAsk struct {
	facts    achievement.Facts
	choices  []achievement.Choice
	selected string
	history  []string
}

func (h *Handler) actor(ctx context.Context, tx pgx.Tx, header http.Header) (string, error) {
	if h.Mode == "dev" {
		actor, problem := auth.DevActor(header)
		if problem != nil {
			return "", bad(problem.Status, problem.Code, problem.Detail)
		}
		return actor.ID, nil
	}
	actor, problem, err := auth.ProdActor(ctx, header, repo.Sessions{Q: tx}, time.Now().UTC())
	if err != nil {
		return "", err
	}
	if problem != nil {
		return "", bad(problem.Status, problem.Code, problem.Detail)
	}
	return actor.ID, nil
}

type routeError struct {
	Status       int
	Code, Detail string
}

func (e *routeError) Error() string             { return e.Code }
func bad(status int, code, detail string) error { return &routeError{status, code, detail} }

type routeSnapshot struct {
	Routes            []achievement.Route   `json:"routes"`
	ActiveRun         *Run                  `json:"active_run"`
	EarnedBadges      []EarnedBadge         `json:"earned_badges"`
	Candidates        []achievement.Choice  `json:"candidates"`
	Chapters          []achievement.Chapter `json:"chapters"`
	MP4Credits        CreditCounts          `json:"mp4_credits"`
	SuggestionPreview map[string]int        `json:"suggestion_preview"`
}

func earnedMap(rows []EarnedBadge) map[string]bool {
	out := make(map[string]bool, len(rows))
	for _, row := range rows {
		out[row.ID] = true
	}
	return out
}

func reconcileOpening(ctx context.Context, s Store, personID string, f achievement.Facts) error {
	for _, id := range achievement.OpeningBadges(f) {
		if err := s.GrantBadge(ctx, personID, id); err != nil {
			return err
		}
	}
	return nil
}

func snapshot(ctx context.Context, s Store, personID string) (routeSnapshot, error) {
	f, err := s.Facts(ctx, personID)
	if err != nil {
		return routeSnapshot{}, err
	}
	if err := reconcileOpening(ctx, s, personID, f); err != nil {
		return routeSnapshot{}, err
	}
	earned, err := s.Earned(ctx, personID)
	if err != nil {
		return routeSnapshot{}, err
	}
	run, err := s.ActiveRun(ctx, personID)
	if err != nil {
		return routeSnapshot{}, err
	}
	credits, err := s.CreditSummary(ctx, personID)
	if err != nil {
		return routeSnapshot{}, err
	}
	history, err := s.RouteHistory(ctx, personID)
	if err != nil {
		return routeSnapshot{}, err
	}
	return routeSnapshot{Routes: achievement.Routes, ActiveRun: run, EarnedBadges: earned, Candidates: achievement.Candidates(f, earnedMap(earned), history...), Chapters: achievement.Chapters(f, earnedMap(earned), history...), MP4Credits: credits, SuggestionPreview: minimalSuggestionSummary(f)}, nil
}

func choiceByID(choices []achievement.Choice, id string) (achievement.Choice, bool) {
	for _, choice := range choices {
		if choice.ID == id {
			return choice, true
		}
	}
	return achievement.Choice{}, false
}

func (h *Handler) handle(ctx context.Context, r *http.Request, s Store, personID string) (int, any, error) {
	path := r.URL.Path
	if path == "/me/achievement-routes" && r.Method == http.MethodGet {
		body, err := snapshot(ctx, s, personID)
		return 200, body, err
	}
	if path == "/me/achievement-runs" && r.Method == http.MethodPost {
		var input struct {
			RouteID  string `json:"route_id"`
			EndingID string `json:"ending_id"`
		}
		if decodeRequest(r, &input) != nil {
			return 400, nil, bad(400, "invalid_request", "Choose one journey ending")
		}
		f, err := s.Facts(ctx, personID)
		if err != nil {
			return 0, nil, err
		}
		earned, err := s.Earned(ctx, personID)
		if err != nil {
			return 0, nil, err
		}
		history, err := s.RouteHistory(ctx, personID)
		if err != nil {
			return 0, nil, err
		}
		choice, ok := choiceByID(achievement.Candidates(f, earnedMap(earned), history...), input.EndingID)
		if !ok || choice.RouteID != input.RouteID {
			return 422, nil, bad(422, "ending_not_available", "This ending is not yet on your map")
		}
		run, err := s.SelectRun(ctx, personID, input.RouteID, input.EndingID)
		return 201, run, err
	}
	if strings.HasPrefix(path, "/me/achievement-runs/") && r.Method == http.MethodPost {
		parts := strings.Split(path, "/")
		runID := parts[3]
		if err := s.LockPerson(ctx, personID); err != nil {
			return 0, nil, err
		}
		run, err := s.RunByID(ctx, personID, runID)
		if err != nil {
			return 0, nil, err
		}
		if run == nil {
			return 404, nil, bad(404, "run_not_found", "This journey was not found")
		}
		if run.FinishedAt != nil {
			return 200, map[string]any{"run": run, "awarded": false}, nil
		}
		if run.ClosedAt != nil {
			return 409, nil, bad(409, "run_replaced", "This journey was replaced by another choice")
		}
		f, err := s.Facts(ctx, personID)
		if err != nil {
			return 0, nil, err
		}
		earned, err := s.Earned(ctx, personID)
		if err != nil {
			return 0, nil, err
		}
		history, err := s.RouteHistory(ctx, personID)
		if err != nil {
			return 0, nil, err
		}
		choice, ok := choiceByID(achievement.Candidates(f, earnedMap(earned), history...), run.EndingID)
		if !ok || !choice.Eligible {
			return 409, nil, bad(409, "ending_not_earned", "The evidence for this ending is not complete")
		}
		already := earnedMap(earned)[choice.ID]
		if err := s.GrantBadge(ctx, personID, choice.ID); err != nil {
			return 0, nil, err
		}
		if choice.ID == "whole_journey" {
			if err := s.GrantCredit(ctx, personID, "true_end"); err != nil {
				return 0, nil, err
			}
		} else {
			firstInRoute := true
			for _, prior := range achievement.Candidates(f, earnedMap(earned), history...) {
				if prior.RouteID == choice.RouteID && prior.Earned {
					firstInRoute = false
					break
				}
			}
			if firstInRoute {
				if err := s.GrantCredit(ctx, personID, "route:"+choice.RouteID); err != nil {
					return 0, nil, err
				}
			}
		}
		if err := s.FinishRun(ctx, personID, runID); err != nil {
			return 0, nil, err
		}
		return 200, map[string]any{"badge_id": choice.ID, "awarded": !already, "reward": choice.Reward}, nil
	}
	if path == "/me/achievement-display" && r.Method == http.MethodPatch {
		var input struct {
			BadgeIDs []string `json:"badge_ids"`
		}
		if decodeRequest(r, &input) != nil {
			return 400, nil, bad(400, "invalid_request", "Choose earned badges to display")
		}
		if len(input.BadgeIDs) > 3 {
			return 422, nil, bad(422, "too_many_badges", "Choose up to three badges")
		}
		earned, err := s.Earned(ctx, personID)
		if err != nil {
			return 0, nil, err
		}
		known := earnedMap(earned)
		seen := map[string]bool{}
		for _, id := range input.BadgeIDs {
			if !known[id] || seen[id] {
				return 422, nil, bad(422, "badge_not_earned", "Choose distinct earned badges")
			}
			seen[id] = true
		}
		if err := s.SetDisplay(ctx, personID, input.BadgeIDs); err != nil {
			return 0, nil, err
		}
		return 200, map[string]any{"badge_ids": input.BadgeIDs}, nil
	}
	if path == "/me/achievement-seen" && r.Method == http.MethodPost {
		// QA UI-160: the book presented these badges as just opened; the next
		// phone or browser does not present them again.
		var input struct {
			BadgeIDs []string `json:"badge_ids"`
		}
		if decodeRequest(r, &input) != nil || len(input.BadgeIDs) == 0 {
			return 400, nil, bad(400, "invalid_request", "Name the badges that were shown")
		}
		earned, err := s.Earned(ctx, personID)
		if err != nil {
			return 0, nil, err
		}
		known := earnedMap(earned)
		if len(input.BadgeIDs) > len(known) {
			return 422, nil, bad(422, "badge_not_earned", "Name distinct earned badges")
		}
		seen := map[string]bool{}
		for _, id := range input.BadgeIDs {
			if !known[id] || seen[id] {
				return 422, nil, bad(422, "badge_not_earned", "Name distinct earned badges")
			}
			seen[id] = true
		}
		if err := s.MarkSeen(ctx, personID, input.BadgeIDs); err != nil {
			return 0, nil, err
		}
		return 200, map[string]any{"badge_ids": input.BadgeIDs}, nil
	}
	if path == "/me/achievement-suggestions" && r.Method == http.MethodPost {
		var input struct {
			Consent bool `json:"consent"`
		}
		if decodeRequest(r, &input) != nil {
			return 400, nil, bad(400, "invalid_request", "Confirm this suggestion request")
		}
		if !input.Consent {
			return 403, nil, bad(403, "consent_required", "Ask Nếp only after previewing and agreeing to this summary")
		}
		f, err := s.Facts(ctx, personID)
		if err != nil {
			return 0, nil, err
		}
		earned, err := s.Earned(ctx, personID)
		if err != nil {
			return 0, nil, err
		}
		run, err := s.ActiveRun(ctx, personID)
		if err != nil {
			return 0, nil, err
		}
		history, err := s.RouteHistory(ctx, personID)
		if err != nil {
			return 0, nil, err
		}
		selected := ""
		if run != nil {
			selected = run.RouteID
		}
		choices := achievement.SuggestedChoices(f, earnedMap(earned), selected, history...)
		return 200, suggestionAsk{facts: f, choices: choices, selected: selected, history: history}, nil
	}
	if strings.HasPrefix(path, "/people/") && r.Method == http.MethodGet {
		parts := strings.Split(path, "/")
		target := parts[2]
		if target != personID {
			blocked, err := s.BlockedBetween(ctx, personID, target)
			if err != nil {
				return 0, nil, err
			}
			if blocked {
				return 403, nil, bad(403, "person_not_visible", "Không xem được hồ sơ này.")
			}
			relations := repo.Repository{Q: s.Q}
			friend, err := relations.AreFriends(ctx, personID, target)
			if err != nil {
				return 0, nil, err
			}
			shared, err := relations.ShareActiveContext(ctx, personID, target)
			if err != nil {
				return 0, nil, err
			}
			if !friend && !shared {
				return 403, nil, bad(403, "person_not_visible", "Không xem được hồ sơ này.")
			}
		}
		var exists bool
		if err := s.Q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM people WHERE id=$1::uuid AND deleted_at IS NULL)`, target).Scan(&exists); err != nil {
			return 0, nil, err
		}
		if !exists {
			return 404, nil, bad(404, "person_not_found", "This profile was not found")
		}
		badges, err := s.PublicDisplay(ctx, target)
		return 200, map[string]any{"person_id": target, "badges": badges}, err
	}
	return 405, nil, bad(405, "method_not_allowed", "This method is not available")
}

func validatedSuggestionIDs(proposed []string, choices []achievement.Choice) []string {
	allowed := map[string]bool{}
	for _, c := range choices {
		if !c.Earned {
			allowed[c.ID] = true
		}
	}
	out := make([]string, 0, 3)
	seen := map[string]bool{}
	for _, id := range proposed {
		if allowed[id] && !seen[id] {
			out = append(out, id)
			seen[id] = true
			if len(out) == 3 {
				break
			}
		}
	}
	return out
}

func fallbackNarration(f achievement.Facts, selected string, history []string) string {
	if f.Checkins == 0 {
		return "Trang đầu còn trắng. Một lần check-in tự khai sẽ mở dấu chân đầu tiên."
	}
	if len(history) >= 2 && history[0] == history[1] {
		return "Bạn đã ghé lại một lối quen. Những trang khác cũng đang chờ được mở."
	}
	if selected == "ky_niem" && f.PhotoDays > 0 {
		return "Những tấm ảnh đã thành dấu mốc. Bạn có thể nối chúng thành câu chuyện của riêng mình."
	}
	if selected == "dong_hanh" && f.SharedOutings > 0 {
		return "Một chuyến đi cùng nhau đã có trong sổ. Xem lối nào dẫn đến cuộc hẹn tiếp theo."
	}
	return "Dấu chân của bạn đã mở vài lối đi. Chọn một trang muốn viết tiếp hôm nay."
}

func safeNarration(raw string) string {
	line := strings.Join(strings.Fields(raw), " ")
	if line == "" || utf8.RuneCountInString(line) > 180 || strings.Contains(strings.ToLower(line), "http") || strings.ContainsAny(line, "@<>") {
		return ""
	}
	return line
}

func minimalSuggestionSummary(f achievement.Facts) map[string]int {
	return map[string]int{
		"checkins":              f.Checkins,
		"distinct_destinations": f.DistinctDestinations,
		"photo_days":            f.PhotoDays,
		"story_days":            f.StoryDays,
		"shared_outings":        f.SharedOutings,
	}
}

// nhanhDuocPhep are the journey directions a choice history may name.
var nhanhDuocPhep = map[string]bool{"dau_chan": true, "ky_niem": true, "dong_hanh": true, "nga_re": true}

// suggestionTimeout bounds Nếp's one call; the fallback answers after it.
const suggestionTimeout = 20 * time.Second

// GoiY is the journey line POST /me/achievement-suggestions serves, after
// consent: candidate ids and one line from the model, or the Go fallback with
// source "go". Exported for cmd/vnlocal-thu's real-request check.
func GoiY(ctx context.Context, may *motluot.May, f achievement.Facts, choices []achievement.Choice, selected string, history []string) ([]string, string, string) {
	return suggestions(ctx, may, f, choices, selected, history)
}

func suggestions(ctx context.Context, may *motluot.May, f achievement.Facts, choices []achievement.Choice, selected string, history []string) ([]string, string, string) {
	offered := []string{}
	for _, c := range choices {
		if !c.Earned {
			offered = append(offered, c.ID)
		}
	}
	fallback := func() []string {
		if len(offered) > 3 {
			return offered[:3]
		}
		return offered
	}
	// The bounds the request was always held to before any call.
	usable := may.CoMay() && len(offered) > 0 && len(offered) <= 9 && len(history) <= 8
	for _, step := range history {
		usable = usable && nhanhDuocPhep[step]
	}
	if !usable {
		return fallback(), "go", fallbackNarration(f, selected, history)
	}
	// Only aggregate counts and candidate identifiers leave Go after consent.
	facts := pyjson.NewOrderedMap()
	summary := minimalSuggestionSummary(f)
	keys := make([]string, 0, len(summary))
	for k := range summary {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		facts.Set(k, pyjson.NewInt(int64(summary[k])))
	}
	prompt, err := goiy.PromptThanhTuu(facts, offered, selected, history)
	if err != nil {
		return fallback(), "go", fallbackNarration(f, selected, history)
	}
	ctx, cancel := context.WithTimeout(ctx, suggestionTimeout)
	defer cancel()
	raw, err := goiy.Goi(ctx, may.Luot(3), prompt)
	if err != nil {
		return fallback(), "go", fallbackNarration(f, selected, history)
	}
	var proposed []string
	list, _ := raw.Get("candidate_ids")
	items, ok := list.(pyjson.List)
	if !ok {
		return fallback(), "go", fallbackNarration(f, selected, history)
	}
	for _, item := range items {
		if s, ok := item.(pyjson.String); ok {
			proposed = append(proposed, string(s))
		}
	}
	lineValue, _ := raw.Get("line")
	rawLine, _ := lineValue.(pyjson.String)
	ids := validatedSuggestionIDs(proposed, choices)
	line := safeNarration(string(rawLine))
	if len(ids) == 0 || line == "" || utf8.RuneCountInString(string(rawLine)) > 180 {
		return fallback(), "go", fallbackNarration(f, selected, history)
	}
	return ids, "ai", line
}
