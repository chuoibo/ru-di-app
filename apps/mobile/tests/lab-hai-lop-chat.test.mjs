/* The lab board for the two classes of a two-person chat (`app/dev/hai-lop-chat.tsx`).
 *
 * The board exists to be looked at: its web screenshots are the visual
 * evidence for the friends' pair and the couple. A picture is only evidence of
 * the product if the board draws the product, so the claims held here are the
 * ones that make it so:
 *  - it is gated exactly like the other lab pages (`CUA_FIXTURE_DEV`, a
 *    redirect before anything renders), so no store build shows it;
 *  - every surface is the shipped component imported from `src/rudi`, never a
 *    local look-alike of the same name;
 *  - the couple's tray gets «Tờ giấy» (`onToGiay`) and the friends' tray does
 *    not; the couple's sticker tray gets `capDoi` and the friends' does not;
 *  - the data is invented («Linh», «Minh»), with no long digit run a real
 *    phone or account number could hide in.
 *
 * NOT proved: what the board looks like (open the screenshots), or that the
 * chat screen passes the same props (`ai-chat-hai-nguoi` holds that).
 */
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";

const HERE = dirname(fileURLToPath(import.meta.url));
const TRANG = readFileSync(join(HERE, "..", "app", "dev", "hai-lop-chat.tsx"), "utf8");

test("the board is gated like the other lab pages, before anything renders", () => {
  assert.match(TRANG, /import \{ CUA_FIXTURE_DEV \} from "\.\.\/\.\.\/src\/rudi\/cua-fixture";/);
  const cong = TRANG.indexOf("if (!CUA_FIXTURE_DEV) return <Redirect");
  assert.ok(cong > 0, "no redirect when the fixture door is closed");
  assert.ok(cong < TRANG.indexOf("return (", cong + 1), "the redirect must come before the board's own render");
});

test("every surface is the shipped component, not a local copy", () => {
  const nguon = {
    CongCuChat: "../../src/rudi/screens/chat/SoHen",
    ChipBoiCanh: "../../src/rudi/screens/chat/ChipBoiCanh",
    KhaySticker: "../../src/rudi/screens/chat/KhaySticker",
    HangTraLoiAiDangViet: "../../src/rudi/screens/chat/TraLoiAiDangViet",
    HangLoiDeNghi: "../../src/rudi/screens/hai-nguoi/HangToGiay",
    HangToGiay: "../../src/rudi/screens/hai-nguoi/HangToGiay",
  };
  for (const [ten, duong] of Object.entries(nguon)) {
    const dong = TRANG.split("\n").find((d) => d.startsWith("import ") && d.includes(`"${duong}"`));
    assert.ok(dong && new RegExp(`\\b${ten}\\b`).test(dong), `${ten} is not imported from ${duong}`);
    assert.doesNotMatch(TRANG, new RegExp(`(function|const)\\s+${ten}\\b`), `${ten} is redefined on the board`);
    assert.match(TRANG, new RegExp(`<${ten}\\b`), `${ten} is imported but never drawn`);
  }
});

test("only the couple's tray carries «Tờ giấy», only the couple's stickers carry the four for two", () => {
  assert.match(TRANG, /onToGiay=\{lop === "doi" \? \(\) => \{\} : undefined\}/);
  assert.match(TRANG, /<KhaySticker capDoi=\{sticker === "doi"\}/);
  // Both trays are a two-person chat: the words are always the pair's.
  assert.match(TRANG, /<CongCuChat[\s\S]*?\bhaiNguoi\b[\s\S]*?\/>/);
  for (const chip of TRANG.match(/<ChipBoiCanh[^>]*\/>/g) ?? []) assert.match(chip, /\bhaiNguoi\b/, chip);
});

test("the invitation row is drawn for both proposals a friends' pair can receive", () => {
  assert.match(TRANG, /<HangLoiDeNghi deNghi=\{\{ purpose: "lap_so", ten: "Minh" \}\}/);
  assert.match(TRANG, /<HangLoiDeNghi deNghi=\{\{ purpose: "bat_doi", ten: "Minh" \}\}/);
});

test("the data is invented: two fake names and no long digit run", () => {
  assert.match(TRANG, /"Linh"|Linh/);
  assert.match(TRANG, /"Minh"/);
  assert.doesNotMatch(TRANG, /\d{10,}/);
});
