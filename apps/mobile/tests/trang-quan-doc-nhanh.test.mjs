/* Finish review 21/10/2026: the place page's ten-second read and the truth of
 * the drawn stand-ins. Real compiled functions, fed the cases that went wrong:
 * a price printed twice, «yên tĩnh» beside «Yên tĩnh», Hội An's lanterns on a
 * Hà Nội eatery and Đà Lạt's lake on every city. */
import assert from "node:assert/strict";
import test from "node:test";

import { cauGio, dongChinh, gioHomNay, khoaNhan, nhanKhiChat, nhanKhongTrung } from "../dist-test/rudi/kham-pha/chi-tiet-gon.js";
import { VUNG_CUA_CANH, canhChoDiaDiem, canhDiemDen, canhDuocDung, phanCanhDanhSach, vungCua } from "../dist-test/rudi/kham-pha/canh-ve.js";
import { chiTietNgan } from "../dist-test/rudi/kham-pha/dia-diem.js";

const HL = {
  taiSaoDangDen: "Vườn cà phê trên sườn đồi.", khongKhi: null, khungGioDep: "6:30–8:30 sáng", mucGia: null, gioGhiChu: null,
  phuHopVoi: [], khauViPhuHop: [], luuY: [], traiNghiem: [], monChinh: [], monPhaiThu: [],
  gia: { minVnd: 35000, maxVnd: 90000, uoc: true },
  gioMoCua: [
    { thu: "mon", nhan: "Thứ Hai", mo: "06:00", dong: "21:30", quaDem: false },
    { thu: "sat", nhan: "Thứ Bảy", mo: "05:30", dong: "22:30", quaDem: false },
    { thu: "sun", nhan: "Chủ nhật", mo: "18:00", dong: "02:00", quaDem: true },
  ],
  canDatTruoc: { ma: "nen", nhan: "Nên đặt trước" },
  trongNgoai: { ma: "ca-hai", nhan: "Cả trong nhà và ngoài trời" },
  thoiLuong: { min: 60, max: 120 },
  danhMuc: [{ id: "cafe", nhan: "Cà phê & đồ uống" }],
  khiChat: [{ id: "yen-tinh", nhan: "Yên tĩnh" }, { id: "view", nhan: "View đẹp" }],
};
const CHO = { priceMinVnd: 40000, priceMaxVnd: 80000, traits: ["yên tĩnh", "view đẹp", "lãng mạn", "  Yen Tinh "] };
// 2026-10-19 is a Monday, 2026-10-25 a Sunday (local time).
const THU_HAI = new Date(2026, 9, 19, 10, 0);
const CHU_NHAT = new Date(2026, 9, 25, 10, 0);

test("giá xuất hiện đúng MỘT lần trong khối số liệu chính, ưu tiên khoảng của chỉ mục («Ước khoảng»)", () => {
  const dong = dongChinh(CHO, HL, THU_HAI);
  const gia = dong.filter((d) => d.id === "gia");
  assert.equal(gia.length, 1, "một dòng giá");
  assert.equal(gia[0].gia, "Ước khoảng 35–90k/người");
  assert.equal(dong.filter((d) => /\d+(k|đ)/.test(d.gia) && d.id !== "gia").length, 0, "không dòng nào khác mang số tiền");
  // Without the index's band the row's own price is the one line.
  const khongChiMuc = dongChinh(CHO, null, THU_HAI);
  assert.deepEqual(khongChiMuc.map((d) => [d.id, d.gia]), [["gia", "40.000đ – 80.000đ mỗi người"]]);
  assert.deepEqual(dongChinh({ priceMinVnd: null, priceMaxVnd: null }, null, THU_HAI), [], "không có gì thì không bịa");
});

test("giờ hôm nay theo thứ trong tuần; giờ đẹp nhất, thời lượng, đặt trước là các dòng riêng", () => {
  assert.deepEqual(dongChinh(CHO, HL, THU_HAI).map((d) => d.id), ["gia", "hom-nay", "gio-dep", "thoi-luong", "dat-truoc"]);
  assert.equal(dongChinh(CHO, HL, THU_HAI).find((d) => d.id === "hom-nay").gia, "06:00 – 21:30");
  assert.equal(cauGio(gioHomNay(HL.gioMoCua, CHU_NHAT)), "18:00 – 02:00 (qua đêm)");
  // A week with no line for today says nothing about today rather than «đóng cửa».
  assert.equal(gioHomNay(HL.gioMoCua, new Date(2026, 9, 21)), null);
  assert.equal(dongChinh(CHO, HL, new Date(2026, 9, 21)).some((d) => d.id === "hom-nay"), false);
  assert.equal(dongChinh(CHO, HL, THU_HAI).find((d) => d.id === "thoi-luong").gia, "Khoảng 60–120 phút");
});

test("chip chỉ là khí chất và mỗi nhãn một lần, bất kể hoa thường, dấu hay khoảng trắng", () => {
  assert.equal(khoaNhan("Yên tĩnh"), khoaNhan("  yen   TINH "));
  const nhan = nhanKhiChat(CHO, HL);
  assert.deepEqual(nhan, ["Cà phê & đồ uống", "Yên tĩnh", "View đẹp", "Cả trong nhà và ngoài trời", "lãng mạn"]);
  for (const khong of ["Nên đặt trước", "Khoảng 60–120 phút"]) assert.equal(nhan.includes(khong), false, `«${khong}» thuộc khối số liệu chính`);
  assert.equal(nhanKhongTrung(["a", "b", "c"], 2).length, 2);
  assert.deepEqual(nhanKhongTrung(["", "  ", "Đẹp", "dep"]), ["Đẹp"]);
});

test("điểm đánh giá luôn một chữ số thập phân: «4.0» cạnh «4.8»", () => {
  const base = { ratingCount: 12, distanceKm: null, priceMinVnd: null, priceMaxVnd: null, openNow: null, openHours: null, address: null };
  assert.equal(chiTietNgan({ ...base, rating: 4 })[0].chu, "4.0 (12)");
  assert.equal(chiTietNgan({ ...base, rating: 4.8 })[0].chu, "4.8 (12)");
});

const LANDMARK = Object.entries(VUNG_CUA_CANH).filter(([, v]) => v !== null && v !== "bien").map(([k]) => k);
const MOI_LOAI = ["cafe", "quan-an-local", "di-choi-dem", "vui-choi", "thien-nhien", undefined];

test("tranh vẽ thay ảnh không bao giờ là cảnh mốc của vùng khác", () => {
  assert.deepEqual(LANDMARK.sort(), ["scene-cafe", "scene-hills", "scene-lanterns", "scene-street"]);
  const vungKhac = [
    vungCua({ diemDen: "d-tinh-1" }), // Hà Nội
    vungCua({ diemDen: "d-tinh-46" }), // Huế
    vungCua({ diemDen: "d-tinh-79" }), // Hồ Chí Minh
    vungCua({ diaChi: "12 Hàng Bạc, Hoàn Kiếm, Hà Nội" }),
    vungCua({}),
  ];
  for (const vung of vungKhac) {
    for (const loai of MOI_LOAI) {
      for (let i = 0; i < 40; i += 1) {
        const canh = canhChoDiaDiem({ id: `p-${i}`, loai, vung });
        assert.ok(canh === null || !LANDMARK.includes(canh), `${loai} ngoài vùng vẽ ${canh}`);
        assert.notEqual(canh, "scene-coast", "không vẽ biển cho tỉnh không có biển");
      }
    }
  }
  // Inside the region the category's own scene is allowed again.
  const lamDong = vungCua({ diemDen: "d-tinh-68" });
  assert.deepEqual(canhDuocDung("cafe", lamDong), ["scene-cafe", "scene-atelier"]);
  assert.ok(vungCua({ diaChi: "Hẻm 24 Trại Mát, Phường 11, Đà Lạt, Lâm Đồng" }).has("lam-dong"));
  assert.deepEqual(canhDuocDung("quan-an-local", vungCua({ diemDen: "d-hoi-an" })), ["scene-street", "scene-atelier"]);
});

test("hai thẻ liền nhau không cùng một tranh khi còn tranh khác; thẻ có ảnh không tính là hàng xóm", () => {
  const lamDong = vungCua({ diemDen: "d-tinh-68" });
  const ds = ["a", "b", "c", "d", "e", "f"].map((id) => ({ id, loai: "cafe", coAnh: false }));
  const canh = phanCanhDanhSach(ds, lamDong);
  for (let i = 1; i < ds.length; i += 1) assert.notEqual(canh.get(ds[i].id), canh.get(ds[i - 1].id), `${ds[i - 1].id}/${ds[i].id}`);
  // The same place keeps its scene across reads.
  assert.equal(canhChoDiaDiem({ id: "a", loai: "cafe", vung: lamDong }), canhChoDiaDiem({ id: "a", loai: "cafe", vung: lamDong }));
  // A single honest scene alternates with the category's object instead of repeating back to back (review 02c94551).
  const haNoi = phanCanhDanhSach([{ id: "x", loai: "cafe", coAnh: false }, { id: "y", loai: "cafe", coAnh: false }, { id: "w", loai: "cafe", coAnh: false }], vungCua({ diemDen: "d-tinh-1" }));
  assert.deepEqual([...haNoi.values()], ["scene-atelier", null, "scene-atelier"]);
  assert.equal(phanCanhDanhSach([{ id: "z", loai: "di-choi-dem", coAnh: false }], vungCua({ diemDen: "d-tinh-1" })).get("z"), null, "không còn tranh đúng thì vẽ đồ vật của loại");
});

test("cảnh riêng của điểm đến chỉ khi vẽ đúng vùng đó", () => {
  assert.equal(canhDiemDen("d-tinh-68"), "scene-hills");
  assert.equal(canhDiemDen("d-da-lat"), "scene-hills");
  assert.equal(canhDiemDen("d-hoi-an"), "scene-lanterns");
  assert.equal(canhDiemDen("d-tinh-56"), "scene-coast");
  for (const id of ["d-tinh-1", "d-ha-noi", "d-tinh-46", "d-tinh-79", "d-tinh-15", "synthetic-city-1"]) assert.equal(canhDiemDen(id), null, id);
});

test("thẻ trang đã giữ không ảnh bìa chỉ có chữ; ô vẽ đồ vật nằm trên giấy sẫm, không trắng trơn", async () => {
  const { readFile } = await import("node:fs/promises");
  const diary = await readFile(new URL("../src/rudi/screens/profile/DiaryLead.tsx", import.meta.url), "utf8");
  const saved = diary.slice(diary.indexOf("function SavedDiary"), diary.indexOf("const styles"));
  assert.doesNotMatch(saved, /<VeLop/, "icon thay ảnh bìa từng đè lên tiêu đề (review 1fd5d576)");
  const hang = await readFile(new URL("../src/rudi/screens/explore/HangDiaDiem.tsx", import.meta.url), "utf8");
  assert.match(hang, /scene === null \? colors\.paperShade : colors\.paper/);
});

test("danh sách không bao giờ có hai thẻ liền nhau cùng một tranh, kể cả khi loại quán chỉ có một cảnh trung thực", () => {
  const lamDongVung = vungCua({ diemDen: "d-tinh-68" });
  for (const loai of ["food", "cafe", "nightlife", "activity", "stay", "khac", ""]) {
    const ds = Array.from({ length: 8 }, (_, i) => ({ id: `q-${loai}-${i}`, loai, coAnh: false }));
    const ra = ds.map((d) => phanCanhDanhSach(ds, lamDongVung).get(d.id));
    for (let i = 1; i < ra.length; i += 1) {
      if (ra[i] !== null) assert.notEqual(ra[i], ra[i - 1], `${loai}: thẻ ${i - 1} và ${i} cùng tranh ${ra[i]}`);
    }
  }
});
