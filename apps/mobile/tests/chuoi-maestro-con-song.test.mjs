/**
 * Chuỗi Maestro còn sống (kế hoạch UI v3 §2, §7): làm lại giao diện KHÔNG được
 * làm gãy một flow native mà container này không chạy được.
 *
 * Mỗi chữ, nhãn trợ năng và testID mà `.maestro/*.yaml` bấm, chờ hay kiểm phải
 * còn có mặt ở đâu đó nó có thể đến từ: mã app (`src/`, `app/`), dữ liệu seed
 * (`tools/seed-rudi-world*.mjs`), mã máy chủ (`services/`), gói dùng chung, hoặc
 * chính chữ mà flow gõ vào (`inputText`). Chuỗi Maestro là regex; phép kiểm
 * tách nó thành các mảnh chữ thật (bỏ `.*`, `\d+`, lớp ký tự...) và đòi MỖI
 * mảnh từ 3 ký tự còn nằm nguyên văn trong kho đó.
 *
 * Có chuỗi được dựng lúc chạy («An QA · Nuoc», «2 món · 200.000đ»): chúng nằm
 * trong `fixtures/chuoi-maestro-dong.json`, và danh sách đó chỉ được CO LẠI --
 * một chuỗi mới không tìm thấy là đỏ, một chuỗi trong danh sách mà nay tìm
 * thấy cũng đỏ (bắt người sửa gạch nó khỏi danh sách).
 *
 * Tên ảnh `takeScreenshot` được ghim nguyên bộ: đổi tên một ảnh là làm gãy
 * phép so ảnh trước/sau của Lead.
 *
 * Một phép khẳng định ÂM (`assertNotVisible`, `notVisible`) trên chữ đã bị XOÁ
 * có chủ đích là hợp lệ: nó chặn chữ đó quay lại. Những chữ như thế nằm trong
 * `daXoa` của fixture, mỗi chữ một lý do. Chữ trong `daXoa` chỉ được dùng ở
 * phép khẳng định âm, và nếu nó xuất hiện lại trong mã thì đỏ (hoặc nó đã quay
 * lại, hoặc danh sách đã cũ).
 *
 * Không chứng minh: flow chạy xanh. Đó là việc của máy thật.
 */
import assert from "node:assert/strict";
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";

const GOC = new URL("..", import.meta.url).pathname;
const REPO = new URL("../../..", import.meta.url).pathname;

function tep(dir, duoi) {
  const ra = [];
  const di = (d) => {
    for (const ten of readdirSync(d)) {
      if (ten === "node_modules" || ten.startsWith(".") || ten === "dist-test" || ten === "__pycache__") continue;
      const p = join(d, ten);
      const st = statSync(p);
      if (st.isDirectory()) di(p);
      else if (duoi.some((x) => ten.endsWith(x))) ra.push(p);
    }
  };
  di(dir);
  return ra;
}

const FLOW = readdirSync(join(GOC, ".maestro")).filter((f) => f.endsWith(".yaml")).sort();
const NGUON_FLOW = Object.fromEntries(FLOW.map((f) => [f, readFileSync(join(GOC, ".maestro", f), "utf8")]));

/** The value of one YAML scalar on a line: quoted or bare, with quotes and escapes undone. */
function giaTri(tho) {
  const s = tho.trim();
  if (s.startsWith('"') && s.endsWith('"')) return s.slice(1, -1).replace(/\\"/g, '"').replace(/\\\\/g, "\\");
  if (s.startsWith("'") && s.endsWith("'")) return s.slice(1, -1).replace(/''/g, "'");
  return s;
}

function trichChuoi() {
  const chu = [];
  const id = [];
  const anh = [];
  const go = [];
  for (const [f, nguon] of Object.entries(NGUON_FLOW)) {
    for (const dong of nguon.split("\n")) {
      if (/^\s*#/.test(dong)) continue;
      let m;
      if ((m = /^\s*-?\s*takeScreenshot:\s*(.+)$/.exec(dong))) anh.push(giaTri(m[1]));
      else if ((m = /^\s*-?\s*inputText:\s*(.+)$/.exec(dong))) go.push(giaTri(m[1]));
      else if ((m = /^\s*-?\s*id:\s*(.+)$/.exec(dong))) id.push({ f, s: giaTri(m[1]) });
      else if ((m = /^\s*-?\s*(tapOn|assertVisible|assertNotVisible|longPressOn|doubleTapOn|visible|notVisible|text|element):\s*(.+)$/.exec(dong))) {
        const s = giaTri(m[2]);
        const am = m[1] === "assertNotVisible" || m[1] === "notVisible";
        if (s !== "" && !s.startsWith("{") && !s.startsWith("[") && !/^\$\{/.test(s)) chu.push({ f, s, am });
      }
    }
  }
  return { chu, id, anh, go };
}

// An escaped metacharacter is a literal: each is parked on a private-use
// character while the regex is cut into pieces, then put back.
const THOAT = ".()?*+[]|^$\\";
const cat = (c) => String.fromCharCode(0xe000 + THOAT.indexOf(c));
const traVe = (s) => s.replace(/[\ue000-\ue00c]/g, (c) => THOAT[c.charCodeAt(0) - 0xe000]);

/** The literal pieces a Maestro regex needs, each alternative separately. */
function manhChu(chuoi) {
  const ra = [];
  const an = chuoi.replace(/\\([.()?*+[\]|^$\\])/g, (_, c) => cat(c));
  for (const nhanh of an.split("|")) {
    const manh = nhanh
      .split(/\.\*|\.\+|\\d\+?|\\s\+?|\[[^\]]*\][*+?]?|[()^$?*+]|\{\d+(?:,\d*)?\}/)
      .map((x) => traVe(x).trim())
      .filter((x) => x.length >= 3);
    ra.push(manh);
  }
  return ra;
}

const KHO = (() => {
  const phan = [];
  for (const f of [...tep(join(GOC, "src"), [".ts", ".tsx"]), ...tep(join(GOC, "app"), [".ts", ".tsx"])]) phan.push(readFileSync(f, "utf8"));
  for (const f of readdirSync(join(GOC, "tools")).filter((x) => x.startsWith("seed-"))) phan.push(readFileSync(join(GOC, "tools", f), "utf8"));
  for (const d of ["services/core", "services/api/app", "packages/shared"]) {
    try {
      for (const f of tep(join(REPO, d), [".go", ".py", ".json", ".ts"])) phan.push(readFileSync(f, "utf8"));
    } catch {
      // a checkout without the server tree still checks against the app
    }
  }
  return phan.join("\n");
})();

const { chu, id, anh, go } = trichChuoi();
const KHO_DAY_DU = `${KHO}\n${go.join("\n")}`;

/** A string is alive if SOME alternative has every literal piece in the corpus. */
function conSong(s) {
  const nhanh = manhChu(s);
  if (nhanh.every((m) => m.length === 0)) return true;
  return nhanh.some((m) => m.length > 0 && m.every((x) => KHO_DAY_DU.includes(x)));
}

const DONG = JSON.parse(readFileSync(new URL("./fixtures/chuoi-maestro-dong.json", import.meta.url), "utf8"));

const DA_XOA = DONG.daXoa ?? {};

test("mọi chữ Maestro bấm hay kiểm còn có mặt trong mã, seed, máy chủ hay chữ flow gõ vào", () => {
  assert.ok(chu.length > 400, `chỉ trích được ${chu.length} chuỗi: bộ đọc YAML đã hỏng`);
  const chet = [...new Set(chu.filter(({ s }) => !conSong(s)).map(({ s }) => s))].sort();
  const moi = chet.filter((s) => !DONG.dong.includes(s) && !(s in DA_XOA));
  assert.deepEqual(moi, [], `chuỗi Maestro không còn ở đâu (đổi tên nhãn thì sửa flow cùng commit):\n${moi.join("\n")}`);
  const daSong = DONG.dong.filter((s) => !chet.includes(s));
  assert.deepEqual(daSong, [], `chuỗi trong danh sách «dựng động» mà nay đã tìm thấy: gạch nó khỏi fixtures/chuoi-maestro-dong.json:\n${daSong.join("\n")}`);
});

test("chữ đã xoá có chủ đích: chỉ ở phép khẳng định âm, có lý do, và không quay lại trong mã", () => {
  for (const [s, lyDo] of Object.entries(DA_XOA)) {
    assert.ok(typeof lyDo === "string" && lyDo.length > 20, `«${s}» trong daXoa thiếu lý do`);
    const dung = chu.filter((c) => c.s === s);
    assert.ok(dung.length > 0, `«${s}» trong daXoa mà không flow nào còn dùng: gạch khỏi danh sách`);
    const duong = dung.filter((c) => !c.am).map((c) => c.f);
    assert.deepEqual(duong, [], `«${s}» đã xoá mà flow vẫn chờ hay bấm nó (không chỉ khẳng định âm)`);
    assert.ok(!conSong(s), `«${s}» lại có trong mã: hoặc nó quay lại, hoặc gạch khỏi daXoa`);
  }
});

test("mọi testID Maestro dùng còn trong mã", () => {
  const chet = [...new Set(id.filter(({ s }) => !conSong(s)).map(({ s }) => s))].sort();
  const moi = chet.filter((s) => !DONG.id.includes(s));
  assert.deepEqual(moi, [], `testID Maestro không còn trong mã:\n${moi.join("\n")}`);
});

test("tên ảnh takeScreenshot được ghim nguyên bộ", () => {
  assert.deepEqual([...anh].sort(), [...DONG.anh].sort(), "đổi, thêm hay bớt tên ảnh là làm gãy phép so ảnh trước/sau: sửa flow và danh sách cùng commit");
});

test("bộ tách mảnh chữ: regex thành mảnh chữ thật, mỗi nhánh riêng", () => {
  assert.deepEqual(manhChu("Bạn nhập tay 2 món.*"), [["Bạn nhập tay 2 món"]]);
  assert.deepEqual(manhChu("Rủ Đi thôi!|Khám phá"), [["Rủ Đi thôi!"], ["Khám phá"]]);
  assert.deepEqual(manhChu("Còn \\d+ ngày"), [["Còn", "ngày"]]);
  assert.deepEqual(manhChu("Đã trả \\(1/2\\)"), [["Đã trả (1/2)"]]);
});
