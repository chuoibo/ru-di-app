/* «Ký hoạ trong sổ» (review 11/09, A1): the sketch a place gets when it has no
 * photo. Mechanism only -- these tests cannot say whether a human reads an
 * awning; a blind read by a fresh reviewer and the team's unlabeled test do.
 *
 * Run from apps/mobile:
 *     npx tsc -p tsconfig.test.json && node tools/fixup-esm.mjs && node --test tests/art-ky-hoa.test.mjs
 */
import assert from "node:assert/strict";
import test from "node:test";

import { KHUNG_KY_HOA, SAN_KHAU_IDS, daoCuTheoTag, hinhKyHoa, moTaKyHoa, sanKhauTheoLoai } from "../dist-test/rudi/art/ky-hoa.js";
import { readFileSync } from "node:fs";
import { kiemLop } from "./_kiem-lop.mjs";

const { w, h } = KHUNG_KY_HOA;
// fixtures.ts pulls expo-image and the theme, so it is read as source: every
// place's category and tags, exactly as the Explore fixture hands them over.
const NGUON = readFileSync(new URL("../src/rudi/fixtures.ts", import.meta.url), "utf8");
const KHOI_LOAI = NGUON.slice(NGUON.indexOf("export const LOAI_MAU"), NGUON.indexOf("};", NGUON.indexOf("export const LOAI_MAU")));
const LOAI_MAU = Object.fromEntries([...KHOI_LOAI.matchAll(/^\s*"?([^"\n:]+?)"?: "([a-z-]+)",?$/gm)].map((m) => [m[1], m[2]]));
assert.equal(Object.keys(LOAI_MAU).length, 4, "LOAI_MAU phải có bốn loại (đọc nguồn)");
const PLACES = [...NGUON.matchAll(/name: "([^"]+)",[\s\S]*?tags: \[([^\]]*)\],\s*category: "([^"]+)"/g)].map((m) => ({
  name: m[1],
  tags: [...m[2].matchAll(/"([^"]+)"/g)].map((t) => t[1]),
  category: m[3],
}));
assert.equal(PLACES.length, 12, "fixture phải có đúng 12 nơi (đọc nguồn)");
const LOAI = ["quan-an-local", "cafe", "vui-choi", "di-choi-dem", "khac"];
const BO = [
  ...PLACES.map((p) => ({ ten: p.name, loai: LOAI_MAU[p.category], tags: p.tags })),
  ...LOAI.map((loai) => ({ ten: `chỉ loại ${loai}`, loai, tags: [] })),
];

test("mọi bộ (loại, tag) của fixture và mỗi loại trần: lớp hợp lệ trong hộp 288×96, cả hai khung đọc", () => {
  for (const b of BO) {
    for (const gon of [false, true]) {
      kiemLop(`${b.ten}${gon ? " (gọn)" : ""}`, hinhKyHoa(b.loai, b.tags, { gon }), w, h);
    }
  }
});

test("đúng MỘT lớp coral -- nguồn sáng -- và ít nhất sáu nét mực ở bản đủ", () => {
  for (const b of BO) {
    const lop = hinhKyHoa(b.loai, b.tags);
    assert.equal(lop.filter((l) => l.mau === "gap").length, 1, b.ten);
    assert.ok(lop.filter((l) => l.mau === "muc" && l.net).length >= (sanKhauTheoLoai(b.loai) === "to-giay" ? 2 : 6), b.ten);
  }
});

test("bản gọn ít lớp hơn bản đủ và cùng sân khấu; hàm xác định", () => {
  for (const b of BO) {
    const day = hinhKyHoa(b.loai, b.tags);
    const gon = hinhKyHoa(b.loai, b.tags, { gon: true });
    assert.ok(gon.length < day.length || sanKhauTheoLoai(b.loai) === "to-giay", `${b.ten}: gọn ${gon.length} ≥ đủ ${day.length}`);
    assert.deepEqual(hinhKyHoa(b.loai, b.tags), day, b.ten);
  }
});

test("mười hai nơi fixture cho ra ít nhất tám ký hoạ khác nhau (tag có thật quyết định)", () => {
  const khac = new Set(PLACES.map((p) => JSON.stringify(hinhKyHoa(LOAI_MAU[p.category], p.tags))));
  assert.ok(khac.size >= 8, `chỉ ${khac.size} bộ lớp khác nhau`);
});

test("đạo cụ: tối đa hai, so cả từ bỏ dấu, không lặp; loại lạ về tờ giấy", () => {
  assert.deepEqual(daoCuTheoTag(["View đẹp", "Chill", "Nhóm đông"]), ["doi-sau", "chill"]);
  assert.deepEqual(daoCuTheoTag(["view dep"]), ["doi-sau"]);
  assert.deepEqual(daoCuTheoTag(["Săn mây", "Ngoài trời"]), ["may"]);
  assert.deepEqual(daoCuTheoTag(["Karaoke"]), []);
  // a stage only takes the props it draws, and the a11y sentence never names a prop that is not there
  assert.deepEqual(daoCuTheoTag(["Món local", "Đi đêm", "Nhộn nhịp"], "pho-dem"), ["them-den"]);
  assert.deepEqual(daoCuTheoTag(["Món local", "Nhóm đông"], "hien-quan"), ["noi", "nhom-dong"]);
  assert.ok(!moTaKyHoa("di-choi-dem", ["Món local", "Đi đêm"]).includes("nồi"));
  // the compact reading drops the props it does not draw, and the sentence follows
  assert.deepEqual(daoCuTheoTag(["View đẹp", "Chill"], "hien-quan", true), ["doi-sau"]);
  // the compact reading never refills the slot a dropped prop leaves: gọn ⊂ đủ
  assert.deepEqual(daoCuTheoTag(["View đẹp", "Chill", "Nhóm đông"], "hien-quan"), ["doi-sau", "chill"]);
  assert.deepEqual(daoCuTheoTag(["View đẹp", "Chill", "Nhóm đông"], "hien-quan", true), ["doi-sau"]);
  assert.ok(moTaKyHoa("quan-an-local", ["Chill"]).includes("cây treo"));
  assert.ok(!moTaKyHoa("quan-an-local", ["Chill"], { gon: true }).includes("cây treo"));
  assert.ok(moTaKyHoa("cafe", ["Nhẹ nhàng"]).includes("rèm"));
  assert.ok(!moTaKyHoa("quan-an-local", ["Chill"]).includes("rèm"));
  assert.equal(sanKhauTheoLoai("gi-do"), "to-giay");
  assert.equal(sanKhauTheoLoai(undefined), "to-giay");
  assert.deepEqual([...SAN_KHAU_IDS].sort(), ["cua-kinh", "doi", "hien-quan", "pho-dem", "to-giay"]);
});

test("câu a11y nói đúng cái đã vẽ: bỏ một tag làm câu đổi ⇔ làm hình đổi, ở cả hai khung đọc", () => {
  for (const b of BO) {
    for (const gon of [false, true]) {
      const cau = moTaKyHoa(b.loai, b.tags, { gon });
      const hinh = JSON.stringify(hinhKyHoa(b.loai, b.tags, { gon }));
      for (let i = 0; i < b.tags.length; i++) {
        const bot = b.tags.filter((_, k) => k !== i);
        const cauDoi = moTaKyHoa(b.loai, bot, { gon }) !== cau;
        const hinhDoi = JSON.stringify(hinhKyHoa(b.loai, bot, { gon })) !== hinh;
        assert.equal(cauDoi, hinhDoi, `${b.ten}${gon ? " (gọn)" : ""}: bỏ «${b.tags[i]}» — câu ${cauDoi ? "đổi" : "giữ"} nhưng hình ${hinhDoi ? "đổi" : "giữ"}`);
      }
    }
  }
});

test("câu a11y: 8–80 ký tự, không gạch dài, nói «ký hoạ», không tên nơi", () => {
  for (const b of BO) {
    for (const s of [moTaKyHoa(b.loai, b.tags), moTaKyHoa(b.loai, b.tags, { gon: true })]) {
      assert.match(s, /^[^—]{8,80}$/, s);
      assert.match(s, /^Ký hoạ /, s);
      assert.ok(!s.includes(b.ten), `${s} nhắc tên ${b.ten}`);
    }
  }
});
