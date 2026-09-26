import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

import { TEN_GIU_CHO, laTenGiuCho, tenThat } from "../dist-test/rudi/ten-giu-cho.js";

const goc = new URL("../../../", import.meta.url);

// The client decides "has this person chosen a name?" by comparing with the
// server's placeholder. If the server changes its spelling and this does not,
// every new account silently becomes "named" again -- so both server copies
// are read, not remembered.
test("chữ giữ chỗ khớp đúng từng chữ với Go và Python", () => {
  const go = readFileSync(new URL("services/core/internal/domain/authsteps/authsteps.go", goc), "utf8");
  const py = readFileSync(new URL("services/api/app/api/service.py", goc), "utf8");
  assert.match(go, new RegExp(`NewPersonName\\s*=\\s*"${TEN_GIU_CHO}"`));
  assert.match(py, new RegExp(`NEW_PERSON_NAME\\s*=\\s*"${TEN_GIU_CHO}"`));
});

test("laTenGiuCho / tenThat quyết theo giá trị, không theo cờ lần đầu", () => {
  assert.equal(laTenGiuCho("Thành viên mới"), true);
  assert.equal(laTenGiuCho("  Thành viên mới "), true);
  assert.equal(laTenGiuCho(""), true);
  assert.equal(laTenGiuCho(undefined), true);
  assert.equal(laTenGiuCho("Linh"), false);
  assert.equal(tenThat("Thành viên mới"), null);
  assert.equal(tenThat(" Linh "), "Linh");
});
