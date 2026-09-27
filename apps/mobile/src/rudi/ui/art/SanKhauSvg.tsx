/**
 * The paper stage drawn with react-native-svg: the fallback renderer, and the
 * frame shown while Skia loads (ADR-0037 D10).
 *
 * Each layer is its own absolutely placed view holding an `Svg` of the whole
 * stage frame, so the layer can fold in 3D around its hinge line with a plain
 * view transform (`perspective` + `rotateX`, origin on the hinge) -- the same
 * angle, from the same `gocBatTang`, that the Skia renderer applies. Standing
 * layers get the two marks of cut paper: a thickness edge (the layer's fills
 * offset a hair down-right in the paper's shade colour) and a soft shadow at
 * the hinge that fades as the layer lies down.
 */
import { StyleSheet, View } from "react-native";
import Animated, { useAnimatedStyle, useDerivedValue, type SharedValue } from "react-native-reanimated";
import Svg, { Ellipse, G, Path } from "react-native-svg";

import type { SanKhau, TangSanKhau } from "../../art/san-khau";
import { bongTheoGoc, gocBatTang, lechThiSai } from "../../san-khau/dong-hoc";
import { mauSanKhau, useRudiTheme, type RudiPalette } from "../../theme";
import { mauLop } from "./VeLop";

export interface SanKhauVeProps {
  san: SanKhau;
  width: number;
  height: number;
  /** 0 = every layer flat in the page, 1 = the stage fully up. */
  mo: SharedValue<number>;
  /** Parallax input in -1..1 (scroll or pan); 0 when absent. */
  thiSai?: SharedValue<number>;
}

const MEP = { dx: 0.9, dy: 1.3 };

function LopTang({ tang, colors, mep }: { tang: TangSanKhau; colors: RudiPalette; mep: string }) {
  return (
    <>
      {tang.dung
        ? tang.lop
            .filter((l) => !l.net)
            .map((l, i) => <Path d={l.d} fill={mep} key={`m${i}`} transform={`translate(${MEP.dx} ${MEP.dy})`} />)
        : null}
      {tang.lop.map((l, i) =>
        l.net && l.net > 0 ? (
          <Path d={l.d} fill="none" key={i} stroke={mauLop(colors, l.mau)} strokeLinecap="round" strokeLinejoin="round" strokeWidth={l.net} />
        ) : (
          <Path d={l.d} fill={mauLop(colors, l.mau)} key={i} />
        ),
      )}
    </>
  );
}

function TangSvg({
  tang,
  i,
  n,
  san,
  width,
  height,
  mo,
  thiSai,
}: {
  tang: TangSanKhau;
  i: number;
  n: number;
  san: SanKhau;
  width: number;
  height: number;
  mo: SharedValue<number>;
  thiSai?: SharedValue<number>;
}) {
  const { colors, dark } = useRudiTheme();
  const tiLe = width / san.khung.w;
  const nepY = tang.nep * tiLe;
  const goc = useDerivedValue(() => (tang.dung ? gocBatTang(mo.value, i, n) : 0));
  const kieu = useAnimatedStyle(() => ({
    transform: [
      { translateX: lechThiSai(thiSai?.value ?? 0, tang.sau) },
      { perspective: 700 },
      { rotateX: `${goc.value}deg` },
    ],
  }));
  const kieuBong = useAnimatedStyle(() => ({ opacity: tang.dung ? bongTheoGoc(goc.value) : 0 }));
  const bong = mauSanKhau(dark).bong;
  return (
    <>
      {tang.dung && tang.cao > 0 ? (
        <Animated.View pointerEvents="none" style={[StyleSheet.absoluteFill, kieuBong]}>
          <Svg height={height} viewBox={`0 0 ${san.khung.w} ${san.khung.h}`} width={width}>
            <Ellipse cx={san.khung.w / 2 + 4} cy={tang.nep + 1.5} fill={bong} opacity={dark ? 0.32 : 0.14} rx={san.khung.w * 0.38} ry={2.6} />
          </Svg>
        </Animated.View>
      ) : null}
      <Animated.View pointerEvents="none" style={[StyleSheet.absoluteFill, { transformOrigin: `${width / 2}px ${nepY}px` }, kieu]}>
        <Svg height={height} viewBox={`0 0 ${san.khung.w} ${san.khung.h}`} width={width}>
          <G>
            <LopTang colors={colors} mep={colors.paperShade} tang={tang} />
          </G>
        </Svg>
      </Animated.View>
    </>
  );
}

export function SanKhauSvg({ san, width, height, mo, thiSai }: SanKhauVeProps) {
  const { colors } = useRudiTheme();
  const n = san.tang.length;
  return (
    <View pointerEvents="none" style={{ width, height }}>
      {san.nen.length > 0 ? (
        <Svg height={height} style={StyleSheet.absoluteFill} viewBox={`0 0 ${san.khung.w} ${san.khung.h}`} width={width}>
          {san.nen.map((l, i) =>
            l.net && l.net > 0 ? (
              <Path d={l.d} fill="none" key={i} stroke={mauLop(colors, l.mau)} strokeLinecap="round" strokeWidth={l.net} />
            ) : (
              <Path d={l.d} fill={mauLop(colors, l.mau)} key={i} />
            ),
          )}
        </Svg>
      ) : null}
      {san.tang.map((tang, i) => (
        <TangSvg height={height} i={i} key={tang.id} mo={mo} n={n} san={san} tang={tang} thiSai={thiSai} width={width} />
      ))}
    </View>
  );
}
