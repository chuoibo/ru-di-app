/* Evidence pointers for the F00/F01 baseline rows judged at checkpoint 2.
 * F01 had no judgement script then; its baseline verdicts were written
 * inline, and three of them named the wrong capture or none at all, so the
 * committed contact sheets were linked only from the generated manifest.
 * `kiem-tai-lieu.mjs` found this at checkpoint 10. Each row is copied
 * unchanged except for `evidence` and a note; the verdicts stand, and the
 * captures were opened again before re-pointing.
 *
 *   node kich-ban/f01-phan-xu.mjs
 */
import { soGhi } from "../thu-vien/ghi.mjs";

const out = process.env.AUDIT_OUT;
if (!out) throw new Error("thiếu AUDIT_OUT");
const so = soGhi(out);

const cuoiCung = new Map();
for (const r of so.doc()) {
  if (r.rut) {
    for (const k of [...cuoiCung.keys()]) if (k.startsWith(`${r.tc}|`)) cuoiCung.delete(k);
    continue;
  }
  cuoiCung.set(`${r.tc}|${r.nenTang ?? "web"}|${r.cauHinh ?? ""}`, r);
}

const ganBangChung = (tc, cauHinh, evidence, lyDo) => {
  const r = cuoiCung.get(`${tc}|web|${cauHinh}`);
  if (!r) throw new Error(`không có hàng ${tc} ${cauHinh}`);
  if (JSON.stringify(r.evidence) === JSON.stringify(evidence)) return; // already re-pointed
  const { luc: _luc, ...giu } = r;
  so.ghi({ ...giu, evidence, ghiChu: `${r.ghiChu ?? ""}; bằng chứng gắn lại ở checkpoint 10: ${lyDo}` });
};

// The `/` sheet (C1–C3, redirected to Welcome without a session) was named
// as the evidence of the Welcome row, which is judged on four configurations.
ganBangChung("TC-F00.S01-BASE", "C1,C2,C3", ["EV-F00.S01-BASE-C1", "EV-F00.S01-BASE-ghep"], "thêm ảnh ghép của `/` ở C1–C3 (không phiên thì hiện Welcome)");
ganBangChung("TC-F01.S01-BASE", "C1,C2,C3,C8", ["EV-F01.S01-BASE-ghep"], "ảnh ghép `/welcome` ở C1, C2, C3, C8; trước đây trỏ nhầm ảnh của `/` (không có C8)");
ganBangChung("TC-F01.S04-BASE", "C1,C2,C3,C8", ["EV-F01.S04-BASE-ghep"], "ảnh ghép `/moi` ở C1, C2, C3, C8; trước đây để trống");
ganBangChung("TC-F01.S05-BASE", "C1,C2,C3,C8", ["EV-F01.S05-BASE-ghep"], "ảnh ghép `/personalization` ở C1, C2, C3, C8; trước đây để trống");
