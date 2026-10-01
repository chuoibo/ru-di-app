/**
 * The screen's overlay slot, reachable from deep inside the screen.
 *
 * A `Sheet` covers the box it is drawn in. Drawn inside a screen's body -- the
 * outing's map draws its day editor beside the map -- its scrim stopped at the
 * body's top edge, and the screen's header stayed lit and live above it (QA
 * UI-041). `RudiScreen` hosts this slot next to its `overlay`; `LenLop` hands
 * its children up to it, so they cover the whole screen and keep the state and
 * callbacks of the component that drew them. Without a host (a lab page, a
 * test) the children stay where they are.
 *
 * The children travel on every render of `LenLop`, in a layout effect: the
 * update it schedules is flushed in the same commit, before paint and before a
 * controlled input restores its value, so an input inside the slot never shows
 * the keystroke before last.
 */
import { createContext, Fragment, useContext, useId, useLayoutEffect, useMemo, useState, type ReactNode } from "react";

type KheLop = { dat(id: string, node: ReactNode): void; bo(id: string): void };

const KheLopContext = createContext<KheLop | null>(null);

/** For the screen: the slot's handle, to provide, and what it holds, to draw over the screen. */
export function useKheLop(): { khe: KheLop; lop: ReactNode } {
  const [noi, setNoi] = useState<ReadonlyMap<string, ReactNode>>(() => new Map());
  // Stable, so the components that use it never re-render because the slot changed.
  const khe = useMemo<KheLop>(
    () => ({
      dat: (id, node) => setNoi((cu) => new Map(cu).set(id, node)),
      bo: (id) =>
        setNoi((cu) => {
          if (!cu.has(id)) return cu;
          const moi = new Map(cu);
          moi.delete(id);
          return moi;
        }),
    }),
    [],
  );
  const lop = [...noi].map(([id, node]) => <Fragment key={id}>{node}</Fragment>);
  return { khe, lop };
}

export function KheLopProvider({ khe, children }: { khe: KheLop; children: ReactNode }) {
  return <KheLopContext.Provider value={khe}>{children}</KheLopContext.Provider>;
}

/** Draw `children` in the screen's overlay slot rather than in place. */
export function LenLop({ children }: { children: ReactNode }) {
  const khe = useContext(KheLopContext);
  const id = useId();
  useLayoutEffect(() => {
    khe?.dat(id, children);
  });
  useLayoutEffect(() => (khe ? () => khe.bo(id) : undefined), [khe, id]);
  return khe ? null : <>{children}</>;
}
