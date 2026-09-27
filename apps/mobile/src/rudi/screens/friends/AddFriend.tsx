/**
 * Thêm bạn bằng số điện thoại (M2).
 *
 * Two steps and two server calls: the number resolves to a person the server
 * already knows (`POST /friends/lookup` -- answers an id and a display name,
 * never a number, and refuses numbers nobody has signed in or been named
 * with), then a request is sent (`POST /friends/requests`). Asking is not
 * adding: the other person accepts on their own phone.
 *
 * The number typed here never leaves this screen except inside that one
 * lookup body, and is not stored.
 */
import { Redirect, useRouter } from "expo-router";
import { useRef, useState } from "react";
import { StyleSheet, Text, View } from "react-native";

import { ApiError, newAttempt, thongDiepNguoiDoc, type Attempt } from "../../../api";
import { guiLoiMoi, timBanTheoSo, type NguoiTimDuoc } from "../../../screens/ca-nhan/ban-be";
import { chuanHoaSo } from "../../../screens/vao-cua/danh-tinh";
import { useRudiSession } from "../../session";
import { tenThat } from "../../ten-giu-cho";
import { bongGiay, mucNguoi, typography, useRudiTheme } from "../../theme";
import { Heading, RudiButton, RudiScreen, TopBar } from "../../ui";
import { HinhNhan } from "../../ui/Avatar";
import { DauLon } from "../../ui/DauLon";
import { ONhapMuc } from "../../ui/ONhapMuc";
import { StampButton } from "../../ui/StampButton";
import { Washi } from "../../ui/Washi";

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
  const [phone, setPhone] = useState("");
  const [trang, setTrang] = useState<Trang>({ pha: "nhap" });
  const lanBam = useRef<{ id: string; attempt: Attempt } | null>(null);

  if (!phienDaDoc) return null;
  if (phien === null) return <Redirect href="/welcome" />;

  const tim = async () => {
    const sach = phone.trim();
    if (chuanHoaSo(sach) === null) {
      setTrang({ pha: "hong", loi: "Chưa đúng dạng số di động Việt Nam." });
      return;
    }
    setTrang({ pha: "dang-tim" });
    try {
      const nguoi = await timBanTheoSo(sach, phien.person_id);
      if (nguoi.person_id === phien.person_id) {
        setTrang({ pha: "hong", loi: "Đó là số của chính bạn." });
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
          title={`Đã gửi lời mời tới ${tenThat(trang.nguoi.display_name) ?? `số đuôi ${duoiSo(phone)}`}`}
          subtitle="Khi người ấy đồng ý, hai bạn là bạn bè và thấy tường của nhau."
        />
        <View style={[styles.danhThiep, { backgroundColor: colors.card, borderColor: colors.lineStrong }, bongGiay(1, dark)]}>
          <HinhNhan name={tenThat(trang.nguoi.display_name) ?? "?"} personId={trang.nguoi.person_id} size={56} />
          <DauLon co="vua" dong nhan="Đã gửi" tilt={-4} tone="ink" />
        </View>
        <RudiButton label="Về danh sách bạn" onPress={() => router.back()} />
      </RudiScreen>
    );
  }

  const ban = trang.pha === "dang-tim" || trang.pha === "dang-gui";
  return (
    <RudiScreen contentStyle={styles.screen} testID="add-friend-screen">
      <TopBar title="Thêm bạn" />
      <Heading
        title="Thêm bạn bằng số điện thoại"
        subtitle="Chỉ tìm được người đã dùng Rủ Đi hoặc đã được ai đó đặt tên bằng số này. Số không được lưu."
      />
      {/* A calling card: the number written on it, and, once found, the
          person standing on it in their own ink (ADR-0037 D1, D6). */}
      <View style={[styles.danhThiep, { backgroundColor: colors.card, borderColor: colors.lineStrong }, bongGiay(1, dark)]} testID="danh-thiep">
        <Washi style={styles.washi} tilt={-2} />
        {trang.pha === "tim-thay" || trang.pha === "dang-gui" ? (
          <View style={styles.nguoi}>
            <HinhNhan name={tenThat(trang.nguoi.display_name) ?? "?"} personId={trang.nguoi.person_id} size={56} />
            <View style={styles.flex}>
              <Text style={[typography.caption, { color: colors.inkSoft }]}>Tìm thấy theo số điện thoại</Text>
              <Text style={[typography.title, { color: tenThat(trang.nguoi.display_name) === null ? colors.ink : mucNguoi(trang.nguoi.person_id, dark) }]}>
                {tenThat(trang.nguoi.display_name) ?? "Người chưa đặt tên"}
              </Text>
              {/* Somebody who has not chosen a name yet only has the server's
                  placeholder; the tail of the number the searcher typed is what
                  tells them they found the right person (QA 23/09). */}
              <Text style={[typography.caption, { color: colors.inkSoft }]}>
                Số đuôi {duoiSo(phone)} · {tenThat(trang.nguoi.display_name) === null ? "chưa đặt tên trên Rủ Đi" : "đã dùng Rủ Đi"}
              </Text>
            </View>
          </View>
        ) : null}
        <ONhapMuc
          accessibilityLabel="Ô số điện thoại bạn"
          autoComplete="tel"
          editable={!ban}
          keyboardType="phone-pad"
          label="Số điện thoại"
          onChangeText={(t) => {
            setPhone(t);
            if (trang.pha !== "nhap") setTrang({ pha: "nhap" });
          }}
          placeholder="Số di động của bạn ấy"
          textContentType="telephoneNumber"
          value={phone}
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

/** The last three digits of what the searcher typed; the number itself is not stored. */
function duoiSo(so: string): string {
  const chuSo = so.replace(/\D/g, "");
  return chuSo.length >= 3 ? `•••${chuSo.slice(-3)}` : "này";
}

const styles = StyleSheet.create({
  screen: { gap: 20, maxWidth: 560 },
  flex: { flex: 1 },
  danhThiep: { borderWidth: 1, borderRadius: 6, padding: 16, paddingTop: 22, gap: 14, alignItems: "stretch" },
  washi: { position: "absolute", top: -10, left: 20, width: 72 },
  nguoi: { flexDirection: "row", alignItems: "center", gap: 14 },
});
