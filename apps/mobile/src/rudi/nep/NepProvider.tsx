import { useFocusEffect, usePathname } from "expo-router";
import { createContext, useCallback, useContext, useEffect, useMemo, useReducer, useRef, useState } from "react";
import type { ReactNode } from "react";

import { docGiaoDienAsync, ghiGiaoDienAsync } from "../kho";
import { KHOA_DOCK, giaiMaDock, maHoaDock } from "./luu-dock";
import { donPhieu, nepPhaiLui, nepPhaiVang, type PhieuNguCanh } from "./phieu";
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
  /** Declare the open screen's slip; returns the id `goPhieu` takes back. */
  khaiPhieu(p: PhieuNguCanh, duong: string): number;
  /** Take back a slip, only if it is still the one showing. */
  goPhieu(id: number): void;
}

/** A slip and the route it was declared on. */
interface PhieuDaKhai {
  id: number;
  duong: string;
  phieu: PhieuNguCanh;
}

const NepContext = createContext<NepDieuKhien | null>(null);
/**
 * The dispatcher alone, which never changes identity. A sheet only needs to
 * say it opened and closed; reading it from `NepContext` would re-render every
 * mounted sheet each time Nếp moves.
 */
const NepGuiContext = createContext<((su: SuKienNep) => void) | null>(null);

export function NepProvider({ children }: { children: ReactNode }) {
  const [dock, gui] = useReducer(chuyen, DOCK_DAU);
  const [daKhai, datDaKhai] = useState<PhieuDaKhai | null>(null);
  const soPhieu = useRef(0);
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
      if (daLuu) datTyLeRaw(daLuu.tyLe);
      datDaDocDia(true);
    });
    return () => {
      song = false;
    };
  }, []);

  // The law, applied on every route change: money, errors and conflict are
  // screens Nếp stands away from (DESIGN.md «Luật Nếp Đứng Xa Tiền», ADR-0033).
  useEffect(() => {
    gui({ kieu: "doi-man", nepLui: nepPhaiLui(duong ?? ""), nepVang: nepPhaiVang(duong ?? "") });
  }, [duong]);

  // A screen's slip belongs to that screen, and to the route it was declared
  // on: a slip is only shown while that route is the open one, so a screen that
  // forgets to declare cannot inherit the previous one's numbers and have Nếp
  // answer about a trip the person already left. (It used to be cleared on
  // every route change instead; a screen still mounted underneath -- a chat
  // behind the notebook it opened -- then lost its slip for good when the
  // person came back, QA 24/09.)
  const phieu = daKhai !== null && daKhai.duong === (duong ?? "") ? daKhai.phieu : null;
  const khaiPhieu = useCallback((p: PhieuNguCanh, noi: string) => {
    soPhieu.current += 1;
    const id = soPhieu.current;
    datDaKhai({ id, duong: noi, phieu: p });
    return id;
  }, []);
  // Taking a slip back never clears the next screen's: blur of the old screen
  // and focus of the new one arrive in either order.
  const goPhieu = useCallback((id: number) => {
    datDaKhai((truoc) => (truoc?.id === id ? null : truoc));
  }, []);

  const datTyLe = useCallback((t: number) => {
    const sach = Number.isFinite(t) ? Math.min(1, Math.max(0, t)) : 0;
    datTyLeRaw(sach);
  }, []);

  // Persist the one fact worth persisting, after the first read so the default
  // never overwrites what the disk holds.
  useEffect(() => {
    if (!daDocDia) return;
    void ghiGiaoDienAsync(KHOA_DOCK, maHoaDock({ tyLe }));
  }, [daDocDia, tyLe]);

  const gia = useMemo<NepDieuKhien>(
    () => ({ dock, phieu, tyLe, daDocDia, gui, datTyLe, khaiPhieu, goPhieu }),
    [dock, phieu, tyLe, daDocDia, datTyLe, khaiPhieu, goPhieu],
  );

  return (
    <NepGuiContext.Provider value={gui}>
      <NepContext.Provider value={gia}>{children}</NepContext.Provider>
    </NepGuiContext.Provider>
  );
}

/**
 * Nếp's dispatcher, or null outside a `NepProvider` (a sheet rendered by a
 * test or a preview has no Nếp to tell, and must not throw).
 */
export function useNepGui(): ((su: SuKienNep) => void) | null {
  return useContext(NepGuiContext);
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
  const { khaiPhieu, goPhieu } = useNep();
  const duong = usePathname() ?? "";
  const sach = donPhieu(tho);
  const khoa = sach ? JSON.stringify(sach) : "";
  // On focus, not on mount: a stack keeps the screen underneath mounted, and
  // it must declare again when the person comes back to it.
  useFocusEffect(
    useCallback(() => {
      if (!khoa) return;
      const id = khaiPhieu(JSON.parse(khoa) as PhieuNguCanh, duong);
      return () => goPhieu(id);
    }, [khoa, duong, khaiPhieu, goPhieu]),
  );
}

/**
 * What a surface that draws its own tray, or takes the whole screen, calls
 * while it is up: the same count `ui/Sheet.tsx` keeps (`mo-sheet` /
 * `dong-sheet`), so Nếp tucks into the edge and is not drawn until the last
 * one closes. «Chừa một chỗ cho nhau» (ADR-0035 §2.5): the tray «Tờ hẹn», the
 * story viewer and the journey map.
 *
 * Tolerates a missing provider, because surfaces render in fixtures and tests
 * that have no Nếp at all.
 */
export function useNhuongChoNep(dangMo: boolean): void {
  const gui = useNepGui();
  useEffect(() => {
    if (!dangMo || !gui) return;
    gui({ kieu: "mo-sheet" });
    return () => gui({ kieu: "dong-sheet" });
  }, [dangMo, gui]);
}
