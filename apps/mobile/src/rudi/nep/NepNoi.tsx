import { useEffect, useRef } from "react";

import { NepBang } from "./NepBang";
import { NepDock } from "./NepDock";
import { useNep } from "./NepProvider";
import { TAT_NEP_QA, VIEC_NEP_QA } from "./qa-nep";

/**
 * The two halves of Nếp mounted together, once, above the navigator.
 *
 * Kept separate from `NepProvider` so the provider can sit high in the tree
 * (inside the session, outside `<Stack>`) while the drawing sits last and
 * paints over whatever route is open. `chuyen` already knows that one tap from
 * the edge only brings Nếp out and the next one opens the panel, so neither
 * child decides that for itself.
 */
export function NepNoi() {
  const { dock, gui, phieu } = useNep();
  // QA only: the states work reaches, made reachable for a screenshot. Sent
  // once, on the first screen that declares itself to Nếp, so the line is
  // still up when a person (or a capture) is looking rather than having timed
  // out behind the sign-in screen.
  const daGiao = useRef(false);
  const coPhieu = phieu !== null;
  useEffect(() => {
    if (!VIEC_NEP_QA || !coPhieu || daGiao.current) return;
    // After the arrival settles: the route change that brought the screen in
    // returns a peeked line to rest, which is right for a person and useless
    // for a capture. Nếp speaks only once it is out, so `hoi` stands for a
    // person who had already pulled Nếp out when the work arrived.
    const t = setTimeout(() => {
      daGiao.current = true;
      if (VIEC_NEP_QA === "hoi") gui({ kieu: "keo-vao" });
      gui({ kieu: "bao-viec", canTraLoi: VIEC_NEP_QA === "hoi" });
    }, 1200);
    return () => clearTimeout(t);
  }, [coPhieu, gui]);
  // The QA table needs every other screen to stay deterministic; a draggable
  // overlay that can cover any button is not that. See `qa-nep.ts`.
  if (TAT_NEP_QA) return null;
  return (
    <>
      <NepDock />
      <NepBang
        onClose={() => gui({ kieu: "dong" })}
        open={dock.trangThai === "mo"}
      />
    </>
  );
}
