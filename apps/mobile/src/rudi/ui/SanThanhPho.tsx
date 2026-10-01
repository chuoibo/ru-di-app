import { useState } from "react";
import { StyleSheet, View } from "react-native";

import { NET_KY_HOA } from "../art/ky-hoa";
import { KHUNG_THANH_PHO, SAN_THANH_PHO, sanKhauThanhPho } from "../art/thanh-pho";
import { useRudiTheme } from "../theme";
import { SanKhau } from "./SanKhau";
import { useAdaptiveLayout } from "./useAdaptiveLayout";

/** The widest the city is drawn: past it the strokes thicken and the scene eats the first screen. */
const RONG_VE = 480;

/**
 * The city as a pop-up stage at the head of Khám phá › Địa điểm (ADR-0037
 * D1), composed as the owner's mockup draws it (finish review 02/10). It
 * rises under the place line above it, so its sky starts there instead of
 * under a band of empty paper; the caller puts that line before this and
 * gives it `zIndex: 1`, so the line stays on top of the sky.
 *
 * On a phone the scene runs edge to edge. On a tablet it is a full-width
 * band: the drawing stays at its own scale in the middle (scaled to the
 * column it doubled its strokes and pushed every place off the first screen)
 * and its ground line runs on to both edges of the column, in the same ink
 * and weight, so the scene sits on one horizon instead of floating in paper.
 */
export function SanThanhPho({ id, ten }: { id: string | null; ten: string }) {
  const dienThoai = useAdaptiveLayout().sizeClass === "compact";
  const { colors } = useRudiTheme();
  const [rong, setRong] = useState(0);
  const rongVe = Math.min(rong, RONG_VE);
  const tiLe = rongVe / KHUNG_THANH_PHO.w;
  const net = NET_KY_HOA.gan * tiLe;
  // The scene's ground line (`mat()` in art/thanh-pho.ts: x 4 .. w-4 at
  // SAN_THANH_PHO), carried past the drawing's own ends to the column's edges.
  const keo = (rong - rongVe) / 2 + 4 * tiLe;
  const chanTroi = { top: SAN_THANH_PHO * tiLe - net / 2, height: net, width: keo, backgroundColor: colors.ink };
  return (
    <View onLayout={(e) => setRong(Math.round(e.nativeEvent.layout.width))} style={[styles.san, dienThoai && styles.tran]} testID="san-thanh-pho">
      {rong > 0 ? (
        <>
          {dienThoai ? null : (
            <>
              <View pointerEvents="none" style={[styles.chanTroi, chanTroi, { left: 0 }]} />
              <View pointerEvents="none" style={[styles.chanTroi, chanTroi, { right: 0 }]} />
            </>
          )}
          <SanKhau coMoTa key={id ?? ten} san={sanKhauThanhPho(id, ten)} width={rongVe} />
        </>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  // The column's gap plus the scene's empty top: its sky starts under the place line.
  san: { alignItems: "center", alignSelf: "stretch", marginTop: -32 },
  // The screen's own gutter (`space.md`) given back so the stage meets both edges.
  tran: { marginHorizontal: -16 },
  chanTroi: { position: "absolute", borderRadius: 2 },
});
