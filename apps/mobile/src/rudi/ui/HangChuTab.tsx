import { useEffect, useRef } from "react";
import { Pressable, ScrollView, StyleSheet, Text, View } from "react-native";

import { TABLIST, tabState } from "../../ui/a11y";
import { typography, useRudiTheme } from "../theme";

export type MucChuTab<T extends string> = { id: T; nhan: string };

type Doan = { x: number; w: number };

/**
 * Where the row must scroll so `o` (a tab, in the row's own coordinates) is in
 * view with `le` to spare on its side, given the visible window `khung`; null
 * when it already is, or when the window is not measured yet. Never before
 * the start of the row.
 */
export function cuonDeThay(o: Doan, khung: Doan, le: number): number | null {
  if (khung.w <= 0) return null;
  if (o.x - le < khung.x) return Math.max(0, o.x - le);
  if (o.x + o.w + le > khung.x + khung.w) return o.x + o.w + le - khung.w;
  return null;
}

/**
 * A row of small text tabs: the community feed's modes (owner's choice,
 * 02/10). It is told apart from the filter chips of Địa điểm on purpose: words
 * at label size on a hairline, the selected one in ink with a 2 dp ink rule
 * under it and the others in `inkSoft` — no outline, no fill, no tick, because
 * a tab picks what the list is, where a chip narrows it. One step below
 * Khám phá's own header (heading size, coral tape), so the two rows never
 * read as the same control.
 *
 * The row runs to both screen edges and scrolls sideways; picking a tab that
 * sits half off screen brings it into view, and `chon` may be a mode with no
 * tab (the hidden posts), when none is selected.
 */
export function HangChuTab<T extends string>({ muc, chon, onChon }: { muc: readonly MucChuTab<T>[]; chon: T | null; onChon: (id: T) => void }) {
  const { colors } = useRudiTheme();
  const cuon = useRef<ScrollView>(null);
  const viTri = useRef(new Map<T, Doan>());
  const khung = useRef<Doan>({ x: 0, w: 0 });
  const hienChon = () => {
    const o = chon === null ? undefined : viTri.current.get(chon);
    if (o === undefined) return;
    const x = cuonDeThay(o, khung.current, LE);
    if (x !== null) cuon.current?.scrollTo({ x, animated: false });
  };
  // Asked again when the row first knows its width and each tab its place.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(hienChon, [chon]);
  return (
    <View style={[styles.vien, { borderBottomColor: colors.line }]}>
      <ScrollView
        contentContainerStyle={styles.trong}
        horizontal
        onLayout={(e) => {
          khung.current = { ...khung.current, w: e.nativeEvent.layout.width };
          hienChon();
        }}
        onScroll={(e) => {
          khung.current = { ...khung.current, x: e.nativeEvent.contentOffset.x };
        }}
        ref={cuon}
        scrollEventThrottle={32}
        showsHorizontalScrollIndicator={false}
      >
        <View {...TABLIST} style={styles.danhSach}>
          {muc.map((m) => {
            const dangChon = m.id === chon;
            return (
              <Pressable
                key={m.id}
                {...tabState(dangChon)}
                accessibilityLabel={m.nhan}
                onLayout={(e) => {
                  viTri.current.set(m.id, { x: e.nativeEvent.layout.x + LE, w: e.nativeEvent.layout.width });
                  if (dangChon) hienChon();
                }}
                onPress={() => {
                  if (!dangChon) onChon(m.id);
                }}
                style={styles.tab}
              >
                <Text numberOfLines={1} style={[typography.label, { color: dangChon ? colors.ink : colors.inkSoft }]}>
                  {m.nhan}
                </Text>
                <View style={[styles.gach, { backgroundColor: dangChon ? colors.ink : "transparent" }]} />
              </Pressable>
            );
          })}
        </View>
      </ScrollView>
    </View>
  );
}

/** The screen's gutter (`space.md`): the row starts on it and keeps it when it scrolls. */
const LE = 16;

const styles = StyleSheet.create({
  // Out to both screen edges, the hairline with it; the words start on the gutter.
  vien: { marginHorizontal: -LE, borderBottomWidth: StyleSheet.hairlineWidth },
  trong: { paddingHorizontal: LE },
  danhSach: { flexDirection: "row", gap: 22 },
  // 48 dp tall, the words low so the rule sits on the hairline.
  tab: { minHeight: 48, justifyContent: "flex-end", paddingTop: 12 },
  gach: { height: 2, marginTop: 11, borderRadius: 1 },
});
