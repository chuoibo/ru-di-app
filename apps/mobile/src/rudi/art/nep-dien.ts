/**
 * The nine performances of the paper puppet (ADR-0037 D5, plan S0.4).
 *
 * A performance is a handful of key poses on a clock of at most `dien`
 * (1 400 ms), each reached with one of the shell's three easings, plus the
 * beats the hand feels (haptics) and the one sentence a screen reader hears.
 * Nếp performs at eight moments (`khoanh-khac.ts`); a moment names which
 * performance, never the other way round.
 *
 * Rules the keys keep, and the gates check on every frame:
 *   - no limb or prop ever paints on the coral fold (the still figure's rule);
 *   - props are paper and ink, never a second coral mark;
 *   - a performance at a money moment wears only `binh-than` or `nhuong`
 *     (ADR-0037 D5: Nếp never celebrates a number);
 *   - the last key is a pose the figure can hold: under Reduce Motion, or when
 *     the moment has already played, that frame (or `khungTinh`) is all there is.
 *
 * `khopTai` samples a performance at a time. It is a worklet: the renderer
 * calls it every frame on the UI thread.
 */
import { EASING, NGAN_SACH_SAN_KHAU, type EasingName } from "../motion";
import type { BieuCamNep } from "./nep";
import { tuThe, type TuTheRoi, type VatCam, type ViTriVat } from "./nep-roi";

export const TIET_MUC_IDS = ["buoc-vao", "keo-tab", "cam-may", "dong-dau", "cui-cam-on", "buoc-di", "gap-thu", "nhay", "nang-tem"] as const;
export type TietMucId = (typeof TIET_MUC_IDS)[number];

export type NhipRung = "chon" | "cham" | "xong";

export interface KhoaTietMuc {
  t: number;
  tt: TuTheRoi;
  /** How the pose before eases into this one. */
  em?: EasingName;
}

export interface TietMuc {
  id: TietMucId;
  ms: number;
  khoa: readonly KhoaTietMuc[];
  /** Haptic beats: `chon` (select), `cham` (impact), `xong` (success). */
  nhip: readonly { t: number; kieu: NhipRung }[];
  /** The still frame shown when the performance does not play: an index into `khoa`. */
  khungTinh: number;
  /** One sentence for a screen reader. */
  moTa: string;
  /** The prop this performance holds, if any. */
  vat: VatCam | null;
}

const vat = (id: VatCam, v: Partial<ViTriVat> = {}): ViTriVat => ({ id, gan: "tay-xa", x: 0, y: 0, xoay: 0, gap: 0, hien: 1, ...v });

// Walking: the ankle targets swap between a planted and a lifted foot, the
// sheet leaning into the direction of travel.
const BUOC_A = { chanGan: [31, 90] as const, chanXa: [66, 87] as const };
const BUOC_B = { chanGan: [42, 87] as const, chanXa: [55, 90] as const };

const buocVao: TietMuc = {
  id: "buoc-vao",
  ms: 1100,
  khungTinh: 5,
  moTa: "Nếp bước vào, đưa tay mời",
  vat: null,
  nhip: [{ t: 700, kieu: "chon" }],
  khoa: [
    { t: 0, tt: tuThe({ dx: -40, nghieng: 7, bieuCam: "hao-hung", nhin: [1.4, -0.2], ...BUOC_A, tayGan: [30, 64], tayXa: [80, 70] }) },
    { t: 240, em: "standard", tt: tuThe({ dx: -28, nghieng: 7, bieuCam: "hao-hung", nhin: [1.4, -0.2], ...BUOC_B, tayGan: [16, 70], tayXa: [86, 62] }) },
    { t: 480, em: "standard", tt: tuThe({ dx: -15, nghieng: 7, bieuCam: "hao-hung", nhin: [1.4, -0.2], ...BUOC_A, tayGan: [30, 64], tayXa: [80, 70] }) },
    { t: 700, em: "decelerate", tt: tuThe({ dx: -2, ha: 1.6, nghieng: 3, bieuCam: "binh-than", nhin: [1, 0.2], tayGan: [16, 70], tayXa: [82, 68] }) },
    { t: 860, em: "standard", tt: tuThe({ dx: 0, ha: 0, nghieng: 0, bieuCam: "nhuong", nhin: [0.8, 0.2], tayGan: [15, 67], tayXa: [88, 56] }) },
    { t: 1100, em: "standard", tt: tuThe({ bieuCam: "nhuong", nhin: [0.4, 0.2], tayGan: [15, 67], tayXa: [89, 54] }) },
  ],
};

const keoTab: TietMuc = {
  id: "keo-tab",
  ms: 1000,
  khungTinh: 5,
  moTa: "Nếp kéo tab mở sổ",
  vat: "the-keo",
  nhip: [
    { t: 450, kieu: "chon" },
    { t: 800, kieu: "chon" },
  ],
  khoa: [
    { t: 0, tt: tuThe({ vat: vat("the-keo", { hien: 0, x: 1, y: -1 }) }) },
    { t: 220, em: "standard", tt: tuThe({ nghieng: 3, bieuCam: "quyet", nhin: [1.6, 0.6], tayGan: [14, 62], tayXa: [90, 60], uonTayXa: 1, vat: vat("the-keo", { x: 1, y: -1 }) }) },
    { t: 450, em: "decelerate", tt: tuThe({ nghieng: -5, ha: 1, bieuCam: "quyet", nhin: [1.6, 0.6], tayGan: [11, 57], tayXa: [83, 61], vat: vat("the-keo", { x: 1, y: -1 }) }) },
    { t: 600, em: "standard", tt: tuThe({ nghieng: -1, ha: 0.4, bieuCam: "quyet", nhin: [1.6, 0.6], tayGan: [13, 60], tayXa: [87, 60], vat: vat("the-keo", { x: 1, y: -1 }) }) },
    { t: 800, em: "decelerate", tt: tuThe({ nghieng: -7, ha: 1.5, bieuCam: "quyet", nhin: [1.6, 0.6], tayGan: [10, 55], tayXa: [80, 62], vat: vat("the-keo", { x: 1, y: -1 }) }) },
    { t: 1000, em: "standard", tt: tuThe({ nghieng: -3, ha: 0.5, bieuCam: "hao-hung", nhin: [1.2, 0.2], tayGan: [12, 58], tayXa: [81, 62], vat: vat("the-keo", { x: 1, y: -1 }) }) },
  ],
};

// A money moment (M2): the face stays level; the camera does the talking.
const camMay: TietMuc = {
  id: "cam-may",
  ms: 900,
  khungTinh: 4,
  moTa: "Nếp giơ máy ảnh chụp hoá đơn",
  vat: "may-anh",
  nhip: [{ t: 450, kieu: "chon" }],
  khoa: [
    { t: 0, tt: tuThe({ tayXa: [84, 66], vat: vat("may-anh", { x: -1, y: 0 }) }) },
    { t: 260, em: "standard", tt: tuThe({ nghieng: 4, nhin: [1.6, 0.2], tayGan: [16, 64], tayXa: [84, 50], vat: vat("may-anh", { x: -1, y: 0 }) }) },
    { t: 450, em: "accelerate", tt: tuThe({ nghieng: 4, ha: 1, nhin: [1.6, 0.4], tayGan: [16, 65], tayXa: [84, 52], vat: vat("may-anh", { x: -1, y: 0, xoay: 4 }) }) },
    { t: 560, em: "decelerate", tt: tuThe({ nghieng: 4, nhin: [1.6, 0.2], tayGan: [16, 64], tayXa: [84, 50], vat: vat("may-anh", { x: -1, y: 0 }) }) },
    { t: 900, em: "standard", tt: tuThe({ nghieng: 3, nhin: [1.2, 0.6], tayGan: [16, 66], tayXa: [85, 54], vat: vat("may-anh", { x: -1, y: 0 }) }) },
  ],
};

// A money moment (M3). The strike lands at 300 + 130 ms: the same three
// beats as the page's big stamp (`NHIP_DAU`), so seal and puppet land together.
const dongDau: TietMuc = {
  id: "dong-dau",
  ms: 1000,
  khungTinh: 5,
  moTa: "Nếp đóng dấu đã ghi sổ",
  vat: "con-dau",
  nhip: [{ t: 430, kieu: "cham" }],
  khoa: [
    { t: 0, tt: tuThe({ tayXa: [84, 66], vat: vat("con-dau") }) },
    { t: 300, em: "standard", tt: tuThe({ nghieng: -3, ha: -0.5, nhin: [1.4, -0.4], tayGan: [14, 60], tayXa: [87, 44], vat: vat("con-dau", { xoay: -8 }) }) },
    { t: 430, em: "accelerate", tt: tuThe({ nghieng: 6, ha: 1.8, nhin: [1.6, 1.2], tayGan: [14, 64], tayXa: [84, 70], vat: vat("con-dau") }) },
    { t: 490, tt: tuThe({ nghieng: 6, ha: 2.2, nhin: [1.6, 1.2], tayGan: [14, 64], tayXa: [84, 71], vat: vat("con-dau") }) },
    { t: 720, em: "decelerate", tt: tuThe({ nghieng: 2, ha: 0, nhin: [1.2, 0.8], tayGan: [15, 66], tayXa: [86, 58], vat: vat("con-dau") }) },
    { t: 1000, em: "standard", tt: tuThe({ nghieng: 1, bieuCam: "nhuong", nhin: [1, 0.6], tayGan: [15, 67], tayXa: [85, 62], vat: vat("con-dau") }) },
  ],
};

// A money moment (M4): a bow of thanks, hands together in front.
const cuiCamOn: TietMuc = {
  id: "cui-cam-on",
  ms: 1000,
  khungTinh: 4,
  moTa: "Nếp cúi đầu cảm ơn",
  vat: null,
  nhip: [{ t: 560, kieu: "xong" }],
  khoa: [
    { t: 0, tt: tuThe({ bieuCam: "nhuong" }) },
    { t: 260, em: "standard", tt: tuThe({ bieuCam: "nhuong", nhin: [0.8, 0.6], tayGan: [40, 71], tayXa: [55, 71], uonTayGan: -1, uonTayXa: -1 }) },
    { t: 560, em: "decelerate", tt: tuThe({ xoay: 13, ha: 2, bieuCam: "nhuong", nhin: [1.2, 1.4], tayGan: [44, 74], tayXa: [58, 74], uonTayGan: -1, uonTayXa: -1 }) },
    { t: 760, tt: tuThe({ xoay: 13, ha: 2, bieuCam: "nhuong", nhin: [1.2, 1.4], tayGan: [44, 74], tayXa: [58, 74], uonTayGan: -1, uonTayXa: -1 }) },
    { t: 1000, em: "standard", tt: tuThe({ xoay: 0, ha: 0, bieuCam: "nhuong", nhin: [0.8, 0.4], tayGan: [40, 71], tayXa: [55, 71], uonTayGan: -1, uonTayXa: -1 }) },
  ],
};

const buocDi: TietMuc = {
  id: "buoc-di",
  ms: 1200,
  khungTinh: 1,
  moTa: "Nếp vẫy tay rồi lên đường",
  vat: null,
  nhip: [],
  khoa: [
    { t: 0, tt: tuThe({ bieuCam: "hao-hung" }) },
    { t: 200, em: "standard", tt: tuThe({ bieuCam: "hao-hung", nhin: [1.4, -0.4], tayGan: [14, 62], tayXa: [84, 32] }) },
    { t: 360, em: "standard", tt: tuThe({ bieuCam: "hao-hung", nhin: [1.4, -0.4], tayGan: [14, 62], tayXa: [90, 38] }) },
    { t: 520, em: "standard", tt: tuThe({ bieuCam: "hao-hung", nhin: [1.4, -0.4], tayGan: [14, 62], tayXa: [84, 32] }) },
    { t: 700, em: "standard", tt: tuThe({ dx: 8, nghieng: 9, bieuCam: "hao-hung", nhin: [1.6, -0.2], ...BUOC_A, tayGan: [30, 64], tayXa: [86, 40] }) },
    { t: 880, em: "standard", tt: tuThe({ dx: 22, nghieng: 9, bieuCam: "hao-hung", nhin: [1.6, -0.2], ...BUOC_B, tayGan: [16, 70], tayXa: [86, 40] }) },
    { t: 1040, em: "standard", tt: tuThe({ dx: 36, nghieng: 9, bieuCam: "hao-hung", nhin: [1.6, -0.2], ...BUOC_A, tayGan: [30, 64], tayXa: [86, 40] }) },
    { t: 1200, em: "standard", tt: tuThe({ dx: 50, nghieng: 9, bieuCam: "hao-hung", nhin: [1.6, -0.2], ...BUOC_B, tayGan: [16, 70], tayXa: [86, 40] }) },
  ],
};

// A sheet held at the chest is folded in half, then sent: the one thing
// allowed to fly, once (ADR-0037 D11).
const gapThu: TietMuc = {
  id: "gap-thu",
  ms: 1300,
  khungTinh: 3,
  moTa: "Nếp gấp tờ giấy rồi gửi đi",
  vat: "to-thu",
  nhip: [
    { t: 420, kieu: "chon" },
    { t: 900, kieu: "xong" },
  ],
  khoa: [
    { t: 0, tt: tuThe({ bieuCam: "giu-kin", nhin: [0.6, 1.4], tayGan: [41, 70], tayXa: [58, 69], uonTayGan: -1, uonTayXa: -1, vat: vat("to-thu", { gan: "giua", y: -1 }) }) },
    { t: 420, em: "standard", tt: tuThe({ bieuCam: "giu-kin", nhin: [0.6, 1.4], tayGan: [41, 70], tayXa: [58, 69], uonTayGan: -1, uonTayXa: -1, vat: vat("to-thu", { gan: "giua", y: -1, gap: 1 }) }) },
    // The far hand takes the folded sheet where it is: same place, new grip.
    { t: 440, tt: tuThe({ bieuCam: "giu-kin", nhin: [0.6, 1.4], tayGan: [41, 70], tayXa: [58, 69], uonTayGan: -1, uonTayXa: -1, vat: vat("to-thu", { gan: "tay-xa", x: -8.5, y: -0.5, gap: 1 }) }) },
    { t: 700, em: "standard", tt: tuThe({ nghieng: -2, bieuCam: "nhuong", nhin: [1.2, 0], tayGan: [16, 64], tayXa: [80, 56], vat: vat("to-thu", { gan: "tay-xa", x: 3, y: -4, gap: 1, xoay: -10 }) }) },
    // Let go at the top of the throw: the free sheet starts at the hand (90, 42) plus its grip (3, -4).
    { t: 900, em: "accelerate", tt: tuThe({ nghieng: 5, bieuCam: "nhuong", nhin: [1.6, -0.8], tayGan: [14, 60], tayXa: [90, 42], vat: vat("to-thu", { gan: "tu-do", x: 93, y: 38, gap: 1, xoay: -20 }) }) },
    { t: 1300, em: "decelerate", tt: tuThe({ nghieng: 3, bieuCam: "nhuong", nhin: [1.6, -1.4], tayGan: [15, 64], tayXa: [88, 46], vat: vat("to-thu", { gan: "tu-do", x: 130, y: 4, gap: 1, xoay: -40, hien: 0 }) }) },
  ],
};

const nhay: TietMuc = {
  id: "nhay",
  ms: 800,
  khungTinh: 4,
  moTa: "Nếp nhảy lên vui mừng",
  vat: null,
  nhip: [{ t: 560, kieu: "cham" }],
  khoa: [
    { t: 0, tt: tuThe({ bieuCam: "hao-hung" }) },
    { t: 160, em: "standard", tt: tuThe({ ha: 3, bieuCam: "hao-hung", nhin: [0.2, 0.6], tayGan: [16, 72], tayXa: [82, 72] }) },
    { t: 360, em: "decelerate", tt: tuThe({ dy: -12, bieuCam: "hao-hung", nhin: [0.2, -0.9], chanGan: [33, 87], chanXa: [62, 87], tayGan: [8, 36], tayXa: [86, 32] }) },
    { t: 560, em: "accelerate", tt: tuThe({ ha: 2.4, bieuCam: "hao-hung", nhin: [0.2, -0.4], tayGan: [10, 50], tayXa: [88, 46] }) },
    { t: 800, em: "decelerate", tt: tuThe({ bieuCam: "hao-hung", nhin: [0, -0.5], tayGan: [11, 40], tayXa: [86, 34] }) },
  ],
};

const nangTem: TietMuc = {
  id: "nang-tem",
  ms: 1000,
  khungTinh: 4,
  moTa: "Nếp giơ con tem mới lên",
  vat: "tem",
  nhip: [{ t: 320, kieu: "xong" }],
  khoa: [
    { t: 0, tt: tuThe({ bieuCam: "hao-hung", tayXa: [84, 66], vat: vat("tem", { y: 2 }) }) },
    { t: 320, em: "decelerate", tt: tuThe({ dy: -2, bieuCam: "hao-hung", nhin: [1.5, -1.4], tayGan: [12, 42], tayXa: [84, 34], vat: vat("tem", { y: 2 }) }) },
    { t: 520, em: "decelerate", tt: tuThe({ dy: -5, bieuCam: "hao-hung", nhin: [1.5, -1.6], tayGan: [11, 38], tayXa: [84, 31], vat: vat("tem", { y: 2, xoay: 6 }) }) },
    { t: 720, em: "accelerate", tt: tuThe({ dy: 0, ha: 1.2, bieuCam: "hao-hung", nhin: [1.5, -1.4], tayGan: [12, 42], tayXa: [84, 34], vat: vat("tem", { y: 2 }) }) },
    { t: 1000, em: "decelerate", tt: tuThe({ bieuCam: "hao-hung", nhin: [1.4, -1.2], tayGan: [12, 42], tayXa: [84, 34], vat: vat("tem", { y: 2 }) }) },
  ],
};

export const TIET_MUC: Readonly<Record<TietMucId, TietMuc>> = Object.freeze({
  "buoc-vao": buocVao,
  "keo-tab": keoTab,
  "cam-may": camMay,
  "dong-dau": dongDau,
  "cui-cam-on": cuiCamOn,
  "buoc-di": buocDi,
  "gap-thu": gapThu,
  nhay,
  "nang-tem": nangTem,
});

/** The faces allowed at a money moment (ADR-0037 D5). */
export const MAT_KHI_TIEN: readonly BieuCamNep[] = ["binh-than", "nhuong"];

// --- sampling (worklets) ------------------------------------------------------

/** cubic-bezier(x1, y1, x2, y2) at x = t, solved by bisection (never overshoots). */
export function emBezier(x1: number, y1: number, x2: number, y2: number, t: number): number {
  "worklet";
  if (t <= 0) return 0;
  if (t >= 1) return 1;
  let lo = 0;
  let hi = 1;
  for (let i = 0; i < 24; i += 1) {
    const s = (lo + hi) / 2;
    const u = 1 - s;
    const x = 3 * u * u * s * x1 + 3 * u * s * s * x2 + s * s * s;
    if (x < t) lo = s;
    else hi = s;
  }
  const s = (lo + hi) / 2;
  const u = 1 - s;
  return 3 * u * u * s * y1 + 3 * u * s * s * y2 + s * s * s;
}

function tron2(a: readonly [number, number], b: readonly [number, number], p: number): [number, number] {
  "worklet";
  return [a[0] + (b[0] - a[0]) * p, a[1] + (b[1] - a[1]) * p];
}

function tronSo(a: number, b: number, p: number): number {
  "worklet";
  return a + (b - a) * p;
}

/**
 * The pose of a performance at `t` ms. Numbers ease from one key to the next;
 * the face, the bends and which prop is held switch at the start of the
 * segment (a paper face is swapped, not morphed); a held prop moving to a
 * different hand eases its offset only.
 */
export function khopTai(tm: TietMuc, t: number): TuTheRoi {
  "worklet";
  const k = tm.khoa;
  if (t <= k[0].t) return k[0].tt;
  const cuoi = k[k.length - 1];
  if (t >= cuoi.t) return cuoi.tt;
  let i = 0;
  while (i < k.length - 2 && t >= k[i + 1].t) i += 1;
  const a = k[i];
  const b = k[i + 1];
  const tho = (t - a.t) / Math.max(1, b.t - a.t);
  const em = EASING[b.em ?? "standard"];
  const p = emBezier(em[0], em[1], em[2], em[3], tho);
  const va = a.tt.vat;
  const vb = b.tt.vat;
  let vat: ViTriVat | null = va;
  if (va && vb && va.id === vb.id) {
    // Where it is held switches at the start of a segment like the face does;
    // a prop only glides between two keys that hold it the same way. A
    // release is authored so the free key starts exactly where the hand had
    // it, which keeps the throw continuous.
    const cung = va.gan === vb.gan;
    vat = {
      id: va.id,
      gan: va.gan,
      x: cung ? tronSo(va.x, vb.x, p) : va.x,
      y: cung ? tronSo(va.y, vb.y, p) : va.y,
      xoay: tronSo(va.xoay, vb.xoay, p),
      gap: tronSo(va.gap, vb.gap, p),
      hien: tronSo(va.hien, vb.hien, p),
    };
  }
  return {
    dx: tronSo(a.tt.dx, b.tt.dx, p),
    dy: tronSo(a.tt.dy, b.tt.dy, p),
    ha: tronSo(a.tt.ha, b.tt.ha, p),
    nghieng: tronSo(a.tt.nghieng, b.tt.nghieng, p),
    xoay: tronSo(a.tt.xoay, b.tt.xoay, p),
    nhin: tron2(a.tt.nhin, b.tt.nhin, p),
    bieuCam: a.tt.bieuCam,
    tayGan: tron2(a.tt.tayGan, b.tt.tayGan, p),
    tayXa: tron2(a.tt.tayXa, b.tt.tayXa, p),
    chanGan: tron2(a.tt.chanGan, b.tt.chanGan, p),
    chanXa: tron2(a.tt.chanXa, b.tt.chanXa, p),
    uonTayGan: a.tt.uonTayGan,
    uonTayXa: a.tt.uonTayXa,
    uonGoiGan: a.tt.uonGoiGan,
    uonGoiXa: a.tt.uonGoiXa,
    vat,
  };
}

/** The pause a joined performance takes between two acts (the pose eases across it). */
export const NOI_TIET_MUC_MS = 150;

/**
 * Several performances as one clock (a moment may play two: M6 pulls the tab,
 * then jumps). Each act after the first starts `NOI_TIET_MUC_MS` after the one
 * before ends, and a prop still in hand at the join fades out across it rather
 * than vanishing. The still frame is the last act's.
 */
export function noiTietMuc(ds: readonly TietMuc[]): TietMuc {
  if (ds.length === 1) return ds[0];
  const khoa: KhoaTietMuc[] = [];
  const nhip: { t: number; kieu: NhipRung }[] = [];
  let lech = 0;
  let khungTinh = 0;
  ds.forEach((tm, i) => {
    const truoc = khoa[khoa.length - 1];
    tm.khoa.forEach((k, j) => {
      let tt = k.tt;
      if (j === 0 && truoc?.tt.vat && !tt.vat) tt = { ...tt, vat: { ...truoc.tt.vat, hien: 0 } };
      khoa.push({ t: k.t + lech, tt, em: j === 0 && i > 0 ? "standard" : k.em });
    });
    for (const n of tm.nhip) nhip.push({ t: n.t + lech, kieu: n.kieu });
    khungTinh = khoa.length - tm.khoa.length + tm.khungTinh;
    lech += tm.ms + NOI_TIET_MUC_MS;
  });
  const ms = khoa[khoa.length - 1].t;
  return {
    id: ds[ds.length - 1].id,
    ms,
    khoa,
    nhip,
    khungTinh,
    moTa: ds.map((tm) => tm.moTa).join(", "),
    vat: ds.find((tm) => tm.vat)?.vat ?? null,
  };
}

/** Every prop any key of a performance holds: what a renderer has to mount. */
export function vatCuaTietMuc(tm: TietMuc): VatCam[] {
  const ra = new Set<VatCam>();
  for (const k of tm.khoa) if (k.tt.vat) ra.add(k.tt.vat.id);
  return [...ra];
}

/** Every performance fits the budget; checked here once and by the gate. */
export function vuaNganSach(tm: TietMuc): boolean {
  return tm.ms <= NGAN_SACH_SAN_KHAU.dien && tm.khoa[tm.khoa.length - 1].t === tm.ms && tm.khoa[0].t === 0;
}
