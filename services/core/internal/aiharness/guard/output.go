package guard

import (
	"regexp"
	"strings"
)

// LoaiRa is why the output guard stopped an answer. It lives in tests and the
// eval only: the metrics row keeps «stopped», never the kind (ADR-0037 §2.8).
type LoaiRa string

const (
	RaSach        LoaiRa = ""
	RaSoDienThoai LoaiRa = "so_dien_thoai"
	RaEmail       LoaiRa = "email"
	RaSoTaiKhoan  LoaiRa = "so_tai_khoan"
	RaTuNhan      LoaiRa = "tu_nhan_hanh_dong"
	RaMaKiem      LoaiRa = "lo_ma_kiem"
	RaLoiNhac     LoaiRa = "lo_loi_nhac"
)

var (
	soDienThoai = regexp.MustCompile(`(?:\+?84|\b0)(?:[\s.\-]?\d){8,10}\b`)
	email       = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	chuoiSo     = regexp.MustCompile(`\d(?:[\s.\-]?\d){8,}`)
	// 1.500.000.000 đồng is an amount, not an account.
	soTien = regexp.MustCompile(`^\d{1,3}(?:[.,]\d{3})+$`)
	// A claim, in Nếp's own voice, of an action it cannot take: Nếp has no
	// tool that moves money, writes an outing or sends anything.
	tuNhan       = regexp.MustCompile(`\b(minh|nep|toi|em|to)\s+(da|vua|da vua)\s+(chuyen (tien|khoan|cho)|ghi (no|vao so|lai khoan|khoan)|thanh toan|tra (tien|no)|tao (keo|nhom|binh chon|lich|khoan|nhac)|chot (keo|lich|dia diem|quan|cho)|dat (ban|cho|lich|ve|phong|keo)|gui (tin|loi moi|tien|cho (ban|moi nguoi|nhom|ca nhom))|nhan tin|moi (ban|nguoi|ca nhom|moi nguoi)|them (ban|nguoi|vao)|xoa (keo|tin|khoan|anh|nhom)|huy (keo|lich|ban|cho)|luu (lai|vao)|hen (gio|lich)|nhac (ban|moi nguoi|ca nhom|lich)|(cap nhat|sua|doi) (keo|lich|khoan))\b`)
	tuNhanDauCau = regexp.MustCompile(`(^|[.!?]\s*)(da|vua) (chuyen (tien|khoan|cho)|ghi no|thanh toan|tao keo|chot keo|dat ban|dat cho|gui (tin|loi moi|tien))\b`)
)

// DauRa is what the output guard needs besides the answer: the canary nonce
// the system instruction carries, and the long lines of the prompt, which an
// answer must never quote.
type DauRa struct {
	MaKiem  string
	LoiNhac []string
}

// Kiem reads a whole answer. S1 never streams, so it reads the full text;
// the streaming window of a later slice calls the same function.
func (d DauRa) Kiem(text string) LoaiRa {
	if d.MaKiem != "" && strings.Contains(strings.ToLower(text), strings.ToLower(d.MaKiem)) {
		return RaMaKiem
	}
	if email.MatchString(text) {
		return RaEmail
	}
	if soDienThoai.MatchString(text) {
		return RaSoDienThoai
	}
	for _, run := range chuoiSo.FindAllString(text, -1) {
		if !soTien.MatchString(run) {
			return RaSoTaiKhoan
		}
	}
	g := Gap(text)
	if tuNhan.MatchString(g) || tuNhanDauCau.MatchString(g) {
		return RaTuNhan
	}
	for _, line := range d.LoiNhac {
		if strings.Contains(g, Gap(line)) {
			return RaLoiNhac
		}
	}
	return RaSach
}
