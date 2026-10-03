/**
 * Every `EmptyState` the app mounts carries a scene of its own world, unless
 * it is one of the silences named here, each with its reason. DESIGN.md
 * («Trạng thái rỗng, tải, lỗi») kept this list in prose and asked for it to
 * be checked by sweeping `illustration=` over every mount; by 03/10 the prose
 * said three and the code had eleven. The list now lives here, where a new
 * mount without art turns this red until someone names why -- or names it a
 * debt (`CHUA_VE`) rather than a decision.
 *
 * Read from the source (the screens are not rendered in node). Does not
 * prove: that a scene suits its silence (the screenshots and the reviews).
 */
import assert from "node:assert/strict";
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const goc = fileURLToPath(new URL("..", import.meta.url));

/** Silences left without art on purpose, by file and title as written in the source. */
const KHONG_HINH = {
  // Administrative lists: managing settings, nobody needs a story told.
  "src/rudi/screens/cai-dat/DaChanScreen.tsx": ['"Bạn chưa chặn ai"'],
  "src/rudi/screens/cai-dat/PhienScreen.tsx": ['"Chưa có phiên nào"'],
  "src/rudi/community/CommunityScreen.tsx": ['"Chưa ẩn bài nào"'],
  // The demo's note about itself («chưa có hộp thư máy chủ»), not a user's silence.
  "src/rudi/screens/Discovery.tsx": ['"Thông báo"'],
  // Money: «Nếp đứng xa tiền», and a ledger screen takes no story.
  "src/rudi/screens/Bill.tsx": ['"Chưa có sổ nào để quyết toán"'],
  // The comments under a wall post: a line under the post it answers, where a
  // scene would outweigh the post itself.
  "src/rudi/screens/tuong/BaiChiTietScreen.tsx": ['"Chưa có lời nhắn nào"'],
  // A private list of notes reached from the feed's settings sheet.
  "src/rudi/community/Keeps.tsx": ['"Chưa có ghi chép nào"'],
};

/**
 * Silences with no reason to stay bare: not drawn yet. Named so the sweep
 * stays green while they wait, and kept apart from `KHONG_HINH` so nobody
 * reads a debt as a decision. Each is a whole screen's content when empty.
 */
const CHUA_VE = {
  "src/rudi/screens/hai-nguoi/ChonNguoi.tsx": ['"Chưa có bạn nào để rủ"'],
  "src/rudi/screens/keo/PickOutingLive.tsx": ['"Chưa có địa điểm để thêm"'],
};

function* tepTsx(thuMuc) {
  for (const ten of readdirSync(thuMuc)) {
    const p = join(thuMuc, ten);
    if (statSync(p).isDirectory()) yield* tepTsx(p);
    else if (p.endsWith(".tsx")) yield p;
  }
}

/** Each `<EmptyState …/>` element of a file: its title as written, and whether it has art. */
function cacORong(nguon) {
  const ra = [];
  for (const m of nguon.matchAll(/<EmptyState\b/g)) {
    let i = m.index + m[0].length;
    let sau = 0;
    while (i < nguon.length && !(sau === 0 && nguon.startsWith("/>", i))) {
      if (nguon[i] === "{") sau += 1;
      else if (nguon[i] === "}") sau -= 1;
      i += 1;
    }
    const the = nguon.slice(m.index, i + 2);
    ra.push({ tieuDe: the.match(/title=("[^"]*"|\{[^}]*\})/)?.[1] ?? "?", coHinh: /\billustration=/.test(the) });
  }
  return ra;
}

test("mọi EmptyState có cảnh, trừ các ô rỗng được đặt tên kèm lý do", () => {
  const thieu = [];
  const daGap = new Set();
  for (const thuMuc of ["src", "app"]) {
    for (const p of tepTsx(join(goc, thuMuc))) {
      const tep = relative(goc, p);
      // The component itself, and the lab page that shows it.
      if (tep === "src/rudi/ui/EmptyState.tsx" || tep.startsWith("app/dev/")) continue;
      for (const o of cacORong(readFileSync(p, "utf8"))) {
        if (o.coHinh) continue;
        if (KHONG_HINH[tep]?.includes(o.tieuDe) || CHUA_VE[tep]?.includes(o.tieuDe)) daGap.add(`${tep} ${o.tieuDe}`);
        else thieu.push(`${tep} ${o.tieuDe}`);
      }
    }
  }
  assert.deepEqual(thieu, [], "EmptyState không có cảnh và không có tên trong KHONG_HINH hay CHUA_VE");
  // A name whose silence has gone (or got its scene) leaves the list too.
  const thua = [...Object.entries(KHONG_HINH), ...Object.entries(CHUA_VE)].flatMap(([tep, ds]) => ds.map((t) => `${tep} ${t}`)).filter((x) => !daGap.has(x));
  assert.deepEqual(thua, [], "KHONG_HINH/CHUA_VE còn tên không ứng với ô rỗng nào không có cảnh");
});
