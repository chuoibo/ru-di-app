import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const doc = (p) => readFileSync(new URL(`../src/rudi/${p}`, import.meta.url), "utf8");

// QA 23/09: the receiver of «Một đôi» could only press «Một đôi», which filed a
// SECOND proposal; the rung then lit on both phones while the server had
// completed nothing. Choosing it must open the rung's sheet, and a proposal
// from the other person must be answered by id.
test("chọn «Một đôi» mở sheet bậc, không đề nghị ngay", () => {
  const kgg = doc("screens/hai-nguoi/KhongGianGiay.tsx");
  const loaiSo = kgg.slice(kgg.indexOf("<LoaiSo "), kgg.indexOf("/>", kgg.indexOf("<LoaiSo ")));
  assert.doesNotMatch(loaiSo, /onChonDoi=\{so\.deNghiBatDoi\}/);
  assert.match(loaiSo, /setMo\("bat-doi"\)/);
  assert.match(loaiSo, /onDongY=\{deNghiBatDoi && !deNghiBatDoi\.cuaToi \? \(\) => void so\.dongYDeNghi\(deNghiBatDoi\.id\)/);
});

test("người nhận thấy nút đồng ý, người đề nghị thấy câu chờ", () => {
  const ls = doc("screens/hai-nguoi/LoaiSo.tsx");
  assert.match(ls, /dangCho && !batDoi && !deNghiCuaToi && onDongY \? <RudiButton label="Đồng ý là một đôi"/);
});

// A sheet must close on the notebook's answer, never on the press.
test("sheet ràng buộc và đồng ý đóng theo kết quả", () => {
  const kgg = doc("screens/hai-nguoi/KhongGianGiay.tsx");
  assert.match(kgg, /onLuu=\{\(rb\) => void so\.datRangBuoc\(rb\)\.then\(\(ok\) => ok && dong\(\)\)\}/);
  assert.doesNotMatch(kgg, /so\.datRangBuoc\(rb\); dong\(\)/);
});

test("hàng ghim báo lời đề nghị của người kia", () => {
  const song = doc("screens/hai-nguoi/HangToGiaySong.tsx");
  assert.match(song, /pending_proposals\.find\(\(d\) => d\.proposed_by_id !== toiId\)/);
});
