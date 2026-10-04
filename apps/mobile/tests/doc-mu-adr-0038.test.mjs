/* ADR-0038: năm quyết định sau lượt đọc mù 26/09, phần phía client.
 *
 * Chạy từ apps/mobile:
 *     npx tsc -p tsconfig.test.json && node tools/fixup-esm.mjs && node --test tests/doc-mu-adr-0038.test.mjs
 *
 * Ca thuần (lý do chưa đăng, hình ruy băng) chạy trên mã đã biên dịch; phần còn
 * lại ghim hình dạng mã. Không chứng minh người thật đọc đúng: điều đó là lượt
 * đọc mù và ảnh chụp ghi trong commit.
 */
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

import { hinhRuyBang, hinhVanTay } from "../dist-test/rudi/art/giay.js";
import { coTheDang, lyDoChuaDang } from "../dist-test/screens/ca-nhan/bai-dang.js";
import { NGAN_SACH, SO_THICH } from "../dist-test/screens/vao-cua/so-thich.js";
import { kiemLop, phanTich } from "./_kiem-lop.mjs";

const doc = (tep) => readFileSync(new URL(`../src/${tep}`, import.meta.url), "utf8");

test("§2.2: lý do chưa đăng có đúng khi và chỉ khi Đăng chưa bấm được", () => {
  const forms = [
    { body: "", audience: "friends", contextId: null },
    { body: "   ", audience: "public", contextId: null },
    { body: "Đi thôi", audience: "group", contextId: null },
    { body: "Đi thôi", audience: "group", contextId: "c1" },
    { body: "Đi thôi", audience: "only_me", contextId: null },
  ];
  for (const f of forms) {
    const lyDo = lyDoChuaDang(f);
    assert.equal(lyDo === null, coTheDang(f), JSON.stringify(f));
    if (lyDo !== null) assert.ok(lyDo.length > 8 && !/—/.test(lyDo), lyDo);
  }
});

test("§2.2: nút tắt không mờ đi mà nói vì sao; lý do tới trình đọc màn hình", () => {
  const ui = doc("rudi/ui.tsx");
  const nut = ui.slice(ui.indexOf("export function RudiButton("), ui.indexOf("export function IconButton("));
  assert.doesNotMatch(nut, /styles\.disabled|opacity: 0\.45/, "RudiButton lại mờ bằng opacity");
  assert.match(nut, /accessibilityHint=\{tat && lyDo \? lyDo : undefined\}/);
  assert.match(nut, /borderStyle: "dashed"/);
  const dau = doc("rudi/ui/StampButton.tsx");
  assert.doesNotMatch(dau, /opacity: disabled/, "StampButton lại mờ bằng opacity");
  assert.match(dau, /accessibilityHint=\{tat && lyDo \? lyDo : undefined\}/);
  for (const [tep, mau] of [
    ["rudi/screens/nguoi/DangBaiScreen.tsx", /lyDo=\{lyDoChuaDang\(form\) \?\? undefined\}/],
    ["rudi/screens/story/DangStoryScreen.tsx", /lyDo=\{anh === null && anhDaTai === null \? "Chọn một tấm ảnh trước đã\."/],
    ["rudi/screens/Onboarding.tsx", /lyDo=\{duDieuKien \? undefined : `Chọn thêm \$\{TOI_THIEU - muc\.length\} mục nữa\.`\}/],
  ]) {
    assert.match(doc(tep), mau, `${tep}: nút chính tắt mà không nói vì sao`);
  }
});

test("§2.2: «Lưu tên» chỉ hiện khi tên đã khác; người cùng nhóm không thấy «Nhắn tin» khoá", () => {
  const caiDat = doc("rudi/screens/chat/CaiDatNhom.tsx");
  assert.match(caiDat, /\{ten\.trim\(\) !== nhom\.display_name \? \(\s*<RudiButton/);
  const hoSo = doc("rudi/screens/nguoi/HoSoNguoiScreen.tsx");
  // B8 (QA UI-078): the branch also waits on the block list (`&& !daChan`).
  const dau = hoSo.indexOf('relation === "groupmate" && !daChan ?');
  assert.ok(dau > 0, "nhánh người cùng nhóm còn đó");
  const nhanh = hoSo.slice(dau, hoSo.indexOf('relation !== "self" ?', dau));
  assert.match(nhanh, /Kết bạn để nhắn riêng\./);
  assert.doesNotMatch(nhanh, /label="Nhắn tin"/, "nút «Nhắn tin» khoá quay lại");
});

test("§2.3: ruy băng mép Nếp nằm gọn trong 10dp, đuôi chữ V, đúng ngữ pháp Java", () => {
  for (const h of [48, 64, 96]) {
    const r = hinhRuyBang(10, h);
    kiemLop(`ruy băng 10×${h}`, [{ d: r.than, mau: "gap" }, { d: r.nep, mau: "muc", net: 1 }], 10, h);
    const diem = phanTich(r.than).filter(({ c }) => c !== "Z").map(({ args }) => args.slice(-2));
    assert.ok(diem.every(([x]) => x >= 0 && x <= 10), "ruy băng tràn khỏi 10dp");
    const dinhV = diem.find(([x, y]) => x === 5 && y < h);
    assert.ok(dinhV, "đuôi ruy băng không có vết cắt chữ V");
    assert.ok(h - dinhV[1] >= 4 && h - dinhV[1] <= h / 4 + 1e-9, `vết V sâu ${h - dinhV[1]}`);
  }
  const dock = doc("rudi/nep/NepDock.tsx");
  assert.match(dock, /const ruyBang = hinhRuyBang\(NEP_MEP_HEP, kich\.h\);/, "ruy băng không dùng đúng bề rộng mép 10dp");
  assert.match(dock, /interpolate\(caiX\.value, \[0, NEP_DIA - NEP_MEP_HEP\], \[0, 1\]/, "ruy băng không nhường chỗ cho mặt Nếp khi kéo ra");
});

test("§2.4: câu chữ mới, id không đổi", () => {
  assert.deepEqual(SO_THICH.map((m) => m.id), ["an-uong", "cafe", "nightlife", "mon-local", "outdoor", "shopping", "karaoke", "game"]);
  for (const m of SO_THICH) assert.doesNotMatch(m.nhan, /^(Nightlife|Outdoor|Shopping|Game)$/, `nhãn tiếng Anh còn sót: ${m.nhan}`);
  assert.match(doc("rudi/hanh-trinh/ThanhCheDo.tsx"), /\["Lịch trình", "Bản đồ"\]/);
  assert.match(doc("rudi/screens/groups/Members.tsx"), /"Người lập nhóm"/);
  const ht = doc("rudi/hanh-trinh/ManHinhHanhTrinh.tsx");
  assert.match(ht, />Các chặng trong ngày</);
  assert.match(ht, />Ngày này chưa có điểm nào trên bản đồ</);
});

test("§2.5: mức chi thứ tư nối liền mức cũ và không có trần; id cũ giữ nguyên", () => {
  assert.deepEqual(NGAN_SACH.map((k) => k.id), ["tiet-kiem", "vua-phai", "thoai-mai", "rong-tay"]);
  for (let i = 1; i < NGAN_SACH.length; i += 1) assert.equal(NGAN_SACH[i].tu, NGAN_SACH[i - 1].den, `hở hoặc chồng giữa ${NGAN_SACH[i - 1].id} và ${NGAN_SACH[i].id}`);
  const top = NGAN_SACH.at(-1);
  assert.equal(top.den, null, "mức trên cùng phải không có trần");
  assert.equal(top.tu, 500_000);
  assert.ok(NGAN_SACH.every((k) => Number.isInteger(k.tu) && (k.den === null || Number.isInteger(k.den))), "luật 1: số nguyên đồng");
});

test("đọc mù: phiếu bầu là vân tay có vân, không phải chấm đặc; phiếu của bạn mang mực của bạn", () => {
  for (const [w, h] of [[12, 15], [24, 30]]) {
    const lop = hinhVanTay(w, h);
    kiemLop(`vân tay ${w}×${h}`, lop, w, h);
    assert.ok(lop.length >= 3, `vân tay cần ít nhất ba đường vân, có ${lop.length}`);
    for (const l of lop) {
      assert.ok(l.net !== undefined && l.net > 0, "vân tay là nét, không phải khối tô");
      assert.ok(!/Z/.test(l.d), "mỗi đường vân để hở một phần tư, không phải vòng bia");
    }
    const dau = lop.map((l) => phanTich(l.d)[0].args[0]);
    assert.ok(dau.some((x) => x > w / 2) && dau.some((x) => x < w / 2), "chỗ hở phải so le hai bên để đọc ra xoáy vân");
  }
  const theAi = doc("rudi/screens/chat/TheAi.tsx");
  assert.doesNotMatch(theAi, /dauVanTay/, "chấm đặc cũ quay lại");
  assert.match(theAi, /lop=\{HINH_VAN_TAY\}/);
  assert.match(theAi, /doiMau=\{cuaToi && i === 0 \? \{ muc: mucNguoi\(personId, dark\) \} : undefined\}/);
});

test("đọc mù: nút gửi chính chưa có gì để gửi không còn là đĩa cam đặc", () => {
  const ui = doc("rudi/ui.tsx");
  const nut = ui.slice(ui.indexOf("export function IconButton("), ui.indexOf("export function IconButton(") + 2600);
  assert.match(nut, /const tatSolid = solid && disabled && !loading;/);
  assert.match(nut, /const background = tatSolid\s*\? colors\.card/);
  assert.match(nut, /const glyph = tatSolid\s*\? colors\.inkSoft/);
  assert.match(nut, /tatSolid && \{ borderColor: colors\.lineStrong, borderStyle: "dashed"/);
  const tuong = doc("rudi/screens/ky-niem/GroupWallLive.tsx");
  assert.match(tuong, /\{nhap\.trim\(\) !== "" \|\| ban \? \(\s*<RudiButton[^>]*label="Gửi bình luận"/, "nút gửi bình luận tường hiện khi ô còn trống");
});
