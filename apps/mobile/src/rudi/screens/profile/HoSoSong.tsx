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
import { useCallback, useRef, useState } from "react";
import { Pressable, StyleSheet, Text, View, useWindowDimensions } from "react-native";

import { ApiError, thongDiepNguoiDoc } from "../../../api";
import { docHoSoToi, doiTenTrongPhien, suaHoSoToi, type HoSoToi, type Phien } from "../../../phien";
import { useRudiSession } from "../../session";
import { nhanCua } from "../../cai-dat/phien-cai-dat";
import { bongGiay, typography, useRudiTheme } from "../../theme";
import { Field, IconButton, RudiButton, TopBar } from "../../ui";
import { chuLon } from "../../adaptive";
import { DauLon } from "../../ui/DauLon";
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

export function HoSoSong({ phien, onEditingChange }: { phien: Phien; onEditingChange?: (editing: boolean) => void }) {
  const { datPhien } = useRudiSession();
  const { colors, dark } = useRudiTheme();
  const { width, fontScale } = useWindowDimensions();
  const [contentWidth, setContentWidth] = useState(Math.min(width - 32, 640));
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
    onEditingChange?.(editing);
  };
  useLuiLop(dangSua, () => {
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
    const giuBanSua = () => { if (!saving.current) { setConNhap(true); doiSua(false); } };
    const huyBanSua = () => { setConNhap(false); doiSua(false); };
    return (
      <View style={styles.form}>
        <TopBar title="Chỉnh hồ sơ" onBack={giuBanSua} right={<IconButton accessibilityLabel="Huỷ" icon="close" disabled={dangLuu} onPress={huyBanSua} quiet />} />
        <View style={styles.editIdentity}>
          <Pressable accessibilityRole="button" accessibilityLabel="Đổi ảnh đại diện" onPress={() => router.push("/settings" as never)} style={styles.editAvatar}>
            <AvatarNguoi name={hoSo.display_name} personId={phien.person_id} size={64} ring />
            <View style={[styles.editCamera, { backgroundColor: colors.card, borderColor: colors.line }]}><RudiIcon name="camera-outline" color={colors.ink} size={16} /></View>
          </Pressable>
          <View style={styles.editIdentityText}>
            <Text style={[typography.h2, { color: colors.ink }]}>Một chút về bạn</Text>
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
        <RudiButton disabled={dangLuu} label="Lưu hồ sơ" loading={dangLuu} onPress={() => void luu()} />
      </View>
    );
  }

  const soDem = [
    `${hoSo.counts.friends} bạn bè`,
    `${hoSo.counts.contexts} nhóm`,
    `${hoSo.counts.outings} kèo`,
    `${hoSo.counts.places_checked_in} nơi đã tới`,
    // The moments posted to a group's wall, named as «Thả khoảnh khắc» names
    // them: «6 kỷ niệm» sat over «Chưa giữ ngày nào» (the kept days, a
    // different thing) and read as lost data (B11 critique).
    `${hoSo.counts.memories} khoảnh khắc`,
  ].join(" · ");

  const namVao = new Date(hoSo.created_at).getFullYear();
  const largeText = chuLon(fontScale);
  const coverWidth = Math.min(contentWidth, 248);
  const coverHeight = coverWidth * 4 / 3;
  // A short identity can sit on the cover's paper label. Longer names and
  // larger type get the full data page, where every line can grow freely.
  const identityOnCover = !largeText && coverWidth >= 220 && hoSo.display_name.length <= 28;
  const identity = (
    <View style={identityOnCover ? styles.nhanTen : [styles.dau, largeText && styles.dauLonChu]}>
      {/* The picture is changed where it is picked, compressed and uploaded
          (Cài đặt); tapping it here is the way there, not a second uploader. */}
      <Pressable
        accessibilityLabel="Đổi ảnh đại diện"
        accessibilityRole="button"
        onPress={() => router.push("/settings" as never)}
        style={({ pressed }) => [styles.anhHoChieu, { borderColor: colors.lineStrong }, pressed && styles.bam]}
      >
        <AvatarNguoi name={hoSo.display_name} personId={phien.person_id} ring size={identityOnCover ? 44 : 64} />
      </Pressable>
      <View style={identityOnCover ? styles.tenTrenBia : [styles.dauChu, largeText && styles.dauChuLon]}>
        {identityOnCover ? null : <Text style={[typography.caption, { color: colors.inkSoft }]}>Họ tên</Text>}
        <Text style={[typography.h1, identityOnCover && styles.tenBia, { color: colors.ink }]}>{hoSo.display_name}</Text>
      </View>
    </View>
  );
  // The bitmap stays in its portrait ratio. Identity and all facts remain
  // live text; edit controls sit on the reading page, outside the artwork.
  return (
    <View
      onLayout={({ nativeEvent: { layout } }) => { if (layout.width > 0) setContentWidth(layout.width); }}
      style={styles.hoChieu}
    >
      <View style={[styles.bia, { width: coverWidth }]}>
        <AssetIllustration height={coverHeight} name="decor-cover" width={coverWidth} style={StyleSheet.absoluteFill} />
        <View style={[styles.noiDungBia, { minHeight: coverHeight, paddingHorizontal: coverWidth * 0.14, paddingTop: coverHeight * 0.12 }]}>
          <View pointerEvents="none" style={styles.tuaBia}>
            <Text style={[typography.stamp, styles.chuBia, { color: colors.coverInk }]}>Hộ chiếu Rủ Đi</Text>
            <RudiIcon color={colors.coverInkSoft} name="compass-outline" size={28} />
          </View>
          {identityOnCover ? (
            <View style={[styles.giayTen, { width: coverWidth * 0.64, marginTop: coverHeight * 0.12, backgroundColor: colors.card }, bongGiay(1, dark)]}>
              {identity}
            </View>
          ) : null}
        </View>
      </View>
      <View style={styles.trang}>
        {identityOnCover ? null : identity}
        {hoSo.city ? (
          <View style={styles.thanhPho}>
            <Text style={[typography.caption, { color: colors.inkSoft }]}>Thành phố</Text>
            <Text style={[typography.label, { color: colors.ink }]}>{hoSo.city}</Text>
          </View>
        ) : null}
        {hoSo.bio ? <Text style={[typography.body, { color: colors.inkSoft }]}>{hoSo.bio}</Text> : null}
        <View style={[styles.hangDau, largeText && styles.hangDauLonChu]}>
          <DauLon co="nho" nhan={`Tham gia ${namVao}`} tilt={-6} tone="ink" />
          <Text style={[typography.caption, styles.phien, largeText && styles.phienLonChu, { color: colors.inkSoft }]}>
            Phiên này: {nhanCua(phien.issued_via ?? "")}
          </Text>
        </View>
        <Text style={[typography.caption, { color: colors.inkSoft }]}>{soDem}</Text>
        <RudiButton compact full={false} icon="create-outline" label={conNhap ? "Sửa tiếp hồ sơ" : "Chỉnh hồ sơ"} onPress={() => moSua(hoSo)} variant="outline" />
        {conNhap ? <Text accessibilityLiveRegion="polite" style={[typography.caption, { color: colors.inkSoft }]}>Bản sửa chưa lưu vẫn còn đây.</Text> : null}
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  hoChieu: { width: "100%", gap: 16 },
  bia: { alignSelf: "center" },
  noiDungBia: { alignItems: "flex-start" },
  tuaBia: { width: "100%", alignItems: "center", gap: 8 },
  chuBia: { fontSize: 14, lineHeight: 19, letterSpacing: 1.1, textAlign: "center" },
  giayTen: { borderRadius: 2, padding: 12, transform: [{ rotate: "-2deg" }] },
  nhanTen: { alignItems: "flex-start", gap: 6 },
  tenTrenBia: { alignSelf: "stretch", gap: 4 },
  tenBia: { fontSize: 18, lineHeight: 22, letterSpacing: -0.3 },
  trang: { gap: 14, paddingHorizontal: 4 },
  anhHoChieu: { borderWidth: 1, padding: 4, borderRadius: 4 },
  hangDau: { flexDirection: "row", alignItems: "center", gap: 16, flexWrap: "wrap", paddingVertical: 4 },
  hangDauLonChu: { flexDirection: "column", alignItems: "flex-start" },
  phien: { flexGrow: 1, flexShrink: 1, flexBasis: 180, minWidth: 180 },
  phienLonChu: { flexBasis: "auto", alignSelf: "stretch" },
  form: { gap: 20 },
  editIdentity: { flexDirection: "row", alignItems: "center", gap: 16, paddingVertical: 4 },
  editIdentityText: { flex: 1, minWidth: 0, gap: 4 },
  editAvatar: { width: 68, height: 76, justifyContent: "center" },
  editCamera: { position: "absolute", right: 0, bottom: 0, width: 28, height: 28, borderRadius: 14, borderWidth: 1, alignItems: "center", justifyContent: "center" },
  editBio: { gap: 4 },
  editCounter: { textAlign: "right", fontVariant: ["tabular-nums"] },
  dau: { flexDirection: "row", flexWrap: "wrap", alignItems: "center", gap: 16 },
  dauLonChu: { flexDirection: "column", alignItems: "flex-start" },
  dauChu: { flexGrow: 1, flexShrink: 1, flexBasis: 180, minWidth: 180, gap: 4 },
  dauChuLon: { flexGrow: 0, flexBasis: "auto", minWidth: 0, alignSelf: "stretch" },
  thanhPho: { gap: 4 },
  bam: { opacity: 0.7 },
});
