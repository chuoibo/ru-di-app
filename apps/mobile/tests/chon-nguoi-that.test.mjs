import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

// QA 23/09: «Rủ một người đi chơi» offered the fixture pair to a real account
// with no friends. The fixture ids may appear only inside the experience-build
// component, and the screen must branch on the session before either renders.
const src = readFileSync(new URL("../src/rudi/screens/hai-nguoi/ChonNguoi.tsx", import.meta.url), "utf8");

function than(ten) {
  const dau = src.indexOf(`function ${ten}(`);
  assert.ok(dau >= 0, `thiếu ${ten}`);
  const sau = src.indexOf("\nfunction ", dau + 1);
  const sauXuat = src.indexOf("\nexport function ", dau + 1);
  const cuoi = [sau, sauXuat].filter((i) => i > 0).sort((a, b) => a - b)[0] ?? src.length;
  return src.slice(dau, cuoi);
}

test("phiên thật đi nhánh sống, không qua hàng fixture", () => {
  assert.match(than("ChonNguoiScreen"), /phien !== null\) return <ChonNguoiSong/);
  const song = than("ChonNguoiSong");
  assert.doesNotMatch(song, /CAP_DEMO|NGUOI_KIA_DEMO/);
  assert.match(song, /docDanhSachBan/);
  assert.match(song, /moNhanRieng/);
});

test("fixture chỉ sống trong ChonNguoiTraiNghiem", () => {
  const ngoai = src.replace(than("ChonNguoiTraiNghiem"), "");
  const dungFixture = ngoai.split("\n").filter((d) => /CAP_DEMO|NGUOI_KIA_DEMO/.test(d) && !d.startsWith("import"));
  assert.deepEqual(dungFixture, []);
});
