import assert from "node:assert/strict";
import { test } from "node:test";

import { DOCK_DAU, chuyen, hienToSau } from "../dist-test/rudi/nep/trang-thai.js";

/** Nếp after the person pulled it out of the edge: the one way it comes to rest outside. */
const RA = chuyen(DOCK_DAU, { kieu: "keo-vao" });

// Nếp starts TUCKED. Text reaches the page margin (message times end exactly
// 16dp from the right edge), so a Nếp resting outside the margin covers words
// at some scroll offset; the first cut rested as a 57dp disc and did, and its
// tap area caught the «Đồng ý» of an invitation (flow 25). So the default is
// the slip's edge in the margin, and «out» is something the person does.

test("Nếp bắt đầu cài trong mép, không có việc, không lui", () => {
  assert.deepEqual(DOCK_DAU, { trangThai: "an", nen: "an", coViec: false, luiLai: false, nhuongCho: false });
});

test("chạm mép chỉ kéo Nếp ra, chưa mở bảng; chạm lần nữa mới mở", () => {
  const ra = chuyen(DOCK_DAU, { kieu: "cham" });
  assert.equal(ra.trangThai, "nghi");
  assert.equal(ra.nen, "nghi", "người dùng đã kéo ra thì đó là lựa chọn của họ");
  assert.equal(chuyen(ra, { kieu: "cham" }).trangThai, "mo");
});

test("vuốt ra thì Nếp nằm gọn lại trong mép, và mép là nền để quay về", () => {
  const an = chuyen(RA, { kieu: "vuot-ra" });
  assert.deepEqual(an, { trangThai: "an", nen: "an", coViec: false, luiLai: false, nhuongCho: false });
  assert.equal(chuyen(an, { kieu: "keo-vao" }).trangThai, "nghi");
});

test("đóng bảng thì Nếp về đúng nền trước đó, không tự hiện ra", () => {
  const moTuRa = chuyen(RA, { kieu: "cham" });
  assert.equal(moTuRa.trangThai, "mo");
  assert.equal(moTuRa.nen, "nghi");
  assert.equal(chuyen(moTuRa, { kieu: "dong" }).trangThai, "nghi");

  const moTuAn = { trangThai: "mo", nen: "an", coViec: false, luiLai: false, nhuongCho: false };
  assert.equal(chuyen(moTuAn, { kieu: "dong" }).trangThai, "an");
});

test("báo việc chỉ ghi nhận; Nếp đang cài thì KHÔNG tự bước ra", () => {
  const chiBao = chuyen(DOCK_DAU, { kieu: "bao-viec" });
  assert.equal(chiBao.coViec, true);
  assert.equal(chiBao.trangThai, "an", "Nếp vẫn cài, chỉ tờ thứ hai hiện");
  assert.equal(hienToSau(chiBao), true);
});

// Nếp never widens over the page by itself. The four-second line that used
// to announce work on a pulled-out slip lay over a card's price and hours on
// the web build (24/09); «không bao giờ che nội dung» has no exception for it.
test("Nếp đã đứng ngoài mà có việc thì vẫn đứng yên đúng chỗ, không tự bung ra một dòng", () => {
  const coViec = chuyen(RA, { kieu: "bao-viec" });
  assert.equal(coViec.trangThai, "nghi");
  assert.equal(coViec.coViec, true);
  assert.equal(hienToSau(coViec), true, "việc được nói bằng tờ thứ hai, không bằng chữ đè lên trang");

  const xong = chuyen(coViec, { kieu: "xong-viec" });
  assert.equal(xong.coViec, false);
  assert.equal(xong.trangThai, "nghi");
});

test("không trạng thái nào rộng hơn Nếp đã ra mà đến được khi người dùng không chạm", () => {
  const khongCham = ["vuot-ra", "keo-vao", "dong", "bao-viec", "xong-viec"];
  let dock = RA;
  for (const kieu of khongCham) for (const nepLui of [false, true]) {
    dock = chuyen(dock, { kieu });
    dock = chuyen(dock, { kieu: "doi-man", nepLui });
    assert.ok(["an", "nghi"].includes(dock.trangThai), `${kieu} → ${dock.trangThai}`);
  }
});

test("Luật Nếp Đứng Xa Tiền: màn tiền ép Nếp lui vào mép", () => {
  const oManTien = chuyen(RA, { kieu: "doi-man", nepLui: true });
  assert.equal(oManTien.trangThai, "an");
  assert.equal(oManTien.luiLai, true);

  const coViecOManTien = chuyen(oManTien, { kieu: "bao-viec" });
  assert.equal(coViecOManTien.trangThai, "an", "màn tiền thì Nếp tuyệt đối không bước ra");
  assert.equal(coViecOManTien.coViec, true, "vẫn ghi nhận có việc, chỉ là không nói ra");
});

test("rời màn tiền thì Nếp trả về đúng ý người dùng trước đó, không tự bật lên", () => {
  const quaManTien = chuyen(DOCK_DAU, { kieu: "doi-man", nepLui: true });
  const roiManTien = chuyen(quaManTien, { kieu: "doi-man", nepLui: false });
  assert.equal(roiManTien.trangThai, "an", "Nếp đang cài thì vẫn cài");
  assert.equal(roiManTien.luiLai, false);

  const roiManTienKhiDaRa = chuyen(chuyen(RA, { kieu: "doi-man", nepLui: true }), { kieu: "doi-man", nepLui: false });
  assert.equal(roiManTienKhiDaRa.trangThai, "nghi", "người đã kéo Nếp ra thì Nếp trở ra");
});

test("đang mở bảng mà vào màn tiền thì bảng đóng lại, không đứng cạnh số tiền", () => {
  const dangMo = chuyen(RA, { kieu: "cham" });
  assert.equal(dangMo.trangThai, "mo");
  assert.equal(chuyen(dangMo, { kieu: "doi-man", nepLui: true }).trangThai, "an");
});

// «Chừa một chỗ cho nhau.» When the page lays another sheet over itself -- a
// tray, a bottom sheet -- Nếp tucks back into the notebook edge and leaves the
// room to it. This is the rule that stops the dock covering the words of the
// sheet the person is reading, and it is written as a character trait rather
// than a z-index because that is what it is: the first scene ever drawn of
// Nếp is Nếp pulling out a chair for someone else.

test("nhường chỗ: trang mở một tờ khác thì Nếp rút vào mép, tờ đóng thì trở ra đúng chỗ cũ", () => {
  const nhuong = chuyen(RA, { kieu: "nhuong-cho", bat: true });
  assert.equal(nhuong.trangThai, "an", "Nếp rút vào mép sổ");
  assert.equal(nhuong.nen, "nghi", "nhường chỗ không phải lựa chọn của người dùng, nên nền không đổi");
  assert.equal(nhuong.nhuongCho, true);

  const traLai = chuyen(nhuong, { kieu: "nhuong-cho", bat: false });
  assert.equal(traLai.trangThai, "nghi", "tờ kia đóng thì Nếp trở ra đúng chỗ người dùng đã để");
  assert.equal(traLai.nhuongCho, false);
});

test("Nếp đang cài thì sau khi nhường chỗ vẫn cài", () => {
  const sau = chuyen(chuyen(DOCK_DAU, { kieu: "nhuong-cho", bat: true }), { kieu: "nhuong-cho", bat: false });
  assert.equal(sau.trangThai, "an");
  assert.equal(sau.nen, "an");
});

test("đang nhường chỗ thì chạm vào mép không lôi Nếp ra đè lên tờ đang mở", () => {
  const nhuong = chuyen(RA, { kieu: "nhuong-cho", bat: true });
  assert.equal(chuyen(nhuong, { kieu: "cham" }).trangThai, "an");
  assert.equal(chuyen(nhuong, { kieu: "keo-vao" }).trangThai, "an");
});

test("đang nhường chỗ thì có việc vẫn được ghi nhận nhưng Nếp không bước ra", () => {
  const nhuong = chuyen(RA, { kieu: "nhuong-cho", bat: true });
  const coViec = chuyen(nhuong, { kieu: "bao-viec" });
  assert.equal(coViec.trangThai, "an", "bước ra lúc này là đè lên tờ người ta đang đọc");
  assert.equal(coViec.coViec, true, "việc không bị nuốt, chỉ đợi");
});

test("bảng của chính Nếp không làm Nếp nhường chỗ cho chính mình", () => {
  const dangMo = chuyen(RA, { kieu: "cham" });
  assert.equal(dangMo.trangThai, "mo");
  const sau = chuyen(dangMo, { kieu: "nhuong-cho", bat: true });
  assert.equal(sau.trangThai, "mo");
});

test("nhả tờ ra khi đang ở màn tiền thì Nếp vẫn lùi, vì luật tiền mạnh hơn", () => {
  const oManTien = chuyen(RA, { kieu: "doi-man", nepLui: true });
  const sau = chuyen(chuyen(oManTien, { kieu: "nhuong-cho", bat: true }), { kieu: "nhuong-cho", bat: false });
  assert.equal(sau.trangThai, "an");
  assert.equal(sau.luiLai, true);
});

test("đổi màn trong lúc còn tờ đang mở không bật Nếp ra đè lên tờ đó", () => {
  const nhuong = chuyen(RA, { kieu: "nhuong-cho", bat: true });
  const doiMan = chuyen(nhuong, { kieu: "doi-man", nepLui: false });
  assert.equal(doiMan.trangThai, "an", "tờ chưa đóng thì Nếp chưa trở ra");
  assert.equal(chuyen(doiMan, { kieu: "nhuong-cho", bat: false }).trangThai, "nghi");
});

// The second slip is Nếp SAYING there is work. The work is recorded wherever
// the person stands; the slip shows only where Nếp may speak (ADR-0033 §2:
// «việc vẫn được ghi nhận, chỉ là không nói ra»). A finish review caught the
// first cut drawing it on money screens and beside an open tray.

test("tờ thứ hai hiện khi có việc ở màn bình thường, cả lúc Nếp cài lẫn lúc đã ra", () => {
  assert.equal(hienToSau(chuyen(DOCK_DAU, { kieu: "bao-viec" })), true);
  assert.equal(hienToSau(chuyen(RA, { kieu: "bao-viec" })), true);
  assert.equal(hienToSau(DOCK_DAU), false, "không có việc thì không có tờ thứ hai");
});

test("màn tiền: có việc vẫn ghi nhận nhưng tờ thứ hai không hiện", () => {
  const oManTien = chuyen(chuyen(DOCK_DAU, { kieu: "doi-man", nepLui: true }), { kieu: "bao-viec" });
  assert.equal(oManTien.coViec, true);
  assert.equal(hienToSau(oManTien), false, "mép ở màn tiền là mép giấy trơn, không mặt, không lời");
  const roiManTien = chuyen(oManTien, { kieu: "doi-man", nepLui: false });
  assert.equal(hienToSau(roiManTien), true, "rời màn tiền thì việc được nói ra");
});

test("đang có tờ khác mở: tờ thứ hai không hiện, tờ kia đóng thì hiện lại", () => {
  const nhuong = chuyen(chuyen(DOCK_DAU, { kieu: "bao-viec" }), { kieu: "nhuong-cho", bat: true });
  assert.equal(hienToSau(nhuong), false);
  assert.equal(hienToSau(chuyen(nhuong, { kieu: "nhuong-cho", bat: false })), true);
});

test("bảng Nếp đang mở thì không vẽ tờ thứ hai sau lưng nó", () => {
  const moCoViec = chuyen(chuyen(RA, { kieu: "bao-viec" }), { kieu: "cham" });
  assert.equal(moCoViec.trangThai, "mo");
  assert.equal(hienToSau(moCoViec), false);
});
