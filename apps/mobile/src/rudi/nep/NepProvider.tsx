import { usePathname } from "expo-router";
import { createContext, useCallback, useContext, useEffect, useMemo, useReducer, useRef, useState } from "react";
import type { ReactNode } from "react";

import { docGiaoDienAsync, ghiGiaoDienAsync } from "../kho";
import { KHOA_DOCK, giaiMaDock, maHoaDock } from "./luu-dock";
import { donPhieu, nepPhaiLui, type PhieuNguCanh } from "./phieu";
import { DOCK_DAU, chuyen, type DockNep, type SuKienNep } from "./trang-thai";

/**
 * The one instance of Nếp, mounted once in `app/_layout.tsx`.
 *
 * It sits INSIDE `RudiSessionProvider` so it can read the session, and OUTSIDE
 * `<Stack>` so a single Nếp serves every route instead of one per screen
 * mounting and losing its position on each push.
 *
 * Two jobs, and they are deliberately separate:
 *   - it owns the dock state machine (`trang-thai.ts`) and the rail position;
 *   - it collects the CONTEXT SLIP each screen declares (`phieu.ts`).
 *
 * The slip is declared, never inferred. `usePathname()` is a fallback that
 * tells Nếp only which route is open, because `CLAUDE.md` forbids the
 * assistant reading conversation or taste on its own, and chat v2 is end to
 * end encrypted anyway. A screen that wants Nếp to know more says so.
 */

export interface NepDieuKhien {
  dock: DockNep;
  /** What the open screen declared, already through the whitelist. */
  phieu: PhieuNguCanh | null;
  /** Position along the rail, 0..1. */
  tyLe: number;
  /** Whether the stored position has been read yet; the dock waits for it. */
  daDocDia: boolean;
  gui(su: SuKienNep): void;
  datTyLe(t: number): void;
  datPhieu(p: PhieuNguCanh | null): void;
}

const NepContext = createContext<NepDieuKhien | null>(null);

export function NepProvider({ children }: { children: ReactNode }) {
  const [dock, gui] = useReducer(chuyen, DOCK_DAU);
  const [phieu, datPhieu] = useState<PhieuNguCanh | null>(null);
  const [tyLe, datTyLeRaw] = useState(0.62);
  const [daDocDia, datDaDocDia] = useState(false);
  const duong = usePathname();

  // Read once. A failed read is indistinguishable from a first launch and both
  // answers are the same: the default position.
  useEffect(() => {
    let song = true;
    void docGiaoDienAsync(KHOA_DOCK).then((raw) => {
      if (!song) return;
      const daLuu = giaiMaDock(raw);
      if (daLuu) {
        datTyLeRaw(daLuu.tyLe);
        if (daLuu.an) gui({ kieu: "vuot-ra" });
      }
      datDaDocDia(true);
    });
    return () => {
      song = false;
    };
  }, []);

  // The law, applied on every route change: money, errors and conflict are
  // screens Nếp stands away from (DESIGN.md «Luật Nếp Đứng Xa Tiền», ADR-0033).
  useEffect(() => {
    gui({ kieu: "doi-man", nepLui: nepPhaiLui(duong ?? "") });
  }, [duong]);

  // A screen's slip belongs to that screen. Clearing on route change means a
  // screen that forgets to declare cannot inherit the previous one's numbers
  // and have Nếp answer about a trip the person already left.
  const duongTruoc = useRef(duong);
  useEffect(() => {
    if (duongTruoc.current !== duong) {
      duongTruoc.current = duong;
      datPhieu(null);
    }
  }, [duong]);

  const datTyLe = useCallback((t: number) => {
    const sach = Number.isFinite(t) ? Math.min(1, Math.max(0, t)) : 0;
    datTyLeRaw(sach);
  }, []);

  // Persist the two facts worth persisting, after the first read so the default
  // never overwrites what the disk holds.
  useEffect(() => {
    if (!daDocDia) return;
    void ghiGiaoDienAsync(KHOA_DOCK, maHoaDock({ tyLe, an: dock.nen === "an" }));
  }, [daDocDia, tyLe, dock.nen]);

  const gia = useMemo<NepDieuKhien>(
    () => ({ dock, phieu, tyLe, daDocDia, gui, datTyLe, datPhieu }),
    [dock, phieu, tyLe, daDocDia, datTyLe],
  );

  return <NepContext.Provider value={gia}>{children}</NepContext.Provider>;
}

export function useNep(): NepDieuKhien {
  const gia = useContext(NepContext);
  if (!gia) throw new Error("useNep phải nằm trong NepProvider");
  return gia;
}

/**
 * What a screen calls to tell Nếp where the person is standing.
 *
 * The slip goes through `donPhieu` here rather than at the call site, so a
 * screen cannot hand the assistant a field the whitelist has not seen. The
 * serialized form is the effect key: screens build the object inline every
 * render, and comparing by identity would write to the provider on every frame.
 */
export function useNepNguCanh(tho: unknown): void {
  const { datPhieu } = useNep();
  const sach = donPhieu(tho);
  const khoa = sach ? JSON.stringify(sach) : "";
  useEffect(() => {
    if (!khoa) return;
    datPhieu(JSON.parse(khoa) as PhieuNguCanh);
    return () => datPhieu(null);
  }, [khoa, datPhieu]);
}
