/* The app manual Nếp answers from must match the app it describes.
 *
 * Nếp answers «how do I do X, what do I tap?» from the manual in
 * `services/core/internal/huongdan/data/*.md`, and the only thing worse than no
 * manual is one that names a button the app no longer has. This gate is what
 * turns a rename into a red CI run instead of a wrong answer:
 *
 *   (a) `_rut.json` -- the map of routes, labels and navigation pulled out of
 *       the source by `tools/rut-huong-dan.mjs` -- is regenerated here in memory
 *       and must equal the committed bytes; and `src/rudi/nep/huong-dan-ban.ts`
 *       must carry the first 12 hex of the sha256 of those committed bytes, the
 *       value the server's `huongdan.BanDung()` computes from its embedded copy;
 *   (b) every manual's front matter parses, and its `man` and every
 *       `di_toi[].man` is a route that map knows;
 *   (c) every «…» in a body is declared in that file's `nhanUI`, and every
 *       `nhanUI` entry is a literal somewhere in `apps/mobile/{src,app}` (a
 *       string, a template with `${}` read as `…`, or JSX text -- never a
 *       comment, because comments are not nodes);
 *   (d) a manual for a money screen (a route whose first segment is in
 *       `MAN_NEP_LUI`, read from `phieu.ts` rather than copied here) says
 *       `tien: true`, keeps to a single navigation section, and has no digit
 *       in its body; no manual anywhere states an amount.
 *
 * (e) is the part that keeps the rest honest. A checker that goes blind -- a
 * regex that stops matching, a set that comes back empty -- reports nothing,
 * and nothing reads exactly like clean. So the same functions are fed
 * synthetic input with a known defect and must refuse it at the predicted
 * rule, and a known-good input that must pass (identity), so a checker that
 * refuses everything is caught too.
 */
import assert from "node:assert/strict";
import { readFileSync, readdirSync } from "node:fs";
import { dirname, join } from "node:path";
import { test } from "node:test";

import { MAN_NEP_LUI } from "../dist-test/rudi/nep/phieu.js";
import {
  DUONG_BAN,
  DUONG_RUT,
  banDung,
  chuoiBan,
  chuoiRut,
  literalTrongMa,
  literalTuNguon,
  maMan,
  rutTuNguon,
} from "../tools/rut-huong-dan.mjs";

const THU_MUC_SO_TAY = dirname(DUONG_RUT);
const KHOA_DAU = ["di_toi", "man", "nhanUI", "tien", "tieu_de"];
/**
 * An amount of money: «200k», «1 triệu», «50.000đ», «300 nghìn». The separator
 * must be followed by a digit, so a list marker («1. Đồng ý») is not an amount.
 */
const SO_TIEN = /(?<![\p{L}\d])\d+(?:[.,]\d+)*\s?(k|nghìn|ngàn|triệu|tr|đ|đồng|vnd)(?![\p{L}\d])/iu;

/** The screens this manual must cover; a missing one is a gap Nếp would fill by guessing. */
const MAN_BAT_BUOC = [
  "plan",
  "outings/new",
  "outings/[id]",
  "places/[id]",
  "explore",
  "groups/[id]/chat",
  "groups/[id]/to-giay",
  "profile",
  "messages",
  "finance",
  "smart-split/[id]/review",
];

/** Null when the committed bytes are what the extractor writes now; otherwise where they part. */
function kiemTuoi(daCommit, sinhLai) {
  if (daCommit === sinhLai) return null;
  const a = daCommit.split("\n");
  const b = sinhLai.split("\n");
  let i = 0;
  while (i < a.length && i < b.length && a[i] === b[i]) i++;
  return `_rut.json lệch từ dòng ${i + 1}: đang có ${JSON.stringify(a[i] ?? "")}, mã sinh ra ${JSON.stringify(b[i] ?? "")}. Chạy \`node tools/rut-huong-dan.mjs\` trong apps/mobile.`;
}

/**
 * Null when `huong-dan-ban.ts` holds exactly what the extractor writes for the
 * committed `_rut.json`; otherwise which constant it has and which it needs.
 */
function kiemBan(banDaCommit, rutDaCommit) {
  if (banDaCommit === chuoiBan(rutDaCommit)) return null;
  const co = /HUONG_DAN_BAN = "([^"]*)"/.exec(banDaCommit ?? "")?.[1] ?? "(không đọc được)";
  return `huong-dan-ban.ts cũ: đang có ${co}, _rut.json đã commit băm ra ${banDung(rutDaCommit)}. Chạy \`node tools/rut-huong-dan.mjs\` trong apps/mobile.`;
}

function laManTien(man) {
  return typeof man === "string" && MAN_NEP_LUI.includes(man.replace(/^\/+/, "").split("/")[0]);
}

/** Front matter between a `---json` first line and the next `---` line, and the body after it. */
function tachSoTay(noiDung) {
  const dong = noiDung.split("\n");
  if (dong[0] !== "---json") return { loi: "dòng đầu phải là ---json" };
  const het = dong.indexOf("---", 1);
  if (het === -1) return { loi: "thiếu dòng --- đóng front matter" };
  try {
    return { dau: JSON.parse(dong.slice(1, het).join("\n")), than: dong.slice(het + 1).join("\n") };
  } catch (error) {
    return { loi: `front matter không phải JSON: ${error.message}` };
  }
}

/**
 * Every rule one manual must satisfy. Returns the violations; empty means the
 * file passes. Pure over its inputs, so the canaries below run the very same
 * function on synthetic text.
 */
function kiemSoTay(ten, noiDung, { cacMan, literal }) {
  const t = tachSoTay(noiDung);
  if (t.loi) return [`${ten}: ${t.loi}`];
  const { dau, than } = t;
  if (!dau || typeof dau !== "object" || Array.isArray(dau)) return [`${ten}: front matter phải là một object`];

  const loi = [];
  const khoa = Object.keys(dau).sort();
  if (khoa.join(",") !== KHOA_DAU.join(",")) loi.push(`${ten}: khoá front matter phải đúng [${KHOA_DAU}], đang có [${khoa}]`);
  if (typeof dau.man !== "string" || dau.man === "") loi.push(`${ten}: man phải là chuỗi`);
  if (typeof dau.tieu_de !== "string" || dau.tieu_de.trim() === "") loi.push(`${ten}: tieu_de phải là chuỗi`);
  if (!Array.isArray(dau.nhanUI) || !dau.nhanUI.every((n) => typeof n === "string" && n !== "")) loi.push(`${ten}: nhanUI phải là mảng chuỗi`);
  if (!Array.isArray(dau.di_toi) || !dau.di_toi.every((d) => d && typeof d.nhan === "string" && typeof d.man === "string" && Object.keys(d).length === 2)) {
    loi.push(`${ten}: di_toi phải là mảng {nhan, man}`);
  }
  if (typeof dau.tien !== "boolean") loi.push(`${ten}: tien phải là true/false`);
  if (loi.length > 0) return loi;

  // (b) Routes, as the extractor named them.
  if (!cacMan.has(dau.man)) loi.push(`${ten}: man «${dau.man}» không có trong _rut.json`);
  const nhanUI = new Set(dau.nhanUI);
  for (const d of dau.di_toi) {
    if (!cacMan.has(d.man)) loi.push(`${ten}: di_toi tới «${d.man}» không có trong _rut.json`);
    if (!nhanUI.has(d.nhan)) loi.push(`${ten}: di_toi đi bằng nhãn «${d.nhan}» không khai trong nhanUI`);
  }

  // (c) Labels: quoted only from nhanUI, and nhanUI only from the source.
  const moNgoac = (than.match(/«/g) ?? []).length;
  const dongNgoac = (than.match(/»/g) ?? []).length;
  if (moNgoac !== dongNgoac) loi.push(`${ten}: số « (${moNgoac}) và » (${dongNgoac}) không khớp`);
  const trich = [...than.matchAll(/«([^«»]*)»/g)].map((m) => m[1]);
  for (const q of trich) if (!nhanUI.has(q)) loi.push(`${ten}: trích «${q}» mà nhãn không khai trong nhanUI`);
  const daDung = new Set([...trich, ...dau.di_toi.map((d) => d.nhan)]);
  for (const n of dau.nhanUI) {
    if (!literal.has(n)) loi.push(`${ten}: nhãn «${n}» không là literal nào trong apps/mobile/{src,app}`);
    if (!daDung.has(n)) loi.push(`${ten}: nhãn «${n}» khai trong nhanUI mà không dùng ở đâu`);
  }

  // Shape: an overview paragraph, then `## việc` sections of at most five steps.
  const [moDau, ...muc] = than.split(/^## /m);
  if (moDau.trim() === "") loi.push(`${ten}: thiếu đoạn tổng quan trước mục ## đầu tiên`);
  if (muc.length === 0) loi.push(`${ten}: chưa có mục ## nào`);
  for (const m of muc) {
    const tieuDe = m.split("\n")[0].trim();
    const buoc = m.split("\n").filter((l) => /^(\d+\.|[-*])\s/.test(l)).length;
    if (buoc === 0) loi.push(`${ten}: mục «${tieuDe}» không có bước nào`);
    if (buoc > 5) loi.push(`${ten}: mục «${tieuDe}» có ${buoc} bước, tối đa 5`);
  }

  // (d) Money: never an amount anywhere; money screens are navigation only.
  const soTien = than.match(SO_TIEN);
  if (soTien) loi.push(`${ten}: có số tiền «${soTien[0]}» trong thân`);
  const tien = laManTien(dau.man);
  if (dau.tien !== tien) loi.push(`${ten}: tien phải là ${tien} cho màn «${dau.man}»`);
  if (tien) {
    if (/\d/.test(than)) loi.push(`${ten}: màn tiền không được có chữ số trong thân`);
    if (muc.length !== 1) loi.push(`${ten}: màn tiền chỉ được có một mục chỉ đường, đang có ${muc.length}`);
  }
  return loi;
}

const RUT_DA_COMMIT = readFileSync(DUONG_RUT, "utf8");
const RUT_SINH_LAI = chuoiRut();
const BAN_DA_COMMIT = readFileSync(DUONG_BAN, "utf8");
const CAC_MAN = new Set(JSON.parse(RUT_DA_COMMIT).routes.map((r) => r.man));
const LITERAL = literalTrongMa();
const NGU_CANH = { cacMan: CAC_MAN, literal: LITERAL };
const SO_TAY = readdirSync(THU_MUC_SO_TAY)
  .filter((ten) => ten.endsWith(".md"))
  .sort()
  .map((ten) => ({ ten, noiDung: readFileSync(join(THU_MUC_SO_TAY, ten), "utf8") }));

/** A manual known to be good, for the identity half of every canary. */
const MAU_DUNG = [
  "---json",
  JSON.stringify({ man: "plan", tieu_de: "Lên plan", nhanUI: ["Tạo kèo"], di_toi: [{ nhan: "Tạo kèo", man: "outings/new" }], tien: false }),
  "---",
  "Tổng quan ngắn.",
  "",
  "## Tạo kèo",
  "",
  "1. Bấm «Tạo kèo».",
  "",
].join("\n");

test("(a) _rut.json khớp từng byte với bản rút lại từ mã", () => {
  assert.equal(kiemTuoi(RUT_DA_COMMIT, RUT_SINH_LAI), null);
});

test("(a) huong-dan-ban.ts mang đúng băm của _rut.json đã commit", () => {
  assert.equal(kiemBan(BAN_DA_COMMIT, RUT_DA_COMMIT), null);
  assert.match(banDung(RUT_DA_COMMIT), /^[0-9a-f]{12}$/);
});

test("(a) bản rút có đủ các màn chính và không đích nào lạc ra ngoài cây route", () => {
  const { routes } = JSON.parse(RUT_SINH_LAI);
  assert.ok(routes.length >= 40, `chỉ rút được ${routes.length} màn`);
  for (const man of MAN_BAT_BUOC) assert.ok(CAC_MAN.has(man), `thiếu màn ${man}`);
  const lac = routes.flatMap((r) => r.di_toi.filter((d) => !CAC_MAN.has(d)).map((d) => `${r.man} -> ${d}`));
  assert.deepEqual(lac, []);
  const keo = routes.find((r) => r.man === "outings/[id]");
  assert.ok(keo.nhan.includes("Tôi đã tới") && keo.di_toi.includes("places/[id]"), "màn kèo mất nhãn hoặc cạnh đã biết");
});

test("(b)(c)(d) mọi file sổ tay khớp mã", () => {
  assert.ok(SO_TAY.length > 0, "không đọc được file sổ tay nào");
  const loi = SO_TAY.flatMap(({ ten, noiDung }) => kiemSoTay(ten, noiDung, NGU_CANH));
  assert.deepEqual(loi, []);
});

test("sổ tay phủ mọi màn bắt buộc, mỗi màn đúng một file", () => {
  const theoMan = new Map();
  for (const { ten, noiDung } of SO_TAY) {
    const { dau } = tachSoTay(noiDung);
    assert.ok(!theoMan.has(dau.man), `${ten} và ${theoMan.get(dau.man)} cùng tả màn ${dau.man}`);
    theoMan.set(dau.man, ten);
  }
  for (const man of MAN_BAT_BUOC) assert.ok(theoMan.has(man), `chưa có sổ tay cho màn ${man}`);
  assert.ok([...theoMan.keys()].some(laManTien), "chưa có sổ tay nào cho màn tiền");
});

test("(e) canary: identity đi qua đúng bộ kiểm, không lỗi nào", () => {
  assert.deepEqual(kiemSoTay("mau-dung.md", MAU_DUNG, NGU_CANH), []);
});

test("(e) canary: nhãn bịa bị từ chối ở luật literal", () => {
  const bia = MAU_DUNG.replace('"nhanUI":["Tạo kèo"]', '"nhanUI":["Tạo kèo","Nút bay lên trời"]').replace(
    "1. Bấm «Tạo kèo».",
    "1. Bấm «Tạo kèo».\n2. Bấm «Nút bay lên trời».",
  );
  assert.notEqual(bia, MAU_DUNG);
  assert.deepEqual(kiemSoTay("bia.md", bia, NGU_CANH), [
    "bia.md: nhãn «Nút bay lên trời» không là literal nào trong apps/mobile/{src,app}",
  ]);
});

test("(e) canary: trích nhãn không khai trong nhanUI bị từ chối", () => {
  const ngoai = MAU_DUNG.replace("1. Bấm «Tạo kèo».", "1. Bấm «Tạo kèo».\n2. Bấm «Lưu thứ tự».");
  assert.deepEqual(kiemSoTay("ngoai.md", ngoai, NGU_CANH), ["ngoai.md: trích «Lưu thứ tự» mà nhãn không khai trong nhanUI"]);
});

test("(e) canary: màn tiền có chữ số và màn thường nêu số tiền đều bị từ chối", () => {
  const tien = MAU_DUNG.replace('"man":"plan"', '"man":"finance"')
    .replace('"tien":false', '"tien":true')
    .replace("1. Bấm «Tạo kèo».", "- Bấm «Tạo kèo» lần 2.");
  assert.deepEqual(kiemSoTay("tien.md", tien, NGU_CANH), ["tien.md: màn tiền không được có chữ số trong thân"]);
  const thuong = MAU_DUNG.replace("Tổng quan ngắn.", "Mỗi người góp 200k.");
  assert.deepEqual(kiemSoTay("thuong.md", thuong, NGU_CANH), ["thuong.md: có số tiền «200k» trong thân"]);
});

test("(e) canary: route lạ trong man hoặc di_toi bị từ chối", () => {
  const lac = MAU_DUNG.replace('"man":"outings/new"', '"man":"outings/khong-co"');
  assert.deepEqual(kiemSoTay("lac.md", lac, NGU_CANH), ["lac.md: di_toi tới «outings/khong-co» không có trong _rut.json"]);
});

test("(e) canary: _rut.json cũ một byte bị bắt ở luật tươi", () => {
  assert.equal(kiemTuoi(RUT_SINH_LAI, RUT_SINH_LAI), null);
  assert.match(kiemTuoi(RUT_SINH_LAI.replace('"Tôi đã tới"', '"Tôi đã tới rồi"'), RUT_SINH_LAI) ?? "", /_rut\.json lệch từ dòng \d+/);
});

test("(e) canary: hằng bản dựng cũ bị bắt ở luật băm", () => {
  const dung = chuoiBan(RUT_DA_COMMIT);
  assert.equal(kiemBan(dung, RUT_DA_COMMIT), null);
  // The constant of an older build: the file is otherwise byte-identical.
  const cu = dung.replace(banDung(RUT_DA_COMMIT), "ffffffffffff");
  assert.notEqual(cu, dung);
  assert.match(kiemBan(cu, RUT_DA_COMMIT) ?? "", /huong-dan-ban\.ts cũ: đang có ffffffffffff/);
  // The map moved by one label and the constant was not regenerated.
  const rutMoi = RUT_DA_COMMIT.replace('"Tôi đã tới"', '"Tôi đã tới rồi"');
  assert.notEqual(rutMoi, RUT_DA_COMMIT);
  assert.match(kiemBan(dung, rutMoi) ?? "", /huong-dan-ban\.ts cũ/);
  assert.match(kiemBan(null, RUT_DA_COMMIT) ?? "", /không đọc được/);
});

test("(e) canary: bộ gom literal thấy chuỗi, template và chữ JSX, không thấy comment", () => {
  const nguon = [
    "// «Nhãn trong comment» must not count",
    "/** Nhãn trong docstring */",
    "export const A = () => <View>",
    '  <RudiButton label="Nhãn thật" />',
    "  <Button label={`Rủ ${ten} tới đây`} />",
    "  <Text>Chữ trong JSX</Text>",
    "  <Text>Ừ, hẹn {ngay}</Text>",
    "</View>;",
  ].join("\n");
  const thay = literalTuNguon(nguon);
  for (const co of ["Nhãn thật", "Rủ … tới đây", "Chữ trong JSX", "Ừ, hẹn …"]) assert.ok(thay.has(co), `bộ gom mù với ${co}`);
  for (const khong of ["«Nhãn trong comment» must not count", "Nhãn trong docstring"]) assert.ok(!thay.has(khong), `bộ gom đếm cả comment: ${khong}`);
});

test("(e) canary: bộ rút đặt tên route và đích như expo-router", () => {
  assert.equal(maMan("(tabs)/plan.tsx"), "plan");
  assert.equal(maMan("outings/[id]/index.tsx"), "outings/[id]");
  assert.equal(maMan("index.tsx"), "index");
  assert.equal(maMan("_layout.tsx"), null);
  assert.equal(maMan("[...legacy].tsx"), null);
  assert.equal(maMan("dev/ui-lab.tsx"), null);
  const nguon = [
    "router.push(`/outings/${k.id}?ctx=${c}` as never);",
    'router.push("/outings/new");',
    'router.replace(("/groups/" + id + "/wall") as never);',
    "router.push({ pathname: `/groups/${id}/to-giay`, params: {} });",
    'const MENU = [{ title: "Tạo", href: "/smart-split/moi/review" }];',
    'const x = <Redirect href="/(tabs)/plan" />;',
    'const y = <A href="https://example.invalid" title="Đi đâu" />;',
  ].join("\n");
  const ra = rutTuNguon(nguon, [...CAC_MAN]);
  assert.deepEqual(ra.di_toi, ["groups/[id]/to-giay", "groups/[id]/wall", "outings/[id]", "outings/new", "plan", "smart-split/[id]/review"]);
  assert.deepEqual(ra.nhan, ["Đi đâu"]);
});
