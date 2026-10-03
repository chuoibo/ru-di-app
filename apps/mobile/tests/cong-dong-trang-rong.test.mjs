/**
 * Khám phá › Cộng đồng's empty lists after the redraw of 03/10: every one is
 * the shared `EmptyState`, the four a reader meets in the feed each carry a
 * scene of the app's own world, and the hidden posts -- an administrative
 * list reached from the settings sheet -- stay without one, as DESIGN.md asks
 * of administrative silences («Bạn chưa chặn ai», «Chưa có phiên nào»).
 * Before, all five were an Ionicons glyph over an `h1`, the last empty states
 * of the app outside `EmptyState`.
 *
 * Read from the source: the screen needs the router, the session and the
 * stream, and is not rendered here. Does not prove: that the scenes read as
 * meant (the screenshots and the finish review), or the restore order on a
 * device (that is `traVeCho`, tested below on its own).
 */
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

import { traVeCho } from "../dist-test/rudi/community/bang-tin.js";

const man = readFileSync(new URL("../src/rudi/community/CommunityScreen.tsx", import.meta.url), "utf8");
// The list's empty slot, from its prop to the footer after it.
const rong = man.slice(man.indexOf("ListEmptyComponent="), man.indexOf("ListFooterComponent="));

/** The `<EmptyState …/>` whose title is `tieuDe`, as written in the source. */
const oRong = (tieuDe) => {
  const i = rong.indexOf(tieuDe);
  assert.ok(i >= 0, `không thấy trạng thái rỗng «${tieuDe}»`);
  const dau = rong.lastIndexOf("<EmptyState", i);
  assert.ok(dau >= 0, `«${tieuDe}» không nằm trong một EmptyState`);
  return rong.slice(dau, rong.indexOf("/>", rong.indexOf("title=", dau)) + 2);
};

test("năm danh sách rỗng đều là EmptyState, không còn icon trên chữ h1", () => {
  assert.ok(rong.length > 0, "không thấy ListEmptyComponent");
  assert.doesNotMatch(rong, /<Ionicons\b/);
  assert.doesNotMatch(rong, /typography\.h1/);
  assert.equal((rong.match(/<EmptyState\b/g) ?? []).length, 5);
});

test("mỗi danh sách người đọc gặp trong bảng tin có cảnh của riêng nó", () => {
  const canh = (tieuDe) => oRong(tieuDe).match(/<Canh id="([^"]+)"/)?.[1] ?? null;
  assert.equal(canh("Chưa lưu bài nào"), "chua-luu-bai");
  assert.equal(canh("Chưa kể chuyện nào"), "chua-co-bai");
  assert.equal(canh("Chưa theo dõi ai"), "chua-co-ban");
  assert.equal(canh("Một ngày đáng kể"), "chua-co-ky-niem");
});

test("«Bài đã ẩn» là danh sách quản trị: rỗng thì không có cảnh", () => {
  assert.doesNotMatch(oRong("Chưa ẩn bài nào"), /illustration=/);
});

test("«Đang theo dõi» rỗng thì chỉ chỗ tìm người để theo dõi, không mời viết bài", () => {
  const o = oRong("Chưa theo dõi ai");
  assert.doesNotMatch(o, /Kể khoảnh khắc đầu tiên|\/community\/new/);
  assert.match(o, /label: "Xem bài thịnh hành", onPress: \(\) => setMode\("trending"\)/);
  // «Đã lưu» fills the same way: saved posts are found in the feeds (finish review 03/10).
  assert.match(oRong("Chưa lưu bài nào"), /label: "Xem bài thịnh hành", onPress: \(\) => setMode\("trending"\)/);
  // The feeds' empty state names the composer as the tray and «Bài của tôi»
  // do, and a short label stays clear of the stamp's column at 320.
  assert.match(oRong("Một ngày đáng kể"), /label: "Viết bài", onPress: \(\) => router\.push\("\/community\/new" as never\)/);
  // «Bài của tôi» still opens the composer.
  assert.match(oRong("Chưa kể chuyện nào"), /label: "Viết bài", onPress: \(\) => router\.push\("\/community\/new" as never\)/);
});

test("mỗi câu thân là một câu: cảnh đã nói vế đầu (DESIGN.md, EmptyState)", () => {
  for (const tieuDe of ["Chưa lưu bài nào", "Chưa kể chuyện nào", "Chưa theo dõi ai", "Một ngày đáng kể", "Chưa ẩn bài nào"]) {
    const than = oRong(tieuDe).match(/body="([^"]+)"/)?.[1];
    assert.ok(than, `«${tieuDe}» không có câu thân`);
    assert.equal((than.match(/[.!?](?=\s|$)/g) ?? []).length, 1, `«${tieuDe}»: «${than}» không phải một câu`);
  }
});

// Deferred minor of 03/10: un-saving on «Đã lưu» takes the card off the list
// at once; when the request failed the card came back at the top, wherever it
// had been.
test("bỏ lưu lỗi thì bài về đúng chỗ cũ trong «Đã lưu»", () => {
  const a = { id: "a" }, b = { id: "b" }, c = { id: "c" };
  assert.deepEqual(traVeCho([a, c], b, 1).map((p) => p.id), ["a", "b", "c"]);
  assert.deepEqual(traVeCho([b, c], a, 0).map((p) => p.id), ["a", "b", "c"]);
  assert.deepEqual(traVeCho([a, b], c, 2).map((p) => p.id), ["a", "b", "c"]);
  // The list moved meanwhile (a reload): past its end goes last, never a hole.
  assert.deepEqual(traVeCho([a], c, 5).map((p) => p.id), ["a", "c"]);
  // Already back (a reload brought it): left as it is.
  const coRoi = [a, b, c];
  assert.equal(traVeCho(coRoi, b, 0), coRoi);
  // Not found when it was taken off: the top, as before.
  assert.deepEqual(traVeCho([a, c], b, -1).map((p) => p.id), ["b", "a", "c"]);
  assert.match(man.slice(man.indexOf("const luu = async")), /traVeCho\(items, p, viTri\)/);
});

// Android 03/10: the one action ran edge to edge on a device and as a compact
// pill on the web, because `full` (width 100%) resolves against different
// parents. Both buttons of `EmptyState` are sized to their label.
test("nút của EmptyState gọn theo chữ trên mọi nền tảng, không giãn hết cột", () => {
  const es = readFileSync(new URL("../src/rudi/ui/EmptyState.tsx", import.meta.url), "utf8");
  const nut = [...es.matchAll(/<RudiButton\b[^>]*?\/>/gs)].map((m) => m[0]);
  assert.equal(nut.length, 2);
  for (const n of nut) assert.match(n, /full=\{false\}/, n);
});

// Finish review 03/10, three fixes of one batch.
test("danh sách rỗng: không tiêu đề chồng tiêu đề, không lời xin thứ hai, nút không lọt dưới thanh tab", () => {
  // The list's own title only once there is a list to name.
  // Not over the skeleton either, or it swaps for «Chưa ẩn bài nào» when the list comes back empty.
  assert.match(man, /\{tieuDeRieng && posts\.length > 0 \? <Text/);
  // The personalisation ask waits for posts: over an empty feed it was a
  // second filled coral button beside the scene's.
  assert.match(man, /mode === "for_you" && posts\.length > 0 \? <View style=\{\[styles\.consent/);
  // One scene size for the five tabs, 120 only where 168 does not fit: a
  // short window that is also cramped (320×640: the action ended under the
  // strip at 168 and at 144), never a whole class of phones by height alone.
  assert.match(man, /const rongCanh = caoCuaSo < 700 && \(hep \|\| fontScale > 1\.15\) \? 120 : 168;/);
  const canh = [...rong.matchAll(/<Canh id="[^"]+" width=\{([^}]+)\} \/>/g)].map((m) => m[1]);
  assert.deepEqual(canh, ["rongCanh", "rongCanh", "rongCanh", "rongCanh"]);
  assert.match(man, /empty: \{ marginHorizontal: 16, paddingTop: 0 \}/);
});

// Finish review 03/10 (R2): the community search runs as one types, so its
// «found nothing» shows with the keyboard up; the scene must not push the
// advice under it.
test("tìm trong Cộng đồng không thấy: cảnh tránh chỗ khi bàn phím đang mở", () => {
  const tim = readFileSync(new URL("../src/rudi/community/Search.tsx", import.meta.url), "utf8");
  assert.match(tim, /const banPhim = useKeyboardOpen\(\);/);
  assert.match(tim, /illustration=\{banPhim \? undefined : <Canh id="tim-khong-ra" width=\{168\} \/>\}/);
});
