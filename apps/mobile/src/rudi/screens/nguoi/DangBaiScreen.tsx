/**
 * Đăng bài (M8): write one post, to one of F42's four audiences.
 *
 * The four audiences are a vocabulary, not a ladder (`bai-dang.ts`): `friends`
 * and `group` reach two disjoint sets and neither contains the other. They are
 * drawn as four rows, each carrying the sentence that names who it reaches --
 * not a slider, not a narrow-to-wide chip row, not a lock that opens in steps.
 *
 * Text only for now. A post's `image_url` has to point at a group photo, which
 * only members of that group may read, so an image on a `friends` or `public`
 * post would be an address most readers cannot open.
 */
import { Image } from "expo-image";
import { useRouter } from "expo-router";
import { useEffect, useState } from "react";
import { Ionicons } from "@expo/vector-icons";
import { Pressable, StyleSheet, Text, View } from "react-native";

import { newAttempt } from "../../../api";
import { boAnh, chonAnh, nenVaDung, type GiaiDoanTaiAnh, type TempPhoto } from "../../ky-niem/chon-anh";
import { taiAnhCaNhan } from "../../nguoi/anh-ca-nhan";
import {
  AUDIENCES,
  MAC_DINH_NGUOI_DOC,
  MUC_NGUOI_DOC,
  coTheDang,
  guiBai,
  type Audience,
} from "../../../screens/ca-nhan/bai-dang";
import { docNhomCuaToi, type NhomTomTat } from "../../../phien";
import { loiRaChu } from "../../nguoi/ho-so-nguoi";
import { laPair } from "../../nhan-rieng/nhan-rieng";
import { useRudiSession } from "../../session";
import { bongGiay, typography, useRudiTheme } from "../../theme";
import { Chip, Heading, RudiButton, RudiScreen, TopBar } from "../../ui";
import { NapGiay } from "../../ui/NapGiay";
import { ONhapMuc } from "../../ui/ONhapMuc";
import { StampButton } from "../../ui/StampButton";

export function DangBaiScreen() {
  const router = useRouter();
  const { colors, dark, radius } = useRudiTheme();
  const { phien, phienDaDoc } = useRudiSession();
  const [than, setThan] = useState("");
  const [muc, setMuc] = useState<Audience>(MAC_DINH_NGUOI_DOC);
  const [nhom, setNhom] = useState<NhomTomTat[]>([]);
  const [nhomChon, setNhomChon] = useState<string | null>(null);
  const [dangGui, setDangGui] = useState(false);
  const [loi, setLoi] = useState<string | null>(null);
  // ADR-0022 §2.1: a picture of one's own goes up first, as a personal
  // photograph nobody may read yet; the post that shows it comes second.
  const [anh, setAnh] = useState<TempPhoto | null>(null);
  const [giaiDoan, setGiaiDoan] = useState<GiaiDoanTaiAnh | null>(null);

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

  useEffect(() => {
    if (phien === null) return;
    let con = true;
    void (async () => {
      try {
        const ds = await docNhomCuaToi(phien.person_id);
        if (!con) return;
        // A pair is not a group to post to (ADR-0021 §2.5); only groups are offered.
        const dangO = ds.filter((n) => n.my_state === "active" && !laPair(n));
        setNhom(dangO);
        setNhomChon((truoc) => {
          // Statement form on purpose (see the id-default gate): this id is a
          // selection, never a label, and the gate reads shape rather than use.
          if (truoc !== null) return truoc;
          const dau = dangO[0];
          if (dau === undefined) return null;
          return dau.id;
        });
      } catch {
        // A group list that does not answer only costs the «Một nhóm» option;
        // the other three audiences still work, so this is not screen-fatal.
      }
    })();
    return () => {
      con = false;
    };
  }, [phien]);

  if (!phienDaDoc) return null;

  const form = { body: than, audience: muc, contextId: muc === "group" ? nhomChon : null };
  const guiDuoc = phien !== null && coTheDang(form) && !dangGui;

  const gui = async () => {
    if (phien === null) return;
    setDangGui(true);
    setLoi(null);
    try {
      let imageUrl: string | null = null;
      if (anh !== null) {
        const daTai = await nenVaDung(anh, (nen) => taiAnhCaNhan(nen, phien.person_id), setGiaiDoan);
        imageUrl = daTai.url;
        setAnh(null);
      }
      await guiBai(phien.person_id, { ...form, imageUrl }, newAttempt());
      router.replace(`/people/${phien.person_id}`);
    } catch (error) {
      setLoi(loiRaChu(error));
    } finally {
      setGiaiDoan(null);
      setDangGui(false);
    }
  };

  const cauGiaiDoan = giaiDoan === "chuan-bi-anh" ? "Đang chuẩn bị ảnh…" : giaiDoan === "dang-gui" ? "Đang tải ảnh lên…" : null;

  return (
    <RudiScreen testID="dang-bai-screen">
      <TopBar title="Đăng bài" />
      {/* The post is a page of a letter: ruled lines, the words in ink (ADR-0037 D1). */}
      <View style={[styles.trangThu, { backgroundColor: colors.card, borderColor: colors.lineStrong }, bongGiay(1, dark)]}>
        <ONhapMuc
          label="Bạn muốn kể gì?"
          multiline
          numberOfLines={5}
          onChangeText={setThan}
          placeholder="Chuyến vừa rồi, quán mới, hay chỉ một câu."
          value={than}
        />
      </View>
      {/* ADR-0022 §2.1: one photograph of one's own, optional; it goes up first
          as a personal picture and the post that shows it comes second. */}
      <View style={styles.khungAnh}>
        {anh === null ? (
          <Text style={[typography.caption, { color: colors.inkFaint }]}>Một tấm ảnh, nếu muốn. Ai đọc được bài thì xem được ảnh.</Text>
        ) : (
          <Image accessibilityLabel="Ảnh đã chọn" contentFit="cover" source={{ uri: anh.uri }} style={[styles.anhXem, { borderRadius: radius.small }]} />
        )}
        <View style={styles.chips}>
          <RudiButton compact disabled={dangGui} full={false} icon="images-outline" label={anh === null ? "Chọn ảnh" : "Chọn ảnh khác"} onPress={() => void chonAnhMoi()} variant="outline" />
          {anh !== null ? <RudiButton compact disabled={dangGui} full={false} label="Bỏ ảnh" onPress={() => void boAnhDaChon()} variant="ghost" /> : null}
        </View>
        {cauGiaiDoan ? <Text style={[typography.caption, { color: colors.inkSoft }]}>{cauGiaiDoan}</Text> : null}
      </View>
      <Heading title="Ai đọc được?" />
      {/* Four envelopes, one per audience; the chosen one is sealed. The rule
          behind the four sits under a flap, still one tap away. */}
      <View style={styles.phongBiLuoi}>
        {AUDIENCES.map((a) => {
          const chon = muc === a;
          return (
            <Pressable
              // Named so a driver (and a screen reader) can pick this one and
              // not the sentence under a neighbour, which mentions «Bạn bè» too.
              accessibilityLabel={`Mức người đọc: ${MUC_NGUOI_DOC[a].nhan}`}
              accessibilityRole="radio"
              accessibilityState={{ selected: chon }}
              key={a}
              onPress={() => setMuc(a)}
              style={({ pressed }) => [
                styles.phongBi,
                { backgroundColor: chon ? colors.accentSoft : colors.card, borderColor: chon ? colors.accent : colors.lineStrong, borderWidth: chon ? 2 : 1 },
                pressed && styles.bam,
              ]}
            >
              <View style={[styles.napPhongBi, { borderColor: chon ? colors.accent : colors.lineStrong }]} />
              <View style={styles.phongBiChu}>
                <Text style={[typography.label, { color: colors.ink }]}>{MUC_NGUOI_DOC[a].nhan}</Text>
                <Text numberOfLines={3} style={[typography.caption, { color: colors.inkSoft }]}>{MUC_NGUOI_DOC[a].giaiThich}</Text>
              </View>
              {chon ? <Ionicons color={colors.accent} name="checkmark-circle" size={20} style={styles.dauChon} /> : null}
            </Pressable>
          );
        })}
      </View>
      <NapGiay tieuDe="Vì sao bốn mức?">
        <Text style={[typography.body, { color: colors.ink }]}>Bốn mức không xếp từ hẹp tới rộng: bạn bè và nhóm là hai tập khác nhau.</Text>
      </NapGiay>
      {muc === "group" ? (
        <View style={styles.khoi}>
          <Text style={[typography.label, { color: colors.ink }]}>Nhóm nào?</Text>
          {nhom.length === 0 ? (
            <Text style={[typography.caption, { color: colors.inkFaint }]}>
              Bạn chưa ở nhóm nào đang hoạt động, nên chưa đăng cho nhóm được.
            </Text>
          ) : (
            <View style={styles.chips}>
              {nhom.map((n) => (
                <Chip
                  key={n.id}
                  label={n.display_name}
                  onPress={() => setNhomChon(n.id)}
                  selected={nhomChon === n.id}
                />
              ))}
            </View>
          )}
        </View>
      ) : null}
      {loi ? <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.warn }]}>{loi}</Text> : null}
      <StampButton disabled={!guiDuoc} label="Đăng" loading={dangGui} onPress={() => void gui()} size="vua" tilt={-1} />
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  trangThu: { borderWidth: 1, borderRadius: 4, padding: 14 },
  phongBiLuoi: { flexDirection: "row", flexWrap: "wrap", gap: 10 },
  phongBi: { flexGrow: 1, flexBasis: 150, minHeight: 104, borderRadius: 4, paddingTop: 22, paddingHorizontal: 12, paddingBottom: 12, overflow: "hidden" },
  // Small enough that its tip ends above the words (it crossed «Chỉ mình tôi» at 44).
  napPhongBi: { position: "absolute", top: -26, alignSelf: "center", width: 30, height: 30, borderWidth: 1, transform: [{ rotate: "45deg" }] },
  phongBiChu: { gap: 2 },
  dauChon: { position: "absolute", top: 6, right: 6 },
  hang: { minHeight: 60, flexDirection: "row", alignItems: "center", gap: 12, paddingVertical: 10, borderBottomWidth: StyleSheet.hairlineWidth },
  hangChu: { flex: 1, gap: 2 },
  khoi: { gap: 8 },
  chips: { flexDirection: "row", flexWrap: "wrap", gap: 8 },
  bam: { opacity: 0.7 },
  khungAnh: { gap: 10 },
  anhXem: { width: "100%", aspectRatio: 4 / 3 },
});
