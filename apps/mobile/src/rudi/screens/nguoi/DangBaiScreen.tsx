/**
 * Đăng bài (M8): write one post, to one of F42's four audiences.
 *
 * The four audiences are a vocabulary, not a ladder (`bai-dang.ts`): `friends`
 * and `group` reach two disjoint sets and neither contains the other. They are
 * drawn as four rows, each carrying the sentence that names who it reaches --
 * not a slider, not a narrow-to-wide chip row, not a lock that opens in steps.
 *
 * A personal photo is uploaded before its post is written; the server decides
 * who can read both through the post audience.
 */
import { Image } from "expo-image";
import { useRouter } from "expo-router";
import { useEffect, useRef, useState } from "react";
import { Ionicons } from "@expo/vector-icons";
import { Pressable, StyleSheet, Text, View } from "react-native";

import { attemptFor, type Attempt } from "../../../api";
import { boAnh, chonAnh, nenVaDung, type GiaiDoanTaiAnh, type TempPhoto } from "../../ky-niem/chon-anh";
import { nguonAnhBai, taiAnhCaNhan } from "../../nguoi/anh-ca-nhan";
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
import { typography, useRudiTheme } from "../../theme";
import { Chip, Field, Heading, RudiButton, RudiScreen, TopBar } from "../../ui";

export function DangBaiScreen() {
  const router = useRouter();
  const { colors, radius } = useRudiTheme();
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
  const [anhDaTai, setAnhDaTai] = useState<string | null>(null);
  const attempts = useRef<Record<string, Attempt>>({});
  const [giaiDoan, setGiaiDoan] = useState<GiaiDoanTaiAnh | null>(null);

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
          // `nenVaDung` deletes the picked file even when upload fails.
          setAnh(null);
        }
      }
      const key = JSON.stringify({ body: form.body.trim(), audience: form.audience, contextId: form.contextId, imageUrl });
      await guiBai(phien.person_id, { ...form, imageUrl }, attemptFor(attempts.current, key));
      router.replace(`/people/${phien.person_id}`);
    } catch (error) {
      const repickHint = anh !== null && !uploadFinished ? " Chọn lại ảnh rồi thử lần nữa." : "";
      setLoi(loiRaChu(error) + repickHint);
    } finally {
      setGiaiDoan(null);
      setDangGui(false);
    }
  };

  const cauGiaiDoan = giaiDoan === "chuan-bi-anh" ? "Đang chuẩn bị ảnh…" : giaiDoan === "dang-gui" ? "Đang tải ảnh lên…" : null;

  return (
    <RudiScreen testID="dang-bai-screen">
      <TopBar title="Đăng bài" />
      <Field
        label="Bạn muốn kể gì?"
        multiline
        numberOfLines={5}
        onChangeText={setThan}
        placeholder="Chuyến vừa rồi, quán mới, hay chỉ một câu."
        value={than}
      />
      {/* ADR-0022 §2.1: one photograph of one's own, optional; it goes up first
          as a personal picture and the post that shows it comes second. */}
      <View style={styles.khungAnh}>
        {anh !== null ? (
          <Image accessibilityLabel="Ảnh đã chọn" contentFit="cover" source={{ uri: anh.uri }} style={[styles.anhXem, { borderRadius: radius.small }]} />
        ) : anhDaTai !== null && phien !== null ? (
          <Image accessibilityLabel="Ảnh đã tải lên, đang chờ đăng bài" contentFit="cover" source={nguonAnhBai(anhDaTai, phien.person_id) ?? undefined} style={[styles.anhXem, { borderRadius: radius.small }]} />
        ) : (
          <Text style={[typography.caption, { color: colors.inkFaint }]}>Một tấm ảnh, nếu muốn. Ai đọc được bài thì xem được ảnh.</Text>
        )}
        <View style={styles.chips}>
          <RudiButton compact disabled={dangGui} full={false} icon="images-outline" label={anh === null && anhDaTai === null ? "Chọn ảnh" : "Chọn ảnh khác"} onPress={() => void chonAnhMoi()} variant="outline" />
          {anh !== null || anhDaTai !== null ? <RudiButton compact disabled={dangGui} full={false} label="Bỏ ảnh" onPress={() => void boAnhDaChon()} variant="ghost" /> : null}
        </View>
        {cauGiaiDoan ? <Text style={[typography.caption, { color: colors.inkSoft }]}>{cauGiaiDoan}</Text> : null}
      </View>
      <Heading subtitle="Chọn ai đọc được bài này. Bốn mức không xếp từ hẹp tới rộng: bạn bè và nhóm là hai tập khác nhau." title="Ai đọc được?" />
      <View>
        {AUDIENCES.map((a) => {
          const chon = muc === a;
          return (
            <Pressable
              // Named so a driver (and a screen reader) can pick this row and
              // not the sentence under a neighbour, which mentions «Bạn bè» too.
              accessibilityLabel={`Mức người đọc: ${MUC_NGUOI_DOC[a].nhan}`}
              accessibilityRole="radio"
              accessibilityState={{ selected: chon }}
              key={a}
              onPress={() => setMuc(a)}
              style={({ pressed }) => [
                styles.hang,
                { borderBottomColor: colors.line },
                pressed && styles.bam,
              ]}
            >
              <Ionicons color={chon ? colors.accent : colors.lineStrong} name={chon ? "checkmark-circle" : "ellipse-outline"} size={22} />
              <View style={styles.hangChu}>
                <Text style={[typography.label, { color: colors.ink }]}>
                  {MUC_NGUOI_DOC[a].nhan}
                </Text>
                <Text style={[typography.caption, { color: colors.inkFaint }]}>
                  {MUC_NGUOI_DOC[a].giaiThich}
                </Text>
              </View>
            </Pressable>
          );
        })}
      </View>
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
      <RudiButton
        disabled={!guiDuoc}
        icon="send-outline"
        label="Đăng"
        loading={dangGui}
        onPress={() => void gui()}
      />
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  hang: { minHeight: 60, flexDirection: "row", alignItems: "center", gap: 12, paddingVertical: 10, borderBottomWidth: StyleSheet.hairlineWidth },
  hangChu: { flex: 1, gap: 2 },
  khoi: { gap: 8 },
  chips: { flexDirection: "row", flexWrap: "wrap", gap: 8 },
  bam: { opacity: 0.7 },
  khungAnh: { gap: 10 },
  anhXem: { width: "100%", aspectRatio: 4 / 3 },
});
