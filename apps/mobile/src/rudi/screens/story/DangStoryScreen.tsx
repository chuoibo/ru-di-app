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
import { useRef, useState } from "react";
import { Ionicons } from "@expo/vector-icons";
import { Pressable, StyleSheet, Text, View } from "react-native";

import { attemptFor, type Attempt } from "../../../api";
import { boAnh, chonAnh, nenVaDung, type GiaiDoanTaiAnh, type TempPhoto } from "../../ky-niem/chon-anh";
import { nguonAnhBai, taiAnhCaNhan } from "../../nguoi/anh-ca-nhan";
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
  const [anhDaTai, setAnhDaTai] = useState<string | null>(null);
  const attempts = useRef<Record<string, Attempt>>({});
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
      setAnhDaTai(null);
      setAnh(daChon);
    } catch (error) {
      setLoi(loiRaChu(error));
    }
  };

  const boAnhDaChon = async () => {
    if (dangGui) return;
    if (anh !== null) await boAnh(anh);
    setAnh(null);
    setAnhDaTai(null);
  };

  if (!phienDaDoc) return null;

  const guiDuoc = phien !== null && (anh !== null || anhDaTai !== null) && chuThich.length <= TRAN_CHU_THICH && !dangGui;

  const gui = async () => {
    if (phien === null || (anh === null && anhDaTai === null)) return;
    setDangGui(true);
    setLoi(null);
    let uploadFinished = false;
    try {
      let imageUrl = anhDaTai;
      if (anh !== null) {
        try {
          const daTai = await nenVaDung(anh, (nen) => taiAnhCaNhan(nen, phien.person_id), setGiaiDoan);
          imageUrl = daTai.url;
          setAnhDaTai(daTai.url);
          uploadFinished = true;
        } finally {
          setAnh(null);
        }
      }
      if (imageUrl === null) return;
      await dangStory(imageUrl, chuThich, phien.person_id, attemptFor(attempts.current, JSON.stringify({ imageUrl, chuThich })));
      router.back();
    } catch (error) {
      const repickHint = anh !== null && !uploadFinished ? " Chọn lại ảnh rồi thử lần nữa." : "";
      setLoi(loiRaChu(error) + repickHint);
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
        {/* Kept after a failed story write: the photo already went up, so a
            retry posts it again instead of asking for it twice. */}
        {anh === null && anhDaTai !== null && phien !== null ? (
          <Image accessibilityLabel="Ảnh đã tải lên, đang chờ đăng story" contentFit="cover" source={nguonAnhBai(anhDaTai, phien.person_id) ?? undefined} style={styles.anhXem} />
        ) : null}
        {anh === null && anhDaTai === null ? (
          <Pressable accessibilityLabel="Chưa có ảnh nào, chạm để chọn" accessibilityRole="button" disabled={dangGui} onPress={() => void chonAnhMoi()} style={[styles.anhTrong, { borderColor: colors.lineStrong, backgroundColor: colors.ground }]}>
            <Nep gap="trang" pose="giu-khung" size={88} />
            <Text style={[typography.caption, { color: colors.inkSoft }]}>Chưa có ảnh nào. Chạm để chọn.</Text>
          </Pressable>
        ) : null}
        {anh !== null ? (
          <Image accessibilityLabel="Ảnh đã chọn" contentFit="cover" source={{ uri: anh.uri }} style={styles.anhXem} />
        ) : null}
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
        <RudiButton compact disabled={dangGui} full={false} icon="images-outline" label={anh === null && anhDaTai === null ? "Chọn ảnh" : "Chọn ảnh khác"} onPress={() => void chonAnhMoi()} variant="outline" />
        {anh !== null || anhDaTai !== null ? <RudiButton compact disabled={dangGui} full={false} label="Bỏ ảnh" onPress={() => void boAnhDaChon()} variant="ghost" /> : null}
      </View>
      {cauGiaiDoan ? <Text style={[typography.caption, { color: colors.inkSoft }]}>{cauGiaiDoan}</Text> : null}
      {loi ? <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.warn }]}>{loi}</Text> : null}
      <StampButton
        disabled={!guiDuoc}
        label="Đăng story"
        loading={dangGui}
        lyDo={anh === null && anhDaTai === null ? "Chọn một tấm ảnh trước đã." : chuThich.length > TRAN_CHU_THICH ? `Chú thích dài quá ${TRAN_CHU_THICH} chữ.` : undefined}
        onPress={() => void gui()}
        size="vua"
        tilt={-1}
      />
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
