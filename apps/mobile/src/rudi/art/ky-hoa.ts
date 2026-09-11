/**
 * «Ký hoạ trong sổ» — the ink sketch a place gets when it has no photo
 * (review 11/09, A1: the Explore lead read as a styled catalogue; the bowl
 * glyph said the category but nothing of light, food or seating).
 *
 * A strip of 288×96 on a sheet of `paper`: a STAGE chosen by the place's
 * category (an eatery's awning and long table · a café's window · a hill ·
 * a night street), at most two PROPS chosen by tags the place really carries
 * («View đẹp» → a ridge behind, «Nhóm đông» → four bowls, «Săn mây» → clouds…),
 * and exactly ONE coral layer — the light source (a hung bulb, a pendant, the
 * sun over the ridge, the lit bulb on the string). It draws a KIND of place,
 * never a specific one: no name, no sign, no photo-like detail; the a11y
 * sentence says so. Three stroke weights carry depth (near 3.0 · mid 2.4 ·
 * far 1.7); shade is a `bong` wash, never a gradient.
 *
 * `gon` is the reading for the 4:1 crop (large text): fewer layers, the same
 * stage. Pure geometry from `net.ts`; roles resolve in `VeLop`.
 */
import { gapChu } from "../kham-pha/ly-do";
import { type LopVe, bau, cong, cungTron, daGiac, giot, khungBo, netCong, netGay, quat, tron } from "./net";

export const KHUNG_KY_HOA = { w: 288, h: 96 } as const;
/** Crop ratios of the two readings; the drawing is one, the frame differs. */
export const TI_LE_KY_HOA = { day: 3, gon: 4 } as const;

export const SAN_KHAU_IDS = ["hien-quan", "cua-kinh", "doi", "pho-dem", "to-giay"] as const;
export type SanKhau = (typeof SAN_KHAU_IDS)[number];

const SAN = 82;
const PI = Math.PI;
const GAN = 3.0;
const VUA = 2.4;
const XA = 1.7;
const m = (d: string, net = VUA): LopVe => ({ d, mau: "muc", net });
const fill = (d: string, mau: LopVe["mau"] = "bong"): LopVe => ({ d, mau });

/** Stage by catalogue category id (the same ids `guTheoLoai` reads). */
export function sanKhauTheoLoai(loai: string | undefined): SanKhau {
  switch (loai) {
    case "quan-an-local":
      return "hien-quan";
    case "cafe":
      return "cua-kinh";
    case "vui-choi":
      return "doi";
    case "di-choi-dem":
      return "pho-dem";
    default:
      return "to-giay";
  }
}

/** Props a tag can add. Folded whole-string match, like `guTheoTag`. */
export type DaoCu = "doi-sau" | "nhom-dong" | "chill" | "noi" | "may" | "them-den" | "khoi" | "hoa" | "coc";
const DAO_CU: readonly (readonly [RegExp, DaoCu])[] = [
  [/^view dep$/, "doi-sau"],
  [/^(nhom dong|lau)$/, "nhom-dong"],
  [/^(chill|nhe nhang)$/, "chill"],
  [/^(mon local|lau)$/, "noi"],
  [/^(ngoai troi|outdoor|san may)$/, "may"],
  [/^(di dem|nhon nhip|nightlife)$/, "them-den"],
  [/^(bbq|nuong)$/, "khoi"],
  [/^(hoa|chup anh)$/, "hoa"],
  [/^(ca phe|cafe|coffee|tra)$/, "coc"],
];

/**
 * The props each stage knows how to draw, and whether the compact (4:1) reading
 * keeps them. A tag naming any other prop changes nothing and is not described;
 * the a11y sentence is built from this same table, so it never names a layer
 * that is not on the sheet (finish review 11/09).
 */
const DAO_CU_CUA: Record<SanKhau, Partial<Record<DaoCu, "ca-hai" | "chi-day">>> = {
  "hien-quan": { "doi-sau": "ca-hai", "nhom-dong": "ca-hai", noi: "ca-hai", chill: "chi-day" },
  "cua-kinh": { chill: "ca-hai", may: "chi-day" },
  doi: { may: "ca-hai", hoa: "chi-day" },
  "pho-dem": { "them-den": "ca-hai", "nhom-dong": "chi-day" },
  "to-giay": {},
};

/**
 * At most two props the stage draws, in tag order, no duplicates. The compact
 * reading is a SUBSET of the full one: the two are chosen for the full sheet,
 * then the compact sheet drops those it does not draw -- it never refills the
 * second slot with a later tag, so the 4:1 crop is the same drawing with less,
 * not a different drawing («một hình, hai khung cắt»).
 */
export function daoCuTheoTag(tags: readonly string[], sanKhau?: SanKhau, gon = false): DaoCu[] {
  const bang = sanKhau === undefined ? null : DAO_CU_CUA[sanKhau];
  const day: DaoCu[] = [];
  for (const tag of tags) {
    const t = gapChu(tag).trim();
    const hit = DAO_CU.find(([re]) => re.test(t));
    if (!hit || day.includes(hit[1])) continue;
    if (bang !== null && bang[hit[1]] === undefined) continue;
    day.push(hit[1]);
    if (day.length === 2) break;
  }
  return gon && bang !== null ? day.filter((d) => bang[d] === "ca-hai") : day;
}

// --- props -------------------------------------------------------------------
const san = (): LopVe[] => [m(netGay([[0, SAN], [KHUNG_KY_HOA.w, SAN]]), XA)];
/** Two-tier pine with a trunk, filled -- not an arrow (blind read 11/09). */
const thong = (x: number, y: number, s = 1): LopVe[] => [
  fill(daGiac([[x, y - 16 * s], [x + 6 * s, y - 7 * s], [x + 3 * s, y - 7 * s], [x + 8 * s, y], [x - 8 * s, y], [x - 3 * s, y - 7 * s], [x - 6 * s, y - 7 * s]]), "muc"),
  m(netGay([[x, y], [x, y + 5 * s]]), XA),
];
const khoi = (x: number, y: number): LopVe[] => [
  m(cong([x, y], [x - 4, y - 7], [x + 5, y - 12], [x, y - 20]), XA),
  m(cong([x + 8, y], [x + 4, y - 6], [x + 12, y - 11], [x + 8, y - 18]), XA),
];
/** A bowl standing ON a surface at `yMat`: rim above, belly down to the surface. */
const bat = (cx: number, yMat: number): LopVe[] => [
  m(cungTron(cx, yMat - 6, 6, 0.04 * PI, 0.96 * PI)),
  m(netGay([[cx - 7, yMat - 6], [cx + 7, yMat - 6]])),
];
const denTreo = (x: number, yTop: number, yBulb: number): LopVe[] => [
  m(netGay([[x, yTop], [x, yBulb - 5]]), XA),
  { d: tron(x, yBulb, 4.2), mau: "gap" },
];
const gheDau = (x: number, yMat = 70): LopVe[] => [
  m(netGay([[x - 7, yMat], [x + 7, yMat]])),
  m(netGay([[x - 5, yMat], [x - 5, SAN]])),
  m(netGay([[x + 5, yMat], [x + 5, SAN]])),
];
/** Scalloped canopy band: straight top, wavy bottom, one ink line on top. */
const maiVat = (x0: number, x1: number, yTop: number, yBot: number, buoc = 12): LopVe[] => {
  const pts: [number, number][] = [[x0, yTop], [x1, yTop]];
  for (let x = x1; x > x0; x -= buoc) {
    pts.push([x, yBot]);
    pts.push([x - buoc / 2, yBot + 3]);
  }
  pts.push([x0, yBot]);
  return [fill(daGiac(pts)), m(netGay([[x0, yTop], [x1, yTop]]), GAN)];
};
/** A cloud: three bumps on a flat base. */
const may = (cx: number, cy: number): LopVe[] => [
  m(cungTron(cx - 10, cy, 6, PI, 2 * PI), XA),
  m(cungTron(cx + 1, cy - 2, 8, PI, 2 * PI), XA),
  m(cungTron(cx + 12, cy, 6, PI, 2 * PI), XA),
  m(netGay([[cx - 16, cy], [cx + 18, cy]]), XA),
];
/** A cubic edge closed down to the floor: the fill follows the very same curve as its line. */
const toXuongSan = (duongCong: string, mau: LopVe["mau"] = "bong"): LopVe =>
  fill(`${duongCong} L ${KHUNG_KY_HOA.w} ${SAN + 1} L 0 ${SAN + 1} Z`, mau);

export interface TuyChonKyHoa {
  /** The 4:1 reading: fewer layers, same stage. */
  gon?: boolean;
}

// --- stages ------------------------------------------------------------------
function hienQuan(dc: readonly DaoCu[], gon: boolean): LopVe[] {
  const lop: LopVe[] = [];
  if (dc.includes("doi-sau")) {
    // the view past the awning: a shaded far ridge and a nearer line, so the hill has a body, not two floating strokes (blind read 11/09 v3)
    const xa = cong([180, 54], [214, 30], [250, 38], [KHUNG_KY_HOA.w, 44]);
    // closed only under the view (x ≥ 180), never under the table
    lop.push(fill(`${xa} L ${KHUNG_KY_HOA.w} ${SAN + 1} L 180 ${SAN + 1} Z`), m(xa, XA), m(cong([180, 66], [220, 52], [258, 60], [KHUNG_KY_HOA.w, 54]), XA));
    if (!gon) lop.push(...thong(246, 46, 0.8));
  }
  lop.push(...san());
  lop.push(...maiVat(12, 184, 14, 20), m(netGay([[18, 20], [18, SAN]]), GAN), m(netGay([[178, 20], [178, SAN]]), GAN));
  lop.push(fill(daGiac([[40, 60], [162, 60], [162, 64], [40, 64]])), m(netGay([[36, 60], [166, 60]]), GAN), m(netGay([[48, 64], [48, SAN]]), GAN), m(netGay([[154, 64], [154, SAN]]), GAN));
  if (!gon) lop.push(...gheDau(70), ...gheDau(132));
  const cx = dc.includes("nhom-dong") ? [58, 78, 126, 146] : [70, 134];
  for (const c of cx) lop.push(...bat(c, 60));
  // a round pot with its lid and handles, steaming: a box read as a parcel (blind read 11/09 v3); «Món local»/«Lẩu» make it the big one
  const w = dc.includes("noi") ? 30 : 22;
  const x0 = 103 - w / 2;
  lop.push(m(khungBo(x0, 50, w, 10, 5)), m(cungTron(103, 50, w / 2 - 1, PI, 2 * PI)), m(tron(103, 50 - (w / 2 - 1), 1.6)), m(netGay([[x0 - 5, 54], [x0, 54]])), m(netGay([[x0 + w, 54], [x0 + w + 5, 54]])), ...khoi(99, 40));
  // a hanging pot with leaves under the awning, clear of the pot's smoke: two thin wavy strokes read as «a hook with smoke» (finish review 11/09)
  if (dc.includes("chill")) {
    lop.push(m(netGay([[60, 20], [60, 28]]), XA));
    lop.push(fill(daGiac([[54, 28], [66, 28], [64, 36], [56, 36]]), "giay"), m(daGiac([[54, 28], [66, 28], [64, 36], [56, 36]])));
    for (const [x, y] of [[55, 42], [60, 45], [65, 42]] as const) lop.push(m(netGay([[x, 36], [x, y - 3]]), XA), fill(bau(x, y, 2.2, 3.6), "muc"));
  }
  lop.push(...denTreo(140, 20, 34));
  return lop;
}
function cuaKinh(dc: readonly DaoCu[], gon: boolean): LopVe[] {
  const lop: LopVe[] = [];
  lop.push(...san());
  lop.push(m(netGay([[22, 10], [158, 10]]), VUA));
  // through the glass: a far hill in shade in the lower third, trees standing ON it (not on a crossbar: blind read 11/09 v3)
  lop.push(fill(daGiac([[31, 73], [31, 62], [56, 50], [92, 58], [124, 48], [149, 54], [149, 73]])));
  if (!gon) for (const [x, y] of [[62, 52], [86, 58], [108, 52], [126, 49]] as const) lop.push(...thong(x, y, 0.4));
  lop.push(m(khungBo(30, 12, 120, 62, 2), GAN), m(netGay([[90, 12], [90, 74]])));
  if (dc.includes("chill")) lop.push(fill(daGiac([[30, 12], [54, 12], [46, 40], [50, 74], [30, 74]])), m(netCong([54, 12], [[[50, 26], [44, 30], [46, 40]], [[48, 56], [52, 66], [50, 74]]]), XA));
  lop.push(m(netGay([[170, 62], [214, 62]]), GAN), m(netGay([[192, 62], [192, SAN]]), VUA), m(netGay([[182, SAN], [202, SAN]]), VUA));
  lop.push(m(khungBo(184, 50, 15, 12, 2)), m(cungTron(200, 56, 4, -0.5 * PI, 0.5 * PI)));
  lop.push(...khoi(189, 48));
  lop.push(m(netGay([[236, 50], [236, SAN]])), m(netGay([[236, 66], [254, 66]])), m(netGay([[252, 66], [252, SAN]])), m(netGay([[236, 50], [240, 50]])));
  if (dc.includes("may")) lop.push(...may(190, 18));
  lop.push(m(netGay([[232, 10], [232, 22]]), XA), fill(daGiac([[224, 22], [240, 22], [244, 32], [220, 32]]), "giay"), m(daGiac([[224, 22], [240, 22], [244, 32], [220, 32]])), { d: tron(232, 36, 3.6), mau: "gap" });
  return lop;
}
function doi(dc: readonly DaoCu[], gon: boolean): LopVe[] {
  const lop: LopVe[] = [];
  lop.push(m(cong([0, 52], [70, 28], [130, 56], [KHUNG_KY_HOA.w, 38]), XA));
  const giua = cong([0, 66], [60, 46], [150, 70], [KHUNG_KY_HOA.w, 58]);
  lop.push(m(giua, VUA));
  if (!gon) lop.push(...thong(196, 56, 0.9), ...thong(212, 60, 0.7));
  const gan = cong([0, 74], [90, 64], [200, 82], [KHUNG_KY_HOA.w, 72]);
  lop.push(toXuongSan(gan, "giay"), m(gan, GAN));
  if (dc.includes("may")) lop.push(...may(58, 24), ...(gon ? [] : may(236, 16)));
  if (dc.includes("hoa")) for (const [x, y] of [[100, 72], [116, 76], [132, 74]] as const) lop.push(m(tron(x, y, 2.2), XA), m(netGay([[x, y + 2], [x, y + 7]]), XA));
  // where you stand: a wooden railing planted in the near ground, below the ridge lines
  lop.push(m(netGay([[14, 64], [14, SAN]]), GAN), m(netGay([[66, 66], [66, SAN]]), GAN), m(netGay([[6, 70], [74, 72]]), VUA), m(netGay([[6, 77], [74, 79]]), VUA));
  lop.push(...san());
  lop.push({ d: quat(236, 40, 9, PI, 2 * PI), mau: "gap" });
  return lop;
}
function phoDem(dc: readonly DaoCu[], gon: boolean): LopVe[] {
  const lop: LopVe[] = [];
  lop.push(...san());
  // a crescent, not a ring (a hollow circle read as a hoop: blind read 11/09 v3)
  lop.push(m(cungTron(18, 30, 7, 0.35 * PI, 1.65 * PI), XA), m(cungTron(21, 30, 6, 0.4 * PI, 1.6 * PI), XA));
  const yDay = (x: number) => 12 + 22 * Math.sin((x / KHUNG_KY_HOA.w) * PI);
  lop.push(m(cong([0, 12], [96, 42], [192, 42], [KHUNG_KY_HOA.w, 12])));
  const bong = dc.includes("them-den") ? [44, 88, 132, 176, 220] : [88, 132, 176];
  for (const x of bong) {
    const y = yDay(x);
    lop.push(m(netGay([[x, y], [x, y + 5]]), XA), fill(giot(x, y + 10, 3.2), "muc"));
  }
  const quay = (x0: number, w: number, hang: "bat" | "long-den" | "khong"): LopVe[] => {
    const x1 = x0 + w;
    const q = [...maiVat(x0 - 4, x1 + 4, 44, 50, 10), m(netGay([[x0, 50], [x0, SAN]]), GAN), m(netGay([[x1, 50], [x1, SAN]]), GAN)];
    q.push(m(netGay([[x0 - 4, 66], [x1 + 4, 66]]), GAN), m(netGay([[x0 - 2, 74], [x1 + 2, 74]]), XA));
    if (hang === "bat") for (const cx of [x0 + 14, x0 + w / 2, x1 - 14]) q.push(...bat(cx, 66));
    if (hang === "long-den")
      for (const cx of [x0 + 18, x1 - 18])
        q.push(m(netGay([[cx, 50], [cx, 53]]), XA), m(netGay([[cx - 3, 53], [cx + 3, 53]])), fill(bau(cx, 59, 4, 5)), m(bau(cx, 59, 4, 5), XA), m(netGay([[cx, 64], [cx, 68]]), XA));
    return q;
  };
  lop.push(...quay(34, 70, gon ? "khong" : "bat"));
  lop.push(...quay(160, 70, gon ? "khong" : dc.includes("nhom-dong") ? "bat" : "long-den"));
  lop.push({ d: giot(132, yDay(132) + 10, 3.2), mau: "gap" });
  return lop;
}
/** An unknown category: the floor, a folded tag on it, one lamp -- still a sheet, never nothing. */
function toGiay(gon: boolean): LopVe[] {
  const lop: LopVe[] = [...san()];
  lop.push(fill(daGiac([[120, 40], [156, 40], [168, 52], [168, SAN - 2], [120, SAN - 2]]), "giay"), m(daGiac([[120, 40], [156, 40], [168, 52], [168, SAN - 2], [120, SAN - 2]])), fill(daGiac([[156, 40], [156, 52], [168, 52]]), "gap"));
  lop.push(m(netGay([[130, 58], [158, 58]]), XA), m(netGay([[130, 66], [152, 66]]), XA));
  if (!gon) lop.push(m(netGay([[130, 74], [146, 74]]), XA));
  return lop;
}

const SAN_KHAU: Record<SanKhau, (dc: readonly DaoCu[], gon: boolean) => LopVe[]> = {
  "hien-quan": hienQuan,
  "cua-kinh": cuaKinh,
  doi,
  "pho-dem": phoDem,
  "to-giay": (_dc, gon) => toGiay(gon),
};

/** The sketch of a kind of place: stage by category, props by tags, one coral light. */
export function hinhKyHoa(loai: string | undefined, tags: readonly string[], tuyChon: TuyChonKyHoa = {}): LopVe[] {
  const san = sanKhauTheoLoai(loai);
  const gon = tuyChon.gon ?? false;
  return SAN_KHAU[san](daoCuTheoTag(tags, san, gon), gon);
}

const TEN_SAN_KHAU: Record<SanKhau, string> = {
  "hien-quan": "hiên quán có bàn dài",
  "cua-kinh": "góc cửa kính có bàn nhỏ",
  doi: "đồi thông nhìn từ lan can",
  "pho-dem": "quầy đêm dưới dây đèn",
  "to-giay": "một tờ ghi chưa có hình",
};
const TEN_DAO_CU: Record<DaoCu, string> = {
  "doi-sau": "đồi phía sau",
  "nhom-dong": "nhiều bát",
  chill: "cây treo",
  noi: "nồi lớn giữa bàn",
  may: "mây",
  "them-den": "thêm đèn",
  khoi: "khói than",
  hoa: "hoa ven đường",
  coc: "cốc có hơi",
};
/** Where a prop is drawn as something else on one stage, the sentence says that thing. */
const TEN_DAO_CU_THEO_SAN: Partial<Record<SanKhau, Partial<Record<DaoCu, string>>>> = {
  "cua-kinh": { chill: "rèm kéo" },
  "pho-dem": { "nhom-dong": "hai quầy đều bày bát" },
};

/** One a11y sentence for one reading: a sketch of a kind of place, naming only what that reading draws. */
export function moTaKyHoa(loai: string | undefined, tags: readonly string[], tuyChon: TuyChonKyHoa = {}): string {
  const san = sanKhauTheoLoai(loai);
  const gon = tuyChon.gon ?? false;
  const dc = daoCuTheoTag(tags, san, gon).map((d) => TEN_DAO_CU_THEO_SAN[san]?.[d] ?? TEN_DAO_CU[d]);
  // The compact hill drops its pines; the stage name must follow the crop
  // just as prop descriptions do, rather than naming a tree that is absent.
  const than = san === "doi" && gon ? "đồi nhìn từ lan can" : TEN_SAN_KHAU[san];
  return `Ký hoạ ${than}${dc.length ? `, ${dc.join(", ")}` : ""}`;
}
