import { Image } from "expo-image";
import { useEffect, useRef, useState } from "react";
import { Animated, Easing, StyleSheet, View } from "react-native";
import Svg, { Circle, G, Path, Text as SvgText } from "react-native-svg";

import type { RandomMode, RandomOption, RandomRound } from "../../chat/e2ee/random-v2";
import { mauLuotChon, typography, useRudiTheme } from "../../theme";
import { Text } from "../../ui/NativeFontMetrics";
import { Nep } from "../../ui/art/Nep";
import { useMotion } from "../../ui/useMotion";

const PAPER_INKS = mauLuotChon.paper;
const DUCK = {
  idle: require("../../../../assets/rudi/random/duck-idle-v2.png"),
  racing: require("../../../../assets/rudi/random/duck-racing-v2.png"),
  winner: require("../../../../assets/rudi/random/duck-winner-v2.png"),
};
const point = (r: number, angle: number) => ({ x: 150 + r * Math.cos(angle * Math.PI / 180), y: 150 + r * Math.sin(angle * Math.PI / 180) });
const wedge = (index: number, count: number) => {
  const a = point(132, -90 + index * 360 / count), b = point(132, -90 + (index + 1) * 360 / count);
  return `M 150 150 L ${a.x} ${a.y} A 132 132 0 ${360 / count > 180 ? 1 : 0} 1 ${b.x} ${b.y} Z`;
};

/** Motion illustrates an already frozen result. It never chooses or writes. */
export function RandomGame({ mode, options, round = null, animate = false, width = 260, onComplete }: {
  mode: RandomMode; options: RandomOption[]; round?: RandomRound | null; animate?: boolean;
  width?: number; onComplete?: () => void;
}) {
  const { colors, brand } = useRudiTheme();
  const { reduced } = useMotion();
  const progress = useRef(new Animated.Value(animate && round ? 0 : 1)).current;
  const callback = useRef(onComplete); callback.current = onComplete;
  const [running, setRunning] = useState(animate && !!round && !reduced);
  useEffect(() => {
    progress.stopAnimation();
    if (!round || !animate || reduced) {
      progress.setValue(1); setRunning(false);
      if (round && animate) callback.current?.();
      return;
    }
    progress.setValue(0); setRunning(true);
    const motion = Animated.timing(progress, { toValue: 1, duration: mode === "wheel" ? 3800 : 4800,
      easing: mode === "wheel" ? Easing.out(Easing.exp) : Easing.linear, useNativeDriver: true });
    motion.start(({ finished }) => { if (finished) { setRunning(false); callback.current?.(); } });
    return () => motion.stop();
  }, [round?.id, animate, mode, reduced, progress]);
  const size = Math.max(160, Math.min(width, 300));
  const count = Math.max(2, options.length);
  const named = options.filter((option) => option.label.trim()).length;
  const finalAngle = round ? 1800 - (round.winner + 0.5) * 360 / count : 0;
  const accessible = round && !running ? `${mode === "wheel" ? "Vòng quay" : "Đua vịt"} chọn ${options[round.winner]?.label}`
    : running ? "Đang chơi lượt đã chốt" : `${mode === "wheel" ? "Vòng quay" : "Đường đua"}, ${options.length} lựa chọn`;
  return <View style={styles.game} accessibilityLabel={accessible} accessible accessibilityLiveRegion="polite">
    <View accessibilityElementsHidden importantForAccessibility="no-hide-descendants" pointerEvents="none" style={styles.object}>
      {mode === "wheel" ? <View style={{ width: size, height: size }}>
        <Animated.View style={{ transform: [{ rotate: progress.interpolate({ inputRange: [0, 1], outputRange: ["0deg", `${finalAngle}deg`] }) }] }}>
          <Svg width={size} height={size} viewBox="0 0 300 300">
            <Circle cx={150} cy={150} r={140} fill={colors.card} stroke={colors.lineStrong} strokeWidth={2} />
            <G>{Array.from({ length: count }, (_, index) => {
              const labelPoint = point(98, -90 + (index + 0.5) * 360 / count);
              return <G key={index}><Path d={wedge(index, count)} fill={named ? PAPER_INKS[index] : colors.paperShade} stroke={colors.card} strokeWidth={2} />
                {options[index]?.label.trim() ? <SvgText x={labelPoint.x} y={labelPoint.y + 6} textAnchor="middle" fontSize={21} fontWeight="700" fill={mauLuotChon.ink}>{index + 1}</SvgText> : null}</G>;
            })}</G>
            <Circle cx={150} cy={150} r={42} fill={colors.card} stroke={colors.lineStrong} strokeWidth={1.5} />
          </Svg>
        </Animated.View>
        <View style={[styles.nep, { left: size / 2 - 29, top: size / 2 - 29 }]}><Nep pose="gop-y" size={58} /></View>
        <View style={[styles.pointer, { left: size / 2 - 9, borderTopColor: brand.coral }]} />
      </View> : <View style={[styles.pond, { width: size, backgroundColor: colors.splitSoft, borderColor: colors.lineStrong }]}>
        <View style={[styles.finish, { borderColor: colors.split }]} />
        {(options.length ? options : [{ label: "" }, { label: "" }]).map((option, index) => {
          const end = size - 76;
          const salt = round ? parseInt(round.seed.slice(index * 3, index * 3 + 3), 16) / 4095 : 0.5;
          const winner = round?.winner === index;
          const x = round ? [0, end * (0.08 + salt * 0.1), end * (0.25 + salt * 0.18), end * (0.48 + salt * 0.18), end * (winner ? 1 : 0.66 + salt * 0.18)] : [0, 0, 0, 0, 0];
          return <View key={index} style={[styles.lane, { borderBottomColor: colors.line }]}>
            <Text style={[typography.label, styles.laneNumber, { color: colors.inkSoft }]}>{option.label ? index + 1 : ""}</Text>
            <View style={[styles.ripple, { borderColor: colors.split }]} />
            <Animated.View style={[styles.duck, { transform: [{ translateX: progress.interpolate({ inputRange: [0, 0.22, 0.48, 0.72, 1], outputRange: x }) }] }]}>
              <Image source={running ? DUCK.racing : winner ? DUCK.winner : DUCK.idle} style={styles.duckImage} contentFit="contain" />
            </Animated.View>
          </View>;
        })}
      </View>}
    </View>
    {named < 2 ? <Text style={[typography.note, styles.status, { color: colors.inkSoft }]}>Thêm hai món hoặc quán để bắt đầu.</Text>
      : running ? <Text style={[typography.label, styles.status, { color: colors.inkSoft }]}>{mode === "wheel" ? "Vòng quay đang chậm lại…" : "Các bạn vịt đang về đích…"}</Text> : null}
  </View>;
}

const styles = StyleSheet.create({
  game: { alignItems: "center", gap: 10, paddingVertical: 12 }, object: { alignItems: "center" },
  nep: { position: "absolute", width: 58, height: 58 },
  pointer: { position: "absolute", top: 2, borderLeftWidth: 9, borderRightWidth: 9, borderTopWidth: 23, borderLeftColor: "transparent", borderRightColor: "transparent" },
  pond: { borderWidth: 1, borderRadius: 18, overflow: "hidden", paddingHorizontal: 4 },
  lane: { height: 56, borderBottomWidth: 1, justifyContent: "center" },
  laneNumber: { position: "absolute", left: 4, top: 4 },
  finish: { position: "absolute", right: 20, top: 0, bottom: 0, borderLeftWidth: 2, borderStyle: "dashed" },
  ripple: { position: "absolute", left: "42%", width: 22, height: 6, borderBottomWidth: 1, borderRadius: 10, opacity: 0.5 },
  duck: { position: "absolute", left: 12, width: 56, height: 52 }, duckImage: { width: 56, height: 52 },
  status: { textAlign: "center", maxWidth: "100%" },
});
