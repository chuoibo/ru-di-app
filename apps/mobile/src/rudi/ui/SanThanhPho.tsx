import { useState } from "react";
import { StyleSheet, View } from "react-native";

import { KHUNG_THANH_PHO, sanKhauThanhPho } from "../art/thanh-pho";
import { SanKhau } from "./SanKhau";
import { useAdaptiveLayout } from "./useAdaptiveLayout";

/**
 * The city as a pop-up stage at the head of Khám phá › Địa điểm (ADR-0037
 * D1), composed as the owner's mockup draws it (finish review 02/10): on a
 * phone the scene runs edge to edge and rises under the place line above it,
 * its sky starting there instead of under a band of empty paper; on a tablet
 * it spans the reading column, the top eighth of its sky cut so it stays a
 * band above the search (no city's sun starts higher than 0.125 of the
 * frame: matTroi(144, 24) with r 9). The caller puts its place line before
 * this and gives that line `zIndex: 1`, so the line stays on top of the sky.
 */
export function SanThanhPho({ id, ten }: { id: string | null; ten: string }) {
  const dienThoai = useAdaptiveLayout().sizeClass === "compact";
  const [rong, setRong] = useState(0);
  const caoKhung = Math.round(((rong * KHUNG_THANH_PHO.h) / KHUNG_THANH_PHO.w) * 0.88);
  return (
    <View
      onLayout={(e) => setRong(Math.round(e.nativeEvent.layout.width))}
      style={[styles.san, dienThoai ? styles.tran : rong > 0 && [styles.bang, { height: caoKhung }]]}
      testID="san-thanh-pho"
    >
      {rong > 0 ? <SanKhau coMoTa key={id ?? ten} san={sanKhauThanhPho(id, ten)} width={dienThoai ? Math.min(rong, 480) : rong} /> : null}
    </View>
  );
}

const styles = StyleSheet.create({
  san: { alignItems: "center", alignSelf: "stretch" },
  // The screen's own gutter (`space.md`) given back so the stage meets both
  // edges, and the column's gap plus the scene's empty top, so its sky starts
  // under the place line.
  tran: { marginHorizontal: -16, marginTop: -32 },
  bang: { overflow: "hidden", justifyContent: "flex-end" },
});
