/* Journey map is a second view of the same itinerary, not a second itinerary.
 *
 * Run from apps/mobile:
 *     npx tsc -p tsconfig.test.json && node tools/fixup-esm.mjs && node --test tests/rudi-hanh-trinh.test.mjs
 *
 * What must stay true: a slot without coordinates never becomes a marker; the
 * numbered milestones follow timeline order; metres are integers; a route
 * efficiency figure is computed or absent, never invented; OSRM failure falls
 * back to a geodesic rather than hanging the view.
 */
import assert from "node:assert/strict";
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
import { geodesic, gioTru, khoaDoan, layDoanDuong } from "../dist-test/rudi/hanh-trinh/duong.js";
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
});

test("OSRM thành công lấy polyline geojson; lỗi thì geodesic hai điểm", async () => {
  const ok = await layDoanDuong(A, C, {
    fetch: async () =>
      new Response(
        JSON.stringify({
          code: "Ok",
          routes: [
            {
              distance: 412.4,
              duration: 88.2,
              geometry: { coordinates: [[A.lng, A.lat], [108.4305, 11.9405], [C.lng, C.lat]] },
            },
          ],
        }),
        { status: 200 },
      ),
  });
  assert.equal(ok.nguon, "osrm");
  assert.equal(ok.distanceMeters, 412);
  assert.equal(ok.durationSeconds, 88);
  assert.equal(ok.polyline.length, 3);

  const hong = await layDoanDuong(A, C, {
    fetch: async () => new Response("no", { status: 500 }),
  });
  assert.equal(hong.nguon, "geodesic");
  assert.deepEqual(hong.polyline, geodesic(A, C));
  assert.equal(hong.distanceMeters, haversineMet(A, C));
});

test("OSRM quá hạn thì geodesic, không ném", async () => {
  const ra = await layDoanDuong(A, B, {
    timeoutMs: 20,
    fetch: () => new Promise(() => {}),
  });
  assert.equal(ra.nguon, "geodesic");
  assert.equal(ra.distanceMeters, haversineMet(A, B));
});

test("cache theo cặp toạ độ, lần sau không gọi fetch", async () => {
  let goi = 0;
  const cache = new Map();
  const fetch = async () => {
    goi += 1;
    return new Response("no", { status: 500 });
  };
  await layDoanDuong(A, B, { fetch, cache });
  await layDoanDuong(A, B, { fetch, cache });
  assert.equal(goi, 1);
  assert.ok(cache.has(khoaDoan(A, B)));
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

test("toạ độ mẫu Đà Lạt có cho mọi placeId trên lịch, không bịa Sài Gòn", () => {
  const can = ["banh-can-le", "ho-tuyen-lam-dem", "cho-dem", "doi-thien-phuc", "still-cafe", "lau-ga-la-e"];
  for (const id of can) {
    const t = TOA_DO_MAU[id];
    assert.ok(t, id);
    assert.ok(t.lat > 11.85 && t.lat < 12.05, `${id} lat ${t.lat}`);
    assert.ok(t.lng > 108.35 && t.lng < 108.55, `${id} lng ${t.lng}`);
  }
});
