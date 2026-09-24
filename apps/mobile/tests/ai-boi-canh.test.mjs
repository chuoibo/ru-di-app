/**
 * What actually leaves the phone when someone asks the AI.
 *
 * Every case below is a privacy property, not a formatting preference. The
 * server holds no key and cannot check any of this; the words under the send
 * button promise it; so this file is the only place it is true.
 */
import assert from "node:assert/strict";
import test from "node:test";
import { cauBoiCanh, catTheoByte, ganNgan, soByte, vanTay } from "../dist-test/rudi/ai/boi-canh.js";
import { gomBoiCanhChat } from "../dist-test/rudi/chat/boi-canh-chat.js";

const toi = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const ban = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";
const banHai = "cccccccc-cccc-4ccc-8ccc-cccccccccccc";

/** Rows arrive newest first, the way an inverted list holds them. */
const tin = (id, extra = {}) => ({
  id,
  context_id: "dddddddd-dddd-4ddd-8ddd-dddddddddddd",
  author_id: ban,
  kind: "text",
  body: `Dòng ${id}`,
  image_url: null,
  card: null,
  cursor: id,
  created_at: `2030-09-22T10:00:0${id}Z`,
  ...extra,
});

const gom = (rows, opts = {}) => gomBoiCanhChat({ tin: rows, personId: toi, ...opts });

test("Bối cảnh đi theo thứ tự đọc: cũ trước, mới sau, ngược với danh sách đảo trên màn", () => {
  // The screen holds newest first. A model reads a conversation forwards, and
  // handing it a reversed one is a quality fault nobody can see on screen.
  const bc = gom([tin("3"), tin("2"), tin("1")]);
  assert.deepEqual(bc.luot.map((l) => l.chu), ["Dòng 1", "Dòng 2", "Dòng 3"]);
});

test("Tin đã xoá vẫn là một lượt, và không bao giờ mang lại nội dung cũ", () => {
  // Still a row on screen, so dropping it would splice two unrelated turns
  // into an exchange that never happened. But a server that left `body` behind
  // must not be able to leak it through here.
  const bc = gom([tin("2", { kind: "deleted", body: "Câu đã bị xoá" }), tin("1")]);
  const daXoa = bc.luot.find((l) => l.loai === "da-xoa");
  assert.equal(daXoa.chu, "Tin nhắn đã bị xoá");
  assert.ok(!JSON.stringify(bc).includes("Câu đã bị xoá"));
});

test("Ảnh đi bằng chú thích; image_url không bao giờ lên dây", () => {
  // An image URL is an authorised read route. Handing one to a server-side
  // model hands over an entry point, not a picture.
  const bc = gom([
    tin("2", { kind: "image", image_url: "/media/anh-rieng-cua-nhom.jpg", body: "Bữa tối hôm qua" }),
    tin("1", { kind: "image", image_url: "/media/khong-chu-thich.jpg", body: null }),
  ]);
  const day = JSON.stringify(bc);
  assert.ok(!day.includes("/media/"));
  assert.deepEqual(bc.luot.map((l) => l.chu), ["Ảnh", "Ảnh: Bữa tối hôm qua"]);
});

test("Sticker và thẻ đi bằng một nhãn, không đi bằng ruột thẻ", () => {
  const the = {
    kind: "poll",
    payload: { vote_id: "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee", question: "Tối nay ăn gì?", options: [{ id: "1", label: "Lẩu" }, { id: "2", label: "Nướng" }] },
  };
  const bc = gom([tin("2", { kind: "ai_card", author_id: null, body: null, card: the }), tin("1", { kind: "sticker", body: "cho-ti" })]);
  assert.equal(bc.luot[0].chu, "Sticker");
  assert.equal(bc.luot[1].chu, "Thẻ bình chọn: Tối nay ăn gì?");
  // A sticker id is this app's private vocabulary; a model reads nothing from it.
  assert.ok(!JSON.stringify(bc).includes("cho-ti"));
});

test("Thẻ tờ hẹn không mang theo ngân sách", () => {
  // Money the person typed into the prompt is their own choice. Money leaking
  // out of a card is automatic, and the two are not the same thing.
  const the = { kind: "itinerary", payload: { title: "Đà Lạt hai ngày", budget_per_person_vnd: 1234567, stops: [] } };
  const bc = gom([tin("1", { kind: "ai_card", author_id: null, body: null, card: the })]);
  assert.ok(!JSON.stringify(bc).includes("1234567"));
});

test("Id tài khoản không lên dây; người khác mang tên hiển thị, ổn định trong MỘT gói", () => {
  // ADR-0034 §5: the label is the name the room already sees above each bubble.
  const ten = { [ban]: "Lan", [banHai]: "Huy", [toi]: "Nam" };
  const bc = gom([tin("4", { author_id: ban }), tin("3", { author_id: ban }), tin("2", { author_id: banHai }), tin("1", { author_id: toi })], {
    tenCua: (id) => ten[id],
  });
  assert.deepEqual(bc.luot.map((l) => l.vai), ["toi", "ban", "ban", "ban"]);
  assert.deepEqual(bc.luot.map((l) => l.biDanh), [undefined, "Huy", "Lan", "Lan"]);
  const day = JSON.stringify(bc);
  assert.ok(!day.includes(ban) && !day.includes(banHai) && !day.includes(toi));
  // The caller is `vai: "toi"`; their own name is the server's to lay on.
  assert.ok(!day.includes("Nam"));
});

test("Hai người trùng tên là hai nhãn, theo thứ tự xuất hiện; tên chưa biết lùi về «Bạn N»", () => {
  const la = "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee";
  const ten = { [ban]: "Lan", [banHai]: " Lan " };
  const bc = gom([tin("3", { author_id: la }), tin("2", { author_id: banHai }), tin("1", { author_id: ban })], {
    tenCua: (id) => ten[id],
  });
  assert.deepEqual(bc.luot.map((l) => l.biDanh), ["Lan", "Lan (2)", "Bạn 1"]);
  // Without a name source at all, every friend is still a distinct «Bạn N».
  const khongTen = gom([tin("2", { author_id: banHai }), tin("1", { author_id: ban })]);
  assert.deepEqual(khongTen.luot.map((l) => l.biDanh), ["Bạn 1", "Bạn 2"]);
});

test("Mỗi lượt mang id tin để máy chủ kiểm được, nhưng id người thì không", () => {
  // The id is what lets the server confirm every turn is a real message of this
  // room before it publishes a card saying «đã đọc N tin» to everyone else.
  // Account ids stay off the wire; a message id is something the server already
  // owns and the model never sees.
  const bc = gom([tin("2"), tin("1")]);
  assert.deepEqual(bc.luot.map((l) => l.id), ["1", "2"]);
  assert.ok(!JSON.stringify(bc).includes(ban));
});

test("Vượt hạn byte thì bỏ lượt CŨ nhất, không đụng lượt mới nhất", () => {
  // The newest turn is the one the person was looking at when they pressed
  // send. It is the last thing that may be touched.
  const rows = ["5", "4", "3", "2", "1"].map((id) => tin(id, { body: `Câu số ${id} ${"dài".repeat(40)}` }));
  const bc = gom(rows, { hanByte: 700 });
  assert.ok(bc.daCat);
  assert.ok(bc.luot.length < 5);
  assert.ok(bc.luot[bc.luot.length - 1].chu.startsWith("Câu số 5"));
  assert.equal(bc.tongLuot, 5, "số tin người dùng đang thấy vẫn phải giữ sau khi cắt");
});

test("Một lượt tự nó quá hạn thì cắt theo byte, không vỡ ký tự nhiều byte", () => {
  const bc = gom([tin("1", { body: "ừ".repeat(200) })], { hanByte: 260 });
  assert.equal(bc.luot.length, 1);
  // Re-encoding a mangled string would produce U+FFFD; round-tripping proves
  // the cut landed on a character boundary.
  const chu = bc.luot[0].chu;
  assert.ok(!chu.includes("�"));
  assert.equal(Buffer.from(chu, "utf8").toString("utf8"), chu);
});

test("catTheoByte cắt đúng trần và không bao giờ vỡ ký tự", () => {
  for (const han of [0, 1, 2, 3, 7, 31]) {
    const ra = catTheoByte("ừ".repeat(40), han);
    assert.ok(soByte(ra) <= han, `han=${han}`);
    assert.ok(!ra.includes("�"), `han=${han}`);
  }
});

test("cauBoiCanh nói đúng ở cả bốn trạng thái, và không viết cứng con số nào", () => {
  assert.match(cauBoiCanh(null), /Chỉ lời nhờ/);
  assert.match(cauBoiCanh(gom([])), /chưa có tin nào/);
  assert.match(cauBoiCanh(gom([tin("2"), tin("1")])), /^2 tin gần nhất bạn đang thấy/);
  const nhieu = Array.from({ length: 50 }, (_, i) => tin(String(i)));
  const cau = cauBoiCanh(gom(nhieu, { soLuot: 5 }));
  assert.match(cau, /^5 tin gần nhất trong 50 tin bạn đang thấy/);
});

test("Vân tay đổi khi bối cảnh đổi — đây là thứ chặn «AI trả lời câu cũ»", () => {
  // Key an attempt on the prompt alone and the same question asked again over
  // NEWER messages collides with the old digest, so the server answers 200
  // with the OLD card and the person believes the AI just read what they just
  // said. Silent, and this is the assertion that stops it.
  const truoc = gom([tin("2"), tin("1")]);
  const sau = gom([tin("3"), tin("2"), tin("1")]);
  assert.notEqual(vanTay(truoc), vanTay(sau));
  assert.equal(vanTay(truoc), vanTay(gom([tin("2"), tin("1")])));
});

test("ganNgan không bao giờ trả về gói rỗng khi còn ít nhất một lượt", () => {
  const bc = gom([tin("1", { body: "x".repeat(500) })], { hanByte: 10 });
  assert.equal(bc.luot.length, 1);
  assert.equal(ganNgan(bc, 10).luot.length, 1);
});
