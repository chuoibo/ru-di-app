/**
 * Vật giấy của sân khấu (ADR-0037 D1): hoá đơn răng cưa, vé có khuyết, tem đục
 * lỗ, cuống xé, phong bì. Mỗi hình là hàm thuần của kích thước đo được.
 *
 * Ba điều test này giữ, trên mọi cỡ từ phone hẹp tới tablet:
 *   1. Đường ra đúng ngữ pháp Java `PathParser` nhận (một token lạ là app chết
 *      ở khung hình đầu) và nằm trong hộp của nó: vật giấy không bao giờ tràn
 *      ra ngoài chỗ component đã đo.
 *   2. Hình là hình thật chứ không phải hộp chữ nhật đổi tên: răng cưa có đỉnh
 *      đúng độ sâu, khuyết vé cắn vào giấy, tem có đủ lỗ ở bốn cạnh, phong bì
 *      mở thì nắp lật lên.
 *   3. Tất định: cùng cỡ ra cùng đường, nên mép xé không «nháy» khi re-render.
 *
 * Run from apps/mobile:
 *     npx tsc -p tsconfig.test.json && node tools/fixup-esm.mjs && node --test tests/giay-vat-the.test.mjs
 */
import assert from "node:assert/strict";
import test from "node:test";

import { diemRangCua, diemXe, hinhCuong, hinhHoaDon, hinhPhongBi, hinhTem, hinhVe } from "../dist-test/rudi/art/giay.js";
import { kiemLop, phanTich } from "./_kiem-lop.mjs";

// Narrow phone card, wide phone, the sketch frame, a tablet column, a spread page.
const RONG = [96, 160, 288, 343, 612];
const CAO = [48, 96, 180];

const to = (d) => ({ d, mau: "giay" });
const vien = (d) => ({ d, mau: "muc", net: 1 });

/** Every end point of every command, in order. */
function diemCuoi(d) {
  return phanTich(d)
    .filter(({ c }) => c !== "Z")
    .map(({ args }) => [args.at(-2), args.at(-1)]);
}

test("mọi vật giấy, mọi cỡ: ngữ pháp Java nhận và nằm trong hộp", () => {
  for (const w of RONG) {
    for (const h of CAO) {
      const ten = `${w}×${h}`;
      for (const tuyChon of [{}, { rangTren: true }, { rangTren: true, rangDuoi: false }, { buoc: 6, sau: 3 }]) {
        const hd = hinhHoaDon(w, h, tuyChon);
        kiemLop(`hoá đơn ${ten} ${JSON.stringify(tuyChon)}`, [to(hd.nen), vien(hd.vien)], w, h, 0.01);
      }
      const ve = hinhVe(w, h);
      kiemLop(`vé ${ten}`, [to(ve.nen), vien(ve.vien), vien(ve.duc)], w, h, 0.01);
      kiemLop(`tem ${ten}`, [to(hinhTem(w, h))], w, h, 0.01);
      const cuong = hinhCuong(w, h);
      kiemLop(`cuống ${ten}`, [to(cuong.nen), vien(cuong.vien)], w, h, 0.01);
      for (const mo of [0, 0.35, 1]) {
        const pb = hinhPhongBi(w, h, mo);
        kiemLop(`phong bì ${ten} mở ${mo}`, [to(pb.than), to(pb.nap), vien(pb.nep)], w, pb.cao, 0.01);
      }
    }
  }
});

test("răng cưa: hai đầu nằm trên mép, đỉnh răng đúng độ sâu, số răng theo bước", () => {
  const diem = diemRangCua(0, 200, 4, 10, 4, -1);
  assert.deepEqual(diem[0], [0, 4]);
  assert.deepEqual(diem.at(-1), [200, 4]);
  const dinh = diem.filter((_, i) => i % 2 === 1);
  assert.equal(dinh.length, 20, "200 chia bước 10 là 20 răng");
  for (const [, y] of dinh) assert.equal(y, 0, "đỉnh răng hướng lên phải chạm mép trên");
  // the bottom edge of a receipt runs right to left, teeth pointing down
  const hd = diemCuoi(hinhHoaDon(200, 80).nen);
  const day = hd.filter(([, y]) => y > 40);
  assert.ok(day.some(([, y]) => y === 80) && day.some(([, y]) => y === 76), "mép dưới phải có cả chân (76) lẫn đỉnh (80)");
  assert.ok(hd.filter(([, y]) => y === 0).length === 2, "mép trên phẳng khi không xin răng");
});

test("vé: hai khuyết cắn vào giấy đúng chỗ xé, đường đục nằm giữa hai khuyết", () => {
  const w = 300;
  const h = 100;
  const ve = hinhVe(w, h, { rKhuyet: 8 });
  assert.ok(Math.abs(ve.xCat - w * 0.72) < 1e-9, "chỗ xé mặc định ở 72% bề ngang");
  const diem = diemCuoi(ve.nen);
  const gan = (p, q) => Math.abs(p[0] - q[0]) < 0.01 && Math.abs(p[1] - q[1]) < 0.01;
  assert.ok(diem.some((p) => gan(p, [ve.xCat, 8])), "khuyết trên phải cắn xuống tới y = r");
  assert.ok(diem.some((p) => gan(p, [ve.xCat, h - 8])), "khuyết dưới phải cắn lên tới y = h - r");
  const duc = diemCuoi(ve.duc);
  assert.ok(duc.every(([x]) => x === ve.xCat), "đường đục dọc theo chỗ xé");
  assert.ok(duc[0][1] > 8 && duc.at(-1)[1] < h - 8, "đường đục không chạm vào khuyết");
  // a ticket too narrow for the default still keeps the notch clear of the corners
  const hep = hinhVe(60, 40);
  assert.ok(hep.xCat > 0 && hep.xCat < 60);
});

test("tem: mỗi cạnh có lỗ đục, tổng số lỗ theo bước", () => {
  const w = 90;
  const h = 54;
  const d = hinhTem(w, h, 9, 2.6);
  // each bite is two quarter-circle cubics
  const soC = phanTich(d).filter(({ c }) => c === "C").length;
  assert.equal(soC, 2 * 2 * (Math.round(w / 9) + Math.round(h / 9)));
  const diem = diemCuoi(d);
  assert.ok(diem.some(([x, y]) => Math.abs(y - 2.6) < 0.01 && x > 0 && x < w), "lỗ mép trên cắn xuống");
  assert.ok(diem.some(([x, y]) => Math.abs(x - (w - 2.6)) < 0.01 && y > 0 && y < h), "lỗ mép phải cắn vào trong");
  assert.ok(diem.some(([x, y]) => Math.abs(y - (h - 2.6)) < 0.01 && x > 0 && x < w), "lỗ mép dưới cắn lên");
  assert.ok(diem.some(([x, y]) => Math.abs(x - 2.6) < 0.01 && y > 0 && y < h), "lỗ mép trái cắn vào trong");
});

test("mép xé: hai đầu nằm trên đường, mọi điểm trong biên độ, tất định", () => {
  const diem = diemXe(0, 240, 50, 3);
  assert.deepEqual(diem[0], [0, 50]);
  assert.deepEqual(diem.at(-1), [240, 50]);
  for (const [, y] of diem) assert.ok(Math.abs(y - 50) <= 3 + 1e-9);
  assert.ok(new Set(diem.map(([, y]) => y.toFixed(3))).size >= 5, "mép xé phải gồ ghề, không phải đường thẳng");
  assert.deepEqual(diemXe(0, 240, 50, 3), diem);
});

test("phong bì: đóng thì nắp úp xuống thân, mở thì nắp lật lên tới đỉnh hộp", () => {
  const w = 200;
  const h = 120;
  const dinh = (mo) => {
    const pb = hinhPhongBi(w, h, mo);
    return { pb, y: diemCuoi(pb.nap).find(([x]) => x === w / 2)[1] };
  };
  const dong = dinh(0);
  const mo = dinh(1);
  assert.equal(dong.pb.yThan, h * 0.55);
  assert.equal(dong.pb.cao, h * 0.55 + h);
  assert.ok(dong.y > dong.pb.yThan, "đóng: đỉnh nắp nằm trên thân");
  assert.equal(mo.y, 0, "mở: đỉnh nắp lên tới mép trên của hộp vẽ");
  let truoc = Infinity;
  for (let k = 0; k <= 10; k += 1) {
    const y = dinh(k / 10).y;
    assert.ok(y <= truoc, `nắp phải lật lên đều, kẹt ở ${k / 10}`);
    truoc = y;
  }
  assert.equal(dinh(-2).y, dong.y, "mở < 0 bị kẹp");
  assert.equal(dinh(3).y, mo.y, "mở > 1 bị kẹp");
});

test("tất định: cùng cỡ ra cùng đường", () => {
  for (const w of RONG) {
    assert.deepEqual(hinhHoaDon(w, 96), hinhHoaDon(w, 96));
    assert.deepEqual(hinhVe(w, 96), hinhVe(w, 96));
    assert.equal(hinhTem(w, 96), hinhTem(w, 96));
    assert.deepEqual(hinhCuong(w, 96), hinhCuong(w, 96));
    assert.deepEqual(hinhPhongBi(w, 96, 0.5), hinhPhongBi(w, 96, 0.5));
  }
});

test("đường đục: hai đầu là mực, các gạch đều nhau, ngữ pháp Java nhận", async () => {
  const { duongDut } = await import("../dist-test/rudi/art/giay.js");
  const d = duongDut([10, 5], [10, 95], 4, 3);
  const lenh = phanTich(d);
  assert.ok(lenh.every(({ c }) => c === "M" || c === "L"));
  const doan = [];
  for (let i = 0; i < lenh.length; i += 2) doan.push([lenh[i].args, lenh[i + 1].args]);
  assert.deepEqual(doan[0][0], [10, 5], "đầu đường là mực");
  assert.ok(Math.abs(doan.at(-1)[1][1] - 95) < 0.02, "cuối đường là mực");
  const dai = doan.map(([p, q]) => Math.hypot(q[0] - p[0], q[1] - p[1]));
  for (const l of dai) assert.ok(Math.abs(l - dai[0]) < 0.03, `gạch lệch ${l} với ${dai[0]}`);
  assert.ok(doan.length >= 12, `chỉ ${doan.length} gạch cho 90dp`);
  kiemLop("đường đục", [{ d, mau: "muc", net: 1 }], 20, 100, 0.01);
  assert.equal(duongDut([0, 0], [0, 0]), "M 0 0 L 0 0");
});

test("nét chữ ký: một đường mở, đúng ngữ pháp Java, nằm trong hộp, bắt đầu ở chỗ bút chạm, tất định", async () => {
  const { netChuKy } = await import("../dist-test/rudi/art/giay.js");
  for (const w of [120, 160, 240, 400]) {
    const d = netChuKy(w, 16);
    const lenh = phanTich(d);
    assert.equal(lenh[0].c, "M");
    assert.ok(lenh.slice(1).every(({ c }) => c === "C"), "chỉ M rồi C: một nét bút, không khép");
    assert.ok(!/Z/.test(d), "nét chữ ký là đường mở");
    kiemLop(`chữ ký ${w}`, [{ d, mau: "muc", net: 1.8 }], w, 16);
    const cuoi = lenh.at(-1).args;
    assert.ok(cuoi[4] > w * 0.9, `nét kết thúc gần mép phải (${cuoi[4]} / ${w})`);
    assert.equal(netChuKy(w, 16), d, "cùng bề rộng, cùng nét");
  }
});
