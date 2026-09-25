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
 *       `di_toi[].man` is a route that map knows; no `di_toi` leads to the
 *       manual's own screen, and every `di_toi` is an edge the app has: in
 *       the route's `_rut.json` `di_toi`, between two tabs, or named in
 *       `CANH_NGOAI_RUT` with the reason;
 *   (c) every «…» in a body is declared in that file's `nhanUI`, and every
 *       `nhanUI` entry is a literal somewhere in `apps/mobile/{src,app}` (a
 *       string, a template with `${}` read as `…`, or JSX text -- never a
 *       comment, because comments are not nodes);
 *   (d) a manual for a money screen (a route whose first segment is in
 *       `MAN_NEP_LUI`, read from `phieu.ts` rather than copied here) says
 *       `tien: true`, keeps to a single navigation section headed «Tới màn
 *       này và đi tiếp», and has no digit in its body; every way in or out
 *       of it that a manual declares is a labelled edge of the code (a
 *       button with that label that leads there, `_rut.json` `canh`), and
 *       every step quotes one of those doors (or the title, printed on it,
 *       of a non-money screen with a way in); no manual anywhere states an
 *       amount.
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
/** The one heading a money screen's single section may have: nothing in it but navigation. */
const TIEU_DE_MAN_TIEN = "Tới màn này và đi tiếp";
/**
 * Manual edges that are real although no route's code shows them. The server
 * keeps the same list (`canhNgoaiRut` in huongdan/nap.go) with the same reason.
 */
const CANH_NGOAI_RUT = [
  {
    tu: "plan",
    den: "create",
    viSao: "the «Tạo mới» button of the tab bar (src/rudi/ui/RudiTabBar.tsx pushes /create), drawn by app/(tabs)/_layout.tsx, which is not a route",
  },
];
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

/** Whether the app has a way from `tu` to `den`: its code, the tab bar, or a named exception. */
function laCanhMa(tu, den, { rut, tab, canhNgoai }) {
  return (rut.get(tu)?.di_toi ?? []).includes(den) || (tab.has(tu) && tab.has(den)) || canhNgoai.some((c) => c.tu === tu && c.den === den);
}

/** Whether `tu` has a button labelled `nhan` that leads to `den` (`_rut.json` `canh`). */
function laCanhCoNhan(tu, den, nhan, { rut }) {
  return (rut.get(tu)?.canh ?? []).some((c) => c.den === den && c.nhan === nhan);
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
function kiemSoTay(ten, noiDung, nguCanh) {
  const { cacMan, literal } = nguCanh;
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
    if (d.man === dau.man) loi.push(`${ten}: di_toi «${d.nhan}» về chính màn «${dau.man}»`);
    else if (cacMan.has(d.man) && cacMan.has(dau.man) && !laCanhMa(dau.man, d.man, nguCanh)) {
      loi.push(`${ten}: di_toi «${d.nhan}» từ «${dau.man}» tới «${d.man}» không phải cạnh nào của mã`);
    }
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
    for (const m of muc) {
      const tieuDe = m.split("\n")[0].trim();
      if (tieuDe !== TIEU_DE_MAN_TIEN) loi.push(`${ten}: màn tiền: tiêu đề mục phải là «${TIEU_DE_MAN_TIEN}», đang là «${tieuDe}»`);
    }
  }
  return loi;
}

/**
 * The money rules that need every manual at once: which ways lead into a
 * money screen, and whether each is a button of the code. A door of a money
 * screen is a label of a declared way in or out that is a labelled edge of
 * the code, or the title of a non-money screen with such a way in, printed
 * on that screen. Every step of a money section must quote a door.
 */
function kiemCuaManTien(cacTep, nguCanh) {
  const trang = [];
  for (const { ten, noiDung } of cacTep) {
    const t = tachSoTay(noiDung);
    if (!t.loi && t.dau && Array.isArray(t.dau.di_toi)) trang.push({ ten, dau: t.dau, than: t.than });
  }
  const loi = [];
  for (const t of trang) {
    if (!laManTien(t.dau.man)) continue;
    const cua = new Set();
    for (const d of t.dau.di_toi) {
      if (laCanhCoNhan(t.dau.man, d.man, d.nhan, nguCanh)) cua.add(d.nhan);
      else loi.push(`${t.ten}: màn tiền: lối ra «${d.nhan}» tới «${d.man}» không phải nút nào của «${t.dau.man}» dẫn tới đó`);
    }
    for (const k of trang) {
      if (k === t) continue;
      for (const d of k.dau.di_toi) {
        if (d.man !== t.dau.man) continue;
        if (!laCanhCoNhan(k.dau.man, t.dau.man, d.nhan, nguCanh)) {
          loi.push(`${t.ten}: màn tiền: lối vào «${d.nhan}» từ ${k.ten} không phải nút nào của «${k.dau.man}» dẫn tới đây`);
          continue;
        }
        cua.add(d.nhan);
        if (!laManTien(k.dau.man) && (nguCanh.rut.get(k.dau.man)?.nhan ?? []).includes(k.dau.tieu_de)) cua.add(k.dau.tieu_de);
      }
    }
    const [, ...muc] = t.than.split(/^## /m);
    for (const m of muc) {
      for (const l of m.split("\n").slice(1)) {
        if (l.trim() === "") continue;
        const buoc = /^(?:\d+\.|[-*])\s+(.*)$/.exec(l);
        if (!buoc) {
          loi.push(`${t.ten}: màn tiền: dòng không phải bước chỉ đường: ${JSON.stringify(l)}`);
          continue;
        }
        const trich = [...buoc[1].matchAll(/«([^«»]*)»/g)].map((x) => x[1]);
        if (!trich.some((q) => cua.has(q))) loi.push(`${t.ten}: màn tiền: bước không chỉ lối vào hay lối ra nào: ${JSON.stringify(buoc[1])}`);
      }
    }
  }
  return loi;
}

const RUT_DA_COMMIT = readFileSync(DUONG_RUT, "utf8");
const RUT_SINH_LAI = chuoiRut();
const BAN_DA_COMMIT = readFileSync(DUONG_BAN, "utf8");
const ROUTES = JSON.parse(RUT_DA_COMMIT).routes;
const CAC_MAN = new Set(ROUTES.map((r) => r.man));
const LITERAL = literalTrongMa();
const NGU_CANH = {
  cacMan: CAC_MAN,
  literal: LITERAL,
  rut: new Map(ROUTES.map((r) => [r.man, r])),
  tab: new Set(ROUTES.filter((r) => r.tep.some((p) => p.startsWith("app/(tabs)/"))).map((r) => r.man)),
  canhNgoai: CANH_NGOAI_RUT,
};
const SO_TAY = readdirSync(THU_MUC_SO_TAY)
  .filter((ten) => ten.endsWith(".md"))
  .sort()
  .map((ten) => ({ ten, noiDung: readFileSync(join(THU_MUC_SO_TAY, ten), "utf8") }));

/** A money manual known to be good: finance, its one door out a real button. */
const MAU_TIEN = [
  "---json",
  JSON.stringify({ man: "finance", tieu_de: "Tài chính của tôi", nhanUI: ["Xem quyết toán"], di_toi: [{ nhan: "Xem quyết toán", man: "settlements/[id]" }], tien: true }),
  "---",
  "Màn tiền. Nếp chỉ chỉ đường tới đây.",
  "",
  `## ${TIEU_DE_MAN_TIEN}`,
  "",
  "- Bấm «Xem quyết toán».",
  "",
].join("\n");

/** The committed manuals with one file's text replaced step by step: each [cu, moi] must match. */
function suaSoTay(ten, ...cacSua) {
  return SO_TAY.map((f) => {
    if (f.ten !== ten) return f;
    let noiDung = f.noiDung;
    for (const [cu, moi] of cacSua) {
      assert.ok(noiDung.includes(cu), `${ten} no longer contains ${cu}`);
      noiDung = noiDung.replace(cu, moi);
    }
    return { ten, noiDung };
  });
}

/** Every rule, per file and across files, over a set of manuals. */
function kiemTatCa(cacTep, nguCanh = NGU_CANH) {
  return [...cacTep.flatMap(({ ten, noiDung }) => kiemSoTay(ten, noiDung, nguCanh)), ...kiemCuaManTien(cacTep, nguCanh)];
}

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

test("(a) mỗi cạnh có nhãn là một cạnh của màn đó, mang một nhãn in trên màn đó", () => {
  let so = 0;
  for (const r of ROUTES) {
    for (const c of r.canh) {
      so++;
      assert.ok(r.di_toi.includes(c.den), `${r.man}: cạnh có nhãn «${c.nhan}» tới ${c.den} không có trong di_toi`);
      assert.ok(r.nhan.includes(c.nhan), `${r.man}: cạnh có nhãn «${c.nhan}» không có trong nhan`);
    }
  }
  assert.ok(so >= 50, `chỉ rút được ${so} cạnh có nhãn`);
});

test("(b)(c)(d) mọi file sổ tay khớp mã", () => {
  assert.ok(SO_TAY.length > 0, "không đọc được file sổ tay nào");
  assert.deepEqual(kiemTatCa(SO_TAY), []);
});

test("(b) mỗi ngoại lệ CANH_NGOAI_RUT có sổ tay dùng, chưa phải cạnh của mã, và nguồn vẫn làm nó có thật", () => {
  const khongNgoaiLe = { ...NGU_CANH, canhNgoai: [] };
  const tabBar = readFileSync(join(dirname(DUONG_BAN), "../ui/RudiTabBar.tsx"), "utf8");
  assert.ok(tabBar.includes('router.push("/create")') && tabBar.includes('accessibilityLabel="Tạo mới"'), "RudiTabBar không còn nút «Tạo mới» đẩy /create");
  for (const c of CANH_NGOAI_RUT) {
    assert.ok(c.viSao.length > 0);
    assert.ok(!laCanhMa(c.tu, c.den, khongNgoaiLe), `${c.tu} -> ${c.den} đã là cạnh của mã: ngoại lệ thừa`);
    assert.ok(NGU_CANH.tab.has(c.tu), `${c.tu} không phải tab, lý do thanh tab không áp`);
    const dung = SO_TAY.some(({ noiDung }) => {
      const { dau } = tachSoTay(noiDung);
      return dau.man === c.tu && dau.di_toi.some((d) => d.man === c.den);
    });
    assert.ok(dung, `không sổ tay nào dùng ${c.tu} -> ${c.den}`);
  }
  // Load-bearing: without it the committed manual is refused at exactly that edge.
  assert.deepEqual(kiemTatCa(SO_TAY, khongNgoaiLe), ["len-plan.md: di_toi «Tạo mới» từ «plan» tới «create» không phải cạnh nào của mã"]);
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
  assert.deepEqual(kiemTatCa([{ ten: "tien.md", noiDung: MAU_TIEN }]), []);
  const tien = MAU_TIEN.replace("- Bấm «Xem quyết toán».", "- Bấm «Xem quyết toán» lần 2.");
  assert.deepEqual(kiemTatCa([{ ten: "tien.md", noiDung: tien }]), ["tien.md: màn tiền không được có chữ số trong thân"]);
  const thuong = MAU_DUNG.replace("Tổng quan ngắn.", "Mỗi người góp 200k.");
  assert.deepEqual(kiemSoTay("thuong.md", thuong, NGU_CANH), ["thuong.md: có số tiền «200k» trong thân"]);
});

test("(e) canary: di_toi về chính màn, hoặc không phải cạnh nào của mã, bị từ chối; cạnh qua thanh tab thì được", () => {
  const tuTro = MAU_DUNG.replace('"man":"outings/new"', '"man":"plan"');
  assert.deepEqual(kiemSoTay("tu-tro.md", tuTro, NGU_CANH), ["tu-tro.md: di_toi «Tạo kèo» về chính màn «plan»"]);
  const khongCo = MAU_DUNG.replace('"man":"outings/new"', '"man":"groups/new"');
  assert.deepEqual(kiemSoTay("khong-co.md", khongCo, NGU_CANH), ["khong-co.md: di_toi «Tạo kèo» từ «plan» tới «groups/new» không phải cạnh nào của mã"]);
  // plan -> explore: no route file navigates it, the tab bar does.
  assert.ok(!(NGU_CANH.rut.get("plan").di_toi ?? []).includes("explore"));
  const tab = MAU_DUNG.replace('"man":"outings/new"', '"man":"explore"');
  assert.deepEqual(kiemSoTay("tab.md", tab, NGU_CANH), []);
});

test("(e) canary: tiêu đề mục trích nhãn không khai bị từ chối", () => {
  const tieuDe = MAU_DUNG.replace("## Tạo kèo", "## Tạo «Kèo bay»");
  assert.deepEqual(kiemSoTay("tieu-de.md", tieuDe, NGU_CANH), ["tieu-de.md: trích «Kèo bay» mà nhãn không khai trong nhanUI"]);
});

// The bypass of review 13, exactly as the reviewer made it on a scratch copy
// of tai-chinh.md: «Đánh dấu đã trả» (a real label, Bill.tsx) added to
// nhanUI, declared as a di_toi back to finance, and a step to press it. Every
// gate passed it; now three rules refuse it.
const NHAN_TRA = [
  '"nhanUI": ["Tài chính của tôi", "Cá nhân", "Chi theo nhóm", "Xem quyết toán"]',
  '"nhanUI": ["Tài chính của tôi", "Cá nhân", "Chi theo nhóm", "Xem quyết toán", "Đánh dấu đã trả"]',
];
const BUOC_TRA = [
  "- Ở mục «Chi theo nhóm», bấm «Xem quyết toán» để mở màn quyết toán của nhóm.",
  "- Ở mục «Chi theo nhóm», bấm «Xem quyết toán» để mở màn quyết toán của nhóm.\n- Chuyển khoản cho người ứng xong thì bấm «Đánh dấu đã trả».",
];
const diToiTra = (den) => [
  '{"nhan": "Xem quyết toán", "man": "settlements/[id]"}',
  `{"nhan": "Xem quyết toán", "man": "settlements/[id]"}, {"nhan": "Đánh dấu đã trả", "man": "${den}"}`,
];
const BUOC_LOI = 'tai-chinh.md: màn tiền: bước không chỉ lối vào hay lối ra nào: "Chuyển khoản cho người ứng xong thì bấm «Đánh dấu đã trả»."';

test("(e) canary: lách màn tiền của review 13, đúng nguyên bản, bị từ chối", () => {
  assert.deepEqual(kiemTatCa(suaSoTay("tai-chinh.md", NHAN_TRA, diToiTra("finance"), BUOC_TRA)), [
    "tai-chinh.md: di_toi «Đánh dấu đã trả» về chính màn «finance»",
    "tai-chinh.md: màn tiền: lối ra «Đánh dấu đã trả» tới «finance» không phải nút nào của «finance» dẫn tới đó",
    BUOC_LOI,
  ]);
});

test("(e) canary: nút trả tiền khai làm lối ra tới một cạnh thật của mã, hay tới chỗ mã không đi, đều bị từ chối", () => {
  assert.deepEqual(kiemTatCa(suaSoTay("tai-chinh.md", NHAN_TRA, diToiTra("settlements/[id]"), BUOC_TRA)), [
    "tai-chinh.md: màn tiền: lối ra «Đánh dấu đã trả» tới «settlements/[id]» không phải nút nào của «finance» dẫn tới đó",
    BUOC_LOI,
  ]);
  assert.deepEqual(kiemTatCa(suaSoTay("tai-chinh.md", NHAN_TRA, diToiTra("messages"), BUOC_TRA)), [
    "tai-chinh.md: di_toi «Đánh dấu đã trả» từ «finance» tới «messages» không phải cạnh nào của mã",
    "tai-chinh.md: màn tiền: lối ra «Đánh dấu đã trả» tới «messages» không phải nút nào của «finance» dẫn tới đó",
    BUOC_LOI,
  ]);
  // On chia-hoa-don the payment button IS printed (Bill.tsx is one of the
  // route's files): only «a button with that label leads there» refuses it.
  assert.ok(NGU_CANH.rut.get("smart-split/[id]/review").nhan.includes("Đánh dấu đã trả"));
  const chia = suaSoTay(
    "chia-hoa-don.md",
    ['"nhanUI": ["Chia hóa đơn", "Tạo mới", "Chia bill buổi này", "Xem quyết toán", "Về Tin nhắn"]', '"nhanUI": ["Chia hóa đơn", "Tạo mới", "Chia bill buổi này", "Xem quyết toán", "Về Tin nhắn", "Đánh dấu đã trả"]'],
    ['{"nhan": "Về Tin nhắn", "man": "messages"}', '{"nhan": "Về Tin nhắn", "man": "messages"}, {"nhan": "Đánh dấu đã trả", "man": "settlements/[id]"}'],
  );
  assert.deepEqual(kiemTatCa(chia), [
    "chia-hoa-don.md: màn tiền: lối ra «Đánh dấu đã trả» tới «settlements/[id]» không phải nút nào của «smart-split/[id]/review» dẫn tới đó",
  ]);
});

test("(e) canary: tiêu đề mục màn tiền đổi thành cách trả bị từ chối", () => {
  assert.deepEqual(kiemTatCa(suaSoTay("tai-chinh.md", ["## Tới màn này và đi tiếp", "## Chuyển khoản cho người ứng rồi báo là đã trả xong"])), [
    "tai-chinh.md: màn tiền: tiêu đề mục phải là «Tới màn này và đi tiếp», đang là «Chuyển khoản cho người ứng rồi báo là đã trả xong»",
  ]);
});

test("(e) canary: bước màn tiền chỉ nêu tiêu đề màn xuất phát thì được, tiêu đề màn khác thì không", () => {
  const buocMoi = (tieuDe) => ["- Mở tab «Cá nhân», bấm «Tài chính của tôi».", `- Mở tab «Cá nhân», bấm «Tài chính của tôi».\n- Bắt đầu từ «${tieuDe}».`];
  // «Cá nhân» is profile's title, printed on it, and ca-nhan.md has a way here.
  assert.deepEqual(kiemTatCa(suaSoTay("tai-chinh.md", buocMoi("Cá nhân"))), []);
  // «Tin nhắn» is a screen with no way here.
  const nhan = ['"nhanUI": ["Tài chính của tôi", "Cá nhân", "Chi theo nhóm", "Xem quyết toán"]', '"nhanUI": ["Tài chính của tôi", "Cá nhân", "Chi theo nhóm", "Xem quyết toán", "Tin nhắn"]'];
  assert.deepEqual(kiemTatCa(suaSoTay("tai-chinh.md", nhan, buocMoi("Tin nhắn"))), ['tai-chinh.md: màn tiền: bước không chỉ lối vào hay lối ra nào: "Bắt đầu từ «Tin nhắn»."']);
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
  // «Tạo» is a menu item written as data: its title is a label and a labelled edge.
  assert.deepEqual(ra.nhan, ["Tạo", "Đi đâu"]);
  assert.deepEqual(ra.canh, [{ den: "smart-split/[id]/review", nhan: "Tạo" }]);
});

test("(e) canary: bộ rút chỉ ghép nhãn với điều hướng của cùng một thứ người ta bấm", () => {
  const nguon = [
    "const a = <View>",
    '  <RudiButton label="Xem quyết toán" onPress={() => router.replace(`/settlements/${ctx}` as never)} />',
    '  <SectionHeader action={id !== null ? "Mở kèo" : undefined} onAction={id !== null ? () => router.push(("/outings/" + id) as never) : undefined} title="Kèo nhóm" />',
    '  <EmptyState title="Chưa có gì" action={{ label: "Tới Tin nhắn", onPress: () => router.replace("/messages" as never) }} />',
    '  <RudiButton label="Đánh dấu đã trả" onPress={() => danhDau(id)} />',
    '  <ListRow title="Hàng" right={<IconButton onPress={() => router.push("/plan")} />} />',
    '  <Pressable onPress={() => router.push("/plan")}><Text>Chữ con không phải thuộc tính</Text></Pressable>',
    "</View>;",
  ].join("\n");
  const ra = rutTuNguon(nguon, [...CAC_MAN]);
  assert.deepEqual(ra.canh, [
    { den: "messages", nhan: "Tới Tin nhắn" },
    { den: "outings/[id]", nhan: "Kèo nhóm" },
    { den: "outings/[id]", nhan: "Mở kèo" },
    { den: "settlements/[id]", nhan: "Xem quyết toán" },
  ]);
  // A button that leads nowhere is a label, never an edge.
  assert.ok(ra.nhan.includes("Đánh dấu đã trả") && !ra.canh.some((c) => c.nhan === "Đánh dấu đã trả"));
  // Neither an action object nor a render prop lends its navigation to the
  // tag's own title: only an on… handler (or href) of that tag is its tap.
  assert.ok(!ra.canh.some((c) => c.nhan === "Chưa có gì" || c.nhan === "Hàng"));
});
