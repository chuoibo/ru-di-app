/**
 * The paper puppet on screen: Skia when it can draw (every part moved per
 * frame on the UI thread), the SVG stop-motion otherwise, both reading the
 * same clock so a hand-over mid-performance continues (ADR-0037 D10). The
 * 96dp reading (brow, finer ink) from 72dp up, the compact one below.
 */
import { View, type StyleProp, type ViewStyle } from "react-native";
import type { SharedValue } from "react-native-reanimated";

import { KHUNG_NEP, type GapNep } from "../art/nep";
import type { TietMuc } from "../art/nep-dien";
import { tuTheRoi } from "../art/nep-roi";
import { VeLop } from "./art/VeLop";
import { useNhuongChoNep } from "../nep/NepProvider";
import { KhungSkia, luoiSkia } from "./KhungSkia";
import { NepRoiSvg } from "./art/NepRoiSvg";
import type { NepRoiVeProps } from "./skia/NepRoiSkia";

const NepRoiSkia = luoiSkia<NepRoiVeProps>(() => import("./skia/NepRoiSkia"));

export function NepRoi({
  tm,
  t,
  width,
  gap = "trang",
  style,
  testID,
}: {
  tm: TietMuc;
  t: SharedValue<number>;
  width: number;
  gap?: GapNep;
  style?: StyleProp<ViewStyle>;
  testID?: string;
}) {
  const props: NepRoiVeProps = { tm, t, width, chiTiet: width >= 72, gap };
  return <KhungSkia props={props} skia={NepRoiSkia} style={[{ width, height: width }, style]} svg={<NepRoiSvg {...props} />} testID={testID} />;
}

/**
 * One pose of the puppet, still: a key frame of a performance (by default its
 * still frame) drawn once as SVG. For a scene where Nếp stands by -- holding
 * the camera on an empty bill -- without a moment to play. Decoration, so a
 * screen reader skips it; and one Nếp at a time, so the dock's tucks away.
 */
export function NepTinh({ tm, khoa, width, gap = "trang", style }: { tm: TietMuc; khoa?: number; width: number; gap?: GapNep; style?: StyleProp<ViewStyle> }) {
  useNhuongChoNep(true);
  const lop = tuTheRoi(tm.khoa[khoa ?? tm.khungTinh].tt, { chiTiet: width >= 72, gap });
  return (
    <View accessibilityElementsHidden importantForAccessibility="no-hide-descendants" pointerEvents="none" style={style}>
      <VeLop height={width} khungH={KHUNG_NEP} khungW={KHUNG_NEP} lop={lop} width={width} />
    </View>
  );
}
