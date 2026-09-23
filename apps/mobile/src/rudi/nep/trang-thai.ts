/**
 * Where Nếp is on screen, and the only place that decides it.
 *
 * Nếp is the character who carries a sheet of paper, so hiding Nếp is tucking
 * that sheet into the edge of the notebook: `an` leaves a paper edge showing,
 * `nghi` is Nếp pulled out on the rail, `he` is one line peeking out, `mo` is
 * the panel open. Four states, and `nen` remembers which of the two resting
 * ones to fall back to, so closing the panel never invents a position the
 * person did not choose.
 *
 * Nếp STARTS tucked. Measured on the running app (23/09), text reaches the
 * page margin: message times in a conversation end exactly 16dp from the
 * right edge, the width of the margin itself. Anything wider than the margin
 * that rests on the rail therefore covers words at some scroll offset, and the
 * first cut, which rested as a 57dp disc, did: it cut «20|0» and «22:|» on
 * Explore and sat on the «Đồng ý» of an invitation (flow 25). Tucked, only the
 * slip's edge shows, inside the margin. Nếp comes further out only when the
 * person pulls it, or for the one line that needs an answer.
 *
 * Two rules carry weight and both are negative, which is why they live in a
 * pure reducer instead of inside a component:
 *
 *   1. DESIGN.md's «Luật Nếp Đứng Xa Tiền» forbids Nếp beside money, errors and
 *      conflict. `luiLai` is that law: on those screens Nếp is pushed to `an`
 *      and `bao-viec` may NOT open a bubble. The work is still recorded, it is
 *      simply not spoken. A component that merely skipped rendering would still
 *      have run the transition, and the next screen would inherit a Nếp that had
 *      popped open next to a settlement.
 *   2. Leaving a money screen restores what the PERSON last chose, never a
 *      default. Someone who swiped Nếp away keeps it away.
 *
 * Pure: no React, no Reanimated, no routing. `NepDock.tsx` maps these states to
 * springs and `NepProvider.tsx` feeds the events in.
 */

/** Two resting states, plus the two that are always temporary. */
export type NenNep = "an" | "nghi";
export type TrangThaiNep = NenNep | "he" | "mo";

export interface DockNep {
  trangThai: TrangThaiNep;
  /** The resting state `he` and `mo` fall back to. */
  nen: NenNep;
  /** There is something waiting; the paper edge thickens. Not a state change. */
  coViec: boolean;
  /** This screen is one Nếp must stand away from (money, errors, conflict). */
  luiLai: boolean;
  /**
   * The page has laid another sheet over itself -- a tray, a bottom sheet --
   * so Nếp has tucked into the edge to leave the room to it. Like `luiLai` it
   * is never the person's choice, so it never touches `nen`.
   */
  nhuongCho: boolean;
}

export const DOCK_DAU: DockNep = Object.freeze({
  trangThai: "an",
  nen: "an",
  coViec: false,
  luiLai: false,
  nhuongCho: false,
});

export type SuKienNep =
  | { kieu: "cham" }
  | { kieu: "vuot-ra" }
  | { kieu: "keo-vao" }
  | { kieu: "dong" }
  /** `canTraLoi` is the difference between a badge and a sentence out loud. */
  | { kieu: "bao-viec"; canTraLoi: boolean }
  | { kieu: "xong-viec" }
  | { kieu: "het-gio-he" }
  | { kieu: "doi-man"; nepLui: boolean }
  /** Another sheet opened over the page (`bat`), or the last one closed. */
  | { kieu: "nhuong-cho"; bat: boolean };

/**
 * Whether the second slip -- «there is something waiting» -- may show.
 *
 * `coViec` stays true on a money screen and while another sheet is up, because
 * the work is real and must still be there afterwards. But a slip that shows
 * it is Nếp saying something, and ADR-0033 §2 forbids exactly that beside
 * money («việc vẫn được ghi nhận, chỉ là không nói ra»); beside an open tray
 * it would be Nếp talking over the sheet the person is reading. Nếp's own
 * panel covers the edge anyway. So the record and the signal are two facts,
 * and this is the only place that joins them.
 */
export function hienToSau(dock: DockNep): boolean {
  return dock.coViec && !dock.luiLai && !dock.nhuongCho && dock.trangThai !== "mo";
}

/** Rest at `nen`, keeping the badge and the law flag as they are. */
function veNen(dock: DockNep): DockNep {
  return { ...dock, trangThai: dock.nen };
}

export function chuyen(dock: DockNep, su: SuKienNep): DockNep {
  switch (su.kieu) {
    case "cham":
      // While a sheet is open the edge is only a sign that Nếp is still there.
      // Pulling Nếp out now would put it straight back over the words of the
      // sheet the person is reading, which is the bug this state exists for.
      if (dock.nhuongCho) return dock;
      // From the edge, one tap only brings Nếp out. Opening the panel is a
      // second, deliberate tap: a sheet that sprang open from a stray swipe
      // back would cover the screen the person was actually reading.
      if (dock.trangThai === "an") return { ...dock, trangThai: "nghi", nen: "nghi" };
      if (dock.trangThai === "nghi" || dock.trangThai === "he") return { ...dock, trangThai: "mo" };
      return dock;

    case "vuot-ra":
      // Swiping the bubble away retracts it; it does not also hide Nếp, which
      // would punish the person for dismissing one sentence.
      if (dock.trangThai === "he") return veNen(dock);
      if (dock.trangThai === "nghi") return { ...dock, trangThai: "an", nen: "an" };
      return dock;

    case "keo-vao":
      if (dock.nhuongCho) return dock;
      if (dock.trangThai === "an") return { ...dock, trangThai: "nghi", nen: "nghi" };
      return dock;

    case "dong":
      return dock.trangThai === "mo" ? veNen(dock) : dock;

    case "bao-viec": {
      const coViec = { ...dock, coViec: true };
      // The law wins over the notification, and it wins silently. So does an
      // open sheet: the work is kept and simply waits for the room to clear.
      if (dock.luiLai || dock.nhuongCho) return coViec;
      if (!su.canTraLoi) return coViec;
      if (dock.trangThai === "an" || dock.trangThai === "nghi") return { ...coViec, trangThai: "he" };
      return coViec;
    }

    case "xong-viec": {
      const xong = { ...dock, coViec: false };
      return xong.trangThai === "he" ? veNen(xong) : xong;
    }

    case "het-gio-he":
      return dock.trangThai === "he" ? veNen(dock) : dock;

    case "doi-man":
      // `nen` is deliberately untouched on the way in: it is the person's last
      // choice, and it is what they get back on the way out.
      if (su.nepLui) return { ...dock, trangThai: "an", luiLai: true };
      // A sheet still open across the route change keeps Nếp tucked until it
      // closes; the release event brings Nếp back, not the navigation.
      return { ...dock, trangThai: dock.nhuongCho ? "an" : dock.nen, luiLai: false };

    case "nhuong-cho":
      if (su.bat) {
        // Nếp's own panel is a sheet too. It does not make room for itself.
        if (dock.trangThai === "mo") return dock;
        return { ...dock, trangThai: "an", nhuongCho: true };
      }
      if (!dock.nhuongCho) return dock;
      // Back to what the person chose -- unless the money law still holds.
      return { ...dock, nhuongCho: false, trangThai: dock.luiLai ? "an" : dock.trangThai === "mo" ? "mo" : dock.nen };

    default:
      return dock;
  }
}
