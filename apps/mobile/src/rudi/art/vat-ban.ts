/**
 * The five paper objects on the «Tạo mới» desk (ADR-0037 D1, plan S3): each
 * way to start something is the thing it makes, sketched small. A calendar
 * leaf for an outing, a thermal receipt for a bill, a photo print for a
 * memory, a polaroid for a story, a letter folded in three for the pair's
 * sheet. Drawn in a 64 x 64 frame, absolute M/L/C/Z only, numbers built at
 * run time (the art rules of `net.ts`).
 */
import { type Diem, type LopVe, bau, daGiac, duong, khungBo, netGay, tron } from "./net";

export const KHUNG_VAT = 64;

export const VAT_BAN = ["lich", "hoa-don", "anh-in", "polaroid", "thu-gap", "sticker", "phieu-bau"] as const;
export type VatBan = (typeof VAT_BAN)[number];

/** A short sentence for each drawing, for the one place it is read aloud. */
export const MO_TA_VAT: Readonly<Record<VatBan, string>> = Object.freeze({
  lich: "Ký hoạ một tờ lịch",
  "hoa-don": "Ký hoạ một tờ hoá đơn",
  "anh-in": "Ký hoạ một tấm ảnh in",
  polaroid: "Ký hoạ một tấm polaroid",
  "thu-gap": "Ký hoạ một lá thư gấp ba",
  sticker: "Ký hoạ một miếng sticker đang bóc",
  "phieu-bau": "Ký hoạ hai tờ giấy nhớ bình chọn",
});

const NET = 2;

/** Rotate points about a centre, for the objects that lie askew on the desk. */
function xoay(diem: readonly Diem[], goc: number, cx: number, cy: number): Diem[] {
  const r = (goc * Math.PI) / 180;
  const c = Math.cos(r);
  const s = Math.sin(r);
  return diem.map(([x, y]) => [cx + (x - cx) * c - (y - cy) * s, cy + (x - cx) * s + (y - cy) * c] as const);
}

function khung(x: number, y: number, w: number, h: number, goc = 0): Diem[] {
  return xoay(
    [
      [x, y],
      [x + w, y],
      [x + w, y + h],
      [x, y + h],
    ],
    goc,
    x + w / 2,
    y + h / 2,
  );
}

function vienKin(diem: readonly Diem[]): string {
  return `${netGay([...diem, diem[0]])}`;
}

function lich(): LopVe[] {
  // A leaf torn from a wall calendar: coral header, two rings, the day.
  const x = 12;
  const y = 12;
  const w = 40;
  const h = 44;
  return [
    { d: khungBo(x, y, w, h, 3), mau: "giay" },
    { d: daGiac([[x, y + 3], [x + 3, y], [x + w - 3, y], [x + w, y + 3], [x + w, y + 12], [x, y + 12]]), mau: "gap" },
    { d: khungBo(x, y, w, h, 3), mau: "muc", net: NET },
    { d: tron(x + 11, y - 1, 2.6), mau: "muc" },
    { d: tron(x + w - 11, y - 1, 2.6), mau: "muc" },
    // The day, a big numeral drawn as strokes: «2», then «6».
    { d: duong("M", 22, 26, "C", 22, 22, 30, 22, 30, 27, "C", 30, 31, 22, 36, 21, 42, "L", 31, 42), mau: "muc", net: NET },
    { d: duong("M", 42, 23, "C", 37, 23, 34, 30, 34, 36, "C", 34, 44, 43, 44, 43, 37, "C", 43, 31, 35, 31, 34, 36), mau: "muc", net: NET },
    { d: netGay([[19, 50], [45, 50]]), mau: "bong", net: 1.4 },
  ];
}

function hoaDon(): LopVe[] {
  // A thermal receipt: straight top, torn teeth at the bottom, lines, a teal total.
  const x0 = 16;
  const x1 = 48;
  const rang: Diem[] = [];
  for (let i = 0; i <= 8; i += 1) rang.push([x1 - i * 4, i % 2 === 0 ? 56 : 52]);
  const vien: Diem[] = [[x0, 8], [x1, 8], ...rang, [x0, 56]];
  return [
    { d: daGiac(vien), mau: "giay" },
    { d: vienKin(vien), mau: "muc", net: NET },
    { d: netGay([[21, 16], [43, 16]]), mau: "muc", net: 1.6 },
    { d: netGay([[21, 23], [35, 23]]), mau: "bong", net: 1.6 },
    { d: netGay([[21, 29], [38, 29]]), mau: "bong", net: 1.6 },
    { d: netGay([[21, 35], [32, 35]]), mau: "bong", net: 1.6 },
    { d: netGay([[21, 43], [43, 43]]), mau: "split", net: 2.4 },
  ];
}

function anhIn(): LopVe[] {
  // A photo print lying a little askew: white border, hills, a sun.
  const goc = -6;
  const vo = khung(10, 12, 44, 38, goc);
  const anh = khung(14, 16, 36, 26, goc);
  const [a0, a1, a2, a3] = anh;
  // Hills along the bottom of the picture, in the picture's own tilt.
  const doi = xoay(
    [
      [14, 42],
      [14, 36],
      [22, 29],
      [29, 35],
      [36, 27],
      [50, 38],
      [50, 42],
    ],
    goc,
    32,
    31,
  );
  const [mx, my] = xoay([[40, 22]], goc, 32, 31)[0];
  return [
    { d: daGiac(vo), mau: "giay" },
    { d: daGiac([a0, a1, a2, a3]), mau: "mo" },
    { d: daGiac(doi), mau: "bong" },
    { d: tron(mx, my, 3.4), mau: "gap" },
    { d: vienKin(vo), mau: "muc", net: NET },
    { d: vienKin(anh), mau: "muc", net: 1.2 },
  ];
}

function polaroid(): LopVe[] {
  // A polaroid: square picture, the thick white margin below it.
  const goc = 5;
  const vo = khung(14, 8, 36, 46, goc);
  const anh = khung(18, 12, 28, 28, goc);
  const [cx, cy] = xoay([[32, 26]], goc, 32, 31)[0];
  const [lx0, ly0] = xoay([[22, 47]], goc, 32, 31)[0];
  const [lx1, ly1] = xoay([[38, 47]], goc, 32, 31)[0];
  return [
    { d: daGiac(vo), mau: "giay" },
    { d: daGiac(anh), mau: "mo" },
    { d: bau(cx, cy, 6, 6), mau: "gap" },
    { d: vienKin(vo), mau: "muc", net: NET },
    { d: vienKin(anh), mau: "muc", net: 1.2 },
    // A caption line on the margin, as if written by hand.
    { d: duong("M", lx0, ly0, "C", lx0 + 5, ly0 - 2, lx1 - 5, ly1 + 2, lx1, ly1), mau: "muc", net: 1.4 },
  ];
}

function thuGap(): LopVe[] {
  // A letter folded in three, half open: two creases and the coral corner.
  const x0 = 10;
  const x1 = 54;
  const y0 = 14;
  const y1 = 50;
  const g = 9;
  const vien: Diem[] = [[x0, y0], [x1 - g, y0], [x1, y0 + g], [x1, y1], [x0, y1]];
  return [
    { d: daGiac(vien), mau: "giay" },
    { d: daGiac([[x0, y0 + 12], [x1, y0 + 12], [x1, y0 + 24], [x0, y0 + 24]]), mau: "bong" },
    { d: daGiac([[x1 - g, y0], [x1, y0 + g], [x1 - g, y0 + g]]), mau: "gap" },
    { d: vienKin(vien), mau: "muc", net: NET },
    { d: netGay([[x0, y0 + 12], [x1, y0 + 12]]), mau: "muc", net: 1.2 },
    { d: netGay([[x0, y0 + 24], [x1, y0 + 24]]), mau: "muc", net: 1.2 },
    { d: netGay([[x0 + 6, y0 + 6], [x1 - g - 6, y0 + 6]]), mau: "muc", net: 1.4 },
    { d: netGay([[x0 + 6, y0 + 30], [x0 + 26, y0 + 30]]), mau: "muc", net: 1.4 },
  ];
}

function sticker(): LopVe[] {
  // A round sticker, its edge peeling up in coral, a smile on it.
  const cx = 30;
  const cy = 32;
  const r = 20;
  return [
    { d: tron(cx, cy, r), mau: "giay" },
    { d: tron(cx, cy, r), mau: "muc", net: NET },
    { d: daGiac([[44, 18], [52, 14], [48, 26]]), mau: "gap" },
    { d: netGay([[44, 18], [52, 14], [48, 26]]), mau: "muc", net: 1.4 },
    { d: tron(23, 28, 2.2), mau: "muc" },
    { d: tron(37, 28, 2.2), mau: "muc" },
    { d: duong("M", 22, 37, "C", 26, 43, 34, 43, 38, 37), mau: "muc", net: NET },
  ];
}

function phieuBau(): LopVe[] {
  // Two sticky notes, one behind the other, the front one ticked.
  const sau = khung(8, 10, 32, 32, -8);
  const truoc = khung(24, 20, 32, 32, 5);
  const [tx, ty] = xoay([[34, 36]], 5, 40, 36)[0];
  return [
    { d: daGiac(sau), mau: "giay" },
    { d: vienKin(sau), mau: "muc", net: 1.4 },
    { d: daGiac(truoc), mau: "mo" },
    { d: vienKin(truoc), mau: "muc", net: NET },
    { d: netGay([[tx - 6, ty], [tx - 1, ty + 5], [tx + 9, ty - 6]]), mau: "muc", net: 2.6 },
  ];
}

/** The drawing of one desk object, as layers in the 64 x 64 frame. */
export function hinhVat(vat: VatBan): LopVe[] {
  switch (vat) {
    case "lich":
      return lich();
    case "hoa-don":
      return hoaDon();
    case "anh-in":
      return anhIn();
    case "polaroid":
      return polaroid();
    case "thu-gap":
      return thuGap();
    case "sticker":
      return sticker();
    case "phieu-bau":
      return phieuBau();
  }
}
