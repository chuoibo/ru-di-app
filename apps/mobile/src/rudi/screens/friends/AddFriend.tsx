/** Find an existing account by username, then ask for friendship. */
import { useRouter } from "expo-router";
import { useRef, useState } from "react";
import { StyleSheet, Text, View } from "react-native";

import { ApiError, newAttempt, thongDiepNguoiDoc, type Attempt } from "../../../api";
import { guiLoiMoi, timBanTheoUsername, type NguoiTimDuoc } from "../../../screens/ca-nhan/ban-be";
import { validUsername } from "../../account";
import { useRudiSession } from "../../session";
import { tenThat } from "../../ten-giu-cho";
import { bongGiay, mucNguoi, typography, useRudiTheme } from "../../theme";
import { Heading, RudiButton, RudiScreen, TopBar } from "../../ui";
import { HinhNhan } from "../../ui/Avatar";
import { DauLon } from "../../ui/DauLon";
import { ONhapMuc } from "../../ui/ONhapMuc";
import { StampButton } from "../../ui/StampButton";
import { Washi } from "../../ui/Washi";
import { CuaDangNhap } from "../../ui/CuaDangNhap";
import { luiVeVe } from "../../lui-ve";

type Trang =
  | { pha: "nhap" }
  | { pha: "dang-tim" }
  | { pha: "tim-thay"; nguoi: NguoiTimDuoc }
  | { pha: "dang-gui"; nguoi: NguoiTimDuoc }
  | { pha: "da-gui"; nguoi: NguoiTimDuoc }
  | { pha: "hong"; loi: string };

export function AddFriendScreen() {
  const router = useRouter();
  const { colors, dark } = useRudiTheme();
  const { phien, phienDaDoc } = useRudiSession();
  const [username, setUsername] = useState("");
  const [trang, setTrang] = useState<Trang>({ pha: "nhap" });
  const lanBam = useRef<{ id: string; attempt: Attempt } | null>(null);

  if (!phienDaDoc) return null;
  if (phien === null) return <CuaDangNhap />;

  const tim = async () => {
    const sach = username.trim();
    if (!validUsername(sach)) {
      setTrang({ pha: "hong", loi: "Tên tài khoản gồm 3–32 chữ, số, dấu chấm hoặc gạch dưới." });
      return;
    }
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
    }
  };

  const gui = async (nguoi: NguoiTimDuoc) => {
    if (lanBam.current === null || lanBam.current.id !== nguoi.person_id) {
      lanBam.current = { id: nguoi.person_id, attempt: newAttempt() };
    }
    setTrang({ pha: "dang-gui", nguoi });
    try {
      await guiLoiMoi(nguoi.person_id, phien.person_id, lanBam.current.attempt);
      setTrang({ pha: "da-gui", nguoi });
    } catch (error) {
      setTrang({
        pha: "hong",
        loi: error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null),
      });
    }
  };

  if (trang.pha === "da-gui") {
    return (
      <RudiScreen testID="add-friend-screen">
        <TopBar title="Thêm bạn" />
        <Heading
          title={`Đã gửi lời mời tới ${tenThat(trang.nguoi.display_name) ?? `@${username.replace(/^@/, "")}`}`}
          subtitle="Khi người ấy đồng ý, hai bạn có thể nhắn riêng và xem những bài chia sẻ với bạn bè."
        />
        <View style={[styles.danhThiep, { backgroundColor: colors.card, borderColor: colors.lineStrong }, bongGiay(1, dark)]}>
          <HinhNhan name={tenThat(trang.nguoi.display_name) ?? "?"} personId={trang.nguoi.person_id} size={56} />
          <DauLon co="vua" dong nhan="Đã gửi" tilt={-4} tone="ink" />
        </View>
        <RudiButton label="Về danh sách bạn" onPress={() => luiVeVe(router as never, "/friends")} />
      </RudiScreen>
    );
  }

  const ban = trang.pha === "dang-tim" || trang.pha === "dang-gui";
  return (
    <RudiScreen contentStyle={styles.screen} testID="add-friend-screen">
      <TopBar title="Thêm bạn" />
      <Heading
        title="Thêm bạn bằng username"
        subtitle="Nhập @username của bạn ấy. Người dùng có thể tắt cho phép tìm kiếm."
      />
      {/* A calling card: the username written on it, and, once found, the
          person standing on it in their own ink (ADR-0037 D1, D6). */}
      <View style={[styles.danhThiep, { backgroundColor: colors.card, borderColor: colors.lineStrong }, bongGiay(1, dark)]} testID="danh-thiep">
        <Washi style={styles.washi} tilt={-2} />
        {trang.pha === "tim-thay" || trang.pha === "dang-gui" ? (
          <View style={styles.nguoi}>
            <HinhNhan name={tenThat(trang.nguoi.display_name) ?? "?"} personId={trang.nguoi.person_id} size={56} />
            <View style={styles.flex}>
              <Text style={[typography.caption, { color: colors.inkSoft }]}>Tìm thấy theo username</Text>
              <Text style={[typography.title, { color: tenThat(trang.nguoi.display_name) === null ? colors.ink : mucNguoi(trang.nguoi.person_id, dark) }]}>
                {tenThat(trang.nguoi.display_name) ?? "Người chưa đặt tên"}
              </Text>
              {/* The username identifies the account even before its owner
                  chooses a display name. */}
              <Text style={[typography.caption, { color: colors.inkSoft }]}>
                @{username.replace(/^@/, "")} · {tenThat(trang.nguoi.display_name) === null ? "chưa đặt tên trên Rủ Đi" : "đã dùng Rủ Đi"}
              </Text>
            </View>
          </View>
        ) : null}
        <ONhapMuc
          accessibilityLabel="Ô username bạn"
          autoComplete="username"
          editable={!ban}
          autoCapitalize="none"
          label="Tên tài khoản"
          onChangeText={(t) => {
            setUsername(t);
            if (trang.pha !== "nhap") setTrang({ pha: "nhap" });
          }}
          placeholder="Tên tài khoản của bạn ấy"
          textContentType="username"
          value={username}
        />
      </View>
      {trang.pha === "tim-thay" || trang.pha === "dang-gui" ? (
        <StampButton disabled={ban} label="Gửi lời mời" loading={trang.pha === "dang-gui"} onPress={() => void gui(trang.nguoi)} size="vua" tilt={-1} />
      ) : (
        <RudiButton disabled={ban} label="Tìm" loading={trang.pha === "dang-tim"} onPress={() => void tim()} />
      )}
      {trang.pha === "hong" ? (
        <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.warn }]}>{trang.loi}</Text>
      ) : null}
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  screen: { gap: 20, maxWidth: 560 },
  flex: { flex: 1 },
  danhThiep: { borderWidth: 1, borderRadius: 6, padding: 16, paddingTop: 22, gap: 14, alignItems: "stretch" },
  washi: { position: "absolute", top: -10, left: 20, width: 72 },
  nguoi: { flexDirection: "row", alignItems: "center", gap: 14 },
});
