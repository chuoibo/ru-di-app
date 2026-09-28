package nepnho

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/aiharness/trinho"
	"mobile/services/core/internal/auth"
	"mobile/services/core/internal/featureroute"
)

// Handler serves the memory routes of /me/nep (Go-only, features block of
// ownership/routes.json). A caller only ever reaches their own settings and
// memory: the person comes from the bearer session, never from the request.
type Handler struct {
	pool *pgxpool.Pool
	kho  *Kho
	mux  *featureroute.Mux
}

// NewHandler registers the routes. kho may be nil only to list the routes.
func NewHandler(pool *pgxpool.Pool, kho *Kho) *Handler {
	h := &Handler{pool: pool, kho: kho, mux: featureroute.NewMux()}
	h.mux.HandleFunc("GET /me/nep/tri-nho/cai-dat", h.docCaiDat)
	h.mux.HandleFunc("PUT /me/nep/tri-nho/cai-dat", h.datCaiDat)
	h.mux.HandleFunc("POST /me/nep/su-kien", h.suKien)
	h.mux.HandleFunc("DELETE /me/nep/tri-nho", h.quenHet)
	return h
}

// Routes lists the patterns in registration order, read from the mux.
func Routes() []string { return NewHandler(nil, nil).mux.Patterns() }

// Matches takes exactly the memory paths; /me/nep/media and
// /me/nep/ai-invocations stay with their owners.
func Matches(path string) bool {
	switch strings.TrimRight(path, "/") {
	case "/me/nep/tri-nho", "/me/nep/tri-nho/cai-dat", "/me/nep/su-kien":
		return true
	}
	return false
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if h.kho == nil {
		// No memory key on this host: nothing can be remembered, and the
		// toggle is not offered.
		refuse(w, 503, "nep_memory_unavailable")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	h.mux.ServeHTTP(w, r.WithContext(ctx))
}

func reply(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func refuse(w http.ResponseWriter, status int, code string) {
	reply(w, status, map[string]string{"code": code, "detail": code})
}

// person resolves the bearer session to its person, locking person then
// session in the order chatassist uses; no other row is read.
func (h *Handler) person(r *http.Request) (string, int, string) {
	token, p := auth.BearerToken(r.Header)
	if p != nil {
		return "", 401, "authentication_required"
	}
	ctx := r.Context()
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return "", 503, "nep_memory_unavailable"
	}
	defer tx.Rollback(ctx)
	digest := auth.TokenDigest(token)
	var person, exists string
	err = tx.QueryRow(ctx, `SELECT person_id::text FROM account_sessions WHERE token_digest=$1`, digest).Scan(&person)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", 401, "authentication_required"
	}
	if err != nil {
		return "", 503, "nep_memory_unavailable"
	}
	err = tx.QueryRow(ctx, `SELECT id::text FROM people WHERE id=$1 AND deleted_at IS NULL FOR SHARE`, person).Scan(&exists)
	if err == nil {
		err = tx.QueryRow(ctx, `SELECT id::text FROM account_sessions WHERE token_digest=$1 AND person_id=$2 AND revoked_at IS NULL AND expires_at>clock_timestamp() FOR SHARE`, digest, person).Scan(&exists)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return "", 401, "authentication_required"
	}
	if err != nil {
		return "", 503, "nep_memory_unavailable"
	}
	return person, 0, ""
}

const maxBody = 32 << 10

func readBody(w http.ResponseWriter, r *http.Request, v any) (int, string) {
	if strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/json" {
		return 415, "json_required"
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil || d.Decode(new(any)) != io.EOF {
		return 400, "invalid_body"
	}
	return 0, ""
}

type caiDatJSON struct {
	Nho           bool       `json:"nho"`
	CongBoBan     *int16     `json:"cong_bo_ban"`
	CongBoAt      *time.Time `json:"cong_bo_at"`
	CongBoHienTai int        `json:"cong_bo_hien_tai"`
}

func (h *Handler) docCaiDat(w http.ResponseWriter, r *http.Request) {
	person, status, code := h.person(r)
	if status != 0 {
		refuse(w, status, code)
		return
	}
	c, err := h.kho.DocCaiDat(r.Context(), person)
	if err != nil {
		refuse(w, 503, "nep_memory_unavailable")
		return
	}
	reply(w, 200, caiDatJSON{Nho: c.Nho, CongBoBan: c.CongBoBan, CongBoAt: c.CongBoAt, CongBoHienTai: CongBoBan})
}

func (h *Handler) datCaiDat(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Nho       *bool `json:"nho"`
		CongBoBan *int  `json:"cong_bo_ban"`
	}
	if status, code := readBody(w, r, &in); status != 0 {
		refuse(w, status, code)
		return
	}
	if in.Nho == nil || (*in.Nho && in.CongBoBan == nil) || (!*in.Nho && in.CongBoBan != nil) {
		refuse(w, 400, "invalid_body")
		return
	}
	person, status, code := h.person(r)
	if status != 0 {
		refuse(w, status, code)
		return
	}
	ctx := r.Context()
	if *in.Nho {
		if err := h.kho.Bat(ctx, person, *in.CongBoBan); err != nil {
			if errors.Is(err, ErrCongBoCu) {
				refuse(w, 409, "nep_cong_bo_cu")
				return
			}
			refuse(w, 503, "nep_memory_unavailable")
			return
		}
		h.docCaiDat(w, r)
		return
	}
	viec, xong, err := h.kho.Tat(ctx, person)
	if err != nil {
		refuse(w, 503, "nep_memory_unavailable")
		return
	}
	reply(w, 200, map[string]any{"nho": false, "xoa_id": viec, "xoa_xong": xong})
}

type suKienJSON struct {
	Loai          string    `json:"loai"`
	Luc           time.Time `json:"luc"`
	DiaDiemID     string    `json:"dia_diem_id,omitempty"`
	DiemDenID     string    `json:"diem_den_id,omitempty"`
	DanhMuc       string    `json:"danh_muc,omitempty"`
	PhuongTien    string    `json:"phuong_tien,omitempty"`
	ThoiLuongPhut int       `json:"thoi_luong_phut,omitempty"`
}

func (h *Handler) suKien(w http.ResponseWriter, r *http.Request) {
	var in struct {
		SuKien []suKienJSON `json:"su_kien"`
	}
	if status, code := readBody(w, r, &in); status != 0 {
		refuse(w, status, code)
		return
	}
	if len(in.SuKien) == 0 || len(in.SuKien) > MaxSuKienMoiLo {
		refuse(w, 400, "invalid_body")
		return
	}
	ds := make([]SuKien, 0, len(in.SuKien))
	for _, s := range in.SuKien {
		loai, err := trinho.LoaiSuKiens.Parse(s.Loai)
		if err != nil {
			refuse(w, 400, "invalid_body")
			return
		}
		e := SuKien{Loai: loai, Luc: s.Luc, DiaDiemID: s.DiaDiemID, DiemDenID: s.DiemDenID, DanhMuc: s.DanhMuc,
			PhuongTien: s.PhuongTien, ThoiLuongPhut: s.ThoiLuongPhut}
		if e.Kiem() != nil {
			refuse(w, 400, "invalid_body")
			return
		}
		ds = append(ds, e)
	}
	person, status, code := h.person(r)
	if status != 0 {
		refuse(w, status, code)
		return
	}
	ghi, err := h.kho.GhiSuKien(r.Context(), person, ds)
	if err != nil {
		if errors.Is(err, ErrQuaNhanh) {
			refuse(w, 429, "nep_su_kien_qua_nhanh")
			return
		}
		refuse(w, 503, "nep_memory_unavailable")
		return
	}
	if !ghi {
		// Memory is off: refused, nothing written. The app drops its queue
		// and stops sending until the person turns memory on.
		refuse(w, 409, "nep_tri_nho_tat")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) quenHet(w http.ResponseWriter, r *http.Request) {
	person, status, code := h.person(r)
	if status != 0 {
		refuse(w, status, code)
		return
	}
	viec, xong, err := h.kho.QuenHet(r.Context(), person)
	if err != nil {
		refuse(w, 503, "nep_memory_unavailable")
		return
	}
	status = http.StatusOK
	if !xong {
		status = http.StatusAccepted
	}
	reply(w, status, map[string]any{"xoa_id": viec, "xoa_xong": xong})
}
