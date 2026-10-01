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

import { BANG_NGAN_NHAT, BAN_DAI_TU, CAO_THE_TRON, DIA, GHE, NHAN_DAY, NHAN_GON, R_NHAN, tenGoi, theMonToiDa, dayBang, diemTrenMui, hinhCuaGhe, hopGhe, hopNhan, soDoChuyen, tenCuaGhe, theMon, viTriGhe } from "../dist-test/rudi/ui/hinh-tien.js";
import { phanTich } from "./_kiem-lop.mjs";

const nguoi = (n) => Array.from({ length: n }, (_, i) => ({ id: `nguoi-${i}`, name: `Người ${i + 1}` }));
const BE_RONG = [288, 300, 328, 380, 520, 700];
/** Up to a big group's twenty (QA UI-050 measured nine seats crossing at 288–358). */
const TOI_DA = 20;

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
    for (let n = 1; n <= TOI_DA; n += 1) {
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
    for (let n = 1; n <= TOI_DA; n += 1) {
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
    for (let n = 1; n <= TOI_DA; n += 1) {
      const ban = viTriGhe(nguoi(n), w);
      for (const g of ban.ghe) {
        if (ban.ban) {
          const b = ban.ban;
          assert.ok(g.dia.x > b.trai && g.dia.x < b.trai + b.rong && g.dia.y > b.tren && g.dia.y < b.tren + b.cao, `w=${w} n=${n}: đĩa của ${g.name} rơi khỏi bàn dài`);
        } else {
          const e = ((g.dia.x - ban.cx) / ban.rx) ** 2 + ((g.dia.y - ban.cy) / ban.ry) ** 2;
          assert.ok(e < 1, `w=${w} n=${n}: đĩa của ${g.name} rơi khỏi bàn (${e.toFixed(2)})`);
        }
        // Against the widest the card may grow to (`theMonToiDa`) and its
        // real height, and the WHOLE plate as drawn: a centre just outside the
        // card left half the plate under it (B4 finish review).
        const the = { w: theMonToiDa(ban), h: ban.ban ? ban.the.h : CAO_THE_TRON };
        const duoiThe = Math.abs(g.dia.x - ban.cx) < the.w / 2 + DIA.rx && Math.abs(g.dia.y - ban.cy) < the.h / 2 + DIA.ry;
        assert.ok(!duoiThe, `w=${w} n=${n}: đĩa của ${g.name} nằm (một phần) dưới thẻ món`);
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
      const h = hopNhan(p, sd.rongNhan);
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
            const quaTen = Math.abs(x - p.x) < sd.rongNhan * 0.41 && y > yTen[0] && y < yTen[1];
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

test("nhóm đông ngồi bàn dài: từ BAN_DAI_TU người, hai dãy dọc bàn, thẻ món ở đầu bàn (QA UI-050)", () => {
  assert.equal(viTriGhe(nguoi(BAN_DAI_TU - 1), 380).kieu, "tron");
  for (const w of BE_RONG) {
    for (let n = BAN_DAI_TU; n <= TOI_DA; n += 1) {
      const ban = viTriGhe(nguoi(n), w);
      assert.equal(ban.kieu, "dai");
      // Two sides of the table, seats alternating, so the order round the table is kept.
      const trai = ban.ghe.filter((g) => g.x < ban.cx).length;
      assert.ok(Math.abs(trai - (n - trai)) <= 1, `w=${w} n=${n}: hai dãy lệch ${trai}/${n - trai}`);
      // Every seat is a full 48 dp to touch, and no seat starts above the card's foot.
      for (const g of ban.ghe) {
        const h = hopGhe(g);
        assert.ok(h.duoi - h.tren >= 48, `w=${w} n=${n}: ghế ${g.name} cao ${h.duoi - h.tren} < 48`);
        assert.ok(g.y - GHE * 0.75 > ban.cy + ban.the.h / 2, `w=${w} n=${n}: ghế ${g.name} đứng ngang thẻ món`);
      }
    }
  }
});

test("sơ đồ quyết toán nhóm đông: tên không đè nhau, không ra ngoài khung, hoặc không vẽ (QA UI-054)", () => {
  // The shape of a minimal settlement: most owe a few.
  const nhom = (n, soNhan) => Array.from({ length: n - soNhan }, (_, i) => ({ tu: `tra-${i}`, toi: `nhan-${i % soNhan}` }));
  for (const [n, soNhan] of [[6, 2], [10, 3], [12, 3], [20, 5]]) {
    for (const w of [288, 300, 320, 360, 398, 700]) {
      const sd = soDoChuyen(nhom(n, soNhan), w);
      if (sd.quaDong) {
        assert.equal(sd.nguoi.length, 0);
        continue;
      }
      const hop = sd.nguoi.map((p) => hopNhan(p, sd.rongNhan));
      for (const [i, h] of hop.entries()) {
        assert.ok(h.trai >= -1 && h.phai <= w + 1, `n=${n} w=${w}: nhãn ${sd.nguoi[i].id} ra ngoài khung`);
        for (const k of hop.slice(i + 1)) assert.ok(!giao(h, k), `n=${n} w=${w}: hai nhãn đè nhau`);
      }
    }
  }
  // Ten people fit at every phone width, with given names when they must.
  for (const w of [288, 320, 398]) assert.equal(soDoChuyen(nhom(10, 3), w).quaDong, false, `10 người ở ${w} vẫn phải vẽ được`);
  assert.equal(soDoChuyen(nhom(10, 3), 320).rongNhan, NHAN_GON);
  assert.equal(soDoChuyen(nhom(6, 2), 390).rongNhan, NHAN_DAY);
  assert.equal(tenGoi("Nguyễn Minh Anh"), "Anh");
});

test("tenVua: tên rút từ phía họ, giữ phần đuôi dài nhất còn vừa ô ghế (QA UI-050)", async () => {
  const { tenVua } = await import("../dist-test/rudi/ui/hinh-tien.js");
  assert.equal(tenVua("Khánh Linh", 72), "Khánh Linh", "vừa thì để nguyên");
  assert.equal(tenVua("Chat Test 07", 72), "Test 07");
  assert.equal(tenVua("Nguyễn Thị Minh Anh", 72), "Minh Anh");
  assert.equal(tenVua("  Bình  ", 72), "Bình");
  assert.equal(tenVua("Nguyễn Hoàngthiênphúc", 72), "Hoàngthiênphúc", "một chữ dài vẫn là tên gọi, ô tự cắt đuôi");
  const hai = ["Chat Test 07", "Chat Test 08"].map((t) => tenVua(t, 72));
  assert.notEqual(hai[0], hai[1], "hai người cùng họ vẫn phân biệt được");
});
