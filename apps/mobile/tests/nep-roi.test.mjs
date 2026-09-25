/**
 * Nếp con rối giấy và chín tiết mục (ADR-0037 D5; kế hoạch UI v3 S0.4).
 *
 * Những điều test này giữ, chạy trên hàm thật, qua MỌI khung (mỗi 10ms) của
 * MỌI tiết mục, ở cả hai cách đọc (96dp chi tiết và 48dp rút gọn):
 *   1. Con rối là giấy của Nếp tĩnh, không vẽ lại: lúc nghỉ, tờ thân và khuôn
 *      mặt trùng từng chuỗi với hinhNep của đúng pose đó (độ nghiêng, ánh mắt,
 *      biểu cảm), và tay chân ghim đúng bốn khớp mà Nếp tĩnh ghim.
 *   2. Giấy không co giãn: mọi khúc tay chân giữ đúng chiều dài ở mọi khung;
 *      mục tiêu với tới được thì bàn tay/cổ chân tới đúng chỗ.
 *   3. Ma trận cho Skia đặt từng bộ phận đúng chỗ tuTheRoi vẽ (sai số ≤ 0,02).
 *   4. Không nét mực nào sơn lên nếp gấp cam ở bất kỳ khung nào, và mỗi khung
 *      có đúng một lớp cam (vật cầm không bao giờ là dấu cam thứ hai).
 *   5. Ở khoảnh khắc tiền, mặt chỉ là binh-than hoặc nhuong; mọi tiết mục
 *      trong ngân sách 1 400ms, khung cuối đứng yên trong hộp 96.
 *
 * Không chứng minh: người xem có đọc ra «đóng dấu» hay «cảm ơn» không. Cái đó
 * là việc của ảnh chụp, đọc mù và máy thật.
 *
 * Run from apps/mobile:
 *     npx tsc -p tsconfig.test.json && node tools/fixup-esm.mjs && node --test tests/nep-roi.test.mjs
 */
import assert from "node:assert/strict";
import test from "node:test";

import { BIEU_CAM, HONG_GAN, HONG_XA, POSE_NEP, VAI_GAN, VAI_XA, hinhNep, matNep, nguCanhNep, thanNep, tuTheCuaPose } from "../dist-test/rudi/art/nep.js";
import { apMaTran, bienDoiDuong } from "../dist-test/rudi/art/net.js";
import {
  CO_CHAN_GAN,
  CO_CHAN_XA,
  TU_THE_NGHI,
  XUONG,
  boPhanRoi,
  giaiRoi,
  ik2,
  maTranBoPhan,
  maTranThan,
  tuThe,
  tuTheRoi,
  tuTheTuPose,
} from "../dist-test/rudi/art/nep-roi.js";
import { MAT_KHI_TIEN, TIET_MUC, TIET_MUC_IDS, emBezier, khopTai, vuaNganSach } from "../dist-test/rudi/art/nep-dien.js";
import { NGAN_SACH_SAN_KHAU } from "../dist-test/rudi/motion.js";
import { kiemLop, phanTich } from "./_kiem-lop.mjs";
import { khongCatNepGap } from "./_nep-gap.mjs";

const BUOC_MS = 10;
const khoangCach = (a, b) => Math.hypot(a[0] - b[0], a[1] - b[1]);
const gan = (a, b, sai = 0.02) => Math.abs(a[0] - b[0]) <= sai && Math.abs(a[1] - b[1]) <= sai;

/** Every sampled frame of every performance. */
function moiKhung() {
  const ra = [];
  for (const id of TIET_MUC_IDS) {
    const tm = TIET_MUC[id];
    for (let t = 0; t <= tm.ms; t += BUOC_MS) ra.push({ id, t, tt: khopTai(tm, t) });
    for (const k of tm.khoa) ra.push({ id, t: k.t, tt: khopTai(tm, k.t) });
  }
  return ra;
}

test("lúc nghỉ, tờ thân và khuôn mặt của con rối là của Nếp tĩnh, từng chuỗi, ở mọi pose", () => {
  for (const pose of POSE_NEP) {
    for (const chiTiet of [true, false]) {
      const tinh = hinhNep(pose, { chiTiet });
      const ctx = nguCanhNep(pose, { chiTiet });
      const soLopThanMat = thanNep(ctx).length + matNep(ctx).length;
      const roi = tuTheRoi(tuTheTuPose(pose), { chiTiet });
      assert.deepEqual(roi.slice(0, soLopThanMat), tinh.slice(0, soLopThanMat), `${pose}/${chiTiet}`);
    }
  }
  // the pose's own lean, gaze and face travel with it
  assert.deepEqual(tuTheTuPose("buoc-di").nghieng, tuTheCuaPose("buoc-di").nghieng);
});

test("tay chân ghim đúng bốn khớp của Nếp tĩnh", () => {
  const g = giaiRoi(TU_THE_NGHI);
  assert.ok(gan(g.vaiGan, VAI_GAN, 1e-9) && gan(g.vaiXa, VAI_XA, 1e-9));
  assert.ok(gan(g.hongGan, HONG_GAN, 1e-9) && gan(g.hongXa, HONG_XA, 1e-9));
  // leaning moves the shoulders with the sheet exactly as the still figure's shear does
  const ctx = nguCanhNep("moi", { nghieng: 9 });
  const g9 = giaiRoi(tuThe({ nghieng: 9 }));
  assert.ok(gan(g9.vaiGan, ctx.L, 1e-9) && gan(g9.vaiXa, ctx.R, 1e-9));
  // the feet stand where the still figure's `dung` stance puts them
  assert.ok(gan(g.coChanGan, CO_CHAN_GAN, 1e-6) && gan(g.coChanXa, CO_CHAN_XA, 1e-6));
});

test("IK hai xương: với tới thì tới đúng; quá tầm thì duỗi thẳng về phía mục tiêu; khuỷu theo chiều đã chọn", () => {
  const goc = [0, 0];
  const toi = ik2(goc, [10, 16], 12, 12, 1);
  assert.ok(gan(toi.cuoi, [10, 16], 1e-9));
  assert.ok(Math.abs(khoangCach(goc, toi.khop) - 12) < 1e-9 && Math.abs(khoangCach(toi.khop, toi.cuoi) - 12) < 1e-9);
  const xa = ik2(goc, [30, 40], 12, 12, 1);
  assert.ok(Math.abs(khoangCach(goc, xa.cuoi) - 24) < 1e-9, "quá tầm: duỗi hết 24");
  assert.ok(Math.abs(xa.cuoi[0] / xa.cuoi[1] - 30 / 40) < 1e-9, "quá tầm: vẫn hướng về mục tiêu");
  // the bend side flips with `uon`
  const trai = ik2(goc, [0, 20], 12, 12, 1);
  const phai = ik2(goc, [0, 20], 12, 12, -1);
  assert.ok(trai.khop[0] < 0 && phai.khop[0] > 0);
  // a target on top of the root does not divide by zero
  const sat = ik2(goc, [0, 0], 12, 12, 1);
  assert.ok(Number.isFinite(sat.khop[0]) && Number.isFinite(sat.cuoi[1]));
});

test("giấy không co giãn: mọi khúc tay chân đúng chiều dài ở mọi khung của mọi tiết mục", () => {
  let dem = 0;
  for (const { id, t, tt } of moiKhung()) {
    const g = giaiRoi(tt);
    const ten = `${id}@${t}`;
    for (const [a, b, l] of [
      [g.vaiGan, g.khuyuGan, XUONG.canhTay],
      [g.khuyuGan, g.tayGan, XUONG.cangTay],
      [g.vaiXa, g.khuyuXa, XUONG.canhTay],
      [g.khuyuXa, g.tayXa, XUONG.cangTay],
      [g.hongGan, g.goiGan, XUONG.dui],
      [g.goiGan, g.coChanGan, XUONG.cang],
      [g.hongXa, g.goiXa, XUONG.dui],
      [g.goiXa, g.coChanXa, XUONG.cang],
    ]) {
      assert.ok(Math.abs(khoangCach(a, b) - l) < 1e-6, `${ten}: một khúc dài ${khoangCach(a, b)} thay vì ${l}`);
      dem += 1;
    }
  }
  assert.ok(dem > 6000, `chỉ xét ${dem} khúc`);
});

test("ma trận cho Skia đặt từng bộ phận đúng chỗ tuTheRoi vẽ", () => {
  const bp = boPhanRoi();
  for (const { id, t, tt } of moiKhung().filter((_, i) => i % 5 === 0)) {
    const g = giaiRoi(tt);
    const m = maTranBoPhan(tt, g);
    const ten = `${id}@${t}`;
    // bone ends: local (0, 0) and (0, len) land on the solved joints
    for (const [mt, dau, cuoi, l] of [
      [m.canhTayGan, g.vaiGan, g.khuyuGan, XUONG.canhTay],
      [m.cangTayGan, g.khuyuGan, g.tayGan, XUONG.cangTay],
      [m.canhTayXa, g.vaiXa, g.khuyuXa, XUONG.canhTay],
      [m.cangTayXa, g.khuyuXa, g.tayXa, XUONG.cangTay],
      [m.duiGan, g.hongGan, g.goiGan, XUONG.dui],
      [m.cangGan, g.goiGan, g.coChanGan, XUONG.cang],
      [m.duiXa, g.hongXa, g.goiXa, XUONG.dui],
      [m.cangXa, g.goiXa, g.coChanXa, XUONG.cang],
    ]) {
      assert.ok(gan(apMaTran(mt, [0, 0]), dau, 1e-9) && gan(apMaTran(mt, [0, l]), cuoi, 1e-9), ten);
      // a rigid turn: no stretch, no shear
      assert.ok(Math.abs(mt[0] * mt[3] - mt[1] * mt[2] - 1) < 1e-9, `${ten}: ma trận khúc xương bị co giãn`);
    }
    // the sheet: the upright sheet under the Skia map = the leaned sheet tuTheRoi draws
    const phang = bp.than.map((l) => bienDoiDuong(l.d, m.than));
    const ve = tuTheRoi(tt).slice(0, bp.than.length).map((l) => l.d);
    for (let i = 0; i < phang.length; i += 1) {
      const a = phanTich(phang[i]).filter((x) => x.c !== "Z").flatMap((x) => x.args);
      const b = phanTich(ve[i]).filter((x) => x.c !== "Z").flatMap((x) => x.args);
      assert.equal(a.length, b.length, ten);
      for (let k = 0; k < a.length; k += 1) assert.ok(Math.abs(a[k] - b[k]) <= 0.02, `${ten}: tờ thân lệch ${Math.abs(a[k] - b[k])}`);
    }
    // the eyes: upright ellipses moved to the leaned centre, exactly as the still figure draws them
    const mat = [bienDoiDuong(bp.mat[0].d, m.matTrai), bienDoiDuong(bp.mat[0].d, m.matPhai)];
    const veMat = tuTheRoi(tt).slice(bp.than.length, bp.than.length + 2).map((l) => l.d);
    for (let i = 0; i < 2; i += 1) {
      const a = phanTich(mat[i]).filter((x) => x.c !== "Z").flatMap((x) => x.args);
      const b = phanTich(veMat[i]).filter((x) => x.c !== "Z").flatMap((x) => x.args);
      assert.equal(a.length, b.length, ten);
      for (let k = 0; k < a.length; k += 1) assert.ok(Math.abs(a[k] - b[k]) <= 0.02, `${ten}: mắt lệch ${Math.abs(a[k] - b[k]).toFixed(3)}`);
    }
    // brow and mouth: the expression's set in the sheet's map (the round mouth moved like the eyes)
    const mat7 = bp.mat7[tt.bieuCam].map((l) => bienDoiDuong(l.d, m.than));
    if (tt.bieuCam === "hoi") mat7.push(bienDoiDuong(bp.miengHoi[0].d, m.miengHoi));
    const veMat7 = tuTheRoi(tt).slice(bp.than.length + 2, bp.than.length + 2 + mat7.length).map((l) => l.d);
    for (let i = 0; i < mat7.length; i += 1) {
      const a = phanTich(mat7[i]).filter((x) => x.c !== "Z").flatMap((x) => x.args);
      const b = phanTich(veMat7[i]).filter((x) => x.c !== "Z").flatMap((x) => x.args);
      assert.equal(a.length, b.length, ten);
      for (let k = 0; k < a.length; k += 1) assert.ok(Math.abs(a[k] - b[k]) <= 0.02, `${ten}: mày/miệng lệch ${Math.abs(a[k] - b[k]).toFixed(3)}`);
    }
  }
  // the sheet's map at rest is the identity
  assert.deepEqual([...maTranThan(TU_THE_NGHI)].map((v) => Math.round(v * 1e9) / 1e9), [1, 0, 0, 1, 0, 0]);
});

test("không nét mực nào sơn lên nếp gấp cam, ở mọi khung của mọi tiết mục, cả hai cách đọc", () => {
  let dem = 0;
  let banVe = 0;
  for (const chiTiet of [true, false]) {
    for (const { id, t, tt } of moiKhung()) {
      const ten = `${id}@${t}/${chiTiet ? "96" : "48"}`;
      const lop = tuTheRoi(tt, { chiTiet });
      // A bow TURNS the sheet, and the shape test `laNepGap` only knows the
      // still figure's shear. The puppet has exactly one coral layer (asserted
      // below), so that layer is the fold: identity by role, then the same sweep.
      const ra = khongCatNepGap(ten, lop, undefined, () => true);
      assert.ok(ra !== null, `${ten}: không nhận ra nếp gấp`);
      dem += ra.daXet;
      banVe += 1;
      assert.equal(lop.filter((l) => l.mau === "gap").length, 1, `${ten}: phải đúng một lớp cam`);
    }
  }
  assert.equal(banVe, 2 * moiKhung().length, "máy quét bỏ sót khung");
  // Floor read off the MEASURED count (24/09: 3_468_976 ink points over 1_982
  // frames), not a round number: a scan that stopped looking at most layers
  // would stay above a floor a tenth of the truth.
  assert.ok(dem > 3_000_000, `chỉ xét ${dem} điểm mực`);
});

test("mọi khung qua ngữ pháp Java; khung cuối và khung tĩnh nằm trong hộp 96", () => {
  for (const { id, t, tt } of moiKhung().filter((_, i) => i % 3 === 0)) {
    // performances travel (walk in, walk off, a sheet flying away): a wide stage box
    kiemLop(`${id}@${t}`, tuTheRoi(tt), 96, 96, 60);
  }
  for (const id of TIET_MUC_IDS) {
    const tm = TIET_MUC[id];
    const tinh = tm.khoa[tm.khungTinh].tt;
    kiemLop(`${id} khung tĩnh`, tuTheRoi(tinh), 96, 96, 1);
    if (id !== "gap-thu") kiemLop(`${id} khung cuối`, tuTheRoi(khopTai(tm, tm.ms)), 96, 96, 1);
  }
});

test("ngân sách, khoá, nhịp và mô tả của chín tiết mục", () => {
  assert.equal(TIET_MUC_IDS.length, 9);
  for (const id of TIET_MUC_IDS) {
    const tm = TIET_MUC[id];
    assert.equal(tm.id, id);
    assert.ok(vuaNganSach(tm), `${id}: ${tm.ms}ms, khoá cuối ${tm.khoa.at(-1).t}`);
    assert.ok(tm.ms <= NGAN_SACH_SAN_KHAU.dien);
    for (let i = 1; i < tm.khoa.length; i += 1) assert.ok(tm.khoa[i].t > tm.khoa[i - 1].t, `${id}: khoá không tăng dần ở ${i}`);
    for (const n of tm.nhip) assert.ok(n.t >= 0 && n.t <= tm.ms, `${id}: nhịp rung ngoài tiết mục`);
    assert.ok(tm.khungTinh >= 0 && tm.khungTinh < tm.khoa.length);
    assert.ok(tm.moTa.startsWith("Nếp "), `${id}: câu mô tả`);
    for (const k of tm.khoa) {
      assert.ok(BIEU_CAM.includes(k.tt.bieuCam), `${id}: biểu cảm lạ`);
      if (k.tt.vat) assert.equal(k.tt.vat.id, tm.vat, `${id}: cầm vật không khai`);
    }
  }
  // the one flying thing flies once and is gone at the end (ADR-0037 D11)
  const cuoi = khopTai(TIET_MUC["gap-thu"], TIET_MUC["gap-thu"].ms);
  assert.equal(cuoi.vat.gan, "tu-do");
  assert.equal(cuoi.vat.hien, 0);
});

test("khoảnh khắc tiền: Nếp chỉ bình thản hoặc nhường, ở mọi khung", () => {
  for (const id of ["cam-may", "dong-dau", "cui-cam-on"]) {
    const tm = TIET_MUC[id];
    for (let t = 0; t <= tm.ms; t += BUOC_MS) {
      assert.ok(MAT_KHI_TIEN.includes(khopTai(tm, t).bieuCam), `${id}@${t}: ${khopTai(tm, t).bieuCam}`);
    }
  }
  // the stamp lands on the page's three beats: raised by 300, down at 300 + 130
  const dau = TIET_MUC["dong-dau"];
  assert.ok(dau.khoa.some((k) => k.t === 430) && dau.nhip.some((n) => n.t === 430 && n.kieu === "cham"));
});

test("nội suy: khoá đúng tại mốc, giữa hai khoá thì nằm giữa, easing không vọt quá", () => {
  const tm = TIET_MUC.nhay;
  for (const k of tm.khoa) assert.deepEqual(khopTai(tm, k.t), k.tt);
  const a = tm.khoa[1].tt;
  const b = tm.khoa[2].tt;
  const giua = khopTai(tm, (tm.khoa[1].t + tm.khoa[2].t) / 2);
  assert.ok(giua.dy < a.dy && giua.dy > b.dy, `dy giữa ${giua.dy}`);
  for (const [x1, y1, x2, y2] of [[0.2, 0, 0, 1], [0, 0, 0.2, 1], [0.3, 0, 1, 1]]) {
    let truoc = 0;
    for (let k = 0; k <= 100; k += 1) {
      const y = emBezier(x1, y1, x2, y2, k / 100);
      assert.ok(y >= truoc - 1e-9 && y <= 1 + 1e-9, `easing (${x1},${y1},${x2},${y2}) ở ${k / 100}`);
      truoc = y;
    }
  }
  // before the first key and after the last, the ends hold
  assert.deepEqual(khopTai(tm, -50), tm.khoa[0].tt);
  assert.deepEqual(khopTai(tm, tm.ms + 999), tm.khoa.at(-1).tt);
});

test("tất định: cùng tư thế ra cùng lớp", () => {
  for (const id of TIET_MUC_IDS) {
    const tm = TIET_MUC[id];
    assert.deepEqual(tuTheRoi(khopTai(tm, 333)), tuTheRoi(khopTai(tm, 333)));
  }
});

test("nối tiết mục (M6: kéo tab rồi nhảy): một đồng hồ, nghỉ giữa hai màn, vật cầm mờ đi chứ không biến mất", async () => {
  const { NOI_TIET_MUC_MS, noiTietMuc, vatCuaTietMuc } = await import("../dist-test/rudi/art/nep-dien.js");
  const a = TIET_MUC["keo-tab"];
  const b = TIET_MUC.nhay;
  const n = noiTietMuc([a, b]);
  assert.equal(n.ms, a.ms + NOI_TIET_MUC_MS + b.ms);
  for (let i = 1; i < n.khoa.length; i += 1) assert.ok(n.khoa[i].t > n.khoa[i - 1].t, `khoá ${i} không tăng dần`);
  assert.equal(n.khoa[n.khungTinh].t, a.ms + NOI_TIET_MUC_MS + b.khoa[b.khungTinh].t, "khung tĩnh là của màn cuối");
  assert.deepEqual(n.nhip.map((x) => x.t), [...a.nhip.map((x) => x.t), ...b.nhip.map((x) => x.t + a.ms + NOI_TIET_MUC_MS)]);
  // the tab held at the end of the pull fades across the pause
  const giua = khopTai(n, a.ms + NOI_TIET_MUC_MS / 2);
  assert.equal(giua.vat.id, "the-keo");
  assert.ok(giua.vat.hien > 0 && giua.vat.hien < 1, `vật cầm ở giữa quãng nghỉ: ${giua.vat.hien}`);
  assert.equal(khopTai(n, a.ms + NOI_TIET_MUC_MS).vat.hien, 0);
  assert.deepEqual(vatCuaTietMuc(n), ["the-keo"]);
  // one performance joins to itself unchanged
  assert.equal(noiTietMuc([b]), b);
  // and the joined clock keeps the fold rule
  for (let t = 0; t <= n.ms; t += 10) {
    const lop = tuTheRoi(khopTai(n, t));
    assert.ok(khongCatNepGap(`M6@${t}`, lop, undefined, () => true) !== null);
  }
});
