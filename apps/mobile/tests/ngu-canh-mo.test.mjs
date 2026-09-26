import assert from "node:assert/strict";
import { test } from "node:test";

import { nguCanhMo } from "../dist-test/rudi/ngu-canh-mo.js";

const phien = {
  context_id: "nhom-hoi",
  contexts: [
    { id: "nhom-hoi", my_state: "active" },
    { id: "cap-linh", my_state: "active" },
    { id: "nhom-moi", my_state: "invited" },
  ],
};

test("không có ctx, hay ctx là nhóm hiện tại, thì mở như cũ", () => {
  assert.deepEqual(nguCanhMo(phien, undefined), { kieu: "hien-tai" });
  assert.deepEqual(nguCanhMo(phien, ""), { kieu: "hien-tai" });
  assert.deepEqual(nguCanhMo(phien, "nhom-hoi"), { kieu: "hien-tai" });
  assert.deepEqual(nguCanhMo(phien, ["cap-linh"]), { kieu: "hien-tai" });
});

test("sổ đôi mình đang ở thì mở đúng sổ đó", () => {
  assert.deepEqual(nguCanhMo(phien, "cap-linh"), { kieu: "khac", contextId: "cap-linh" });
  // Signed in with no current group: the pair still opens.
  assert.deepEqual(nguCanhMo({ ...phien, context_id: null }, "cap-linh"), { kieu: "khac", contextId: "cap-linh" });
});

test("context lạ hoặc mới được mời thì không mở", () => {
  assert.deepEqual(nguCanhMo(phien, "nhom-moi"), { kieu: "tu-choi" });
  assert.deepEqual(nguCanhMo(phien, "cap-nguoi-khac"), { kieu: "tu-choi" });
  assert.deepEqual(nguCanhMo({ context_id: null, contexts: null }, "cap-linh"), { kieu: "tu-choi" });
});
