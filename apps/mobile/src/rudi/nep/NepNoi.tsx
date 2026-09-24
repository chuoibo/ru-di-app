import { NepBang } from "./NepBang";
import { NepDock } from "./NepDock";
import { useNep } from "./NepProvider";
import { TAT_NEP_QA } from "./qa-nep";

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
  const { dock, gui } = useNep();
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
