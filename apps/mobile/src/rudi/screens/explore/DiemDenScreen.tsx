/**
 * Chọn điểm đến (M10): which city Khám phá is showing.
 *
 * The catalogue spans fifteen destinations, so the app has to ask. The list is
 * the server's; the choice is this phone's (AsyncStorage), because it is
 * browsing state rather than a fact about the person.
 *
 * «Gần tôi» is not on this screen yet: reading the device's position needs a
 * native module and a permission, and that arrives with its own build and its
 * own explanation screen (ADR-0018). What is here is the honest half -- pick a
 * city by name -- and it works with no permission at all.
 *
 * UI v2: rows on the paper with a hairline between them, the chosen city
 * marked by a check as well as by colour; loading is the list's own shape.
 */
import { Ionicons } from "@expo/vector-icons";
import { useRouter } from "expo-router";
import { useCallback, useEffect, useState } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";

import { ApiError, thongDiepNguoiDoc } from "../../../api";
import {
  docDiemDen,
  docDiemDenDaChon,
  dongPhuDiemDen,
  luuDiemDen,
  type DiemDen,
} from "../../kham-pha/diem-den";
import { typography, useRudiTheme } from "../../theme";
import { RudiScreen, SearchField, TopBar } from "../../ui";
import { EmptyState } from "../../ui/EmptyState";
import { ErrorState } from "../../ui/ErrorState";
import { SkeletonGroup, SkeletonRow } from "../../ui/Skeleton";

type Trang =
  | { pha: "dang-doc" }
  | { pha: "xong"; ds: DiemDen[] }
  | { pha: "hong"; loi: string };

function khongDau(s: string): string {
  return s.normalize("NFD").replace(/[̀-ͯ]/g, "").replace(/đ/gi, "d").toLowerCase();
}

export function DiemDenScreen() {
  const router = useRouter();
  const { colors } = useRudiTheme();
  const [trang, setTrang] = useState<Trang>({ pha: "dang-doc" });
  const [tim, setTim] = useState("");
  const [dangChon, setDangChon] = useState<string | null>(null);

  const nap = useCallback(async () => {
    setTrang({ pha: "dang-doc" });
    try {
      const [ds, daChon] = await Promise.all([docDiemDen(), docDiemDenDaChon()]);
      setDangChon(daChon);
      setTrang({ pha: "xong", ds: ds.diemDen });
    } catch (error) {
      setTrang({
        pha: "hong",
        loi: error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null),
      });
    }
  }, []);

  useEffect(() => {
    void nap();
  }, [nap]);

  const chon = async (diemDen: DiemDen) => {
    setDangChon(diemDen.id);
    await luuDiemDen(diemDen.id);
    router.back();
  };

  const loc =
    trang.pha === "xong"
      ? trang.ds.filter((d) => {
          const q = khongDau(tim.trim());
          if (q === "") return true;
          return khongDau(`${d.name} ${d.province ?? ""}`).includes(q);
        })
      : [];

  return (
    <RudiScreen testID="diem-den-screen">
      <TopBar title="Đi đâu?" />
      <SearchField
        onChangeText={setTim}
        placeholder="Tìm thành phố hoặc tỉnh"
        value={tim}
      />
      {trang.pha === "dang-doc" ? (
        <SkeletonGroup>
          <SkeletonRow leading={0} />
          <SkeletonRow leading={0} />
          <SkeletonRow leading={0} />
          <SkeletonRow leading={0} />
        </SkeletonGroup>
      ) : null}
      {trang.pha === "hong" ? <ErrorState body={trang.loi} onRetry={() => void nap()} title="Chưa đọc được danh sách nơi đến" /> : null}
      {trang.pha === "xong" && loc.length === 0 ? (
        <EmptyState
          action={{ label: "Xóa ô tìm", onPress: () => setTim("") }}
          body="Rủ Đi mới biết mười lăm nơi. Thử tên khác, hoặc xoá ô tìm để xem hết."
          kind="no-results"
          layout="inline"
          title="Chưa có nơi nào khớp"
        />
      ) : null}
      {loc.length > 0 ? (
        <View>
          {loc.map((d) => {
            const chonRoi = dangChon === d.id;
            return (
              <Pressable
                accessibilityLabel={`Chọn ${d.name}`}
                accessibilityRole="button"
                accessibilityState={{ selected: chonRoi }}
                key={d.id}
                onPress={() => void chon(d)}
                style={({ pressed }) => [styles.hang, { borderBottomColor: colors.line }, pressed && styles.bam]}
              >
                <View style={styles.hangChu}>
                  <Text style={[typography.title, { color: chonRoi ? colors.accent : colors.ink }]}>{d.name}</Text>
                  <Text style={[typography.caption, { color: colors.inkFaint }]}>{dongPhuDiemDen(d)}</Text>
                  {d.blurb === null ? null : (
                    <Text numberOfLines={2} style={[typography.caption, { color: colors.inkSoft }]}>
                      {d.blurb}
                    </Text>
                  )}
                </View>
                {/* Chosen is a check as well as a colour. */}
                {chonRoi ? <Ionicons color={colors.accent} name="checkmark-circle" size={22} /> : <Ionicons color={colors.inkFaint} name="chevron-forward" size={18} />}
              </Pressable>
            );
          })}
        </View>
      ) : null}
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  hang: { minHeight: 64, flexDirection: "row", alignItems: "center", gap: 12, paddingVertical: 12, borderBottomWidth: StyleSheet.hairlineWidth },
  hangChu: { flex: 1, gap: 3 },
  bam: { opacity: 0.7 },
});
