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
import { StyleSheet, Text, View } from "react-native";

import { ApiError, newAttempt, taiAnhDaiDien, thongDiepNguoiDoc } from "../../../api";
import { baoDaDoiAnh } from "../../nguoi/anh-dai-dien";
import { boAnh, chonAnh, nenVaDung } from "../../ky-niem/chon-anh";
import { AnhNhomError } from "../../../camera/anh-nhom";
import { CHINH_SACH, datChinhSachBinhLuan, laChinhSach } from "../../nguoi/chinh-sach-tuong";
import { docHoSoToi, type HoSoToi } from "../../../phien";
import { NHAN_GIAO_DIEN } from "../../giao-dien";
import { useRudiSession } from "../../session";
import { typography, useRudiTheme } from "../../theme";
import { Chip, Inline, ListRow, NhomHang, RudiButton, RudiScreen, SectionHeader, Segmented, TopBar } from "../../ui";
import { AvatarNguoi } from "../../ui/AvatarNguoi";
import { CauTaiCho } from "../../ui/CauTaiCho";
import { congTac } from "../../ui/cong-tac";
import { useGiaoDien } from "../../ui/GiaoDienProvider";

export function CaiDatScreen() {
  const router = useRouter();
  const { colors } = useRudiTheme();
  const { phien, phienDaDoc } = useRudiSession();
  const { cheDo, datCheDo } = useGiaoDien();
  const [hoSo, setHoSo] = useState<HoSoToi | null>(null);
  // QA UI-107: each control says its own failure under itself; one sentence
  // at the foot of the page was 700px below the switch that failed.
  const [loi, setLoi] = useState<string | null>(null);
  const [loiAnh, setLoiAnh] = useState<string | null>(null);
  const [loiTim, setLoiTim] = useState<string | null>(null);
  const [loiChinhSach, setLoiChinhSach] = useState<string | null>(null);
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

  const doiChinhSach = async (ma: string) => {
    if (!laChinhSach(ma)) return;
    if (phien === null || dangLuu) return;
    setDangLuu(true);
    setLoiChinhSach(null);
    try {
      const sau = await datChinhSachBinhLuan(ma, phien.person_id, newAttempt());
      setHoSo((truoc) => (truoc === null ? truoc : { ...truoc, wall_comment_policy: sau.wall_comment_policy }));
    } catch (error) {
      setLoiChinhSach(error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null));
    } finally {
      setDangLuu(false);
    }
  };

  const doiAnhDaiDien = async () => {
    if (phien === null || dangDoiAnh) return;
    setLoiAnh(null);
    const daChon = await chonAnh().catch((error: unknown) => {
      setLoiAnh(error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null));
      return null;
    });
    if (daChon === null) return;
    setDangDoiAnh(true);
    try {
      const daTai = await nenVaDung(daChon, (nen) => taiAnhDaiDien(phien.person_id, nen, phien.person_id));
      // The new id is the new version: every frame on this phone switches now;
      // the stream tells everyone who shares a group with us.
      baoDaDoiAnh(phien.person_id, phien.person_id, daTai.id);
    } catch (error) {
      await boAnh(daChon);
      // `AnhNhomError` carries the device's own words ("not a picture", "too
      // large"); replacing them with the network sentence sent people looking
      // at their Wi-Fi for a file that was never an image (measured 2026-09-24).
      setLoiAnh(error instanceof ApiError || error instanceof AnhNhomError ? error.message : thongDiepNguoiDoc(0, null));
    } finally {
      setDangDoiAnh(false);
    }
  };

  if (!phienDaDoc) return null;

  // QA UI-015: nothing drawn from a stand-in. The name comes from the session
  // until the profile answers.
  const tenToi = hoSo?.display_name ?? phien?.profile?.display_name ?? "";

  return (
    <RudiScreen testID="cai-dat-screen">
      <TopBar title="Cài đặt" />
      {loi !== null && hoSo === null ? <CauTaiCho cau={loi} hanhDong={{ label: "Thử lại", onPress: () => { setLoi(null); void nap(); } }} /> : null}
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
            <AvatarNguoi name={tenToi} personId={phien?.person_id} size={64} />
            <View style={styles.hangChu}>
              <Text style={[typography.label, { color: colors.ink }]}>{tenToi}</Text>
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
          <CauTaiCho cau={loiAnh} co="nho" />
        </View>
        <ListRow
          icon="person-outline"
          onPress={() => router.push("/personalization" as never)}
          subtitle="Món ăn, kiểu đi chơi và mức chi bạn thích"
          title="Sở thích"
        />
      </NhomHang>
      <SectionHeader title="Đăng nhập & phiên" />
      <ListRow icon="shield-checkmark-outline" onPress={() => router.push("/settings/account" as never)} title="Tài khoản & bảo mật" subtitle="Username, email, mật khẩu và Google" />
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
        <View style={styles.khoi}>
          <Text style={[typography.label, { color: colors.ink }]}>Ai được bình luận tường tôi</Text>
          <View accessibilityRole="radiogroup" style={styles.chips}>
            {CHINH_SACH.map((muc) => (
              <Chip
                key={muc.id}
                label={muc.nhan}
                onPress={() => void doiChinhSach(muc.id)}
                selected={hoSo !== null && hoSo.wall_comment_policy === muc.id}
                vaiRadio
              />
            ))}
          </View>
          {hoSo !== null ? (
            <Text style={[typography.caption, { color: colors.inkFaint }]}>
              {(CHINH_SACH.find((muc) => muc.id === hoSo.wall_comment_policy) ?? CHINH_SACH[0]).giaiThich}
            </Text>
          ) : null}
          <CauTaiCho cau={loiChinhSach} co="nho" />
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
      {/* QA UI-111: the name is changed in «Chỉnh hồ sơ», not in «Tài khoản». */}
      <Text style={[typography.caption, { color: colors.inkFaint }]}>
        Tên hiển thị và lời giới thiệu đổi ở «Chỉnh hồ sơ» trên trang Cá nhân; đăng xuất ở mục Tài khoản của trang đó.
      </Text>
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  khoi: { gap: 12, paddingVertical: 6 },
  hang: { flexDirection: "row", alignItems: "center", gap: 12 },
  loiHang: { paddingBottom: 6 },
  hangChu: { flex: 1, gap: 2 },
  chips: { flexDirection: "row", flexWrap: "wrap", gap: 8 },
});
