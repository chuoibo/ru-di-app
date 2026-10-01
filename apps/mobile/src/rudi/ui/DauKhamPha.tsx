import type { ReactNode } from "react";
import { Pressable, ScrollView, StyleSheet, Text, View } from "react-native";

import { TABLIST, tabState } from "../../ui/a11y";
import { typography, useRudiTheme } from "../theme";
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
 * words scroll sideways rather than clip when a large text size outgrows a
 * narrow phone; `phai` (the feed settings, or the demo door) stays put,
 * outside the tablist, which may own tabs only.
 */
export function DauKhamPha({ muc, onDoiMuc, phai }: { muc: MucKhamPha; onDoiMuc: (muc: MucKhamPha) => void; phai?: ReactNode }) {
  const { colors } = useRudiTheme();
  return (
    <View style={styles.hang} testID="dau-kham-pha">
      <ScrollView contentContainerStyle={styles.cuonTrong} horizontal showsHorizontalScrollIndicator={false} style={styles.cuon}>
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
                <Text numberOfLines={1} style={[typography.h2, { color: chon ? colors.ink : colors.inkFaint }]}>
                  {m.nhan}
                </Text>
                <View style={[styles.bang, { backgroundColor: chon ? colors.accent : "transparent" }]} />
              </Pressable>
            );
          })}
        </View>
      </ScrollView>
      {phai ? <View style={styles.phai}>{phai}</View> : null}
    </View>
  );
}

const styles = StyleSheet.create({
  hang: { flexDirection: "row", alignItems: "center", gap: 8 },
  cuon: { flexGrow: 1, flexShrink: 1 },
  cuonTrong: { flexGrow: 1 },
  danhSach: { flexDirection: "row", gap: 24 },
  muc: { minHeight: 48, alignItems: "center", justifyContent: "center", paddingTop: 6, gap: 4 },
  bang: { width: 28, height: 4, borderRadius: 2 },
  phai: { flexShrink: 0 },
});
