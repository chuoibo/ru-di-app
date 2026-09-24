/**
 * Where Nếp is on screen, and the only place that decides it.
 *
 * Nếp is the character who carries a sheet of paper, so hiding Nếp is tucking
 * that sheet into the edge of the notebook: `an` leaves a paper edge showing,
 * `nghi` is Nếp pulled out on the rail, `mo` is the panel open. Three states,
 * and only one of them rests: `an`.
 *
 * Pulled out is a passage, not a place. It is the first of the two taps that
 * open the panel, so a pulled-out Nếp that stayed out -- as the resting state
 * the panel closed back to, or carried across screens -- was 56dp over the
 * page for anyone who had once opened the panel (finish review, 24/09; the
 * canary measured it over «200.000đ» and «22:30»). Closing the panel, leaving
 * the screen, a sheet closing, and a few seconds with no second tap all put
 * Nếp back in the edge (ADR-0035).
 *
 * Nếp STARTS tucked. Measured on the running app (23/09), text reaches the
 * page margin: message times in a conversation end exactly 16dp from the
 * right edge, the width of the margin itself. Anything wider than the margin
 * that rests on the rail therefore covers words at some scroll offset, and the
 * first cut, which rested as a 57dp disc, did: it cut «20|0» and «22:|» on
 * Explore and sat on the «Đồng ý» of an invitation (flow 25). Tucked, only the
 * slip's edge shows, inside the margin. Nếp comes further out only when the
 * person pulls it.
 *
 * Nếp never widens by itself. An earlier cut let work that needed an answer
 * write one line on a pulled-out slip for four seconds (`he`); measured on
 * the web build (24/09) that line was 234dp wide and lay over a card's price
 * and opening hours, a live button over the words. «Không bao giờ che nội
 * dung» has no four-second exception, so work shows only as the second slip,
 * and what the work is waits in the panel.
 *
 * Two rules carry weight and both are negative, which is why they live in a
 * pure reducer instead of inside a component:
 *
 *   1. DESIGN.md's «Luật Nếp Đứng Xa Tiền» forbids Nếp beside money, errors and
 *      conflict. `luiLai` is that law: on those screens Nếp is pushed to `an`
 *      and the second slip may NOT show. The work is still recorded, it is
 *      simply not spoken. A component that merely skipped rendering would still
 *      have run the transition, and the next screen would inherit a Nếp that had
 *      popped open next to a settlement.
 *      A tap on the edge there does not bring Nếp out either: the edge is a
 *      door back to Nếp from elsewhere, not a face beside a figure.
 *   2. Nothing but the person's tap brings Nếp out, and nothing keeps it out
 *      past the screen it was brought out on.
 *
 * Two more, both about Nếp not standing on something else (couple QA 23/09):
 *
 *   3. While a sheet is open (`coSheet > 0`) Nếp is not drawn at all and goes
 *      back into the edge. A global floating object painted over an open sheet
 *      covered its inputs (the «Đừng» box, the «Đi tiếp» field) and read as a
 *      bug, not as depth. A count, not a flag: sheets can stack, and each
 *      closes its own. «Chừa một chỗ cho nhau» (ADR-0035 §2.5).
 *   4. On the sign-in and first-run screens (`vang`) Nếp is absent: Nếp is the
 *      person's own assistant (`/me/nep/*`) and there is no person yet, and the
 *      disc sat on the taste chips and the «Tạo nhóm» button.
 *
 * Pure: no React, no Reanimated, no routing. `NepDock.tsx` maps these states to
 * springs and `NepProvider.tsx` feeds the events in.
 */

/** Tucked (the one resting state), pulled out, and the panel. */
export type TrangThaiNep = "an" | "nghi" | "mo";

export interface DockNep {
  trangThai: TrangThaiNep;
  /** There is something waiting; the paper edge thickens. Not a state change. */
  coViec: boolean;
  /** This screen is one Nếp must stand away from (money, errors, conflict). */
  luiLai: boolean;
  /** How many sheets are open over the screen; Nếp is not drawn while any is. */
  coSheet: number;
  /** A screen Nếp is absent from (sign-in, first run). */
  vang: boolean;
}

export const DOCK_DAU: DockNep = Object.freeze({
  trangThai: "an",
  coViec: false,
  luiLai: false,
  coSheet: 0,
  vang: false,
});

export type SuKienNep =
  | { kieu: "cham" }
  | { kieu: "vuot-ra" }
  | { kieu: "keo-vao" }
  | { kieu: "dong" }
  | { kieu: "bao-viec" }
  | { kieu: "xong-viec" }
  /** Pulled out and left alone: back into the edge (ADR-0035 §2.2). */
  | { kieu: "tu-cat" }
  | { kieu: "mo-sheet" }
  | { kieu: "dong-sheet" }
  | { kieu: "doi-man"; nepLui: boolean; nepVang?: boolean };

/** Whether Nếp is drawn at all. The panel (`mo`) is itself a sheet. */
export function nepHien(dock: DockNep): boolean {
  return !dock.vang && dock.coSheet === 0 && dock.trangThai !== "mo";
}

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
  return dock.coViec && !dock.luiLai && !dock.vang && dock.coSheet === 0 && dock.trangThai !== "mo";
}

/** Back into the edge, keeping the badge and the law flag as they are. */
function veMep(dock: DockNep): DockNep {
  return { ...dock, trangThai: "an" };
}

export function chuyen(dock: DockNep, su: SuKienNep): DockNep {
  switch (su.kieu) {
    case "cham":
      // While a sheet is open the edge is only a sign that Nếp is still there.
      // Pulling Nếp out now would put it straight back over the words of the
      // sheet the person is reading, which is the bug this state exists for.
      // Beside money the edge is only a door, and a tap may not put a face next
      // to a figure (ADR-0033 §2.2).
      if (dock.coSheet > 0 || dock.luiLai || dock.vang) return dock;
      // From the edge, one tap only brings Nếp out. Opening the panel is a
      // second, deliberate tap: a sheet that sprang open from a stray swipe
      // back would cover the screen the person was actually reading.
      if (dock.trangThai === "an") return { ...dock, trangThai: "nghi" };
      if (dock.trangThai === "nghi") return { ...dock, trangThai: "mo" };
      return dock;

    case "vuot-ra":
      if (dock.trangThai === "nghi") return veMep(dock);
      return dock;

    case "tu-cat":
      // A stray tap on a 10dp edge beside the system's Back strip must not
      // leave 56dp over the text for as long as the person keeps reading.
      return dock.trangThai === "nghi" ? veMep(dock) : dock;

    case "keo-vao":
      if (dock.coSheet > 0 || dock.luiLai || dock.vang) return dock;
      if (dock.trangThai === "an") return { ...dock, trangThai: "nghi" };
      return dock;

    case "dong":
      return dock.trangThai === "mo" ? veMep(dock) : dock;

    case "bao-viec":
      // Recorded, never spoken out loud: `hienToSau` decides whether the
      // second slip may say it, and nothing here widens Nếp over the page.
      return { ...dock, coViec: true };

    case "xong-viec":
      return { ...dock, coViec: false };

    case "doi-man":
      // Every screen starts with Nếp in the edge; out was for the last one.
      return { ...dock, trangThai: "an", luiLai: su.nepLui && !su.nepVang, vang: su.nepVang === true };

    case "mo-sheet":
      // Nếp's own panel is a sheet too, and does not make room for itself.
      return { ...dock, coSheet: dock.coSheet + 1, trangThai: dock.trangThai === "mo" ? "mo" : "an" };

    case "dong-sheet":
      // The room is given back and Nếp stays in the edge it went to.
      return { ...dock, coSheet: Math.max(0, dock.coSheet - 1) };

    default:
      return dock;
  }
}
