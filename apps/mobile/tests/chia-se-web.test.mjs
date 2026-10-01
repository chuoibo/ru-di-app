/* What a share tap answers on the web (QA UI-049 P1, UI-136).
 *
 * react-native-web's `Share.share` resolves `undefined` or rejects, so the
 * batch screen read `ketQua.action` off nothing, threw, and told the organiser
 * «Kiểm tra mạng» while the link went nowhere. `chiaSe` replaces it with four
 * outcomes the screen can word in place. These run the compiled module under
 * react-native-web (the test build swaps it in, as Expo's web build does), so
 * `Platform.OS` is "web" here, with the browser APIs stubbed on `navigator`.
 */
import assert from "node:assert/strict";
import test from "node:test";

import { chiaSe } from "../dist-test/rudi/web/chia-se.js";

/** Install `share` / `clipboard` on the global navigator for one test. */
function voiNavigator(t, { share, clipboard }) {
  for (const [ten, giaTri] of [["share", share], ["clipboard", clipboard]]) {
    const cu = Object.getOwnPropertyDescriptor(globalThis.navigator, ten);
    Object.defineProperty(globalThis.navigator, ten, { configurable: true, value: giaTri });
    t.after(() => {
      if (cu) Object.defineProperty(globalThis.navigator, ten, cu);
      else delete globalThis.navigator[ten];
    });
  }
}

test("Web Share mở được: «đã mở khay», và link đi riêng trong trường url", async (t) => {
  const nhan = [];
  voiNavigator(t, { share: async (d) => void nhan.push(d), clipboard: undefined });
  const ketQua = await chiaSe({ title: "Bài trên Rủ Đi", url: "https://rudi.example/community/posts/p1" });
  assert.equal(ketQua, "da-mo-khay");
  assert.deepEqual(nhan, [{ title: "Bài trên Rủ Đi", text: undefined, url: "https://rudi.example/community/posts/p1" }]);
});

test("người dùng đóng khay (AbortError): «huỷ», không chép gì, không báo lỗi", async (t) => {
  let daChep = false;
  voiNavigator(t, {
    share: async () => { throw new DOMException("Share canceled", "AbortError"); },
    clipboard: { writeText: async () => { daChep = true; } },
  });
  assert.equal(await chiaSe({ text: "Phần của Lan: 120.000đ\nhttps://x/e/1" }), "huy");
  assert.equal(daChep, false);
});

test("Web Share từ chối vì lý do khác: rơi xuống clipboard, chép đủ chữ lẫn link", async (t) => {
  const chep = [];
  voiNavigator(t, {
    share: async () => { throw new DOMException("no user gesture", "NotAllowedError"); },
    clipboard: { writeText: async (s) => void chep.push(s) },
  });
  assert.equal(await chiaSe({ text: "Bài trên Rủ Đi", url: "https://x/p/1" }), "da-chep");
  assert.deepEqual(chep, ["Bài trên Rủ Đi\nhttps://x/p/1"]);
});

test("không có Web Share: chép vào clipboard, «đã chép»", async (t) => {
  const chep = [];
  voiNavigator(t, { share: undefined, clipboard: { writeText: async (s) => void chep.push(s) } });
  assert.equal(await chiaSe({ text: "Phần của Lan: 120.000đ\nhttps://x/e/1" }), "da-chep");
  assert.deepEqual(chep, ["Phần của Lan: 120.000đ\nhttps://x/e/1"]);
});

test("không chia sẻ, không chép được: «không được», để màn hiện link cho chép tay", async (t) => {
  voiNavigator(t, { share: undefined, clipboard: { writeText: async () => { throw new Error("denied"); } } });
  assert.equal(globalThis.document, undefined, "không có document nên cách chép cũ cũng không chạy");
  assert.equal(await chiaSe({ text: "https://x/e/1" }), "khong-duoc");
});
