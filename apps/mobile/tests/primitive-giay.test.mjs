/**
 * Số học của các primitive giấy (UI v3 S0.5): lịch xé, đĩa xoay, câu rủ có ô
 * trống, bộ nhớ ảnh đại diện hỏng. Chạy trên hàm thật.
 *
 * Lịch được đối chiếu CHÉO với `ngayVeISO` (bộ đọc ngày app đang dùng để gửi
 * lên máy chủ) qua mười năm ngày: một ngày lịch xé sinh ra phải là ngày bộ đọc
 * đó nhận, và một ngày bộ đọc từ chối (31/02) thì lịch cũng không có.
 */
import assert from "node:assert/strict";
import test from "node:test";

import { THU_NGAN, congNgay, dinhDangNgay, docNgay, laNamNhuan, luoiThang, soNgayTrongThang, tenThu, thangSau, thuTrongTuan } from "../dist-test/rudi/ui/lich/lich-thang.js";
import { NAC_MOT_NGAY, buocGioiHan, docGio, docGioThanhLoi, gioTuNac, gocTuCham, gocTuNac, nacGio, nacTiep, nacTuGoc } from "../dist-test/rudi/ui/ban-xoay.js";
import { tachCau } from "../dist-test/rudi/ui/cau-ru.js";
import { TRAN_ANH_HONG, anhDaHong, danhDauAnhHong, quenAnhHong } from "../dist-test/rudi/nguoi/anh-dai-dien-cache.js";
import { ngayVeISO } from "../dist-test/rudi/chat/to-hen-chung.js";

test("lịch: mười năm ngày, mỗi ngày lịch sinh ra là ngày ngayVeISO nhận, thứ khớp Date của hệ thống", () => {
  let n = { ngay: 1, thang: 1, nam: 2024 };
  let dem = 0;
  while (n.nam < 2034) {
    const chu = dinhDangNgay(n);
    const iso = ngayVeISO(chu);
    assert.equal(iso, `${n.nam}-${String(n.thang).padStart(2, "0")}-${String(n.ngay).padStart(2, "0")}`, chu);
    assert.deepEqual(docNgay(chu), n);
    const thuHeThong = (new Date(`${iso}T00:00:00Z`).getUTCDay() + 6) % 7;
    assert.equal(thuTrongTuan(n), thuHeThong, `${chu}: thứ`);
    n = congNgay(n, 1);
    dem += 1;
  }
  assert.equal(dem, 3653, "mười năm có 3 653 ngày (ba năm nhuận)");
  // what the typed reader refuses, the calendar does not have either
  for (const sai of ["31/02/2026", "29/02/2025", "00/01/2026", "12/13/2026", "1/1/26", "31/04/2026"]) {
    assert.equal(docNgay(sai), null, sai);
    assert.equal(ngayVeISO(sai), null, sai);
  }
  assert.deepEqual(docNgay("29/02/2028"), { ngay: 29, thang: 2, nam: 2028 });
  assert.deepEqual(docNgay("5/9/2026"), { ngay: 5, thang: 9, nam: 2026 }, "một chữ số vẫn đọc");
  assert.ok(laNamNhuan(2000) && !laNamNhuan(1900) && laNamNhuan(2028));
  assert.equal(soNgayTrongThang(2, 2026), 28);
});

test("lưới tháng: 42 ô, bắt đầu thứ Hai, đủ mọi ngày của tháng, đúng thứ tự", () => {
  for (const [thang, nam] of [[9, 2026], [2, 2026], [2, 2028], [12, 2026], [1, 2027], [6, 2025]]) {
    const o = luoiThang(thang, nam);
    assert.equal(o.length, 42);
    assert.equal(thuTrongTuan(o[0]), 0, `${thang}/${nam}: ô đầu là thứ Hai`);
    const trong = o.filter((x) => x.trongThang).map((x) => x.ngay);
    assert.deepEqual(trong, Array.from({ length: soNgayTrongThang(thang, nam) }, (_, i) => i + 1));
    for (let i = 1; i < 42; i += 1) assert.deepEqual(congNgay(o[i - 1], 1), { ngay: o[i].ngay, thang: o[i].thang, nam: o[i].nam });
  }
  assert.equal(THU_NGAN.length, 7);
  assert.deepEqual(thangSau(12, 2026, 1), { thang: 1, nam: 2027 });
  assert.deepEqual(thangSau(1, 2026, -1), { thang: 12, nam: 2025 });
  assert.equal(tenThu({ ngay: 25, thang: 9, nam: 2026 }), "Thứ Sáu");
  assert.equal(tenThu({ ngay: 27, thang: 9, nam: 2026 }), "Chủ nhật");
});

test("đĩa xoay: góc ra nấc, nấc ra góc, giờ đọc được, qua 12 thì đi tiếp chứ không nhảy về 0", () => {
  const tam = { x: 100, y: 100 };
  assert.equal(Math.round(gocTuCham(tam, { x: 100, y: 0 })), 0, "trên cùng là 0");
  assert.equal(Math.round(gocTuCham(tam, { x: 200, y: 100 })), 90, "bên phải là 90");
  assert.equal(Math.round(gocTuCham(tam, { x: 100, y: 200 })), 180);
  assert.equal(Math.round(gocTuCham(tam, { x: 0, y: 100 })), 270);
  for (let n = 0; n < 48; n += 1) assert.equal(nacTuGoc(gocTuNac(n, 48), 48), n);
  assert.equal(nacTuGoc(359, 48), 0, "sát 360 là về nấc 0");
  // a 12-hour face turned twice: 11:45 -> the next quarter is 12:00, not 0:00
  assert.equal(nacTiep(47, 0, 48), 48);
  assert.equal(nacTiep(48, 47, 48), 47, "lùi qua 12 thì về 11:45");
  assert.equal(nacTiep(10, 12, 48), 12);
  assert.equal(gioTuNac(nacGio(18 * 60 + 30)), "18:30");
  assert.equal(gioTuNac(nacGio(18 * 60 + 38)), "18:45", "tròn về nấc 15 phút gần nhất");
  assert.equal(gioTuNac(NAC_MOT_NGAY + 2), "00:30", "qua nửa đêm thì quay vòng");
  assert.equal(gioTuNac(-1), "23:45");
  assert.equal(docGio("6:05"), 365);
  assert.equal(docGio("18h30"), 1110);
  assert.equal(docGio("24:00"), null);
  assert.equal(docGio("7:5"), null);
  assert.equal(docGioThanhLoi(nacGio(18 * 60 + 30)), "18 giờ 30");
  assert.equal(docGioThanhLoi(nacGio(7 * 60)), "7 giờ");
  assert.equal(buocGioiHan(3, 1, 0, 3), 3);
  assert.equal(buocGioiHan(0, -1, 0, 3), 0);
});

test("câu rủ: chữ tách theo từ, ô trống giữ dấu câu đi sau, ô lạ để nguyên là chữ", () => {
  const p = tachCau("Rủ {nhom} đi {ten} từ {tu} tới {den}, {so} người", ["nhom", "ten", "tu", "den", "so"]);
  assert.deepEqual(p.map((x) => (x.kieu === "o" ? `[${x.ten}${x.sau}]` : x.chu)), ["Rủ", "[nhom]", "đi", "[ten]", "từ", "[tu]", "tới", "[den,]", "[so]", "người"]);
  const la = tachCau("Rủ {khong-biet} đi", ["nhom"]);
  assert.deepEqual(la, [{ kieu: "chu", chu: "Rủ" }, { kieu: "chu", chu: "{khong-biet}" }, { kieu: "chu", chu: "đi" }]);
  assert.deepEqual(tachCau("   ", []), []);
});

test("ảnh đại diện hỏng: nhớ trong phiên, có trần, người vừa tải ảnh mới thì được thử lại", () => {
  quenAnhHong();
  danhDauAnhHong("a");
  assert.ok(anhDaHong("a"));
  quenAnhHong("a");
  assert.ok(!anhDaHong("a"));
  for (let i = 0; i <= TRAN_ANH_HONG; i += 1) danhDauAnhHong(`p${i}`);
  assert.ok(!anhDaHong("p0"), "cũ nhất bị quên");
  assert.ok(anhDaHong(`p${TRAN_ANH_HONG}`));
  quenAnhHong();
});
