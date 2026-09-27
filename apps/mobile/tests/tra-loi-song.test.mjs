/* Một câu trả lời AI hiện dần trên màn hình người hỏi (lát 11, hợp đồng §4.1).
 *
 * Ca này chạy đúng mã màn hình dùng (`dist-test/rudi/ai/tra-loi-song.js`,
 * `nep/hoi.js`, `chat/ai-invocations.js`), không chép lại luật:
 *   - bộ đọc SSE + bộ gộp: thứ tự, trùng, nối lại, ping, sự kiện lạ;
 *   - máy trạng thái: đang nghĩ → đang viết → xong/lỗi/huỷ/thu hồi;
 *   - lùi về polling khi stream hỏng hai lần, khi nền tảng không stream được,
 *     khi máy chủ từ chối (503, 429) hoặc đóng mà không có sự kiện kết thúc;
 *   - thay chữ đang chạy bằng câu cuối (Nếp: xong.text + chips; nhóm: thẻ
 *     đã đăng theo message_id);
 *   - Reduce Motion: chữ tới theo từng câu, không theo nhịp 16 rune.
 *
 * Nó KHÔNG chứng minh: rằng expo/fetch trên máy thật đưa body từng mẩu (cần
 * thiết bị), rằng màn hình vẽ đúng (ảnh chụp), hay độ trễ thật.
 */
import assert from "node:assert/strict";
import { test } from "node:test";

import { taoBoDoc } from "../dist-test/rudi/ai/sse.js";
import {
  TRA_LOI_DAU,
  buocTraLoi,
  buocTuHang,
  chuDaViet,
  chuHienThi,
  daKetThuc,
  idSau,
  theoDoiTraLoi,
} from "../dist-test/rudi/ai/tra-loi-song.js";
import { cauTrangThaiNep, ketCucNep } from "../dist-test/rudi/nep/hoi.js";
import { hangTraLoiSong, laTraLoiDangCho } from "../dist-test/rudi/chat/ai-invocations.js";

const enc = new TextEncoder();

/** Folds a whole SSE body, split at every byte, into an answer. */
function gop(than, s = TRA_LOI_DAU) {
  const doc = taoBoDoc();
  for (const ch of than) for (const e of doc.doc(ch)) s = buocTraLoi(s, e);
  return s;
}

const su = (id, loai, data) => `${id ? `id: ${id}\n` : ""}event: ${loai}\ndata: ${JSON.stringify(data)}\n\n`;

/* ------------------------------------------------ bộ đọc + bộ gộp -------- */

test("gộp theo thứ tự id; ping, hello và sự kiện lạ không đổi chữ", () => {
  const than =
    "retry: 2000\n\n" +
    su(null, "hello", { nhip_ms: 15000 }) +
    su("1-0", "trang_thai", { cau: "dang_doc" }) +
    ": ping\n\n" +
    su("1-1", "delta", { p: 0, text: "Đà Lạt tối nay " }) +
    "event: la_lam\ndata: {\"x\":1}\n\n" +
    su("1-2", "phan", { kind: "places", json: "{}" }) +
    su("2-0", "delta", { p: 0, text: "se lạnh." });
  const s = gop(than);
  assert.equal(s.pha, "dang_viet");
  assert.equal(chuDaViet(s), "Đà Lạt tối nay se lạnh.");
  assert.equal(s.idCuoi, "2-0");
  assert.equal(s.trangThai, "dang_doc");
});

test("sự kiện trùng hoặc cũ hơn (phát lại khi nối lại) bị bỏ, không nhân đôi chữ", () => {
  let s = gop(su("5-0", "delta", { p: 0, text: "Một " }) + su("5-1", "delta", { p: 0, text: "hai " }));
  // A reconnect that replays from an older position, then continues.
  s = gop(su("5-0", "delta", { p: 0, text: "Một " }) + su("5-1", "delta", { p: 0, text: "hai " }) + su("5-2", "delta", { p: 0, text: "ba." }), s);
  assert.equal(chuDaViet(s), "Một hai ba.");
  // Ids compare as numbers, not strings: 10-0 is after 9-0.
  assert.equal(idSau("10-0", "9-0"), true);
  assert.equal(idSau("9-10", "9-9"), true);
  assert.equal(idSau("9-9", "9-9"), false);
  assert.equal(idSau("rác", "1-0"), false);
});

test("delta của phần cũ hơn phần mới nhất bị bỏ; p hỏng bị bỏ; nhiều phần nối bằng dòng trống", () => {
  let s = gop(
    su("1-0", "delta", { p: 0, text: "Phần một." }) +
      su("1-1", "delta", { p: 1, text: "Phần hai." }) +
      su("1-2", "delta", { p: 0, text: "CHEN" }) +
      su("1-3", "delta", { p: -1, text: "x" }) +
      su("1-4", "delta", { p: 1.5, text: "x" }) +
      su("1-5", "delta", { p: 99, text: "x" }) +
      su("1-6", "delta", { p: 1, text: 7 }),
  );
  assert.equal(chuDaViet(s), "Phần một.\n\nPhần hai.");
  assert.equal(s.idCuoi, "1-6", "sự kiện bị bỏ vẫn ghi vị trí nối lại");
  s = buocTraLoi(s, { id: null, loai: "delta", data: { p: 1, text: " Tiếp." } });
  assert.equal(chuDaViet(s), "Phần một.\n\nPhần hai. Tiếp.", "delta không id (hiếm) vẫn được nối");
});

/* ------------------------------------------------ máy trạng thái --------- */

test("đang nghĩ → đang viết → xong: câu niêm phong thay phần đã chạy, sau đó không gì đổi được", () => {
  let s = TRA_LOI_DAU;
  assert.equal(s.pha, "dang_nghi");
  s = buocTraLoi(s, { id: "1-0", loai: "trang_thai", data: { cau: "dang_nghi" } });
  assert.equal(s.trangThai, "dang_nghi");
  s = buocTraLoi(s, { id: "1-1", loai: "delta", data: { p: 0, text: "Chào" } });
  assert.equal(s.pha, "dang_viet");
  s = buocTraLoi(s, { id: "1-2", loai: "trang_thai", data: { cau: "dang_doc" } });
  assert.equal(s.trangThai, "dang_nghi", "trạng thái tới sau chữ không kéo màn hình về «đang nghĩ»");
  assert.equal(s.pha, "dang_viet");
  s = buocTraLoi(s, { id: "1-3", loai: "xong", data: { text: "Chào bạn.", chips: ["Mở bản đồ", 3], nguon: [] } });
  assert.equal(s.pha, "xong");
  assert.equal(chuDaViet(s), "Chào bạn.");
  assert.deepEqual(s.ketThuc.chips, ["Mở bản đồ"]);
  assert.equal(daKetThuc(s), true);
  const sau = buocTraLoi(s, { id: "9-0", loai: "delta", data: { p: 0, text: "rò" } });
  assert.equal(sau, s, "delta sau kết thúc không mở lại câu trả lời");
});

test("lam_lai về «đang nghĩ» và bỏ phần đã hiện; that_bai giữ chữ; thu_hoi ẩn chữ; huy", () => {
  let s = gop(su("1-0", "delta", { p: 0, text: "Nháp" }) + su("1-1", "lam_lai", {}));
  assert.equal(s.pha, "dang_nghi");
  assert.equal(chuDaViet(s), "");
  const loi = gop(su("2-0", "delta", { p: 0, text: "Đã nhả một đoạn" }) + su("2-1", "that_bai", { code: "worker_interrupted" }));
  assert.equal(loi.pha, "loi");
  assert.equal(loi.ma, "worker_interrupted");
  assert.equal(chuDaViet(loi), "Đã nhả một đoạn");
  const thu = gop(su("3-0", "delta", { p: 0, text: "Bí mật" }) + su("3-1", "thu_hoi", {}));
  assert.equal(thu.pha, "thu_hoi");
  assert.equal(chuDaViet(thu), "");
  assert.equal(gop(su("4-0", "huy", {})).pha, "huy");
});

test("hàng đọc được (polling) chỉ đổi gì khi lời gọi đã kết thúc", () => {
  const dang = gop(su("1-0", "delta", { p: 0, text: "Đang" }));
  assert.equal(buocTuHang(dang, { status: "running", code: null }), dang);
  const xong = buocTuHang(dang, { status: "succeeded", code: null, text: "Đang viết xong." });
  assert.equal(chuDaViet(xong), "Đang viết xong.");
  const xongNhom = buocTuHang(TRA_LOI_DAU, { status: "succeeded", code: null, message_id: "m-1" });
  assert.equal(xongNhom.ketThuc.messageId, "m-1");
  assert.equal(buocTuHang(dang, { status: "failed", code: "ai_tu_choi" }).ma, "ai_tu_choi");
  assert.equal(buocTuHang(dang, { status: "cancelled", code: null }).pha, "huy");
});

/* ------------------------------------------------ theo dõi + lùi về polling */

function traLoiSSE(status, chunks = [], { moMai = false } = {}) {
  const body = new ReadableStream({
    start(c) {
      for (const ch of chunks) c.enqueue(enc.encode(ch));
      // moMai: the connection stays open and silent (a black-holed link).
      if (!moMai) c.close();
    },
  });
  return { status, ok: status >= 200 && status < 300, body };
}

/** Runs the follower with scripted fetch answers, poll rows and hand-run timers. */
function chay({ url = "http://x/events", answers = [], hang = [], viTriQua } = {}) {
  const goi = [];
  const hoi = [];
  const timers = [];
  // The follow's one overall deadline (choToiDaMs) is kept apart from the
  // reconnect and poll timers these tests step through one by one.
  const hanChung = [];
  const trangThai = [];
  let hetGio = 0;
  const theo = theoDoiTraLoi({
    url,
    headers: { Authorization: "Bearer t" },
    viTriQua,
    hoi: async () => {
      hoi.push(1);
      const h = hang.shift();
      if (h instanceof Error) throw h;
      return h ?? { status: "running", code: null };
    },
    nhipHoi: (n) => 800 + n,
    choToiDaMs: 90_000,
    khiDoi: (s) => trangThai.push(s),
    khiHetGio: () => hetGio++,
    fetchImpl: async (u, init) => {
      goi.push({ url: u, headers: init.headers });
      const a = answers.shift();
      if (a instanceof Error) throw a;
      if (!a) return new Promise(() => {});
      return a;
    },
    hen: (fn, ms) => (ms === 90_000 ? (hanChung.push({ fn, ms, song: true }), -hanChung.length) : (timers.push({ fn, ms }), timers.length)),
    boHen: (h) => {
      if (typeof h === "number" && h < 0) hanChung[-h - 1].song = false;
    },
    ngauNhien: () => 0.5,
  });
  const nghi = () => new Promise((r) => setTimeout(r, 5));
  const chayHen = async () => {
    const t = timers.shift();
    t.fn();
    await nghi();
    return t;
  };
  return { theo, goi, hoi, timers, hanChung, trangThai, nghi, chayHen, hetGio: () => hetGio };
}

test("stream hỏng hai lần liền thì nhường cho polling, và polling đưa câu trả lời về", async () => {
  const r = chay({ answers: [new Error("mạng"), new Error("mạng")], hang: [{ status: "running", code: null }, { status: "succeeded", code: null, text: "Xong rồi." }] });
  await r.nghi();
  assert.equal(r.goi.length, 1);
  await r.chayHen(); // reconnect after the first failure
  assert.equal(r.goi.length, 2, "hỏng lần hai");
  assert.equal(r.hoi.length, 0);
  assert.equal(r.theo.trangThai().hoiThay, "loi-lap-lai");
  const t = await r.chayHen();
  assert.equal(t.ms, 800, "polling bắt đầu ở nhịp của màn hình");
  assert.equal(r.hoi.length, 1);
  await r.chayHen();
  assert.equal(r.theo.trangThai().pha, "xong");
  assert.equal(chuDaViet(r.theo.trangThai()), "Xong rồi.");
  assert.equal(r.timers.length, 0, "kết thúc thì thôi đọc");
  assert.equal(r.goi.length, 2, "không mở lại stream sau khi đã lùi về polling");
});

test("nền tảng không stream được thì đọc bằng polling ngay, không gọi stream", async () => {
  const r = chay({ url: null, hang: [{ status: "failed", code: "provider_unavailable" }] });
  await r.nghi();
  assert.equal(r.goi.length, 0);
  assert.equal(r.theo.trangThai().hoiThay, "khong-ho-tro");
  await r.chayHen();
  assert.equal(r.theo.trangThai().pha, "loi");
  assert.equal(r.theo.trangThai().ma, "provider_unavailable");
});

for (const status of [503, 429, 404]) {
  test(`máy chủ trả ${status} thì lùi về polling, không thử stream lại`, async () => {
    const r = chay({ answers: [traLoiSSE(status)] });
    await r.nghi();
    assert.equal(r.goi.length, 1);
    assert.equal(r.timers.length, 1);
    assert.equal(r.timers[0].ms, 800, "hẹn kế tiếp là một lần đọc polling, không phải nối lại stream");
    assert.ok(r.theo.trangThai().hoiThay);
  });
}

test("stream kết thúc bằng xong thì đóng, không polling", async () => {
  const r = chay({
    answers: [traLoiSSE(200, [su("1-0", "trang_thai", { cau: "dang_nghi" }), su("1-1", "delta", { p: 0, text: "A b." }), su("1-2", "xong", { text: "A b.", chips: [], nguon: [] })])],
  });
  await r.nghi();
  assert.equal(r.theo.trangThai().pha, "xong");
  assert.equal(r.hoi.length, 0);
  assert.equal(r.timers.length, 0);
  assert.deepEqual(r.trangThai.map((s) => s.pha), ["dang_nghi", "dang_viet", "xong"]);
});

test("đứt giữa chừng thì nối lại mang Last-Event-ID (native) hoặc ?after= (web), không nhân đôi chữ", async () => {
  for (const viTriQua of ["header", "query"]) {
    const r = chay({
      viTriQua,
      answers: [
        traLoiSSE(200, [su("7-0", "delta", { p: 0, text: "Một " })]),
        traLoiSSE(200, [su("7-1", "delta", { p: 0, text: "hai." }), su("7-2", "xong", { text: "Một hai.", chips: [], nguon: [] })]),
      ],
    });
    await r.nghi();
    await r.chayHen();
    const lan2 = r.goi[1];
    if (viTriQua === "header") {
      assert.equal(lan2.headers["Last-Event-ID"], "7-0");
      assert.equal(lan2.url, "http://x/events");
    } else {
      assert.equal(lan2.headers["Last-Event-ID"], undefined);
      assert.equal(lan2.url, "http://x/events?after=7-0");
    }
    assert.equal(chuDaViet(r.theo.trangThai()), "Một hai.");
  }
});

test("dong() dừng mọi thứ: không đọc, không đổi trạng thái nữa", async () => {
  const r = chay({ url: null });
  await r.nghi();
  r.theo.dong();
  const n = r.trangThai.length;
  if (r.timers.length) await r.chayHen();
  assert.equal(r.hoi.length, 0);
  assert.equal(r.trangThai.length, n);
});

/* ------------------------------------------------ thay bằng câu cuối ----- */

test("Nếp: xong.text + chips thay chữ đang chạy; thiếu text thì dùng phần đã chạy; rỗng là không trả lời", () => {
  const dang = gop(su("1-0", "delta", { p: 0, text: "Mở tab " }));
  assert.equal(ketCucNep(dang), null, "còn đang viết thì chưa có kết cục");
  const xong = buocTraLoi(dang, { id: "1-1", loai: "xong", data: { text: "Mở tab Kèo rồi bấm Tạo.", chips: ["Tạo kèo"], nguon: [] } });
  assert.deepEqual(ketCucNep(xong), { kieu: "tra-loi", chu: "Mở tab Kèo rồi bấm Tạo.", chips: ["Tạo kèo"] });
  const khongText = buocTraLoi(gop(su("2-0", "delta", { p: 0, text: "Đủ rồi." })), { id: "2-1", loai: "xong", data: { text: null, chips: [], nguon: [] } });
  assert.equal(ketCucNep(khongText).chu, "Đủ rồi.");
  const rong = buocTraLoi(TRA_LOI_DAU, { id: "3-0", loai: "xong", data: { text: null } });
  assert.equal(ketCucNep(rong).kieu, "loi");
  const ngat = gop(su("4-0", "delta", { p: 0, text: "Một nửa" }) + su("4-1", "that_bai", { code: "worker_interrupted" }));
  assert.deepEqual(ketCucNep(ngat), { kieu: "loi", cau: "Nếp bị ngắt giữa chừng. Bạn hỏi lại nhé.", conLai: "Một nửa" });
  assert.equal(ketCucNep(gop(su("5-0", "delta", { p: 0, text: "x" }) + su("5-1", "thu_hoi", {}))).conLai, null);
});

test("Nếp: câu «đang nghĩ» theo mã trạng thái, mã lạ và dang_xep_hang đọc là đang nghĩ", () => {
  assert.equal(cauTrangThaiNep("dang_doc"), "Nếp đang đọc câu hỏi…");
  assert.equal(cauTrangThaiNep("dang_nghi"), "Nếp đang nghĩ…");
  assert.equal(cauTrangThaiNep("dang_xep_hang"), "Nếp đang nghĩ…");
  assert.equal(cauTrangThaiNep(null), "Nếp đang nghĩ…", "đúng câu bảng Nếp vẫn in trước lát 11");
});

test("Nhóm: câu đang đọc → chữ đang chạy → nhường cho thẻ thật khi message_id đã có trong luồng", () => {
  const request = { id: "inv-1", status: "running", code: null, message_id: null, trigger_message_id: "t-1", so_tin_doc: 3, created_at: "a", updated_at: "a" };
  assert.equal(laTraLoiDangCho(request), true);
  assert.equal(laTraLoiDangCho({ ...request, trigger_message_id: null }), false);
  assert.equal(laTraLoiDangCho({ ...request, status: "succeeded" }), false);
  const khong = () => false;
  assert.deepEqual(hangTraLoiSong(request, TRA_LOI_DAU, "", khong), {
    kieu: "nghi",
    tieuDe: "Rủ Đi AI đang đọc 3 tin…",
    cau: "Câu trả lời sẽ hiện ngay dưới tin của bạn.",
  });
  const dang = gop(su("1-0", "delta", { p: 0, text: "Quán A mở tới 22 giờ." }));
  assert.deepEqual(hangTraLoiSong(request, dang, chuDaViet(dang), khong), { kieu: "viet", chu: "Quán A mở tới 22 giờ." });
  const xong = buocTraLoi(dang, { id: "1-1", loai: "xong", data: { message_id: "m-9" } });
  assert.equal(hangTraLoiSong(request, xong, chuDaViet(xong), khong).kieu, "viet", "thẻ chưa tới thì chữ vẫn ở đó, không chớp mất");
  assert.equal(hangTraLoiSong(request, xong, chuDaViet(xong), (id) => id === "m-9").kieu, "an", "thẻ đã có thì hàng chờ nhường chỗ");
  assert.equal(hangTraLoiSong(request, gop(su("2-0", "that_bai", { code: "ai_tu_choi" })), "", khong).kieu, "an", "thất bại để hàng polling có «Thử lại» nói");
});

/* ------------------------------------------------ Reduce Motion ---------- */

test("Reduce Motion: chữ tới theo từng câu trọn, cả câu khi xong; không giảm thì hiện hết", () => {
  const dang = gop(su("1-0", "delta", { p: 0, text: "Câu một xong. Câu hai đang " }));
  assert.equal(chuHienThi(dang, false), "Câu một xong. Câu hai đang ");
  assert.equal(chuHienThi(dang, true), "Câu một xong.");
  const chuaCau = gop(su("2-0", "delta", { p: 0, text: "Chưa hết câu" }));
  assert.equal(chuHienThi(chuaCau, true), "", "chưa trọn câu nào thì vẫn là «đang nghĩ»");
  const soThapPhan = gop(su("3-0", "delta", { p: 0, text: "Giá 1.5 triệu" }));
  assert.equal(chuHienThi(soThapPhan, true), "", "dấu chấm giữa số không phải hết câu");
  const xuongDong = gop(su("4-0", "delta", { p: 0, text: "Dòng một\nDòng hai" }));
  assert.equal(chuHienThi(xuongDong, true), "Dòng một");
  const xong = buocTraLoi(chuaCau, { id: "2-1", loai: "xong", data: { text: "Chưa hết câu mà đã xong" } });
  assert.equal(chuHienThi(xong, true), "Chưa hết câu mà đã xong");
});

/* ------------------------------------------------ hạn chung ------------- */

test("stream còn «đang làm» trên kết nối chết vẫn hết hạn chung và báo nghĩ lâu quá", async () => {
  // The connection answers and then never sends another byte: no FIN, no
  // ping. The follow's deadline is armed while streaming, not only once
  // polling took over (review of slices 9/11, mobile minor 3.2).
  const r = chay({ answers: [traLoiSSE(200, [su("1-0", "trang_thai", { cau: "dang_nghi" })], { moMai: true })] });
  await r.nghi();
  assert.equal(r.theo.trangThai().pha, "dang_nghi");
  assert.equal(r.hoi.length, 0, "vẫn đang đọc stream, chưa polling");
  assert.equal(r.hanChung.length, 1);
  assert.equal(r.hanChung[0].ms, 90_000);
  r.hanChung[0].fn();
  await r.nghi();
  assert.equal(r.hetGio(), 1, "hết hạn thì màn nói nghĩ lâu quá");
  assert.equal(r.theo.trangThai().pha, "dang_nghi");
});

test("xong trước hạn thì hạn chung được huỷ, không báo hết giờ", async () => {
  const r = chay({ answers: [traLoiSSE(200, [su("1-0", "xong", { text: "A.", chips: [], nguon: [] })])] });
  await r.nghi();
  assert.equal(r.theo.trangThai().pha, "xong");
  assert.equal(r.hanChung[0].song, false, "hạn chung phải được gỡ khi đã xong");
  r.hanChung[0].fn();
  assert.equal(r.hetGio(), 0);
});
