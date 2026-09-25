/* Câu trả lời của Rủ Đi AI trong luồng (ADR-0039, đề xuất).
 *
 * Đo: `docTheAi` đọc thẻ `tra_loi` mà không tin hình của nó — mỗi phần qua
 * đúng hàm đọc thẻ, phần lạ bị bỏ, thẻ không ký «rudi-ai» hay không còn phần
 * nào là `khac`; chữ ký ở chân đọc số tin máy chủ xác nhận; tờ hẹn nằm trong
 * câu trả lời vẫn là tờ hẹn; một hàm danh tính duy nhất cho tác giả NULL; câu
 * trả lời đi vào gói lần sau bằng chính chữ của nó; mặt Nếp không lên thẻ nhóm.
 *
 * KHÔNG đo: ảnh chụp màn thật (chưa chạy ở lát này), hay máy chủ đăng đúng
 * hình (cổng Postgres `TestTraLoiTrichDungTinTag` đo điều đó).
 *
 * Chạy từ apps/mobile:
 *     npx tsc -p tsconfig.test.json && node tools/fixup-esm.mjs
 *     node --test tests/tra-loi-ai.test.mjs
 */
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";

import { chuKyTraLoi, docTheAi, lichTrinhTrongThe, tacGiaTin, trichTu } from "../dist-test/rudi/chat/tin-song.js";
import { gomBoiCanhChat } from "../dist-test/rudi/chat/boi-canh-chat.js";

const HERE = dirname(fileURLToPath(import.meta.url));
const SRC = join(HERE, "..", "src", "rudi");
const outing = "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee";

const cho = { id: "p1", name: "Quán Một" };
function traLoi(extra = {}, phan = null) {
  return {
    kind: "tra_loi",
    payload: {
      ban: 1, tac_gia: "rudi-ai", invocation_id: "0b7c8a1e-2f43-4c55-9a8e-1d2f3a4b5c6d", lenh: "plan",
      doc: { so_tin: 20, chi_loi_nho: false },
      phan: phan ?? [
        { kind: "text", payload: { text: "Tối nay ăn lẩu ở Q1 nhé" } },
        { kind: "places", payload: { intro: "Hai chỗ", places: [cho] } },
        { kind: "itinerary", payload: { title: "Tối thứ Sáu", stops: [{ time_text: "19:00", note: "ăn", place: cho }] } },
      ],
      ...extra,
    },
  };
}

test("thẻ tra_loi được đọc từng phần, bằng đúng hàm đọc thẻ", () => {
  const the = docTheAi(traLoi());
  assert.equal(the.loai, "tra_loi");
  assert.equal(the.soTin, 20);
  assert.equal(the.chiLoiNho, false);
  assert.deepEqual(the.phan.map((p) => p.loai), ["text", "places", "itinerary"]);
  assert.equal(the.phan[0].text, "Tối nay ăn lẩu ở Q1 nhé");
  assert.equal(chuKyTraLoi(the), "Rủ Đi AI · đọc 20 tin");
  const chiLoiNho = docTheAi(traLoi({ doc: { so_tin: 0, chi_loi_nho: true } }));
  assert.equal(chuKyTraLoi(chiLoiNho), "Rủ Đi AI · chỉ đọc lời nhờ");
});

test("phần lạ bị bỏ; thẻ không ký rudi-ai hay không còn phần nào là khac", () => {
  const lan = docTheAi(traLoi({}, [{ kind: "poll", payload: { vote_id: "v", question: "?", options: [] } }, { kind: "text", payload: { text: "Còn lại" } }, { kind: "tra_loi", payload: {} }]));
  assert.equal(lan.loai, "tra_loi");
  assert.deepEqual(lan.phan.map((p) => p.loai), ["text"]);
  assert.equal(docTheAi(traLoi({ tac_gia: "lan" })).loai, "khac");
  assert.equal(docTheAi(traLoi({}, [{ kind: "poll", payload: {} }])).loai, "khac");
  assert.equal(docTheAi(traLoi({ invocation_id: 7 })).loai, "khac");
  // A negative or fractional count is not a count.
  assert.equal(docTheAi(traLoi({ doc: { so_tin: -3 } })).soTin, 0);
});

test("tờ hẹn trong câu trả lời vẫn là tờ hẹn, và kèo đóng trên câu trả lời đi xuống phần đó", () => {
  const the = docTheAi(traLoi({ outing_id: outing }));
  const lich = lichTrinhTrongThe(the);
  assert.equal(lich?.loai, "itinerary");
  assert.equal(lich.outingId, outing);
  assert.equal(lichTrinhTrongThe(docTheAi(traLoi({}, [{ kind: "text", payload: { text: "x" } }]))), null);
  const itinerary = docTheAi({ kind: "itinerary", payload: { title: "T", stops: [{ time_text: "19:00", note: "", place: cho }] } });
  assert.equal(lichTrinhTrongThe(itinerary), itinerary);
});

test("một hàm danh tính cho tác giả NULL, dùng ở mọi chỗ từng tự đoán", () => {
  assert.deepEqual(tacGiaTin({ author_id: null }), { loai: "ai" });
  assert.deepEqual(tacGiaTin({ author_id: "x" }), { loai: "nguoi", id: "x" });
  for (const tep of ["chat/boi-canh-chat.ts", "screens/groups/Conversations.tsx", "screens/chat/GroupChatLive.tsx"]) {
    const nguon = readFileSync(join(SRC, tep), "utf8");
    assert.match(nguon, /tacGiaTin\(/, `${tep} phải hỏi tacGiaTin`);
    assert.doesNotMatch(nguon, /author_id === null \? "Rủ Đi AI"|if \(tin\.author_id === null\) return/, `${tep} còn tự đoán tác giả NULL`);
  }
});

test("trả lời vào câu trả lời của AI trích đúng chữ của nó, như máy chủ sẽ trích", () => {
  const tin = { id: "m", context_id: "c", author_id: null, kind: "ai_card", body: null, image_url: null, card: traLoi(), created_at: "2030-09-22T10:00:00Z", cursor: "m" };
  assert.equal(trichTu(tin, () => "Rủ Đi AI").preview, "Rủ Đi AI: Tối nay ăn lẩu ở Q1 nhé");
  // And a follow-up bundle reads what the AI said, as the AI's turn.
  const goi = gomBoiCanhChat({ tin: [tin], personId: "p" });
  assert.equal(goi.luot[0].vai, "ai");
  assert.equal(goi.luot[0].chu, "Tối nay ăn lẩu ở Q1 nhé");
});

// The server's traLoiPreview and this client are held to ONE vector file; the
// Go side runs it in TestXemTruocCauTraLoiTheoVectorChung. Before this, a reply
// with no text part read «Rủ Đi AI: Tờ hẹn: …» here and «[Rủ Đi AI]» on the
// server, and the client cut on UTF-16 units where the server cuts on runes.
test("dòng trích câu trả lời của AI khớp máy chủ từng vector (không phần chữ, cắt theo rune)", () => {
  const vectors = JSON.parse(readFileSync(join(HERE, "..", "..", "..", "services", "core", "internal", "routes", "testdata", "tra_loi_preview.json"), "utf8"));
  assert.ok(vectors.length >= 10, "tệp vector phải có đủ ca");
  for (const v of vectors) {
    const tin = { id: "m", context_id: "c", author_id: null, kind: "ai_card", body: null, image_url: null, card: v.card, created_at: "2030-09-22T10:00:00Z", cursor: "m" };
    assert.equal(trichTu(tin, () => "Tên người").preview, v.preview, v.ten);
  }
  const khongChu = { id: "m", context_id: "c", author_id: null, kind: "ai_card", body: null, image_url: null, card: traLoi({}, [{ kind: "itinerary", payload: { title: "Tối thứ Sáu", stops: [{ time_text: "19:00", note: "ăn", place: cho }] } }]), created_at: "2030-09-22T10:00:00Z", cursor: "m" };
  assert.equal(trichTu(khongChu, () => "Tên người").preview, "[Rủ Đi AI]");
});

test("thẻ nhóm ký bằng tia lấp lánh tông ai, không mặt Nếp, không màu mới", () => {
  const nguon = readFileSync(join(SRC, "screens", "chat", "TraLoiAi.tsx"), "utf8");
  assert.match(nguon, /name="sparkles"/);
  assert.match(nguon, /color: colors\.ai/);
  assert.match(nguon, /chuKyTraLoi\(the\)/, "chữ ký phải đọc số tin từ thẻ");
  assert.doesNotMatch(nguon, /art\/Nep|<Nep\b/);
  assert.doesNotMatch(nguon, /#[0-9a-f]{3,8}\b|rgb\(/i);
});
