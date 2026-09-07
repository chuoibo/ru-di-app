/**
 * The eight tastes, drawn with one pen on a 48 grid (report 07/09 §7.2).
 *
 * Content glyphs, not utility icons: an object you would recognise on a
 * table, in a single stroke weight, with at most one coral detail each so the
 * set is related to Nếp's fold without shouting. The ids are the server's
 * vocabulary (`src/screens/vao-cua/so-thich.ts`, held equal to `GET /interests`
 * by `tests/test_interest_vocabulary_matches_client.py`); only the picture
 * lives here. An id this build does not know draws a folded tag, never
 * nothing and never a throw.
 *
 * Main stroke ≈ 2.4 on the 48 grid: about 2dp at 48dp, still a line at 32dp.
 */
import { type LopVe, bau, cong, cungTron, daGiac, giot, khungBo, netGay, qCong, tron } from "./net";

export const KHUNG_GU = 48;

export const GU_IDS = ["an-uong", "cafe", "nightlife", "mon-local", "outdoor", "shopping", "karaoke", "game"] as const;
export type GuId = (typeof GU_IDS)[number];

const NET = 2.4;
const MANH = 1.7;

const HINH: Record<GuId, () => LopVe[]> = {
  // Two pairs of chopsticks into one bowl: eating is something a group does.
  "an-uong": () => [
    { d: cungTron(24, 23, 13, 0.1 * Math.PI, 0.9 * Math.PI), mau: "muc", net: NET },
    { d: netGay([[9, 22], [39, 22]]), mau: "muc", net: NET },
    { d: netGay([[9, 5], [20, 21]]), mau: "muc", net: NET },
    { d: netGay([[15, 4], [24, 21]]), mau: "muc", net: NET },
    { d: netGay([[39, 5], [28, 21]]), mau: "muc", net: NET },
    { d: netGay([[33, 4], [24, 21]]), mau: "muc", net: NET },
    { d: bau(24, 29, 4.5, 2.2), mau: "gap" },
  ],
  // A phin on a glass and one drop that has stopped.
  cafe: () => [
    { d: khungBo(12, 11, 24, 11, 2), mau: "muc", net: NET },
    { d: netGay([[36, 15], [41, 15]]), mau: "muc", net: NET },
    { d: netGay([[16, 26], [18, 43], [30, 43], [32, 26]]), mau: "muc", net: NET },
    { d: netGay([[13, 26], [35, 26]]), mau: "muc", net: NET },
    { d: giot(24, 31.5, 2.3), mau: "gap" },
  ],
  // A hanging lamp throwing one beam, and a ticket stub: a night out is not
  // by default a drink, and the lamp reads at 32 where a streak did not.
  nightlife: () => [
    { d: netGay([[24, 4], [24, 13]]), mau: "muc", net: NET },
    { d: tron(24, 19, 5.5), mau: "muc", net: NET },
    { d: tron(24, 19, 2), mau: "gap" },
    { d: netGay([[19, 24], [10, 42]]), mau: "muc", net: MANH },
    { d: netGay([[29, 24], [38, 42]]), mau: "muc", net: MANH },
    { d: khungBo(27, 33, 15, 9, 2), mau: "muc", net: NET },
    { d: netGay([[34, 35], [34, 40]]), mau: "muc", net: MANH },
  ],
  // A small pot with its lid and a menu tag: local food, not a burger.
  "mon-local": () => [
    { d: khungBo(11, 21, 26, 16, 5), mau: "muc", net: NET },
    { d: cungTron(24, 21, 11.5, Math.PI, 2 * Math.PI), mau: "muc", net: NET },
    { d: tron(24, 9.5, 2.2), mau: "muc" },
    { d: netGay([[5, 26], [11, 26]]), mau: "muc", net: NET },
    { d: netGay([[37, 26], [43, 26]]), mau: "muc", net: NET },
    { d: daGiac([[33, 36], [44, 33], [44, 42], [33, 45]]), mau: "gap" },
    { d: tron(36, 39, 1.3), mau: "giay" },
  ],
  // A trail rising to a pine on the ridge; the coral dot is where you stand.
  outdoor: () => [
    { d: cong([8, 43], [16, 22], [26, 40], [40, 14]), mau: "muc", net: NET },
    { d: tron(8, 43, 2.6), mau: "gap" },
    { d: daGiac([[33, 12], [42, 30], [24, 30]]), mau: "muc" },
    { d: netGay([[33, 30], [33, 38]]), mau: "muc", net: NET },
  ],
  // A cloth bag with the same folded corner as the sheet.
  shopping: () => [
    { d: daGiac([[13, 18], [35, 18], [38, 42], [10, 42]]), mau: "muc", net: NET },
    { d: cungTron(24, 18, 8, Math.PI, 2 * Math.PI), mau: "muc", net: NET },
    { d: daGiac([[35, 18], [38, 26.5], [28, 18]]), mau: "gap" },
    { d: netGay([[16, 30], [17, 42]]), mau: "muc", net: MANH },
  ],
  // A microphone and one single note, no sparkle.
  karaoke: () => [
    { d: tron(20, 14, 6.5), mau: "muc", net: NET },
    { d: netGay([[16, 12], [24, 12]]), mau: "muc", net: MANH },
    { d: netGay([[16, 16], [24, 16]]), mau: "muc", net: MANH },
    { d: netGay([[20, 21], [20, 43]]), mau: "muc", net: NET },
    { d: netGay([[15, 22], [25, 22]]), mau: "muc", net: NET },
    { d: netGay([[36, 11], [36, 30]]), mau: "muc", net: NET },
    { d: qCong([36, 11], [44, 15], [39, 24]), mau: "muc", net: NET },
    { d: bau(33.5, 31, 3.6, 2.6), mau: "gap" },
  ],
  // A controller with two buttons, and no more detail than 32dp can carry.
  game: () => [
    { d: khungBo(7, 17, 34, 17, 8), mau: "muc", net: NET },
    { d: netGay([[13, 25.5], [21, 25.5]]), mau: "muc", net: NET },
    { d: netGay([[17, 21.5], [17, 29.5]]), mau: "muc", net: NET },
    { d: tron(30.5, 22.5, 2.3), mau: "gap" },
    { d: tron(35.5, 28, 2.3), mau: "muc" },
  ],
};

/** A folded tag, for an id this build has no picture for. */
function theGap(): LopVe[] {
  const than = daGiac([[10, 10], [31, 10], [38, 17], [38, 38], [10, 38]]);
  return [
    { d: than, mau: "muc", net: NET },
    { d: daGiac([[31, 10], [31, 17], [38, 17]]), mau: "gap" },
    { d: tron(16, 16, 1.6), mau: "muc" },
  ];
}

export function laGuId(id: string): id is GuId {
  return (GU_IDS as readonly string[]).includes(id);
}

/** The layers of one taste, drawn fresh each call (they are cheap). */
export function hinhGu(id: string): LopVe[] {
  return laGuId(id) ? HINH[id]() : theGap();
}
