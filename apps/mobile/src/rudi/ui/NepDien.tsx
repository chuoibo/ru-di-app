/**
 * Nếp performing one of its eight moments, in a slot of its own
 * (ADR-0037 D5).
 *
 * The rules this component keeps so no screen has to:
 *   - it takes room in the layout (128 / 112 / 88 dp, none on a short
 *     window) and is never laid over content;
 *   - it plays once per event, the still frame after that, the still frame
 *     only under Reduce Motion, and nothing at all on an error or a state the
 *     server has not confirmed (`useKhoanhKhac`);
 *   - a tap skips to the still frame; it never blocks an input;
 *   - while it is on screen the dock's Nếp tucks away (one Nếp at a time);
 *   - while playing it is a named skip button; afterwards one static image;
 *   - `EXPO_PUBLIC_QA_TAT_NEP_DIEN=1` turns it off for evidence runs that
 *     need the page without it.
 */
import { Pressable, View, useWindowDimensions, type StyleProp, type ViewStyle } from "react-native";

import type { GapNep } from "../art/nep";
import type { KhoanhKhacId } from "../khoanh-khac";
import { useNhuongChoNep } from "../nep/NepProvider";
import { kichThuocNepDien } from "../san-khau/kich-thuoc";
import { NepRoi } from "./NepRoi";
import { useAdaptiveLayout } from "./useAdaptiveLayout";
import { useKhoanhKhac } from "./useKhoanhKhac";

const TAT_NEP_DIEN_QA: boolean = process.env.EXPO_PUBLIC_QA_TAT_NEP_DIEN === "1";

export interface NepDienProps {
  khoanhKhac: KhoanhKhacId;
  /** The event this moment is about (an expense id, a plan id); null shows nothing. */
  suKien: string | null;
  /** The state is confirmed by the server; a moment never plays ahead of it. */
  hopLe?: boolean;
  coLoi?: boolean;
  /** A fixed side (dp) instead of the layout's. */
  co?: number;
  gap?: GapNep;
  style?: StyleProp<ViewStyle>;
  testID?: string;
}

export function NepDien({ khoanhKhac, suKien, hopLe = true, coLoi = false, co, gap = "trang", style, testID }: NepDienProps) {
  const layout = useAdaptiveLayout();
  const { fontScale } = useWindowDimensions();
  const canh = co ?? kichThuocNepDien({ sizeClass: layout.sizeClass, heightClass: layout.heightClass, fontScale });
  const kk = useKhoanhKhac(khoanhKhac, TAT_NEP_DIEN_QA ? null : suKien, { hopLe, coLoi });
  const hien = kk.tm !== null && canh > 0;
  useNhuongChoNep(hien);
  if (!hien || !kk.tm) return null;
  const hinh = (
    <View accessibilityLabel={kk.tm.moTa} accessibilityRole="image" accessible={!kk.dangChay} aria-hidden={kk.dangChay} style={{ width: canh, height: canh }}>
      <NepRoi gap={gap} t={kk.t} tm={kk.tm} width={canh} />
    </View>
  );
  if (!kk.dangChay) return <View style={[{ width: canh, height: canh }, style]} testID={testID}>{hinh}</View>;
  return (
    <Pressable accessibilityRole="button" accessibilityLabel={`${kk.tm.moTa}. Bỏ qua chuyển động của Nếp`} onPress={kk.boQua} style={[{ width: canh, height: canh }, style]} testID={testID}>
      {hinh}
    </Pressable>
  );
}
