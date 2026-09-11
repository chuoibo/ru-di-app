/* Shared validator for drawn layers: parse `d` the way Java's PathParser does,
 * refuse what it would refuse, and keep every coordinate inside the box.
 * Extracted from art-duong.test.mjs so art-ky-hoa.test.mjs runs the same gate. */
import assert from "node:assert/strict";

import { MAU_VE } from "../dist-test/rudi/art/net.js";

const ARITY = { M: 2, L: 2, C: 6, Z: 0 };
const SO = /^-?\d+(\.\d+)?$/;

/** Parse like the Java side: throw on anything it would refuse. */
export function phanTich(d) {
  const tokens = d.trim().split(/[\s,]+/);
  const cmds = [];
  let i = 0;
  while (i < tokens.length) {
    const c = tokens[i++];
    assert.ok(c in ARITY, `lệnh lạ «${c}» trong: ${d.slice(0, 60)}`);
    const args = [];
    for (let k = 0; k < ARITY[c]; k++) {
      const t = tokens[i++];
      assert.ok(t !== undefined && SO.test(t), `lệnh ${c} thiếu/sai số «${t}» trong: ${d.slice(0, 60)}`);
      args.push(Number(t));
    }
    cmds.push({ c, args });
  }
  return cmds;
}

/** Every layer of a drawing: grammar, closure, box, colour role, stroke width. */
export function kiemLop(ten, lop, w, h, biên = 1) {
  assert.ok(Array.isArray(lop) && lop.length > 0, `${ten}: không có lớp nào`);
  for (const [i, l] of lop.entries()) {
    const nhan = `${ten} lớp ${i}`;
    const cmds = phanTich(l.d);
    assert.equal(cmds[0].c, "M", `${nhan}: phải mở bằng M`);
    assert.ok(!/L\s+Z/.test(l.d) && !/(^|\s)-0(?=\s|$)/.test(l.d) && !/e/.test(l.d), `${nhan}: ${l.d.slice(0, 40)}`);
    if (l.net === undefined) {
      assert.equal(cmds.at(-1).c, "Z", `${nhan}: một lớp tô phải kín`);
      assert.notEqual(cmds.at(-2).c, "M", `${nhan}: Z phải đứng sau một điểm thật`);
    } else {
      assert.ok(Number.isFinite(l.net) && l.net > 0, `${nhan}: nét ${l.net}`);
      assert.ok(cmds.length >= 2, `${nhan}: một nét phải có ít nhất một đoạn`);
    }
    assert.ok(MAU_VE.includes(l.mau), `${nhan}: vai màu lạ ${l.mau}`);
    for (const { args } of cmds) {
      for (let k = 0; k < args.length; k += 2) {
        assert.ok(args[k] >= -biên && args[k] <= w + biên, `${nhan}: x ngoài hộp ${args[k]}`);
        assert.ok(args[k + 1] >= -biên && args[k + 1] <= h + biên, `${nhan}: y ngoài hộp ${args[k + 1]}`);
      }
    }
  }
}
