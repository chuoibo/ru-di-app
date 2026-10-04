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
import { HoiTaiHang } from "../../ui/HoiTaiHang";
import { useCallback, useRef, useState } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";

import { ApiError, attemptFor, thongDiepNguoiDoc, type Attempt } from "../../../api";
import {
  coTheDoiVaiTro,
  datVaiTro,
  loiNhacQuanTriCuoi,
  nhanNutVaiTro,
  vaiTroDoiThanh,
} from "../../../screens/quan-tri/quan-tri";
import { danhSachThanhVien, type ThanhVien } from "../../../screens/vao-cua/cong-api";
import { docNhom } from "../../chat/nhom-cai-dat";
import { tenCuocTroChuyen } from "../../nhan-rieng/nhan-rieng";
import { useRudiSession } from "../../session";
import { mucNguoi, typography, useRudiTheme } from "../../theme";
import { CauTaiCho } from "../../ui/CauTaiCho";
import { ChuThichLe } from "../../ui/ChuThichLe";
import { StampButton } from "../../ui/StampButton";
import { Heading, ListRow, RudiButton, RudiScreen, TopBar } from "../../ui";
import { AvatarNguoi } from "../../ui/AvatarNguoi";
import { LuoiNguoi } from "../../ui/LuoiNguoi";
import { ErrorState } from "../../ui/ErrorState";
import { SkeletonGroup, SkeletonRow } from "../../ui/Skeleton";
import { Stamp } from "../../ui/Stamp";
import { CuaDangNhap } from "../../ui/CuaDangNhap";

type Trang =
  | { pha: "dang-doc" }
  | { pha: "xong"; thanhVien: ThanhVien[] }
  | { pha: "hong"; loi: string };

export function GroupMembersScreen() {
  const router = useRouter();
  const { colors, dark } = useRudiTheme();
  const { id } = useLocalSearchParams<{ id: string }>();
  const { phien, phienDaDoc } = useRudiSession();
  const [trang, setTrang] = useState<Trang>({ pha: "dang-doc" });
  const [dangDoiVaiTro, setDangDoiVaiTro] = useState<string | null>(null);
  // A refused role change is said under the row it was pressed on (QA UI-095 pattern).
  const [loiVaiTro, setLoiVaiTro] = useState<{ id: string; cau: string } | null>(null);
  // QA UI-074: stepping down from admin cannot be undone by oneself; it asks
  // in the row first.
  const [hoiTuBo, setHoiTuBo] = useState(false);
  // QA UI-075: «Người lập nhóm» is the person who opened the group, read from
  // the group itself; every other admin is «Quản trị». Unknown until it answers.
  const [nguoiLap, setNguoiLap] = useState<string | null>(null);
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
      if (phien !== null && typeof id === "string") {
        docNhom(id, phien.person_id).then((nhom) => setNguoiLap(nhom.created_by_id)).catch(() => setNguoiLap(null));
      }
    }, [nap, phien, id]),
  );

  if (!phienDaDoc) return null;
  if (phien === null) return <CuaDangNhap />;
  if (typeof id !== "string") return <Redirect href="/messages" />;

  const tenNhom = tenCuocTroChuyen(phien.contexts?.find((nhom) => nhom.id === id));
  const conSong = trang.pha === "xong" ? trang.thanhVien.filter((tv) => tv.state !== "left") : [];
  const nhacQuanTriCuoi = trang.pha === "xong" ? loiNhacQuanTriCuoi(conSong, phien.person_id) : null;

  /** Promote or demote one member (ADR-0021 L1 wires the M2 route into the roster). */
  const doiVaiTro = async (tv: ThanhVien, daHoi = false) => {
    if (dangDoiVaiTro !== null) return;
    const vaiTro = vaiTroDoiThanh(tv);
    if (tv.person_id === phien.person_id && vaiTro === "member" && !daHoi) {
      setHoiTuBo(true);
      setLoiVaiTro(null);
      return;
    }
    setDangDoiVaiTro(tv.person_id);
    setLoiVaiTro(null);
    try {
      await datVaiTro(id, tv.person_id, vaiTro, phien.person_id, attemptFor(attempts.current, `${tv.person_id}:${vaiTro}`));
      setHoiTuBo(false);
      await nap();
    } catch (error) {
      setLoiVaiTro({ id: tv.person_id, cau: error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null) });
    } finally {
      setDangDoiVaiTro(null);
    }
  };
  // The caption names the role unless the row already wears the «Quản trị»
  // stamp; the one who opened the group is always said.
  const nhanVaiTro = (tv: ThanhVien, coDau: boolean): string | null => {
    if (tv.state === "invited") return "Đã mời, chưa đồng ý";
    if (nguoiLap === tv.person_id) return "Người lập nhóm";
    if (tv.role === "admin" && tv.state === "active") return coDau ? null : "Quản trị";
    return "Thành viên";
  };

  return (
    // A list of people is read, not stretched: the reading column on a tablet
    // keeps each action beside its name (QA UI-081: 487–679px apart).
    <RudiScreen cot="doc" testID="group-members-screen">
      <TopBar title={tenNhom} />
      <Heading
        title="Thành viên"
        subtitle={
          trang.pha === "xong"
            ? `${conSong.filter((tv) => tv.state === "active").length} đang ở trong nhóm, ${conSong.filter((tv) => tv.state === "invited").length} đang được mời.`
            : trang.pha === "hong"
              ? undefined
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
          <LuoiNguoi vachTrong={false} hang={conSong.map((tv) => {
            const ten = tv.display_name ?? "Thành viên";
            const laToi = tv.person_id === phien.person_id;
            const duocMoi = tv.state === "invited";
            const coNut = coTheDoiVaiTro(conSong, phien.person_id, tv) && !(laToi && hoiTuBo);
            const coDau = !coNut && tv.role === "admin" && tv.state === "active" && !(laToi && hoiTuBo);
            const vai = nhanVaiTro(tv, coDau);
            const nutVaiTro = coNut ? (
              <RudiButton
                accessibilityLabel={laToi ? "Bỏ quyền quản trị của bạn" : tv.role === "admin" ? `Bỏ quyền quản trị của ${ten}` : `Đặt ${ten} làm quản trị`}
                compact
                full={false}
                label={nhanNutVaiTro(tv)}
                loading={dangDoiVaiTro === tv.person_id}
                onPress={() => void doiVaiTro(tv)}
                // A quiet word, not a pill: the action repeats on every
                // row of a 20-person group.
                variant="ghost"
              />
            ) : null;
            return { key: tv.id, node: (luoi: boolean) => (
              // In the grid a cell fills its line, so neighbours share one hairline.
              <View style={[styles.hangKhoi, luoi && styles.oLuoi, { borderBottomColor: colors.line }]}>
                <View style={styles.hang}>
                  {/* The person is the way to their profile (QA UI-076); the role
                      action stands beside it, never inside it. */}
                  <Pressable
                    accessibilityLabel={laToi ? "Xem hồ sơ của bạn" : `Xem hồ sơ ${ten}`}
                    accessibilityRole="button"
                    onPress={() => router.push(`/people/${tv.person_id}` as never)}
                    style={({ pressed }) => [styles.nguoi, pressed && styles.bam]}
                  >
                    <AvatarNguoi name={ten} personId={tv.person_id} ring={laToi} size={40} />
                    <View style={styles.hangChu}>
                      {/* Each member in their own ink (ADR-0037 D6); somebody still only invited is pencil. */}
                      <Text style={[typography.body, { color: duocMoi ? colors.inkSoft : mucNguoi(tv.person_id, dark) }]}>
                        {ten}
                        {laToi ? " (bạn)" : ""}
                      </Text>
                      {vai ? <Text style={[typography.caption, { color: colors.inkFaint }]}>{vai}</Text> : null}
                    </View>
                  </Pressable>
                  {/* Beside the name on a phone; under it, in the text column, in
                      a two-column grid, where beside it broke the name in two. */}
                  {nutVaiTro && !luoi ? (
                    nutVaiTro
                  ) : coDau ? (
                    <Stamp label="Quản trị" tilt={-3} tone="ink" />
                  ) : null}
                </View>
                {nutVaiTro && luoi ? <View style={styles.nutDuoi}>{nutVaiTro}</View> : null}
                {laToi && hoiTuBo ? (
                  <View style={styles.hoi}>
                    <HoiTaiHang
                      cau="Bỏ quyền quản trị của bạn? Sau đó bạn không tự lấy lại được; một quản trị khác phải đặt lại cho bạn."
                      dangLam={dangDoiVaiTro === tv.person_id}
                      nhan="Bỏ quyền"
                      onDongY={() => void doiVaiTro(tv, true)}
                      onThoi={() => setHoiTuBo(false)}
                      testID="thanh-vien-hoi-tu-bo"
                    />
                  </View>
                ) : null}
                {loiVaiTro?.id === tv.person_id ? <CauTaiCho cau={loiVaiTro.cau} co="nho" /> : null}
              </View>
            ) };
          })} />
          {nhacQuanTriCuoi ? <Text style={[typography.caption, { color: colors.inkFaint }]}>{nhacQuanTriCuoi}</Text> : null}
        </View>
      ) : null}
      <StampButton label="Mời bằng số điện thoại" onPress={() => router.push(`/groups/${id}/invite` as never)} size="vua" tilt={-1} />
      {/* Consent, said where the invitation starts: a margin note, still read. */}
      <ChuThichLe icon="mail-open-outline">
        Người được mời thấy lời mời ở tab Tin nhắn ngay khi đăng nhập bằng số đó, và chính họ bấm «Đồng ý». Không ai bị đưa vào nhóm mà chưa gật đầu.
      </ChuThichLe>
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  loiVao: { paddingVertical: 4, borderTopWidth: StyleSheet.hairlineWidth, borderBottomWidth: StyleSheet.hairlineWidth },
  hangKhoi: { paddingVertical: 8, gap: 6, borderBottomWidth: StyleSheet.hairlineWidth },
  hang: { flexDirection: "row", alignItems: "center", gap: 12, minHeight: 48 },
  nguoi: { flex: 1, flexDirection: "row", alignItems: "center", gap: 12, minHeight: 48 },
  bam: { opacity: 0.7 },
  hangChu: { flex: 1, gap: 2 },
  hoi: { gap: 4, paddingLeft: 52 },
  hoiNut: { flexDirection: "row", gap: 8 },
  // The ghost button's own padding (14) aligned to the text column (52).
  nutDuoi: { alignItems: "flex-start", marginTop: -12, paddingLeft: 38 },
  oLuoi: { flexGrow: 1 },
});
