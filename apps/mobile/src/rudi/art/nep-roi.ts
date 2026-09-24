/**
 * Nếp as a paper puppet (ADR-0037 D5, plan S0.4).
 *
 * The still figure (`nep.ts`) is one drawing per pose. The puppet is that same
 * sheet and that same face -- taken from `thanNep` / `matNepPhan`, never
 * redrawn -- with the four limbs cut free and pinned where the still figure
 * hinges them (`VAI_*`, `HONG_*`). A performance moves the pins, not the
 * paper:
 *   - the sheet leans with the still figure's own shear (`nghieng`: the top
 *     travels, the feet stay), turns about its hips (`xoay`, a bow), sinks
 *     toward its feet (`ha`, a crouch) and travels (`dx`, `dy`);
 *   - each arm and leg is two RIGID bones solved to reach a hand or an ankle
 *     target (two-bone IK): paper does not stretch, so a target out of reach
 *     gets a limb held straight toward it;
 *   - the face swaps between the seven expressions the still figure has, and
 *     the eyes follow `nhin` exactly as they do there.
 *
 * One geometry, two outputs:
 *   - `tuTheRoi(tt)` -> `LopVe[]` in the 96-box: the SVG renderer's frame and
 *     what the node gates sweep (path grammar, the box, the coral fold);
 *   - `giaiRoi(tt)` + `maTranBoPhan(...)` -> one affine per part, for Skia's
 *     groups on the UI thread (every solver here is a worklet). The gate
 *     proves the matrices put each part exactly where `tuTheRoi` draws it.
 *
 * Draw order is the still figure's -- sheet, face, legs, arms -- so both arms
 * are in front of the sheet as they always were, and the coral fold rule
 * (`khongCatNepGap`) applies to every limb in every frame. Two brass pins, a
 * ring at each shoulder and hip, are drawn last: they are what says «paper
 * puppet» and not «the drawing wobbling».
 */
import { CHAN_NEP, HONG_GAN, HONG_XA, VAI_GAN, VAI_GAN_MANH, VAI_XA, matNepPhan, nguCanhNep, thanNep, tuTheCuaPose, type BieuCamNep, type GapNep } from "./nep";
import { MA_TRAN_DON_VI, apMaTran, bienDoiLop, khungBo, netGay, nhanMaTran, thon, tron, vien, type Diem, type LopVe, type MaTran } from "./net";
import { hinhTem } from "./giay";

/** Bone lengths in the 96-box: upper arm, forearm, thigh, shin. */
export const XUONG = Object.freeze({ canhTay: 12, cangTay: 12, dui: 8.5, cang: 7.5 });

/** The sheet turns (a bow) about the middle of its hips. */
export const TAM_HONG: Diem = [47.5, 75.5];

/** Where the ankles stand in the still figure's `dung` stance. */
export const CO_CHAN_GAN: Diem = [36, 90];
export const CO_CHAN_XA: Diem = [60, 90];

/** What the puppet holds; each is drawn in its hand's frame, never coral. */
export const VAT_CAM = ["may-anh", "con-dau", "to-thu", "tem", "the-keo"] as const;
export type VatCam = (typeof VAT_CAM)[number];

/**
 * Where a prop is. `tay-xa` / `tay-gan`: in that hand (its frame turns with
 * the forearm); `giua`: between the hands, upright; `tu-do`: let go, at
 * (`x`, `y`) in the figure's frame, turned `xoay` degrees. `gap` folds a sheet
 * (0 open .. 1 folded in half) and `hien` fades the prop (0..1).
 */
export interface ViTriVat {
  id: VatCam;
  gan: "tay-xa" | "tay-gan" | "giua" | "tu-do";
  x: number;
  y: number;
  xoay: number;
  gap: number;
  hien: number;
}

/**
 * One pose of the puppet. Hands and ankles are TARGETS in the figure's own
 * frame (before `dx`/`dy`): the solver reaches for them. `uon*` picks which
 * way an elbow or a knee bends (+1 / -1 across the reach line).
 */
export interface TuTheRoi {
  dx: number;
  dy: number;
  /** The sheet sinks this far toward the feet (a crouch); the feet stay. */
  ha: number;
  /** The still figure's lean: the top of the sheet travels this far sideways. */
  nghieng: number;
  /** A turn of the sheet about its hips, degrees, clockwise on screen (a bow to the right). */
  xoay: number;
  nhin: readonly [number, number];
  bieuCam: BieuCamNep;
  tayGan: Diem;
  tayXa: Diem;
  chanGan: Diem;
  chanXa: Diem;
  uonTayGan: number;
  uonTayXa: number;
  uonGoiGan: number;
  uonGoiXa: number;
  vat: ViTriVat | null;
}

/** Standing at rest, arms loose, looking a little toward the page. */
export const TU_THE_NGHI: TuTheRoi = Object.freeze({
  dx: 0,
  dy: 0,
  ha: 0,
  nghieng: 0,
  xoay: 0,
  nhin: [0.4, 0.2] as const,
  bieuCam: "binh-than" as BieuCamNep,
  // Near full reach, so a resting arm hangs with a slight give at the elbow
  // instead of the chicken wing a short target folds it into.
  tayGan: [12, 72] as Diem,
  tayXa: [87, 67] as Diem,
  chanGan: CO_CHAN_GAN,
  chanXa: CO_CHAN_XA,
  uonTayGan: 1,
  uonTayXa: 1,
  uonGoiGan: -1,
  uonGoiXa: -1,
  vat: null,
});

/** A pose from the rest pose and the fields that differ. */
export function tuThe(khac: Partial<TuTheRoi>): TuTheRoi {
  return { ...TU_THE_NGHI, ...khac };
}

// --- the solver (worklets: they run per frame on the UI thread) --------------

/**
 * The sheet's own map: the still figure's shear about the ground line (so the
 * feet stay and the head travels `nghieng`), then the turn about the hips,
 * then the sink and the travel.
 */
export function maTranThan(tt: TuTheRoi): MaTran {
  "worklet";
  const k = tt.nghieng / (CHAN_NEP - 20);
  const cat: MaTran = [1, 0, -k, 1, k * CHAN_NEP, 0];
  const r = (tt.xoay * Math.PI) / 180;
  const cs = Math.cos(r);
  const sn = Math.sin(r);
  const hx = TAM_HONG[0];
  const hy = TAM_HONG[1];
  const quay: MaTran = [cs, sn, -sn, cs, hx - cs * hx + sn * hy, hy - sn * hx - cs * hy];
  const dich: MaTran = [1, 0, 0, 1, tt.dx, tt.dy + tt.ha];
  return nhanMaTran(dich, nhanMaTran(quay, cat));
}

/**
 * Two rigid bones from `goc` reaching for `dich`: the joint and the end.
 * Out of reach, the limb points straight at the target at full length; too
 * close, it folds as far as its bones allow. `uon` picks the side of the
 * reach line the joint bends to.
 */
export function ik2(goc: Diem, dich: Diem, l1: number, l2: number, uon: number): { khop: Diem; cuoi: Diem } {
  "worklet";
  const vx = dich[0] - goc[0];
  const vy = dich[1] - goc[1];
  const dTho = Math.sqrt(vx * vx + vy * vy);
  const ux = dTho > 1e-9 ? vx / dTho : 0;
  const uy = dTho > 1e-9 ? vy / dTho : 1;
  const d = Math.min(l1 + l2, Math.max(Math.abs(l1 - l2) + 1e-6, dTho));
  const a = (l1 * l1 - l2 * l2 + d * d) / (2 * d);
  const h = Math.sqrt(Math.max(0, l1 * l1 - a * a));
  const s = uon >= 0 ? 1 : -1;
  const khop: Diem = [goc[0] + ux * a - uy * h * s, goc[1] + uy * a + ux * h * s];
  const cuoi: Diem = [goc[0] + ux * d, goc[1] + uy * d];
  return { khop, cuoi };
}

/** Every joint of one pose, in the figure's frame after travel. */
export interface GiaiRoi {
  than: MaTran;
  vaiGan: Diem;
  vaiXa: Diem;
  hongGan: Diem;
  hongXa: Diem;
  khuyuGan: Diem;
  khuyuXa: Diem;
  tayGan: Diem;
  tayXa: Diem;
  goiGan: Diem;
  goiXa: Diem;
  coChanGan: Diem;
  coChanXa: Diem;
}

export function giaiRoi(tt: TuTheRoi, gap: GapNep = "trang"): GiaiRoi {
  "worklet";
  const than = maTranThan(tt);
  const vaiGan = apMaTran(than, gap === "manh" ? VAI_GAN_MANH : VAI_GAN);
  const vaiXa = apMaTran(than, VAI_XA);
  const hongGan = apMaTran(than, HONG_GAN);
  const hongXa = apMaTran(than, HONG_XA);
  const theoDich = (p: Diem): Diem => [p[0] + tt.dx, p[1] + tt.dy];
  const tayG = ik2(vaiGan, theoDich(tt.tayGan), XUONG.canhTay, XUONG.cangTay, tt.uonTayGan);
  const tayX = ik2(vaiXa, theoDich(tt.tayXa), XUONG.canhTay, XUONG.cangTay, tt.uonTayXa);
  const chanG = ik2(hongGan, theoDich(tt.chanGan), XUONG.dui, XUONG.cang, tt.uonGoiGan);
  const chanX = ik2(hongXa, theoDich(tt.chanXa), XUONG.dui, XUONG.cang, tt.uonGoiXa);
  return {
    than,
    vaiGan,
    vaiXa,
    hongGan,
    hongXa,
    khuyuGan: tayG.khop,
    khuyuXa: tayX.khop,
    tayGan: tayG.cuoi,
    tayXa: tayX.cuoi,
    goiGan: chanG.khop,
    goiXa: chanX.khop,
    coChanGan: chanG.cuoi,
    coChanXa: chanX.cuoi,
  };
}

/**
 * The map that lays a bone drawn from (0, 0) down to (0, len) onto the
 * segment `dau` -> `cuoi`: a turn and a move, never a stretch.
 */
export function maTranXuong(dau: Diem, cuoi: Diem): MaTran {
  "worklet";
  const vx = cuoi[0] - dau[0];
  const vy = cuoi[1] - dau[1];
  const l = Math.sqrt(vx * vx + vy * vy);
  const ux = l > 1e-9 ? vx / l : 0;
  const uy = l > 1e-9 ? vy / l : 1;
  return [uy, -ux, ux, uy, dau[0], dau[1]];
}

/** A plain move. */
export function maTranDich(p: Diem): MaTran {
  "worklet";
  return [1, 0, 0, 1, p[0], p[1]];
}

/** The map of each moving part for one solved pose. */
export interface MaTranBoPhan {
  than: MaTran;
  /**
   * The two eyes and the round «hoi» mouth. The still figure draws these as
   * upright ellipses at a leaned CENTRE (only the centre goes through the
   * shear), so they are moved, never sheared: the sheet's rigid part after a
   * move to the leaned centre.
   */
  matTrai: MaTran;
  matPhai: MaTran;
  miengHoi: MaTran;
  canhTayGan: MaTran;
  cangTayGan: MaTran;
  canhTayXa: MaTran;
  cangTayXa: MaTran;
  duiGan: MaTran;
  cangGan: MaTran;
  banChanGan: MaTran;
  duiXa: MaTran;
  cangXa: MaTran;
  banChanXa: MaTran;
  vat: MaTran;
  ghimVaiGan: MaTran;
  ghimVaiXa: MaTran;
  ghimHongGan: MaTran;
  ghimHongXa: MaTran;
}

/**
 * Where a prop's own frame is for this pose: its grip point at the hand (or
 * between the hands, or wherever it was let go), turned only by its own
 * `xoay`. A held thing keeps roughly upright in the hand rather than turning
 * with the forearm: a camera raised on a forearm pointing up-right would
 * otherwise be drawn on its back.
 */
export function maTranVat(tt: TuTheRoi, g: GiaiRoi): MaTran {
  "worklet";
  const v = tt.vat;
  if (!v) return MA_TRAN_DON_VI;
  const r = (v.xoay * Math.PI) / 180;
  const cs = Math.cos(r);
  const sn = Math.sin(r);
  let x = v.x + tt.dx;
  let y = v.y + tt.dy;
  if (v.gan === "tay-xa") {
    x = g.tayXa[0] + v.x;
    y = g.tayXa[1] + v.y;
  } else if (v.gan === "tay-gan") {
    x = g.tayGan[0] + v.x;
    y = g.tayGan[1] + v.y;
  } else if (v.gan === "giua") {
    x = (g.tayGan[0] + g.tayXa[0]) / 2 + v.x;
    y = (g.tayGan[1] + g.tayXa[1]) / 2 + v.y;
  }
  return [cs, sn, -sn, cs, x, y];
}

/** The eyes' and the round mouth's centres on the upright sheet (`nep.ts`). */
export const TAM_MAT_TRAI: Diem = [39, 45];
export const TAM_MAT_PHAI: Diem = [52, 43];
export const TAM_MIENG_HOI: Diem = [50.5, 54.6];

export function maTranBoPhan(tt: TuTheRoi, g: GiaiRoi): MaTranBoPhan {
  "worklet";
  const nx = Math.max(-3, Math.min(3, tt.nhin[0]));
  const ny = Math.max(-3, Math.min(3, tt.nhin[1]));
  const cung = maTranThan({ ...tt, nghieng: 0 });
  const k = tt.nghieng / (CHAN_NEP - 20);
  // A point of the upright sheet leaned the still figure's way, then moved rigidly.
  const tam = (x: number, y: number): MaTran => nhanMaTran(cung, [1, 0, 0, 1, x + k * (CHAN_NEP - y), y]);
  return {
    than: g.than,
    matTrai: tam(TAM_MAT_TRAI[0] + nx, TAM_MAT_TRAI[1] + ny),
    matPhai: tam(TAM_MAT_PHAI[0] + nx, TAM_MAT_PHAI[1] + ny),
    miengHoi: tam(TAM_MIENG_HOI[0], TAM_MIENG_HOI[1]),
    canhTayGan: maTranXuong(g.vaiGan, g.khuyuGan),
    cangTayGan: maTranXuong(g.khuyuGan, g.tayGan),
    canhTayXa: maTranXuong(g.vaiXa, g.khuyuXa),
    cangTayXa: maTranXuong(g.khuyuXa, g.tayXa),
    duiGan: maTranXuong(g.hongGan, g.goiGan),
    cangGan: maTranXuong(g.goiGan, g.coChanGan),
    banChanGan: maTranDich(g.coChanGan),
    duiXa: maTranXuong(g.hongXa, g.goiXa),
    cangXa: maTranXuong(g.goiXa, g.coChanXa),
    banChanXa: maTranDich(g.coChanXa),
    vat: maTranVat(tt, g),
    ghimVaiGan: maTranDich(g.vaiGan),
    ghimVaiXa: maTranDich(g.vaiXa),
    ghimHongGan: maTranDich(g.hongGan),
    ghimHongXa: maTranDich(g.hongXa),
  };
}

// --- the parts (built once, in their own frames) ------------------------------

export interface TuyChonRoi {
  /** The 96dp reading (brow, finer ink) or the compact one. */
  chiTiet?: boolean;
  gap?: GapNep;
}

/** Every piece of paper the puppet is cut from, each in its own frame. */
export interface BoPhanRoi {
  /** The sheet, upright, in the 96-box (no lean: the sheet's map adds it). */
  than: LopVe[];
  /** One eye, centred on (0, 0); `matTrai` / `matPhai` place the two. */
  mat: LopVe[];
  /** Brow and mouth on the upright sheet, one set per expression (the round «hoi» mouth apart). */
  mat7: Record<BieuCamNep, LopVe[]>;
  /** The round «hoi» mouth, centred on (0, 0). */
  miengHoi: LopVe[];
  canhTay: LopVe[];
  cangTay: LopVe[];
  dui: LopVe[];
  cang: LopVe[];
  banChanGan: LopVe[];
  banChanXa: LopVe[];
  ghim: LopVe[];
  vat: Record<VatCam, { la: LopVe[]; gapLai?: LopVe[] }>;
}

const BIEU_CAM_ROI: readonly BieuCamNep[] = ["binh-than", "hao-hung", "hoi", "quyet", "met", "nhuong", "giu-kin"];

/** The props, each drawn about its grip point (0, 0): paper and ink, never coral. */
function hinhVat(chiTiet: boolean): BoPhanRoi["vat"] {
  const w = chiTiet ? 1.4 : 1.8;
  const may = khungBo(-2, -7, 14, 9.5, 2);
  const tem = hinhTem(15, 12, 3.2, 1);
  // A stamp reads as one when it has a picture: a frame line and a small hill under a sun.
  const temDat = bienDoiLop(
    [
      { d: tem, mau: "giay" },
      { d: tem, mau: "muc", net: w },
      { d: khungBo(2.8, 2.6, 9.4, 6.8, 0.4), mau: "muc", net: w * 0.6 },
      { d: tron(9.4, 4.8, 1.1), mau: "muc" },
      { d: netGay([[3.4, 8.8], [6, 6], [8, 7.8], [11.6, 8.8]]), mau: "muc", net: w * 0.7 },
    ],
    [1, 0, 0, 1, -7.5, -15],
  );
  // The sheet in the hand, as two panels either side of a fold line at x = 0:
  // the right panel is what `gap` turns over onto the left one.
  const trai = khungBo(-9, -6, 9, 11, 0.8);
  const phai = khungBo(0, -6, 9, 11, 0.8);
  return {
    "may-anh": {
      la: [
        { d: may, mau: "giay" },
        { d: may, mau: "muc", net: w },
        { d: tron(6, -2.2, 2.7), mau: "muc", net: w },
        { d: khungBo(7.5, -9, 4, 2, 0.6), mau: "muc" },
      ],
    },
    // A rubber stamp held by its knob: the neck under the mitten, the wooden
    // block, and the inked rubber face at the bottom.
    "con-dau": {
      la: [
        { d: vien([0, 1], [0, 5], 3.2), mau: "muc" },
        { d: khungBo(-7, 4.5, 14, 5, 1.4), mau: "giay" },
        { d: khungBo(-7, 4.5, 14, 5, 1.4), mau: "muc", net: w },
        { d: khungBo(-7.5, 9.5, 15, 2.8, 0.7), mau: "muc" },
      ],
    },
    "to-thu": {
      la: [
        { d: trai, mau: "giay" },
        { d: trai, mau: "muc", net: w },
      ],
      gapLai: [
        { d: phai, mau: "giay" },
        { d: phai, mau: "muc", net: w },
      ],
    },
    tem: { la: temDat },
    "the-keo": {
      la: [
        { d: khungBo(-1, -4, 13, 8, 2), mau: "giay" },
        { d: khungBo(-1, -4, 13, 8, 2), mau: "muc", net: w },
        { d: vien([8, -2], [8, 2], 1.2), mau: "muc" },
      ],
    },
  };
}

export function boPhanRoi(tuyChon: TuyChonRoi = {}): BoPhanRoi {
  const { chiTiet = true, gap = "trang" } = tuyChon;
  const ctx0 = nguCanhNep("moi", { chiTiet, gap, nghieng: 0, nhin: [0, 0] });
  const mat7 = {} as Record<BieuCamNep, LopVe[]>;
  for (const bieuCam of BIEU_CAM_ROI) {
    const { may, mieng } = matNepPhan(nguCanhNep("moi", { chiTiet, gap, nghieng: 0, nhin: [0, 0], bieuCam }));
    mat7[bieuCam] = bieuCam === "hoi" ? [...may] : [...may, mieng];
  }
  // The same shapes the still figure draws, built about the origin.
  const tai = (x: number, y: number) => (l: LopVe): LopVe => ({ ...l, d: bienDoiLop([l], [1, 0, 0, 1, -x, -y])[0].d });
  const matGoc = matNepPhan(ctx0).mat[0];
  const miengHoiGoc = matNepPhan(nguCanhNep("moi", { chiTiet, gap, nghieng: 0, nhin: [0, 0], bieuCam: "hoi" })).mieng;
  const wTay = ctx0.wTay;
  const rBan = ctx0.rBan;
  const wDui = 5.5;
  const wGoi = 4.6;
  const wCo = 3.6;
  const rBanChan = 3.6;
  return {
    than: thanNep(ctx0),
    mat: [tai(TAM_MAT_TRAI[0], TAM_MAT_TRAI[1])(matGoc)],
    mat7,
    miengHoi: [tai(TAM_MIENG_HOI[0], TAM_MIENG_HOI[1])(miengHoiGoc)],
    canhTay: [{ d: vien([0, 0], [0, XUONG.canhTay], wTay), mau: "muc" }],
    cangTay: [
      { d: vien([0, 0], [0, XUONG.cangTay], wTay), mau: "muc" },
      { d: tron(0, XUONG.cangTay, rBan), mau: "muc" },
    ],
    dui: [{ d: thon([0, 0], [0, XUONG.dui], wDui, wGoi), mau: "muc" }],
    // The knee is a disc where the two tapered bones meet, so a bent leg
    // shows no notch on its outer side.
    cang: [
      { d: tron(0, 0, wGoi / 2), mau: "muc" },
      { d: thon([0, 0], [0, XUONG.cang], wGoi, wCo), mau: "muc" },
    ],
    // The feet keep the still figure's: the near heel reaches back, the far toe forward.
    banChanGan: [{ d: vien([-5, 1], [3, 1], rBanChan), mau: "muc" }],
    banChanXa: [{ d: vien([-2, 1], [7, 1], rBanChan), mau: "muc" }],
    // A brass pin: a paper ring on the ink, read at 48dp as one light dot.
    ghim: [
      { d: tron(0, 0, 1.6), mau: "muc" },
      { d: tron(0, 0, 0.8), mau: "giay" },
    ],
    vat: hinhVat(chiTiet),
  };
}

/** The folded panel's map inside the prop frame: turned over the fold line by `gap`. */
export function maTranGapTo(gap: number): MaTran {
  "worklet";
  const g = Math.max(0, Math.min(1, gap));
  return [1 - 2 * g, 0, 0, 1, 0, 0];
}

const cacheBoPhan = new Map<string, BoPhanRoi>();

function boPhanDaCo(tuyChon: TuyChonRoi): BoPhanRoi {
  const khoa = `${tuyChon.chiTiet ?? true}|${tuyChon.gap ?? "trang"}`;
  let bp = cacheBoPhan.get(khoa);
  if (!bp) {
    bp = boPhanRoi(tuyChon);
    cacheBoPhan.set(khoa, bp);
  }
  return bp;
}

/**
 * One pose of the puppet as layers in the 96-box, back to front: sheet, face,
 * legs, arms, prop, pins. The sheet and the face come from the still figure
 * with this pose's lean, gaze and face (so a puppet at rest is the still
 * figure's own paper), then only the rigid part of the sheet's map -- turn,
 * sink, travel -- is applied to them.
 */
export function tuTheRoi(tt: TuTheRoi, tuyChon: TuyChonRoi = {}): LopVe[] {
  const { chiTiet = true, gap = "trang" } = tuyChon;
  const bp = boPhanDaCo(tuyChon);
  const g = giaiRoi(tt, gap);
  const m = maTranBoPhan(tt, g);
  const ctx = nguCanhNep("moi", { chiTiet, gap, nghieng: tt.nghieng, nhin: tt.nhin, bieuCam: tt.bieuCam });
  const { mat, may, mieng } = matNepPhan(ctx);
  const cungNep = [...thanNep(ctx), ...mat, ...may, mieng];
  const cung = maTranCung(tt);
  const giay = cung === null ? cungNep : bienDoiLop(cungNep, cung);
  const ra: LopVe[] = [
    ...giay,
    ...bienDoiLop(bp.dui, m.duiGan),
    ...bienDoiLop(bp.cang, m.cangGan),
    ...bienDoiLop(bp.banChanGan, m.banChanGan),
    ...bienDoiLop(bp.dui, m.duiXa),
    ...bienDoiLop(bp.cang, m.cangXa),
    ...bienDoiLop(bp.banChanXa, m.banChanXa),
    ...bienDoiLop(bp.canhTay, m.canhTayXa),
    ...bienDoiLop(bp.cangTay, m.cangTayXa),
    ...bienDoiLop(bp.canhTay, m.canhTayGan),
    ...bienDoiLop(bp.cangTay, m.cangTayGan),
  ];
  if (tt.vat && tt.vat.hien > 0) {
    const v = bp.vat[tt.vat.id];
    ra.push(...bienDoiLop(v.la, m.vat));
    if (v.gapLai) ra.push(...bienDoiLop(v.gapLai, nhanMaTran(m.vat, maTranGapTo(tt.vat.gap))));
  }
  for (const ghim of [m.ghimVaiXa, m.ghimHongXa, m.ghimHongGan, m.ghimVaiGan]) ra.push(...bienDoiLop(bp.ghim, ghim));
  return ra;
}

/**
 * The rigid part of the sheet's map (turn, sink, travel), or null when there
 * is none: the still figure's own shear already put the lean in.
 */
export function maTranCung(tt: TuTheRoi): MaTran | null {
  if (tt.xoay === 0 && tt.dx === 0 && tt.dy === 0 && tt.ha === 0) return null;
  return maTranThan({ ...tt, nghieng: 0 });
}

/** A still pose of the puppet taken from one of the still figure's poses: its lean, gaze and face. */
export function tuTheTuPose(pose: string, khac: Partial<TuTheRoi> = {}): TuTheRoi {
  const { nghieng, nhin, bieuCam } = tuTheCuaPose(pose);
  return tuThe({ nghieng, nhin, bieuCam, ...khac });
}
