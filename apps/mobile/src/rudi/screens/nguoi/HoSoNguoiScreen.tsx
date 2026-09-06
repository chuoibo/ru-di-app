/**
 * Somebody else's profile and wall (M8): `/people/{id}`.
 *
 * Two independent reads. The profile is the gate -- a refusal there is the
 * whole screen, because there is nothing honest to draw without it. The wall
 * is loaded after and fails on its own, so a wall that does not answer does
 * not hide a person who did.
 *
 * UI v2 (đợt 7): initial, name, when they joined, the relation as a word;
 * posts are rows on the paper with a hairline between them.
 */
import { useFocusEffect, useLocalSearchParams, useRouter } from "expo-router";
import { useCallback, useState } from "react";
import { StyleSheet, Text, View } from "react-native";

import {
  cauNgayVao,
  cauQuanHe,
  cauTuongRong,
  docHoSoNguoi,
  docTuongCua,
  dongPhuBai,
  loiRaChu,
  type Bai,
  type HoSoNguoi,
} from "../../nguoi/ho-so-nguoi";
import { useRudiSession } from "../../session";
import { typography, useRudiTheme } from "../../theme";
import { Chip, Heading, RudiButton, RudiScreen, TopBar } from "../../ui";
import { Avatar } from "../../ui/Avatar";
import { EmptyState } from "../../ui/EmptyState";
import { ErrorState } from "../../ui/ErrorState";
import { SkeletonGroup, SkeletonLines, SkeletonRow } from "../../ui/Skeleton";

type TrangHoSo =
  | { pha: "dang-doc" }
  | { pha: "xong"; hoSo: HoSoNguoi }
  | { pha: "hong"; loi: string };

type TrangTuong =
  | { pha: "dang-doc" }
  | { pha: "xong"; bai: Bai[] }
  | { pha: "hong"; loi: string };

export function HoSoNguoiScreen() {
  const router = useRouter();
  const { colors } = useRudiTheme();
  const { phien, phienDaDoc } = useRudiSession();
  const params = useLocalSearchParams<{ id?: string }>();
  // Written as a statement, not `x ? x : ""`: the id-default scanner reads that
  // shape as a display fallback wherever it appears, and it is right to.
  let personId = "";
  if (typeof params.id === "string") personId = params.id;
  const [hoSo, setHoSo] = useState<TrangHoSo>({ pha: "dang-doc" });
  const [tuong, setTuong] = useState<TrangTuong>({ pha: "dang-doc" });

  const napHoSo = useCallback(async () => {
    if (phien === null || personId === "") return;
    setHoSo({ pha: "dang-doc" });
    try {
      setHoSo({ pha: "xong", hoSo: await docHoSoNguoi(personId, phien.person_id) });
    } catch (error) {
      setHoSo({ pha: "hong", loi: loiRaChu(error) });
    }
  }, [personId, phien]);

  const napTuong = useCallback(async () => {
    if (phien === null || personId === "") return;
    setTuong({ pha: "dang-doc" });
    try {
      setTuong({ pha: "xong", bai: await docTuongCua(personId, phien.person_id) });
    } catch (error) {
      setTuong({ pha: "hong", loi: loiRaChu(error) });
    }
  }, [personId, phien]);

  useFocusEffect(
    useCallback(() => {
      void napHoSo();
      void napTuong();
    }, [napHoSo, napTuong]),
  );

  if (!phienDaDoc) return null;

  return (
    <RudiScreen testID="ho-so-nguoi-screen">
      <TopBar title="Hồ sơ" />
      {hoSo.pha === "dang-doc" ? (
        <SkeletonGroup>
          <SkeletonRow leading={60} />
        </SkeletonGroup>
      ) : null}
      {hoSo.pha === "hong" ? <ErrorState body={hoSo.loi} onRetry={() => void napHoSo()} title="Chưa mở được hồ sơ" /> : null}
      {hoSo.pha === "xong" ? (
        <>
          <View style={styles.hoSo}>
            <View style={styles.dau}>
              <Avatar name={hoSo.hoSo.display_name} size={60} />
              <View style={styles.dauChu}>
                <Text numberOfLines={2} style={[typography.h2, { color: colors.ink }]}>
                  {hoSo.hoSo.display_name}
                </Text>
                <Text style={[typography.caption, { color: colors.inkFaint }]}>
                  {cauNgayVao(hoSo.hoSo.created_at)}
                </Text>
              </View>
            </View>
            <View style={styles.chips}>
              <Chip label={cauQuanHe(hoSo.hoSo.relation)} />
              {hoSo.hoSo.city ? <Chip icon="location-outline" label={hoSo.hoSo.city} /> : null}
            </View>
            {hoSo.hoSo.bio ? (
              <Text style={[typography.body, { color: colors.inkSoft }]}>{hoSo.hoSo.bio}</Text>
            ) : (
              <Text style={[typography.caption, { color: colors.inkFaint }]}>
                {hoSo.hoSo.relation === "self"
                  ? "Bạn chưa viết giới thiệu. Sửa ở Cá nhân."
                  : "Người này chưa viết giới thiệu."}
              </Text>
            )}
            {hoSo.hoSo.relation === "self" ? (
              <RudiButton
                compact
                full={false}
                icon="create-outline"
                label="Đăng bài mới"
                onPress={() => router.push("/posts/new")}
              />
            ) : null}
          </View>
          <Heading title={hoSo.hoSo.relation === "self" ? "Tường của bạn" : "Tường cá nhân"} />
          {tuong.pha === "dang-doc" ? (
            <SkeletonGroup>
              <SkeletonLines lines={2} />
            </SkeletonGroup>
          ) : null}
          {tuong.pha === "hong" ? <ErrorState body={tuong.loi} onRetry={() => void napTuong()} title="Chưa đọc được tường" /> : null}
          {tuong.pha === "xong" && tuong.bai.length === 0 ? (
            <EmptyState kind="first-use" layout="inline" title={cauTuongRong(hoSo.hoSo.relation)} />
          ) : null}
          {tuong.pha === "xong" && tuong.bai.length > 0 ? (
            <View>
              {tuong.bai.map((bai) => (
                <View key={bai.id} style={[styles.bai, { borderBottomColor: colors.line }]}>
                  <Text style={[typography.body, { color: colors.ink }]}>{bai.body}</Text>
                  <Text style={[typography.caption, { color: colors.inkFaint }]}>
                    {dongPhuBai(bai)}
                  </Text>
                </View>
              ))}
            </View>
          ) : null}
        </>
      ) : null}
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  hoSo: { gap: 10 },
  dau: { flexDirection: "row", alignItems: "center", gap: 14 },
  dauChu: { flex: 1, gap: 2 },
  chips: { flexDirection: "row", flexWrap: "wrap", gap: 8 },
  bai: { gap: 6, paddingVertical: 12, borderBottomWidth: StyleSheet.hairlineWidth },
});
