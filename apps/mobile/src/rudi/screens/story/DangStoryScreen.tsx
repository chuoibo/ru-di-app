/**
 * Đăng story (L4, ADR-0022 §2.3): one photograph of one's own, an optional
 * caption, to one's friends, for 24 hours.
 *
 * Same order as a post with a picture: the photo goes up first as a personal
 * photograph nobody may read yet (`POST /people/me/photos`), and the story
 * that shows it comes second. No audience row: the story has one audience and
 * the screen says so in a sentence instead of offering a choice that is not
 * one.
 */
import { Image } from "expo-image";
import { useRouter } from "expo-router";
import { useState } from "react";
import { Ionicons } from "@expo/vector-icons";
import { Pressable, StyleSheet, Text, View } from "react-native";

import { newAttempt } from "../../../api";
import { boAnh, chonAnh, nenVaDung, type GiaiDoanTaiAnh, type TempPhoto } from "../../ky-niem/chon-anh";
import { taiAnhCaNhan } from "../../nguoi/anh-ca-nhan";
import { loiRaChu } from "../../nguoi/ho-so-nguoi";
import { useRudiSession } from "../../session";
import { dangStory } from "../../story/story";
import { bongGiay, typography, useRudiTheme } from "../../theme";
import { RudiButton, RudiScreen, TopBar } from "../../ui";
import { Nep } from "../../ui/art/Nep";
import { ONhapMuc } from "../../ui/ONhapMuc";
import { StampButton } from "../../ui/StampButton";

const TRAN_CHU_THICH = 200;

export function DangStoryScreen() {
  const router = useRouter();
  const { colors, dark } = useRudiTheme();
  const { phien, phienDaDoc } = useRudiSession();
  const [anh, setAnh] = useState<TempPhoto | null>(null);
  const [chuThich, setChuThich] = useState("");
  const [giaiDoan, setGiaiDoan] = useState<GiaiDoanTaiAnh | null>(null);
  const [dangGui, setDangGui] = useState(false);
  const [loi, setLoi] = useState<string | null>(null);

  const chonAnhMoi = async () => {
    if (dangGui) return;
    setLoi(null);
    try {
      const daChon = await chonAnh();
      if (daChon === null) return;
      if (anh !== null) await boAnh(anh);
      setAnh(daChon);
    } catch (error) {
      setLoi(loiRaChu(error));
    }
  };

  const boAnhDaChon = async () => {
    if (anh === null || dangGui) return;
    await boAnh(anh);
    setAnh(null);
  };

  if (!phienDaDoc) return null;

  const guiDuoc = phien !== null && anh !== null && chuThich.length <= TRAN_CHU_THICH && !dangGui;

  const gui = async () => {
    if (phien === null || anh === null) return;
    setDangGui(true);
    setLoi(null);
    try {
      const daTai = await nenVaDung(anh, (nen) => taiAnhCaNhan(nen, phien.person_id), setGiaiDoan);
      await dangStory(daTai.url, chuThich, phien.person_id, newAttempt());
      setAnh(null);
      router.back();
    } catch (error) {
      setLoi(loiRaChu(error));
    } finally {
      setGiaiDoan(null);
      setDangGui(false);
    }
  };

  const cauGiaiDoan = giaiDoan === "chuan-bi-anh" ? "Đang chuẩn bị ảnh…" : giaiDoan === "dang-gui" ? "Đang tải ảnh lên…" : null;
  const conLai = TRAN_CHU_THICH - chuThich.length;

  return (
    <RudiScreen testID="dang-story-screen">
      <TopBar title="Đăng story" />
      <Text style={[typography.body, { color: colors.inkSoft }]}>Một tấm ảnh, chỉ bạn bè thấy, trong 24 giờ.</Text>
      {/* A polaroid that lasts a day (ADR-0037 D1): the picture, then the
          caption on its white margin, and an hourglass for the 24 hours. The
          empty frame is itself the way to pick the photo. */}
      <View style={[styles.polaroid, { backgroundColor: colors.card, borderColor: colors.lineStrong }, bongGiay(2, dark)]}>
        {anh === null ? (
          <Pressable accessibilityLabel="Chưa có ảnh nào, chạm để chọn" accessibilityRole="button" disabled={dangGui} onPress={() => void chonAnhMoi()} style={[styles.anhTrong, { borderColor: colors.lineStrong, backgroundColor: colors.ground }]}>
            <Nep gap="trang" pose="giu-khung" size={88} />
            <Text style={[typography.caption, { color: colors.inkSoft }]}>Chưa có ảnh nào. Chạm để chọn.</Text>
          </Pressable>
        ) : (
          <Image accessibilityLabel="Ảnh đã chọn" contentFit="cover" source={{ uri: anh.uri }} style={styles.anhXem} />
        )}
        <ONhapMuc
          accessibilityLabel="Ô chú thích"
          label="Chú thích, nếu muốn"
          maxLength={TRAN_CHU_THICH}
          multiline
          numberOfLines={2}
          onChangeText={setChuThich}
          placeholder="Một câu cho tấm ảnh."
          value={chuThich}
        />
        <View style={styles.dongCuoi}>
          <Ionicons color={colors.inkSoft} name="hourglass-outline" size={16} />
          <Text style={[typography.caption, styles.flex, { color: colors.inkSoft }]}>24 giờ</Text>
          <Text style={[typography.caption, { color: conLai < 20 ? colors.warn : colors.inkSoft }]}>Còn {conLai} ký tự.</Text>
        </View>
      </View>
      <View style={styles.chips}>
        <RudiButton compact disabled={dangGui} full={false} icon="images-outline" label={anh === null ? "Chọn ảnh" : "Chọn ảnh khác"} onPress={() => void chonAnhMoi()} variant="outline" />
        {anh !== null ? <RudiButton compact disabled={dangGui} full={false} label="Bỏ ảnh" onPress={() => void boAnhDaChon()} variant="ghost" /> : null}
      </View>
      {cauGiaiDoan ? <Text style={[typography.caption, { color: colors.inkSoft }]}>{cauGiaiDoan}</Text> : null}
      {loi ? <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.warn }]}>{loi}</Text> : null}
      <StampButton disabled={!guiDuoc} label="Đăng story" loading={dangGui} onPress={() => void gui()} size="vua" tilt={-1} />
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  flex: { flex: 1 },
  // A polaroid: even sides, the deep margin under the picture for the words.
  polaroid: { gap: 12, padding: 12, paddingBottom: 18, borderWidth: 1, borderRadius: 3, alignSelf: "center", width: "100%", maxWidth: 420 },
  anhXem: { width: "100%", aspectRatio: 3 / 4 },
  anhTrong: { width: "100%", aspectRatio: 3 / 4, alignItems: "center", justifyContent: "center", gap: 8, borderWidth: 1, borderStyle: "dashed" },
  dongCuoi: { flexDirection: "row", alignItems: "center", gap: 6 },
  chips: { flexDirection: "row", flexWrap: "wrap", gap: 8 },
});
