/**
 * Tám khoảnh khắc của Nếp (ADR-0037 D4, D5): mỗi sự kiện diễn một lần trong
 * phiên, giảm chuyển động thì chỉ khung tĩnh, không bao giờ ở lỗi hay trạng
 * thái chưa hợp lệ, và khoảnh khắc tiền chỉ mang mặt bình thản hoặc nhường.
 *
 * Run from apps/mobile:
 *     npx tsc -p tsconfig.test.json && node tools/fixup-esm.mjs && node --test tests/khoanh-khac.test.mjs
 */
import assert from "node:assert/strict";
import test from "node:test";

import { KHOANH_KHAC, KHOANH_KHAC_IDS, TRAN_KHOANH_KHAC, daDienRoi, ghiDaDien, khoaKhoanhKhac, nenDien, quenKhoanhKhac } from "../dist-test/rudi/khoanh-khac.js";
import { MAT_KHI_TIEN, TIET_MUC, TIET_MUC_IDS, khopTai } from "../dist-test/rudi/art/nep-dien.js";
import { NGAN_SACH_SAN_KHAU } from "../dist-test/rudi/motion.js";

test("một sự kiện diễn một lần; lần sau là khung tĩnh", () => {
  quenKhoanhKhac();
  const khoa = khoaKhoanhKhac("M3", "chi-1");
  assert.equal(nenDien({ khoa, reduced: false }), "dien");
  assert.equal(nenDien({ khoa, reduced: false }), "tinh", "mount lại không diễn lại");
  assert.equal(nenDien({ khoa: khoaKhoanhKhac("M3", "chi-2"), reduced: false }), "dien", "sự kiện khác vẫn diễn");
  assert.equal(nenDien({ khoa: khoaKhoanhKhac("M4", "chi-1"), reduced: false }), "dien", "cùng id nhưng khoảnh khắc khác");
});

test("giảm chuyển động: khung tĩnh ngay từ đầu, và vẫn tính là đã diễn", () => {
  quenKhoanhKhac();
  const khoa = khoaKhoanhKhac("M1", "khay");
  assert.equal(nenDien({ khoa, reduced: true }), "tinh");
  assert.ok(daDienRoi(khoa));
  assert.equal(nenDien({ khoa, reduced: false }), "tinh", "bật lại chuyển động không phát lại sự kiện đã qua");
});

test("không bao giờ ở lỗi, trạng thái chưa hợp lệ, hay khoá rỗng; và không ghi nhớ chúng", () => {
  quenKhoanhKhac();
  const khoa = khoaKhoanhKhac("M4", "dot-1");
  assert.equal(nenDien({ khoa, reduced: false, coLoi: true }), "khong");
  assert.equal(nenDien({ khoa, reduced: false, hopLe: false }), "khong");
  assert.equal(nenDien({ khoa: "  ", reduced: false }), "khong");
  assert.ok(!daDienRoi(khoa), "một lần lỗi không được «tiêu» mất khoảnh khắc thật");
  assert.equal(nenDien({ khoa, reduced: false }), "dien");
});

test("sổ nhớ có trần: sự kiện cũ nhất bị quên trước, sự kiện mới dùng lại vẫn nằm cuối", () => {
  quenKhoanhKhac();
  for (let i = 0; i < TRAN_KHOANH_KHAC; i += 1) ghiDaDien(`k${i}`);
  ghiDaDien("k0");
  ghiDaDien("moi");
  assert.ok(!daDienRoi("k1"), "cũ nhất bị quên");
  assert.ok(daDienRoi("k0"), "dùng lại thì được nhớ tiếp");
  assert.ok(daDienRoi("moi"));
});

test("tám khoảnh khắc trỏ tới tiết mục có thật; khoảnh khắc tiền chỉ mang mặt bình thản hoặc nhường", () => {
  assert.equal(KHOANH_KHAC_IDS.length, 8);
  const dung = new Set();
  for (const id of KHOANH_KHAC_IDS) {
    const kk = KHOANH_KHAC[id];
    let tong = 0;
    for (const tm of kk.tietMuc) {
      assert.ok(TIET_MUC_IDS.includes(tm), `${id}: tiết mục lạ ${tm}`);
      dung.add(tm);
      tong += TIET_MUC[tm].ms;
      if (kk.tien) {
        for (let t = 0; t <= TIET_MUC[tm].ms; t += 10) assert.ok(MAT_KHI_TIEN.includes(khopTai(TIET_MUC[tm], t).bieuCam), `${id}/${tm}@${t}`);
      }
    }
    assert.ok(tong <= 2 * NGAN_SACH_SAN_KHAU.dien, `${id}: ${tong}ms là bắt người ta xem kịch`);
  }
  assert.deepEqual([...dung].sort(), [...TIET_MUC_IDS].sort(), "tiết mục nào cũng có khoảnh khắc dùng nó");
  assert.deepEqual(KHOANH_KHAC_IDS.filter((id) => KHOANH_KHAC[id].tien), ["M2", "M3", "M4"]);
});
