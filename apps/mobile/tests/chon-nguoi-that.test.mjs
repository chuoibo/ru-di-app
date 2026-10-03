import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

// QA 23/09: «Rủ một người đi chơi» offered the fixture pair to a real account
// with no friends. There is no fixture pair any more (2026-10-03): a session
// gets the friend list, no session gets the sign-in door.
const src = readFileSync(new URL("../src/rudi/screens/hai-nguoi/ChonNguoi.tsx", import.meta.url), "utf8");

function than(ten) {
  const dau = src.indexOf(`function ${ten}(`);
  assert.ok(dau >= 0, `thiếu ${ten}`);
  const sau = src.indexOf("\nfunction ", dau + 1);
  const sauXuat = src.indexOf("\nexport function ", dau + 1);
  const cuoi = [sau, sauXuat].filter((i) => i > 0).sort((a, b) => a - b)[0] ?? src.length;
  return src.slice(dau, cuoi);
}

test("phiên thật đi nhánh sống, không phiên thì cửa đăng nhập", () => {
  assert.match(than("ChonNguoiScreen"), /phien === null\) return <CuaDangNhap[^\n]*\n\s*return <ChonNguoiSong/);
  const song = than("ChonNguoiSong");
  assert.doesNotMatch(song, /CAP_DEMO|NGUOI_KIA_DEMO/);
  assert.match(song, /docDanhSachBan/);
  assert.match(song, /moNhanRieng/);
});

test("không còn cặp mẫu nào trong màn chọn người", () => {
  assert.doesNotMatch(src, /CAP_DEMO|NGUOI_KIA_DEMO|TraiNghiem|fixtures-doi/);
});

// QA 23/09: the conversation list read a direct conversation as «Mở nhóm Linh».
test("hàng nhắn riêng đọc là cuộc trò chuyện, không phải nhóm", () => {
  const ds = readFileSync(new URL("../src/rudi/screens/groups/Conversations.tsx", import.meta.url), "utf8");
  assert.match(ds, /laPair\(nhom\) \? `Mở cuộc trò chuyện với \$\{tenCuocTroChuyen\(nhom\)\}`/);
});
