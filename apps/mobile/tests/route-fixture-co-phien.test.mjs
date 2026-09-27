/* B5 (QC 24/09): route của bản trải nghiệm không hiện dữ liệu demo trên phiên thật.
 *
 * Chạy từ apps/mobile:
 *     node --test tests/route-fixture-co-phien.test.mjs
 *
 * Mở thẳng `/votes/{id}`, `/check-ins/new`, `/trips/{id}/itinerary`, `/ai-match`
 * khi đã đăng nhập (link sâu, thông báo) từng hiện «BBQ tối thứ Bảy ở đâu?», «Lan
 * Anh, Minh Khoa…», «Không phải kết quả LLM». Mỗi route nay đợi đọc xong phiên,
 * có phiên thì chuyển sang màn live làm cùng việc, không phiên thì vẫn là bản
 * trải nghiệm.
 *
 * Phép ghim nguồn: chứng minh nhánh có mặt và đứng TRƯỚC màn fixture. Không
 * chứng minh màn đích đúng -- đó là việc của ảnh chụp trên stack thật.
 */
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const ROUTE = {
  "app/ai-match.tsx": { fixture: "AiMatchScreen", den: '"/explore"' },
  "app/votes/[id]/index.tsx": { fixture: "VotingScreen", den: '"/messages"' },
  "app/check-ins/new.tsx": { fixture: "CheckInScreen", den: '"/(tabs)/plan"' },
  "app/trips/[id]/itinerary.tsx": { fixture: "AiItineraryScreen", den: "`/outings/${id}`" },
};

for (const [tep, { fixture, den }] of Object.entries(ROUTE)) {
  test(`${tep}: có phiên thì chuyển sang màn live, không phiên thì bản trải nghiệm`, () => {
    const nguon = readFileSync(new URL(`../${tep}`, import.meta.url), "utf8");
    assert.ok(!/export\s*\{[^}]*as default/.test(nguon), "route vẫn xuất thẳng màn fixture");
    const doc = nguon.indexOf("if (!phienDaDoc) return null;");
    const chuyen = nguon.indexOf("if (phien !== null) return <Redirect");
    const man = nguon.indexOf(`return <${fixture} />`);
    assert.ok(doc >= 0, "không đợi đọc xong phiên: khung đầu vẫn là fixture");
    assert.ok(chuyen > doc, "không chuyển hướng khi có phiên");
    assert.ok(man > chuyen, "màn fixture đứng trước nhánh có phiên");
    assert.ok(nguon.slice(chuyen, man).includes(den), `chuyển sai chỗ, phải về ${den}`);
  });
}
