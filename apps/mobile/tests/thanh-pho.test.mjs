/* Sân khấu thành phố của «Khám phá» (ADR-0037 D1, kế hoạch UI v3 S4).
 *
 * Chạy từ apps/mobile:
 *     npx tsc -p tsconfig.test.json && node tools/fixup-esm.mjs && node --test tests/thanh-pho.test.mjs
 *
 * Giữ bốn điều, trên hàm thật:
 *   1. Mọi điểm đến máy chủ liệt kê (đọc thẳng từ destinations_vn.py) đều có
 *      ký hoạ riêng; điểm đến lạ nhận bưu thiếp chung mang tên máy chủ gửi.
 *   2. Mọi sân khấu hợp lệ theo kiemSanKhau, trong khung, ngữ pháp Java nhận.
 *   3. Đúng một lớp cam (nguồn sáng), nằm trên sàn, trong khung.
 *   4. Câu mô tả bắt đầu «Ký hoạ {tên}»; mười lăm cảnh khác nhau; tất định.
 *
 * Không chứng minh: người xem nhận ra thành phố. Cái đó là việc của ảnh chụp.
 */
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

import { KHUNG_THANH_PHO, SAN_THANH_PHO, THANH_PHO_IDS, hinhThanhPho, moTaThanhPho, sanKhauThanhPho } from "../dist-test/rudi/art/thanh-pho.js";
import { kiemSanKhau, lopPhang } from "../dist-test/rudi/art/san-khau.js";
import { kiemLop } from "./_kiem-lop.mjs";

const NGUON = readFileSync(new URL("../../../services/api/app/places/destinations_vn.py", import.meta.url), "utf8");
const MAY_CHU = [...NGUON.matchAll(/"id":\s*"(d-[a-z0-9-]+)",\s*\n\s*"name":\s*"([^"]+)"/g)].map((m) => ({ id: m[1], ten: m[2] }));

test("bộ đọc thấy đủ điểm đến của máy chủ", () => {
  assert.ok(MAY_CHU.length >= 15, `chỉ đọc được ${MAY_CHU.length} điểm đến: bộ đọc đã hỏng`);
});

test("mọi điểm đến máy chủ liệt kê đều có ký hoạ riêng, gọi đúng tên", () => {
  for (const { id, ten } of MAY_CHU) {
    assert.ok(THANH_PHO_IDS.includes(id), `${id} (${ten}) chưa có ký hoạ`);
    assert.ok(moTaThanhPho(id).startsWith(`Ký hoạ ${ten}: `), `${id}: câu mô tả «${moTaThanhPho(id)}» không gọi tên ${ten}`);
  }
});

test("điểm đến lạ: bưu thiếp chung, mang tên máy chủ gửi", () => {
  assert.equal(sanKhauThanhPho("d-khong-co", "Côn Đảo").id, "thanh-pho:buu-thiep");
  assert.match(moTaThanhPho("d-khong-co", "Côn Đảo"), /^Ký hoạ Côn Đảo: /);
  assert.match(moTaThanhPho(null), /^Ký hoạ một thành phố: /);
});

test("mọi sân khấu: hợp lệ, trong khung, đúng một nguồn sáng trên sàn", () => {
  for (const id of [...THANH_PHO_IDS, "d-khong-co"]) {
    const san = sanKhauThanhPho(id);
    assert.deepEqual(kiemSanKhau(san), [], id);
    kiemLop(`thành phố ${id}`, lopPhang(san), KHUNG_THANH_PHO.w, KHUNG_THANH_PHO.h, 0);
    const cam = lopPhang(san).filter((l) => l.mau === "gap" && !l.net);
    assert.equal(cam.length, 1, `${id}: ${cam.length} lớp cam, phải đúng một nguồn sáng`);
    assert.ok(san.nguonSang && san.nguonSang.y < SAN_THANH_PHO && san.nguonSang.x > 0 && san.nguonSang.x < KHUNG_THANH_PHO.w, `${id}: nguồn sáng ngoài bầu trời`);
    assert.ok(san.tang.length >= 2, `${id}: một tầng thì không có chiều sâu`);
  }
});

test("mười lăm cảnh khác nhau, và vẽ hai lần ra một", () => {
  const dau = new Set(THANH_PHO_IDS.map((id) => JSON.stringify(hinhThanhPho(id))));
  assert.equal(dau.size, THANH_PHO_IDS.length, "hai thành phố dùng chung một cảnh");
  for (const id of THANH_PHO_IDS) assert.deepEqual(hinhThanhPho(id), hinhThanhPho(id));
});
