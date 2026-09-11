/**
 * Một lý do cho một địa điểm (tái audit 10/09, R3): lý do là MỘT tag, và không
 * phải tag mà dòng mô tả đã nói; hình gu theo tag có thật trước, theo danh mục sau.
 */
import assert from "node:assert/strict";
import test from "node:test";

import { chonLyDo, guTheoTag } from "../dist-test/rudi/kham-pha/ly-do.js";

test("chonLyDo bỏ tag mà mô tả đã nói, lấy tag đầu chưa nói", () => {
  // Đúng ca của Codex: «Chill» và «View đẹp» đều nằm trong «Nướng thơm lừng, view đồi cực chill».
  assert.equal(chonLyDo(["Chill", "View đẹp", "Nhóm đông"], "Nướng thơm lừng, view đồi cực chill"), "Nhóm đông");
  assert.equal(chonLyDo(["View đẹp", "Chill"], "Nướng than thơm cả con dốc, hiên rộng"), "View đẹp");
  // Không dấu vẫn nhận: «view» trong «View đồi».
  assert.equal(chonLyDo(["View đẹp", "Lẩu"], "View đồi rất rộng"), "Lẩu");
});

test("chonLyDo so cả từ, không so chuỗi con: «Săn mây» không bị «sáng» nuốt", () => {
  // Reviewer 11/09: bản so chuỗi con loại «Săn mây» vì «san» ⊂ «sang».
  assert.equal(chonLyDo(["Săn mây", "Ngoài trời"], "Lên đồi từ 5 giờ sáng, mang áo ấm"), "Săn mây");
  assert.equal(chonLyDo(["Trà", "Ngoài trời"], "Trà thảo mộc, bàn dài"), "Ngoài trời");
});

test("chonLyDo: mọi tag đều đã nói thì giữ tag đầu; không tag thì không lý do", () => {
  assert.equal(chonLyDo(["Chill"], "Quán rất chill"), "Chill");
  assert.equal(chonLyDo([], "gì cũng được"), undefined);
  assert.equal(chonLyDo(["Lẩu"], undefined), "Lẩu");
});

test("guTheoTag: tag có thật chọn hình; không tag nào thì null để danh mục quyết", () => {
  assert.equal(guTheoTag(["Món local", "Bình dân"]), "mon-local");
  assert.equal(guTheoTag(["Săn mây", "Ngoài trời"]), "outdoor");
  // Tag đứng trước thắng: «Trà» nói cà phê/trà trước «Ngoài trời».
  assert.equal(guTheoTag(["Trà", "Ngoài trời"]), "cafe");
  assert.equal(guTheoTag(["Chill", "View đẹp", "Nhóm đông"]), null);
  assert.equal(guTheoTag(["Lẩu", "Nhóm đông"]), null);
  assert.equal(guTheoTag([]), null);
});
