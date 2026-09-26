/**
 * The catalogue names of the places a sheet's stops point at.
 *
 * A stop carries only the place's catalogue id; the sheet used to print the
 * stop's own line («Ăn tối») and nothing about where, even after Nếp's draft
 * chose a place and said so in its reason (2026-09-24). Names are read from
 * `GET /places/{id}` once per id per app run: the catalogue is public and a
 * sheet is re-rendered on every notebook read. An id the catalogue no longer
 * has, or a read that fails, simply draws no name -- the stop's own line is
 * still the truth of what was agreed.
 */
import { useEffect, useState } from "react";

import { docChiTiet } from "../kham-pha/dia-diem";

const daBiet = new Map<string, string | null>();

export function useTenCho(ids: readonly (string | null | undefined)[]): Readonly<Record<string, string>> {
  const can = [...new Set(ids.filter((id): id is string => typeof id === "string" && id !== ""))].sort();
  const khoa = can.join("\n");
  const [ten, setTen] = useState<Record<string, string>>(() => docSan(can));

  useEffect(() => {
    let song = true;
    const thieu = can.filter((id) => !daBiet.has(id));
    setTen(docSan(can));
    if (thieu.length === 0) return;
    void Promise.allSettled(thieu.map((id) => docChiTiet(id))).then((ket) => {
      ket.forEach((k, i) => daBiet.set(thieu[i], k.status === "fulfilled" ? k.value.name : null));
      if (song) setTen(docSan(can));
    });
    return () => {
      song = false;
    };
    // `khoa` is `can` spelled as a string, the identity the effect depends on.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [khoa]);

  return ten;
}

function docSan(ids: readonly string[]): Record<string, string> {
  const out: Record<string, string> = {};
  for (const id of ids) {
    const name = daBiet.get(id);
    if (name) out[id] = name;
  }
  return out;
}
