import assert from "node:assert/strict";
import { readFileSync, readdirSync, statSync } from "node:fs";
import { join } from "node:path";
import { test } from "node:test";

import { KHOA_SO_LIEU } from "../dist-test/rudi/nep/phieu.js";

/**
 * `donPhieu` drops a stray field at runtime, which keeps the server clean but
 * tells the person who wrote it nothing. This is the other half: the call sites
 * themselves. A screen that types `tenThanhVien` should turn CI red, not
 * silently lose the field and read as working.
 */

const KHOA_TREN_CUNG = new Set(["man", "tieuDe", "nhip", "loaiSo", "soLieu", "goiY"]);
const GOC = new URL("../src/", import.meta.url).pathname;

function moiFile(thuMuc) {
  const ra = [];
  for (const ten of readdirSync(thuMuc)) {
    const duong = join(thuMuc, ten);
    if (statSync(duong).isDirectory()) ra.push(...moiFile(duong));
    else if (/\.tsx?$/.test(ten)) ra.push(duong);
  }
  return ra;
}

/** Text between the parens of the first `useNepNguCanh(` in `noiDung`, balanced. */
function doiSo(noiDung, tu) {
  let sau = 0;
  let i = tu;
  for (; i < noiDung.length; i++) {
    if (noiDung[i] === "(") sau++;
    else if (noiDung[i] === ")") {
      sau--;
      if (sau === 0) break;
    }
  }
  return noiDung.slice(tu + 1, i);
}

/** Keys written at brace depth 1 of an object literal. */
function khoaTang1(chu) {
  const ra = [];
  let sau = 0;
  const re = /([{}])|(?:^|[{,\s])([A-Za-z_$][\w$]*)\s*:/g;
  let m;
  while ((m = re.exec(chu))) {
    if (m[1] === "{") sau++;
    else if (m[1] === "}") sau--;
    else if (m[2] && sau === 1) ra.push(m[2]);
  }
  return ra;
}

const FILES = moiFile(GOC);

test("mọi lời gọi useNepNguCanh chỉ dùng khoá trong danh sách đóng", () => {
  const pham = [];
  for (const f of FILES) {
    const noiDung = readFileSync(f, "utf8");
    let tu = noiDung.indexOf("useNepNguCanh(");
    while (tu !== -1) {
      const arg = doiSo(noiDung, tu + "useNepNguCanh".length);
      for (const k of khoaTang1(arg)) {
        if (!KHOA_TREN_CUNG.has(k)) pham.push(`${f.replace(GOC, "src/")}: khoá lạ «${k}»`);
      }
      tu = noiDung.indexOf("useNepNguCanh(", tu + 1);
    }
  }
  assert.deepEqual(pham, [], `phiếu ngữ cảnh là danh sách đóng:\n${pham.join("\n")}`);
});

test("soLieu chỉ đếm những thứ đã khai, không mang tên người và không mang tiền", () => {
  const pham = [];
  for (const f of FILES) {
    const noiDung = readFileSync(f, "utf8");
    const re = /soLieu\s*:\s*\{/g;
    let m;
    while ((m = re.exec(noiDung))) {
      const mo = m.index + m[0].length - 1;
      let sau = 0;
      let i = mo;
      for (; i < noiDung.length; i++) {
        if (noiDung[i] === "{") sau++;
        else if (noiDung[i] === "}") {
          sau--;
          if (sau === 0) break;
        }
      }
      for (const k of khoaTang1(noiDung.slice(mo, i + 1))) {
        if (!KHOA_SO_LIEU.includes(k)) pham.push(`${f.replace(GOC, "src/")}: soLieu.${k}`);
      }
    }
  }
  assert.deepEqual(pham, [], `khoá số liệu ngoài danh sách:\n${pham.join("\n")}`);
});

test("cổng này chỉ có nghĩa khi nó thật sự đọc được nguồn", () => {
  assert.ok(FILES.length > 200, `chỉ quét được ${FILES.length} file, đường dẫn hỏng`);
  assert.ok(FILES.some((f) => f.endsWith("NepProvider.tsx")), "không thấy NepProvider.tsx");
});
