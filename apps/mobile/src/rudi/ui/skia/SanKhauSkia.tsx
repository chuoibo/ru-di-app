/**
 * The paper stage drawn with Skia (ADR-0037 D10): one canvas, one group per
 * layer folding in 3D about its hinge line, far layers first -- the same angles
 * as the SVG renderer, from the same `gocBatTang`, read off the same shared
 * value, so a swap between the two never restarts a pop-up.
 *
 * What Skia adds over the SVG twin, all of it tied to the fold:
 *   - the scene's light: by day a warm pool under the scene's one coral light,
 *     an ellipse inscribed in the stage so it fades out before any edge (a
 *     circle clipped by the canvas read as a pale box); at night a small halo
 *     screened around the light itself, the lamp glowing -- a wide warm wash
 *     over the indigo cloth only read as a grey smudge (dark capture 24/09).
 *     Either way it dims as the stage lies down;
 *   - a cut-paper shadow per standing layer: its whole silhouette, blurred and
 *     dropped down-right from the one light (ADR-0037 D2), shortening to
 *     nothing as the layer folds flat;
 *   - a soft contact shadow where each layer meets its hinge;
 *   - the paper's thickness, an offset copy of the fills.
 *
 * Only reachable through `luoiSkia(() => import("./skia/SanKhauSkia"))`.
 */
import { BlurMask, Canvas, Circle, Group, Oval, Path, RadialGradient, rect, vec } from "@shopify/react-native-skia";
import { useMemo } from "react";
import { useDerivedValue, type SharedValue } from "react-native-reanimated";

import type { SanKhau, TangSanKhau } from "../../art/san-khau";
import { bongTheoGoc, gocBatTang, lechThiSai } from "../../san-khau/dong-hoc";
import { mauSanKhau, phuMau, useRudiTheme, type RudiPalette } from "../../theme";
import { LopSkia, duongSkia, hinhBongLop } from "./VeSkia";

export interface SanKhauVeProps {
  san: SanKhau;
  width: number;
  height: number;
  mo: SharedValue<number>;
  thiSai?: SharedValue<number>;
}

const MEP = { dx: 0.9, dy: 1.3 };
/** The drop of a standing layer's shadow, in stage units, before the fold shortens it. */
const DO_BONG = { dx: 1.4, dy: 2.4, mo: 2.2 };

function TangSkia({
  tang,
  i,
  n,
  san,
  mo,
  thiSai,
  colors,
  bong,
  dark,
}: {
  tang: TangSanKhau;
  i: number;
  n: number;
  san: SanKhau;
  mo: SharedValue<number>;
  thiSai?: SharedValue<number>;
  colors: RudiPalette;
  bong: string;
  dark: boolean;
}) {
  const goc = useDerivedValue(() => (tang.dung ? gocBatTang(mo.value, i, n) : 0));
  const bienDoi = useDerivedValue(() => [
    { translateX: lechThiSai(thiSai?.value ?? 0, tang.sau) },
    { perspective: 700 },
    { rotateX: (goc.value * Math.PI) / 180 },
  ]);
  const dung = useDerivedValue(() => (tang.dung ? bongTheoGoc(goc.value) : 0));
  const doBongChan = useDerivedValue(() => dung.value * (dark ? 0.42 : 0.2));
  const doBongTang = useDerivedValue(() => dung.value * (dark ? 0.22 : 0.16));
  const dichBong = useDerivedValue(() => [{ translateX: DO_BONG.dx * dung.value }, { translateY: DO_BONG.dy * dung.value }]);
  const hinhBong = useMemo(() => (tang.dung && tang.cao > 0 ? hinhBongLop(tang.lop) : null), [tang]);
  const cx = san.khung.w / 2;
  return (
    <>
      {tang.dung && tang.cao > 0 ? (
        <Oval color={bong} opacity={doBongChan} rect={rect(cx - san.khung.w * 0.38 + 4, tang.nep - 1, san.khung.w * 0.76, 5)}>
          <BlurMask blur={3} style="normal" />
        </Oval>
      ) : null}
      <Group origin={vec(cx, tang.nep)} transform={bienDoi}>
        {hinhBong ? (
          <Group opacity={doBongTang} transform={dichBong}>
            <Path color={bong} path={hinhBong}>
              <BlurMask blur={DO_BONG.mo} style="normal" />
            </Path>
          </Group>
        ) : null}
        {tang.dung ? (
          <Group transform={[{ translateX: MEP.dx }, { translateY: MEP.dy }]}>
            {tang.lop
              .filter((l) => !l.net)
              .map((l, k) => (
                <Path color={colors.paperShade} key={k} path={duongSkia(l.d)} />
              ))}
          </Group>
        ) : null}
        <LopSkia colors={colors} lop={tang.lop} />
      </Group>
    </>
  );
}

/**
 * The light pool: centred between the light and the floor, pulled toward the
 * middle, with radii that stop at the nearer edges -- so the gradient reaches
 * zero inside the canvas and never draws the canvas's own rectangle.
 */
function hoSang(san: SanKhau) {
  const { w, h } = san.khung;
  const sang = san.nguonSang ?? { x: w * 0.3, y: h * 0.15 };
  const san0 = san.tang[0]?.nep ?? h;
  const cx = Math.min(w * 0.7, Math.max(w * 0.3, sang.x));
  const cy = Math.min(h * 0.75, Math.max(h * 0.25, (sang.y + san0) / 2));
  const rx = Math.min(cx, w - cx);
  const ry = Math.min(cy, h - cy);
  return { cx, cy, rx, ry };
}

/** The night halo: around the light itself, no wider than the nearest edge allows. */
function quangDen(san: SanKhau): { x: number; y: number; r: number } | null {
  if (!san.nguonSang) return null;
  const { w, h } = san.khung;
  const { x, y } = san.nguonSang;
  const r = Math.min(Math.min(w, h) * 0.2, x, w - x, y, h - y);
  return r > 4 ? { x, y, r } : null;
}

export default function SanKhauSkia({ san, width, height, mo, thiSai }: SanKhauVeProps) {
  const { colors, dark } = useRudiTheme();
  const tiLe = width / san.khung.w;
  const n = san.tang.length;
  const { bong, anhSang } = mauSanKhau(dark);
  const ho = useMemo(() => hoSang(san), [san]);
  const quang = useMemo(() => quangDen(san), [san]);
  // The light is the stage's: it is up when the stage is up.
  const doSang = useDerivedValue(() => Math.min(1, Math.max(0, mo.value)));
  return (
    <Canvas style={{ width, height }}>
      <Group transform={[{ scale: tiLe }]}>
        {dark ? null : (
          <Group opacity={doSang} transform={[{ translateX: ho.cx }, { translateY: ho.cy }, { scaleY: ho.ry / ho.rx }]}>
            <Circle c={vec(0, 0)} r={ho.rx}>
              <RadialGradient c={vec(0, 0)} colors={[phuMau(anhSang, 0.55), phuMau(anhSang, 0)]} r={ho.rx} />
            </Circle>
          </Group>
        )}
        {dark && quang ? (
          <Group blendMode="screen" opacity={doSang}>
            <Circle c={vec(quang.x, quang.y)} r={quang.r}>
              <RadialGradient c={vec(quang.x, quang.y)} colors={[phuMau(anhSang, 0.55), phuMau(anhSang, 0.18), phuMau(anhSang, 0)]} positions={[0, 0.35, 1]} r={quang.r} />
            </Circle>
          </Group>
        ) : null}
        {san.nen.length > 0 ? <LopSkia colors={colors} lop={san.nen} /> : null}
        {san.tang.map((tang, i) => (
          <TangSkia bong={bong} colors={colors} dark={dark} i={i} key={tang.id} mo={mo} n={n} san={san} tang={tang} thiSai={thiSai} />
        ))}
      </Group>
    </Canvas>
  );
}
