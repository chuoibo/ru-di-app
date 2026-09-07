/**
 * Geometry builders for the art layer ("nét": one stroke of the pen).
 *
 * Deliberately dumb, for three reasons that each cost a red board once:
 *
 * - react-native-svg hands `d` to Java's `PathParser` at MOUNT time. One token
 *   it does not expect (an `L` with no numbers, an exponent, a `NaN`) throws
 *   inside Fabric's mount loop and the app dies on its first frame, before any
 *   screen is visible; tsc and the web export are both blind to it. So every
 *   builder here returns explicit `M` / `L` / `C` / `Z` commands with a fixed
 *   arity and plain decimals, and `tests/art-duong.test.mjs` parses every
 *   shape the way the Java side does. Quadratics and arcs are converted to
 *   cubics here, never emitted.
 * - Paths are built from numbers at runtime rather than written as literals:
 *   the repo guard reads nine digits with single spaces between them as an
 *   account number, and a hand-typed path is exactly that shape.
 * - Nothing here knows a colour. A layer names a ROLE (`MauVe`) and the
 *   component resolves it from the theme, so one drawing reads on light and
 *   dark ground alike and `tests/rudi-khong-hex.test.mjs` stays green.
 */

/**
 * Palette roles of a layer, resolved by the component:
 * `giay` paper (card) · `bong` the shaded face of a fold (line) · `muc` ink ·
 * `gap` the coral fold corner (accent) · `mo` a pale wash (accentSoft) ·
 * `split` money teal · `ai` violet.
 */
export type MauVe = "giay" | "bong" | "muc" | "gap" | "mo" | "split" | "ai";
export const MAU_VE: readonly MauVe[] = ["giay", "bong", "muc", "gap", "mo", "split", "ai"];

/** One path. `net` > 0 draws it as a stroke of that width (round caps), otherwise it is filled. */
export interface LopVe {
  d: string;
  mau: MauVe;
  net?: number;
}

export type Diem = readonly [number, number];

/** Plain decimal, never exponent notation, never `-0`. */
export function so(n: number): string {
  const s = n.toFixed(2).replace(/\.?0+$/, "");
  return s === "-0" ? "0" : s;
}

/** A path from its tokens, so numbers are formatted in exactly one place. */
export function duong(...phan: (string | number)[]): string {
  return phan.map((p) => (typeof p === "number" ? so(p) : p)).join(" ");
}

/** A closed polygon through the given points (a fill). */
export function daGiac(diem: readonly Diem[]): string {
  const [dau, ...con] = diem;
  return [duong("M", dau[0], dau[1]), ...con.map(([x, y]) => duong("L", x, y)), "Z"].join(" ");
}

/** An open polyline through the given points (a stroke). */
export function netGay(diem: readonly Diem[]): string {
  const [dau, ...con] = diem;
  return [duong("M", dau[0], dau[1]), ...con.map(([x, y]) => duong("L", x, y))].join(" ");
}

/** One cubic segment as an open path. */
export function cong(p0: Diem, c1: Diem, c2: Diem, p1: Diem): string {
  return duong("M", p0[0], p0[1], "C", c1[0], c1[1], c2[0], c2[1], p1[0], p1[1]);
}

/** A chain of cubic segments as an open path: start, then [c1, c2, end] per segment. */
export function netCong(dau: Diem, doan: readonly (readonly [Diem, Diem, Diem])[]): string {
  return [duong("M", dau[0], dau[1]), ...doan.map(([c1, c2, p]) => duong("C", c1[0], c1[1], c2[0], c2[1], p[0], p[1]))].join(" ");
}

/**
 * A quadratic Bézier written as the cubic it exactly equals
 * (c1 = p0 + 2/3 (q - p0), c2 = p1 + 2/3 (q - p1)); the parser has no `Q`.
 */
export function qCong(p0: Diem, q: Diem, p1: Diem): string {
  const c1: Diem = [p0[0] + (2 / 3) * (q[0] - p0[0]), p0[1] + (2 / 3) * (q[1] - p0[1])];
  const c2: Diem = [p1[0] + (2 / 3) * (q[0] - p1[0]), p1[1] + (2 / 3) * (q[1] - p1[1])];
  return cong(p0, c1, c2, p1);
}

const K = 0.5523;

/** A circle as four cubics (a fill). */
export function tron(cx: number, cy: number, r: number): string {
  return bau(cx, cy, r, r);
}

/** An ellipse as four cubics (a fill). */
export function bau(cx: number, cy: number, rx: number, ry: number): string {
  const kx = rx * K, ky = ry * K;
  return [
    duong("M", cx + rx, cy),
    duong("C", cx + rx, cy + ky, cx + kx, cy + ry, cx, cy + ry),
    duong("C", cx - kx, cy + ry, cx - rx, cy + ky, cx - rx, cy),
    duong("C", cx - rx, cy - ky, cx - kx, cy - ry, cx, cy - ry),
    duong("C", cx + kx, cy - ry, cx + rx, cy - ky, cx + rx, cy),
    "Z",
  ].join(" ");
}

/**
 * The `C` commands of a circular arc from angle `tu` to `den` (radians, y down,
 * so a growing angle turns clockwise on screen), split into quarter turns or
 * less so each cubic stays within a hair of the circle. No leading `M`.
 */
function doanCung(cx: number, cy: number, r: number, tu: number, den: number): string[] {
  const quet = den - tu;
  const n = Math.max(1, Math.ceil(Math.abs(quet) / (Math.PI / 2) - 1e-9));
  const buoc = quet / n;
  const k = (4 / 3) * Math.tan(buoc / 4);
  const ra: string[] = [];
  for (let i = 0; i < n; i += 1) {
    const a0 = tu + i * buoc;
    const a1 = a0 + buoc;
    const x0 = cx + r * Math.cos(a0), y0 = cy + r * Math.sin(a0);
    const x1 = cx + r * Math.cos(a1), y1 = cy + r * Math.sin(a1);
    ra.push(
      duong(
        "C",
        x0 - k * r * Math.sin(a0),
        y0 + k * r * Math.cos(a0),
        x1 + k * r * Math.sin(a1),
        y1 - k * r * Math.cos(a1),
        x1,
        y1,
      ),
    );
  }
  return ra;
}

/** An open circular arc (a stroke). Angles in radians, y down. */
export function cungTron(cx: number, cy: number, r: number, tu: number, den: number): string {
  return [duong("M", cx + r * Math.cos(tu), cy + r * Math.sin(tu)), ...doanCung(cx, cy, r, tu, den)].join(" ");
}

/** A closed wedge of a circle: centre, arc, back to centre (a fill). */
export function quat(cx: number, cy: number, r: number, tu: number, den: number): string {
  return [duong("M", cx, cy), duong("L", cx + r * Math.cos(tu), cy + r * Math.sin(tu)), ...doanCung(cx, cy, r, tu, den), "Z"].join(" ");
}

/** A rounded rectangle (a fill); `r` is clamped to half the shorter side. */
export function khungBo(x: number, y: number, w: number, h: number, r: number): string {
  const rr = Math.min(r, w / 2, h / 2);
  const k = rr * K;
  return [
    duong("M", x + rr, y),
    duong("L", x + w - rr, y),
    duong("C", x + w - rr + k, y, x + w, y + rr - k, x + w, y + rr),
    duong("L", x + w, y + h - rr),
    duong("C", x + w, y + h - rr + k, x + w - rr + k, y + h, x + w - rr, y + h),
    duong("L", x + rr, y + h),
    duong("C", x + rr - k, y + h, x, y + h - rr + k, x, y + h - rr),
    duong("L", x, y + rr),
    duong("C", x, y + rr - k, x + rr - k, y, x + rr, y),
    "Z",
  ].join(" ");
}

/**
 * A capsule of width `w` from one point to another (a fill): an ink limb or a
 * chopstick that must not depend on the renderer's line caps.
 */
export function vien(a: Diem, b: Diem, w: number): string {
  const dx = b[0] - a[0], dy = b[1] - a[1];
  const len = Math.hypot(dx, dy) || 1;
  const ux = dx / len, uy = dy / len;
  const nx = -uy, ny = ux;
  const h = w / 2;
  const gocN = Math.atan2(ny, nx);
  return [
    duong("M", a[0] + nx * h, a[1] + ny * h),
    duong("L", b[0] + nx * h, b[1] + ny * h),
    ...doanCung(b[0], b[1], h, gocN, gocN - Math.PI),
    duong("L", a[0] - nx * h, a[1] - ny * h),
    ...doanCung(a[0], a[1], h, gocN + Math.PI, gocN),
    "Z",
  ].join(" ");
}

/** A tapered stroke from width `w0` at `a` to width `w1` at `b`, square-ended (a fill). */
export function thon(a: Diem, b: Diem, w0: number, w1: number): string {
  const dx = b[0] - a[0], dy = b[1] - a[1];
  const len = Math.hypot(dx, dy) || 1;
  const nx = -dy / len, ny = dx / len;
  return daGiac([
    [a[0] + nx * (w0 / 2), a[1] + ny * (w0 / 2)],
    [b[0] + nx * (w1 / 2), b[1] + ny * (w1 / 2)],
    [b[0] - nx * (w1 / 2), b[1] - ny * (w1 / 2)],
    [a[0] - nx * (w0 / 2), a[1] - ny * (w0 / 2)],
  ]);
}

/** A drop: a circle of radius `r` about (cx, cy) drawn up to a point at `cy - 2r` (a fill). */
export function giot(cx: number, cy: number, r: number): string {
  const c = Math.cos(Math.PI / 6) * r, s = Math.sin(Math.PI / 6) * r;
  return [
    duong("M", cx, cy - 2 * r),
    duong("L", cx + c, cy - s),
    ...doanCung(cx, cy, r, -Math.PI / 6, (7 * Math.PI) / 6),
    "Z",
  ].join(" ");
}

/**
 * A point transform for composing one drawing inside another: offset, then
 * scale. Every builder takes finished points, so a figure is authored once in
 * its own frame and placed by mapping its points through this.
 */
export function bienDoi(x0: number, y0: number, tiLe = 1): (x: number, y: number) => Diem {
  return (x, y) => [x0 + x * tiLe, y0 + y * tiLe];
}
