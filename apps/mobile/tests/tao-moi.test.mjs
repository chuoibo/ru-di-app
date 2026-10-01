/* The «Tạo mới» desk puts the tab's own kind of thing first, and hides nothing.
 *
 * With the stamp on every tab, the desk is opened from five places; each one
 * moves exactly one card to the front (`?tu=`), and every action stays on the
 * desk in one stable order beneath it. A cold or unknown `tu` keeps the order
 * the desk always had.
 */
import assert from "node:assert/strict";
import test from "node:test";

import { VIEC_BAI, VIEC_BILL, VIEC_CAP, VIEC_HEN, VIEC_KY_NIEM, VIEC_STORY, tabTu, thuTuViec } from "../dist-test/rudi/tao-moi.js";

/** The desk's own list, as `screens/Create.tsx` declares it (hrefs are what the order reads). */
const VIEC_TAO = [VIEC_HEN, VIEC_BILL, VIEC_KY_NIEM, VIEC_STORY, VIEC_CAP, VIEC_BAI].map((href) => ({ href }));

const dau = (tu, coCap = false) => thuTuViec({ viec: VIEC_TAO, tu, coCap, coCongDong: true }).viec[0].href;

test("mỗi tab đặt việc của nó lên đầu", () => {
  assert.equal(dau("community"), VIEC_BAI);
  assert.equal(dau("explore"), VIEC_HEN);
  assert.equal(dau("plan"), VIEC_HEN);
  assert.equal(dau("profile"), VIEC_KY_NIEM);
  assert.equal(dau("messages"), VIEC_STORY);
  assert.equal(dau("messages", true), VIEC_CAP, "người có chat đôi: Tin nhắn mở lời hẹn một người trước");
});

test("không giấu việc nào; chỉ một thẻ đổi chỗ, phần còn lại giữ thứ tự", () => {
  for (const tu of ["community", "explore", "plan", "messages", "profile", null]) {
    const { viec } = thuTuViec({ viec: VIEC_TAO, tu, coCap: false, coCongDong: true });
    assert.equal(viec.length, VIEC_TAO.length, `${tu}: đủ ${VIEC_TAO.length} việc`);
    assert.deepEqual([...viec].map((v) => v.href).sort(), VIEC_TAO.map((v) => v.href).sort());
    const conLai = viec.slice(1).map((v) => v.href);
    const goc = VIEC_TAO.map((v) => v.href).filter((h) => h !== viec[0].href);
    assert.deepEqual(conLai, goc, `${tu}: phần dưới giữ thứ tự gốc`);
  }
});

test("mở lạnh, tab lạ: thứ tự cũ, chỉ cặp đôi được ưu tiên như trước", () => {
  assert.equal(tabTu("dau-do"), null);
  assert.equal(tabTu(undefined), null);
  assert.equal(tabTu(["plan"]), null);
  assert.equal(thuTuViec({ viec: VIEC_TAO, tu: null, coCap: false, coCongDong: true }).viec[0].href, VIEC_TAO[0].href);
  assert.equal(thuTuViec({ viec: VIEC_TAO, tu: null, coCap: true, coCongDong: true }).viec[0].href, VIEC_CAP);
  assert.equal(thuTuViec({ viec: VIEC_TAO, tu: null, coCap: false, coCongDong: true }).hopCho, false);
  assert.equal(thuTuViec({ viec: VIEC_TAO, tu: "plan", coCap: false, coCongDong: true }).hopCho, true);
});

test("Cộng đồng tắt thì «Viết bài» không có chỗ để đi", () => {
  const { viec } = thuTuViec({ viec: VIEC_TAO, tu: "community", coCap: false, coCongDong: false });
  assert.equal(viec.some((v) => v.href === VIEC_BAI), false);
  assert.equal(viec.length, VIEC_TAO.length - 1);
});
