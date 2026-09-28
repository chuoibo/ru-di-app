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
  // QA only: the state work reaches, made reachable for a screenshot. Sent
  // once, on the first screen that declares itself to Nếp, so it lands on a
  // real page rather than behind the sign-in screen.
  const daGiao = useRef(false);
  const coPhieu = phieu !== null;
  useEffect(() => {
    if (!VIEC_NEP_QA || !coPhieu || daGiao.current) return;
    // After the arrival settles, so the capture sees the steady state. `hoi`
    // stands for a person who had already pulled Nếp out when the work
    // arrived: the second slip shows behind the pulled-out slip, and nothing
    // widens over the page.
    const t = setTimeout(() => {
      daGiao.current = true;
      if (VIEC_NEP_QA === "hoi") gui({ kieu: "keo-vao" });
      gui({ kieu: "bao-viec" });
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
