/* Production (2026-10-03): không còn bản trải nghiệm. Mở một route khi chưa
 * đăng nhập (link sâu, thông báo, mở lạnh) thì đi qua cửa đăng nhập, không bao
 * giờ rơi về một màn dữ liệu mẫu.
 *
 * Chạy từ apps/mobile:
 *     node --test tests/route-khong-phien-cua-dang-nhap.test.mjs
 *
 * Phép ghim nguồn: chứng minh mỗi route đợi đọc xong phiên rồi mới quyết, có
 * nhánh «không phiên» tới cửa đăng nhập, và không import màn fixture nào (các
 * file đó đã xoá). Không chứng minh màn đích đúng -- đó là việc của ảnh chụp
 * trên stack thật.
 */
import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import test from "node:test";

const ROUTE = [
  "app/(tabs)/explore.tsx",
  "app/(tabs)/plan.tsx",
  "app/(tabs)/messages.tsx",
  "app/ai-match.tsx",
  "app/votes/[id]/index.tsx",
  "app/check-ins/new.tsx",
  "app/trips/[id]/itinerary.tsx",
  "app/trips/[id]/timeline.tsx",
  "app/trips/[id]/album.tsx",
  "app/groups/[id]/chat.tsx",
  "app/groups/[id]/wall.tsx",
  "app/groups/[id]/to-giay.tsx",
  "app/places/[id]/index.tsx",
  "app/outings/new.tsx",
  "app/moments/new.tsx",
  "app/smart-split/[id]/review.tsx",
  "app/smart-split/[id]/assignment.tsx",
  "app/batches/[id]/index.tsx",
  "app/achievements.tsx",
];

const MAN_DA_XOA = ["screens/Group", "screens/Discovery", "screens/Outing", "screens/Memories"];

for (const tep of ROUTE) {
  test(`${tep}: chưa đăng nhập thì tới cửa đăng nhập, không có màn mẫu`, () => {
    const nguon = readFileSync(new URL(`../${tep}`, import.meta.url), "utf8");
    const doc = nguon.indexOf("if (!phienDaDoc) return null;");
    const cua = nguon.indexOf("<CuaDangNhap");
    assert.ok(doc >= 0, "không đợi đọc xong phiên");
    assert.ok(cua > doc, "không có nhánh cửa đăng nhập sau khi đọc phiên");
    for (const man of MAN_DA_XOA) assert.ok(!nguon.includes(man), `còn import ${man}`);
  });
}

test("các màn fixture đã xoá hẳn khỏi cây", () => {
  for (const man of MAN_DA_XOA) {
    assert.equal(existsSync(new URL(`../src/rudi/${man}.tsx`, import.meta.url)), false, man);
  }
  for (const tep of ["src/rudi/fixtures.ts", "src/rudi/cua-fixture.ts", "src/rudi/to-giay/fixtures-doi.ts"]) {
    assert.equal(existsSync(new URL(`../${tep}`, import.meta.url)), false, tep);
  }
});

test("Cá nhân, Tài chính, Quyết toán: chưa đăng nhập thì cửa đăng nhập", () => {
  const profile = readFileSync(new URL("../src/rudi/screens/Profile.tsx", import.meta.url), "utf8");
  assert.match(profile, /if \(phien === null\) return <CuaDangNhap tiep="\/profile" \/>;/);
  assert.match(profile, /return <CuaDangNhap tiep="\/finance" \/>;/);
  const bill = readFileSync(new URL("../src/rudi/screens/Bill.tsx", import.meta.url), "utf8");
  assert.match(bill, /if \(session\.phien === null\) return <CuaDangNhap \/>;/);
});
