package dieuchinh

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/domain/allocator"
	"mobile/services/core/internal/httpapi/bodylimit"
	guestmw "mobile/services/core/internal/httpapi/mw/guest"
	"mobile/services/core/internal/repo"
)

// The routes of ADR-0056, all Go-only (python: absent):
//
//	POST /batches/{batch_id}/amendments                          propose
//	GET  /batches/{batch_id}/amendments                          list, for members
//	POST /batches/{batch_id}/amendments/{amendment_id}/decision  answer, in the app
//	GET  /g/{token}/dieu-chinh                                   a guest's review page
//	POST /g/{token}/dieu-chinh                                   a guest's answer
//
// Wrap also marks every request for the succession-aware reads of package
// repo, and answers 409 obligation_superseded for a receipt confirmation on
// an obligation an amendment replaced, before the route behind it runs.

// Routes names the routes for the ownership manifest's features block.
func Routes() []string { return RouteIDs() }

// RouteIDs is Routes as scripts/check_api_contract.py reads it.
func RouteIDs() []string {
	return []string{
		"POST /batches/{batch_id}/amendments",
		"GET /batches/{batch_id}/amendments",
		"POST /batches/{batch_id}/amendments/{amendment_id}/decision",
		"GET /g/{token}/dieu-chinh",
		"POST /g/{token}/dieu-chinh",
	}
}

var uuidText = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// Matches reserves exactly the routes above.
func Matches(path string) bool {
	p := strings.Split(strings.Trim(path, "/"), "/")
	switch {
	case len(p) == 3 && p[0] == "batches" && p[2] == "amendments":
		return true
	case len(p) == 5 && p[0] == "batches" && p[2] == "amendments" && p[4] == "decision":
		return true
	case len(p) == 3 && p[0] == "g" && p[2] == "dieu-chinh":
		return true
	}
	return false
}

// Handler serves the routes; Mode is MOBILE_AUTH_MODE.
type Handler struct {
	Store Store
	Mode  string
	guest http.Handler
}

func New(pool *pgxpool.Pool, mode string) *Handler {
	h := &Handler{Store: Store{Pool: pool}, Mode: mode}
	h.guest = guestmw.Middleware(func(r *http.Request) string { return r.URL.Path }, http.HandlerFunc(h.servePage))
	return h
}

// Wrap puts the handler in front of next; cors wraps the app routes, as
// it wraps every other feature's.
func (h *Handler) Wrap(next http.Handler, cors func(http.Handler) http.Handler) http.Handler {
	api := cors(bodylimit.Wrap(64<<10, 30*time.Second, http.HandlerFunc(h.serveAPI)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(repo.WithSuccessions(r.Context()))
		switch {
		case Matches(r.URL.Path) && strings.HasPrefix(r.URL.Path, "/g/"):
			h.guest.ServeHTTP(w, r)
		case Matches(r.URL.Path):
			api.ServeHTTP(w, r)
		case h.superseded(w, r):
		default:
			next.ServeHTTP(w, r)
		}
	})
}

func answer(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func fail(w http.ResponseWriter, err error) {
	var r *Refusal
	if errors.As(err, &r) {
		answer(w, r.Status, map[string]string{"code": r.Code, "detail": r.Detail})
		return
	}
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		answer(w, 413, map[string]string{"code": "request_body_too_large", "detail": "Request body is too large"})
		return
	}
	answer(w, 503, map[string]string{"code": "amendments_unavailable", "detail": "Amendments are unavailable"})
}

var sessionInvalid = refuse(401, "authentication_required", "Session is not valid")

// who is the request's person, read in the acting transaction: in prod the
// bearer's session and person, both held FOR SHARE until it ends.
func (h *Handler) who(r *http.Request) (Who, error) {
	if h.Mode == "dev" {
		actor, problem := auth.DevActor(r.Header)
		if problem != nil {
			return nil, refuse(problem.Status, problem.Code, problem.Detail)
		}
		return As(actor.ID), nil
	}
	token, problem := auth.BearerToken(r.Header)
	if problem != nil {
		return nil, refuse(problem.Status, problem.Code, problem.Detail)
	}
	digest := auth.TokenDigest(token)
	return func(ctx context.Context, tx pgx.Tx) (string, error) {
		var person string
		err := tx.QueryRow(ctx, `SELECT s.person_id::text FROM account_sessions s JOIN people p ON p.id = s.person_id
			 WHERE s.token_digest=$1 AND s.revoked_at IS NULL AND s.expires_at > now() AND p.deleted_at IS NULL
			   FOR SHARE OF s, p`, digest).Scan(&person)
		if errors.Is(err, pgx.ErrNoRows) {
			return "", sessionInvalid
		}
		return person, err
	}, nil
}

func (h *Handler) serveAPI(w http.ResponseWriter, r *http.Request) {
	p := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if !uuidText.MatchString(p[1]) || (len(p) == 5 && !uuidText.MatchString(p[3])) {
		fail(w, refuse(404, "batch_not_found", "Batch does not exist"))
		return
	}
	who, err := h.who(r)
	if err != nil {
		fail(w, err)
		return
	}
	ctx := r.Context()
	switch {
	case len(p) == 3 && r.Method == http.MethodGet:
		list, err := h.Store.List(ctx, p[1], who)
		if err != nil {
			fail(w, err)
			return
		}
		answer(w, 200, list)
	case len(p) == 3 && r.Method == http.MethodPost:
		var body struct {
			ExpenseID   string                     `json:"expense_id"`
			Reason      string                     `json:"reason"`
			Allocations map[string]json.RawMessage `json:"allocations"`
		}
		if err := strictJSON(r, &body); err != nil {
			fail(w, err)
			return
		}
		if !uuidText.MatchString(body.ExpenseID) {
			fail(w, refuse(422, "invalid_expense_id", "expense_id must be a UUID"))
			return
		}
		allocations := map[string]int64{}
		for person, raw := range body.Allocations {
			amount, ok := wholeDong(raw)
			if !uuidText.MatchString(person) || !ok {
				fail(w, refuse(422, "allocation_amount_invalid", "Each allocation is a person id and whole đồng"))
				return
			}
			allocations[person] = amount
		}
		got, err := h.Store.ProposeAs(ctx, ProposeInput{BatchID: p[1], ExpenseID: body.ExpenseID, Reason: body.Reason, Allocations: allocations}, who)
		if err != nil {
			fail(w, err)
			return
		}
		answer(w, 201, got)
	case len(p) == 5 && r.Method == http.MethodPost:
		var body struct {
			Accept *bool `json:"accept"`
		}
		if err := strictJSON(r, &body); err != nil {
			fail(w, err)
			return
		}
		if body.Accept == nil {
			fail(w, refuse(422, "decision_invalid", "Say accept true or false"))
			return
		}
		status, err := h.Store.DecideInBatch(ctx, p[1], p[3], who, *body.Accept)
		if err != nil {
			fail(w, err)
			return
		}
		answer(w, 200, map[string]string{"amendment_id": p[3], "status": status})
	default:
		w.Header().Set("Allow", map[bool]string{true: "GET, POST", false: "POST"}[len(p) == 3])
		fail(w, refuse(405, "method_not_allowed", "Method not allowed"))
	}
}

func strictJSON(r *http.Request, into any) error {
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		return refuse(415, "unsupported_media_type", "Send application/json")
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(into); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return err
		}
		return refuse(422, "invalid_body", "The body is not the expected JSON object")
	}
	if decoder.More() {
		return refuse(422, "invalid_body", "One JSON object only")
	}
	return nil
}

// wholeDong is a JSON integer of đồng: no fraction, no exponent, no sign
// but a leading minus, within the amount ceiling (money law 1).
func wholeDong(raw json.RawMessage) (int64, bool) {
	text := strings.TrimSpace(string(raw))
	if text == "" || len(text) > 13 || strings.ContainsAny(text, ".eE+") {
		return 0, false
	}
	var n int64
	if err := json.Unmarshal([]byte(text), &n); err != nil || n < 0 || n > int64(allocator.MaxAmountVND) {
		return 0, false
	}
	return n, true
}

// superseded answers a receipt confirmation on an obligation an applied
// amendment replaced: 409 with its successor, the route never runs. Only a
// guard of convenience: a receipt that slips past it, between this read and
// an amendment applying, still counts, on the pair and down the chain.
func (h *Handler) superseded(w http.ResponseWriter, r *http.Request) bool {
	p := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if r.Method != http.MethodPost || len(p) != 3 || p[0] != "obligations" || p[2] != "confirm-receipt" || !uuidText.MatchString(p[1]) {
		return false
	}
	// Only a member of the obligation's group learns it was replaced, and by
	// what; anyone else reaches the route, which answers as it always has.
	who, err := h.who(r)
	if err != nil {
		return false
	}
	tx, err := h.Store.Pool.Begin(r.Context())
	if err != nil {
		return false
	}
	defer tx.Rollback(context.WithoutCancel(r.Context()))
	actor, err := who(r.Context(), tx)
	if err != nil {
		return false
	}
	var member bool
	if err := tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM collection_obligations o
		  JOIN collection_batch_versions v ON v.id = o.batch_version_id JOIN collection_batches b ON b.id = v.batch_id
		  JOIN memberships m ON m.context_id = b.context_id AND m.person_id = $2::uuid AND m.state = 'active' AND m.left_at IS NULL
		 WHERE o.id = $1::uuid)`, p[1], actor).Scan(&member); err != nil || !member {
		return false
	}
	var successor string
	err = tx.QueryRow(r.Context(), `WITH RECURSIVE forward(id) AS (
			SELECT $1::uuid
			UNION SELECT s.new_obligation_id FROM collection_obligation_successions s JOIN forward ON s.old_obligation_id = forward.id)
		SELECT id::text FROM forward WHERE id <> $1::uuid
		   AND NOT EXISTS (SELECT 1 FROM collection_obligation_successions s WHERE s.old_obligation_id = forward.id)`, p[1]).Scan(&successor)
	if err != nil {
		// No successor, or the read failed: the route answers as it would.
		return false
	}
	answer(w, 409, map[string]string{"code": "obligation_superseded",
		"detail": "An amendment replaced this obligation; confirm the receipt on its successor", "successor_obligation_id": successor})
	return true
}

// AmendmentView is one amendment as a member of its group sees it.
type AmendmentView struct {
	ID           string       `json:"id"`
	Status       string       `json:"status"`
	ExpenseID    string       `json:"expense_id"`
	ProposedByID string       `json:"proposed_by_id"`
	Reason       string       `json:"reason"`
	CreatedAt    time.Time    `json:"created_at"`
	ExpiresAt    time.Time    `json:"expires_at"`
	ResolvedAt   *time.Time   `json:"resolved_at"`
	Lines        []LineView   `json:"lines"`
	Parties      []PartyView  `json:"parties"`
	Applied      *string      `json:"applied_batch_version_id"`
	Pending      []string     `json:"pending_person_ids"`
	ReviewLinks  []ReviewLink `json:"-"`
}

type LineView struct {
	SenderID     string `json:"sender_id"`
	RecipientID  string `json:"recipient_id"`
	OldAmountVND int64  `json:"old_amount_vnd"`
	NewAmountVND int64  `json:"new_amount_vnd"`
}

type PartyView struct {
	PersonID string  `json:"person_id"`
	Decision *string `json:"decision"`
	Via      *string `json:"via"`
}

// ExpenseView is one expense of the batch's latest version as an amendment
// starts from: its allocations now, whole đồng, every participant.
type ExpenseView struct {
	ExpenseID    string           `json:"expense_id"`
	VersionID    string           `json:"expense_version_id"`
	Description  *string          `json:"description"`
	PaidByID     string           `json:"paid_by_id"`
	RecordedByID string           `json:"recorded_by_id"`
	TotalVND     int64            `json:"total_amount_vnd"`
	Allocations  []AllocationView `json:"allocations"`
}

type AllocationView struct {
	PersonID  string `json:"person_id"`
	AmountVND int64  `json:"amount_vnd"`
}

// Listing is GET /batches/{batch_id}/amendments.
type Listing struct {
	BatchOwnerID string          `json:"batch_owner_id"`
	BatchStatus  string          `json:"batch_status"`
	Expenses     []ExpenseView   `json:"expenses"`
	Amendments   []AmendmentView `json:"amendments"`
}

// List is the batch's amendments, newest first, and the expenses one may
// start from, for an active member of its group; an amendment past its
// expiry is ended first.
func (s Store) List(ctx context.Context, batchID string, who Who) (Listing, error) {
	amendments, b, err := s.list(ctx, batchID, who)
	if err != nil {
		return Listing{}, err
	}
	out := Listing{BatchOwnerID: b.OwnerID, BatchStatus: b.Status, Amendments: amendments, Expenses: []ExpenseView{}}
	for _, src := range b.Sources {
		e := ExpenseView{ExpenseID: src.ExpenseID, VersionID: src.ID, Description: src.Description, PaidByID: src.PaidByID,
			RecordedByID: src.RecordedByID, Allocations: []AllocationView{}}
		for _, a := range src.Allocations {
			e.Allocations = append(e.Allocations, AllocationView{PersonID: a.ParticipantID, AmountVND: a.AmountVND})
			e.TotalVND += a.AmountVND
		}
		out.Expenses = append(out.Expenses, e)
	}
	return out, nil
}

func (s Store) list(ctx context.Context, batchID string, who Who) ([]AmendmentView, *batchState, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	actor, err := who(ctx, tx)
	if err != nil {
		return nil, nil, err
	}
	b, err := loadBatch(ctx, tx, batchID)
	if err != nil {
		return nil, nil, err
	}
	if ok, err := activeMember(ctx, tx, b.ContextID, actor); err != nil || !ok {
		if err == nil {
			err = refuse(404, "batch_not_found", "Batch does not exist")
		}
		return nil, nil, err
	}
	if err := expireDue(ctx, tx, batchID, s.now()); err != nil {
		return nil, nil, err
	}
	rows, err := tx.Query(ctx, `SELECT id::text, status, expense_id::text, proposed_by_id::text, reason, created_at, expires_at, resolved_at, applied_batch_version_id::text
		FROM collection_amendments WHERE batch_id=$1::uuid ORDER BY created_at DESC, id`, batchID)
	if err != nil {
		return nil, nil, err
	}
	out := []AmendmentView{}
	for rows.Next() {
		var a AmendmentView
		if err := rows.Scan(&a.ID, &a.Status, &a.ExpenseID, &a.ProposedByID, &a.Reason, &a.CreatedAt, &a.ExpiresAt, &a.ResolvedAt, &a.Applied); err != nil {
			rows.Close()
			return nil, nil, err
		}
		a.CreatedAt, a.ExpiresAt = a.CreatedAt.UTC(), a.ExpiresAt.UTC()
		if a.ResolvedAt != nil {
			at := a.ResolvedAt.UTC()
			a.ResolvedAt = &at
		}
		out = append(out, a)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	for i := range out {
		a := &out[i]
		a.Lines, a.Parties, a.Pending = []LineView{}, []PartyView{}, []string{}
		lines, err := tx.Query(ctx, `SELECT sender_id::text, recipient_id::text, old_amount_vnd, new_amount_vnd FROM collection_amendment_lines WHERE amendment_id=$1::uuid ORDER BY sender_id, recipient_id`, a.ID)
		if err != nil {
			return nil, nil, err
		}
		for lines.Next() {
			var l LineView
			if err := lines.Scan(&l.SenderID, &l.RecipientID, &l.OldAmountVND, &l.NewAmountVND); err != nil {
				lines.Close()
				return nil, nil, err
			}
			a.Lines = append(a.Lines, l)
		}
		lines.Close()
		parties, err := tx.Query(ctx, `SELECT p.person_id::text, CASE WHEN d.accept THEN 'accept' WHEN NOT d.accept THEN 'reject' END, d.via
			FROM collection_amendment_parties p LEFT JOIN collection_amendment_decisions d ON d.amendment_id = p.amendment_id AND d.person_id = p.person_id
			WHERE p.amendment_id=$1::uuid ORDER BY p.person_id`, a.ID)
		if err != nil {
			return nil, nil, err
		}
		for parties.Next() {
			var p PartyView
			if err := parties.Scan(&p.PersonID, &p.Decision, &p.Via); err != nil {
				parties.Close()
				return nil, nil, err
			}
			a.Parties = append(a.Parties, p)
			if p.Decision == nil {
				a.Pending = append(a.Pending, p.PersonID)
			}
		}
		parties.Close()
	}
	return out, b, tx.Commit(ctx)
}

// DecideInBatch is an app answer to an amendment of the batch in the path.
func (s Store) DecideInBatch(ctx context.Context, batchID, amendmentID string, who Who, accept bool) (string, error) {
	// Who first: nothing about the amendment is said to an unauthenticated
	// caller, and its batch is checked in the same transaction.
	inBatch := func(ctx context.Context, tx pgx.Tx) (string, error) {
		person, err := who(ctx, tx)
		if err != nil {
			return "", err
		}
		var ok bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM collection_amendments WHERE id=$1::uuid AND batch_id=$2::uuid)`, amendmentID, batchID).Scan(&ok); err != nil {
			return "", err
		}
		if !ok {
			return "", refuse(404, "amendment_not_found", "No such amendment")
		}
		return person, nil
	}
	return s.DecideAs(ctx, amendmentID, inBatch, accept, "app")
}

//go:embed page.html
var pageFiles embed.FS

var reviewPage = template.Must(template.New("page.html").Funcs(template.FuncMap{"deref": func(b *bool) bool { return b != nil && *b }}).ParseFS(pageFiles, "page.html"))

// GuestLine is one obligation of the guest's as the review page shows it.
type GuestLine struct {
	RecipientName string
	OldVND        string
	NewVND        string
}

// GuestReview is the review page's view model: the guest's own lines only,
// never another sender's, never a balance (the guest view's leak boundary).
type GuestReview struct {
	ProposerName string
	Reason       string
	ExpiresAt    string
	Lines        []GuestLine
	Answered     *bool
	Status       string
	Token        string
}

// review reads the token's amendment for its sender alone. found is false for
// a token no review link carries.
func (s Store) review(ctx context.Context, token string) (GuestReview, string, string, bool, error) {
	v := GuestReview{Token: token}
	var amendmentID, sender string
	var expires time.Time
	err := s.Pool.QueryRow(ctx, `SELECT a.id::text, l.sender_id::text, a.status, a.reason, a.expires_at, coalesce(p.display_name, '')
		  FROM collection_amendment_links l JOIN collection_amendments a ON a.id = l.amendment_id
		  LEFT JOIN people p ON p.id = a.proposed_by_id AND p.deleted_at IS NULL
		 WHERE l.token_digest=$1 AND l.expires_at > $2`, auth.TokenDigest(token), s.now()).Scan(&amendmentID, &sender, &v.Status, &v.Reason, &expires, &v.ProposerName)
	if errors.Is(err, pgx.ErrNoRows) {
		return v, "", "", false, nil
	}
	if err != nil {
		return v, "", "", false, err
	}
	if v.Status == "proposed" && !expires.After(s.now()) {
		v.Status = "expired"
	}
	v.ExpiresAt = expires.In(vietnam).Format("15:04 02/01/2006")
	rows, err := s.Pool.Query(ctx, `SELECT coalesce(p.display_name, ''), l.old_amount_vnd, l.new_amount_vnd
		  FROM collection_amendment_lines l LEFT JOIN people p ON p.id = l.recipient_id AND p.deleted_at IS NULL
		 WHERE l.amendment_id=$1::uuid AND l.sender_id=$2::uuid ORDER BY l.recipient_id`, amendmentID, sender)
	if err != nil {
		return v, "", "", false, err
	}
	for rows.Next() {
		var name string
		var old, next int64
		if err := rows.Scan(&name, &old, &next); err != nil {
			rows.Close()
			return v, "", "", false, err
		}
		v.Lines = append(v.Lines, GuestLine{RecipientName: name, OldVND: dong(old), NewVND: dong(next)})
	}
	rows.Close()
	var accept *bool
	err = s.Pool.QueryRow(ctx, `SELECT accept FROM collection_amendment_decisions WHERE amendment_id=$1::uuid AND person_id=$2::uuid`, amendmentID, sender).Scan(&accept)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return v, "", "", false, err
	}
	v.Answered = accept
	return v, amendmentID, sender, true, nil
}

var vietnam = time.FixedZone("ICT", 7*3600)

// dong writes whole đồng with dots between thousands: 150.000.
func dong(n int64) string {
	digits := []byte(itoa(n))
	var out []byte
	for i, c := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			out = append(out, '.')
		}
		out = append(out, c)
	}
	return string(out)
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func (h *Handler) servePage(w http.ResponseWriter, r *http.Request) {
	token := strings.Split(strings.Trim(r.URL.Path, "/"), "/")[1]
	ctx := r.Context()
	if r.Method == http.MethodPost {
		h.answerPage(w, r, token)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method not allowed", 405)
		return
	}
	v, _, _, found, err := h.Store.review(ctx, token)
	switch {
	case err != nil:
		http.Error(w, "Trang tạm thời không mở được", 503)
		return
	case !found:
		http.Error(w, "Link không còn dùng được", 404)
		return
	case v.Status == "applied" || v.Status == "rejected":
		// Resolved: the same token is a guest link now, to what it shows.
		http.Redirect(w, r, "/g/"+token, http.StatusSeeOther)
		return
	}
	var page bytes.Buffer
	if err := reviewPage.Execute(&page, v); err != nil {
		http.Error(w, "Trang tạm thời không mở được", 503)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'self'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(page.Bytes())
}

func (h *Handler) answerPage(w http.ResponseWriter, r *http.Request, token string) {
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Yêu cầu không hợp lệ", 400)
		return
	}
	var accept bool
	switch r.PostForm.Get("tra_loi") {
	case "dong_y":
		accept = true
	case "khong_dong_y":
	default:
		http.Error(w, "Yêu cầu không hợp lệ", 400)
		return
	}
	digest := auth.TokenDigest(token)
	var amendmentID string
	if err := h.Store.Pool.QueryRow(r.Context(), `SELECT amendment_id::text FROM collection_amendment_links WHERE token_digest=$1`, digest).Scan(&amendmentID); err != nil {
		http.Error(w, "Link không còn dùng được", 404)
		return
	}
	who := func(ctx context.Context, tx pgx.Tx) (string, error) {
		var sender string
		// The review token lives no longer than the link it replaced.
		err := tx.QueryRow(ctx, `SELECT sender_id::text FROM collection_amendment_links WHERE token_digest=$1 AND expires_at > now() FOR SHARE`, digest).Scan(&sender)
		if errors.Is(err, pgx.ErrNoRows) {
			return "", refuse(404, "link_not_usable", "This link cannot be used")
		}
		return sender, err
	}
	_, err := h.Store.DecideAs(r.Context(), amendmentID, who, accept, "guest_link")
	var refusal *Refusal
	if err != nil && !errors.As(err, &refusal) {
		http.Error(w, "Trang tạm thời không mở được", 503)
		return
	}
	// Answered, already answered or already ended: the page says which.
	http.Redirect(w, r, "/g/"+token+"/dieu-chinh", http.StatusSeeOther)
}
