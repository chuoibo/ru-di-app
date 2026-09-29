/* Compare a re-run ledger with the main ledger, key by key
 * (test case | platform | configuration group), the way the final
 * verification re-ran part of the audit in a clean worktree.
 *
 *   node so-sanh-so.mjs <main results.jsonl> <re-run results.jsonl>
 *
 * Two readings, because the main ledger holds two kinds of rows. A verdict
 * reached by looking replaces the automatic one: since F02 such rows start with
 * «phân xử bằng mắt:», earlier ones (checkpoint 1) do not. So each re-run row is
 * compared with the LAST automatic row of its key (what the matrix shows unless
 * a later verdict overrode it) and with the FIRST one (the measurement as first
 * taken, before any withdrawal and re-measure). Millisecond timings vary run to
 * run and are masked before notes are compared.
 */
import { readFileSync } from "node:fs";

const [fChinh, fLai] = process.argv.slice(2);
if (!fChinh || !fLai) throw new Error("dùng: node so-sanh-so.mjs <sổ chính> <sổ chạy lại>");
const doc = (f) => readFileSync(f, "utf8").split("\n").filter(Boolean).map((l) => JSON.parse(l));
const khoa = (r) => `${r.tc}|${r.nenTang ?? "web"}|${r.cauHinh ?? ""}`;
const boMs = (s) => String(s ?? "").replace(/\d+ ms/g, "N ms").replace(/sau khoảng [\d,]+s/g, "sau N s");
const tuDong = (r) => !r.rut && !String(r.ghiChu ?? "").startsWith("phân xử bằng mắt:");

const chinh = doc(fChinh).filter(tuDong);
const lai = new Map();
for (const r of doc(fLai)) if (!r.rut) lai.set(khoa(r), r);

const dem = { cuoiKhop: 0, cuoiLech: 0, dauGiong: 0, dauKhac: 0, khongCo: 0 };
for (const [k, r] of lai) {
  const cung = chinh.filter((c) => khoa(c) === k);
  if (!cung.length) {
    dem.khongCo++;
    console.log(`KHÔNG CÓ  ${k}: ${r.status} (sổ chính không có hàng tự động)`);
    continue;
  }
  const cuoi = cung[cung.length - 1];
  const dau = cung[0];
  if (cuoi.status === r.status) dem.cuoiKhop++;
  else {
    dem.cuoiLech++;
    console.log(`LỆCH CUỐI ${k}: sổ chính ${cuoi.status}, chạy lại ${r.status}\n  chính: ${boMs(cuoi.ghiChu).slice(0, 200)}\n  lại:   ${boMs(r.ghiChu).slice(0, 200)}`);
  }
  if (dau.status === r.status && boMs(dau.ghiChu) === boMs(r.ghiChu)) dem.dauGiong++;
  else {
    dem.dauKhac++;
    console.log(`KHÁC ĐẦU  ${k}: hàng đầu ${dau.status}, chạy lại ${r.status}\n  đầu: ${boMs(dau.ghiChu).slice(0, 200)}\n  lại: ${boMs(r.ghiChu).slice(0, 200)}`);
  }
}
console.log(JSON.stringify({ hang: lai.size, ...dem }));
