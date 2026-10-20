/**
 * Hand-drawn passport stamp glyphs for the Cá nhân traveler passport &
 * journey footprint ledger (ADR-0037 «Sân khấu giấy»).
 *
 * Each glyph is authored in a 40 × 40 frame using `net.ts` builders only
 * (explicit M/L/C/Z, paper fill before ink strokes, at most one coral `gap`
 * mark per glyph).
 */
import { type LopVe, cungTron, daGiac, duong, khungBo, netGay, tron } from "./net";

export const KHUNG_DAU_CHAN = 40;

export type DauChanId =
  | "friends"
  | "contexts"
  | "outings"
  | "places"
  | "memories"
  | "taste"
  | "passport-seal";

const NET = 1.75;

function vienKin(diem: readonly (readonly [number, number])[]): string {
  return netGay([...diem, diem[0]]);
}

function friends(): LopVe[] {
  // A pocket compass rose: warm paper dial, coral north needle, ink rim.
  return [
    { d: tron(20, 20, 14), mau: "giay" },
    { d: daGiac([[20, 9], [23.5, 20], [20, 18], [16.5, 20]]), mau: "gap" },
    { d: daGiac([[20, 31], [23.5, 20], [20, 22], [16.5, 20]]), mau: "bong" },
    { d: tron(20, 20, 14), mau: "muc", net: NET },
    { d: vienKin([[20, 9], [23.5, 20], [20, 31], [16.5, 20]]), mau: "muc", net: 1.3 },
    { d: tron(20, 20, 2), mau: "muc" },
  ];
}

function contexts(): LopVe[] {
  // A stitched group notebook with a coral spine ribbon.
  return [
    { d: khungBo(8, 6, 24, 28, 3), mau: "giay" },
    { d: daGiac([[8, 9], [11, 6], [13, 6], [13, 34], [11, 34], [8, 31]]), mau: "gap" },
    { d: khungBo(8, 6, 24, 28, 3), mau: "muc", net: NET },
    { d: netGay([[13, 6], [13, 34]]), mau: "muc", net: 1.3 },
    { d: netGay([[17, 14], [27, 14]]), mau: "muc", net: 1.4 },
    { d: netGay([[17, 20], [25, 20]]), mau: "bong", net: 1.4 },
    { d: netGay([[17, 26], [23, 26]]), mau: "bong", net: 1.4 },
  ];
}

function outings(): LopVe[] {
  // A perforated travel ticket stub with a coral corner tab.
  const vien = [
    [6, 11],
    [34, 11],
    [34, 29],
    [6, 29],
  ] as const;
  return [
    { d: daGiac(vien), mau: "giay" },
    { d: daGiac([[6, 11], [13, 11], [13, 29], [6, 29]]), mau: "gap" },
    { d: vienKin(vien), mau: "muc", net: NET },
    { d: netGay([[13, 11], [13, 29]]), mau: "muc", net: 1.2 },
    { d: netGay([[18, 17], [29, 17]]), mau: "muc", net: 1.5 },
    { d: netGay([[18, 23], [26, 23]]), mau: "bong", net: 1.4 },
  ];
}

function places(): LopVe[] {
  // A stamped map pin over a folded map square.
  const banDo = [
    [7, 23],
    [33, 23],
    [31, 33],
    [9, 33],
  ] as const;
  const ghim = duong(
    "M", 20, 30,
    "C", 16, 24, 12, 19, 12, 14,
    "C", 12, 9.5, 15.6, 6, 20, 6,
    "C", 24.4, 6, 28, 9.5, 28, 14,
    "C", 28, 19, 24, 24, 20, 30,
    "Z",
  );
  return [
    { d: daGiac(banDo), mau: "giay" },
    { d: ghim, mau: "gap" },
    { d: tron(20, 14, 3.2), mau: "giay" },
    { d: vienKin(banDo), mau: "muc", net: 1.4 },
    { d: ghim, mau: "muc", net: NET },
    { d: tron(20, 14, 3.2), mau: "muc", net: 1.3 },
  ];
}

function memories(): LopVe[] {
  // A tilted polaroid frame with a warm sun and hill silhouette.
  return [
    { d: khungBo(8, 6, 24, 28, 2.5), mau: "giay" },
    { d: daGiac([[11, 9], [29, 9], [29, 24], [11, 24]]), mau: "mo" },
    { d: daGiac([[11, 24], [16, 18], [21, 22], [25, 17], [29, 24]]), mau: "bong" },
    { d: tron(24, 13.5, 2.4), mau: "gap" },
    { d: khungBo(8, 6, 24, 28, 2.5), mau: "muc", net: NET },
    { d: vienKin([[11, 9], [29, 9], [29, 24], [11, 24]]), mau: "muc", net: 1.2 },
    { d: netGay([[14, 29], [26, 29]]), mau: "muc", net: 1.4 },
  ];
}

function taste(): LopVe[] {
  // A wax-sealed taste card with a steaming bowl sketch.
  return [
    { d: khungBo(6, 8, 28, 24, 3), mau: "giay" },
    { d: tron(29, 13, 3.6), mau: "gap" },
    { d: khungBo(6, 8, 28, 24, 3), mau: "muc", net: NET },
    { d: cungTron(18, 21, 6.5, 0, Math.PI), mau: "muc", net: 1.5 },
    { d: netGay([[11.5, 21], [24.5, 21]]), mau: "muc", net: 1.5 },
    { d: netGay([[15, 28], [21, 28]]), mau: "bong", net: 1.4 },
    { d: duong("M", 16, 17, "C", 15, 15, 17, 14, 16, 12), mau: "bong", net: 1.3 },
    { d: duong("M", 20, 17, "C", 19, 15, 21, 14, 20, 12), mau: "bong", net: 1.3 },
  ];
}

function passportSeal(): LopVe[] {
  // A circular postmark stamp with inner ring and cancellation waves.
  return [
    { d: tron(20, 20, 14), mau: "giay" },
    { d: tron(20, 20, 3.2), mau: "gap" },
    { d: tron(20, 20, 14), mau: "muc", net: 1.5 },
    { d: cungTron(20, 20, 10.5, 0.3, Math.PI * 1.7), mau: "bong", net: 1.2 },
    { d: duong("M", 6, 17, "C", 10, 15, 14, 19, 18, 17), mau: "muc", net: 1.2 },
    { d: duong("M", 22, 23, "C", 26, 21, 30, 25, 34, 23), mau: "muc", net: 1.2 },
  ];
}

export function hinhDauChan(id: DauChanId): LopVe[] {
  switch (id) {
    case "friends":
      return friends();
    case "contexts":
      return contexts();
    case "outings":
      return outings();
    case "places":
      return places();
    case "memories":
      return memories();
    case "taste":
      return taste();
    case "passport-seal":
      return passportSeal();
  }
}
