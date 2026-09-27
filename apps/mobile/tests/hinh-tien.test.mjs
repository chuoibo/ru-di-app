/* Hình của các màn tiền (kế hoạch UI v3, S1): ghế quanh bàn chia bill, dải
 * băng washi của đợt thu, mũi tên mực của quyết toán.
 *
 * Chạy từ apps/mobile:
 *     npx tsc -p tsconfig.test.json && node tools/fixup-esm.mjs && node --test tests/hinh-tien.test.mjs
 *
 * Chứng minh: bàn của một đôi thấp (danh sách món bên dưới còn nằm trong màn
 * đầu, `.maestro/28` bấm vào đó mà không cuộn); không ghế nào tràn khung hay
 * đè ghế khác, đĩa nằm trên mặt bàn và ra khỏi thẻ món; dải băng không bao giờ
 * nuốt mất một lượt về; mũi tên dừng trước hình nhân, đúng ngữ pháp đường vẽ
 * của Java, tất định. Không chứng minh: bàn ĐẸP, hay tay người kéo thẻ trúng
 * ghế -- đó là việc của ảnh chụp và máy thật.
 */
import assert from "node:assert/strict";
import test from "node:test";

import { BANG_NGAN_NHAT, GHE, R_NHAN, dayBang, diemTrenMui, hinhCuaGhe, hopGhe, hopNhan, soDoChuyen, tenCuaGhe, theMon, viTriGhe } from "../dist-test/rudi/ui/hinh-tien.js";
import { phanTich } from "./_kiem-lop.mjs";

const nguoi = (n) => Array.from({ length: n }, (_, i) => ({ id: `nguoi-${i}`, name: `Người ${i + 1}` }));
const BE_RONG = [300, 328, 380, 520, 700];

test("bàn của một đôi: hai người ngồi hai đầu, bàn thấp hơn hẳn bàn tròn", () => {
  for (const w of BE_RONG) {
    const doi = viTriGhe(nguoi(2), w);
    assert.ok(doi.cao <= 160, `w=${w}: bàn hai người cao ${doi.cao} > 160`);
    assert.ok(Math.abs(doi.ghe[0].y - doi.cy) < 1e-6 && Math.abs(doi.ghe[1].y - doi.cy) < 1e-6, "hai người ngồi ngang tâm bàn");
    assert.ok(doi.ghe[0].x < doi.cx && doi.ghe[1].x > doi.cx, "một người mỗi đầu");
    assert.ok(viTriGhe(nguoi(3), w).cao - doi.cao >= 80, `w=${w}: bàn tròn phải cao hơn bàn đôi ít nhất 80`);
  }
});

test("không ghế nào tràn khung, không hai ghế nào đè nhau", () => {
  for (const w of BE_RONG) {
    for (let n = 1; n <= 8; n += 1) {
      const ban = viTriGhe(nguoi(n), w);
      const hop = ban.ghe.map(hopGhe);
      for (const [i, h] of hop.entries()) {
        assert.ok(h.trai >= -1 && h.phai <= w + 1, `w=${w} n=${n} ghế ${i} tràn ngang: ${h.trai.toFixed(1)}..${h.phai.toFixed(1)}`);
        assert.ok(h.tren >= -1 && h.duoi <= ban.cao + 1, `w=${w} n=${n} ghế ${i} tràn dọc: ${h.tren.toFixed(1)}..${h.duoi.toFixed(1)} / ${ban.cao}`);
      }
      // Standees (not whole boxes with their name lines) must not overlap.
      for (let a = 0; a < n; a += 1) {
        for (let b = a + 1; b < n; b += 1) {
          const d = Math.hypot(ban.ghe[a].x - ban.ghe[b].x, ban.ghe[a].y - ban.ghe[b].y);
          assert.ok(d >= GHE, `w=${w} n=${n}: ghế ${a} và ${b} cách ${d.toFixed(1)} < ${GHE}`);
        }
      }
    }
  }
});

const giao = (a, b) => a.trai < b.phai && b.trai < a.phai && a.tren < b.duoi && b.tren < a.duoi;

test("tên của một ghế không đè lên hình nhân hay tên của ghế khác", () => {
  for (const w of BE_RONG) {
    for (let n = 1; n <= 8; n += 1) {
      const ghe = viTriGhe(nguoi(n), w).ghe;
      for (const a of ghe) {
        for (const b of ghe) {
          if (a === b) continue;
          assert.ok(!giao(tenCuaGhe(a), hinhCuaGhe(b)), `w=${w} n=${n}: tên ${a.name} đè lên hình ${b.name}`);
          assert.ok(!giao(tenCuaGhe(a), tenCuaGhe(b)), `w=${w} n=${n}: tên ${a.name} đè lên tên ${b.name}`);
        }
      }
    }
  }
});

test("ghế sau bàn mang tên ở trên đầu; ghế trước bàn ở dưới", () => {
  const ban = viTriGhe(nguoi(6), 380);
  for (const g of ban.ghe) assert.equal(g.truoc, g.y >= ban.cy - 1e-6, `${g.name}: y=${g.y.toFixed(1)} cy=${ban.cy.toFixed(1)}`);
  assert.ok(ban.ghe.some((g) => !g.truoc) && ban.ghe.some((g) => g.truoc));
});

test("đĩa nằm trên mặt bàn, ngoài thẻ món ở giữa", () => {
  for (const w of BE_RONG) {
    for (let n = 1; n <= 8; n += 1) {
      const ban = viTriGhe(nguoi(n), w);
      for (const g of ban.ghe) {
        const e = ((g.dia.x - ban.cx) / ban.rx) ** 2 + ((g.dia.y - ban.cy) / ban.ry) ** 2;
        assert.ok(e < 1, `w=${w} n=${n}: đĩa của ${g.name} rơi khỏi bàn (${e.toFixed(2)})`);
        const the = theMon(ban.rx);
        const trongThe = Math.abs(g.dia.x - ban.cx) < the.w / 2 - 2 && Math.abs(g.dia.y - ban.cy) < the.h / 2 - 2;
        assert.ok(!trongThe, `w=${w} n=${n}: đĩa của ${g.name} nằm dưới thẻ món`);
      }
    }
  }
});

test("dải băng: không có gì thì không có băng, về đủ thì kín, một lượt về trong nhiều lượt vẫn thấy", () => {
  assert.equal(dayBang(0, 10, 300), 0);
  assert.equal(dayBang(3, 0, 300), 0, "không có lượt nào thì không có băng");
  assert.equal(dayBang(3, 3, 300), 300);
  assert.equal(dayBang(5, 3, 300), 300, "về dư không tràn khỏi rãnh");
  assert.ok(dayBang(1, 40, 300) >= BANG_NGAN_NHAT, "một lượt về trong 40 lượt vẫn phải thấy băng");
  let truoc = 0;
  for (let da = 0; da <= 12; da += 1) {
    const d = dayBang(da, 12, 320);
    assert.ok(d >= truoc, `băng co lại khi thêm lượt về (${da})`);
    truoc = d;
  }
});

test("mũi tên quyết toán: đúng ngữ pháp, dừng trước hình nhân, tất định", () => {
  const chuyen = [
    { tu: "a", toi: "b" },
    { tu: "c", toi: "b" },
    { tu: "d", toi: "e" },
  ];
  for (const w of BE_RONG) {
    const sd = soDoChuyen(chuyen, w);
    assert.equal(sd.nguoi.length, 5, "chỉ những người có tên trong một khoản chuyển");
    assert.equal(sd.muiTen.length, 3);
    for (const m of sd.muiTen) {
      assert.deepEqual(phanTich(m.d).map((c) => c.c), ["M", "C"], m.d);
      assert.deepEqual(phanTich(m.dau).map((c) => c.c), ["M", "L", "L"], m.dau);
      const dich = sd.nguoi.find((p) => p.id === m.toi);
      const nguon = sd.nguoi.find((p) => p.id === m.tu);
      assert.ok(Math.hypot(m.mui[0] - dich.x, m.mui[1] - dich.y) >= R_NHAN, `mũi tên ${m.tu}→${m.toi} đâm vào hình nhân`);
      const [x0, y0] = phanTich(m.d)[0].args;
      // The path prints two decimals: allow that rounding, nothing more.
      assert.ok(Math.hypot(x0 - nguon.x, y0 - nguon.y) >= R_NHAN - 0.02, `mũi tên ${m.tu}→${m.toi} mọc từ trong hình nhân`);
      assert.ok(m.dai > 0 && Number.isFinite(m.dai));
    }
    for (const p of sd.nguoi) {
      const h = hopNhan(p);
      assert.ok(h.trai >= -1 && h.phai <= w + 1 && h.tren >= -1 && h.duoi <= sd.h + 1, `hình nhân ${p.id} tràn khung ${w}×${sd.h}`);
    }
    assert.deepEqual(soDoChuyen(chuyen, w), sd, "cùng đầu vào, cùng hình");
  }
  const doi = soDoChuyen([{ tu: "a", toi: "b" }], 380);
  assert.ok(doi.h <= 120, `hai người thì sơ đồ thấp (${doi.h})`);
  assert.ok(doi.nguoi[0].x < doi.nguoi[1].x && Math.abs(doi.nguoi[0].y - doi.nguoi[1].y) < 1e-6, "hai người: trái và phải");
  assert.deepEqual(soDoChuyen([], 380).muiTen, [], "không khoản chuyển nào thì không mũi tên nào");
});

test("mũi tên không đi xuyên qua hình nhân thứ ba, người trả và người nhận ở hai phía", () => {
  // The shape of a real minimal settlement: many owe one, a few owe another.
  const nhom = [
    { tu: "thai", toi: "anh" },
    { tu: "linh", toi: "anh" },
    { tu: "chau", toi: "anh" },
    { tu: "thao", toi: "anh" },
    { tu: "vy", toi: "anh" },
    { tu: "vy", toi: "huy" },
    { tu: "kiet", toi: "huy" },
  ];
  const nho = [
    { tu: "a", toi: "c" },
    { tu: "b", toi: "c" },
    { tu: "b", toi: "d" },
  ];
  for (const chuyen of [nhom, nho, [{ tu: "a", toi: "b" }]]) {
    for (const w of BE_RONG) {
      const sd = soDoChuyen(chuyen, w);
      const nguoiTra = new Set(chuyen.map((c) => c.tu));
      for (const m of sd.muiTen) {
        const [x0, y0] = phanTich(m.d)[0].args;
        for (const [x, y] of diemTrenMui([x0, y0], m.dieuKhien, m.mui)) {
          for (const p of sd.nguoi) {
            // Nobody's standee (head and body, a circle round them) or name
            // (a band 72 wide) is crossed -- the arrow's own two people
            // included, once it has left them.
            const quaNguoi = Math.hypot(x - p.x, y - (p.y - 3)) < 24;
            const yTen = p.ten === "tren" ? [p.y - 46, p.y - 28] : [p.y + 22, p.y + 40];
            const quaTen = Math.abs(x - p.x) < 36 && y > yTen[0] && y < yTen[1];
            assert.ok(!quaNguoi && !quaTen, `w=${w}: mũi tên ${m.tu}→${m.toi} đi qua ${quaNguoi ? "hình" : "tên"} của ${p.id} tại (${x.toFixed(0)}, ${y.toFixed(0)})`);
          }
        }
        assert.ok(Math.hypot(m.mui[0] - x0, m.mui[1] - y0) >= 24, `w=${w}: mũi tên ${m.tu}→${m.toi} ngắn quá để đọc là mũi tên`);
      }
      // Payers and the paid never share a row or a column.
      const yTra = new Set(sd.nguoi.filter((p) => nguoiTra.has(p.id)).map((p) => Math.round(p.y)));
      const yNhan = new Set(sd.nguoi.filter((p) => !nguoiTra.has(p.id)).map((p) => Math.round(p.y)));
      const xTra = new Set(sd.nguoi.filter((p) => nguoiTra.has(p.id)).map((p) => Math.round(p.x)));
      const xNhan = new Set(sd.nguoi.filter((p) => !nguoiTra.has(p.id)).map((p) => Math.round(p.x)));
      const chungHang = [...yTra].some((y) => yNhan.has(y));
      const chungCot = [...xTra].some((x) => xNhan.has(x));
      assert.ok(!chungHang || !chungCot, `w=${w}: người trả và người nhận lẫn vào nhau`);
    }
  }
});
