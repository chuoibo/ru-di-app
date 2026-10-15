import { useRef, useState } from "react";
import { StyleSheet, Text, View, type GestureResponderEvent } from "react-native";
import { dinhDangTienVnd } from "../../screens/chat/ke-hoach";
import { BUOC_NGAN_SACH, TRAN_THANH_NGAN_SACH, dichNganSach, docNganSach, nganSachTuViTri } from "../keo/ngan-sach-keo";
import { typography, useRudiTheme } from "../theme";
import { PressScale } from "./PressScale";
import { useMotion } from "./useMotion";
import { Money } from "./Money";

const NHANH = [200000, 500000, 1000000];

/** One adjustable native target, with exact entry supplied by its parent. */
export function ThanhNganSach({ value, onChange, disabled = false }: { value: string; onChange: (next: string) => void; disabled?: boolean }) {
  const { colors } = useRudiTheme();
  const motion = useMotion();
  const [width, setWidth] = useState(0);
  const amount = docNganSach(value);
  const last = useRef(amount);
  const lastHaptic = useRef(0);
  last.current = amount;
  const fraction = Math.min(1, (amount ?? 0) / TRAN_THANH_NGAN_SACH);
  const outside = amount !== null && amount > TRAN_THANH_NGAN_SACH;
  const select = (next: number) => {
    if (disabled || last.current === next) return;
    last.current = next;
    if (Date.now() - lastHaptic.current >= 80) { motion.haptic.select(); lastHaptic.current = Date.now(); }
    onChange(String(next));
  };
  const drag = (e: GestureResponderEvent) => {
    const next = nganSachTuViTri(e.nativeEvent.locationX - 14, width - 28);
    if (next !== null) select(next);
  };
  return <View style={styles.block}>
    <View style={styles.valueLine}>
      {amount === null ? <Text style={[typography.title, { color: colors.inkSoft }]}>Chọn mức chi</Text> : <Money size="money" vnd={amount} numberOfLines={0} style={styles.amount} />}
      {amount === null ? null : <Text style={[typography.note, { color: colors.inkSoft }]}>/ người</Text>}
    </View>
    {outside ? <Text accessibilityLiveRegion="polite" style={[typography.note, { color: colors.inkSoft }]}>Ngoài thang kéo 0–2 triệu. Giữ nguyên số bạn nhập.</Text> : null}
    <View
      testID="outing-budget-slider"
      accessible
      accessibilityRole="adjustable"
      accessibilityLabel="Ngân sách mỗi người"
      accessibilityHint="Vuốt ngang để chọn, mỗi bước 25 nghìn đồng. Hoặc nhập số chính xác bên dưới."
      accessibilityState={{ disabled }}
      accessibilityValue={{ min: 0, max: Math.max(TRAN_THANH_NGAN_SACH, amount ?? 0), now: amount ?? 0, text: amount === null ? "Chưa chọn" : `${dinhDangTienVnd(amount)} mỗi người${outside ? ", ngoài thang kéo" : ""}` }}
      accessibilityActions={[{ name: "increment", label: "Tăng 25 nghìn" }, { name: "decrement", label: "Giảm 25 nghìn" }]}
      onAccessibilityAction={(e) => { if (e.nativeEvent.actionName === "increment" || e.nativeEvent.actionName === "decrement") select(dichNganSach(amount, e.nativeEvent.actionName === "increment" ? 1 : -1)); }}
      onLayout={(e) => setWidth(e.nativeEvent.layout.width)}
      onStartShouldSetResponder={() => !disabled}
      onMoveShouldSetResponder={() => !disabled}
      onResponderGrant={drag}
      onResponderMove={drag}
      style={styles.touch}
    >
      <View pointerEvents="none" style={[styles.track, { backgroundColor: colors.lineStrong }]}>
        <View style={[styles.fill, { width: `${fraction * 100}%`, backgroundColor: colors.accent }]} />
        {[0, .25, .5, .75, 1].map((position) => <View key={position} style={[styles.tick, { left: `${position * 100}%`, backgroundColor: position <= fraction ? colors.accentInk : colors.card }]} />)}
        <View style={[styles.thumb, { left: `${fraction * 100}%`, borderColor: colors.accent, backgroundColor: colors.card }]}><View style={[styles.thumbDot, outside && styles.outsideMark, { backgroundColor: colors.accent }]} /></View>
      </View>
    </View>
    <View style={styles.scale} accessible={false} importantForAccessibility="no-hide-descendants"><Text style={[typography.note, { color: colors.inkSoft }]}>0đ</Text><Text style={[typography.note, { color: colors.inkSoft }]}>1 triệu</Text><Text style={[typography.note, { color: colors.inkSoft }]}>2 triệu</Text></View>
    <View style={styles.presets}>{NHANH.map((next) => <PressScale key={next} accessibilityRole="button" accessibilityLabel={next === 1000000 ? "1 triệu" : `${next / 1000} nghìn`} accessibilityState={{ selected: amount === next, disabled }} disabled={disabled} onPress={() => select(next)} style={[styles.preset, { borderColor: amount === next ? colors.accent : colors.line, backgroundColor: amount === next ? colors.accentSoft : colors.card }]}><Text style={[typography.label, { color: amount === next ? colors.accent : colors.inkSoft }]}>{next === 1000000 ? "1 triệu" : `${next / 1000} nghìn`}</Text></PressScale>)}</View>
    <Text style={[typography.note, { color: colors.inkSoft }]}>Kéo để chọn · bước {dinhDangTienVnd(BUOC_NGAN_SACH)}</Text>
  </View>;
}

const styles = StyleSheet.create({
  block: { gap: 4 },
  valueLine: { flexDirection: "row", flexWrap: "wrap", alignItems: "baseline", columnGap: 6, rowGap: 2 },
  amount: { maxWidth: "100%", flexShrink: 1 },
  touch: { height: 56, justifyContent: "center", paddingHorizontal: 14 },
  track: { height: 6, borderRadius: 3 },
  fill: { height: 6, borderRadius: 3 },
  tick: { position: "absolute", top: 2, width: 2, height: 2, borderRadius: 1, marginLeft: -1 },
  thumb: { position: "absolute", top: -11, marginLeft: -14, width: 28, height: 28, borderRadius: 14, borderWidth: 2, alignItems: "center", justifyContent: "center" },
  thumbDot: { width: 8, height: 8, borderRadius: 4 },
  outsideMark: { width: 10, height: 3, borderRadius: 1 },
  scale: { flexDirection: "row", justifyContent: "space-between", marginBottom: 10 },
  presets: { flexDirection: "row", flexWrap: "wrap", gap: 8 },
  preset: { flexGrow: 1, minHeight: 48, borderWidth: 1, borderRadius: 12, paddingHorizontal: 10, paddingVertical: 12, alignItems: "center", justifyContent: "center" },
});
