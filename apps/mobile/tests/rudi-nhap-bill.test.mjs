/* The bill being typed (B4, QA UI-052): kept per group on every change, so the
 * browser's Back/Forward and a reload of the tab bring it back instead of an
 * empty step 1; gone once it is recorded or the person drops it.
 *
 * Run from apps/mobile:
 *     npx tsc -p tsconfig.test.json && node --test tests/rudi-nhap-bill.test.mjs
 */
import assert from "node:assert/strict";
import test from "node:test";

const MODULE = new URL("../dist-test/rudi/chia-bill/nhap-bill.js", import.meta.url).href;
// A fresh module instance is what a reload of the tab is: the memory copy is gone.
const taiLai = (lan) => import(`${MODULE}?tai-lai=${lan}`);

function khoPhien() {
  const m = new Map();
  return { getItem: (k) => (m.has(k) ? m.get(k) : null), setItem: (k, v) => m.set(k, String(v)), removeItem: (k) => m.delete(k), _m: m };
}

const NHOM_A = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const NHOM_B = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";
const NHAP = { buoc: "sua", reading: { lines: [{ id: "l1", name: "Lẩu gà", quantity: 1, lineTotalVnd: 360000 }] } };

test("trên web: bản nháp sống qua một lần tải lại tab, theo đúng nhóm", async () => {
  const kho = khoPhien();
  globalThis.sessionStorage = kho;
  try {
    const truoc = await taiLai(1);
    truoc.luuNhapBill(NHOM_A, NHAP);
    const sau = await taiLai(2);
    assert.deepEqual(sau.docNhapBill(NHOM_A), NHAP);
    assert.equal(sau.docNhapBill(NHOM_B), null, "bản nháp của nhóm này không sang nhóm khác");
    sau.boNhapBill(NHOM_A);
    assert.equal((await taiLai(3)).docNhapBill(NHOM_A), null, "bỏ rồi thì tải lại không còn");
    assert.equal(kho._m.size, 0);
  } finally {
    delete globalThis.sessionStorage;
  }
});

test("không có sessionStorage (điện thoại): giữ trong bộ nhớ suốt đời app, không ném", async () => {
  const m = await taiLai(4);
  m.luuNhapBill(NHOM_A, NHAP);
  assert.deepEqual(m.docNhapBill(NHOM_A), NHAP);
  m.boNhapBill(NHOM_A);
  assert.equal(m.docNhapBill(NHOM_A), null);
});

test("kho bị chặn hay đầy: vẫn giữ bản trong bộ nhớ", async () => {
  globalThis.sessionStorage = { getItem() { throw new Error("blocked"); }, setItem() { throw new Error("quota"); }, removeItem() { throw new Error("blocked"); } };
  try {
    const m = await taiLai(5);
    m.luuNhapBill(NHOM_A, NHAP);
    assert.deepEqual(m.docNhapBill(NHOM_A), NHAP);
    assert.equal(m.docNhapBill(NHOM_B), null);
    m.boNhapBill(NHOM_A);
  } finally {
    delete globalThis.sessionStorage;
  }
});

test("coMonDangGo: chỉ hỏi trước khi bỏ khi có món mang tên hay số tiền", async () => {
  const { coMonDangGo } = await taiLai(6);
  assert.equal(coMonDangGo([]), false);
  assert.equal(coMonDangGo([{ name: "  ", lineTotalVnd: 0 }]), false, "dòng trống không phải bill đang gõ");
  assert.equal(coMonDangGo([{ name: "", lineTotalVnd: 0 }, { name: "Bia", lineTotalVnd: 0 }]), true);
  assert.equal(coMonDangGo([{ name: "", lineTotalVnd: 1 }]), true);
});

test("buocMoLai: vừa rời (tải lại, Back rồi Forward) thì mở lại đúng bước; để lâu thì mời từ bước 1", async () => {
  const { buocMoLai, MO_LAI_TRONG_MS } = await taiLai(7);
  const bayGio = 1_800_000_000_000;
  const ban = { ten: "gan-mon", bill: { id: "b1" } };
  assert.deepEqual(buocMoLai({ luc: bayGio - 5_000, buoc: ban }, bayGio), ban);
  assert.deepEqual(buocMoLai({ luc: bayGio - MO_LAI_TRONG_MS, buoc: ban }, bayGio), ban, "đúng mốc vẫn mở lại");
  assert.equal(buocMoLai({ luc: bayGio - MO_LAI_TRONG_MS - 1, buoc: ban }, bayGio), null, "để lâu: không tự mở bill cũ");
  assert.equal(buocMoLai({ luc: bayGio, buoc: undefined }, bayGio), null, "bản nháp ở bước 1 thì mở bước 1");
  assert.equal(buocMoLai({ buoc: ban }, bayGio), null, "bản nháp không có giờ (bản cũ) thì không tự mở");
  assert.equal(buocMoLai(null, bayGio), null);
});

test("ADR-0054: bill viết từ một kèo có bản nháp riêng, không lẫn sang kèo khác hay sang bill của cả nhóm", async () => {
  const kho = khoPhien();
  globalThis.sessionStorage = kho;
  try {
    const m = await taiLai(30);
    const KEO_1 = "11111111-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
    const KEO_2 = "22222222-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
    assert.equal(m.khoaNhapBill(NHOM_A), NHOM_A, "bill không từ kèo nào giữ đúng khoá cũ");
    m.luuNhapBill(m.khoaNhapBill(NHOM_A, KEO_1), NHAP);
    assert.deepEqual(m.docNhapBill(m.khoaNhapBill(NHOM_A, KEO_1)), NHAP);
    assert.equal(m.docNhapBill(m.khoaNhapBill(NHOM_A, KEO_2)), null);
    assert.equal(m.docNhapBill(m.khoaNhapBill(NHOM_A)), null);
  } finally {
    delete globalThis.sessionStorage;
  }
});
