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
import Svg, { Ellipse } from "react-native-svg";

import { mucNguoi, phuMau, typography, useRudiTheme } from "../theme";
import { HinhNhan } from "./Avatar";
import { GHE, RONG_GHE, TEN_TREN, theMon, viTriGhe, type NguoiQuanhBan, type ViTriGhe } from "./hinh-tien";
import { Money } from "./Money";
import { Stamp } from "./Stamp";
import { useMotion } from "./useMotion";

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
  const tatCa = nguoi.length > 1 && nguoi.every((p) => dangDung.includes(p.id));

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
        {g.name}
      </Text>
    );
    return (
      <Pressable
        accessibilityLabel={`Ghế ${g.name}`}
        accessibilityRole="checkbox"
        accessibilityState={{ checked: co, disabled }}
        aria-checked={co}
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
          <Animated.View pointerEvents="none" style={[StyleSheet.absoluteFill, { transformOrigin: `${hinh.cx}px ${hinh.cy}px` }, kieuMat]}>
            <Svg height={hinh.cao} width={w}>
              <Ellipse cx={hinh.cx + 3} cy={hinh.cy + 10} fill={phuMau(colors.ink, dark ? 0.35 : 0.08)} rx={hinh.rx} ry={hinh.ry} />
              <Ellipse cx={hinh.cx} cy={hinh.cy + 5} fill={colors.paperShade} rx={hinh.rx} ry={hinh.ry} stroke={colors.lineStrong} strokeWidth={1} />
              <Ellipse cx={hinh.cx} cy={hinh.cy} fill={colors.card} rx={hinh.rx} ry={hinh.ry} stroke={colors.lineStrong} strokeWidth={1.2} />
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
            // The card stands in the middle of the table whatever its height:
            // «Chia đều» is stamped on the card itself, where no seat or plate is.
            <View pointerEvents="box-none" style={[styles.giua, { left: hinh.cx - hinh.rx, top: hinh.cy - hinh.ry, width: hinh.rx * 2, height: hinh.ry * 2 }]}>
              <GestureDetector gesture={keo}>
                <Animated.View
                  accessibilityHint="Kéo thẻ món tới một ghế, hoặc chạm vào ghế"
                  accessibilityLabel={`Món trên bàn: ${mon.ten}`}
                  style={[styles.the, { width: theMon(hinh.rx).w, minHeight: theMon(hinh.rx).h, backgroundColor: colors.card, borderColor: colors.lineStrong, borderRadius: radius.small / 2 }, kieuThe]}
                >
                  <Text numberOfLines={1} style={[typography.label, { color: colors.ink }]}>
                    {mon.ten}
                  </Text>
                  <Money size="label" tone="split" vnd={mon.tienVnd} />
                  {tatCa ? <Stamp label="Chia đều" tilt={-2} tone="split" /> : null}
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
});
