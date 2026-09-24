/**
 * One group's roster, and the door to invite somebody (M2).
 *
 * Reuses the legacy client module (`danhSachThanhVien`): the route is the
 * same one App B called, with the bearer now doing the identifying. Names
 * come with the roster (`display_name`), initials are drawn in one tone -- the
 * design system does not colour people.
 *
 * UI v2: rows on the paper, the group's two memory doors as plain rows, the
 * invite as the one action at the foot.
 */
import { Redirect, useFocusEffect, useLocalSearchParams, useRouter } from "expo-router";
import { useCallback, useRef, useState } from "react";
import { StyleSheet, Text, View } from "react-native";

import { ApiError, attemptFor, thongDiepNguoiDoc, type Attempt } from "../../../api";
import {
  coTheDoiVaiTro,
  datVaiTro,
  loiNhacQuanTriCuoi,
  nhanNutVaiTro,
  vaiTroDoiThanh,
} from "../../../screens/quan-tri/quan-tri";
import { danhSachThanhVien, type ThanhVien } from "../../../screens/vao-cua/cong-api";
import { tenCuocTroChuyen } from "../../nhan-rieng/nhan-rieng";
import { useRudiSession } from "../../session";
import { typography, useRudiTheme } from "../../theme";
import { Heading, ListRow, RudiButton, RudiScreen, TopBar } from "../../ui";
import { Avatar } from "../../ui/Avatar";
import { ErrorState } from "../../ui/ErrorState";
import { SkeletonGroup, SkeletonRow } from "../../ui/Skeleton";
import { Stamp } from "../../ui/Stamp";

type Trang =
  | { pha: "dang-doc" }
  | { pha: "xong"; thanhVien: ThanhVien[] }
  | { pha: "hong"; loi: string };

export function GroupMembersScreen() {
  const router = useRouter();
  const { colors } = useRudiTheme();
  const { id } = useLocalSearchParams<{ id: string }>();
  const { phien, phienDaDoc } = useRudiSession();
  const [trang, setTrang] = useState<Trang>({ pha: "dang-doc" });
  const [dangDoiVaiTro, setDangDoiVaiTro] = useState<string | null>(null);
  const [loiVaiTro, setLoiVaiTro] = useState<string | null>(null);
  // One attempt per (person, target role): a re-render never mints a second key.
  const attempts = useRef<Record<string, Attempt>>({});

  const nap = useCallback(async () => {
    if (phien === null || typeof id !== "string") return;
    try {
      setTrang({ pha: "xong", thanhVien: await danhSachThanhVien(id, phien.person_id) });
    } catch (error) {
      setTrang({
        pha: "hong",
        loi: error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null),
      });
    }
  }, [id, phien]);

  useFocusEffect(
    useCallback(() => {
      void nap();
    }, [nap]),
  );

  if (!phienDaDoc) return null;
  if (phien === null) return <Redirect href="/welcome" />;
  if (typeof id !== "string") return <Redirect href="/messages" />;

  const tenNhom = tenCuocTroChuyen(phien.contexts?.find((nhom) => nhom.id === id));
  const conSong = trang.pha === "xong" ? trang.thanhVien.filter((tv) => tv.state !== "left") : [];
  const nhacQuanTriCuoi = trang.pha === "xong" ? loiNhacQuanTriCuoi(conSong, phien.person_id) : null;

  /** Promote or demote one member (ADR-0021 L1 wires the M2 route into the roster). */
  const doiVaiTro = async (tv: ThanhVien) => {
    if (dangDoiVaiTro !== null) return;
    const vaiTro = vaiTroDoiThanh(tv);
    setDangDoiVaiTro(tv.person_id);
    setLoiVaiTro(null);
    try {
      await datVaiTro(id, tv.person_id, vaiTro, phien.person_id, attemptFor(attempts.current, `${tv.person_id}:${vaiTro}`));
      await nap();
    } catch (error) {
      setLoiVaiTro(error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null));
    } finally {
      setDangDoiVaiTro(null);
    }
  };

  return (
    <RudiScreen testID="group-members-screen">
      <TopBar title={tenNhom} />
      <Heading
        title="Thành viên"
        subtitle={
          trang.pha === "xong"
            ? `${conSong.filter((tv) => tv.state === "active").length} đang ở trong nhóm, ${conSong.filter((tv) => tv.state === "invited").length} đang được mời.`
            : "Đang đọc danh sách thành viên…"
        }
      />
      <View style={[styles.loiVao, { borderTopColor: colors.line, borderBottomColor: colors.line }]}>
        <ListRow icon="images-outline" onPress={() => router.push(`/groups/${id}/wall` as never)} subtitle="Ảnh, check-in, tim và bình luận. Chỉ thành viên thấy." title="Tường kỷ niệm" />
        <ListRow icon="albums-outline" onPress={() => router.push(`/groups/${id}/album` as never)} subtitle="Mỗi kèo một album." title="Album chuyến đi" />
      </View>
      {trang.pha === "dang-doc" ? (
        <SkeletonGroup>
          <SkeletonRow />
          <SkeletonRow />
        </SkeletonGroup>
      ) : null}
      {trang.pha === "hong" ? <ErrorState body={trang.loi} onRetry={() => void nap()} title="Chưa đọc được danh sách thành viên" /> : null}
      {trang.pha === "xong" ? (
        <View>
          {conSong.map((tv) => {
            const ten = tv.display_name ?? "Thành viên";
            const laToi = tv.person_id === phien.person_id;
            const duocMoi = tv.state === "invited";
            return (
              <View key={tv.id} style={[styles.hang, { borderBottomColor: colors.line }]}>
                <Avatar name={ten} ring={laToi} size={40} />
                <View style={styles.hangChu}>
                  <Text style={[typography.body, { color: duocMoi ? colors.inkSoft : colors.ink }]}>
                    {ten}
                    {laToi ? " (bạn)" : ""}
                  </Text>
                  <Text style={[typography.caption, { color: colors.inkFaint }]}>
                    {duocMoi ? "Đã mời, chưa đồng ý" : tv.role === "admin" && tv.state === "active" ? "Mở nhóm này" : "Thành viên"}
                  </Text>
                </View>
                {coTheDoiVaiTro(conSong, phien.person_id, tv) ? (
                  <RudiButton
                    compact
                    full={false}
                    label={nhanNutVaiTro(tv)}
                    loading={dangDoiVaiTro === tv.person_id}
                    onPress={() => void doiVaiTro(tv)}
                    variant="soft"
                  />
                ) : tv.role === "admin" && tv.state === "active" ? (
                  <Stamp label="Quản trị" />
                ) : null}
              </View>
            );
          })}
          {nhacQuanTriCuoi ? <Text style={[typography.caption, { color: colors.inkFaint }]}>{nhacQuanTriCuoi}</Text> : null}
          {loiVaiTro ? <Text style={[typography.caption, { color: colors.warn }]}>{loiVaiTro}</Text> : null}
        </View>
      ) : null}
      <RudiButton
        icon="person-add-outline"
        label="Mời bằng số điện thoại"
        onPress={() => router.push(`/groups/${id}/invite` as never)}
      />
      <Text style={[typography.caption, { color: colors.inkFaint }]}>
        Người được mời thấy lời mời ở tab Tin nhắn ngay khi đăng nhập bằng số đó, và chính họ bấm «Đồng
        ý». Không ai bị đưa vào nhóm mà chưa gật đầu.
      </Text>
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  loiVao: { paddingVertical: 4, borderTopWidth: StyleSheet.hairlineWidth, borderBottomWidth: StyleSheet.hairlineWidth },
  hang: { flexDirection: "row", alignItems: "center", gap: 12, minHeight: 60, paddingVertical: 8, borderBottomWidth: StyleSheet.hairlineWidth },
  hangChu: { flex: 1, gap: 2 },
});
