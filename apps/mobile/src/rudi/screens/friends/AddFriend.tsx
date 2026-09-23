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
import { typography, useRudiTheme } from "../../theme";
import { Field, Heading, RudiButton, RudiScreen, TopBar } from "../../ui";
import { HangNguoi } from "./HangNguoi";

type Trang =
  | { pha: "nhap" }
  | { pha: "dang-tim" }
  | { pha: "tim-thay"; nguoi: NguoiTimDuoc }
  | { pha: "dang-gui"; nguoi: NguoiTimDuoc }
  | { pha: "da-gui"; nguoi: NguoiTimDuoc }
  | { pha: "hong"; loi: string };

export function AddFriendScreen() {
  const router = useRouter();
  const { colors } = useRudiTheme();
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
      <View style={styles.form}>
        <Field
          accessibilityLabel="Ô số điện thoại bạn"
          autoComplete="tel"
          editable={!ban}
          icon="call-outline"
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
        {trang.pha === "tim-thay" || trang.pha === "dang-gui" ? (
          <>
            <Text style={[typography.caption, { color: colors.inkSoft }]}>Tìm thấy theo số điện thoại</Text>
            {/* Somebody who has not chosen a name yet only has the server's
                placeholder; the tail of the number the searcher typed is what
                tells them they found the right person (QA 23/09). */}
            <HangNguoi
              phu={`Số đuôi ${duoiSo(phone)} · ${tenThat(trang.nguoi.display_name) === null ? "chưa đặt tên trên Rủ Đi" : "đã dùng Rủ Đi"}`}
              ten={tenThat(trang.nguoi.display_name) ?? "Người chưa đặt tên"}
            />
            <RudiButton
              disabled={ban}
              label="Gửi lời mời"
              loading={trang.pha === "dang-gui"}
              onPress={() => void gui(trang.nguoi)}
            />
          </>
        ) : (
          <RudiButton disabled={ban} label="Tìm" loading={trang.pha === "dang-tim"} onPress={() => void tim()} />
        )}
        {trang.pha === "hong" ? (
          <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.warn }]}>{trang.loi}</Text>
        ) : null}
      </View>
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
  form: { gap: 14 },
});
