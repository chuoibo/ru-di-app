import { Image } from "expo-image";
import { useEffect, useMemo, useRef, useState } from "react";
import { Animated, Easing, StyleSheet, View } from "react-native";
import Svg, { Circle, G, Path, Rect, Text as SvgText } from "react-native-svg";

import { duongDua, khungDuongDua, MOC_DUONG_DUA } from "../../chat/e2ee/duong-dua";
import type { RandomLiveView, RandomPlayableRound } from "../../chat/e2ee/random-live";
import type { RandomMode, RandomOption } from "../../chat/e2ee/random-v2";
import { mauLuotChon, typography, useRudiTheme } from "../../theme";
import { Text } from "../../ui/NativeFontMetrics";
import { Nep } from "../../ui/art/Nep";
import { useMotion } from "../../ui/useMotion";

const PAPER_INKS = mauLuotChon.paper;
const DUCKS = [
  require("../../../../assets/rudi/random/duck-coral-v3.png"),
  require("../../../../assets/rudi/random/duck-mint-v3.png"),
  require("../../../../assets/rudi/random/duck-navy-v3.png"),
  require("../../../../assets/rudi/random/duck-lilac-v3.png"),
];
const RIVER = require("../../../../assets/rudi/random/river-arena-v2.png");
const point = (r: number, angle: number) => ({ x: 150 + r * Math.cos(angle * Math.PI / 180), y: 150 + r * Math.sin(angle * Math.PI / 180) });
const wedge = (index: number, count: number) => {
  const a = point(132, -90 + index * 360 / count), b = point(132, -90 + (index + 1) * 360 / count);
  return `M 150 150 L ${a.x} ${a.y} A 132 132 0 ${360 / count > 180 ? 1 : 0} 1 ${b.x} ${b.y} Z`;
};

/** Motion illustrates an already frozen result. It never chooses or writes. */
export function RandomGame({ mode, options, round = null, live = null, animate = false, compact = false, width = 260, onComplete }: {
  mode: RandomMode; options: RandomOption[]; round?: RandomPlayableRound | null; live?: RandomLiveView | null; animate?: boolean; compact?: boolean;
  width?: number; onComplete?: () => void;
}) {
  const { colors, brand } = useRudiTheme();
  const { reduced } = useMotion();
  const progress = useRef(new Animated.Value(0)).current;
  const callback = useRef(onComplete); callback.current = onComplete;
  const [localRunning, setRunning] = useState(animate && !!round && !reduced);
  const scheduled = round?.v === 2 && live !== null;
  const running = scheduled ? live.phase === "racing" && !reduced : localRunning;
  const reveal = scheduled ? live.reveal : !!round && !localRunning;
  useEffect(() => {
    if (scheduled) return;
    progress.stopAnimation();
    if (!round || !animate || reduced) {
      progress.setValue(round ? 1 : 0); setRunning(false);
      if (round && animate) callback.current?.();
      return;
    }
    progress.setValue(0); setRunning(true);
    const motion = Animated.timing(progress, { toValue: 1, duration: mode === "wheel" ? 3800 : 12_000,
      easing: mode === "wheel" ? Easing.out(Easing.exp) : Easing.linear, useNativeDriver: true });
    motion.start(({ finished }) => { if (finished) { setRunning(false); callback.current?.(); } });
    return () => motion.stop();
  }, [round?.id, animate, mode, reduced, progress, scheduled]);
  useEffect(() => {
    if (!scheduled) return;
    progress.stopAnimation();
    progress.setValue(reduced && !live.reveal ? 0 : live.reveal ? 1 : Math.min(live.progress, 0.97));
    if (reduced || live.phase !== "racing" || live.progress >= 0.97) return;
    // Native frames run between clock updates; a late join seeks before moving.
    const motion = Animated.timing(progress, { toValue: 0.97, duration: live.remainingMs, easing: Easing.linear, useNativeDriver: true });
    motion.start();
    return () => motion.stop();
  }, [scheduled, live?.phase, live?.progress, live?.remainingMs, live?.reveal, reduced, progress]);
  const size = Math.max(160, Math.min(width, 300));
  const count = Math.max(2, options.length);
  const arena = khungDuongDua(width, count, compact);
  const tracks = useMemo(() => Array.from({ length: count }, (_, index) => ({
    x: round ? duongDua(round.seed, index, round.winner).map((distance) => distance * arena.travel) : MOC_DUONG_DUA.map(() => index % 2 * 14),
    bob: MOC_DUONG_DUA.map((step) => Math.sin(step * Math.PI * 12 + index * 1.4) * 2.3),
    tilt: MOC_DUONG_DUA.map((step) => `${Math.sin(step * Math.PI * 10 + index) * 3}deg`),
  })), [round?.id, round?.seed, round?.winner, count, arena.travel]);
  const named = options.filter((option) => option.label.trim()).length;
  const finalAngle = round ? 1800 - (round.winner + 0.5) * 360 / count : 0;
  const liveStatus = scheduled ? live.phase === "countdown" ? `Xuất phát sau ${Math.ceil(live.remainingMs / 1000)} giây`
    : live.phase === "racing" ? `Đang ${mode === "wheel" ? "quay" : "đua"} · ${Math.ceil(live.remainingMs / 1000)} giây`
    : live.phase === "syncing" ? "Đang đồng bộ giờ…" : live.phase === "unavailable" ? live.reason : null : null;
  const accessible = round && reveal ? `${mode === "wheel" ? "Vòng quay" : "Đua vịt"} chọn ${options[round.winner]?.label}`
    : liveStatus ? liveStatus
    : running ? "Đang chơi lượt đã chốt" : `${mode === "wheel" ? "Vòng quay" : "Đường đua"}, ${options.length} lựa chọn`;
  return <View style={styles.game} accessibilityLabel={accessible} accessible accessibilityLiveRegion="polite">
    {liveStatus ? <Text style={[typography.label, styles.status, { color: colors.inkSoft }]}>{liveStatus}</Text> : null}
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
      </View> : <View testID="random-river-arena" style={[styles.pond, { width: arena.width, height: arena.height }]}>
        <Image source={RIVER} style={StyleSheet.absoluteFill} contentFit="fill" cachePolicy="memory-disk" />
        <Svg width={arena.width} height={arena.height} style={StyleSheet.absoluteFill}>
          <Rect x={arena.finish - 1} y={18} width={2} height={arena.height - 36} fill={mauLuotChon.ink} />
          {Array.from({ length: Math.ceil((arena.height - 48) / 10) }, (_, row) => [0, 1].map((col) =>
            <Rect key={`${row}:${col}`} x={arena.finish - 6 + col * 6} y={24 + row * 10} width={6} height={10}
              fill={(row + col) % 2 ? mauLuotChon.ink : "#FFF7E9"} />))}
          <Path d={`M ${arena.finish - 18} 18 Q ${arena.finish - 8} 12 ${arena.finish + 6} 18 L ${arena.finish + 6} 30 Q ${arena.finish - 8} 24 ${arena.finish - 18} 30 Z`} fill={brand.coral} />
        </Svg>
        {tracks.map((track, index) => {
          const top = 26 + (arena.height - 52 - arena.duck) * index / (count - 1);
          return <Animated.View key={index} style={[styles.runner, { left: 12, top, width: arena.duck, height: arena.duck,
            transform: [{ translateX: progress.interpolate({ inputRange: MOC_DUONG_DUA, outputRange: track.x }) },
              { translateY: running ? progress.interpolate({ inputRange: MOC_DUONG_DUA, outputRange: track.bob }) : 0 }] }]}>
            <Svg width={arena.duck + 22} height={22} style={[styles.wake, { bottom: 0, left: -20, opacity: running ? 0.85 : 0.45 }]}>
              <Path d={`M 2 13 Q 12 6 22 13 T ${arena.duck + 16} 13 M 6 18 Q 14 12 24 18 T ${arena.duck + 12} 18`} fill="none" stroke="#E8F5E9" strokeWidth={1.5} />
            </Svg>
            <Animated.View style={{ transform: [{ rotate: running ? progress.interpolate({ inputRange: MOC_DUONG_DUA, outputRange: track.tilt }) : "0deg" }] }}>
              <Image source={DUCKS[index % DUCKS.length]} style={{ width: arena.duck, height: arena.duck }} contentFit="contain" />
            </Animated.View>
            <View style={[styles.badge, { backgroundColor: PAPER_INKS[index], borderColor: reveal && round?.winner === index ? brand.coral : "#FFF7E9" }]}>
              <Text allowFontScaling={false} style={[styles.badgeText, { color: mauLuotChon.ink }]}>{index + 1}</Text>
            </View>
          </Animated.View>;
        })}
      </View>}
    </View>
    {named < 2 ? <Text style={[typography.note, styles.status, { color: colors.inkSoft }]}>Thêm hai món hoặc quán để bắt đầu.</Text>
      : running && !scheduled ? <Text style={[typography.label, styles.status, { color: colors.inkSoft }]}>{mode === "wheel" ? "Vòng quay đang chậm lại…" : "Các bạn vịt đang về đích…"}</Text> : null}
  </View>;
}

const styles = StyleSheet.create({
  game: { alignItems: "center", gap: 8, paddingVertical: 8, maxWidth: "100%" }, object: { alignItems: "center", maxWidth: "100%" },
  nep: { position: "absolute", width: 58, height: 58 },
  pointer: { position: "absolute", top: 2, borderLeftWidth: 9, borderRightWidth: 9, borderTopWidth: 23, borderLeftColor: "transparent", borderRightColor: "transparent" },
  pond: { borderRadius: 18, overflow: "hidden", backgroundColor: "#176A73" },
  runner: { position: "absolute" }, wake: { position: "absolute" },
  badge: { position: "absolute", left: -6, bottom: 0, width: 21, height: 21, borderRadius: 11, borderWidth: 2, alignItems: "center", justifyContent: "center" },
  badgeText: { fontSize: 12, lineHeight: 15, fontWeight: "700" },
  status: { textAlign: "left", alignSelf: "stretch", maxWidth: "100%" },
});
