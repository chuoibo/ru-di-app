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
import { useCallback, useEffect, useRef, useState } from "react";
import { Pressable, StyleSheet, View } from "react-native";

import { ApiError, newAttempt, taiAnhDaiDien, thongDiepNguoiDoc } from "../../../api";
import { baoDaDoiAnh } from "../../nguoi/anh-dai-dien";
import { boAnh, chonAnhDaiDien, nenVaDung, type TempPhoto } from "../../ky-niem/chon-anh";
import { AnhNhomError } from "../../../camera/anh-nhom";
import { CHINH_SACH, datChinhSachBinhLuan, laChinhSach } from "../../nguoi/chinh-sach-tuong";
import { docHoSoToi, type HoSoToi } from "../../../phien";
import { NHAN_GIAO_DIEN } from "../../giao-dien";
import { useRudiSession } from "../../session";
import { typography, useRudiTheme } from "../../theme";
import { Chip, Inline, ListRow, NhomHang, RudiButton, RudiScreen, SectionHeader, TopBar } from "../../ui";
import { RudiIcon } from "../../ui/RudiIcon";
import { AvatarNguoi } from "../../ui/AvatarNguoi";
import { CauTaiCho } from "../../ui/CauTaiCho";
import { congTac } from "../../ui/cong-tac";
import { SkeletonLines } from "../../ui/Skeleton";
import { toggleState } from "../../../ui/a11y";
import { useGiaoDien } from "../../ui/GiaoDienProvider";
import { NativeFontMetricsProvider, Text, useNativeFontMetricsOwner } from "../../ui/NativeFontMetrics";
import { useNativeFontScale } from "../../chia-bill/useBillFontScale";

export function CaiDatScreen() {
  const owned = useNativeFontMetricsOwner();
  return owned ? <CaiDatContent /> : <ObserveSettingsMetrics />;
}

function ObserveSettingsMetrics() {
  const snapshot = useNativeFontScale();
  return <NativeFontMetricsProvider snapshot={snapshot}><CaiDatContent /></NativeFontMetricsProvider>;
}

function CaiDatContent() {
  const router = useRouter();
  const { colors } = useRudiTheme();
  const { phien, phienDaDoc } = useRudiSession();
  const { cheDo, datCheDo } = useGiaoDien();
  const policyWorking = useRef(false); const avatarWorking = useRef(false);
  const [pendingPhoto, setPendingPhoto] = useState<TempPhoto | null>(null);
  const [loading, setLoading] = useState(true);
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
    if (phien === null) { setLoading(false); return; }
    setLoading(true); setLoi(null);
    try {
      setHoSo(await docHoSoToi(phien.person_id));
    } catch (error) {
      setLoi(error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null));
    } finally { setLoading(false); }
  }, [phien]);

  useEffect(() => {
    void nap();
  }, [nap]);

  const doiChinhSach = async (ma: string) => {
    if (!laChinhSach(ma)) return;
    if (phien === null || policyWorking.current || !hoSo) return;
    policyWorking.current = true;
    setDangLuu(true);
    setLoiChinhSach(null);
    try {
      const sau = await datChinhSachBinhLuan(ma, phien.person_id, newAttempt());
      setHoSo((truoc) => (truoc === null ? truoc : { ...truoc, wall_comment_policy: sau.wall_comment_policy }));
    } catch (error) {
      setLoiChinhSach(error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null));
    } finally {
      policyWorking.current = false; setDangLuu(false);
    }
  };

  const doiAnhDaiDien = async (retry?: TempPhoto) => {
    if (phien === null || avatarWorking.current) return;
    avatarWorking.current = true; setDangDoiAnh(true); setLoiAnh(null);
    try {
      const chosen = retry ?? await chonAnhDaiDien();
      if (!chosen) return;
      if (pendingPhoto && pendingPhoto !== chosen) await boAnh(pendingPhoto);
      setPendingPhoto(chosen);
      const uploaded = await nenVaDung(chosen, (compressed) => taiAnhDaiDien(phien.person_id, compressed, phien.person_id));
      setPendingPhoto(null);
      baoDaDoiAnh(phien.person_id, phien.person_id, uploaded.id);
    } catch (error) {
      setLoiAnh(error instanceof ApiError || error instanceof AnhNhomError ? error.message : thongDiepNguoiDoc(0, null));
    } finally { avatarWorking.current = false; setDangDoiAnh(false); }
  };

  if (!phienDaDoc) return null;

  // QA UI-015: nothing drawn from a stand-in. The name comes from the session
  // until the profile answers.
  const tenToi = hoSo?.display_name ?? phien?.profile?.display_name ?? "";

  return (
    <RudiScreen material="quiet" cot="form" testID="cai-dat-screen">
      <TopBar title="Cài đặt" />
      {loading && !hoSo ? <SkeletonLines lines={3}/> : null}
      {loi !== null && hoSo === null ? <CauTaiCho cau={loi} hanhDong={{ label: "Thử lại", onPress: () => { setLoi(null); void nap(); } }} /> : null}
      <View style={styles.nhomGiaoDien}>
        <SectionHeader title="Giao diện" />
        <View accessibilityRole="radiogroup" style={[styles.luaChonGiaoDien, { backgroundColor: colors.card, borderColor: colors.lineStrong }]}>
          {NHAN_GIAO_DIEN.map((muc, index) => <Pressable
            key={muc.ma}
            {...toggleState("radio", cheDo === muc.ma, () => datCheDo(muc.ma))}
            accessibilityLabel={muc.nhan}
            onPress={() => datCheDo(muc.ma)}
            style={({ pressed }) => [styles.hangGiaoDien, index > 0 && { borderTopWidth: StyleSheet.hairlineWidth, borderTopColor: colors.line }, cheDo === muc.ma && { backgroundColor: colors.paper }, pressed && { backgroundColor: colors.ground }]}
          >
            <RudiIcon name={muc.ma === "sang" ? "sunny-outline" : muc.ma === "toi" ? "moon-outline" : "desktop-outline"} size={22} color={cheDo === muc.ma ? colors.split : colors.ink} />
            <Text style={[typography.body, styles.hangChu, { color: colors.ink }]}>{muc.nhan}</Text>
            <View pointerEvents="none" style={[styles.dauChon, { borderColor: cheDo === muc.ma ? colors.split : colors.lineStrong, backgroundColor: cheDo === muc.ma ? colors.split : "transparent" }]}>
              {cheDo === muc.ma ? <View style={[styles.chamChon, { backgroundColor: colors.card }]} /> : null}
            </View>
          </Pressable>)}
        </View>
        <Text style={[typography.caption, { color: colors.inkFaint }]}>Lựa chọn này chỉ ở trên máy này, không gửi đi đâu.</Text>
      </View>
      {/* Rows on paper, one hairline under each: the same surface system as
          Cá nhân and the ledger next door. Eight floating white cards here read
          as a second UI kit (re-audit 10/09, R5), and a card around `Segmented`
          was a card inside a card. */}
      <NhomHang>
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
          icon="shield-checkmark-outline"
          iconTreatment="plain"
          onPress={() => router.push("/settings/account" as never)}
          subtitle="Username, email, mật khẩu và Google"
          title="Tài khoản & bảo mật"
        />
        <ListRow
          icon="phone-portrait-outline"
          iconTreatment="plain"
          onPress={() => router.push("/settings/phien" as never)}
          subtitle="Xem nơi tài khoản đang đăng nhập, đăng xuất từ xa"
          title="Phiên đăng nhập"
        />
        <ListRow
          icon="lock-closed-outline"
          onPress={() => router.push("/settings/thiet-bi" as never)}
          subtitle="Máy nào mở được tin mã hoá đầu cuối của bạn"
          title="Thiết bị nhắn tin mã hoá"
          tone="split"
        />
      </NhomHang>
      <SectionHeader title="Quyền riêng tư" />
      <NhomHang>
        <View style={styles.khoi}>
          <Text style={[typography.label, { color: colors.ink }]}>Ai được bình luận tường tôi</Text>
          {hoSo ? <View accessibilityRole="radiogroup" style={styles.chips}>
            {CHINH_SACH.map((muc) => <Pressable key={muc.id} {...toggleState("radio", hoSo.wall_comment_policy === muc.id, () => void doiChinhSach(muc.id))} accessibilityLabel={muc.nhan} disabled={dangLuu} aria-disabled={dangLuu} onPress={() => void doiChinhSach(muc.id)} style={{ minHeight: 48, justifyContent: "center" }}><Chip label={muc.nhan} selected={hoSo.wall_comment_policy === muc.id}/></Pressable>)}
          </View> : loading ? <SkeletonLines lines={2}/> : null}
          {dangLuu ? <Text accessibilityLiveRegion="polite" style={[typography.caption, { color: colors.inkSoft }]}>Đang lưu lựa chọn…</Text> : null}
          {hoSo !== null ? (
            <Text style={[typography.caption, { color: colors.inkFaint }]}>
              {(CHINH_SACH.find((muc) => muc.id === hoSo.wall_comment_policy) ?? CHINH_SACH[0]).giaiThich}
            </Text>
          ) : null}
          <CauTaiCho cau={loiChinhSach} co="nho" />
        </View>
        <ListRow
          icon="hand-left-outline"
          iconTreatment="plain"
          onPress={() => router.push("/settings/da-chan" as never)}
          subtitle="Xem và gỡ chặn những người bạn đã chặn"
          title="Người đã chặn"
        />
      </NhomHang>
      <SectionHeader title="Hồ sơ" />
      <NhomHang>
        <View style={styles.khoi}>
          <Inline gap={12}>
            {/* A 404 before the first upload is ordinary; the frame draws
                initials for it rather than an empty ring (board 2026-09-07). */}
            <AvatarNguoi name={tenToi} personId={phien?.person_id} size={64} ring />
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
          {pendingPhoto && !dangDoiAnh ? <><RudiButton label="Thử lại ảnh đã chọn" variant="outline" onPress={() => void doiAnhDaiDien(pendingPhoto)}/><RudiButton label="Bỏ ảnh đang chờ" variant="ghost" onPress={() => { void boAnh(pendingPhoto); setPendingPhoto(null); setLoiAnh(null); }}/></> : null}
        </View>
      </NhomHang>
      <SectionHeader title="Về Rủ Đi" />
      <NhomHang>
        <ListRow
          icon="document-text-outline"
          iconTreatment="plain"
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
          tone="accent"
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
  nhomGiaoDien: { gap: 8 },
  luaChonGiaoDien: { borderWidth: StyleSheet.hairlineWidth, borderRadius: 6 },
  hangGiaoDien: { flexDirection: "row", alignItems: "center", gap: 16, minHeight: 52, paddingHorizontal: 12, paddingVertical: 10 },
  dauChon: { width: 20, height: 20, borderWidth: 1.4, borderRadius: 10, alignItems: "center", justifyContent: "center" },
  chamChon: { width: 6, height: 6, borderRadius: 3 },
  khoi: { gap: 12, paddingVertical: 6 },
  hang: { flexDirection: "row", alignItems: "center", gap: 12 },
  loiHang: { paddingBottom: 6 },
  hangChu: { flex: 1, gap: 2 },
  chips: { flexDirection: "row", flexWrap: "wrap", gap: 8 },
});
