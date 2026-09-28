/* Não chữ của Nếp phía máy (ADR-0036 §2.7, §2.8, §4).
 *
 * Chạy từ apps/mobile:
 *     npx tsc -p tsconfig.test.json && node tools/fixup-esm.mjs
 *     node --test tests/nep-hoi.test.mjs
 */
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";

import {
  GIOI_HAN_PHIEN,
  LOI_KET_QUA_NEP,
  cauKetQuaNep,
  cauPhienDiKem,
  conCho,
  gomPhien,
  khoaLanHoi,
  nepDuocHoi,
  nhipHoiNepMs,
} from "../dist-test/rudi/nep/hoi.js";

const HERE = dirname(fileURLToPath(import.meta.url));
const NEP = join(HERE, "..", "src", "rudi", "nep");

test("phiên gửi kèm là các lượt MỚI NHẤT còn vừa giới hạn, giữ thứ tự cũ trước", () => {
  const luot = Array.from({ length: 30 }, (_, i) => ({ vai: i % 2 ? "nep" : "toi", chu: `lượt ${i}` }));
  const kem = gomPhien(luot, "câu mới");
  assert.equal(kem.length, GIOI_HAN_PHIEN.luot);
  assert.equal(kem[0].chu, "lượt 6");
  assert.equal(kem.at(-1).chu, "lượt 29");
});

test("tổng chữ đếm theo code point như máy chủ, và câu hỏi cũng tính vào tổng", () => {
  const moi = "ạ".repeat(GIOI_HAN_PHIEN.chuMoiLuot);
  const luot = Array.from({ length: 10 }, () => ({ vai: "toi", chu: moi }));
  const kem = gomPhien(luot, "x".repeat(GIOI_HAN_PHIEN.hoi));
  const tong = kem.reduce((n, l) => n + [...l.chu].length, GIOI_HAN_PHIEN.hoi);
  assert.ok(tong <= GIOI_HAN_PHIEN.tong, `gửi ${tong} chữ, máy chủ sẽ từ chối`);
  assert.equal(kem.length, 7);
  // An emoji is one code point to the server and two UTF-16 units to JS.
  const emoji = gomPhien([{ vai: "toi", chu: "😀".repeat(GIOI_HAN_PHIEN.chuMoiLuot) }], "");
  assert.equal(emoji.length, 1, "đếm theo UTF-16 sẽ bỏ nhầm lượt này");
});

test("không lấy lượt rỗng hay lượt dài hơn máy chủ nhận, và dừng ở đó chứ không nhảy qua", () => {
  const kem = gomPhien(
    [
      { vai: "toi", chu: "cũ" },
      { vai: "nep", chu: "a".repeat(GIOI_HAN_PHIEN.chuMoiLuot + 1) },
      { vai: "toi", chu: "mới" },
    ],
    "x",
  );
  assert.deepEqual(kem, [{ vai: "toi", chu: "mới" }], "nhảy qua lượt hỏng sẽ gửi một phiên có lỗ ở giữa");
  assert.deepEqual(gomPhien([{ vai: "toi", chu: "  " }], "x"), []);
});

test("gửi kèm chỉ mang vai và chữ, không trường nào khác", () => {
  const kem = gomPhien([{ vai: "toi", chu: "a", luc: "x", ten: "Lan" }], "b");
  assert.deepEqual(Object.keys(kem[0]).sort(), ["chu", "vai"]);
});

test("khối Mình đang thấy nói đúng số lượt đi kèm, và nói Nếp không đọc gì thêm", () => {
  assert.match(cauPhienDiKem(0), /Chưa có lượt/);
  assert.match(cauPhienDiKem(4), /Kèm 4 lượt/);
  for (const n of [0, 4]) assert.match(cauPhienDiKem(n), /không đọc chat, gu hay lịch sử/);
});

test("ở màn tiền Nếp không được hỏi; màn có chữ finance ở giữa thì vẫn được", () => {
  assert.equal(nepDuocHoi({ man: "/settlements/7" }), false);
  assert.equal(nepDuocHoi({ man: "finance" }), false);
  assert.equal(nepDuocHoi({ man: "financial-report" }), true);
  assert.equal(nepDuocHoi(null), true);
});

test("khoá lần hỏi đổi khi phiên hay phiếu đổi, không chỉ khi câu đổi", () => {
  const a = khoaLanHoi("x", [], null);
  assert.notEqual(a, khoaLanHoi("x", [{ vai: "toi", chu: "y" }], null));
  assert.notEqual(a, khoaLanHoi("x", [], { man: "explore" }));
  assert.equal(a, khoaLanHoi("x", [], null));
});

test("đọc lại có trần, và dừng khi việc đã xong dù xong kiểu gì", () => {
  assert.equal(nhipHoiNepMs(0), 800);
  assert.equal(nhipHoiNepMs(1000), 4000);
  assert.equal(nhipHoiNepMs(Number.NaN), 800);
  for (const s of ["succeeded", "failed", "cancelled"]) assert.equal(conCho(s), false);
  for (const s of ["queued", "running"]) assert.equal(conCho(s), true);
});

test("mọi mã kết quả hỏng có câu người đọc, mã lạ rơi về câu chung", () => {
  for (const [ma, cau] of Object.entries(LOI_KET_QUA_NEP)) {
    assert.ok(cau.length > 20, ma);
    assert.doesNotMatch(cau, /[a-z]+_[a-z_]+|lỗi|[—–]/i, `${ma}: ${cau}`);
  }
  assert.match(cauKetQuaNep("ma_moi"), /hỏi lại/);
  assert.match(cauKetQuaNep(null), /hỏi lại/);
});

test("phiên Nếp không bao giờ xuống đĩa: hook và bảng không chạm kho", () => {
  for (const tep of ["useNepHoi.ts", "hoi.ts", "NepBang.tsx"]) {
    const nguon = readFileSync(join(NEP, tep), "utf8");
    // Imports and calls, not prose: the hook's own comment names what it avoids.
    assert.doesNotMatch(
      nguon,
      /from "(@react-native-async-storage[^"]*|expo-secure-store|expo-file-system[^"]*|\.\.\/kho|\.\/luu-dock)"|ghiGiaoDien\w*\(|localStorage\./,
      `${tep} ghi phiên xuống đĩa`,
    );
  }
  // And it ends with the panel: the hook clears on close.
  const hook = readFileSync(join(NEP, "useNepHoi.ts"), "utf8");
  assert.match(hook, /if \(mo\) return;[\s\S]*?datLuot\(\[\]\)/, "đóng bảng phải xoá phiên");
});

test("Nếp không vẽ lên thẻ của nhóm: thẻ AI nhóm và đường gọi nhóm không import Nếp", () => {
  // ADR-0036 §2.6: two roles, two faces. The group answer card is «Rủ Đi AI».
  const rudi = join(HERE, "..", "src", "rudi");
  for (const tep of ["screens/chat/TheAi.tsx", "screens/chat/TraLoiAi.tsx", "screens/chat/ChipBoiCanh.tsx", "chat/useChatAi.ts", "chat/ai-invocations.ts", "chat/nhac-ai.ts"]) {
    assert.doesNotMatch(readFileSync(join(rudi, tep), "utf8"), /art\/Nep"|\/nep\//, tep);
  }
});
