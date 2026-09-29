/* Numbers in the ledger's prose, made safe for the repository guard.
 *
 * The guard's long-number rule rejects nine or more digits in one run, with
 * single dots between them counted as part of the run (a raw map coordinate
 * once slipped into the matrix: incident 1). Only such runs that are plain
 * decimals, one dot, are rounded, to one place. Everything else is printed as
 * the ledger wrote it. Above all Vietnamese amounts, which group thousands with
 * dots: «75.000đ» is seventy-five thousand. The first version rounded every
 * dotted number and printed «75đ» and «13.7.678đ» (incident 4, checkpoint 13).
 * A grouped amount of nine digits or more is left for the guard to stop.
 */
export function lamTron(s) {
  return String(s ?? "").replace(/\d+\.\d+(?:\.\d+)*/g, (m) =>
    m.replace(/\./g, "").length >= 9 && /^\d+\.\d+$/.test(m) ? String(Math.round(Number(m) * 10) / 10) : m,
  );
}
