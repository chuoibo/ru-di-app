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
import { StyleSheet, Text, View } from "react-native";

import { newAttempt } from "../../../api";
import { boAnh, chonAnh, nenVaDung, type GiaiDoanTaiAnh, type TempPhoto } from "../../ky-niem/chon-anh";
import { taiAnhCaNhan } from "../../nguoi/anh-ca-nhan";
import { loiRaChu } from "../../nguoi/ho-so-nguoi";
import { useRudiSession } from "../../session";
import { dangStory } from "../../story/story";
import { typography, useRudiTheme } from "../../theme";
import { Card, Field, RudiButton, RudiScreen, TopBar } from "../../ui";

const TRAN_CHU_THICH = 200;

export function DangStoryScreen() {
  const router = useRouter();
  const { colors, radius } = useRudiTheme();
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
      <Card style={styles.khungAnh}>
        {anh === null ? (
          <View style={[styles.anhTrong, { borderColor: colors.lineStrong, borderRadius: radius.small }]}>
            <Text style={[typography.caption, { color: colors.inkFaint }]}>Chưa có ảnh nào.</Text>
          </View>
        ) : (
          <Image accessibilityLabel="Ảnh đã chọn" contentFit="cover" source={{ uri: anh.uri }} style={[styles.anhXem, { borderRadius: radius.small }]} />
        )}
        <View style={styles.chips}>
          <RudiButton compact disabled={dangGui} full={false} icon="images-outline" label={anh === null ? "Chọn ảnh" : "Chọn ảnh khác"} onPress={() => void chonAnhMoi()} variant="outline" />
          {anh !== null ? <RudiButton compact disabled={dangGui} full={false} label="Bỏ ảnh" onPress={() => void boAnhDaChon()} variant="ghost" /> : null}
        </View>
        {cauGiaiDoan ? <Text style={[typography.caption, { color: colors.inkSoft }]}>{cauGiaiDoan}</Text> : null}
      </Card>
      <Card>
        <Field
          accessibilityLabel="Ô chú thích"
          label="Chú thích, nếu muốn"
          maxLength={TRAN_CHU_THICH}
          multiline
          numberOfLines={3}
          onChangeText={setChuThich}
          placeholder="Một câu cho tấm ảnh."
          value={chuThich}
        />
        <Text style={[typography.caption, { color: conLai < 20 ? colors.warn : colors.inkFaint }]}>Còn {conLai} ký tự.</Text>
      </Card>
      {loi ? (
        <Card>
          <Text style={[typography.body, { color: colors.warn }]}>{loi}</Text>
        </Card>
      ) : null}
      <RudiButton disabled={!guiDuoc} icon="aperture-outline" label="Đăng story" loading={dangGui} onPress={() => void gui()} />
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  khungAnh: { gap: 10 },
  anhXem: { width: "100%", aspectRatio: 3 / 4 },
  anhTrong: { width: "100%", aspectRatio: 3 / 4, alignItems: "center", justifyContent: "center", borderWidth: 1, borderStyle: "dashed" },
  chips: { flexDirection: "row", flexWrap: "wrap", gap: 8 },
});
