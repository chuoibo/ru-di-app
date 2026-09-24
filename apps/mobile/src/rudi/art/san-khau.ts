/**
 * The paper stage (ADR-0037 D1): a scene cut into layers that stand up out of
 * the page's fold, far layers first.
 *
 * A stage is data, not a drawing: each layer carries its geometry (`LopVe[]`
 * from the existing builders, never redrawn), its depth `sau` (0 far .. 3
 * near -- the stagger order and the parallax factor), the line it is hinged on
 * (`nep`, a y in the stage frame: points ON that line never move while the
 * layer folds) and its paper height `cao` (the shadow it casts when upright).
 * The renderers (`ui/SanKhau.tsx`: Skia, and SVG as the fallback) only read it.
 *
 * Existing art becomes stages without a single coordinate changing:
 *   - an empty-state scene (`canh.ts`) is its props and its figure, two layers
 *     hinged on the scene's floor;
 *   - a place sketch (`ky-hoa.ts`) already carries depth in its three stroke
 *     weights (far 1.7 · mid 2.4 · near 3.0), so it lifts into three layers by
 *     weight, each fill travelling with the stroke that follows it -- the order
 *     the sketch is built in (`fill(...)`, then its outline).
 *
 * Pure: node tests validate every stage (path grammar, bounds, one coral light,
 * depth order) exactly as the other art gates do.
 */
import { KHUNG_CANH, SAN_CANH, moTaCanh, tangCanh, type TuyChonCanh } from "./canh";
import { KHUNG_KY_HOA, NET_KY_HOA, SAN_KY_HOA, hinhKyHoa, moTaKyHoa, type TuyChonKyHoa } from "./ky-hoa";
import type { LopVe } from "./net";

export type DoSau = 0 | 1 | 2 | 3;
/** Paper height (ADR-0037 D2); mirrors `san-khau/token.ts` without importing JSON into the art layer. */
export type CaoTang = 0 | 1 | 2 | 3;

export interface TangSanKhau {
  id: string;
  /** 0 far .. 3 near: the stagger order and the parallax factor. */
  sau: DoSau;
  /** The hinge line (y in the stage frame). */
  nep: number;
  /** Stands up out of the page (a pop-up layer) or stays printed flat on it. */
  dung: boolean;
  /** Paper height when upright: 0 printed, 2 standing. */
  cao: CaoTang;
  lop: LopVe[];
}

export interface SanKhau {
  id: string;
  khung: { w: number; h: number };
  /** Printed on the page behind everything: never folds. */
  nen: LopVe[];
  /** Back to front. */
  tang: TangSanKhau[];
  /** The one coral light of the scene, if it has one: where the stage's light comes from. */
  nguonSang: { x: number; y: number } | null;
  /** One sentence for a screen reader, the same one the flat drawing had. */
  moTa: string;
}

/** An empty-state scene as a stage: props, then the figure, both hinged on the floor. */
export function sanKhauTuCanh(id: string, tuyChon: TuyChonCanh = {}): SanKhau {
  const { nen, nep } = tangCanh(id, tuyChon);
  const tang: TangSanKhau[] = [{ id: "dao-cu", sau: 1, nep: SAN_CANH, dung: true, cao: 2, lop: nen }];
  if (nep.length > 0) tang.push({ id: "nep", sau: 2, nep: SAN_CANH, dung: true, cao: 2, lop: nep });
  return { id: `canh:${id}`, khung: { ...KHUNG_CANH }, nen: [], tang, nguonSang: nguonSangCua([...nen, ...nep]), moTa: moTaCanh(id) };
}

/**
 * Split a layer list into depth bands by stroke weight: strokes at or under
 * `xa` are far, under `vua`+ε mid, the rest near; a fill joins the band of the
 * next stroke after it (a sketch is built fill-then-outline), or the last band
 * seen when no stroke follows.
 */
export function tachDoSau(lop: readonly LopVe[], net: { xa: number; vua: number }): [LopVe[], LopVe[], LopVe[]] {
  const bang: [LopVe[], LopVe[], LopVe[]] = [[], [], []];
  const bangCuaNet = (w: number): 0 | 1 | 2 => (w <= net.xa + 1e-6 ? 0 : w <= net.vua + 1e-6 ? 1 : 2);
  let cho: LopVe[] = [];
  let cuoi: 0 | 1 | 2 = 1;
  for (const l of lop) {
    if (l.net && l.net > 0) {
      const b = bangCuaNet(l.net);
      bang[b].push(...cho, l);
      cho = [];
      cuoi = b;
    } else {
      cho.push(l);
    }
  }
  if (cho.length > 0) bang[cuoi].push(...cho);
  return bang;
}

/** A place sketch as a three-layer stage hinged on its floor line. */
export function sanKhauKyHoa(loai: string | undefined, tags: readonly string[], tuyChon: TuyChonKyHoa = {}): SanKhau {
  const lop = hinhKyHoa(loai, tags, tuyChon);
  const [xa, vua, gan] = tachDoSau(lop, NET_KY_HOA);
  const tang: TangSanKhau[] = [];
  if (xa.length > 0) tang.push({ id: "xa", sau: 0, nep: SAN_KY_HOA, dung: true, cao: 2, lop: xa });
  if (vua.length > 0) tang.push({ id: "vua", sau: 1, nep: SAN_KY_HOA, dung: true, cao: 2, lop: vua });
  if (gan.length > 0) tang.push({ id: "gan", sau: 2, nep: SAN_KY_HOA, dung: true, cao: 2, lop: gan });
  return {
    id: `ky-hoa:${loai ?? "khac"}`,
    khung: { ...KHUNG_KY_HOA },
    nen: [],
    tang,
    nguonSang: nguonSangCua(lop),
    moTa: moTaKyHoa(loai, tags, tuyChon),
  };
}

/** Every layer of a stage, back to front, as one flat list: the frame it rests in. */
export function lopPhang(san: SanKhau): LopVe[] {
  return [...san.nen, ...san.tang.flatMap((t) => t.lop)];
}

/** The centre of the first coral fill: the scene's light source. */
export function nguonSangCua(lop: readonly LopVe[]): { x: number; y: number } | null {
  for (const l of lop) {
    if (l.mau !== "gap" || (l.net && l.net > 0)) continue;
    const so = l.d.split(/\s+/).filter((t) => t !== "" && !/^[A-Za-z]$/.test(t)).map(Number).filter((n) => !Number.isNaN(n));
    if (so.length < 2) continue;
    let sx = 0, sy = 0, dem = 0;
    for (let i = 0; i + 1 < so.length; i += 2) {
      sx += so[i];
      sy += so[i + 1];
      dem += 1;
    }
    return { x: sx / dem, y: sy / dem };
  }
  return null;
}

/**
 * What a well-formed stage looks like; an empty list means valid. The node
 * gate runs it on every stage the app builds.
 */
export function kiemSanKhau(san: SanKhau): string[] {
  const loi: string[] = [];
  if (san.tang.length === 0) loi.push("không có tầng nào");
  const ids = new Set<string>();
  let sauTruoc = -1;
  for (const t of san.tang) {
    if (ids.has(t.id)) loi.push(`tầng trùng id ${t.id}`);
    ids.add(t.id);
    if (t.sau < sauTruoc) loi.push(`tầng ${t.id} gần hơn mà đứng trước tầng xa hơn`);
    sauTruoc = t.sau;
    if (t.nep < 0 || t.nep > san.khung.h) loi.push(`đường gập của ${t.id} ra ngoài khung`);
    if (t.lop.length === 0) loi.push(`tầng ${t.id} rỗng`);
    if (t.dung && t.cao === 0) loi.push(`tầng ${t.id} đứng dựng mà không có độ cao giấy`);
  }
  if (san.moTa.trim().length < 8) loi.push("thiếu câu mô tả cho trình đọc màn hình");
  return loi;
}
