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
 *   (b) every manual's front matter parses, with no key twice in one object
 *       (`JSON.parse` keeps the last and says nothing), and its `man` and every
 *       `di_toi[].man` is a route that map knows; no `di_toi` leads to the
 *       manual's own screen, and every `di_toi` is an edge the app has: in
 *       the route's `_rut.json` `di_toi`, between two tabs, or named in
 *       `CANH_NGOAI_RUT` with the reason;
 *   (c) every « and » of a body pairs up on its line, in order; every «…»
 *       in a body is declared in that file's `nhanUI`, and every `nhanUI`
 *       entry is a literal somewhere in `apps/mobile/{src,app}` (a string, a
 *       template with `${}` read as `…`, or JSX text -- never a comment,
 *       because comments are not nodes);
 *   (d) a manual for a money screen (a route whose first segment is in
 *       `MAN_NEP_LUI`, read from `phieu.ts` rather than copied here) says
 *       `tien: true`, keeps to a single navigation section headed «Tới màn
 *       này và đi tiếp», and has no digit in its body; every way in or out
 *       of it that a manual declares is a labelled edge of the code (a
 *       button with that label that leads there, `_rut.json` `canh`), and
 *       every step quotes at least one of those doors (or the title, printed
 *       on it, of a non-money screen with a way in) and no other label, not
 *       even the money screen's own title; no manual anywhere states an
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

/**
 * The first key that appears twice in one object of a JSON text that already
 * parsed, or null. `JSON.parse` keeps the last of two keys and says nothing,
 * so a second «nhanUI» would silently replace the list every rule reads. A
 * scan, not a parse: strings are skipped whole (escapes included), so a «{»
 * or a «"» inside a value is never taken for structure.
 */
function khoaTrung(json) {
  const ngan = []; // per open container: the keys seen, or null for an array
  let choKhoa = false;
  for (let i = 0; i < json.length; i++) {
    const c = json[i];
    if (c === '"') {
      let j = i + 1;
      while (j < json.length && json[j] !== '"') j += json[j] === "\\" ? 2 : 1;
      if (choKhoa) {
        const khoa = JSON.parse(json.slice(i, j + 1));
        const daThay = ngan[ngan.length - 1];
        if (daThay.has(khoa)) return khoa;
        daThay.add(khoa);
        choKhoa = false;
      }
      i = j;
    } else if (c === "{") {
      ngan.push(new Set());
      choKhoa = true;
    } else if (c === "[") {
      ngan.push(null);
    } else if (c === "}" || c === "]") {
      ngan.pop();
    } else if (c === ",") {
      choKhoa = ngan[ngan.length - 1] instanceof Set;
    }
  }
  return null;
}

/**
 * Whether the « and » of one line pair up in order: every « is closed by a »
 * before the next « and before the line ends, and every » closes a «. Equal
 * counts alone let »Đánh dấu đã trả« through (review 13 round 3, NF3): it
 * reads like a quoted button, but no «…» match sees it.
 */
function ngoacThanhCap(dong) {
  let mo = false;
  for (const c of dong) {
    if (c === "«") {
      if (mo) return false;
      mo = true;
    } else if (c === "»") {
      if (!mo) return false;
      mo = false;
    }
  }
  return !mo;
}

/** Front matter between a `---json` first line and the next `---` line, and the body after it. */
function tachSoTay(noiDung) {
  const dong = noiDung.split("\n");
  if (dong[0] !== "---json") return { loi: "dòng đầu phải là ---json" };
  const het = dong.indexOf("---", 1);
  if (het === -1) return { loi: "thiếu dòng --- đóng front matter" };
  const json = dong.slice(1, het).join("\n");
  let dau;
  try {
    dau = JSON.parse(json);
  } catch (error) {
    return { loi: `front matter không phải JSON: ${error.message}` };
  }
  const trung = khoaTrung(json);
  if (trung !== null) return { loi: `front matter có khoá trùng «${trung}»` };
  return { dau, than: dong.slice(het + 1).join("\n") };
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

  // (c) Labels: «…» pair up on every line, are quoted only from nhanUI, and
  // nhanUI only from the source.
  for (const dong of than.split("\n")) {
    if (!ngoacThanhCap(dong)) loi.push(`${ten}: « và » không thành cặp trên dòng: ${JSON.stringify(dong)}`);
  }
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
 * on that screen. Every step of a money section must quote a door, and
 * nothing but doors: a heading printed on the screen («Chi theo nhóm»), the
 * payment button beside a door, or the money screen's own title is not one.
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
        if (!trich.some((q) => cua.has(q))) {
          loi.push(`${t.ten}: màn tiền: bước không chỉ lối vào hay lối ra nào: ${JSON.stringify(buoc[1])}`);
          continue;
        }
        for (const q of new Set(trich.filter((x) => !cua.has(x)))) {
          loi.push(`${t.ten}: màn tiền: bước trích «${q}», không phải lối vào hay lối ra nào: ${JSON.stringify(buoc[1])}`);
        }
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

/**
 * The Go loader's fixture, mirrored (mauDung in
 * services/core/internal/huongdan/nap_test.go): the same four routes and the
 * same two manuals, so every money rule is held on both sides by the same
 * shapes (review 13 round 2, N4). a and b are tabs, finance is the money
 * screen, «Mở tiền» (a -> finance) and «Về A» (finance -> a) are buttons of
 * the code. Only the literal set is this file's own: the Go loader does not
 * read the source.
 */
const RUT_GO = [
  { canh: [{ den: "finance", nhan: "Mở tiền" }], di_toi: ["b", "finance"], man: "a", nhan: ["Màn A", "Mở tiền", "Nút B"], tep: ["app/(tabs)/a.tsx"] },
  { canh: [], di_toi: [], man: "b", nhan: [], tep: ["app/(tabs)/b.tsx"] },
  { canh: [{ den: "a", nhan: "Về A" }], di_toi: ["a"], man: "finance", nhan: ["Về A"], tep: ["app/finance.tsx"] },
  { canh: [], di_toi: ["a"], man: "welcome", nhan: [], tep: ["app/welcome.tsx"] },
];
const LITERAL_GO = new Set(["Màn A", "Mở tiền", "Nút B", "Về A", "Màn C", "Quyết toán", "Về tài chính", "Đánh dấu đã trả", "Tiền"]);
function nguCanhGo(routes = RUT_GO) {
  return {
    cacMan: new Set(routes.map((r) => r.man)),
    literal: LITERAL_GO,
    rut: new Map(routes.map((r) => [r.man, r])),
    tab: new Set(routes.filter((r) => r.tep.some((p) => p.startsWith("app/(tabs)/"))).map((r) => r.man)),
    canhNgoai: [],
  };
}
const A_GO = [
  "---json",
  '{"man":"a","tieu_de":"Màn A","nhanUI":["Nút B","Mở tiền"],"di_toi":[{"nhan":"Nút B","man":"b"},{"nhan":"Mở tiền","man":"finance"}],"tien":false}',
  "---",
  "Tổng quan của màn A.",
  "",
  "## Đi sang B",
  "",
  "1. Bấm «Nút B».",
  "",
  "## Mở màn tiền",
  "",
  "1. Bấm «Mở tiền».",
  "2. Xem xong thì quay lại.",
  "",
].join("\n");
const TIEN_GO = [
  "---json",
  '{"man":"finance","tieu_de":"Tiền","nhanUI":["Về A","Mở tiền"],"di_toi":[{"nhan":"Về A","man":"a"}],"tien":true}',
  "---",
  "Màn tiền. Nếp chỉ chỉ đường tới đây.",
  "",
  "## Tới màn này và đi tiếp",
  "",
  "- Từ màn A: bấm «Mở tiền».",
  "- Xong thì bấm «Về A».",
  "",
].join("\n");
/** tien.md of the Go fixture with each [cu, moi] applied in turn, next to a.md and any extra files. */
function soTayGo(cacSua = [], ...them) {
  let tien = TIEN_GO;
  for (const [cu, moi] of cacSua) {
    assert.ok(tien.includes(cu), `fixture drifted: ${cu}`);
    tien = tien.replace(cu, moi);
  }
  return [{ ten: "a.md", noiDung: A_GO }, { ten: "tien.md", noiDung: tien }, ...them];
}
/** RUT_GO with one route replaced (same man) or added. */
function rutGo(...doi) {
  const ra = RUT_GO.map((r) => doi.find((d) => d.man === r.man) ?? r);
  return [...ra, ...doi.filter((d) => !RUT_GO.some((r) => r.man === d.man))];
}
const XONG = "- Xong thì bấm «Về A».";

test("(e) canary: fixture của bộ nạp Go — màn tiền một mục, chỉ bước, bước chỉ trích cửa, cửa là nút thật", () => {
  assert.deepEqual(kiemTatCa(soTayGo(), nguCanhGo()), []);
  // MM5 of review 13 round 2: exactly two sections.
  assert.deepEqual(kiemTatCa(soTayGo([[XONG, `${XONG}\n\n## Chia tiền\n\n- Bấm «Về A».`]]), nguCanhGo()), [
    "tien.md: màn tiền chỉ được có một mục chỉ đường, đang có 2",
    "tien.md: màn tiền: tiêu đề mục phải là «Tới màn này và đi tiếp», đang là «Chia tiền»",
  ]);
  // MM4: a line of prose in the money section.
  assert.deepEqual(kiemTatCa(soTayGo([[XONG, `${XONG}\nChuyển khoản cho người ứng.`]]), nguCanhGo()), [
    'tien.md: màn tiền: dòng không phải bước chỉ đường: "Chuyển khoản cho người ứng."',
  ]);
  assert.deepEqual(kiemTatCa(soTayGo([["## Tới màn này và đi tiếp", "## Chuyển khoản cho người ứng rồi báo là đã trả xong"]]), nguCanhGo()), [
    "tien.md: màn tiền: tiêu đề mục phải là «Tới màn này và đi tiếp», đang là «Chuyển khoản cho người ứng rồi báo là đã trả xong»",
  ]);
  assert.deepEqual(kiemTatCa(soTayGo([["Màn tiền. Nếp", "Màn tiền của 2 người. Nếp"]]), nguCanhGo()), ["tien.md: màn tiền không được có chữ số trong thân"]);
  assert.deepEqual(kiemTatCa(soTayGo([['"tien":true', '"tien":false']]), nguCanhGo()), ["tien.md: tien phải là true cho màn «finance»"]);
  // A door on the step does not carry the payment button with it.
  const nhanTra = ['"nhanUI":["Về A","Mở tiền"]', '"nhanUI":["Về A","Mở tiền","Đánh dấu đã trả"]'];
  assert.deepEqual(kiemTatCa(soTayGo([nhanTra, [XONG, "- Chuyển khoản xong thì bấm «Đánh dấu đã trả», rồi bấm «Về A»."]]), nguCanhGo()), [
    'tien.md: màn tiền: bước trích «Đánh dấu đã trả», không phải lối vào hay lối ra nào: "Chuyển khoản xong thì bấm «Đánh dấu đã trả», rồi bấm «Về A»."',
  ]);
  assert.deepEqual(kiemTatCa(soTayGo([nhanTra, [XONG, "- Bấm «Về A» sau khi đã bấm «Đánh dấu đã trả»."]]), nguCanhGo()), [
    'tien.md: màn tiền: bước trích «Đánh dấu đã trả», không phải lối vào hay lối ra nào: "Bấm «Về A» sau khi đã bấm «Đánh dấu đã trả»."',
  ]);
  // The payment button declared as a way out, printed on finance, but no
  // button with that label leads to a.
  const inNutTra = rutGo({ ...RUT_GO[2], nhan: ["Về A", "Đánh dấu đã trả"] });
  const buocTra = "- Chuyển khoản cho người ứng xong thì bấm «Đánh dấu đã trả».";
  assert.deepEqual(
    kiemTatCa(soTayGo([nhanTra, ['{"nhan":"Về A","man":"a"}', '{"nhan":"Về A","man":"a"},{"nhan":"Đánh dấu đã trả","man":"a"}'], [XONG, `${XONG}\n${buocTra}`]]), nguCanhGo(inNutTra)),
    [
      "tien.md: màn tiền: lối ra «Đánh dấu đã trả» tới «a» không phải nút nào của «finance» dẫn tới đó",
      `tien.md: màn tiền: bước không chỉ lối vào hay lối ra nào: ${JSON.stringify(buocTra.slice(2))}`,
    ],
  );
  // The way in from a is not a button of a.
  assert.deepEqual(kiemTatCa(soTayGo(), nguCanhGo(rutGo({ ...RUT_GO[0], canh: [] }))), [
    "tien.md: màn tiền: lối vào «Mở tiền» từ a.md không phải nút nào của «a» dẫn tới đây",
    'tien.md: màn tiền: bước không chỉ lối vào hay lối ra nào: "Từ màn A: bấm «Mở tiền»."',
  ]);
});

test("(e) canary: fixture của bộ nạp Go — cửa tiêu đề chỉ là tiêu đề in trên một màn thường có lối vào", () => {
  // The step names the start screen by its title alone; «Mở tiền» leaves the
  // body, so it leaves nhanUI too (this gate refuses an unused label).
  const chiTieuDe = (tieuDe) => [
    ['"nhanUI":["Về A","Mở tiền"]', `"nhanUI":["Về A","${tieuDe}"]`],
    ["- Từ màn A: bấm «Mở tiền».", `- Bắt đầu từ «${tieuDe}».`],
  ];
  const khongPhaiCua = (tieuDe) => [`tien.md: màn tiền: bước không chỉ lối vào hay lối ra nào: "Bắt đầu từ «${tieuDe}»."`];
  assert.deepEqual(kiemTatCa(soTayGo(chiTieuDe("Màn A")), nguCanhGo()), []);
  // MM1 of review 13 round 2: the same title, no longer printed on a.
  assert.deepEqual(kiemTatCa(soTayGo(chiTieuDe("Màn A")), nguCanhGo(rutGo({ ...RUT_GO[0], nhan: ["Mở tiền", "Nút B"] }))), khongPhaiCua("Màn A"));
  // The title of a screen with no way here.
  const c = { canh: [], di_toi: [], man: "c", nhan: ["Màn C"], tep: ["app/c.tsx"] };
  const cMd = { ten: "c.md", noiDung: '---json\n{"man":"c","tieu_de":"Màn C","nhanUI":[],"di_toi":[],"tien":false}\n---\nMàn C.\n\n## Xem C\n\n1. Xem.\n' };
  assert.deepEqual(kiemTatCa(soTayGo(chiTieuDe("Màn C"), cMd), nguCanhGo(rutGo(c))), khongPhaiCua("Màn C"));
  // MM2: the title of another money screen that does have a way here.
  const q = { canh: [{ den: "finance", nhan: "Về tài chính" }], di_toi: ["finance"], man: "settlements/[id]", nhan: ["Quyết toán", "Về tài chính"], tep: ["app/settlements/[id]/index.tsx"] };
  const qMd = {
    ten: "q.md",
    noiDung:
      '---json\n{"man":"settlements/[id]","tieu_de":"Quyết toán","nhanUI":["Về tài chính"],"di_toi":[{"nhan":"Về tài chính","man":"finance"}],"tien":true}\n---\nMàn quyết toán.\n\n## Tới màn này và đi tiếp\n\n- Bấm «Về tài chính».\n',
  };
  assert.deepEqual(kiemTatCa(soTayGo([], qMd), nguCanhGo(rutGo(q))), []);
  assert.deepEqual(kiemTatCa(soTayGo(chiTieuDe("Quyết toán"), qMd), nguCanhGo(rutGo(q))), khongPhaiCua("Quyết toán"));
  // NF2 of review 13 round 3: the money screen's own title, beside a real
  // door, printed on it or not. On the committed manuals both money titles
  // are doors in as well, so only this fixture, where «Tiền» is none, tells
  // a rule that counts the own title as a door from one that does not.
  const tieuDeRieng = [
    ['"nhanUI":["Về A","Mở tiền"]', '"nhanUI":["Về A","Mở tiền","Tiền"]'],
    [XONG, "- Ở «Tiền», bấm «Về A»."],
  ];
  const tieuDeRiengLoi = ['tien.md: màn tiền: bước trích «Tiền», không phải lối vào hay lối ra nào: "Ở «Tiền», bấm «Về A»."'];
  assert.deepEqual(kiemTatCa(soTayGo(tieuDeRieng), nguCanhGo()), tieuDeRiengLoi);
  assert.deepEqual(kiemTatCa(soTayGo(tieuDeRieng), nguCanhGo(rutGo({ ...RUT_GO[2], nhan: ["Tiền", "Về A"] }))), tieuDeRiengLoi);
});

// The bypass of review 13, exactly as the reviewer made it on a scratch copy
// of tai-chinh.md: «Đánh dấu đã trả» (a real label, Bill.tsx) added to
// nhanUI, declared as a di_toi back to finance, and a step to press it. Every
// gate passed it; now three rules refuse it.
const NHAN_TC = '"nhanUI": ["Tài chính của tôi", "Cá nhân", "Xem quyết toán"]';
const BUOC2_TC = "- Ở mục chi theo nhóm, bấm «Xem quyết toán» để mở màn quyết toán của nhóm.";
/** [cu, moi] for suaSoTay: one more label in tai-chinh.md's nhanUI. */
const themNhanTC = (nhan) => [NHAN_TC, NHAN_TC.replace(/\]$/, `, "${nhan}"]`)];
/** [cu, moi] for suaSoTay: one more line after tai-chinh.md's last step. */
const themBuocTC = (dong) => [BUOC2_TC, `${BUOC2_TC}\n${dong}`];
const NHAN_TRA = themNhanTC("Đánh dấu đã trả");
const BUOC_TRA = themBuocTC("- Chuyển khoản cho người ứng xong thì bấm «Đánh dấu đã trả».");
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
  assert.deepEqual(kiemTatCa(suaSoTay("tai-chinh.md", themNhanTC("Tin nhắn"), buocMoi("Tin nhắn"))), ['tai-chinh.md: màn tiền: bước không chỉ lối vào hay lối ra nào: "Bắt đầu từ «Tin nhắn»."']);
});

test("(e) canary: bước màn tiền trích gì ngoài cửa (nút trả tiền cạnh cửa, tiêu đề mục in trên màn) bị từ chối", () => {
  // Review 13 round 2, P9 exactly: the payment button on a line that also
  // names the door «Xem quyết toán».
  const p9 = "- Chuyển khoản cho người ứng xong thì bấm «Đánh dấu đã trả», rồi bấm «Xem quyết toán».";
  assert.deepEqual(kiemTatCa(suaSoTay("tai-chinh.md", NHAN_TRA, themBuocTC(p9))), [
    `tai-chinh.md: màn tiền: bước trích «Đánh dấu đã trả», không phải lối vào hay lối ra nào: ${JSON.stringify(p9.slice(2))}`,
  ]);
  // The door first: every quote is read, not only those before the first door.
  const cuaTruoc = "- Bấm «Xem quyết toán», chuyển khoản xong thì bấm «Đánh dấu đã trả».";
  assert.deepEqual(kiemTatCa(suaSoTay("tai-chinh.md", NHAN_TRA, themBuocTC(cuaTruoc))), [
    `tai-chinh.md: màn tiền: bước trích «Đánh dấu đã trả», không phải lối vào hay lối ra nào: ${JSON.stringify(cuaTruoc.slice(2))}`,
  ]);
  // The line as it stood before this rule: «Chi theo nhóm» is the heading the
  // button sits under, not a door.
  const cu = "- Ở mục «Chi theo nhóm», bấm «Xem quyết toán» để mở màn quyết toán của nhóm.";
  assert.deepEqual(kiemTatCa(suaSoTay("tai-chinh.md", themNhanTC("Chi theo nhóm"), [BUOC2_TC, cu])), [
    `tai-chinh.md: màn tiền: bước trích «Chi theo nhóm», không phải lối vào hay lối ra nào: ${JSON.stringify(cu.slice(2))}`,
  ]);
  // Review 13 round 2, P5b: the heading declared as a way out. The extractor
  // pairs onAction with action only, so no «Chi theo nhóm» button of finance
  // leads to settlements/[id].
  assert.ok(NGU_CANH.rut.get("finance").nhan.includes("Chi theo nhóm"));
  assert.ok(!laCanhCoNhan("finance", "settlements/[id]", "Chi theo nhóm", NGU_CANH));
  const p5b = suaSoTay(
    "tai-chinh.md",
    themNhanTC("Chi theo nhóm"),
    ['{"nhan": "Xem quyết toán", "man": "settlements/[id]"}', '{"nhan": "Xem quyết toán", "man": "settlements/[id]"}, {"nhan": "Chi theo nhóm", "man": "settlements/[id]"}'],
    themBuocTC("- Đọc số ở «Chi theo nhóm» rồi chuyển khoản cho người ứng."),
  );
  assert.deepEqual(kiemTatCa(p5b), [
    "tai-chinh.md: màn tiền: lối ra «Chi theo nhóm» tới «settlements/[id]» không phải nút nào của «finance» dẫn tới đó",
    'tai-chinh.md: màn tiền: bước không chỉ lối vào hay lối ra nào: "Đọc số ở «Chi theo nhóm» rồi chuyển khoản cho người ứng."',
  ]);
});

test("(e) canary: front matter có khoá trùng bị từ chối, kể cả nhanUI thứ hai của review 13 vòng 2", () => {
  // Identity: keys repeat across objects (man at the top and in di_toi), and a
  // value may hold «{», «"» and «,» without being taken for structure.
  const kyTu = MAU_DUNG.replace('"tieu_de":"Lên plan"', '"tieu_de":"Lên \\"plan\\", {man}: [nhanUI]"');
  assert.notEqual(kyTu, MAU_DUNG);
  assert.deepEqual(kiemSoTay("ky-tu.md", kyTu, NGU_CANH), []);
  assert.equal(khoaTrung('{"a":{"b":1,"c":[{"b":2},{"b":3}]},"b":"}\\\\"}'), null);
  assert.equal(khoaTrung('{"a":1,"b":{"a":2},"b":3}'), "b");
  // An escaped quote inside a value does not end it.
  assert.equal(khoaTrung('{"a":"x\\"","b":1,"b":2}'), "b");
  const trung = MAU_DUNG.replace('"tien":false}', '"tien":false,"tien":false}');
  assert.deepEqual(kiemSoTay("trung.md", trung, NGU_CANH), ["trung.md: front matter có khoá trùng «tien»"]);
  // In a di_toi entry the last «man» is a real edge, so only this rule sees it.
  const trongDiToi = MAU_DUNG.replace('{"nhan":"Tạo kèo","man":"outings/new"}', '{"nhan":"Tạo kèo","man":"groups/new","man":"outings/new"}');
  assert.deepEqual(kiemSoTay("di-toi.md", trongDiToi, NGU_CANH), ["di-toi.md: front matter có khoá trùng «man»"]);
  // P9's second nhanUI on the real finance manual.
  const nhanUI2 = suaSoTay("tai-chinh.md", ['"tien": true', `"tien": true,\n  ${NHAN_TC.replace(/\]$/, ', "Đánh dấu đã trả"]')}`]);
  assert.deepEqual(kiemTatCa(nhanUI2), ["tai-chinh.md: front matter có khoá trùng «nhanUI»"]);
});

test("(e) canary: « và » phải thành cặp trên từng dòng, theo thứ tự", () => {
  // Each case breaks one part of the rule: a « left open at the end of the
  // line, a » with no « before it, a « opened again before it closed, marks
  // reversed, a quote across two lines, and one outside the steps.
  const loiDong = (ten, ...dong) => dong.map((d) => `${ten}: « và » không thành cặp trên dòng: ${JSON.stringify(d)}`);
  const buoc = (dong) => MAU_DUNG.replace("1. Bấm «Tạo kèo».", dong);
  assert.deepEqual(kiemSoTay("x.md", buoc("1. Bấm «Tạo kèo."), NGU_CANH), loiDong("x.md", "1. Bấm «Tạo kèo."));
  assert.deepEqual(kiemSoTay("x.md", buoc("1. Bấm «Tạo kèo»»."), NGU_CANH), loiDong("x.md", "1. Bấm «Tạo kèo»»."));
  assert.deepEqual(kiemSoTay("x.md", buoc("1. Bấm «Kèo bay «Tạo kèo»."), NGU_CANH), loiDong("x.md", "1. Bấm «Kèo bay «Tạo kèo»."));
  assert.deepEqual(kiemSoTay("x.md", buoc("1. Bấm »Tạo kèo«."), NGU_CANH), loiDong("x.md", "1. Bấm »Tạo kèo«."));
  // Across two lines the «…» match still reads «Tạo\nkèo», which is no label.
  assert.deepEqual(kiemSoTay("x.md", buoc("1. Bấm «Tạo\nkèo»."), NGU_CANH), [
    ...loiDong("x.md", "1. Bấm «Tạo", "kèo»."),
    'x.md: trích «Tạo\nkèo» mà nhãn không khai trong nhanUI',
  ]);
  const tongQuan = MAU_DUNG.replace("Tổng quan ngắn.", "Tổng quan ngắn, có nút »Tạo kèo«.");
  assert.notEqual(tongQuan, MAU_DUNG);
  assert.deepEqual(kiemSoTay("x.md", tongQuan, NGU_CANH), loiDong("x.md", "Tổng quan ngắn, có nút »Tạo kèo«."));
  // Review 13 round 3, probe Q1 exactly, on the real finance manual: the
  // payment button between reversed marks, beside a door.
  const q1 = "- Bấm «Xem quyết toán», chuyển khoản xong thì bấm »Đánh dấu đã trả«.";
  assert.deepEqual(kiemTatCa(suaSoTay("tai-chinh.md", themBuocTC(q1))), loiDong("tai-chinh.md", q1));
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
    // A heading that carries a button (Profile.tsx «Chi theo nhóm»): only
    // `action` names what `onAction` does; the heading is no button.
    '  <SectionHeader action={id !== null ? "Mở kèo" : undefined} onAction={id !== null ? () => router.push(("/outings/" + id) as never) : undefined} title="Kèo nhóm" />',
    '  <SectionHeader onAction={() => router.push("/messages")} title="Chỉ tiêu đề" />',
    '  <Card action="Nút của onAction" onPress={() => router.push("/messages")} />',
    // A row or a button tapped as a whole is named by its title or its
    // accessibility label.
    '  <ListRow title="Tài chính của tôi" onPress={() => router.push("/finance")} />',
    '  <IconButton accessibilityLabel="Mở Khám phá" onPress={() => router.push("/explore")} />',
    // Handlers that are not a tap of the thing.
    '  <Sheet accessibilityLabel="Tấm tạo mới" onClosed={() => router.replace("/plan")} />',
    '  <SearchBar placeholder="Tìm quán" onSubmitEditing={() => router.push("/explore")} />',
    '  <EmptyState title="Chưa có gì" action={{ label: "Tới Tin nhắn", onPress: () => router.replace("/messages" as never) }} />',
    '  <RudiButton label="Đánh dấu đã trả" onPress={() => danhDau(id)} />',
    '  <ListRow title="Hàng" right={<IconButton onPress={() => router.push("/plan")} />} />',
    '  <Pressable onPress={() => router.push("/plan")}><Text>Chữ con không phải thuộc tính</Text></Pressable>',
    "</View>;",
    'const MENU = [{ title: "Mục có chạm", onPress: () => router.push("/friends") }, { label: "Mục đóng lại", onClosed: () => router.push("/friends") }];',
  ].join("\n");
  const ra = rutTuNguon(nguon, [...CAC_MAN]);
  assert.deepEqual(ra.canh, [
    { den: "explore", nhan: "Mở Khám phá" },
    { den: "finance", nhan: "Tài chính của tôi" },
    { den: "friends", nhan: "Mục có chạm" },
    { den: "messages", nhan: "Tới Tin nhắn" },
    { den: "outings/[id]", nhan: "Mở kèo" },
    { den: "settlements/[id]", nhan: "Xem quyết toán" },
  ]);
  // The heading is still printed on the screen: a label, never a door.
  assert.ok(ra.nhan.includes("Kèo nhóm") && ra.nhan.includes("Chỉ tiêu đề"));
  // A button that leads nowhere is a label, never an edge.
  assert.ok(ra.nhan.includes("Đánh dấu đã trả") && !ra.canh.some((c) => c.nhan === "Đánh dấu đã trả"));
  // Neither an action object nor a render prop lends its navigation to the
  // tag's own title: only an on… handler (or href) of that tag is its tap.
  assert.ok(!ra.canh.some((c) => c.nhan === "Chưa có gì" || c.nhan === "Hàng"));
});
