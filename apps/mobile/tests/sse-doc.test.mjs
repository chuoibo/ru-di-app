import assert from "node:assert/strict";
import { test } from "node:test";

import { KET_THUC, SO_LAN_HONG_TOI_DA, moLuong, nhipNoiLai, taoBoDoc } from "../dist-test/rudi/ai/sse.js";

const enc = new TextEncoder();

/** A fetch answer whose body arrives in exactly these byte chunks. */
function traLoi(status, chunks) {
  const body = new ReadableStream({
    start(c) {
      for (const ch of chunks) c.enqueue(typeof ch === "string" ? enc.encode(ch) : ch);
      c.close();
    },
  });
  return { status, ok: status >= 200 && status < 300, body };
}

/** Split bytes at every position, the worst chunking a network can do. */
function tungByte(s) {
  return [...enc.encode(s)].map((b) => new Uint8Array([b]));
}

/** Runs moLuong with fetch answers in order and timers run by hand. */
function chayLuong(answers, extra = {}) {
  const calls = [];
  const events = [];
  const timers = [];
  let fallback = null;
  let closed = 0;
  const fetchImpl = async (url, init) => {
    calls.push(init.headers);
    const a = answers.shift();
    if (a instanceof Error) throw a;
    if (!a) return new Promise(() => {});
    return a;
  };
  const luong = moLuong({
    url: "http://x/events",
    headers: { Authorization: "Bearer t" },
    khiSuKien: (e) => events.push(e),
    khiChuyenSangHoi: (l) => (fallback = l),
    khiDong: () => closed++,
    fetchImpl,
    hen: (fn, ms) => (timers.push({ fn, ms }), timers.length),
    boHen: () => {},
    ngauNhien: () => 0.5,
    ...extra,
  });
  const nghi = () => new Promise((r) => setTimeout(r, 5));
  return { calls, events, timers, luong, nghi, get fallback() { return fallback; }, get closed() { return closed; } };
}

test("bộ đọc chịu được mọi cách cắt, kể cả giữa một ký tự UTF-8", async () => {
  const wire = 'id: 1-0\nevent: delta\ndata: {"p":0,"text":"Chào cả nhóm 👋"}\n\n: ping\n\nid: 1-1\nevent: xong\ndata: {"message_id":"m"}\n\n';
  const r = chayLuong([traLoi(200, tungByte(wire))]);
  await r.nghi();
  assert.deepEqual(r.events.map((e) => e.loai), ["delta", "xong"]);
  assert.equal(r.events[0].data.text, "Chào cả nhóm 👋");
  assert.equal(r.events[1].id, "1-1");
  assert.equal(r.closed, 1, "kết thúc thì đóng");
  assert.equal(r.calls.length, 1, "kết thúc thì không nối lại");
});

test("sự kiện ngoài bộ từ vựng và data không phải JSON bị bỏ, không đoán", () => {
  const d = taoBoDoc();
  const ra = d.doc('event: retract\ndata: {}\n\nevent: delta\ndata: not json\n\nevent: delta\ndata: {"text":"a"}\n\n');
  assert.deepEqual(ra.map((e) => e.loai), ["delta"]);
});

test("data nhiều dòng ghép bằng xuống dòng; CRLF và retry được hiểu", () => {
  const d = taoBoDoc();
  const ra = d.doc('retry: 2000\r\n\r\nevent: phan\r\ndata: {"kind":"text",\r\ndata: "json":{}}\r\n\r\n');
  assert.equal(d.retryMs(), 2000);
  assert.equal(ra.length, 1);
  assert.deepEqual(ra[0].data, { kind: "text", json: {} });
});

test("dòng kết thúc bằng \\r nằm ở cuối chunk chờ chunk sau", () => {
  const d = taoBoDoc();
  assert.deepEqual(d.doc('event: delta\rdata: {"text":"x"}\r'), []);
  assert.equal(d.doc("\n\r\n").length, 1);
});

test("stream đứt giữa chừng thì nối lại từ id cuối, không phát lại", async () => {
  const r = chayLuong([
    traLoi(200, ['id: 5-0\nevent: delta\ndata: {"text":"a"}\n\n']),
    traLoi(200, ['id: 5-1\nevent: xong\ndata: {"message_id":"m"}\n\n']),
  ]);
  await r.nghi();
  assert.equal(r.timers.length, 1);
  assert.equal(r.timers[0].ms, 500, "nhận được gì đó thì nối lại ngay ở nấc đầu");
  r.timers[0].fn();
  await r.nghi();
  assert.equal(r.calls[1]["Last-Event-ID"], "5-0");
  assert.deepEqual(r.events.map((e) => e.loai), ["delta", "xong"]);
});

test("máy chủ bảo nối lại (ket_noi_lai) thì nối lại đúng lúc, mang id cuối", async () => {
  const r = chayLuong([
    traLoi(200, ['id: 7-0\nevent: delta\ndata: {"text":"a"}\n\nevent: ket_noi_lai\ndata: {"sau_ms":0}\n\n']),
    traLoi(200, ['id: 7-1\nevent: xong\ndata: {}\n\n']),
  ]);
  await r.nghi();
  assert.equal(r.timers[0].ms, 0);
  assert.ok(!r.events.some((e) => e.loai === "ket_noi_lai"), "ket_noi_lai không lọt ra màn");
  r.timers[0].fn();
  await r.nghi();
  assert.equal(r.calls[1]["Last-Event-ID"], "7-0");
});

test("bị từ chối (401, 403, 404) thì dừng hẳn, không hỏi lại", async () => {
  for (const status of [401, 403, 404]) {
    const r = chayLuong([traLoi(status, [])]);
    await r.nghi();
    assert.equal(r.closed, 1, String(status));
    assert.equal(r.timers.length, 0, String(status));
    assert.equal(r.fallback, null, String(status));
  }
});

test("503 chuyển ngay sang hỏi lại bằng polling", async () => {
  const r = chayLuong([traLoi(503, [])]);
  await r.nghi();
  assert.equal(r.fallback, "may-chu-tu-choi");
});

test("hỏng liên tiếp đủ số lần thì nhường cho polling", async () => {
  const answers = Array.from({ length: SO_LAN_HONG_TOI_DA }, () => new Error("mạng"));
  const r = chayLuong(answers);
  for (let i = 0; i < SO_LAN_HONG_TOI_DA - 1; i++) {
    await r.nghi();
    r.timers[i].fn();
  }
  await r.nghi();
  assert.equal(r.fallback, "loi-lap-lai");
  assert.equal(r.calls.length, SO_LAN_HONG_TOI_DA);
});

test("không đọc được body dạng stream thì nói rõ, không giả vờ", async () => {
  const r = chayLuong([{ status: 200, ok: true, body: null }]);
  await r.nghi();
  assert.equal(r.fallback, "khong-ho-tro");
});

test("nhịp nối lại tăng dần, có trần 15 giây", () => {
  assert.equal(nhipNoiLai(0, () => 0.5), 500);
  assert.equal(nhipNoiLai(1, () => 0.5), 1000);
  assert.equal(nhipNoiLai(20, () => 0.5), 15000);
  assert.equal(nhipNoiLai(20, () => 1), 18000);
  assert.equal(nhipNoiLai(-3, () => 0.5), 500);
});

test("bốn sự kiện kết thúc đúng hợp đồng", () => {
  assert.deepEqual([...KET_THUC].sort(), ["huy", "that_bai", "thu_hoi", "xong"]);
});
