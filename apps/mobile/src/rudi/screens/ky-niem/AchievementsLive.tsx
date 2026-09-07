/**
 * Thành tích on a real session (M6). There is no achievements route: every
 * badge, level and weekly challenge here is App B's `thanh-tich.ts` over
 * `GET /people/{id}/finance`, so each one is explainable from the ledger and
 * nothing is awarded on the phone's say-so. A badge the ledger cannot decide
 * yet says "chưa đo được" rather than pretending to be locked.
 *
 * UI v2 (đợt 7): one badge is put in the spotlight (the first the ledger
 * has opened) with its rule; the rest are rows with the state as a word, so
 * a locked badge is still readable and a badge is remembered for what it
 * means, not for a coloured tile. No XP, streak or ranking beyond the sums.
 */
import { Ionicons } from "@expo/vector-icons";
import { useEffect, useState } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";

import type { Phien } from "../../../phien";
import { FinanceError } from "../../../screens/ca-nhan/tai-chinh";
import { phanSo, tiLe } from "../../../screens/thanh-tich/thanh-tich";
import { demHuyHieuMo, docThanhTich, type HuyHieu, type ThanhTich } from "../../ky-niem/ky-niem";
import { displayFace, typography, useRudiTheme } from "../../theme";
import { RudiScreen, SectionHeader, TopBar, type IconName } from "../../ui";
import { ErrorState } from "../../ui/ErrorState";
import { SkeletonGroup, SkeletonLines, SkeletonRow } from "../../ui/Skeleton";
import { Stamp } from "../../ui/Stamp";

type Trang = { pha: "dang-doc" } | { pha: "xong"; tt: ThanhTich } | { pha: "hong"; loi: string };

function loiRaChu(error: unknown): string {
  if (error instanceof FinanceError) return error.message;
  if (error instanceof Error && error.message !== "") return error.message;
  return "Chưa đọc được sổ để tính thành tích.";
}

function chuTrangThai(h: HuyHieu): string {
  if (h.trangThai === "mo") return "Đã mở";
  if (h.trangThai === "chua-do-duoc") return "Chưa đo được";
  if (h.daDat !== undefined && h.can !== undefined) return `${phanSo(h.daDat, h.can)}`;
  return "Chưa đạt";
}

/** Three states, three glyphs: a badge the ledger cannot decide yet is not a locked one. */
function bieuTuongHuyHieu(h: HuyHieu): IconName {
  if (h.trangThai === "mo") return "ribbon";
  if (h.trangThai === "chua-do-duoc") return "help-circle-outline";
  return "lock-closed-outline";
}

/** The rule, plus what is missing, in a member's words; the shared "chưa đo được" reason is said once under the list. */
function phuHuyHieu(h: HuyHieu): string {
  if (h.trangThai === "chua-do-duoc") return h.dieuKien;
  if (h.thieuGi !== undefined && h.thieuGi !== "") return `${h.dieuKien} · ${h.thieuGi}`;
  return h.dieuKien;
}

export function AchievementsLiveScreen({ phien }: { phien: Phien }) {
  const { colors, radius } = useRudiTheme();
  const [moCachTinh, setMoCachTinh] = useState(false);
  const [trang, setTrang] = useState<Trang>({ pha: "dang-doc" });

  const doc = async () => {
    try {
      setTrang({ pha: "xong", tt: await docThanhTich(phien.person_id) });
    } catch (error) {
      setTrang({ pha: "hong", loi: loiRaChu(error) });
    }
  };
  useEffect(() => {
    let song = true;
    void docThanhTich(phien.person_id)
      .then((tt) => {
        if (song) setTrang({ pha: "xong", tt });
      })
      .catch((error: unknown) => {
        if (song) setTrang({ pha: "hong", loi: loiRaChu(error) });
      });
    return () => {
      song = false;
    };
  }, [phien.person_id]);

  if (trang.pha === "dang-doc") {
    return (
      <RudiScreen testID="achievements-screen">
        <TopBar title="Thành tích" />
        <SkeletonGroup style={styles.khung}>
          <SkeletonLines lastWidth="50%" lineHeight={24} lines={2} />
          <SkeletonRow />
          <SkeletonRow />
        </SkeletonGroup>
      </RudiScreen>
    );
  }
  if (trang.pha === "hong") {
    return (
      <RudiScreen testID="achievements-screen">
        <TopBar title="Thành tích" />
        <ErrorState body={trang.loi} onRetry={() => void doc()} title="Chưa đọc được sổ để tính thành tích" />
      </RudiScreen>
    );
  }
  const { tienDo, huyHieu, thuThach, so } = trang.tt;
  const phanTram = Math.round(tiLe(tienDo.diemTrongCap, tienDo.diemMoiCap) * 100);
  const noiBat = huyHieu.find((h) => h.trangThai === "mo");
  return (
    <RudiScreen testID="achievements-screen">
      <TopBar title="Thành tích" />
      <View style={styles.dau}>
        <Text style={[styles.cap, { color: colors.ink }]}>Cấp {tienDo.cap}</Text>
        <Text style={[typography.body, { color: colors.inkSoft }]}>
          {tienDo.diemTrongCap}/{tienDo.diemMoiCap} điểm tới cấp {tienDo.cap + 1} · tính từ {so.expense_count} khoản chi, {so.group_count} nhóm trong sổ của bạn
        </Text>
        <View accessibilityLabel={`Tiến độ ${phanTram} phần trăm tới cấp sau`} style={[styles.thanh, { backgroundColor: colors.line, borderRadius: radius.pill }]}>
          <View style={[styles.thanhDay, { backgroundColor: colors.accent, borderRadius: radius.pill, width: `${phanTram}%` }]} />
        </View>
      </View>

      {noiBat !== undefined ? (
        // The one badge worth remembering today, and the rule that earned it.
        <View style={[styles.noiBat, { backgroundColor: colors.accentSoft, borderRadius: radius.base }]}>
          <View style={[styles.noiBatIcon, { backgroundColor: colors.card }]}>
            <Ionicons color={colors.accent} name="ribbon" size={30} />
          </View>
          <View style={styles.flex}>
            <Stamp label="Đã mở" />
            <Text style={[typography.h2, { color: colors.ink }]}>{noiBat.ten}</Text>
            <Text style={[typography.body, { color: colors.inkSoft }]}>{noiBat.dieuKien}</Text>
          </View>
        </View>
      ) : null}

      <SectionHeader title="Huy hiệu" />
      <Text style={[typography.note, { color: colors.inkSoft }]}>{demHuyHieuMo(huyHieu)}</Text>
      <View>
        {huyHieu.map((h) => {
          const mo = h.trangThai === "mo";
          return (
            <View key={h.id} style={[styles.hang, { borderBottomColor: colors.line }]}>
              <View style={[styles.hangIcon, { backgroundColor: mo ? colors.accentSoft : colors.card, borderColor: mo ? colors.accentSoft : colors.line }]}>
                <Ionicons color={mo ? colors.accent : colors.inkFaint} name={bieuTuongHuyHieu(h)} size={20} />
              </View>
              <View style={styles.flex}>
                <Text style={[typography.label, { color: colors.ink }]}>{h.ten}</Text>
                <Text style={[typography.caption, { color: colors.inkSoft }]}>{phuHuyHieu(h)}</Text>
              </View>
              {mo ? <Stamp label="Đã mở" /> : <Text style={[typography.caption, { color: colors.inkSoft }]}>{chuTrangThai(h)}</Text>}
            </View>
          );
        })}
      </View>

      <SectionHeader title="Thử thách tuần này" />
      {thuThach.length === 0 ? <Text style={[typography.caption, { color: colors.inkFaint }]}>Tuần này chưa có thử thách nào đo được từ sổ.</Text> : null}
      <View>
        {thuThach.map((t) => (
          <View key={t.id} style={[styles.hang, { borderBottomColor: colors.line }]}>
            <Ionicons color={t.xong ? colors.accent : colors.lineStrong} name={t.xong ? "checkmark-circle" : "ellipse-outline"} size={24} />
            <View style={styles.flex}>
              <Text style={[typography.label, { color: colors.ink }]}>{t.ten}</Text>
              <Text style={[typography.caption, { color: colors.inkSoft }]}>{phanSo(t.daDat, t.can)}{t.xong ? " · xong" : ""}</Text>
            </View>
          </View>
        ))}
      </View>
      {/* The rules stay true and stay available, but behind one word instead
          of standing under every list (report 07/09 §8.5). */}
      <Pressable
        accessibilityRole="button"
        accessibilityState={{ expanded: moCachTinh }}
        aria-expanded={moCachTinh}
        onPress={() => setMoCachTinh((v) => !v)}
        style={({ pressed }) => [styles.cachTinh, pressed && styles.bam]}
      >
        <Text style={[typography.label, { color: colors.inkSoft }]}>Cách tính</Text>
        <Ionicons color={colors.inkFaint} name={moCachTinh ? "chevron-up" : "chevron-down"} size={16} />
      </Pressable>
      {moCachTinh ? (
        <View style={styles.giaiThich}>
          <Text style={[typography.note, { color: colors.inkSoft }]}>Thành tích tính lại mỗi lần mở, từ đúng những gì có trong sổ. Không có điểm thưởng nào cấp ngoài sổ.</Text>
          {huyHieu.some((h) => h.trangThai === "chua-do-duoc") ? (
            <Text style={[typography.note, { color: colors.inkSoft }]}>«Chưa đo được»: sổ chưa ghi mục đó theo từng người, nên chưa có gì để đếm. Không phải khoá.</Text>
          ) : null}
        </View>
      ) : null}
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  flex: { flex: 1 },
  cachTinh: { flexDirection: "row", alignItems: "center", gap: 4, minHeight: 48, alignSelf: "flex-start" },
  giaiThich: { gap: 6 },
  bam: { opacity: 0.7 },
  khung: { gap: 14 },
  dau: { gap: 6 },
  cap: { fontFamily: displayFace.extraBold, fontSize: 34, lineHeight: 39, letterSpacing: -1 },
  thanh: { height: 8, overflow: "hidden", marginTop: 6 },
  thanhDay: { height: 8 },
  noiBat: { flexDirection: "row", alignItems: "center", gap: 14, padding: 16 },
  noiBatIcon: { width: 64, height: 64, borderRadius: 32, alignItems: "center", justifyContent: "center" },
  hang: { flexDirection: "row", alignItems: "center", gap: 12, minHeight: 60, paddingVertical: 8, borderBottomWidth: StyleSheet.hairlineWidth },
  hangIcon: { width: 40, height: 40, borderRadius: 20, borderWidth: 1, alignItems: "center", justifyContent: "center" },
});
