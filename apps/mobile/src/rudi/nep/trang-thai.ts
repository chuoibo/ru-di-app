/**
 * Where Nếp is on screen, and the only place that decides it.
 *
 * Nếp is the character who carries a sheet of paper, so hiding Nếp is tucking
 * that sheet into the edge of the notebook: `an` leaves a paper edge showing,
 * `nghi` is Nếp resting on the rail, `he` is one line peeking out, `mo` is the
 * panel open. Four states, and `nen` remembers which of the two resting ones
 * to fall back to, so closing the panel never invents a position the person
 * did not choose.
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
}

export const DOCK_DAU: DockNep = Object.freeze({
  trangThai: "nghi",
  nen: "nghi",
  coViec: false,
  luiLai: false,
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
  | { kieu: "doi-man"; nepLui: boolean };

/** Rest at `nen`, keeping the badge and the law flag as they are. */
function veNen(dock: DockNep): DockNep {
  return { ...dock, trangThai: dock.nen };
}

export function chuyen(dock: DockNep, su: SuKienNep): DockNep {
  switch (su.kieu) {
    case "cham":
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
      if (dock.trangThai === "an") return { ...dock, trangThai: "nghi", nen: "nghi" };
      return dock;

    case "dong":
      return dock.trangThai === "mo" ? veNen(dock) : dock;

    case "bao-viec": {
      const coViec = { ...dock, coViec: true };
      // The law wins over the notification, and it wins silently.
      if (dock.luiLai) return coViec;
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
      return { ...dock, trangThai: dock.nen, luiLai: false };

    default:
      return dock;
  }
}
