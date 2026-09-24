/**
 * Cài đặt thật (L5, ADR-0023 §2.5), thay cho panel tự nhận là «không phải cài
 * đặt production».
 *
 * Sáu mục, mỗi mục là một câu người dùng hiểu được: hồ sơ, sở thích, phiên,
 * quyền riêng tư, giao diện, và cuối cùng là xoá tài khoản. «Tài khoản» và
 * «Đăng xuất» KHÔNG chuyển vào đây: mọi lượt Maestro đăng xuất đi qua panel
 * ấy trên màn Cá nhân, và dời nó đi là làm hỏng bằng chứng của mọi lát khác.
 *
 * Giao diện là tuỳ chọn trên máy; hai mục còn lại của «Quyền riêng tư» đi
 * thẳng lên máy chủ qua `PATCH /people/me`, mỗi lần một khoá.
 */
import { useRouter } from "expo-router";
import { useCallback, useEffect, useState } from "react";
import { StyleSheet, Switch, Text, View } from "react-native";

import { ApiError, newAttempt, taiAnhDaiDien, thongDiepNguoiDoc } from "../../../api";
import { baoDaDoiAnh } from "../../nguoi/anh-dai-dien";
import { boAnh, chonAnh, nenVaDung } from "../../ky-niem/chon-anh";
import { CHINH_SACH, datChinhSachBinhLuan, laChinhSach } from "../../nguoi/chinh-sach-tuong";
import { docHoSoToi, suaHoSoToi, type HoSoToi } from "../../../phien";
import { NHAN_GIAO_DIEN } from "../../giao-dien";
import { useRudiSession } from "../../session";
import { typography, useRudiTheme } from "../../theme";
import { Chip, Inline, ListRow, NhomHang, RudiButton, RudiScreen, SectionHeader, Segmented, TopBar } from "../../ui";
import { AvatarNguoi } from "../../ui/AvatarNguoi";
import { useGiaoDien } from "../../ui/GiaoDienProvider";

export function CaiDatScreen() {
  const router = useRouter();
  const { colors } = useRudiTheme();
  const { phien, phienDaDoc } = useRudiSession();
  const { cheDo, datCheDo } = useGiaoDien();
  const [hoSo, setHoSo] = useState<HoSoToi | null>(null);
  const [loi, setLoi] = useState<string | null>(null);
  const [dangLuu, setDangLuu] = useState(false);
  const [dangDoiAnh, setDangDoiAnh] = useState(false);

  const nap = useCallback(async () => {
    if (phien === null) return;
    try {
      setHoSo(await docHoSoToi(phien.person_id));
    } catch (error) {
      setLoi(error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null));
    }
  }, [phien]);

  useEffect(() => {
    void nap();
  }, [nap]);

  const doiTimTheoSo = async (bat: boolean) => {
    if (phien === null || dangLuu) return;
    setDangLuu(true);
    setLoi(null);
    try {
      const moi = await suaHoSoToi(phien.person_id, { discoverable_by_phone: bat });
      setHoSo(moi);
    } catch (error) {
      setLoi(error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null));
    } finally {
      setDangLuu(false);
    }
  };

  const doiChinhSach = async (ma: string) => {
    if (!laChinhSach(ma)) return;
    if (phien === null || dangLuu) return;
    setDangLuu(true);
    setLoi(null);
    try {
      const sau = await datChinhSachBinhLuan(ma, phien.person_id, newAttempt());
      setHoSo((truoc) => (truoc === null ? truoc : { ...truoc, wall_comment_policy: sau.wall_comment_policy }));
    } catch (error) {
      setLoi(error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null));
    } finally {
      setDangLuu(false);
    }
  };

  const doiAnhDaiDien = async () => {
    if (phien === null || dangDoiAnh) return;
    setLoi(null);
    const daChon = await chonAnh().catch((error: unknown) => {
      setLoi(error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null));
      return null;
    });
    if (daChon === null) return;
    setDangDoiAnh(true);
    try {
      await nenVaDung(daChon, (nen) => taiAnhDaiDien(phien.person_id, nen, phien.person_id));
      // The address never changes, so every frame showing it -- here, on the
      // profile, in the rosters -- has to be told to reload, not just this one.
      baoDaDoiAnh(phien.person_id, phien.person_id);
    } catch (error) {
      await boAnh(daChon);
      setLoi(error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null));
    } finally {
      setDangDoiAnh(false);
    }
  };

  if (!phienDaDoc) return null;

  const timDuoc = hoSo?.discoverable_by_phone ?? true;

  return (
    <RudiScreen testID="cai-dat-screen">
      <TopBar title="Cài đặt" />
      {/* Rows on paper, one hairline under each: the same surface system as
          Cá nhân and the ledger next door. Eight floating white cards here read
          as a second UI kit (re-audit 10/09, R5), and a card around `Segmented`
          was a card inside a card. */}
      <SectionHeader title="Hồ sơ" />
      <NhomHang>
        <View style={styles.khoi}>
          <Inline gap={12}>
            {/* A 404 before the first upload is ordinary; the frame draws
                initials for it rather than an empty ring (board 2026-09-07). */}
            <AvatarNguoi name={hoSo?.display_name ?? "Bạn"} personId={phien?.person_id} size={64} />
            <View style={styles.hangChu}>
              <Text style={[typography.label, { color: colors.ink }]}>{hoSo?.display_name ?? "Bạn"}</Text>
              <Text style={[typography.caption, { color: colors.inkFaint }]}>
                Ảnh này hiện với những người chung nhóm với bạn.
              </Text>
            </View>
          </Inline>
          <RudiButton
            label="Đổi ảnh đại diện"
            loading={dangDoiAnh}
            onPress={() => void doiAnhDaiDien()}
            variant="outline"
          />
        </View>
        <ListRow
          icon="person-outline"
          onPress={() => router.push("/personalization" as never)}
          subtitle="Món ăn, kiểu đi chơi và mức chi bạn thích"
          title="Sở thích"
        />
      </NhomHang>
      <SectionHeader title="Đăng nhập & phiên" />
      <NhomHang>
        <ListRow
          icon="phone-portrait-outline"
          onPress={() => router.push("/settings/phien" as never)}
          subtitle="Xem nơi tài khoản đang đăng nhập, đăng xuất từ xa"
          title="Phiên đăng nhập"
        />
      </NhomHang>
      <SectionHeader title="Quyền riêng tư" />
      <NhomHang>
        <View style={styles.hang}>
          <View style={styles.hangChu}>
            <Text style={[typography.label, { color: colors.ink }]}>Tìm theo số điện thoại</Text>
            <Text style={[typography.caption, { color: colors.inkFaint }]}>
              {timDuoc
                ? "Bạn bè nhập đúng số của bạn thì tìm thấy bạn."
                : "Không ai tìm được bạn theo số điện thoại."}
            </Text>
          </View>
          <Switch
            accessibilityLabel="Cho tìm theo số điện thoại"
            disabled={dangLuu || hoSo === null}
            onValueChange={(bat) => void doiTimTheoSo(bat)}
            thumbColor={colors.card}
            trackColor={{ true: colors.accent, false: colors.line }}
            value={timDuoc}
          />
        </View>
        <View style={styles.khoi}>
          <Text style={[typography.label, { color: colors.ink }]}>Ai được bình luận tường tôi</Text>
          <View accessibilityRole="radiogroup" style={styles.chips}>
            {CHINH_SACH.map((muc) => (
              <Chip
                key={muc.id}
                label={muc.nhan}
                onPress={() => void doiChinhSach(muc.id)}
                selected={(hoSo?.wall_comment_policy ?? "readers") === muc.id}
              />
            ))}
          </View>
          <Text style={[typography.caption, { color: colors.inkFaint }]}>
            {(CHINH_SACH.find((muc) => muc.id === (hoSo?.wall_comment_policy ?? "readers")) ?? CHINH_SACH[0]).giaiThich}
          </Text>
        </View>
        <ListRow
          icon="hand-left-outline"
          onPress={() => router.push("/settings/da-chan" as never)}
          subtitle="Xem và gỡ chặn những người bạn đã chặn"
          title="Người đã chặn"
        />
      </NhomHang>
      <SectionHeader title="Giao diện" />
      <View style={styles.khoi}>
        <Segmented
          items={NHAN_GIAO_DIEN.map((muc) => muc.nhan)}
          onSelect={(chi_so) => datCheDo(NHAN_GIAO_DIEN[chi_so].ma)}
          selected={NHAN_GIAO_DIEN.findIndex((muc) => muc.ma === cheDo)}
        />
        <Text style={[typography.caption, { color: colors.inkFaint }]}>
          Lựa chọn này chỉ ở trên máy này, không gửi đi đâu.
        </Text>
      </View>
      <SectionHeader title="Về Rủ Đi" />
      <NhomHang>
        <ListRow
          icon="document-text-outline"
          onPress={() => router.push("/settings/ve-rudi" as never)}
          subtitle="Điều khoản, dữ liệu Rủ Đi giữ, và điều gì xảy ra khi bạn xoá tài khoản"
          title="Điều khoản và dữ liệu"
        />
      </NhomHang>
      <SectionHeader title="Tài khoản" />
      <NhomHang>
        <ListRow
          icon="trash-outline"
          onPress={() => router.push("/settings/xoa-tai-khoan" as never)}
          subtitle="Xoá vĩnh viễn hồ sơ và nội dung của bạn"
          title="Xoá tài khoản"
        />
      </NhomHang>
      {loi ? <Text style={[typography.body, { color: colors.warn }]}>{loi}</Text> : null}
      <Text style={[typography.caption, { color: colors.inkFaint }]}>
        Đăng nhập, đăng xuất và tên hiển thị vẫn nằm ở mục Tài khoản trên màn Cá nhân.
      </Text>
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  khoi: { gap: 12, paddingVertical: 6 },
  hang: { flexDirection: "row", alignItems: "center", gap: 12 },
  hangChu: { flex: 1, gap: 2 },
  chips: { flexDirection: "row", flexWrap: "wrap", gap: 8 },
});
