/* Journey map is a second view of the same itinerary, not a second itinerary.
 *
 * Run from apps/mobile:
 *     npx tsc -p tsconfig.test.json && node tools/fixup-esm.mjs && node --test tests/rudi-hanh-trinh.test.mjs
 *
 * What must stay true: a slot without coordinates never becomes a marker; the
 * numbered milestones follow timeline order; metres are integers; a route
 * efficiency figure is computed or absent, never invented; road geometry
 * comes only from the server's Valhalla preview, never a public router.
 */
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

import {
  chieuTuChang,
  chieuTuNgay,
  ganMappedTheoId,
  ganMappedVaoCho,
  ganTrangThai,
  idSlot,
} from "../dist-test/rudi/hanh-trinh/chieu.js";
import {
  chuKhoangCach,
  chuThoiGian,
  giayTuMet,
  haversineMet,
  hieuSuatTuyen,
  tomTatHanhTrinh,
} from "../dist-test/rudi/hanh-trinh/tom-tat.js";
import { geodesic, gioTru, khoaDoan } from "../dist-test/rudi/hanh-trinh/duong.js";
import { toiUuGanNhat } from "../dist-test/rudi/hanh-trinh/toi-uu.js";
import { TOA_DO_MAU } from "../dist-test/rudi/hanh-trinh/toa-do-mau.js";

const A = { lat: 11.94, lng: 108.43 };
const B = { lat: 11.98, lng: 108.45 };
const C = { lat: 11.941, lng: 108.431 };

const CHO = [
  { id: "cafe", name: "Cafe", lat: A.lat, lng: A.lng, address: "1 Nguyễn Thị Minh Khai" },
  { id: "bao-tang", name: "Bảo tàng", lat: B.lat, lng: B.lng, address: null },
  { id: "quan", name: "Quán", lat: C.lat, lng: C.lng, address: "2 Bạch Đằng" },
];

test("chặng không toạ độ vẫn có trong danh sách nhưng không thành marker, không nối polyline", () => {
  const ra = chieuTuNgay(
    {
      day: "Ngày 1",
      items: [
        { time: "07:00", title: "Khởi hành từ TP.HCM" },
        { time: "12:30", title: "Ăn trưa", placeId: "cafe" },
        { time: "15:00", title: "Picnic" },
        { time: "18:00", title: "Tối", placeId: "quan" },
      ],
    },
    CHO,
  );
  assert.equal(ra.activities.length, 4);
  assert.deepEqual(
    ra.activities.map((a) => [a.so, a.lat === null, a.tieuDe]),
    [
      [null, true, "Khởi hành từ TP.HCM"],
      [1, false, "Ăn trưa"],
      [null, true, "Picnic"],
      [2, false, "Tối"],
    ],
  );
  assert.equal(ra.routeSegments.length, 1);
  assert.equal(ra.routeSegments[0].fromActivityId, ra.activities[1].id);
  assert.equal(ra.routeSegments[0].toActivityId, ra.activities[3].id);
});

test("số milestone đi theo thứ tự lịch, không theo khoảng cách", () => {
  const ra = chieuTuNgay(
    {
      day: "Ngày 2",
      items: [
        { time: "09:00", title: "Xa", placeId: "bao-tang" },
        { time: "11:00", title: "Gần", placeId: "quan" },
        { time: "15:00", title: "Cafe", placeId: "cafe" },
      ],
    },
    CHO,
  );
  assert.deepEqual(
    ra.activities.map((a) => [a.so, a.placeId]),
    [
      [1, "bao-tang"],
      [2, "quan"],
      [3, "cafe"],
    ],
  );
});

test("chặng live lấy toạ độ từ catalogue; chặng không place_id không có marker", () => {
  const ra = chieuTuChang(
    [
      { id: "s-1", position: 0, at: "12:00", label: "Ăn trưa", place_name: null, place_id: null },
      { id: "s-2", position: 1, at: "18:00", label: "Ăn tối", place_name: "Cafe", place_id: "cafe" },
    ],
    CHO,
  );
  assert.equal(ra.activities[0].id, "s-1");
  assert.equal(ra.activities[0].so, null);
  assert.equal(ra.activities[1].so, 1);
  assert.equal(ra.activities[1].lat, A.lat);
  assert.equal(ra.routeSegments.length, 0);
});

test("haversine trả mét nguyên, cùng hai điểm là 0", () => {
  assert.equal(haversineMet(A, A), 0);
  const met = haversineMet(A, B);
  assert.equal(met, Math.round(met));
  assert.ok(met > 4000 && met < 6000, `Đà Lạt A→B phải vài km, nhận ${met}`);
});

test("hiệu suất tuyến vắng khi không đủ hai chặng có toạ độ, không bịa 82", () => {
  const mot = chieuTuNgay({ day: "x", items: [{ time: "09:00", title: "Cafe", placeId: "cafe" }] }, CHO);
  assert.equal(tomTatHanhTrinh(mot.activities, mot.routeSegments).hieuSuat, null);
  assert.equal(hieuSuatTuyen([]), null);
  assert.equal(hieuSuatTuyen([{ lat: A.lat, lng: A.lng }]), null);
});

test("hiệu suất là số nguyên (NN / hiện tại) × 100, không phải hằng", () => {
  const lech = [A, B, C];
  const tot = [A, C, B];
  const lechSo = hieuSuatTuyen(lech);
  const totSo = hieuSuatTuyen(tot);
  assert.equal(typeof lechSo, "number");
  assert.equal(lechSo, Math.round(lechSo));
  assert.ok(lechSo < 100, `lộ trình A→xa→gần phải < 100, nhận ${lechSo}`);
  assert.equal(totSo, 100);
  assert.notEqual(lechSo, 82);
});

test("tóm tắt đếm đúng chặng có vị trí và cộng mét các đoạn", () => {
  const ra = chieuTuNgay(
    {
      day: "x",
      items: [
        { time: "09:00", title: "Cafe", placeId: "cafe" },
        { time: "11:00", title: "Quán", placeId: "quan" },
      ],
    },
    CHO,
  );
  const tom = tomTatHanhTrinh(ra.activities, ra.routeSegments);
  assert.equal(tom.soChang, 2);
  assert.equal(tom.soChangCoViTri, 2);
  assert.equal(tom.met, haversineMet(A, C));
  assert.equal(tom.giay, giayTuMet(tom.met));
  assert.equal(tom.hieuSuat, 100);
});

test("nearest-neighbor giữ điểm đầu, xếp các marker còn lại theo gần nhất", () => {
  const ids = toiUuGanNhat([
    { id: "a", lat: A.lat, lng: A.lng },
    { id: "b", lat: B.lat, lng: B.lng },
    { id: "c", lat: C.lat, lng: C.lng },
  ]);
  assert.deepEqual(ids, ["a", "c", "b"]);
});

test("ganMappedVaoCho chỉ hoán vị các slot có toạ độ, giữ chỗ các slot không plot", () => {
  const items = [
    { time: "07:00", title: "Khởi hành" },
    { time: "12:30", title: "Cafe", placeId: "cafe" },
    { time: "14:00", title: "Nghỉ" },
    { time: "18:00", title: "Quán", placeId: "quan" },
    { time: "20:00", title: "Bảo tàng", placeId: "bao-tang" },
  ];
  const ra = ganMappedVaoCho(items, ["quan", "cafe", "bao-tang"]);
  assert.deepEqual(
    ra.map((s) => s.title),
    ["Khởi hành", "Quán", "Nghỉ", "Cafe", "Bảo tàng"],
  );
  // Giờ thuộc về CHỖ, không thuộc về chặng: xếp lại đường đi không được làm
  // ngày chạy ngược. Đo trên máy ảo 12/09: một cú bấm «Tối ưu lộ trình» in ra
  // chợ đêm 20:00 đứng TRƯỚC tiệc nướng 18:00.
  assert.deepEqual(
    ra.map((s) => s.time),
    ["07:00", "12:30", "14:00", "18:00", "20:00"],
  );
  assert.deepEqual(
    ra.map((s) => `${s.time} ${s.title}`),
    ["07:00 Khởi hành", "12:30 Quán", "14:00 Nghỉ", "18:00 Cafe", "20:00 Bảo tàng"],
  );
});

test("giờ trong ngày luôn tăng sau khi xếp lại, dù thứ tự chặng đổi thế nào", () => {
  const items = [
    { time: "09:00", title: "A", placeId: "a" },
    { time: "13:00", title: "B", placeId: "b" },
    { time: "19:00", title: "C", placeId: "c" },
  ];
  for (const thuTu of [["c", "b", "a"], ["b", "c", "a"], ["a", "c", "b"]]) {
    const gio = ganMappedVaoCho(items, thuTu).map((s) => s.time);
    assert.deepEqual(gio, ["09:00", "13:00", "19:00"], `thứ tự ${thuTu.join(",")}`);
  }
});

test("không còn router công khai nào trong app: đường thật chỉ đến từ Valhalla của máy chủ", () => {
  // ADR-0028: a stop's coordinates never reach a public routing service. The
  // OSRM client that used to live here had no caller left; this keeps it out.
  const nguon = readFileSync(new URL("../src/rudi/hanh-trinh/duong.ts", import.meta.url), "utf8");
  assert.doesNotMatch(nguon, /project-osrm|router\.|fetch\(/);
  assert.deepEqual(geodesic(A, B), [A, B]);
  assert.equal(khoaDoan(A, B), `${A.lat},${A.lng};${B.lat},${B.lng}`);
});

test("giờ rời = giờ tới trừ thời gian đi, bọc qua nửa đêm", () => {
  assert.equal(gioTru("15:00", 14 * 60), "14:46");
  assert.equal(gioTru("00:10", 20 * 60), "23:50");
});

test("tương lai: mọi chặng sắp tới; đang đi: đã tới rồi hiện tại rồi sắp", () => {
  const ra = chieuTuChang(
    [
      { id: "s-1", position: 0, at: "09:00", label: "Sáng", place_name: "Cafe", place_id: "cafe" },
      { id: "s-2", position: 1, at: "12:00", label: "Trưa", place_name: "Quán", place_id: "quan" },
      { id: "s-3", position: 2, at: "18:00", label: "Tối", place_name: "Bảo tàng", place_id: "bao-tang" },
    ],
    CHO,
  );
  const ids = ra.activities.map((a) => a.id);
  assert.deepEqual(ganTrangThai(ids, [], false), ["sap-toi", "sap-toi", "sap-toi"]);
  assert.deepEqual(ganTrangThai(ids, ["s-1"], true), ["xong", "hien-tai", "sap-toi"]);
});

test("chữ khoảng cách / thời gian đọc được, không hiện số giả", () => {
  assert.equal(chuKhoangCach(412), "412 m");
  assert.equal(chuKhoangCach(3100), "3,1 km");
  assert.equal(chuThoiGian(60), "1 phút");
  assert.equal(chuThoiGian(90 * 60), "1 giờ 30 phút");
  assert.equal(chuThoiGian(3600), "1 giờ");
});

test("id slot ổn định theo place hoặc giờ+tên, không theo vị trí mảng", () => {
  assert.equal(idSlot({ time: "09:00", title: "Cafe", placeId: "cafe" }, 0), "cafe");
  assert.equal(idSlot({ time: "07:00", title: "Khởi hành" }, 4), "gio:07:00:Khởi hành");
});

test("ganMappedTheoId hoán vị theo id, giữ chỗ các hàng không plot", () => {
  const ra = ganMappedTheoId(
    [
      { id: "u", label: "Khởi hành" },
      { id: "a", label: "Cafe" },
      { id: "b", label: "Quán" },
    ],
    ["b", "a"],
  );
  assert.deepEqual(
    ra.map((s) => s.id),
    ["u", "b", "a"],
  );
});

test("ganMappedTheoId giữ giờ của chỗ khi được nêu tên trường giờ", () => {
  const stops = [
    { id: "u", at: "08:00", label: "Khởi hành" },
    { id: "a", at: "12:00", label: "Cafe" },
    { id: "b", at: "19:00", label: "Quán" },
  ];
  const ra = ganMappedTheoId(stops, ["b", "a"], "at");
  assert.deepEqual(
    ra.map((s) => `${s.at} ${s.label}`),
    ["08:00 Khởi hành", "12:00 Quán", "19:00 Cafe"],
  );
  // Không nêu tên trường giờ thì hành vi cũ giữ nguyên: cả hàng đi theo.
  const cu = ganMappedTheoId(stops, ["b", "a"]);
  assert.deepEqual(
    cu.map((s) => `${s.at} ${s.label}`),
    ["08:00 Khởi hành", "19:00 Quán", "12:00 Cafe"],
  );
});

test("toạ độ mẫu Đà Lạt có cho mọi placeId trên lịch, không bịa Sài Gòn", () => {
  const can = ["banh-can-le", "ho-tuyen-lam-dem", "cho-dem", "doi-thien-phuc", "still-cafe", "lau-ga-la-e"];
  for (const id of can) {
    const t = TOA_DO_MAU[id];
    assert.ok(t, id);
    assert.ok(t.lat > 11.85 && t.lat < 12.05, `${id} lat ${t.lat}`);
    assert.ok(t.lng > 108.35 && t.lng < 108.55, `${id} lng ${t.lng}`);
  }
});

test("BanDo.native không import MapLibre ở top-level — APK cũ không được redbox cả tab Plan", () => {
  const src = readFileSync(new URL("../src/rudi/hanh-trinh/BanDo.native.tsx", import.meta.url), "utf8");
  assert.equal(src.includes("from \"@maplibre/maplibre-react-native\""), false);
  assert.equal(src.includes("NativeModules"), true);
  assert.equal(src.includes("BanDoThieu"), true);
});

test("chỗ ở tâm tỉnh giữ tên và địa chỉ trong lịch trình nhưng không thành marker, không nối đường", () => {
  const cho = [
    ...CHO,
    { id: "tam-tinh", name: "Bánh mì Tâm", lat: 10.7769, lng: 106.7009, geoPrecision: "province_centroid", address: "Chợ Bến Thành" },
    { id: "doan", name: "Quán đoán", lat: 11.95, lng: 108.44, geoPrecision: "suy_luan", address: null },
    { id: "pho", name: "Quán phố", lat: C.lat, lng: C.lng, geoPrecision: "street", address: "2 Bạch Đằng" },
  ];
  const ngay = chieuTuNgay(
    {
      day: "Ngày 1",
      items: [
        { time: "08:00", title: "Sáng", placeId: "tam-tinh" },
        { time: "10:00", title: "Giữa", placeId: "doan" },
        { time: "12:00", title: "Trưa", placeId: "pho" },
      ],
    },
    cho,
  );
  assert.deepEqual(
    ngay.activities.map((a) => [a.tenDiaDiem, a.diaChi, a.lat]),
    [["Bánh mì Tâm", "Chợ Bến Thành", null], ["Quán đoán", null, null], ["Quán phố", "2 Bạch Đằng", C.lat]],
  );
  assert.equal(ngay.routeSegments.length, 0, "một chặng vẽ được thì không có đoạn nào để nối");

  const chang = chieuTuChang(
    [
      { id: "s1", at: "08:00", label: "Sáng", place_id: "tam-tinh", place_name: "tên cũ", meeting_point: null },
      { id: "s2", at: "12:00", label: "Trưa", place_id: "pho", place_name: "tên cũ", meeting_point: null },
    ],
    cho,
  );
  assert.deepEqual(chang.activities.map((a) => [a.tenDiaDiem, a.lat]), [["Bánh mì Tâm", null], ["Quán phố", C.lat]]);
});

/* ---------------------------------------------------------------------------
 * M7 bản đồ: con tem, nét mực, nét chì, trang ngày.
 * ------------------------------------------------------------------------ */
import { catDuong, chuNeo, doDai, giuaDoan, hinhTem, kieuBanDo, lopDuong, mocChum, nhanDoan, nhanMoc, nhipVe, tapHop, veDenDau } from "../dist-test/rudi/hanh-trinh/kieu-ban-do.js";
import { chuNgay, trangThaiTuyen } from "../dist-test/rudi/hanh-trinh/ke-hoach.js";

const MAU = { giay: "#fff", vien: "#777", muc: "#ba3e20", mucTrenMuc: "#1f2230", mo: "#676e7b", chu: "#1f2230", netChi: "#4e5563", bong: "b1", bongCao: "b2", nen: "#fff" };
const TEM = { so: 2, chon: false, trangThai: null };

test("tem ghim nói trạng thái bằng hình và chữ: đã tới là bút chì + tick, điểm tiếp theo là coral nổi", () => {
  const sap = hinhTem(TEM, MAU);
  assert.deepEqual([sap.nen, sap.vien, sap.so, sap.dauTick, sap.bong], [MAU.giay, MAU.vien, MAU.muc, false, "b1"]);
  const xong = hinhTem({ ...TEM, trangThai: "xong" }, MAU);
  assert.deepEqual([xong.nen, xong.so, xong.dauTick], [MAU.giay, MAU.mo, true]);
  // Reached must not look like upcoming: different number ink AND a tick.
  assert.notEqual(xong.so, sap.so);
  const toi = hinhTem({ ...TEM, trangThai: "hien-tai" }, MAU);
  assert.deepEqual([toi.nen, toi.so, toi.bong], [MAU.muc, MAU.mucTrenMuc, "b2"]);
  const chon = hinhTem({ ...TEM, chon: true }, MAU);
  assert.equal(chon.nghieng, 0, "tem đang chọn đứng thẳng");
  assert.ok(chon.co > sap.co && sap.co >= 44, "tem ≥ 44dp, tem chọn to hơn");
  assert.notEqual(hinhTem({ ...TEM, so: 1 }, MAU).nghieng, hinhTem({ ...TEM, so: 2 }, MAU).nghieng, "nghiêng xen kẽ");
});

test("nhãn trình đọc màn hình nói đủ trạng thái mà mắt đọc từ tem", () => {
  const moc = { so: 2, gio: "12:30", tieuDe: "Cà phê Vườn", trangThai: null, neo: null };
  assert.equal(nhanMoc(moc), "Mốc 2, 12:30, Cà phê Vườn");
  assert.equal(nhanMoc({ ...moc, trangThai: "xong" }), "Mốc 2, 12:30, Cà phê Vườn, đã tới");
  assert.equal(nhanMoc({ ...moc, trangThai: "hien-tai" }), "Mốc 2, 12:30, Cà phê Vườn, điểm tiếp theo");
  assert.equal(nhanMoc({ ...moc, neo: "ve" }), "Mốc 2, 12:30, Cà phê Vườn, xuất phát · về");
  assert.deepEqual([chuNeo("xuat-phat"), chuNeo("ket-thuc"), chuNeo(null)], ["XUẤT PHÁT", "KẾT THÚC", null]);
});

test("phút chỉ in trên tuyến thật; nét nháp không bao giờ mang số", () => {
  assert.equal(nhanDoan({ nguon: "valhalla", durationSeconds: 534 }), "9 phút");
  assert.equal(nhanDoan({ nguon: "geodesic", durationSeconds: 534 }), null);
  const fc = tapHop([
    { id: "a", polyline: [A, B], chon: false, nhan: "9 phút" },
    { id: "b", polyline: [B, C], chon: false, uocLuong: true, nhan: "4 phút" },
  ]);
  const net = fc.features.filter((f) => f.geometry.type === "LineString");
  assert.deepEqual(net.map((f) => f.properties.nhan), ["9 phút", ""]);
  // Each real leg's minutes sit on one point halfway along it -- a label that
  // cannot lose its place to street names; the draft gets no point at all.
  const diem = fc.features.filter((f) => f.geometry.type === "Point");
  assert.deepEqual(diem.map((f) => f.properties.nhan), ["9 phút"]);
  const lop = lopDuong(MAU);
  assert.equal(lop.nhan.layout["symbol-placement"], "point");
  assert.equal(lop.nhan.layout["text-allow-overlap"], true);
  for (const k of ["vien", "duong", "nhap"]) assert.match(JSON.stringify(lop[k].filter), /LineString/, `${k} chỉ vẽ nét, không vẽ điểm nhãn`);
});

test("nét nháp đi ngược lại đúng đoạn trước chỉ vẽ một lần, không thành đường ray", () => {
  const fc = tapHop([
    { id: "a", polyline: [A, B], chon: false, uocLuong: true },
    { id: "b", polyline: [B, C], chon: false, uocLuong: true },
  ]);
  assert.deepEqual(fc.features.map((f) => f.properties.trung), [0, 1]);
  assert.match(JSON.stringify(lopDuong(MAU).nhap.filter), /trung/);
  // A real road that comes back is ink both ways; only the pencil folds.
  const that = tapHop([{ id: "a", polyline: [A, B], chon: false }, { id: "b", polyline: [B, C], chon: false }]);
  assert.deepEqual(that.features.filter((f) => f.geometry.type === "LineString").map((f) => f.properties.trung), [0, 0]);
});

test("con tem gộp giữ trạng thái gấp nhất: điểm tiếp theo vẫn coral, mốc neo giữ nhãn", () => {
  const m = (id, so, trangThai, neo = null) => ({ id, so, lat: A.lat, lng: A.lng, tieuDe: id, gio: "09:00", chon: false, trangThai, neo });
  const chum = mocChum([m("a", 1, "xong", "xuat-phat"), m("b", 2, "hien-tai")]);
  assert.equal(chum.trangThai, "hien-tai");
  assert.equal(chum.neo, "xuat-phat");
  assert.equal(hinhTem(chum, MAU).nen, MAU.muc);
  assert.equal(mocChum([m("a", 1, "xong"), m("b", 2, "xong")]).trangThai, "xong");
  assert.equal(mocChum([m("a", 1, "xong"), m("b", 2, null)]).trangThai, null);
  assert.equal(mocChum([{ ...m("a", 1, null), nhip: "dong" }, { ...m("b", 2, null), nhip: "cho" }]).nhip, "cho", "gộp chỉ đóng khi mực tới mốc cuối");
});

test("không chọn gì thì cả tuyến là một nét mực liền; chọn một đoạn thì đoạn khác mới mờ", () => {
  const legs = [{ id: "a", polyline: [A, B], chon: false }, { id: "b", polyline: [B, C], chon: false }];
  assert.deepEqual(tapHop(legs).features.map((f) => f.properties.mo), [0, 0]);
  assert.deepEqual(tapHop([{ ...legs[0], chon: true }, legs[1]]).features.map((f) => f.properties.mo), [0, 1]);
});

test("trang ngày đóng dấu đúng loại nét: đường thật, đang tính, chưa tính được, nét nháp", () => {
  const base = { fixture: false, dangTinh: false, coTuyen: false, preview: null };
  assert.equal(trangThaiTuyen({ ...base, coTuyen: true }), "that");
  assert.equal(trangThaiTuyen({ ...base, dangTinh: true }), "dangTinh");
  assert.equal(trangThaiTuyen({ ...base, preview: { status: "unavailable", issues: [] } }), "khongTinhDuoc");
  assert.equal(trangThaiTuyen({ ...base, preview: { status: "incomplete", issues: [{ code: "routing_busy", stop_id: null, message: "x" }] } }), "khongTinhDuoc");
  assert.equal(trangThaiTuyen({ ...base, preview: { status: "incomplete", issues: [{ code: "missing_location", stop_id: "s", message: "x" }] } }), "uocLuong");
  assert.equal(trangThaiTuyen({ ...base, fixture: true, coTuyen: true }), "uocLuong", "bản dùng thử không bao giờ xưng đường thật");
  assert.equal(chuNgay("2026-10-17"), "T7 17/10");
  assert.equal(chuNgay("2026-10-18"), "CN 18/10");
});

test("nền bản đồ: đường có ba bậc, không POI bên thứ ba, mọi màu lấy từ token", () => {
  for (const toi of [false, true]) {
    const kieu = JSON.parse(kieuBanDo(toi));
    const ids = kieu.layers.map((l) => l.id);
    for (const bac of ["lon", "chinh", "pho"]) {
      assert.ok(ids.includes(`canh-${bac}`) && ids.includes(`duong-${bac}`), bac);
      assert.ok(ids.indexOf(`canh-${bac}`) < ids.indexOf("duong-pho"), "viền vẽ trước mọi dải giấy");
    }
    assert.ok(!kieu.layers.some((l) => l["source-layer"] === "poi"), "không POI của bên thứ ba");
    assert.match(kieu.sources.openmaptiles.attribution, /OpenStreetMap/);
    const tokens = JSON.parse(readFileSync(new URL("../../../packages/shared/tokens.json", import.meta.url), "utf8"));
    const bang = new Set(Object.values(toi ? tokens.color.dark : tokens.color.light));
    const mau = JSON.stringify(kieu).match(/#[0-9a-fA-F]{6}\b/g) ?? [];
    assert.ok(mau.length > 0 && mau.every((m) => bang.has(m)), "chỉ màu token");
  }
});

test("bản đồ web có worker để tải tile: BanDo đặt đúng URL mà tool chép tới", () => {
  // maplibre-gl 6 derives its worker URL from import.meta.url, which Metro's
  // bundle does not carry: without setWorkerUrl no tile ever loads (2026-09-29).
  const banDo = readFileSync(new URL("../src/rudi/hanh-trinh/BanDo.tsx", import.meta.url), "utf8");
  const tool = readFileSync(new URL("../tools/chep-maplibre-worker.mjs", import.meta.url), "utf8");
  const pkg = JSON.parse(readFileSync(new URL("../package.json", import.meta.url), "utf8"));
  assert.match(banDo, /setWorkerUrl\(MAPLIBRE_WORKER_URL\)/);
  assert.match(banDo, /MAPLIBRE_WORKER_URL = "\/maplibre\/maplibre-gl-worker\.mjs"/);
  assert.match(tool, /join\(GOC, "public", "maplibre"\)/);
  assert.match(tool, /"maplibre-gl-worker\.mjs", "maplibre-gl-shared\.mjs"/);
  assert.match(pkg.scripts["build:check"], /chep-maplibre-worker/);
  assert.match(pkg.scripts.postinstall, /chep-maplibre-worker/);
  // No inline `position` on a pin: it pushed every later pin 48px off its point.
  assert.doesNotMatch(banDo.slice(banDo.indexOf("function veMoc"), banDo.indexOf("export function BanDo")), /"position:relative"/);
});

test("nét mực tự vẽ: cắt theo chiều dài thật, tem chỉ đóng dấu khi mực tới", () => {
  const P = { lat: 10.77, lng: 106.70 };
  const Q = { lat: 10.78, lng: 106.70 };
  const R2 = { lat: 10.80, lng: 106.70 };
  const nua = catDuong([P, Q, R2], 0.5);
  assert.ok(Math.abs(doDai(nua) - doDai([P, Q, R2]) / 2) < 1, "một nửa theo mét, không theo số đỉnh");
  assert.deepEqual(catDuong([P, Q], 0), []);
  assert.deepEqual(catDuong([P, Q], 1), [P, Q]);
  const ngay = [
    { id: "a", tu: "s1", den: "s2", polyline: [P, Q], chon: false, nhan: "3 phút" },
    { id: "b", tu: "s2", den: "s3", polyline: [Q, R2], chon: false, nhan: "5 phút" },
  ];
  // Leg a is a third of the day: at 20% only s1 has been reached.
  const dau = veDenDau(ngay, 0.2);
  assert.deepEqual([...dau.daCham].sort(), ["s1"]);
  assert.equal(dau.doan[0].nhan, null, "phút chỉ hiện khi đoạn đã vẽ xong");
  assert.equal(dau.doan[1].polyline.length, 0);
  const giua = veDenDau(ngay, 0.5);
  assert.deepEqual([...giua.daCham].sort(), ["s1", "s2"]);
  assert.equal(giua.doan[0].nhan, "3 phút");
  const xong = veDenDau(ngay, 1);
  assert.deepEqual([...xong.daCham].sort(), ["s1", "s2", "s3"]);
  assert.ok(!("tu" in xong.doan[0]), "trường nội bộ không lọt ra bản đồ");
});

test("nhãn đoạn nằm giữa đoạn theo chiều dài; nhịp vẽ tới nơi không vượt", () => {
  const P = { lat: 10.77, lng: 106.70 };
  const Q = { lat: 10.78, lng: 106.70 };
  const R2 = { lat: 10.80, lng: 106.70 };
  const g = giuaDoan([P, Q, R2]);
  assert.ok(Math.abs(g.lat - 10.785) < 1e-6, `giữa theo mét: ${g.lat}`);
  assert.equal(giuaDoan([]), null);
  assert.equal(nhipVe(0), 0);
  assert.equal(nhipVe(1), 1);
  assert.ok(nhipVe(0.1) < 0.1 && nhipVe(0.9) > 0.9, "đặt bút và nhấc bút chậm");
  assert.ok(Math.abs(nhipVe(0.5) - 0.5) < 1e-9, "giữa nét đi đều, không vội: nửa thời gian là nửa nét");
  for (let x = 0; x <= 1; x += 0.05) assert.ok(nhipVe(x) <= 1 && nhipVe(x) >= 0, "không vượt đích, không nảy");
  // The chosen leg shows its paper tag, not the small label.
  const nhan = lopDuong({ giay: "#fff", vien: "#777", muc: "#b00", mucTrenMuc: "#000", mo: "#666", chu: "#111", netChi: "#444", bong: "", bongCao: "", nen: "#fff" }).nhan;
  assert.ok(JSON.stringify(nhan.filter).includes('["!=",["get","chon"],1]'));
});

test("GeoJSON luôn hợp lệ trong lúc vẽ: đoạn chưa đủ hai điểm không được gửi đi", () => {
  // Native MapLibre drops the whole source on a LineString with < 2 positions.
  const P = { lat: 10.77, lng: 106.70 };
  const Q = { lat: 10.78, lng: 106.70 };
  const fc = tapHop([
    { id: "a", polyline: [P, Q], chon: false },
    { id: "b", polyline: [], chon: false },
    { id: "c", polyline: [Q], chon: false },
  ]);
  assert.deepEqual(fc.features.map((f) => f.properties.id), ["a"]);
  assert.ok(fc.features.every((f) => f.geometry.coordinates.length >= 2));
  const dau = veDenDau([{ id: "a", tu: "s1", den: "s2", polyline: [P, Q], chon: false }], 0);
  assert.equal(tapHop(dau.doan).features.length, 0, "khung đầu không có đường nào, không phải đường rỗng");
});

test("nét mực chờ bản đồ hiện rồi mới vẽ: không diễn trên bản đồ trắng", () => {
  const nguon = (f) => readFileSync(new URL(`../src/rudi/hanh-trinh/${f}`, import.meta.url), "utf8");
  assert.match(nguon("ManHinhHanhTrinh.tsx"), /useNetMuc\(khoaVe, banDoSan && !dangTimCho\)/);
  assert.match(nguon("BanDoMapLibre.native.tsx"), /onDidFinishLoadingMap=\{[^}]*onSan\?\.\(\)/);
  assert.match(nguon("BanDo.tsx"), /map\.on\("load"[\s\S]*?cbs\.current\.onSan\?\.\(\)/);
  const netMuc = nguon("net-muc.ts");
  assert.match(netMuc, /if \(!san\) return;\s*daVe\.add\(khoa\)/, "chưa sẵn thì chưa tính là đã vẽ");
});
