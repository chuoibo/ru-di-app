/**
 * The profile card for a real session (M2): what `GET /people/me` says.
 *
 * Name, bio and city are the person's own words; the five numbers are the
 * server's counts, each from the table that owns it (friends, active groups,
 * outings of those groups, distinct stops checked in at, memories authored).
 * Nothing on this card is derived on the phone, and nothing comes from the
 * fixture -- which is the whole reason it exists: the fixture hero showed
 * «Minh Anh · Cấp 12» to whoever signed in.
 *
 * Editing goes through `PATCH /people/me` and the card re-reads the server's
 * answer rather than trusting the form.
 *
 * UI v2 (đợt 7): a footprint, not a trophy page -- initial, name, one line,
 * and the five counts as one sentence on the paper. The form sits on the
 * page too, with the error next to the field it is about.
 */
import { useFocusEffect, useRouter } from "expo-router";
import { useCallback, useLayoutEffect, useRef, useState } from "react";
import { Keyboard, Pressable, StyleSheet, Text, View, useWindowDimensions } from "react-native";

import { ApiError, thongDiepNguoiDoc } from "../../../api";
import { docHoSoToi, doiTenTrongPhien, suaHoSoToi, type HoSoToi, type Phien } from "../../../phien";
import { useRudiSession } from "../../session";
import { typography, useRudiTheme } from "../../theme";
import { Field, RudiButton } from "../../ui";
import { chuLon } from "../../adaptive";
import { AssetIllustration } from "../../asset-pack/react-native/AssetIllustration";
import { RudiIcon } from "../../ui/RudiIcon";
import { AvatarNguoi } from "../../ui/AvatarNguoi";
import { ErrorState } from "../../ui/ErrorState";
import { SkeletonGroup, SkeletonRow } from "../../ui/Skeleton";
import { useLuiLop } from "../../ui/useLuiLop";

type Trang =
  | { pha: "dang-doc" }
  | { pha: "xong"; hoSo: HoSoToi }
  | { pha: "hong"; loi: string };

function loiRaChu(error: unknown): string {
  return error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null);
}

export type ProfileEditorControls = { busy: boolean; save: () => void; back: () => void; cancel: () => void };

export function HoSoSong({ phien, onEditorChange }: { phien: Phien; onEditorChange: (controls: ProfileEditorControls | null) => void }) {
  const { datPhien } = useRudiSession();
  const { colors } = useRudiTheme();
  const { fontScale } = useWindowDimensions();
  const router = useRouter();
  const [trang, setTrang] = useState<Trang>({ pha: "dang-doc" });
  const [dangSua, setDangSua] = useState(false);
  const [ten, setTen] = useState("");
  const [bio, setBio] = useState("");
  const [city, setCity] = useState("");
  const saving = useRef(false);
  const [dangLuu, setDangLuu] = useState(false);
  const [loiLuu, setLoiLuu] = useState<string | null>(null);
  // QA UI-108: Back closes the form and stays on Cá nhân; what was typed is
  // kept and comes back when the form opens again. «Huỷ» is what drops it.
  const [conNhap, setConNhap] = useState(false);
  // The passport and its form are different pages in the same scroll view.
  const doiSua = (editing: boolean) => {
    setDangSua(editing);
    if (!editing) Keyboard.dismiss();
  };
  useLuiLop(dangSua, () => {
    if (saving.current) return;
    setConNhap(true);
    doiSua(false);
  });

  const nap = useCallback(async () => {
    try {
      setTrang({ pha: "xong", hoSo: await docHoSoToi(phien.person_id) });
    } catch (error) {
      setTrang({ pha: "hong", loi: loiRaChu(error) });
    }
  }, [phien.person_id]);

  useFocusEffect(
    useCallback(() => {
      void nap();
    }, [nap]),
  );

  const moSua = (hoSo: HoSoToi) => {
    if (!conNhap) {
      setTen(hoSo.display_name);
      setBio(hoSo.bio ?? "");
      setCity(hoSo.city ?? "");
    }
    setLoiLuu(null);
    doiSua(true);
  };

  const luu = async () => {
    if (saving.current) return;
    if (ten.trim() === "") {
      setLoiLuu("Tên hiển thị không được rỗng.");
      return;
    }
    saving.current = true; setDangLuu(true);
    try {
      const hoSo = await suaHoSoToi(phien.person_id, { display_name: ten.trim(), bio, city });
      setTrang({ pha: "xong", hoSo });
      // Keep the session's greeting in step with the server (QA 23/09).
      datPhien(await doiTenTrongPhien(phien, hoSo.display_name));
      setConNhap(false);
      doiSua(false);
    } catch (error) {
      setLoiLuu(loiRaChu(error));
    } finally {
      saving.current = false; setDangLuu(false);
    }
  };

  // The parent owns the fixed app bar/footer; this mounted form owns its draft.
  // Explicit draft dependencies keep the callbacks current without a render loop.
  useLayoutEffect(() => {
    onEditorChange(dangSua ? {
      busy: dangLuu,
      save: () => void luu(),
      back: () => { if (!saving.current) { setConNhap(true); doiSua(false); } },
      cancel: () => { if (!saving.current) { setConNhap(false); doiSua(false); } },
    } : null);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [onEditorChange, dangSua, dangLuu, ten, bio, city, phien, datPhien]);

  if (trang.pha === "dang-doc") {
    return (
      <SkeletonGroup>
        <SkeletonRow leading={64} />
      </SkeletonGroup>
    );
  }
  if (trang.pha === "hong") {
    return <ErrorState body={trang.loi} onRetry={() => void nap()} title="Chưa đọc được hồ sơ" />;
  }
  const { hoSo } = trang;

  if (dangSua) {
    return (
      <View style={styles.form}>
        <View style={styles.editIdentity}>
          <Pressable accessibilityRole="button" accessibilityLabel="Đổi ảnh đại diện" onPress={() => router.push("/settings" as never)} style={styles.editAvatar}>
            <AvatarNguoi name={hoSo.display_name} personId={phien.person_id} size={64} ring />
            <View style={[styles.editCamera, { backgroundColor: colors.card, borderColor: colors.line }]}><RudiIcon name="camera-outline" color={colors.ink} size={16} /></View>
          </Pressable>
          <View style={styles.editIdentityText}>
            <Text style={[typography.title, { color: colors.ink }]}>Một chút về bạn</Text>
            <Text style={[typography.note, { color: colors.inkSoft }]}>Tên, vài lời giới thiệu và nơi bạn hay ở.</Text>
          </View>
        </View>
        <Field accessibilityLabel="Ô tên hiển thị" label="Tên hiển thị" icon="person-outline" maxLength={200} editable={!dangLuu} onChangeText={setTen} value={ten} error={loiLuu} />
        <View style={styles.editBio}>
        <Field
          accessibilityLabel="Ô giới thiệu"
          label="Giới thiệu"
          maxLength={500}
          multiline
          numberOfLines={3}
          editable={!dangLuu}
          style={{ minHeight: 56 }}
          onChangeText={setBio}
          placeholder="Bạn thích đi đâu, làm gì?"
          value={bio}
        />
        <Text style={[typography.note, styles.editCounter, { color: colors.inkSoft }]}>{bio.length}/500</Text>
        </View>
        <Field
          accessibilityLabel="Ô thành phố"
          label="Thành phố"
          icon="location-outline"
          maxLength={120}
          editable={!dangLuu}
          onChangeText={setCity}
          placeholder="Nơi bạn hay ở"
          value={city}
        />
      </View>
    );
  }

  const soDem = [
    `${hoSo.counts.friends} bạn bè`, `${hoSo.counts.contexts} nhóm`,
    `${hoSo.counts.outings} kèo`, `${hoSo.counts.places_checked_in} nơi đã tới`,
    `${hoSo.counts.memories} khoảnh khắc`,
  ].join(" · ");
  const largeText = chuLon(fontScale);
  return <View style={[styles.passport, { borderBottomColor: colors.line }]}>
    <View style={[styles.identityRow, largeText && styles.identityStack]}>
      <Pressable accessibilityRole="button" accessibilityLabel="Đổi ảnh đại diện" onPress={() => router.push("/settings" as never)} style={styles.avatarTarget}>
        <AvatarNguoi name={hoSo.display_name} personId={phien.person_id} size={56} ring />
      </Pressable>
      <View style={[styles.identityCopy, largeText && styles.identityCopyStack]}>
        <Text style={[typography.h2, { color: colors.ink }]}>{hoSo.display_name}</Text>
        <Text style={[typography.note, { color: colors.inkSoft }]}>{[hoSo.city, `Tham gia ${new Date(hoSo.created_at).getFullYear()}`].filter(Boolean).join(" · ")}</Text>
      </View>
      {!largeText ? <View accessible={false} importantForAccessibility="no-hide-descendants"><AssetIllustration name="decor-cover" width={56} height={56 * 4 / 3} /></View> : null}
    </View>
    {hoSo.bio ? <Text style={[typography.body, { color: colors.inkSoft }]}>{hoSo.bio}</Text> : null}
    <Text style={[typography.note, { color: colors.inkSoft }]}>{soDem}</Text>
    <RudiButton compact full style={[styles.editAction, largeText && styles.editActionLarge]} icon="create-outline" label={conNhap ? "Sửa tiếp hồ sơ" : "Chỉnh hồ sơ"} onPress={() => moSua(hoSo)} variant="outline" />
    {conNhap ? <Text accessibilityLiveRegion="polite" style={[typography.note, { color: colors.inkSoft }]}>Bản sửa chưa lưu vẫn còn đây.</Text> : null}
  </View>;
}

const styles = StyleSheet.create({
  passport: { gap: 16, paddingBottom: 20, borderBottomWidth: StyleSheet.hairlineWidth },
  identityRow: { flexDirection: "row", alignItems: "center", gap: 12 },
  identityStack: { flexDirection: "column", alignItems: "flex-start" },
  identityCopy: { flex: 1, minWidth: 0, gap: 4 },
  identityCopyStack: { flex: 0, alignSelf: "stretch" },
  avatarTarget: { minWidth: 56, minHeight: 56, alignItems: "center", justifyContent: "center" },
  editAction: { alignSelf: "flex-start", width: "auto", minWidth: 48, maxWidth: "100%" },
  editActionLarge: { alignSelf: "stretch", width: "100%" },
  form: { gap: 20 },
  editIdentity: { flexDirection: "row", alignItems: "center", gap: 16, paddingVertical: 4 },
  editIdentityText: { flex: 1, minWidth: 0, gap: 4 },
  editAvatar: { width: 68, height: 76, justifyContent: "center" },
  editCamera: { position: "absolute", right: 0, bottom: 0, width: 28, height: 28, borderRadius: 14, borderWidth: 1, alignItems: "center", justifyContent: "center" },
  editBio: { gap: 4 },
  editCounter: { textAlign: "right", fontVariant: ["tabular-nums"] },
});
