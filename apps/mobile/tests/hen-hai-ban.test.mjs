import assert from "node:assert/strict";
import { test } from "node:test";

import { henHaiBan } from "../dist-test/rudi/keo/hen-hai-ban.js";

const keo = (id, starts_on, ends_on = starts_on) => ({ id, title: id, starts_on, ends_on });

test("kèo sắp tới của mọi sổ đôi, sớm nhất trước, kèm tên người kia", () => {
  const doi = [{ id: "cap-linh", tenNguoiKia: "Linh" }, { id: "cap-an", tenNguoiKia: "An" }];
  const theoDoi = new Map([
    ["cap-linh", [keo("lau-ga", "2026-09-26"), keo("da-qua", "2026-09-01")]],
    ["cap-an", [keo("cafe", "2026-09-24")]],
  ]);
  const hen = henHaiBan(doi, theoDoi, "2026-09-23");
  assert.deepEqual(hen.map((h) => [h.contextId, h.tenNguoiKia, h.keo.id]), [
    ["cap-an", "An", "cafe"],
    ["cap-linh", "Linh", "lau-ga"],
  ]);
  // The row is the outing as read, not a copy with the bookkeeping field.
  assert.equal(Object.hasOwn(hen[0].keo, "__doi"), false);
});

test("buổi đã qua không vào danh sách hẹn; sổ chưa đọc được là rỗng, không lỗi", () => {
  const hen = henHaiBan([{ id: "cap-linh", tenNguoiKia: "Linh" }, { id: "cap-moi", tenNguoiKia: "Mai" }],
    new Map([["cap-linh", [keo("hom-qua", "2026-09-22")]]]), "2026-09-23");
  assert.deepEqual(hen, []);
});

test("buổi đang diễn ra và hôm nay vẫn là hẹn", () => {
  const hen = henHaiBan([{ id: "c", tenNguoiKia: "Linh" }],
    new Map([["c", [keo("nay", "2026-09-23"), keo("dang", "2026-09-22", "2026-09-24")]]]), "2026-09-23");
  assert.deepEqual(hen.map((h) => h.keo.id).sort(), ["dang", "nay"]);
});
