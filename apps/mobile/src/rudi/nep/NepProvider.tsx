import { usePathname } from "expo-router";
import { createContext, useCallback, useContext, useEffect, useMemo, useReducer, useRef, useState } from "react";
import type { ReactNode } from "react";

import { docGiaoDienAsync, ghiGiaoDienAsync } from "../kho";
import { KHOA_DOCK, giaiMaDock, maHoaDock } from "./luu-dock";
import { donPhieu, ghiPhieu, nepPhaiLui, phieuDangMo, type PhieuCuaMan, type PhieuNguCanh, type SuPhieu } from "./phieu";
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
  /** A screen declares its slip, or withdraws the one it declared. */
  ghiPhieu(su: Omit<Extract<SuPhieu, { kieu: "khai" }>, "duong"> | Extract<SuPhieu, { kieu: "bo" }>): void;
  /** A sheet opened over the page (`true`) or closed (`false`). Counted. */
  nhuongCho(mo: boolean): void;
}

const NepContext = createContext<NepDieuKhien | null>(null);

export function NepProvider({ children }: { children: ReactNode }) {
  const [dock, gui] = useReducer(chuyen, DOCK_DAU);
  const [ghi, datGhi] = useState<PhieuCuaMan | null>(null);
  const [tyLe, datTyLeRaw] = useState(0.62);
  const [daDocDia, datDaDocDia] = useState(false);
  const duong = usePathname();
  // Read during render so a child's effect, which runs before this
  // component's effects in the same commit, tags its slip with the route that
  // commit is showing.
  const duongHienTai = useRef(duong);
  duongHienTai.current = duong;

  // Read once. A failed read is indistinguishable from a first launch and both
  // answers are the same: the default position.
  useEffect(() => {
    let song = true;
    void docGiaoDienAsync(KHOA_DOCK).then((raw) => {
      if (!song) return;
      const daLuu = giaiMaDock(raw);
      if (daLuu) {
        datTyLeRaw(daLuu.tyLe);
        if (daLuu.ra) gui({ kieu: "keo-vao" });
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

  const phieu = phieuDangMo(ghi, duong ?? "");
  const guiPhieu = useCallback<NepDieuKhien["ghiPhieu"]>((su) => {
    datGhi((cu) => ghiPhieu(cu, su.kieu === "khai" ? { ...su, duong: duongHienTai.current ?? "" } : su));
  }, []);

  // Sheets nest: a confirm can open over a tray. Nếp comes back only when the
  // LAST one closes, so the count is what matters, not the latest event.
  const soTo = useRef(0);
  const nhuongCho = useCallback((mo: boolean) => {
    const truoc = soTo.current;
    soTo.current = Math.max(0, truoc + (mo ? 1 : -1));
    if (truoc === 0 && soTo.current > 0) gui({ kieu: "nhuong-cho", bat: true });
    if (truoc > 0 && soTo.current === 0) gui({ kieu: "nhuong-cho", bat: false });
  }, []);

  const datTyLe = useCallback((t: number) => {
    const sach = Number.isFinite(t) ? Math.min(1, Math.max(0, t)) : 0;
    datTyLeRaw(sach);
  }, []);

  // Persist the two facts worth persisting, after the first read so the default
  // never overwrites what the disk holds.
  useEffect(() => {
    if (!daDocDia) return;
    void ghiGiaoDienAsync(KHOA_DOCK, maHoaDock({ tyLe, ra: dock.nen === "nghi" }));
  }, [daDocDia, tyLe, dock.nen]);

  const gia = useMemo<NepDieuKhien>(
    () => ({ dock, phieu, tyLe, daDocDia, gui, datTyLe, ghiPhieu: guiPhieu, nhuongCho }),
    [dock, phieu, tyLe, daDocDia, datTyLe, guiPhieu, nhuongCho],
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
  const { ghiPhieu: gui } = useNep();
  const sach = donPhieu(tho);
  const khoa = sach ? JSON.stringify(sach) : "";
  useEffect(() => {
    if (!khoa) return;
    const phieu = JSON.parse(khoa) as PhieuNguCanh;
    gui({ kieu: "khai", phieu });
    return () => gui({ kieu: "bo", phieu });
  }, [khoa, gui]);
}

/**
 * What a surface calls while it lays another sheet over the page.
 *
 * «Chừa một chỗ cho nhau»: while the sheet is open Nếp tucks into the notebook
 * edge and leaves the room to it, and comes back to exactly where the person
 * left it when the sheet closes. `ui/Sheet.tsx` calls this for every bottom
 * sheet in the app; a surface that draws its own tray calls it too.
 *
 * Tolerates a missing provider, because sheets render in fixtures and tests
 * that have no Nếp at all -- and a sheet with no Nếp to make room for has
 * nothing to do.
 */
export function useNhuongChoNep(dangMo: boolean): void {
  const nhuong = useContext(NepContext)?.nhuongCho;
  useEffect(() => {
    if (!dangMo || !nhuong) return;
    nhuong(true);
    return () => nhuong(false);
  }, [dangMo, nhuong]);
}

