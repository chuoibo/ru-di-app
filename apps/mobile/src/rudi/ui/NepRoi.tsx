/**
 * The paper puppet on screen: Skia when it can draw (every part moved per
 * frame on the UI thread), the SVG stop-motion otherwise, both reading the
 * same clock so a hand-over mid-performance continues (ADR-0037 D10). The
 * 96dp reading (brow, finer ink) from 72dp up, the compact one below.
 */
import type { StyleProp, ViewStyle } from "react-native";
import type { SharedValue } from "react-native-reanimated";

import type { GapNep } from "../art/nep";
import type { TietMuc } from "../art/nep-dien";
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
