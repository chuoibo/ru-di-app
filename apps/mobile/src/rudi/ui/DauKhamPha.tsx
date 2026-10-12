import type { ReactNode } from "react";
import { Pressable, StyleSheet, Text as RNText, View, useWindowDimensions } from "react-native";

import { TABLIST, tabState } from "../../ui/a11y";
import { typography, useRudiTheme } from "../theme";
import { Text as NativeMetricText } from "./NativeFontMetrics";
import type { MucKhamPha } from "./thanh-tab";

/** How a Khám phá screen asks its route for the header, with its own right-hand control. */
export type DungDau = (phai?: ReactNode) => ReactNode;

const MUC: readonly { muc: MucKhamPha; nhan: string }[] = [
  { muc: "explore", nhan: "Địa điểm" },
  { muc: "community", nhan: "Cộng đồng" },
];

/**
 * Khám phá's header (owner's mockup, 01/10): its two sections as words at
 * heading size, the open one in ink over the strip's coral tape, the other
 * faint; one font for both, so nothing shifts when the section changes. It
 * only draws: the route file owns the navigation, because the guide's
 * extractor (`tools/rut-huong-dan.mjs`) reads a route's exits there. The
 * two labels wrap inside the available row, so both sections stay visible
 * at large native text sizes. `phai` stays outside the tablist, which may
 * own tabs only. Below 360dp non-inline actions rise above the words.
 */
export function DauKhamPha({ muc, onDoiMuc, phai, nativeMetrics = false, inlineActions = false }: { muc: MucKhamPha; onDoiMuc: (muc: MucKhamPha) => void; phai?: ReactNode; nativeMetrics?: boolean; inlineActions?: boolean }) {
  const { colors } = useRudiTheme();
  const CopyText = nativeMetrics ? NativeMetricText : RNText;
  // The words stay first in reading order; only the controls' place on screen changes.
  // A single overflow target leaves the section words on the same row.
  const hep = useWindowDimensions().width < 360 && phai !== undefined && !inlineActions;
  return (
    <View style={[styles.hang, hep && styles.hangHep]} testID="dau-kham-pha">
      <View style={[styles.mucTrongHang, inlineActions && styles.mucInline]}>
        <View {...TABLIST} style={styles.danhSach}>
          {MUC.map((m) => {
            const chon = m.muc === muc;
            return (
              <Pressable
                key={m.muc}
                {...tabState(chon)}
                accessibilityLabel={m.nhan}
                onPress={() => {
                  if (!chon) onDoiMuc(m.muc);
                }}
                style={styles.muc}
              >
                <CopyText style={[typography.h2, styles.nhan, { color: chon ? colors.ink : colors.inkFaint }]}>
                  {m.nhan}
                </CopyText>
                <View style={[styles.bang, { backgroundColor: chon ? colors.accent : "transparent" }]} />
              </Pressable>
            );
          })}
        </View>
      </View>
      {phai ? <View style={[styles.phai, hep && styles.phaiHep]}>{phai}</View> : null}
    </View>
  );
}

const styles = StyleSheet.create({
  hang: { flexDirection: "row", alignItems: "center", gap: 8 },
  mucTrongHang: { flexGrow: 1, flexShrink: 1, minWidth: 0 },
  mucInline: { flexBasis: 0 },
  danhSach: { flexDirection: "row", gap: 24 },
  muc: { minHeight: 48, flexShrink: 1, minWidth: 0, alignItems: "center", justifyContent: "center", paddingTop: 6, gap: 4 },
  nhan: { maxWidth: "100%", textAlign: "center" },
  bang: { width: 28, height: 4, borderRadius: 2 },
  phai: { flexShrink: 0 },
  hangHep: { paddingTop: 48 },
  phaiHep: { position: "absolute", top: 0, right: 0 },
});
