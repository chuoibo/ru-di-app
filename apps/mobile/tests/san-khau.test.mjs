/**
 * Máy sân khấu pop-up (ADR-0037 D1, D3; kế hoạch UI v3 S0.3).
 *
 * Bốn điều test này giữ, đều chạy trên hàm thật:
 *   1. Sân khấu là tranh cũ dựng đứng, không vẽ lại: trải phẳng các tầng của
 *      mười cảnh ra đúng từng lớp `hinhCanh`, và ba tầng của một ký hoạ là đúng
 *      bộ lớp `hinhKyHoa` (cùng số lần mỗi lớp), chỉ đổi thứ tự theo độ sâu.
 *   2. Mọi sân khấu app dựng đều hợp lệ: ngữ pháp đường Java nhận, trong khung,
 *      xa trước gần sau, nếp gập trong khung, có câu cho trình đọc màn hình, và
 *      đúng một nguồn sáng (lớp cam) khi tranh có một.
 *   3. Toán gập: nằm phẳng lúc 0, đứng thẳng lúc 1, tầng xa luôn dựng trước, và
 *      cú bật dựng dùng hết đúng ngân sách `batToiDa` chứ không hơn.
 *   4. `kiemSanKhau` bắt được từng kiểu sân khấu hỏng (mỗi đột biến một lỗi).
 *
 * Không chứng minh: người xem có đọc ra chiều sâu không, hay khung hình nào
 * giật trên máy thật. Cái đó là việc của ảnh chụp và của Lead trên native.
 *
 * Run from apps/mobile:
 *     npx tsc -p tsconfig.test.json && node tools/fixup-esm.mjs && node --test tests/san-khau.test.mjs
 */
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

import { CANH_IDS, CANH_KHONG_NEP, KHUNG_CANH, SAN_CANH, hinhCanh } from "../dist-test/rudi/art/canh.js";
import { KHUNG_KY_HOA, NET_KY_HOA, SAN_KY_HOA, hinhKyHoa } from "../dist-test/rudi/art/ky-hoa.js";
import { kiemSanKhau, lopPhang, nguonSangCua, sanKhauKyHoa, sanKhauTuCanh, tachDoSau } from "../dist-test/rudi/art/san-khau.js";
import {
  BIA_MO,
  NHIP_DAU,
  bongTheoGoc,
  choLatBia,
  giaiDoanDau,
  giamToc,
  gocBatTang,
  gocGapCuon,
  huongLat,
  lechThiSai,
} from "../dist-test/rudi/san-khau/dong-hoc.js";
import { MOTION_MS, NGAN_SACH_SAN_KHAU, batToiDa } from "../dist-test/rudi/motion.js";
import { kiemLop } from "./_kiem-lop.mjs";

// The same (category, tags) pairs the Explore fixture hands to a sketch, read
// as source because fixtures.ts pulls expo-image (as art-ky-hoa.test.mjs does).
const NGUON = readFileSync(new URL("../src/rudi/fixtures.ts", import.meta.url), "utf8");
const KHOI_LOAI = NGUON.slice(NGUON.indexOf("export const LOAI_MAU"), NGUON.indexOf("};", NGUON.indexOf("export const LOAI_MAU")));
const LOAI_MAU = Object.fromEntries([...KHOI_LOAI.matchAll(/^\s*"?([^"\n:]+?)"?: "([a-z-]+)",?$/gm)].map((m) => [m[1], m[2]]));
const PLACES = [...NGUON.matchAll(/name: "([^"]+)",[\s\S]*?tags: \[([^\]]*)\],\s*category: "([^"]+)"/g)].map((m) => ({
  name: m[1],
  tags: [...m[2].matchAll(/"([^"]+)"/g)].map((t) => t[1]),
  category: m[3],
}));
assert.equal(PLACES.length, 12, "fixture phải có đúng 12 nơi (đọc nguồn)");
const KY_HOA = [
  ...PLACES.map((p) => ({ ten: p.name, loai: LOAI_MAU[p.category], tags: p.tags })),
  ...["quan-an-local", "cafe", "vui-choi", "di-choi-dem", "khac", undefined].map((loai) => ({ ten: `chỉ loại ${loai}`, loai, tags: [] })),
];

/** Every stage the app can build from the art it has today. */
function moiSanKhau() {
  const ra = [];
  for (const id of CANH_IDS) {
    for (const nep of [true, false]) ra.push({ ten: `cảnh ${id}${nep ? "" : " (không Nếp)"}`, san: sanKhauTuCanh(id, { nep }), khung: KHUNG_CANH });
  }
  for (const k of KY_HOA) {
    for (const gon of [false, true]) ra.push({ ten: `ký hoạ ${k.ten}${gon ? " (gọn)" : ""}`, san: sanKhauKyHoa(k.loai, k.tags, { gon }), khung: KHUNG_KY_HOA });
  }
  return ra;
}

/** A multiset of layers, as a sorted list of their JSON. */
const boLop = (lop) => lop.map((l) => JSON.stringify(l)).sort();

test("sân khấu của cảnh trải phẳng ra đúng từng lớp của tranh cũ, cùng thứ tự", () => {
  for (const id of CANH_IDS) {
    for (const nep of [true, false]) {
      assert.deepEqual(lopPhang(sanKhauTuCanh(id, { nep })), hinhCanh(id, { nep }), `${id} nep=${nep}`);
    }
  }
  // an unknown id stands in for the first scene, as `hinhCanh` does
  assert.deepEqual(lopPhang(sanKhauTuCanh("khong-co-canh-nay")), hinhCanh("khong-co-canh-nay"));
});

test("cảnh không bao giờ có Nếp (lỗi, xung đột) không dựng tầng Nếp dù người gọi xin", () => {
  for (const id of CANH_IDS) {
    const san = sanKhauTuCanh(id);
    const coNep = san.tang.some((t) => t.id === "nep");
    assert.equal(coNep, !CANH_KHONG_NEP.has(id), id);
    assert.ok(sanKhauTuCanh(id, { nep: false }).tang.every((t) => t.id !== "nep"), `${id}: nep=false vẫn còn tầng Nếp`);
  }
});

test("ba tầng của ký hoạ là đúng bộ lớp của ký hoạ, không thêm không bớt", () => {
  for (const k of KY_HOA) {
    for (const gon of [false, true]) {
      const san = sanKhauKyHoa(k.loai, k.tags, { gon });
      assert.deepEqual(boLop(lopPhang(san)), boLop(hinhKyHoa(k.loai, k.tags, { gon })), `${k.ten} gon=${gon}`);
    }
  }
});

test("mỗi tầng ký hoạ chỉ mang nét đúng độ dày của nó: xa 1,7 · vừa 2,4 · gần 3,0", () => {
  const cuaTang = { xa: NET_KY_HOA.xa, vua: NET_KY_HOA.vua, gan: NET_KY_HOA.gan };
  for (const k of KY_HOA) {
    const san = sanKhauKyHoa(k.loai, k.tags);
    assert.ok(san.tang.length >= 2, `${k.ten}: chỉ ${san.tang.length} tầng, không đọc ra chiều sâu`);
    for (const t of san.tang) {
      for (const l of t.lop) {
        if (l.net && l.net > 0) assert.equal(l.net, cuaTang[t.id], `${k.ten}: nét ${l.net} lạc vào tầng ${t.id}`);
      }
    }
  }
});

test("mọi sân khấu app dựng: hợp lệ, trong khung, ngữ pháp Java nhận, có câu mô tả", () => {
  const tatCa = moiSanKhau();
  assert.ok(tatCa.length >= 20 + 36, `chỉ ${tatCa.length} sân khấu`);
  for (const { ten, san, khung } of tatCa) {
    assert.deepEqual(kiemSanKhau(san), [], ten);
    assert.deepEqual(san.khung, { w: khung.w, h: khung.h }, ten);
    for (const t of san.tang) kiemLop(`${ten} / ${t.id}`, t.lop, khung.w, khung.h);
    if (san.nen.length > 0) kiemLop(`${ten} / nền`, san.nen, khung.w, khung.h);
  }
});

test("nếp gập nằm trên sàn của tranh; nguồn sáng là lớp cam duy nhất và nằm trong khung", () => {
  for (const { ten, san } of moiSanKhau()) {
    const san0 = san.id.startsWith("canh:") ? SAN_CANH : SAN_KY_HOA;
    for (const t of san.tang) assert.equal(t.nep, san0, `${ten}: tầng ${t.id} gập ở ${t.nep}`);
    const cam = lopPhang(san).filter((l) => l.mau === "gap" && !(l.net > 0));
    if (cam.length === 0) {
      assert.equal(san.nguonSang, null, `${ten}: không có lớp cam mà vẫn có nguồn sáng`);
      continue;
    }
    assert.ok(san.nguonSang, `${ten}: có lớp cam mà không có nguồn sáng`);
    assert.deepEqual(san.nguonSang, nguonSangCua(cam), `${ten}: nguồn sáng không phải lớp cam đầu tiên`);
    assert.ok(san.nguonSang.x >= 0 && san.nguonSang.x <= san.khung.w && san.nguonSang.y >= 0 && san.nguonSang.y <= san.khung.h, ten);
  }
  // the sketches carry exactly one coral light (art-ky-hoa pins it); the stage keeps it
  for (const k of KY_HOA) assert.ok(sanKhauKyHoa(k.loai, k.tags).nguonSang, k.ten);
});

test("sân khấu là hàm tất định: dựng hai lần ra cùng một dữ liệu", () => {
  for (const id of CANH_IDS) assert.deepEqual(sanKhauTuCanh(id), sanKhauTuCanh(id));
  for (const k of KY_HOA) assert.deepEqual(sanKhauKyHoa(k.loai, k.tags), sanKhauKyHoa(k.loai, k.tags));
});

test("tachDoSau: lớp tô đi theo nét ngay sau nó; lớp tô cuối cùng ở lại với dải trước", () => {
  const fill = (d) => ({ d, mau: "bong" });
  const net = (d, w) => ({ d, mau: "muc", net: w });
  const lop = [fill("a"), net("b", 1.7), fill("c"), fill("d"), net("e", 3), net("f", 2.4), fill("g")];
  const [xa, vua, gan] = tachDoSau(lop, { xa: 1.7, vua: 2.4 });
  assert.deepEqual(xa.map((l) => l.d), ["a", "b"]);
  assert.deepEqual(gan.map((l) => l.d), ["c", "d", "e"]);
  assert.deepEqual(vua.map((l) => l.d), ["f", "g"]);
  // a drawing of fills only falls in the middle band
  assert.deepEqual(tachDoSau([fill("x")], { xa: 1.7, vua: 2.4 })[1].map((l) => l.d), ["x"]);
});

// --- fold maths ---------------------------------------------------------------

/** An independent solve of cubic-bezier(0, 0, 0.2, 1): dense sampling, no bisection. */
function giamTocMau(t) {
  let tot = { dx: Infinity, y: 0 };
  for (let k = 0; k <= 20000; k += 1) {
    const s = k / 20000;
    const u = 1 - s;
    const x = 3 * u * s * s * 0.2 + s * s * s;
    const y = 3 * u * s * s + s * s * s;
    const dx = Math.abs(x - t);
    if (dx < tot.dx) tot = { dx, y };
  }
  return tot.y;
}

test("giamToc là đường cong decelerate: 0 → 0, 1 → 1, đơn điệu, khớp một phép giải độc lập", () => {
  assert.equal(giamToc(0), 0);
  assert.ok(Math.abs(giamToc(1) - 1) < 1e-6);
  let truoc = -1;
  for (let k = 0; k <= 100; k += 1) {
    const t = k / 100;
    const y = giamToc(t);
    assert.ok(y >= truoc - 1e-9, `không đơn điệu ở ${t}`);
    assert.ok(Math.abs(y - giamTocMau(t)) < 2e-3, `lệch ở ${t}: ${y} với ${giamTocMau(t)}`);
    truoc = y;
  }
  // decelerate: most of the way is covered early
  assert.ok(giamToc(0.5) > 0.8, `giamToc(0.5) = ${giamToc(0.5)}`);
  assert.equal(giamToc(-3), 0);
  assert.ok(Math.abs(giamToc(7) - 1) < 1e-6);
});

test("bật dựng: phẳng lúc 0, đứng lúc 1, tầng xa luôn dựng trước, mỗi tầng chỉ dựng lên", () => {
  for (const n of [1, 2, 3, 4, 6]) {
    for (let i = 0; i < n; i += 1) {
      assert.equal(gocBatTang(0, i, n), 90, `n=${n} i=${i} lúc 0`);
      assert.ok(Math.abs(gocBatTang(1, i, n)) < 1e-6, `n=${n} i=${i} lúc 1: ${gocBatTang(1, i, n)}`);
    }
    for (let k = 0; k <= 200; k += 1) {
      const mo = k / 200;
      for (let i = 0; i < n; i += 1) {
        const g = gocBatTang(mo, i, n);
        assert.ok(g >= -1e-9 && g <= 90, `n=${n} i=${i} mo=${mo}: ${g}`);
        if (k > 0) assert.ok(g <= gocBatTang((k - 1) / 200, i, n) + 1e-9, `n=${n} i=${i}: nằm xuống lại ở ${mo}`);
        if (i > 0) assert.ok(gocBatTang(mo, i - 1, n) <= g + 1e-9, `n=${n}: tầng ${i} dựng trước tầng ${i - 1} ở ${mo}`);
      }
    }
  }
});

test("bật dựng dùng hết đúng ngân sách: tầng chậm nhất xong đúng lúc 1, không sớm hơn", () => {
  for (const n of [1, 2, 3, 4, 5, 9]) {
    const cuoi = Math.min(3, n - 1);
    const tong = batToiDa(n, false);
    // one frame (16 ms) before the end, the slowest layer is still rising
    const truoc = 1 - 16 / tong;
    assert.ok(gocBatTang(truoc, cuoi, n) > 0.01, `n=${n}: tầng chậm nhất đã đứng trước hạn`);
    // and the fifth layer on starts with the fourth, so nothing overruns the budget
    for (let i = 4; i < n; i += 1) {
      for (const mo of [0.1, 0.5, 0.9]) assert.equal(gocBatTang(mo, i, n), gocBatTang(mo, 3, n));
    }
  }
  // the first layer of a 4-layer stage finishes after exactly `shared` ms of the 420
  const tong4 = batToiDa(4, false);
  assert.equal(tong4, NGAN_SACH_SAN_KHAU.batToiDa);
  assert.ok(Math.abs(gocBatTang(MOTION_MS.shared / tong4, 0, 4)) < 1e-6);
  assert.ok(gocBatTang((MOTION_MS.shared - 16) / tong4, 0, 4) > 0.01);
});

test("gập theo cuộn, thị sai, bóng, hướng lật", () => {
  assert.equal(gocGapCuon(-40, 200), 0, "kéo quá đầu không gập ngược");
  assert.equal(gocGapCuon(0, 200), 0);
  assert.equal(gocGapCuon(100, 200), 40);
  assert.equal(gocGapCuon(900, 200), 80);
  assert.equal(gocGapCuon(50, 0), 0, "sân khấu cao 0 không chia cho 0");
  assert.equal(gocGapCuon(100, 200, 60), 30);

  assert.equal(lechThiSai(1, 3), 6, "tầng gần nhất đi trọn biên độ");
  assert.equal(lechThiSai(1, 0), 1.5, "tầng xa nhất đi một phần tư");
  assert.equal(lechThiSai(-1, 3), -6);
  assert.equal(lechThiSai(5, 3), 6, "đầu vào bị kẹp");
  assert.equal(lechThiSai(0.5, 1, 8), 2);
  for (const v of [-1, -0.3, 0.4, 1]) {
    for (let s = 1; s <= 3; s += 1) assert.ok(Math.abs(lechThiSai(v, s)) > Math.abs(lechThiSai(v, s - 1)), `v=${v} sau=${s}`);
  }

  assert.equal(bongTheoGoc(0), 1);
  assert.ok(bongTheoGoc(90) < 1e-9);
  assert.ok(bongTheoGoc(30) > bongTheoGoc(60));
  assert.equal(bongTheoGoc(-20), 1, "góc âm bị kẹp");

  assert.equal(huongLat(1, 2), 1);
  assert.equal(huongLat(3, 1), -1);
  assert.equal(huongLat(2, 2), 1);
});

test("ba nhịp của dấu: lao (tăng tốc), chạm mực, xong đúng lúc lao + chạm", () => {
  assert.deepEqual(giaiDoanDau(0), { roi: 0, muc: 0, xong: false });
  assert.equal(giaiDoanDau(NHIP_DAU.lao / 2).roi, 0.25, "rơi tăng tốc: nửa thời gian mới được một phần tư đường");
  assert.deepEqual(giaiDoanDau(NHIP_DAU.lao), { roi: 1, muc: 0, xong: false });
  assert.equal(giaiDoanDau(NHIP_DAU.lao + NHIP_DAU.cham / 2).muc, 0.5);
  assert.deepEqual(giaiDoanDau(NHIP_DAU.lao + NHIP_DAU.cham), { roi: 1, muc: 1, xong: true });
  assert.ok(NHIP_DAU.lao + NHIP_DAU.cham <= MOTION_MS.shared, "một cú dập nằm trong shared");
});

/*
 * The notebook cover of M6 (plan S2). The room is checked by projecting the
 * cover's four corners independently, every quarter degree, the way CSS and
 * React Native draw `perspective` then `rotateY` about the spine's centre: a
 * point `z` nearer is drawn `p / (p - z)` larger about the transform origin.
 */
function gocBia(rong, cao, goc, p) {
  const r = (goc * Math.PI) / 180;
  return [0, rong].flatMap((x) =>
    [-cao / 2, cao / 2].map((y) => {
      const z = x * Math.sin(r);
      const k = p / (p - z);
      return { x: x * Math.cos(r) * k, y: y * k };
    }),
  );
}

function raNgoai(rong, cao, cho) {
  const ngoai = [];
  for (let g = 0; g <= BIA_MO.gocToiDa; g += 0.25) {
    for (const d of gocBia(rong, cao, g, BIA_MO.phoiCanh)) {
      if (d.x < -cho.trai - 1e-9 || Math.abs(d.y) > cao / 2 + cho.doc + 1e-9) ngoai.push({ g, ...d });
    }
  }
  return ngoai;
}

test("bìa sổ mở: qua thẳng đứng, chưa nằm phẳng; giữ đủ chỗ cho bìa ở mọi góc", () => {
  assert.ok(BIA_MO.gocToiDa > 90 && BIA_MO.gocToiDa < 180, "bìa phải mở quá thẳng đứng (thấy mặt trong) mà chưa nằm phẳng");
  for (const rong of [96, 112, 128, 140, 180]) {
    const cao = Math.round(rong * 1.32);
    const cho = choLatBia(rong, cao);
    assert.deepEqual(raNgoai(rong, cao, cho), [], `rong=${rong}: bìa vẽ ra ngoài chỗ giữ`);
    // Canary: the room is not vacuous; without it the swing does leave the book.
    assert.ok(raNgoai(rong, cao, { trai: 0, doc: 0 }).length > 0, `rong=${rong}: canary không đỏ`);
    // And not wasteful: no more than a dp over what the corners reach.
    const xa = Math.max(...Array.from({ length: BIA_MO.gocToiDa * 4 + 1 }, (_, i) => Math.max(...gocBia(rong, cao, i / 4, BIA_MO.phoiCanh).map((d) => -d.x))));
    assert.ok(cho.trai - xa <= 1, `rong=${rong}: giữ ${cho.trai}, bìa chỉ tới ${xa.toFixed(1)}`);
  }
  assert.deepEqual(choLatBia(140, 185, 0), { trai: 0, doc: 0 }, "bìa đóng không cần chỗ");
});

// --- the validator catches each broken stage --------------------------------

test("kiemSanKhau: sân khấu thật sạch; mỗi đột biến bị bắt đúng một lỗi", () => {
  const goc = sanKhauKyHoa("quan-an-local", ["Món local", "Nhóm đông"]);
  assert.deepEqual(kiemSanKhau(goc), [], "canary danh tính: sân khấu thật phải sạch");
  const dotBien = [
    ["không tầng", (s) => ({ ...s, tang: [] }), /không có tầng/],
    ["trùng id", (s) => ({ ...s, tang: [s.tang[0], { ...s.tang[1], id: s.tang[0].id }, ...s.tang.slice(2)] }), /trùng id/],
    ["gần đứng trước xa", (s) => ({ ...s, tang: [...s.tang].reverse() }), /gần hơn mà đứng trước/],
    ["nếp gập ra ngoài", (s) => ({ ...s, tang: s.tang.map((t, i) => (i === 0 ? { ...t, nep: s.khung.h + 4 } : t)) }), /ra ngoài khung/],
    ["tầng rỗng", (s) => ({ ...s, tang: s.tang.map((t, i) => (i === 1 ? { ...t, lop: [] } : t)) }), /rỗng/],
    ["đứng mà không cao", (s) => ({ ...s, tang: s.tang.map((t, i) => (i === 0 ? { ...t, cao: 0 } : t)) }), /độ cao giấy/],
    ["không câu mô tả", (s) => ({ ...s, moTa: " " }), /mô tả/],
  ];
  for (const [ten, doi, mong] of dotBien) {
    const loi = kiemSanKhau(doi(goc));
    assert.ok(loi.length >= 1 && loi.some((l) => mong.test(l)), `${ten}: ${JSON.stringify(loi)}`);
  }
});

// --- how big a stage stands at the head of a screen ---------------------------

import { kichThuocSanKhau } from "../dist-test/rudi/san-khau/kich-thuoc.js";
import { layoutFor } from "../dist-test/rudi/adaptive.js";

const kt = (rong, cao, tiLe, fontScale = 1, rongCho = rong - 32) => {
  const l = layoutFor(rong, cao);
  return kichThuocSanKhau({ rongCho, caoCuaSo: cao, tiLe, sizeClass: l.sizeClass, heightClass: l.heightClass, fontScale });
};
const TI_LE_CANH = KHUNG_CANH.w / KHUNG_CANH.h;
const TI_LE_KY_HOA = KHUNG_KY_HOA.w / KHUNG_KY_HOA.h;

test("kích thước sân khấu: giữ tỉ lệ khung, nằm trong cột, cao theo cỡ cửa sổ", () => {
  // phones: the scene is height-bound (160-220), the wide sketch is width-bound
  for (const [rong, cao] of [[360, 640], [390, 844], [412, 915], [430, 932]]) {
    const canh = kt(rong, cao, TI_LE_CANH);
    assert.ok(canh.h >= 159 && canh.h <= 220, `${rong}×${cao}: cảnh cao ${canh.h}`);
    assert.ok(canh.w <= rong - 32, `${rong}×${cao}: cảnh tràn cột`);
    assert.ok(Math.abs(canh.w / canh.h - TI_LE_CANH) < 0.02, `${rong}×${cao}: cảnh méo ${canh.w}×${canh.h}`);
    const kyHoa = kt(rong, cao, TI_LE_KY_HOA);
    assert.equal(kyHoa.w, Math.floor(rong - 32), `${rong}×${cao}: ký hoạ phải lấy đủ cột`);
    assert.ok(Math.abs(kyHoa.w / kyHoa.h - TI_LE_KY_HOA) < 0.05);
  }
  // a taller window gives a taller stage, capped
  assert.ok(kt(390, 844, TI_LE_CANH).h > kt(360, 640, TI_LE_CANH).h);
  assert.equal(kt(430, 1400, TI_LE_CANH).h, 220);
  // tablets stand it taller
  const tab = kt(768, 1024, TI_LE_CANH, 1, 720);
  assert.ok(tab.h >= 220 && tab.h <= 260, `tablet: ${tab.h}`);
  const rong = kt(1280, 800, TI_LE_CANH, 1, 900);
  assert.ok(rong.h >= 240 && rong.h <= 300, `expanded: ${rong.h}`);
});

test("kích thước sân khấu: điện thoại nằm ngang thì không có sân khấu; chữ lớn thì nhỏ lại và đọc gọn", () => {
  assert.equal(kt(844, 390, TI_LE_CANH), null, "cửa sổ thấp: chỗ là của nội dung");
  assert.equal(kt(390, 844, TI_LE_CANH, 1, 0), null, "cột chưa đo không dựng sân khấu 0dp");
  const thuong = kt(390, 844, TI_LE_CANH);
  const lon = kt(390, 844, TI_LE_CANH, 1.3);
  assert.equal(thuong.gon, false);
  assert.equal(lon.gon, true, "chữ lớn đọc bản gọn");
  assert.ok(lon.h <= Math.ceil(thuong.h * 0.75) + 1, `chữ lớn: ${lon.h} so với ${thuong.h}`);
  // Android hands 1.3 as a float a hair below it; it still counts as large text
  assert.equal(kt(390, 844, TI_LE_CANH, Math.fround(1.3)).gon, true);
});

test("chỗ của Nếp khi diễn: 128 tablet, 112 phone, 88 chữ lớn, 0 khi cửa sổ thấp", async () => {
  const { kichThuocNepDien } = await import("../dist-test/rudi/san-khau/kich-thuoc.js");
  const co = (rong, cao, fontScale = 1) => {
    const l = layoutFor(rong, cao);
    return kichThuocNepDien({ sizeClass: l.sizeClass, heightClass: l.heightClass, fontScale });
  };
  assert.equal(co(390, 844), 112);
  assert.equal(co(768, 1024), 128);
  assert.equal(co(1280, 800), 128);
  assert.equal(co(390, 844, 1.3), 88, "chữ lớn: chữ trước, Nếp nhỏ lại");
  assert.equal(co(390, 844, Math.fround(1.3)), 88);
  assert.equal(co(844, 390), 0, "điện thoại nằm ngang: không có chỗ cho Nếp");
});
