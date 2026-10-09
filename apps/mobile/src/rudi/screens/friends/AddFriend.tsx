/** Find an existing account by username, then ask for friendship. */
import { useRouter } from "expo-router";
import { useRef, useState } from "react";
import { StyleSheet, Text, View } from "react-native";

import { ApiError, newAttempt, thongDiepNguoiDoc, type Attempt } from "../../../api";
import { guiLoiMoi, timBanTheoUsername, type NguoiTimDuoc } from "../../../screens/ca-nhan/ban-be";
import { canonicalUsername, validUsername } from "../../account";
import { useRudiSession } from "../../session";
import { tenThat } from "../../ten-giu-cho";
import { bongGiay, mucNguoi, typography, useRudiTheme } from "../../theme";
import { Heading, RudiButton, RudiScreen, TopBar } from "../../ui";
import { HinhNhan } from "../../ui/Avatar";
import { AnimatedEnvelope } from "../../asset-pack/react-native/AnimatedEnvelope";
import { ONhapMuc } from "../../ui/ONhapMuc";
import { StampButton } from "../../ui/StampButton";
import { Washi } from "../../ui/Washi";
import { CuaDangNhap } from "../../ui/CuaDangNhap";
import { CauTaiCho } from "../../ui/CauTaiCho";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { luiVeVe } from "../../lui-ve";

type Trang =
  | { pha: "nhap" }
  | { pha: "dang-tim" }
  | { pha: "tim-thay"; nguoi: NguoiTimDuoc }
  | { pha: "dang-gui"; nguoi: NguoiTimDuoc }
  | { pha: "da-gui"; nguoi: NguoiTimDuoc; eventId: string }
  | { pha: "hong"; loi: string; nguoi?: NguoiTimDuoc };

/** The handle as the server spelled it, not as it was typed («@Ten_Ban » finds «ten_ban»). */
const tenTaiKhoan = (nguoi: NguoiTimDuoc, daGo: string) => `@${nguoi.username ?? canonicalUsername(daGo)}`;

export function AddFriendScreen() {
  const router = useRouter();
  const insets = useSafeAreaInsets();
  const { colors, dark } = useRudiTheme();
  const { phien, phienDaDoc } = useRudiSession();
  const [username, setUsername] = useState("");
  const [trang, setTrang] = useState<Trang>({ pha: "nhap" });
  const lock = useRef(false);
  const lanBam = useRef<{ id: string; attempt: Attempt } | null>(null);

  if (!phienDaDoc) return null;
  if (phien === null) return <CuaDangNhap />;

  const tim = async () => {
    if (lock.current) return;
    const sach = username.trim();
    if (!validUsername(sach)) {
      setTrang({ pha: "hong", loi: "Tên tài khoản gồm 3–32 chữ, số, dấu chấm hoặc gạch dưới." });
      return;
    }
    lock.current = true;
    setTrang({ pha: "dang-tim" });
    try {
      const nguoi = await timBanTheoUsername(sach, phien.person_id);
      if (nguoi.person_id === phien.person_id) {
        setTrang({ pha: "hong", loi: "Đó là tài khoản của chính bạn." });
        return;
      }
      setTrang({ pha: "tim-thay", nguoi });
    } catch (error) {
      setTrang({
        pha: "hong",
        loi: error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null),
      });
    } finally { lock.current = false; }
  };

  const gui = async (nguoi: NguoiTimDuoc) => {
    if (lock.current) return;
    lock.current = true;
    if (lanBam.current === null || lanBam.current.id !== nguoi.person_id) {
      lanBam.current = { id: nguoi.person_id, attempt: newAttempt() };
    }
    setTrang({ pha: "dang-gui", nguoi });
    try {
      const response = await guiLoiMoi(nguoi.person_id, phien.person_id, lanBam.current.attempt);
      setTrang({ pha: "da-gui", nguoi, eventId: response.id });
    } catch (error) {
      setTrang({
        pha: "hong", nguoi,
        loi: error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null),
      });
    } finally { lock.current = false; }
  };

  if (trang.pha === "da-gui") {
    const nguoi = trang.nguoi;
    return (
      <RudiScreen material="quiet" cot="form" avoidKeyboard bottomInset={16} footerInset={insets.bottom + 12} testID="add-friend-screen"
        header={<TopBar title="Thêm bạn" />}
        footer={<RudiButton label="Về danh sách bạn" onPress={() => luiVeVe(router as never, "/friends")} />}
      >
        <Heading
          title="Lời mời đã gửi"
          subtitle="Chờ bạn ấy đồng ý để kết nối."
        />
        <View style={[styles.danhThiep, { backgroundColor: colors.card, borderColor: colors.lineStrong }, bongGiay(1, dark)]}>
          <View style={styles.nguoi}>
            <HinhNhan name={tenThat(nguoi.display_name) ?? "?"} personId={nguoi.person_id} size={48} />
            <View style={styles.flex}>
              <Text style={[typography.title, { color: colors.ink }]}>{tenThat(nguoi.display_name) ?? "Người chưa đặt tên"}</Text>
              <Text style={[typography.caption, { color: colors.inkSoft }]}>{tenTaiKhoan(nguoi, username)}</Text>
            </View>
          </View>
          <View style={styles.art} pointerEvents="none" accessible={false}><AnimatedEnvelope eventKey={trang.eventId} width={104} /></View>
        </View>
      </RudiScreen>
    );
  }

  const nguoi = "nguoi" in trang ? trang.nguoi : undefined;
  const ban = trang.pha === "dang-tim" || trang.pha === "dang-gui";
  return (
    <RudiScreen material="quiet" cot="form" avoidKeyboard bottomInset={16} footerInset={insets.bottom + 12} contentStyle={styles.screen} testID="add-friend-screen"
      header={<TopBar title="Thêm bạn" />}
      footer={<View style={styles.footer}>
        <CauTaiCho cau={trang.pha === "hong" ? trang.loi : null} />
        {nguoi ? <StampButton disabled={ban} label="Gửi lời mời" loading={trang.pha === "dang-gui"} onPress={() => void gui(nguoi)} size="vua" tilt={-1} />
          : <RudiButton disabled={ban} icon="search-outline" label="Tìm" loading={trang.pha === "dang-tim"} onPress={() => void tim()} />}
      </View>}
    >
      <Heading
        title="Tìm một người bạn"
        subtitle="Nhập tên tài khoản để tìm đúng người."
      />
      {/* A calling card: the username written on it, and, once found, the
          person standing on it in their own ink (ADR-0037 D1, D6). */}
      <View style={[styles.danhThiep, { backgroundColor: colors.card, borderColor: colors.lineStrong }, bongGiay(1, dark)]} testID="danh-thiep">
        <Washi style={styles.washi} tilt={-2} />
        {nguoi ? (
          <View style={styles.nguoi}>
            <HinhNhan name={tenThat(nguoi.display_name) ?? "?"} personId={nguoi.person_id} size={56} />
            <View style={styles.flex}>
              <Text style={[typography.caption, { color: colors.inkSoft }]}>Người nhận lời mời</Text>
              <Text style={[typography.title, { color: tenThat(nguoi.display_name) === null ? colors.ink : mucNguoi(nguoi.person_id, dark) }]}>
                {tenThat(nguoi.display_name) ?? "Người chưa đặt tên"}
              </Text>
              {/* The username identifies the account even before its owner
                  chooses a display name. */}
              <Text style={[typography.caption, { color: colors.inkSoft }]}>
                {tenTaiKhoan(nguoi, username)} · {tenThat(nguoi.display_name) === null ? "chưa đặt tên trên Rủ Đi" : "đã dùng Rủ Đi"}
              </Text>
            </View>
          </View>
        ) : null}
        <ONhapMuc
          accessibilityLabel="Tên tài khoản của bạn ấy"
          autoComplete="username"
          editable={!ban}
          autoCapitalize="none"
          autoCorrect={false}
          label="Tên tài khoản"
          onChangeText={(t) => {
            setUsername(t);
            if (trang.pha !== "nhap") setTrang({ pha: "nhap" });
          }}
          placeholder="Tên tài khoản của bạn ấy"
          textContentType="username"
          returnKeyType="search"
          onSubmitEditing={() => void tim()}
          value={username}
        />
      </View>
      <Text style={[typography.note, { color: colors.inkSoft }]}>Chỉ tài khoản cho phép tìm kiếm mới hiện ở đây. Lời mời cần được bạn ấy đồng ý.</Text>
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  screen: { gap: 24, maxWidth: 560 },
  flex: { flex: 1, minWidth: 0 },
  footer: { gap: 8 },
  art: { alignSelf: "center" },
  danhThiep: { borderWidth: 1, borderRadius: 6, padding: 16, paddingTop: 22, gap: 14, alignItems: "stretch" },
  washi: { position: "absolute", top: -10, left: 20, width: 72 },
  nguoi: { flexDirection: "row", alignItems: "center", gap: 14 },
});
