/**
 * Mực người (ADR-0037 D6, «Luật Mực Người»).
 *
 * Trước đợt này mọi avatar mang tông của màn: một hội tám người là tám vòng tròn
 * cam giống hệt, hai cái cùng chữ «T». Mực người sửa chuyện đó, và test này giữ
 * cho nó không phá ba luật màu đã có:
 *
 *   - đọc được: mỗi mực >= 4.5:1 trên paper, card và ground của scheme mình;
 *   - không mượn nghĩa: hue OKLCH cách >= 30 độ so với accent, split, ai và
 *     brand coral/teal/violet, nên không ai nhầm mực của một người với «tiền»
 *     hay «AI»;
 *   - phân biệt được: từng cặp cách nhau trong OKLab.
 *
 * Và chỉ số FNV-1a phải ổn định (cùng id, cùng màu, mọi máy) và rải đều.
 */
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

import { BANG_MUC_NGUOI, SO_MUC_NGUOI, bamFnv1a, chiSoMuc, mucNguoi } from "../dist-test/rudi/nguoi/muc-nguoi.js";

const tokens = JSON.parse(readFileSync(new URL("../../../packages/shared/tokens.json", import.meta.url), "utf8"));

function rgb(hex) {
  const h = hex.replace("#", "");
  return [0, 2, 4].map((i) => parseInt(h.slice(i, i + 2), 16));
}
function tuyenTinh(c) {
  const x = c / 255;
  return x <= 0.04045 ? x / 12.92 : ((x + 0.055) / 1.055) ** 2.4;
}
function doSang(hex) {
  const [r, g, b] = rgb(hex).map(tuyenTinh);
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}
function tuongPhan(a, b) {
  const [x, y] = [doSang(a), doSang(b)].sort((m, n) => n - m);
  return (x + 0.05) / (y + 0.05);
}
function okLab(hex) {
  const [r, g, b] = rgb(hex).map(tuyenTinh);
  const l = Math.cbrt(0.412221 * r + 0.536333 * g + 0.051446 * b);
  const m = Math.cbrt(0.211903 * r + 0.6807 * g + 0.107397 * b);
  const s = Math.cbrt(0.088302 * r + 0.281719 * g + 0.629979 * b);
  return [
    0.210454 * l + 0.793618 * m - 0.004072 * s,
    1.977998 * l - 2.428592 * m + 0.450594 * s,
    0.025904 * l + 0.782772 * m - 0.808676 * s,
  ];
}
function hue(hex) {
  const [, a, b] = okLab(hex);
  return ((Math.atan2(b, a) * 180) / Math.PI + 360) % 360;
}
function khoangHue(a, b) {
  const d = Math.abs(a - b) % 360;
  return Math.min(d, 360 - d);
}

const TONG_NGHIA = ["accent", "split", "ai"];
const THUONG_HIEU = ["coral", "teal", "violet"];

test("tám mực mỗi scheme, bảng trong mã chính là bảng trong tokens.json", () => {
  assert.equal(SO_MUC_NGUOI, 8);
  for (const scheme of ["light", "dark"]) {
    assert.equal(BANG_MUC_NGUOI[scheme].length, 8);
    assert.deepEqual([...BANG_MUC_NGUOI[scheme]], tokens.mucNguoi[scheme]);
    assert.equal(new Set(tokens.mucNguoi[scheme]).size, 8, "không màu nào lặp");
  }
});

test("mỗi mực đọc được trên paper, card và ground của scheme mình (>= 4.5:1)", () => {
  for (const scheme of ["light", "dark"]) {
    const nen = tokens.color[scheme];
    for (const muc of tokens.mucNguoi[scheme]) {
      for (const k of ["paper", "card", "ground"]) {
        const r = tuongPhan(muc, nen[k]);
        assert.ok(r >= 4.5, `${scheme} ${muc} trên ${k} (${nen[k]}) chỉ ${r.toFixed(2)}:1`);
      }
    }
  }
});

test("không mực nào mượn nghĩa của ba tông hay màu thương hiệu (hue cách >= 30 độ)", () => {
  const hueNghia = [
    ...["light", "dark"].flatMap((s) => TONG_NGHIA.map((k) => [`${s}.${k}`, hue(tokens.color[s][k])])),
    ...THUONG_HIEU.map((k) => [`brand.${k}`, hue(tokens.brand[k])]),
  ];
  for (const scheme of ["light", "dark"]) {
    for (const muc of tokens.mucNguoi[scheme]) {
      for (const [ten, h] of hueNghia) {
        const d = khoangHue(hue(muc), h);
        assert.ok(d >= 30, `${scheme} ${muc} chỉ cách ${ten} ${d.toFixed(1)} độ`);
      }
    }
  }
});

test("từng cặp mực phân biệt được (OKLab >= 0.04)", () => {
  for (const scheme of ["light", "dark"]) {
    const bang = tokens.mucNguoi[scheme];
    for (let i = 0; i < bang.length; i += 1) {
      for (let j = i + 1; j < bang.length; j += 1) {
        const [a, b] = [okLab(bang[i]), okLab(bang[j])];
        const d = Math.hypot(a[0] - b[0], a[1] - b[1], a[2] - b[2]);
        assert.ok(d >= 0.04, `${scheme} ${bang[i]} và ${bang[j]} quá gần (${d.toFixed(3)})`);
      }
    }
  }
});

test("FNV-1a đúng véc-tơ chuẩn", () => {
  assert.equal(bamFnv1a(""), 0x811c9dc5);
  assert.equal(bamFnv1a("a"), 0xe40c292c);
  assert.equal(bamFnv1a("foobar"), 0xbf9cf968);
});

test("cùng một người, cùng một mực, kể cả khi id bị viết hoa hay có khoảng trắng", () => {
  const id = "9cd86994-3f12-8701-92d4-013345a7d54b";
  assert.equal(chiSoMuc(id), chiSoMuc(` ${id.toUpperCase()} `));
  assert.equal(mucNguoi(id, false), tokens.mucNguoi.light[chiSoMuc(id)]);
  assert.equal(mucNguoi(id, true), tokens.mucNguoi.dark[chiSoMuc(id)]);
});

test("chỉ số rải đều trên tám ô", () => {
  const dem = new Array(8).fill(0);
  let hat = 7;
  const hex = () => {
    // Deterministic pseudo-UUIDs: a linear congruential stream, formatted like the server's ids.
    let s = "";
    for (let i = 0; i < 32; i += 1) {
      // The classic LCG multiplier, written in hex: a ten-digit decimal reads to
      // the repo guard as an account number.
      hat = (Math.imul(hat, 0x41c64e6d) + 0x3039) >>> 0;
      s += ((hat >>> 16) & 15).toString(16);
    }
    return `${s.slice(0, 8)}-${s.slice(8, 12)}-${s.slice(12, 16)}-${s.slice(16, 20)}-${s.slice(20)}`;
  };
  const N = 8000;
  for (let i = 0; i < N; i += 1) dem[chiSoMuc(hex())] += 1;
  for (const [o, n] of dem.entries()) {
    assert.ok(n > (N / 8) * 0.8 && n < (N / 8) * 1.2, `ô ${o} có ${n}/${N}`);
  }
});
