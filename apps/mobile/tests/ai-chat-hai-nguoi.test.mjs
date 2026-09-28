/* Rủ Đi AI trong chat hai người: hai class «đám bạn» / «cặp đôi».
 *
 * Chủ sản phẩm chốt 2026-09-28, thay quyết định «cặp chỉ hoi» của PR #659:
 *  - Đám bạn = mọi nhóm + mọi chat hai người thường: Rủ Đi AI và công cụ y
 *    hệt nhóm («@Rủ Đi» là plan, «/plan», «/chia-bill» được gợi ý, khay có
 *    «Tờ hẹn», hàng ghim tờ hẹn).
 *  - Cặp đôi = chat hai người mà cả hai đã bật «Một đôi» (máy chủ nói qua
 *    `cap_doi` trong chat-capabilities): như đám bạn, cộng hàng ghim «Tờ
 *    giấy», công cụ «Tờ giấy» trong khay và bốn sticker đôi.
 *
 * Đo: nhac-ai không còn nhánh cặp; gợi ý lệnh và khay của cặp có đủ lệnh
 * nhóm, chỉ đổi chữ cho hai người; `cap_doi` đọc đóng (thiếu = không phải
 * cặp đôi); sticker đôi chỉ cho cặp đôi; màn chat gác phần cặp đôi qua
 * `capDoi`, không còn cổng `nhanRieng` nào chặn AI hay tờ hẹn; dòng «Tờ giấy
 * của hai mình» trong cài đặt còn cho mọi cặp.
 *
 * KHÔNG đo: máy chủ thật trả `cap_doi` và nhận plan/chia_bill ở cặp (lát P1,
 * test phía Go), màn vẽ ra sao trên máy thật (Maestro 41/47, chưa chạy được ở
 * cloud).
 */
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";

import { LOI_NHO_PLAN, timNhacAi } from "../dist-test/rudi/chat/nhac-ai.js";
import * as nhacAi from "../dist-test/rudi/chat/nhac-ai.js";
import { CAU_CHO_TRA_LOI, LOI_GOI_AI, goiAi, laCapDoi, lenhSanSang } from "../dist-test/rudi/chat/ai-invocations.js";
import { CONG_CU_TO_GIAY, MO_DAU_HOI_AI, chuKhay, lenhGoiY } from "../dist-test/rudi/chat/khay-cong-cu.js";
import { stickerChoKhay } from "../dist-test/rudi/chat/sticker.js";
import { cauXem, cauXemCach, chuChip } from "../dist-test/rudi/chat/chip-boi-canh.js";
import { gomBoiCanhChat } from "../dist-test/rudi/chat/boi-canh-chat.js";
import { datTokenPhien } from "../dist-test/danh-tinh.js";

const HERE = dirname(fileURLToPath(import.meta.url));
const SRC = join(HERE, "..", "src", "rudi");
const toi = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const ban = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";
const phong = "cccccccc-cccc-4ccc-8ccc-cccccccccccc";
const NHOM = /nhóm|hội/i;

function tin(i, chu) {
  const id = `aaaaaaaa-bbbb-4ccc-8ddd-${String(i).padStart(12, "e")}`;
  return { id, context_id: phong, author_id: i % 2 ? toi : ban, kind: "text", body: chu, image_url: null, card: null, created_at: `2030-09-22T10:${String(i % 60).padStart(2, "0")}:00Z`, cursor: id };
}

/** chat-capabilities as the server reports them for a friends' pair: like a group. */
const CAP_BAN = {
  protocol: "legacy",
  realtime: { available: true },
  ai: {
    plan: { available: true, reason: null },
    chia_bill: { available: true, reason: null },
    hoi: { available: true, reason: null },
    share_scope: "caller_attached",
    mention: true,
  },
  media: { image: true, sticker: true, voice: false },
  cap_doi: false,
};
const CAP_DOI = { ...CAP_BAN, cap_doi: true };

test("đám bạn hai người: @Rủ Đi là plan, /plan và /chia-bill là lời gọi — y hệt nhóm", () => {
  assert.equal(timNhacAi.length, 1, "timNhacAi không còn nhận cờ cặp");
  assert.equal("LOI_NHO_HOI_HAI_NGUOI" in nhacAi, false, "câu mặc định riêng cho cặp đã gỡ");
  assert.deepEqual(timNhacAi("@Rủ Đi gợi ý quán"), { lenh: "plan", loiNho: "gợi ý quán" });
  assert.deepEqual(timNhacAi("tối nay @rudi đi đâu"), { lenh: "plan", loiNho: "tối nay đi đâu" });
  assert.deepEqual(timNhacAi("/plan tối nay đi đâu"), { lenh: "plan", loiNho: "tối nay đi đâu" });
  assert.equal(timNhacAi("/chia-bill mình trả 300k")?.lenh, "chia_bill");
  assert.equal(timNhacAi("@Rủ Đi")?.loiNho, LOI_NHO_PLAN);
  // Even an extra argument from an old caller cannot bring the pair branch back.
  assert.deepEqual(timNhacAi("@Rủ Đi gợi ý quán", true), { lenh: "plan", loiNho: "gợi ý quán" });
  assert.equal(timNhacAi("/chia-bill", true)?.lenh, "chia_bill");
  assert.equal(timNhacAi("tối nay ăn gì"), null);
});

test("độ sẵn sàng đọc từ chat-capabilities: cặp đám bạn có plan, chia_bill và hoi như nhóm", () => {
  assert.equal(lenhSanSang(CAP_BAN, "plan"), true);
  assert.equal(lenhSanSang(CAP_BAN, "chia_bill"), true);
  assert.equal(lenhSanSang(CAP_BAN, "hoi"), true);
  assert.equal(lenhSanSang({ ...CAP_BAN, ai: { ...CAP_BAN.ai, plan: { available: false, reason: "provider_unavailable" } } }, "plan"), false);
  assert.equal(lenhSanSang(null, "plan"), false);
});

test("cap_doi đọc đóng: chỉ true tường minh mới là cặp đôi", () => {
  assert.equal(laCapDoi(CAP_DOI), true);
  assert.equal(laCapDoi(CAP_BAN), false);
  const { cap_doi: _bo, ...cu } = CAP_BAN;
  assert.equal(laCapDoi(cu), false, "máy chủ cũ không có cap_doi: không phải cặp đôi");
  assert.equal(laCapDoi({ ...CAP_BAN, cap_doi: "true" }), false);
  assert.equal(laCapDoi(null), false);
});

test("cặp đám bạn: lời gọi lên máy chủ là plan, kèm tin tag và gói người gọi đã xem", async () => {
  const goi = gomBoiCanhChat({ tin: [tin(2, "cuối tuần rảnh không"), tin(1, "rảnh nè")], personId: toi });
  const nhac = timNhacAi("@Rủ Đi đi đâu giờ");
  const original = globalThis.fetch;
  const calls = [];
  datTokenPhien("synthetic-test-token");
  globalThis.fetch = async (url, options) => { calls.push({ url, ...options }); return { ok: true, status: 202, json: async () => ({ id: "job" }), text: async () => "{}" }; };
  try {
    const trigger = "dddddddd-dddd-4ddd-8ddd-dddddddddddd";
    await goiAi(phong, toi, nhac.loiNho, toi, goi, nhac.lenh, trigger);
    assert.equal(calls.length, 1);
    assert.match(String(calls[0].url), new RegExp(`/contexts/${phong}/ai-invocations$`));
    const body = JSON.parse(calls[0].body);
    assert.equal(body.command, "plan");
    assert.equal(body.prompt, "đi đâu giờ");
    assert.equal(body.trigger_message_id, trigger);
    assert.equal(body.boi_canh.luot.length, 2);
  } finally { globalThis.fetch = original; datTokenPhien(null); }
});

test("ô soạn của cặp gợi ý đủ bốn lệnh như nhóm, chữ viết cho hai người", () => {
  const cap = lenhGoiY(true);
  const nhom = lenhGoiY(false);
  assert.deepEqual(cap.map((l) => l.nhan), ["/plan", "/vote", "/chia-bill", "@Rủ Đi"]);
  assert.deepEqual(cap.map((l) => l.nhan), nhom.map((l) => l.nhan));
  assert.deepEqual(cap.map((l) => l.goiY), nhom.map((l) => l.goiY));
  for (const l of cap) {
    assert.doesNotMatch(l.moTa, NHOM, l.moTa);
    // Each suggestion is the command it names: filling it in asks the same thing as in a group.
    if (l.nhan !== "/vote") assert.equal(timNhacAi(l.goiY + "x")?.lenh, timNhacAi(nhom.find((g) => g.nhan === l.nhan).goiY + "x")?.lenh);
  }
});

test("khay: «Hỏi Rủ Đi AI» điền /plan ở mọi phòng; chữ khay của cặp không nói «hội» hay «nhóm»", () => {
  assert.equal(MO_DAU_HOI_AI, "/plan ");
  assert.equal(timNhacAi(MO_DAU_HOI_AI)?.lenh, "plan");
  assert.equal(CONG_CU_TO_GIAY, "Tờ giấy");
  const k = chuKhay(true);
  assert.deepEqual(Object.keys(k).sort(), ["goiYPoll", "loiHoiAi", "nhanPlan", "tieuDePoll"], "khay không còn khe cặp riêng (congCuHen/lenhAi/hoiAiTrenKhay)");
  for (const cau of Object.values(k)) assert.doesNotMatch(cau, NHOM, cau);
  assert.match(k.loiHoiAi, /cả hai bạn/);
  assert.match(chuKhay(false).loiHoiAi, /cả nhóm/);
});

test("sticker đôi chỉ cho cặp đôi; cặp đám bạn có tám hình như nhóm", () => {
  assert.deepEqual(stickerChoKhay(false).doi, []);
  assert.equal(stickerChoKhay(false).chung.length, 8);
  assert.deepEqual([...stickerChoKhay(true).doi], ["hen-nhe", "nho-nhau", "ve-toi-chua", "om-cai"]);
});

test("chữ của chip, tấm «Xem» và câu từ chối ở cặp không nói «nhóm»", () => {
  const trong = gomBoiCanhChat({ tin: [], personId: toi });
  const goi = gomBoiCanhChat({ tin: [tin(1, "chào")], personId: toi });
  const cauCap = [
    chuChip(trong, true, true, true).cau,
    chuChip(goi, true, true, true).cau,
    chuChip(goi, false, true, true).cau,
    chuChip(goi, true, false, true).cau,
    cauXem(goi, true),
    cauXemCach(true),
    ...Object.values(LOI_GOI_AI),
    ...Object.values(CAU_CHO_TRA_LOI),
  ];
  for (const cau of cauCap) assert.doesNotMatch(cau, /nhóm|thành viên/i, cau);
  assert.match(cauXem(goi, true), /cả hai bạn/);
  assert.equal(chuChip(trong, true, true).cau, "Nhóm chưa có tin nào, chỉ gửi lời nhờ");
  assert.match(cauXem(goi), /cả nhóm/);
});

test("màn chat: không cổng nhanRieng nào chặn AI hay tờ hẹn; phần cặp đôi gác qua capDoi", () => {
  const live = readFileSync(join(SRC, "screens", "chat", "GroupChatLive.tsx"), "utf8");
  // No pair gate left on the AI, the pinned tờ hẹn or the empty-state plan button.
  assert.doesNotMatch(live, /timNhacAi\([^)]*nhanRieng/, "timNhacAi không còn nhận cờ cặp");
  assert.doesNotMatch(live, /if \(nhanRieng\) return null;/, "cặp đám bạn có hàng ghim tờ hẹn như nhóm");
  assert.doesNotMatch(live, /!nhanRieng \? <RudiButton label="Rủ hội một buổi"/);
  assert.match(live, /\n\s*<RudiButton label="Rủ hội một buổi" variant="outline"/, "nút mở tờ hẹn ở màn trống có ở mọi phòng");
  assert.match(live, /const nhacDangGo = timNhacAi\(nhap\)/);
  assert.match(live, /const nhac = command === undefined \? timNhacAi\(body\) : null/);
  assert.match(live, /MO_DAU_HOI_AI \+ nhapRef\.current\.text/);
  // The couple's extras read the server's `cap_doi`, and only in a pair.
  assert.match(live, /const capDoi = nhanRieng && laCapDoi\(ai\.capabilities\);/);
  assert.match(live, /\{capDoi && phien !== null \? <HangToGiaySong /, "hàng ghim «Tờ giấy» chỉ cho cặp đôi");
  assert.doesNotMatch(live, /nhanRieng && phien !== null \? <HangToGiaySong/);
  assert.match(live, /onToGiay=\{capDoi \? /, "công cụ «Tờ giấy» trong khay chỉ cho cặp đôi");
  assert.match(live, /<KhaySticker capDoi=\{capDoi\} /, "sticker đôi chỉ cho cặp đôi");
  // Nếp's notebook kind goes through the one module that decides it.
  assert.match(live, /loaiSo: nhom \? loaiSoCua\(nhom, \{ bat: capDoi \}\) : undefined/);
  // Wording for two stays for every pair.
  assert.match(live, /<ChipBoiCanh goi=\{goiChip\} haiNguoi=\{nhanRieng\}/);
  assert.match(live, /lenhGoiY\(nhanRieng\)/);
  assert.match(live, /accessibilityLabel=\{nhanRieng \? "Xem hồ sơ" : "Thành viên nhóm"\}/);
  // «Tự tạo kèo» under a failed request belongs to a plan, never to `hoi`.
  assert.match(live, /\(request\.command \?\? "plan"\) === "plan" \? <RudiButton label="Tự tạo kèo"/);

  const soHen = readFileSync(join(SRC, "screens", "chat", "SoHen.tsx"), "utf8");
  assert.match(soHen, /lenhSanSang\(capabilities, "plan"\)/, "khay đọc độ sẵn sàng của plan ở mọi phòng");
  assert.match(soHen, /\{ vat: "lich", label: "Tờ hẹn", action: \(\) => onPanel\("plan"\) \}/, "«Tờ hẹn» có ở mọi khay, không bị «Tờ giấy» thay chỗ");
  assert.match(soHen, /\.\.\.\(onToGiay \? \[/, "«Tờ giấy» là công cụ THÊM của cặp đôi");

  // The settings row stays for every pair: it is how a friends' pair reaches «Một đôi».
  const caiDat = readFileSync(join(SRC, "screens", "chat", "CaiDatNhom.tsx"), "utf8");
  assert.match(caiDat, /\{laPair \? \(\s*<ListRow[\s\S]*?title="Tờ giấy của hai mình"/);
});
