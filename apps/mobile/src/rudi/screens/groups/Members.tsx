/**
 * One group's roster, and the door to invite somebody (M2).
 *
 * Reuses the legacy client module (`danhSachThanhVien`): the route is the
 * same one App B called, with the bearer now doing the identifying. Names
 * come with the roster (`display_name`), initials are drawn in one tone -- the
 * design system does not colour people.
 */
import { Redirect, useFocusEffect, useLocalSearchParams, useRouter } from "expo-router";
import { useCallback, useRef, useState } from "react";
import { StyleSheet, Text, View } from "react-native";

import { ApiError, attemptFor, thongDiepNguoiDoc, type Attempt } from "../../../api";
import { chuDau } from "../../../screens/ca-nhan/ban-be";
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
import { Card, Chip, Heading, ListRow, RudiButton, RudiScreen, TopBar } from "../../ui";

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
            : "Đang đọc danh sách từ máy chủ..."
        }
      />
      <Card style={styles.loiVao}>
        <ListRow icon="images-outline" onPress={() => router.push(`/groups/${id}/wall` as never)} subtitle="Ảnh, check-in, tim và bình luận. Chỉ thành viên thấy." title="Tường kỷ niệm" />
        <ListRow icon="albums-outline" onPress={() => router.push(`/groups/${id}/album` as never)} subtitle="Mỗi kèo một album, có thước phim." title="Album chuyến đi" />
      </Card>
      {trang.pha === "hong" ? (
        <Card>
          <Text style={[typography.body, { color: colors.warn }]}>{trang.loi}</Text>
          <RudiButton label="Thử lại" onPress={() => void nap()} variant="outline" />
        </Card>
      ) : null}
      {trang.pha === "xong" ? (
        <Card style={styles.danhSach}>
          {conSong.map((tv) => {
            const ten = tv.display_name ?? "Thành viên";
            const laToi = tv.person_id === phien.person_id;
            return (
              <View key={tv.id} style={styles.hang}>
                <View style={[styles.chuDau, { backgroundColor: colors.accentSoft }]}>
                  <Text style={[typography.title, { color: colors.accent }]}>{chuDau(ten)}</Text>
                </View>
                <View style={styles.hangChu}>
                  <Text style={[typography.body, { color: colors.ink }]}>
                    {ten}
                    {laToi ? " (bạn)" : ""}
                  </Text>
                  <Text style={[typography.caption, { color: colors.inkFaint }]}>
                    {tv.state === "invited" ? "Đã mời, chưa đồng ý" : tv.role === "admin" ? "Quản trị" : "Thành viên"}
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
                  <Chip label="Quản trị" />
                ) : null}
              </View>
            );
          })}
          {nhacQuanTriCuoi ? <Text style={[typography.caption, { color: colors.inkFaint }]}>{nhacQuanTriCuoi}</Text> : null}
          {loiVaiTro ? <Text style={[typography.caption, { color: colors.warn }]}>{loiVaiTro}</Text> : null}
        </Card>
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
  loiVao: { gap: 0, paddingVertical: 4 },
  danhSach: { gap: 12 },
  hang: { flexDirection: "row", alignItems: "center", gap: 12 },
  hangChu: { flex: 1, gap: 2 },
  chuDau: { width: 40, height: 40, borderRadius: 14, alignItems: "center", justifyContent: "center" },
});
