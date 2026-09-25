/**
 * A dial for a time of day (ADR-0037 D1, D7, plan S0.5): turn the hand to the
 * hour in quarter hours instead of typing «18:30» into a box. A 12-hour face
 * turned twice; the time in the middle is 24-hour words, so which half of the
 * day is never ambiguous.
 *
 * Three doors to one value (D7): drag the hand round (it snaps to quarter
 * hours, with a light tick each time it passes one); a screen reader's
 * increment and decrement (±15 minutes, role `adjustable`, the time spoken as
 * words); or tap the middle to TYPE the time (`oLabel`, the field Maestro and
 * a keyboard use). The dial is a control: it never animates by itself.
 */
import { useState } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";
import { Gesture, GestureDetector } from "react-native-gesture-handler";
import { runOnJS } from "react-native-reanimated";
import Svg, { Circle, Line } from "react-native-svg";

import { displayFace, typography, useRudiTheme } from "../theme";
import { ONhapMuc } from "./ONhapMuc";
import { PHUT_MOI_NAC, docGio, docGioThanhLoi, gioTuNac, gocTuCham, gocTuNac, nacGio, nacTiep, nacTuGoc } from "./ban-xoay";
import { useMotion } from "./useMotion";

const NAC_MOT_VONG = 48;

export function BanXoay({
  phut,
  onChange,
  nhan,
  oLabel,
  co = 184,
  testID,
}: {
  /** Minutes from midnight, or null for none yet. */
  phut: number | null;
  onChange: (phut: number) => void;
  /** What the time is for: «Giờ chặng». */
  nhan: string;
  /** The typed field's accessibility label: «Ô giờ chặng». */
  oLabel?: string;
  co?: number;
  testID?: string;
}) {
  const { colors } = useRudiTheme();
  const motion = useMotion();
  const [nac, setNac] = useState(() => nacGio(phut ?? 18 * 60));
  const [go, setGo] = useState(false);
  const [chuGo, setChuGo] = useState("");
  const r = co / 2;
  const coGio = phut !== null;

  const datNac = (moi: number) => {
    if (moi === nac) return;
    setNac(moi);
    motion.haptic.select();
    onChange((((moi % 96) + 96) % 96) * PHUT_MOI_NAC);
  };
  const theoTay = (x: number, y: number) => {
    // Near the middle the angle means nothing (and the middle is the «type
    // it» button): only a finger on the face turns the hand.
    if (Math.hypot(x - r, y - r) < r * 0.3) return;
    const goc = gocTuCham({ x: r, y: r }, { x, y });
    datNac(nacTiep(nac, nacTuGoc(goc, NAC_MOT_VONG), NAC_MOT_VONG));
  };
  // A drag, not a touch: a tap on the middle must reach the «type it» button.
  const keo = Gesture.Pan()
    .minDistance(4)
    .onUpdate((e) => {
      runOnJS(theoTay)(e.x, e.y);
    });

  const gocKim = (gocTuNac(nac, NAC_MOT_VONG) * Math.PI) / 180;
  const dauKim = { x: r + Math.sin(gocKim) * (r - 22), y: r - Math.cos(gocKim) * (r - 22) };
  const vach = Array.from({ length: NAC_MOT_VONG }, (_, i) => {
    const g = (i * 2 * Math.PI) / NAC_MOT_VONG;
    const dai = i % 4 === 0 ? 10 : 5;
    return { x1: r + Math.sin(g) * (r - 6), y1: r - Math.cos(g) * (r - 6), x2: r + Math.sin(g) * (r - 6 - dai), y2: r - Math.cos(g) * (r - 6 - dai), dam: i % 4 === 0 };
  });

  return (
    <View style={styles.khoi} testID={testID}>
      <Text style={[typography.caption, { color: colors.inkSoft }]}>{nhan}</Text>
      <GestureDetector gesture={keo}>
        <View
          accessibilityActions={[{ name: "increment" }, { name: "decrement" }]}
          accessibilityLabel={nhan}
          accessibilityRole="adjustable"
          accessibilityValue={{ text: coGio ? docGioThanhLoi(nac) : "chưa chọn" }}
          accessible
          onAccessibilityAction={(e) => {
            if (e.nativeEvent.actionName === "increment") datNac(nac + 1);
            else if (e.nativeEvent.actionName === "decrement") datNac(nac - 1);
          }}
          style={{ width: co, height: co }}
        >
          <Svg height={co} pointerEvents="none" width={co}>
            <Circle cx={r} cy={r} fill={colors.card} r={r - 1} stroke={colors.lineStrong} strokeWidth={1.5} />
            {vach.map((v, i) => (
              <Line key={i} stroke={v.dam ? colors.ink : colors.inkFaint} strokeLinecap="round" strokeWidth={v.dam ? 2 : 1.2} x1={v.x1} x2={v.x2} y1={v.y1} y2={v.y2} />
            ))}
            <Line stroke={colors.accent} strokeLinecap="round" strokeWidth={3} x1={r} x2={dauKim.x} y1={r} y2={dauKim.y} />
            <Circle cx={dauKim.x} cy={dauKim.y} fill={colors.accent} r={6} />
            <Circle cx={r} cy={r} fill={colors.card} r={r * 0.36} stroke={colors.line} strokeWidth={1} />
          </Svg>
          <Pressable
            accessibilityLabel={`Gõ giờ: ${nhan}`}
            accessibilityRole="button"
            onPress={() => {
              setChuGo(coGio ? gioTuNac(nac) : "");
              setGo((cu) => !cu);
            }}
            style={[styles.giua, { left: r - r * 0.36, top: r - r * 0.36, width: r * 0.72, height: r * 0.72, borderRadius: r * 0.36 }]}
          >
            <Text style={[styles.gio, { color: coGio ? colors.ink : colors.inkFaint }]}>{coGio ? gioTuNac(nac) : "--:--"}</Text>
          </Pressable>
        </View>
      </GestureDetector>
      {go ? (
        <ONhapMuc
          accessibilityLabel={oLabel ?? nhan}
          autoFocus
          keyboardType="numbers-and-punctuation"
          onChangeText={(t) => {
            setChuGo(t);
            const p = docGio(t);
            if (p !== null) {
              setNac(nacGio(p));
              onChange(p);
            }
          }}
          placeholder="18:30"
          value={chuGo}
        />
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  khoi: { gap: 8, alignItems: "flex-start" },
  giua: { position: "absolute", alignItems: "center", justifyContent: "center" },
  gio: { fontFamily: displayFace.extraBold, fontSize: 24, lineHeight: 28, fontVariant: ["tabular-nums"] },
});
