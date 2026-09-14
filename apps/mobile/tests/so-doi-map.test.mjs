// The live provider's mapping, driven without a device. It is the only new
// logic between the wire and thirteen screens that were read blind in Phase 2.
import assert from "node:assert/strict";
import test from "node:test";

import { caHaiDongY, rangBuocCua, toTomTatThanhTo } from "../dist-test/rudi/to-giay/so-doi-map.js";

const TOI = "toi", KIA = "nguoi-ay";
const so = (over = {}) => ({
  context_id: "cap-1",
  cycle_state: "active",
  participants: [TOI, KIA],
  my_consents: [
    { purpose: "lap_so", granted: true },
    { purpose: "bat_doi", granted: false },
    { purpose: "doc_chat", granted: false },
  ],
  their_consents_granted: { lap_so: true, bat_doi: false, doc_chat: false },
  pending_proposals: [],
  constraints: [],
  nep_gui_ho: false,
  open_paper_id: null,
  ...over,
});

test("một bậc chỉ bật khi CẢ HAI đồng ý", () => {
  assert.equal(caHaiDongY(so(), "lap_so"), true);
  assert.equal(caHaiDongY(so(), "bat_doi"), false);

  // Tôi đồng ý, người kia chưa: im lặng không phải đồng ý.
  const chiToi = so({
    my_consents: [{ purpose: "bat_doi", granted: true }],
    their_consents_granted: { bat_doi: false },
  });
  assert.equal(caHaiDongY(chiToi, "bat_doi"), false);

  // Người kia đồng ý, tôi chưa — chiều ngược lại cũng phải tắt, và đây là
  // chiều mà một lần đọc nhầm trường sẽ bật đèn cho một người tự đồng ý.
  const chiHo = so({
    my_consents: [{ purpose: "bat_doi", granted: false }],
    their_consents_granted: { bat_doi: true },
  });
  assert.equal(caHaiDongY(chiHo, "bat_doi"), false);

  assert.equal(caHaiDongY(null, "lap_so"), false, "chưa có sổ thì chưa có bậc nào");
});

test("ràng buộc đọc theo CHỦ, không theo thứ tự hàng", () => {
  const s = so({
    constraints: [
      { owner_id: KIA, kind: "khong_an_duoc", content: "Cay", version: 1 },
      { owner_id: TOI, kind: "dung", content: "Đừng rủ sau 21:00.", version: 2 },
      { owner_id: TOI, kind: "khong_an_duoc", content: "Hải sản", version: 1 },
    ],
  });
  assert.deepEqual(rangBuocCua(s, TOI), { khong_an_duoc: "Hải sản", dung: "Đừng rủ sau 21:00." });
  assert.deepEqual(rangBuocCua(s, KIA), { khong_an_duoc: "Cay", dung: "" });
  assert.deepEqual(rangBuocCua(s, null), { khong_an_duoc: "", dung: "" });
  assert.deepEqual(rangBuocCua(null, TOI), { khong_an_duoc: "", dung: "" });
});

const tomTat = (over = {}) => ({
  id: "to-9",
  state: "da_giu",
  version: 2,
  tuan: "2026-09-14",
  ngay: "2026-09-19",
  expires_at: "2026-09-20T17:00:00Z",
  chang_dau: { gio: "19:00", viec: "Ăn tối, quán mới" },
  dong_giu_dau: "Cái đèn ở góc bàn.",
  ...over,
});

test("một hàng đã khép mang đủ sáu sự thật nó hiện, và không bịa gì thêm", () => {
  const to = toTomTatThanhTo(tomTat());
  assert.equal(to.id, "to-9");
  assert.equal(to.state, "da_giu");
  assert.equal(to.version, 2);
  assert.equal(to.tuan, "2026-09-14");
  assert.equal(to.versions[0].content.ngay, "2026-09-19");
  assert.deepEqual(to.versions[0].content.chang[0], {
    gio: "19:00",
    viec: "Ăn tối, quán mới",
    place_id: null,
    can_kiem: false,
  });
  assert.equal(to.keeps[0].line, "Cái đèn ở góc bàn.");
  assert.equal(to.co_the_ghi_da_di, false, "nút «đã đi» không bao giờ mọc trên một hàng");
  assert.equal(to.outing_id, null);
  assert.equal(to.sent_by, null);
});

test("thiếu mẩu nào thì để trống, không đoán", () => {
  const to = toTomTatThanhTo(tomTat({ chang_dau: null, dong_giu_dau: null, ngay: null }));
  assert.deepEqual(to.versions[0].content.chang, [], "không có chặng giả");
  assert.deepEqual(to.keeps, [], "không có dòng giữ giả");
  assert.equal(to.versions[0].content.ngay, "");
});
