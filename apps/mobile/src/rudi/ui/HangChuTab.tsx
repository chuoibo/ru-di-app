import { useEffect, useMemo, useRef, useState } from "react";
import { Animated, Easing, Pressable, ScrollView, StyleSheet, Text, View } from "react-native";

import { TABLIST, tabState } from "../../ui/a11y";
import { EASING, durationFor } from "../motion";
import { typography, useRudiTheme } from "../theme";
import { useAdaptiveLayout } from "./useAdaptiveLayout";

export type MucChuTab<T extends string> = { id: T; nhan: string };

type Doan = { x: number; w: number };

/**
 * Where the row must scroll so `o` (a tab, in the row's own coordinates) is in
 * view with `leTrai` to spare on its left and `lePhai` on its right, given the
 * visible window `khung`; null when it already is, or when the window is not
 * measured yet. Never before the start of the row. The caller spares a gap
 * and a peek on a side where another tab follows, so that tab still shows.
 */
export function cuonDeThay(o: Doan, khung: Doan, leTrai: number, lePhai: number = leTrai): number | null {
  if (khung.w <= 0) return null;
  if (o.x - leTrai < khung.x) return Math.max(0, o.x - leTrai);
  if (o.x + o.w + lePhai > khung.x + khung.w) return o.x + o.w + lePhai - khung.w;
  return null;
}

/** The gap between tabs, and how far it may move to make the edge cut a word. */
const KHOANG = 22;
const KHOANG_MIN = 12;
const KHOANG_MAX = 32;
/** A cut word shows at least this much, and hides at least this much past the edge. */
const LO_HIEN = 16;
const LO_AN = 10;

/** How much paper may follow the last tab beyond the gutter, to place the left edge. */
const DEM_THEM = 8;

/**
 * The gap between tabs, and the paper after the last one, that make a window
 * edge cut a word in two when the row is wider than the window, so the row
 * says it goes on (finish review 03/10: at 390 on the web, and at 1.3 on
 * Android, the edge fell between two tabs and the row looked finished, with
 * «Bài của tôi» out of sight). Both ends count: at rest the right edge cuts a
 * word, and scrolled to the end (the last tab picked) the left edge does, or
 * «Dành cho bạn» went with no trace (verdict 03/10). One gap cannot always
 * place both edges, so the paper after the last tab may grow by up to 8 dp.
 * No fade or arrow: the world takes no gradient and the tabs no chrome. The
 * gap closest to the usual one wins, then the least paper; a row that fits
 * keeps the usual gap and gutter.
 */
export function khoangCachLo(rong: readonly number[], khung: number, le: number): { khoang: number; demCuoi: number } {
  const thuong = { khoang: KHOANG, demCuoi: le };
  if (rong.length === 0 || khung <= 0) return thuong;
  const tong = (g: number, p: number) => rong.reduce((a, b) => a + b, 0) + g * (rong.length - 1) + le + p;
  if (tong(KHOANG, le) <= khung) return thuong;
  // Whether a tab straddles the window's edge at `bien` (row coordinates),
  // showing at least LO_HIEN on the window's side and hiding at least LO_AN.
  const catTai = (g: number, bien: number, cuaSo: "trai" | "phai") => {
    let x = le;
    for (const w of rong) {
      const hien = cuaSo === "trai" ? bien - x : x + w - bien;
      const an = cuaSo === "trai" ? x + w - bien : bien - x;
      if (hien >= LO_HIEN && an >= LO_AN) return true;
      x += w + g;
    }
    return false;
  };
  for (let d = 0; d <= KHOANG_MAX - KHOANG_MIN; d++) {
    for (const g of [KHOANG - d, KHOANG + d]) {
      if (g < KHOANG_MIN || g > KHOANG_MAX || !catTai(g, khung, "trai")) continue;
      for (let p = le; p <= le + DEM_THEM; p++) {
        // At rest the window is [0, khung]; scrolled to the end, [tong - khung, tong].
        if (tong(g, p) > khung && catTai(g, tong(g, p) - khung, "phai")) return { khoang: g, demCuoi: p };
      }
    }
  }
  return thuong;
}

/**
 * Where the ink rule sits under tab `chon`, in the list's own coordinates
 * (after the gutter): the widths before it and their gaps. Null with no tab
 * picked or a width not measured yet.
 */
export function viTriGach<T extends string>(muc: readonly MucChuTab<T>[], rong: Readonly<Record<string, number>>, khoang: number, chon: T | null): Doan | null {
  let x = 0;
  for (const m of muc) {
    const w = rong[m.id];
    if (w === undefined) return null;
    if (m.id === chon) return { x, w };
    x += w + khoang;
  }
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
 * On a phone the row runs to both screen edges and scrolls sideways, its gap
 * set so the edge cuts a word when it does not fit (`khoangCachLo`); in a
 * tablet's reading column it keeps to the column, words and hairline alike.
 * Picking a tab that sits half off screen brings it into view, and `chon` may
 * be a mode with no tab (the hidden posts), when none is selected.
 */
export function HangChuTab<T extends string>({
  muc,
  chon,
  onChon,
  giamChuyenDong = false,
}: {
  muc: readonly MucChuTab<T>[];
  chon: T | null;
  onChon: (id: T) => void;
  /** Reduce Motion: the rule jumps to the picked tab instead of sliding. */
  giamChuyenDong?: boolean;
}) {
  const { colors } = useRudiTheme();
  // Out to the screen's edges only where the column is the screen.
  const tran = useAdaptiveLayout().sizeClass === "compact";
  const le = tran ? LE : 0;
  const cuon = useRef<ScrollView>(null);
  const khung = useRef<Doan>({ x: 0, w: 0 });
  // Each word's width and the window's: they do not depend on the gap, so
  // the gap is worked out once both are known and the row is laid out again.
  const [rongTab, setRongTab] = useState<Readonly<Record<string, number>>>({});
  const [rongKhung, setRongKhung] = useState(0);
  const { khoang, demCuoi } = useMemo(() => {
    const rong = muc.map((m) => rongTab[m.id]);
    return rong.every((w) => w !== undefined) ? khoangCachLo(rong as number[], rongKhung, le) : { khoang: KHOANG, demCuoi: le };
  }, [muc, rongTab, rongKhung, le]);
  // One ink rule that slides from tab to tab over `standard`, as the strip's
  // own tape does; a jump under Reduce Motion, and on the first placement.
  // Until every width is measured each tab draws its own rule instead.
  const gach = viTriGach(muc, rongTab, khoang, chon);
  const gachX = useRef(new Animated.Value(0)).current;
  const gachW = useRef(new Animated.Value(0)).current;
  const daDatGach = useRef(false);
  useEffect(() => {
    if (gach === null) return;
    if (!daDatGach.current || giamChuyenDong) {
      gachX.setValue(gach.x);
      gachW.setValue(gach.w);
      daDatGach.current = true;
      return;
    }
    const cach = { duration: durationFor("standard", false), easing: Easing.bezier(...EASING.standard), useNativeDriver: false };
    Animated.parallel([Animated.timing(gachX, { ...cach, toValue: gach.x }), Animated.timing(gachW, { ...cach, toValue: gach.w })]).start();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [gach?.x, gach?.w, giamChuyenDong]);
  // Where the selected tab sits in the row, from the widths and the gap: on
  // the web onLayout reports a change of size only, so a position read from
  // it went stale when the gap moved and the row stopped 20 dp short.
  const viTriChon = (): (Doan & { i: number }) | undefined => {
    let x = le;
    for (const [i, m] of muc.entries()) {
      const w = rongTab[m.id];
      if (w === undefined) return undefined;
      if (m.id === chon) return { x, w, i };
      x += w + khoang;
    }
    return undefined;
  };
  const hienChon = () => {
    const o = viTriChon();
    if (o === undefined) return;
    // A neighbour keeps a peek in view, so the row still says it goes on.
    const canh = khoang + LO_HIEN;
    const x = cuonDeThay(o, khung.current, o.i === 0 ? le : canh, o.i === muc.length - 1 ? demCuoi : canh);
    if (x !== null) cuon.current?.scrollTo({ x, animated: false });
  };
  // Asked again once the row knows its width, its words' and its gap.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(hienChon, [chon, khoang, demCuoi, rongKhung]);
  return (
    <View style={[styles.vien, tran && styles.tran, { borderBottomColor: colors.line }]}>
      <ScrollView
        contentContainerStyle={tran ? { paddingLeft: le, paddingRight: demCuoi } : null}
        horizontal
        onLayout={(e) => {
          khung.current = { ...khung.current, w: e.nativeEvent.layout.width };
          setRongKhung(Math.round(e.nativeEvent.layout.width));
        }}
        onScroll={(e) => {
          khung.current = { ...khung.current, x: e.nativeEvent.contentOffset.x };
        }}
        ref={cuon}
        scrollEventThrottle={32}
        showsHorizontalScrollIndicator={false}
      >
        <View {...TABLIST} style={[styles.danhSach, { gap: khoang }]}>
          {muc.map((m) => {
            const dangChon = m.id === chon;
            return (
              <Pressable
                key={m.id}
                {...tabState(dangChon)}
                accessibilityLabel={m.nhan}
                onLayout={(e) => {
                  const w = Math.round(e.nativeEvent.layout.width);
                  setRongTab((cu) => (cu[m.id] === w ? cu : { ...cu, [m.id]: w }));
                }}
                onPress={() => {
                  if (!dangChon) onChon(m.id);
                }}
                style={styles.tab}
              >
                <Text numberOfLines={1} style={[typography.label, { color: dangChon ? colors.ink : colors.inkSoft }]}>
                  {m.nhan}
                </Text>
                <View style={[styles.gach, { backgroundColor: dangChon && gach === null ? colors.ink : "transparent" }]} />
              </Pressable>
            );
          })}
          {gach !== null ? (
            <Animated.View pointerEvents="none" style={[styles.gachTruot, { left: gachX, width: gachW, backgroundColor: colors.ink }]} testID="gach-chu-tab" />
          ) : null}
        </View>
      </ScrollView>
    </View>
  );
}

/** The screen's gutter (`space.md`): the row starts on it and keeps it when it scrolls. */
const LE = 16;

const styles = StyleSheet.create({
  vien: { borderBottomWidth: StyleSheet.hairlineWidth },
  // On a phone: out to both screen edges, the hairline with it; the words start on the gutter.
  tran: { marginHorizontal: -LE },
  danhSach: { flexDirection: "row" },
  // 48 dp tall, the words low so the rule sits on the hairline.
  tab: { minHeight: 48, justifyContent: "flex-end", paddingTop: 12 },
  gach: { height: 2, marginTop: 11, borderRadius: 1 },
  gachTruot: { position: "absolute", bottom: 0, height: 2, borderRadius: 1 },
});
