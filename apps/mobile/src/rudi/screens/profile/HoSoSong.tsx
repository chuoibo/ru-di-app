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
 * The mounted form owns its draft and field-local errors. The parent places
 * saved content and tools after this identity through a read-only content slot.
 */
import { useFocusEffect, useRouter } from "expo-router";
import { useCallback, useLayoutEffect, useRef, useState, type ReactNode } from "react";
import { Keyboard, Pressable, StyleSheet, View, useWindowDimensions } from "react-native";

import { ApiError, thongDiepNguoiDoc } from "../../../api";
import { docHoSoToi, doiTenTrongPhien, suaHoSoToi, type HoSoToi, type Phien } from "../../../phien";
import { KHUNG_DAU_CHAN, hinhDauChan } from "../../art/dau-chan";
import { useRudiSession } from "../../session";
import { typography, useRudiTheme } from "../../theme";
import { Field } from "../../ui";
import { chuLon } from "../../adaptive";
import { Text, useNativeFontMetricsOwner, useNativeStandaloneTextMetrics } from "../../ui/NativeFontMetrics";
import { RudiIcon } from "../../ui/RudiIcon";
import { PressScale } from "../../ui/PressScale";
import { AvatarNguoi } from "../../ui/AvatarNguoi";
import { ErrorState } from "../../ui/ErrorState";
import { SkeletonGroup, SkeletonRow } from "../../ui/Skeleton";
import { useLuiLop } from "../../ui/useLuiLop";
import { VeLop } from "../../ui/art/VeLop";

type Trang =
  | { pha: "dang-doc" }
  | { pha: "xong"; hoSo: HoSoToi }
  | { pha: "hong"; loi: string };

function loiRaChu(error: unknown): string {
  return error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null);
}

export type ProfileEditorControls = { busy: boolean; save: () => void; back: () => void; cancel: () => void };

export function HoSoSong({ phien, onEditorChange, renderContent }: { phien: Phien; onEditorChange: (controls: ProfileEditorControls | null) => void; renderContent?: (profile: HoSoToi | null) => ReactNode }) {
  const { datPhien } = useRudiSession();
  const { colors, radius } = useRudiTheme();
  const { fontScale } = useWindowDimensions();
  const metricsOwned = useNativeFontMetricsOwner();
  const bodyMetrics = useNativeStandaloneTextMetrics(typography.body);
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
    return <>
      <SkeletonGroup>
        <SkeletonRow leading={64} />
      </SkeletonGroup>
      {renderContent?.(null)}
    </>;
  }
  if (trang.pha === "hong") {
    return <><ErrorState body={trang.loi} onRetry={() => void nap()} title="Chưa đọc được hồ sơ" />{renderContent?.(null)}</>;
  }
  const { hoSo } = trang;

  if (dangSua) {
    return (
      <View style={styles.form}>
        <View style={[styles.editIdentity, { backgroundColor: colors.card, borderColor: colors.lineStrong, borderRadius: radius.base }]}>
          <Pressable accessibilityRole="button" accessibilityLabel="Đổi ảnh đại diện" onPress={() => router.push("/settings" as never)} style={styles.editAvatar}>
            <AvatarNguoi name={hoSo.display_name} personId={phien.person_id} size={64} ring />
            <View style={[styles.editCamera, { backgroundColor: colors.card, borderColor: colors.line }]}><RudiIcon name="camera-outline" color={colors.ink} size={16} /></View>
          </Pressable>
          <View style={styles.editIdentityText}>
            <Text style={[typography.title, { color: colors.ink }]}>Một chút về bạn</Text>
            <Text style={[typography.note, { color: colors.inkSoft }]}>Tên, vài lời giới thiệu và nơi bạn hay ở.</Text>
          </View>
        </View>
        <Field nativeMetrics accessibilityLabel="Ô tên hiển thị" label="Tên hiển thị" icon="person-outline" maxLength={200} editable={!dangLuu} onChangeText={setTen} value={ten} error={loiLuu} />
        <View style={styles.editBio}>
        <Field
          nativeMetrics
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
          nativeMetrics
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

  const largeText = bodyMetrics !== null
    ? (bodyMetrics.fontSize ?? typography.body.fontSize) >= typography.body.fontSize * 1.25
    : metricsOwned || chuLon(fontScale);
  const editLabel = conNhap ? "Sửa tiếp hồ sơ" : "Chỉnh hồ sơ";
  return <View style={styles.passport}>
    <View
      testID="profile-identity"
      style={[
        styles.identityBlock,
        {
          backgroundColor: colors.card,
          borderColor: colors.lineStrong,
          borderRadius: radius.base,
        },
      ]}
    >
      <View
        accessibilityElementsHidden
        importantForAccessibility="no-hide-descendants"
        pointerEvents="none"
        style={[styles.passportTopBand, { backgroundColor: colors.accent }]}
      />
      <View
        accessibilityElementsHidden
        importantForAccessibility="no-hide-descendants"
        pointerEvents="none"
        style={[styles.passportInnerRule, { borderColor: colors.line, borderRadius: Math.max(4, radius.base - 4) }]}
      />
      <View
        accessibilityElementsHidden
        importantForAccessibility="no-hide-descendants"
        pointerEvents="none"
        style={styles.passportSealMark}
      >
        <VeLop height={36} khungH={KHUNG_DAU_CHAN} khungW={KHUNG_DAU_CHAN} lop={hinhDauChan("passport-seal")} width={36} />
      </View>
      <View style={styles.identityRow}>
        <View style={[styles.identityMain, largeText && styles.identityMainStack]}>
          <Pressable accessibilityRole="button" accessibilityLabel="Đổi ảnh đại diện" onPress={() => router.push("/settings" as never)} style={styles.avatarTarget}>
            <AvatarNguoi name={hoSo.display_name} personId={phien.person_id} size={60} ring />
          </Pressable>
          <View style={[styles.identityCopy, largeText && styles.identityCopyStack]}>
            <Text accessibilityRole="header" style={[typography.h2, { color: colors.ink }]}>{hoSo.display_name}</Text>
            <Text style={[typography.note, { color: colors.inkSoft }]}>{[hoSo.city, `Tham gia ${new Date(hoSo.created_at).getFullYear()}`].filter(Boolean).join(" · ")}</Text>
          </View>
        </View>
        <PressScale
          accessibilityRole="button"
          accessibilityLabel={editLabel}
          accessibilityHint="Sửa tên, giới thiệu và thành phố"
          onPress={() => moSua(hoSo)}
          pressedScale={0.98}
          style={[
            styles.editAction,
            {
              backgroundColor: colors.ground,
              borderColor: colors.line,
              borderRadius: radius.control,
            },
          ]}
        >
          <RudiIcon color={colors.ink} name="create-outline" size={20} />
        </PressScale>
      </View>
      {hoSo.bio ? <Text style={[typography.body, styles.bioCopy, { color: colors.inkSoft, borderTopColor: colors.line }]}>{hoSo.bio}</Text> : null}
      {conNhap ? <Text accessibilityLiveRegion="polite" style={[typography.note, { color: colors.accent }]}>Bản sửa chưa lưu vẫn còn đây.</Text> : null}
    </View>
    {renderContent?.(hoSo)}
  </View>;
}

const styles = StyleSheet.create({
  passport: { gap: 16 },
  identityBlock: { position: "relative", overflow: "hidden", borderWidth: 1, paddingHorizontal: 16, paddingTop: 18, paddingBottom: 16, gap: 12 },
  passportTopBand: { position: "absolute", top: 0, left: 0, right: 0, height: 4 },
  passportInnerRule: { position: "absolute", top: 6, left: 6, right: 6, bottom: 6, borderWidth: StyleSheet.hairlineWidth, borderStyle: "dashed" },
  passportSealMark: { position: "absolute", right: 74, bottom: 12, opacity: 0.26 },
  identityRow: { flexDirection: "row", alignItems: "center", gap: 10 },
  identityMain: { flex: 1, minWidth: 0, flexDirection: "row", alignItems: "center", gap: 14 },
  identityMainStack: { flexDirection: "column", alignItems: "flex-start" },
  identityCopy: { flex: 1, minWidth: 0, gap: 4 },
  identityCopyStack: { flex: 0, alignSelf: "stretch" },
  avatarTarget: { minWidth: 60, minHeight: 60, alignItems: "center", justifyContent: "center" },
  editAction: { minWidth: 48, minHeight: 48, flexShrink: 0, alignItems: "center", justifyContent: "center", borderWidth: 1 },
  bioCopy: { paddingTop: 10, borderTopWidth: StyleSheet.hairlineWidth },
  form: { gap: 20 },
  editIdentity: { flexDirection: "row", alignItems: "center", gap: 16, padding: 14, borderWidth: 1 },
  editIdentityText: { flex: 1, minWidth: 0, gap: 4 },
  editAvatar: { width: 68, height: 76, justifyContent: "center" },
  editCamera: { position: "absolute", right: 0, bottom: 0, width: 28, height: 28, borderRadius: 14, borderWidth: 1, alignItems: "center", justifyContent: "center" },
  editBio: { gap: 4 },
  editCounter: { textAlign: "right", fontVariant: ["tabular-nums"] },
});
