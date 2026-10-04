/**
 * A button that destroys something somebody made or is counting on wears the
 * `warn` tone (B11 consistency, plan «Pattern lệch xuyên app»): «Xoá cuốn sổ»
 * was a solid coral button, «Bỏ bản phác» looked like «Tuần này nghỉ», and
 * «Rời nhóm», «Chặn», «Xoá tin» each drew as an ordinary choice.
 *
 * Swept from the source, every `RudiButton` whose label starts with a
 * destroying verb. The ones that only let go of something not yet kept (a
 * picked photo, a place in a form, a hint) are named here with why. Does not
 * prove: a button drawn another way (a Pressable) or a label built at runtime.
 */
import assert from "node:assert/strict";
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const goc = fileURLToPath(new URL("..", import.meta.url));
const DONG_TU = /^(Xoá|Xóa|Rời nhóm|Chặn|Bỏ bản|Bỏ nháp|Bỏ tờ|Bỏ thứ tự|Huỷ buổi|Thu hồi)/;

/** Labels that match the verbs but take nothing that was kept, file by file. */
const KHONG_PHA = {
  // «Bỏ trang này» moves a page out of a notebook still being put together; the page can be added back.
  "src/rudi/diary/EndingScreen.tsx": ["Bỏ trang này"],
};

function tsx(thuMuc, ra = []) {
  for (const ten of readdirSync(thuMuc)) {
    const p = join(thuMuc, ten);
    if (statSync(p).isDirectory()) tsx(p, ra);
    else if (p.endsWith(".tsx")) ra.push(p);
  }
  return ra;
}

test("nút phá huỷ mang tông warn", () => {
  const thieu = [];
  let dem = 0;
  for (const p of tsx(join(goc, "src/rudi"))) {
    const rel = relative(goc, p);
    const src = readFileSync(p, "utf8");
    for (const m of src.matchAll(/<RudiButton\b(?:(?!<RudiButton\b)[\s\S])*?\/>/g)) {
      const nhan = /\blabel="([^"]+)"/.exec(m[0])?.[1];
      if (nhan === undefined) continue;
      if (!DONG_TU.test(nhan) || (KHONG_PHA[rel] ?? []).includes(nhan)) continue;
      dem += 1;
      if (!/\btone="warn"/.test(m[0])) thieu.push(`${rel}: «${nhan}»`);
    }
  }
  assert.ok(dem >= 10, `quét được ${dem} nút; ít hơn thế là phép quét đã hỏng`);
  assert.deepEqual(thieu, [], "nút phá huỷ thiếu tone=\"warn\"");
});
