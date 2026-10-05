/**
 * The bill table (ADR-0037 D1, D7; plan S1 «BanGanMon»): who had what, asked
 * the way a table answers it. The group sits around a paper table seen at an
 * angle, each person a standee in their own ink (D6); the dish being settled
 * is a folded card in the middle. Tap a seat -- or drag the card to it -- and
 * that person is in or out of the dish; a plate with an ink dot appears in
 * front of everyone who had it.
 *
 * Every door calls the caller's one `onToggle(personId)` (the screen passes
 * `toggle(assignment, line.id, id)` unchanged): tapping a seat, dragging the
 * card onto one, or a screen reader checking «Ghế {tên}». The seats are
 * controls, so they never move; only the table surface rises when the table
 * first opens. Nothing here computes a share -- the server splits.
 */
import { useEffect, useState } from "react";
import { Pressable, StyleSheet, Text, View, type LayoutChangeEvent } from "react-native";
import { Gesture, GestureDetector } from "react-native-gesture-handler";
import Animated, { Easing, runOnJS, useAnimatedStyle, useSharedValue, withSpring, withTiming } from "react-native-reanimated";
import Svg, { Ellipse, Rect } from "react-native-svg";

import { mucNguoi, phuMau, typography, useRudiTheme } from "../theme";
import { HinhNhan } from "./Avatar";
import { GHE, RONG_GHE, TEN_TREN, tenVua, theMonToiDa, viTriGhe, type NguoiQuanhBan, type ViTriGhe } from "./hinh-tien";
import { Money } from "./Money";
import { useMotion } from "./useMotion";
import { toggleState } from "../../ui/a11y";
import { gocBien } from "./goc-bien";

export function BanGanMon({
  mon,
  nguoi,
  dangDung,
  onToggle,
  disabled = false,
  testID,
}: {
  /** The dish on the table now. */
  mon: { id: string; ten: string; tienVnd: number } | null;
  nguoi: readonly NguoiQuanhBan[];
  /** Who had this dish. */
  dangDung: readonly string[];
  onToggle: (personId: string) => void;
  disabled?: boolean;
  testID?: string;
}) {
  const { colors, dark, radius } = useRudiTheme();
  const motion = useMotion();
  const [w, setW] = useState(0);
  const mat = useSharedValue(motion.reduced ? 1 : 0.25);
  const keoX = useSharedValue(0);
  const keoY = useSharedValue(0);
  useEffect(() => {
    mat.value = withTiming(1, { duration: motion.ms("shared"), easing: Easing.bezier(0, 0, 0.2, 1), reduceMotion: motion.reanimated });
    // The table rises once when it first appears.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  const hinh = w > 0 ? viTriGhe(nguoi, w) : null;

  const bam = (id: string) => {
    if (disabled) return;
    motion.haptic.select();
    onToggle(id);
  };
  const tha = (x: number, y: number) => {
    if (!hinh) return;
    let gan: ViTriGhe | null = null;
    let kc = 52;
    for (const g of hinh.ghe) {
      const d = Math.hypot(g.x - x, g.y - y);
      if (d < kc) {
        kc = d;
        gan = g;
      }
    }
    if (gan) bam(gan.id);
  };
  const keo = Gesture.Pan()
    .enabled(!disabled && mon !== null && hinh !== null)
    .minDistance(6)
    .onUpdate((e) => {
      keoX.value = e.translationX;
      keoY.value = e.translationY;
    })
    .onEnd((e) => {
      if (hinh) runOnJS(tha)(hinh.cx + e.translationX, hinh.cy + e.translationY);
    })
    .onFinalize(() => {
      keoX.value = withSpring(0, motion.spring.settle);
      keoY.value = withSpring(0, motion.spring.settle);
    });
  const kieuMat = useAnimatedStyle(() => ({ transform: [{ scaleY: mat.value }] }));
  const kieuThe = useAnimatedStyle(() => ({ transform: [{ translateX: keoX.value }, { translateY: keoY.value }] }));

  const ghe = (g: ViTriGhe) => {
    const co = dangDung.includes(g.id);
    const ten = (
      <Text numberOfLines={1} style={[typography.caption, { color: co ? mucNguoi(g.id, dark) : colors.inkSoft, maxWidth: GHE * 1.8, textAlign: "center" }]}>
        {tenVua(g.name, RONG_GHE)}
      </Text>
    );
    return (
      <Pressable
        accessibilityLabel={`Ghế ${g.name}`}
        {...toggleState("checkbox", co, disabled ? undefined : () => bam(g.id))}
        aria-disabled={disabled}
        key={g.id}
        onPress={() => bam(g.id)}
        style={[styles.ghe, { left: g.x - RONG_GHE / 2, top: g.y - GHE * 0.75 - (g.truoc ? 0 : TEN_TREN), width: RONG_GHE }]}
      >
        {g.truoc ? null : ten}
        {/* Out of the dish: the figure steps back (faded), so in and out differ by more than the ring's colour. */}
        <HinhNhan name={g.name} personId={g.id} ring={co} size={GHE * 0.78} style={co ? undefined : styles.vang} />
        {g.truoc ? ten : null}
      </Pressable>
    );
  };

  return (
    <View
      onLayout={(e: LayoutChangeEvent) => setW(Math.round(e.nativeEvent.layout.width))}
      style={[styles.khung, { height: hinh?.cao ?? 240 }]}
      testID={testID}
    >
      {hinh ? (
        <>
          {hinh.ghe.filter((g) => !g.truoc).map(ghe)}
          <Animated.View pointerEvents="none" style={[StyleSheet.absoluteFill, { transformOrigin: gocBien(hinh.cx, hinh.cy) }, kieuMat]}>
            <Svg height={hinh.cao} width={w}>
              {hinh.ban ? (
                // A big group's long table, seen from above: its shadow, its
                // edge, its top, drawn like the round one (QA UI-050).
                <>
                  <Rect fill={phuMau(colors.ink, dark ? 0.35 : 0.08)} height={hinh.ban.cao} rx={12} width={hinh.ban.rong} x={hinh.ban.trai + 3} y={hinh.ban.tren + 10} />
                  <Rect fill={colors.paperShade} height={hinh.ban.cao} rx={12} stroke={colors.lineStrong} strokeWidth={1} width={hinh.ban.rong} x={hinh.ban.trai} y={hinh.ban.tren + 5} />
                  <Rect fill={colors.card} height={hinh.ban.cao} rx={12} stroke={colors.lineStrong} strokeWidth={1.2} width={hinh.ban.rong} x={hinh.ban.trai} y={hinh.ban.tren} />
                </>
              ) : (
                <>
                  <Ellipse cx={hinh.cx + 3} cy={hinh.cy + 10} fill={phuMau(colors.ink, dark ? 0.35 : 0.08)} rx={hinh.rx} ry={hinh.ry} />
                  <Ellipse cx={hinh.cx} cy={hinh.cy + 5} fill={colors.paperShade} rx={hinh.rx} ry={hinh.ry} stroke={colors.lineStrong} strokeWidth={1} />
                  <Ellipse cx={hinh.cx} cy={hinh.cy} fill={colors.card} rx={hinh.rx} ry={hinh.ry} stroke={colors.lineStrong} strokeWidth={1.2} />
                </>
              )}
              {hinh.ghe.map((g) =>
                dangDung.includes(g.id) ? (
                  <Ellipse cx={g.dia.x} cy={g.dia.y} fill={colors.card} key={`dia-${g.id}`} rx={13} ry={6} stroke={mucNguoi(g.id, dark)} strokeWidth={2} />
                ) : null,
              )}
              {hinh.ghe.map((g) =>
                dangDung.includes(g.id) ? <Ellipse cx={g.dia.x} cy={g.dia.y - 1} fill={mucNguoi(g.id, dark)} key={`cham-${g.id}`} rx={3.2} ry={2} /> : null,
              )}
            </Svg>
          </Animated.View>
          {mon ? (
            // The card stands in the middle of the table. On the round table
            // it stays one name line tall: grown to two lines and a «Chia đều»
            // stamp it covered four plates, and the stamp laid over its corner
            // covered a fifth (B4 finish review). Who shares the dish is said
            // by the plates and by the dish's row under the table; the whole
            // name is there and in the card's name.
            <View pointerEvents="box-none" style={[styles.giua, { left: hinh.cx - hinh.rx, top: hinh.cy - hinh.ry, width: hinh.rx * 2, height: hinh.ry * 2 }]}>
              <GestureDetector gesture={keo}>
                <Animated.View
                  accessibilityHint="Kéo thẻ món tới một ghế, hoặc chạm vào ghế"
                  accessibilityLabel={`Món trên bàn: ${mon.ten}`}
                  // At least the card's size, grown to its amount, never past
                  // the table: a fixed width cut «12.345.678đ» to «12.345.6…»
                  // at 320 (QA UI-048). The dish's name wraps first.
                  style={[styles.the, { minWidth: hinh.the.w, maxWidth: theMonToiDa(hinh), minHeight: hinh.the.h, backgroundColor: colors.card, borderColor: colors.lineStrong, borderRadius: radius.small / 2 }, kieuThe]}
                >
                  <Text numberOfLines={hinh.ban ? 2 : 1} style={[typography.label, styles.tenMon, { color: colors.ink }]}>
                    {mon.ten}
                  </Text>
                  {/* One size down on a card too small for the label size: the
                      amount is written smaller, never cut. */}
                  <Money size={theMonToiDa(hinh) < 124 ? "caption" : "label"} tone="split" vnd={mon.tienVnd} />
                </Animated.View>
              </GestureDetector>
            </View>
          ) : null}
          {hinh.ghe.filter((g) => g.truoc).map(ghe)}
        </>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  khung: { width: "100%" },
  ghe: { position: "absolute", alignItems: "center", gap: 2, minHeight: 48 },
  vang: { opacity: 0.45 },
  giua: { position: "absolute", alignItems: "center", justifyContent: "center" },
  the: { borderWidth: 1, paddingHorizontal: 10, paddingVertical: 6, alignItems: "center", justifyContent: "center", gap: 2 },
  tenMon: { textAlign: "center" },
});
