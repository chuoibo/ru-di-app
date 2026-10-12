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
 * The profile reads as a person's page: identity, their own words, then
 * connections and journey records along a small journal route. The form keeps
 * its draft and field-local errors in the same mounted owner.
 */
import { useFocusEffect, useRouter } from "expo-router";
import { useCallback, useLayoutEffect, useRef, useState } from "react";
import { Keyboard, PixelRatio, Pressable, StyleSheet, View, useWindowDimensions } from "react-native";
import Svg, { Path } from "react-native-svg";

import { ApiError, thongDiepNguoiDoc } from "../../../api";
import { docHoSoToi, doiTenTrongPhien, suaHoSoToi, type HoSoToi, type Phien } from "../../../phien";
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

type Trang =
  | { pha: "dang-doc" }
  | { pha: "xong"; hoSo: HoSoToi }
  | { pha: "hong"; loi: string };

const EDIT_ACTION = { paddingHorizontal: 14, paddingVertical: 10, border: 1, icon: 18, gap: 9 };

function loiRaChu(error: unknown): string {
  return error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null);
}

export type ProfileEditorControls = { busy: boolean; save: () => void; back: () => void; cancel: () => void };

export function HoSoSong({ phien, onEditorChange }: { phien: Phien; onEditorChange: (controls: ProfileEditorControls | null) => void }) {
  const { datPhien } = useRudiSession();
  const { colors, radius } = useRudiTheme();
  const { fontScale } = useWindowDimensions();
  const metricsOwned = useNativeFontMetricsOwner();
  const bodyMetrics = useNativeStandaloneTextMetrics(typography.body);
  const editLabelMetrics = useNativeStandaloneTextMetrics(typography.label);
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
  const [editColumnWidth, setEditColumnWidth] = useState(0);
  const [editMeasurement, setEditMeasurement] = useState<{ key: string; width: number | null } | null>(null);
  const editMeasurementKeyRef = useRef("");
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

  const facts = [
    { id: "friends", value: hoSo.counts.friends, label: "bạn bè", icon: "person-add-outline" as const },
    { id: "contexts", value: hoSo.counts.contexts, label: "nhóm", icon: "people-outline" as const },
    { id: "outings", value: hoSo.counts.outings, label: "kèo", icon: "calendar-outline" as const },
    { id: "places", value: hoSo.counts.places_checked_in, label: "điểm check-in", icon: "location-outline" as const },
    { id: "memories", value: hoSo.counts.memories, label: "khoảnh khắc", icon: "image-outline" as const },
  ];
  // An unusable owned snapshot keeps a conservative stack without a child observer.
  const largeText = bodyMetrics !== null
    ? (bodyMetrics.fontSize ?? typography.body.fontSize) >= typography.body.fontSize * 1.25
    : metricsOwned || chuLon(fontScale);
  // Longer integer counts use the reading flow rather than overflowing a small milestone.
  const storyStack = largeText || facts.some((fact) => String(fact.value).length > 4);
  const editLabel = conNhap ? "Sửa tiếp hồ sơ" : "Chỉnh hồ sơ";
  const editMetricsKey = editLabelMetrics === null
    ? JSON.stringify([metricsOwned ? "owned-fallback" : "unowned", fontScale])
    : JSON.stringify(["manual", editLabelMetrics.fontSize, editLabelMetrics.lineHeight, editLabelMetrics.letterSpacing]);
  const editMeasurementKey = `${editLabel}:${editMetricsKey}:${PixelRatio.get()}:${editColumnWidth}`;
  editMeasurementKeyRef.current = editMeasurementKey;
  // Android can round an auto-sized Text down below its fractional glyph width.
  // Measure in the finite column, then request an exact width rounded up to a pixel.
  const editLabelAvailable = Math.max(0, editColumnWidth - 2 * (EDIT_ACTION.paddingHorizontal + EDIT_ACTION.border) - EDIT_ACTION.icon - EDIT_ACTION.gap);
  const editLabelWidth = editMeasurement?.key === editMeasurementKey && editColumnWidth > 0
    ? Math.min(editMeasurement.width ?? editLabelAvailable, editLabelAvailable)
    : undefined;
  return <View onLayout={(event) => setEditColumnWidth(event.nativeEvent.layout.width)} style={styles.passport}>
    <Text key={editMeasurementKey} accessible={false} importantForAccessibility="no-hide-descendants" pointerEvents="none" onTextLayout={(event) => {
      if (editMeasurementKeyRef.current !== editMeasurementKey) return;
      const lines = event.nativeEvent.lines;
      const width = lines.length === 1 ? Math.ceil(lines[0].width * PixelRatio.get()) / PixelRatio.get() : null;
      setEditMeasurement((old) => old?.key === editMeasurementKey && old.width === width ? old : { key: editMeasurementKey, width });
    }} style={[typography.label, styles.editMeasure]}>{editLabel}</Text>
    <View style={styles.identityBlock}>
    <View style={[styles.identityRow, largeText && styles.identityStack]}>
      <Pressable accessibilityRole="button" accessibilityLabel="Đổi ảnh đại diện" onPress={() => router.push("/settings" as never)} style={styles.avatarTarget}>
        <AvatarNguoi name={hoSo.display_name} personId={phien.person_id} size={56} ring />
      </Pressable>
      <View style={[styles.identityCopy, largeText && styles.identityCopyStack]}>
        <Text style={[typography.h2, { color: colors.ink }]}>{hoSo.display_name}</Text>
        <Text style={[typography.note, { color: colors.inkSoft }]}>{[hoSo.city, `Tham gia ${new Date(hoSo.created_at).getFullYear()}`].filter(Boolean).join(" · ")}</Text>
      </View>
    </View>
    {hoSo.bio ? <Text style={[typography.body, { color: colors.inkSoft }]}>{hoSo.bio}</Text> : null}
    <PressScale accessibilityRole="button" accessibilityLabel={editLabel} onPress={() => moSua(hoSo)} pressedScale={0.98} style={[styles.editAction, { borderRadius: radius.control }]}>
      <RudiIcon color={colors.accent} name="create-outline" size={EDIT_ACTION.icon} style={styles.editIcon} />
      <Text style={[typography.label, styles.editLabel, largeText && styles.editLabelLarge, editLabelWidth !== undefined && { width: editLabelWidth }, { color: colors.inkSoft }]}>{editLabel}</Text>
    </PressScale>
    {conNhap ? <Text accessibilityLiveRegion="polite" style={[typography.note, { color: colors.inkSoft }]}>Bản sửa chưa lưu vẫn còn đây.</Text> : null}
    </View>
    <View testID="profile-counts" style={[styles.story, storyStack && styles.storyStack]}>
      {/* This line relates the five facts; it does not encode progress or completed outings. */}
      {!storyStack ? <Svg pointerEvents="none" accessible={false} focusable={false} aria-hidden width="100%" height={24} viewBox="0 0 100 24" preserveAspectRatio="none" style={styles.storyRoute}>
        <Path d="M10 12C18 12 22 5 30 5S42 20 50 20S62 8 70 8S82 16 90 16" fill="none" stroke={colors.lineStrong} strokeOpacity={0.45} strokeWidth={0.5} strokeDasharray="1.4 1.4" />
      </Svg> : <View pointerEvents="none" accessible={false} style={[styles.storySpine, { borderLeftColor: colors.line }]} />}
      {facts.map((fact, index) => <View accessible accessibilityLabel={`${fact.value} ${fact.label}`} key={fact.id} testID={`profile-fact-${fact.id}`} style={[styles.storyFact, storyStack && styles.storyFactRow]}>
        <View accessible={false} importantForAccessibility="no-hide-descendants" style={[styles.storyIcon, { backgroundColor: colors.ground }]}>
          <RudiIcon name={fact.icon} color={index === 2 ? colors.accent : colors.inkSoft} size={20} />
        </View>
        <View style={[styles.storyWords, storyStack && styles.storyWordsRow]}>
          <Text accessible={false} testID={`profile-count-${fact.id}`} style={[typography.title, styles.storyValue, !storyStack && styles.storyCentered, { color: colors.ink }]}>{fact.value}</Text>
          <Text accessible={false} testID={`profile-label-${fact.id}`} style={[typography.note, styles.storyLabel, !storyStack && styles.storyCentered, { color: colors.inkSoft }]}>{fact.label}</Text>
        </View>
      </View>)}
    </View>
  </View>;
}

const styles = StyleSheet.create({
  passport: { gap: 12 },
  identityBlock: { gap: 8 },
  story: { width: "100%", minWidth: 0, flexDirection: "row", paddingVertical: 4 },
  storyStack: { flexDirection: "column", gap: 8 },
  storyRoute: { position: "absolute", left: 0, top: 4 },
  storySpine: { position: "absolute", left: 12, top: 28, bottom: 28, borderLeftWidth: 1 },
  storyFact: { width: "20%", minWidth: 0, flexDirection: "column", alignItems: "center", gap: 4 },
  storyFactRow: { width: "100%", flexDirection: "row", alignItems: "center", gap: 12, minHeight: 48 },
  storyIcon: { width: 24, height: 24, alignItems: "center", justifyContent: "center", borderRadius: 12, flexShrink: 0 },
  storyWords: { width: "100%", minWidth: 0, flexGrow: 0, flexShrink: 1, flexBasis: "auto", flexDirection: "column", alignItems: "stretch", flexWrap: "nowrap", gap: 2 },
  storyWordsRow: { width: "auto", flexGrow: 1, flexDirection: "row", alignItems: "baseline", flexWrap: "wrap", columnGap: 8, rowGap: 2 },
  storyValue: { fontVariant: ["tabular-nums"], maxWidth: "100%", minWidth: 0, flexShrink: 1 },
  storyLabel: { minWidth: 0, flexShrink: 1 },
  storyCentered: { textAlign: "center" },
  identityRow: { flexDirection: "row", alignItems: "center", gap: 12 },
  identityStack: { flexDirection: "column", alignItems: "flex-start" },
  identityCopy: { flex: 1, minWidth: 0, gap: 4 },
  identityCopyStack: { flex: 0, alignSelf: "stretch" },
  avatarTarget: { minWidth: 56, minHeight: 56, alignItems: "center", justifyContent: "center" },
  editAction: { alignSelf: "flex-start", minWidth: 48, minHeight: 48, maxWidth: "100%", flexDirection: "row", alignItems: "center", gap: EDIT_ACTION.gap, borderWidth: EDIT_ACTION.border, borderColor: "transparent", backgroundColor: "transparent", paddingHorizontal: EDIT_ACTION.paddingHorizontal, paddingVertical: EDIT_ACTION.paddingVertical },
  editIcon: { flexShrink: 0 },
  editLabel: { flexShrink: 0 },
  editLabelLarge: { flexShrink: 1, minWidth: 0 },
  editMeasure: { position: "absolute", left: 0, right: 0, opacity: 0 },
  form: { gap: 20 },
  editIdentity: { flexDirection: "row", alignItems: "center", gap: 16, paddingVertical: 4 },
  editIdentityText: { flex: 1, minWidth: 0, gap: 4 },
  editAvatar: { width: 68, height: 76, justifyContent: "center" },
  editCamera: { position: "absolute", right: 0, bottom: 0, width: 28, height: 28, borderRadius: 14, borderWidth: 1, alignItems: "center", justifyContent: "center" },
  editBio: { gap: 4 },
  editCounter: { textAlign: "right", fontVariant: ["tabular-nums"] },
});
