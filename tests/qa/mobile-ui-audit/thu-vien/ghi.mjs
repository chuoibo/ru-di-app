/* The audit's ledger: one JSON line per (test case × configuration group).
 *
 * `tong-hop.mjs` builds the coverage matrix and every count in the report from
 * this file alone, so a status that is not written here does not exist.
 * Status is one of the five the audit allows; anything else is refused at write
 * time rather than discovered in the report.
 */
import { appendFileSync, existsSync, mkdirSync, readFileSync } from "node:fs";
import { join } from "node:path";

export const TRANG_THAI = Object.freeze(["NOT_TESTED", "PASS", "FAIL", "BLOCKED", "NOT_APPLICABLE"]);
export const PHUONG_PHAP = Object.freeze(["RUNTIME-WEB", "STATIC", "HYPOTHESIS"]);

export function soGhi(out) {
  mkdirSync(out, { recursive: true });
  const file = join(out, "results.jsonl");
  return {
    file,
    ghi(rec) {
      for (const k of ["tc", "feature", "screen", "status", "method"]) {
        if (!rec[k]) throw new Error(`thiếu ${k} trong ${JSON.stringify(rec).slice(0, 120)}`);
      }
      if (!TRANG_THAI.includes(rec.status)) throw new Error(`status lạ: ${rec.status}`);
      if (!PHUONG_PHAP.includes(rec.method)) throw new Error(`method lạ: ${rec.method}`);
      if (rec.status === "PASS" && rec.method !== "RUNTIME-WEB") throw new Error(`PASS chỉ cho ca đã chạy runtime: ${rec.tc}`);
      appendFileSync(file, `${JSON.stringify({ ...rec, luc: new Date().toISOString() })}\n`);
    },
    /**
     * Withdraw every earlier row of a test case whose verdict came from a
     * harness defect (a wrong selector, a wrong button name). The rows stay in
     * the ledger; the matrix drops them and lists the withdrawal with its
     * reason, so a FAIL never disappears without a written cause.
     */
    rut(tc, lyDo) {
      if (!tc || !lyDo) throw new Error("rút hàng cần tc và lý do");
      appendFileSync(file, `${JSON.stringify({ tc, rut: true, lyDo, luc: new Date().toISOString() })}\n`);
    },
    doc() {
      if (!existsSync(file)) return [];
      return readFileSync(file, "utf8")
        .split("\n")
        .filter(Boolean)
        .map((l) => JSON.parse(l));
    },
  };
}
