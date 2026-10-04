/**
 * Picking a day (ADR-0037 D1, plan S0.5): a tear-off calendar leaf, or the
 * typed date for someone who would rather type. Either way the value is the
 * `dd/mm/yyyy` string the existing validators read (`ngayVeISO`), so a screen
 * that switches to this changes nothing downstream.
 *
 * The month opens IN PLACE under the leaf (a sheet inside a scrolling page
 * would rise below the viewport): seven columns, Monday first, today ringed,
 * the chosen day filled. Every day is a button with its full spoken date.
 *
 * The columns share the width the month actually has, up to 48 dp each, and
 * every row is 48 dp tall: seven fixed 44 dp columns plus the panel's edges
 * came to 326 dp, wider than a 320 dp phone's 288 dp column (B11).
 */
import { Ionicons } from "@expo/vector-icons";
import { useState } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";
import Animated, { FadeIn } from "react-native-reanimated";

import { displayFace, typography, useRudiTheme } from "../theme";
import { ONhapMuc } from "./ONhapMuc";
import { THU_NGAN, cungNgay, dinhDangNgay, docNgay, luoiThang, tenThang, tenThu, thangSau, type NgayLich } from "./lich/lich-thang";
import { useMotion } from "./useMotion";
import { giuState } from "../../ui/a11y";

function homNayThat(): NgayLich {
  const d = new Date();
  return { ngay: d.getDate(), thang: d.getMonth() + 1, nam: d.getFullYear() };
}

export interface ChonNgayLichProps {
  /** `dd/mm/yyyy`, or "" for none yet. */
  giaTri: string;
  onChange: (giaTri: string) => void;
  /** What the day is for, spoken and printed: «Ngày đi». */
  nhan: string;
  /** `to-lich`: a calendar leaf; `dong`: a typed line with a calendar beside it. */
  kieu?: "to-lich" | "dong";
  /** The typed field's accessibility label (Maestro types into it): «Ô ngày đi». */
  oLabel?: string;
  homNay?: NgayLich;
  testID?: string;
}

export function ChonNgayLich({ giaTri, onChange, nhan, kieu = "to-lich", oLabel, homNay, testID }: ChonNgayLichProps) {
  const { colors, radius } = useRudiTheme();
  const motion = useMotion();
  const hn = homNay ?? homNayThat();
  const ngay = docNgay(giaTri);
  const [mo, setMo] = useState(false);
  const [rongLuoi, setRongLuoi] = useState(7 * O_TOI_DA);
  const o = Math.max(1, Math.min(O_TOI_DA, Math.floor(rongLuoi / 7)));
  const [xem, setXem] = useState({ thang: ngay?.thang ?? hn.thang, nam: ngay?.nam ?? hn.nam });
  const moLich = () => {
    setXem({ thang: ngay?.thang ?? hn.thang, nam: ngay?.nam ?? hn.nam });
    setMo((cu) => !cu);
  };
  const noiRo = ngay ? `${nhan}: ${tenThu(ngay)}, ${ngay.ngay} tháng ${ngay.thang} năm ${ngay.nam}` : `${nhan}: chưa chọn`;
  return (
    <View style={styles.khoi} testID={testID}>
      {kieu === "to-lich" ? (
        <Pressable
          accessibilityHint="Mở lịch để chọn ngày"
          accessibilityLabel={noiRo}
          accessibilityRole="button"
          aria-expanded={mo}
          onPress={moLich}
          style={({ pressed }) => [styles.la, { borderColor: colors.lineStrong, backgroundColor: colors.card, borderRadius: radius.small, opacity: pressed ? 0.85 : 1 }]}
          testID={testID ? `${testID}-la` : undefined}
        >
          <View style={[styles.bangLa, { backgroundColor: colors.accent }]}>
            <Text style={[typography.stamp, { color: colors.accentInk }]}>{ngay ? `Th ${ngay.thang}` : nhan}</Text>
          </View>
          <Text style={[styles.so, { color: ngay ? colors.ink : colors.inkFaint }]}>{ngay ? String(ngay.ngay) : "?"}</Text>
          <Text numberOfLines={1} style={[typography.caption, { color: colors.inkSoft }]}>
            {ngay ? tenThu(ngay) : "Chọn ngày"}
          </Text>
        </Pressable>
      ) : (
        <ONhapMuc
          accessibilityLabel={oLabel ?? nhan}
          autoCorrect={false}
          keyboardType="numbers-and-punctuation"
          label={nhan}
          onChangeText={onChange}
          placeholder="dd/mm/yyyy"
          trailing={
            <Pressable accessibilityLabel={`Mở lịch: ${nhan}`} accessibilityRole="button" hitSlop={6} onPress={moLich} style={styles.nutLich}>
              <Ionicons color={colors.inkSoft} name="calendar-outline" size={22} />
            </Pressable>
          }
          value={giaTri}
        />
      )}
      {mo ? (
        <Animated.View entering={FadeIn.duration(motion.ms("standard")).reduceMotion(motion.reanimated)} style={[styles.thang, { backgroundColor: colors.card, borderColor: colors.lineStrong, borderRadius: radius.small }]}>
          <View style={styles.dauThang}>
            <Pressable accessibilityLabel="Tháng trước" accessibilityRole="button" onPress={() => setXem((x) => thangSau(x.thang, x.nam, -1))} style={styles.nutThang}>
              <Ionicons color={colors.ink} name="chevron-back" size={20} />
            </Pressable>
            <Text accessibilityRole="header" style={[typography.title, { color: colors.ink }]}>
              {tenThang(xem.thang, xem.nam)}
            </Text>
            <Pressable accessibilityLabel="Tháng sau" accessibilityRole="button" onPress={() => setXem((x) => thangSau(x.thang, x.nam, 1))} style={styles.nutThang}>
              <Ionicons color={colors.ink} name="chevron-forward" size={20} />
            </Pressable>
          </View>
          <View onLayout={(e) => setRongLuoi(e.nativeEvent.layout.width)} style={styles.hang}>
            {THU_NGAN.map((t) => (
              <Text importantForAccessibility="no" key={t} style={[typography.caption, styles.thu, { width: o, color: colors.inkSoft }]}>
                {t}
              </Text>
            ))}
          </View>
          <View style={[styles.luoi, { width: o * 7 }]}>
            {luoiThang(xem.thang, xem.nam).map((d) => {
              const chon = cungNgay(d, ngay);
              const laHomNay = cungNgay(d, hn);
              return (
                <Pressable
                  accessibilityLabel={`${tenThu(d)}, ${d.ngay} tháng ${d.thang} năm ${d.nam}${laHomNay ? ", hôm nay" : ""}`}
                  accessibilityRole="button"
                  {...giuState(chon)}
                  aria-selected={chon}
                  key={`${d.nam}-${d.thang}-${d.ngay}`}
                  onPress={() => {
                    onChange(dinhDangNgay(d));
                    setMo(false);
                  }}
                  style={[styles.o, { width: o }]}
                >
                  <View style={[styles.oTron, { width: Math.min(38, o - 2), height: Math.min(38, o - 2), borderRadius: 19 }, chon ? { backgroundColor: colors.accent } : laHomNay ? { borderWidth: 1.5, borderColor: colors.ink } : null]}>
                    <Text style={[typography.body, { color: chon ? colors.accentInk : d.trongThang ? colors.ink : colors.inkFaint }]}>{d.ngay}</Text>
                  </View>
                </Pressable>
              );
            })}
          </View>
        </Animated.View>
      ) : null}
    </View>
  );
}

/** A day column's widest; the month's own width decides below it. */
const O_TOI_DA = 48;

const styles = StyleSheet.create({
  khoi: { gap: 8 },
  la: { width: 84, minHeight: 96, borderWidth: 1, alignItems: "center", overflow: "hidden", paddingBottom: 8 },
  bangLa: { alignSelf: "stretch", alignItems: "center", paddingVertical: 4 },
  so: { fontFamily: displayFace.extraBold, fontSize: 34, lineHeight: 40 },
  nutLich: { width: 48, height: 48, alignItems: "center", justifyContent: "center" },
  thang: { borderWidth: 1, padding: 8, gap: 4, alignSelf: "stretch", maxWidth: O_TOI_DA * 7 + 18 },
  dauThang: { flexDirection: "row", alignItems: "center", justifyContent: "space-between" },
  nutThang: { width: 48, height: 48, alignItems: "center", justifyContent: "center" },
  hang: { flexDirection: "row", alignSelf: "stretch" },
  thu: { textAlign: "center" },
  luoi: { flexDirection: "row", flexWrap: "wrap" },
  o: { height: 48, alignItems: "center", justifyContent: "center" },
  oTron: { alignItems: "center", justifyContent: "center" },
});
