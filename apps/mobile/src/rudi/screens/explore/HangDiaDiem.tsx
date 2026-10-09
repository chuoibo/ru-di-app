import { Ionicons } from "@expo/vector-icons";
import { Image } from "expo-image";
import { useEffect, useState } from "react";
import { Pressable, StyleSheet, Text, View, useWindowDimensions, type TextStyle } from "react-native";

import { AssetIllustration, type IllustrationName } from "../../asset-pack/react-native/AssetIllustration";
import { chuLon } from "../../adaptive";
import { typography, useRudiTheme } from "../../theme";
import { IconButton, Inline, type IconName } from "../../ui";
import { MediaSlot } from "../../ui/MediaSlot";
import { khoaNguon, veKhung, type AnhCoGhiCong } from "../../ui/ghi-cong";
import { Stamp } from "../../ui/Stamp";
import { useAdaptiveLayout } from "../../ui/useAdaptiveLayout";
import { GuGlyph } from "../../ui/art/Gu";
import { chiaDongSuThat, guTheoLoai, moDauBangSao, tachGia } from "../../kham-pha/dia-diem";

/*
 * 2026-09-11 (re-audit 10/09, R3): one mark per place. A place prints EITHER
 * its one reason (`lyDo`, the tag the group matched on, or the model's own
 * sentence) OR the seal (`badge`) -- never both. The lead used to carry the
 * section title's promise («đúng gu»), the seal («HỢP GU»), a reason that
 * repeated it («Hợp gu nhờ …») and a subtitle that repeated the tags: four
 * readings of one promise. The section title keeps the promise; each place
 * adds one fact. And the empty frame is a paper slot -- `paper` on `line` (the card tone by day, a night sheet lighter than the cloth ground by dark — PR C 11/09)
 * with the category drawn in ink and its one coral detail -- not a tinted
 * disc with an all-coral icon, which is every app's default.
 */

/**
 * One place, two sizes, one vocabulary -- for the fixture catalogue and the
 * server's alike.
 *
 * Until this file the fixture drew photo cards with rating pills and the live
 * screen drew icon tiles in cards; the 2026-09-06 review read both as a
 * contact list. Here the first place of a list is the **lead**: a media slot
 * (a licensed photo when the catalogue has one, the category glyph on the
 * paper-toned frame when it does not), then the name at section size. Every
 * other place is a **row**: a small thumbnail, the name, one line of what it
 * is, one line of the facts the server actually holds, and the save heart.
 * Rows sit on the paper with a hairline between them; there is no card.
 *
 * A screen adapts its own data into `DiaDiemHienThi`; nothing here fetches
 * or invents. A badge is printed only when the caller says the match is real.
 *
 * 2026-09-08 (report 07/09 §9.4): between the lead and the rows there is now a
 * **pair** to compare, two places side by side on the same axis (picture,
 * name, one line, the facts), so choosing is a comparison before it is a
 * scroll. And the empty frame's artwork is the category drawn with the art
 * layer's pen (`GuGlyph`) rather than a system icon blown up in a disc.
 */
export interface DiaDiemHienThi {
  id: string;
  name: string;
  /** Kinds and travel time, or the fixture's one-line description. */
  sub: string;
  /** Only the facts this place has; an empty list is an honest list. */
  facts: { icon: IconName; text: string }[];
  /** Category glyph, the fallback artwork when there is no photo. */
  glyph: IconName;
  /** Catalogue category id; with it the fallback is the drawn object, without it the icon above. */
  loai?: string;
  /**
   * The picture together with its credit, or null. One field, not two: a
   * screen cannot hand these frames the address without the words the
   * picture is allowed to be shown under (ADR-0017 §2.5; review 08/09 vòng 2).
   */
  anh: AnhCoGhiCong | null;
  /** «Rất hợp gu» from a computed match; null otherwise. Printed only when there is no `lyDo`. */
  badge: string | null;
  /**
   * One grounded reason: a single tag the group really matched on (fixture,
   * via `chonLyDo`) or the model's own sentence (live). Present → this is the
   * place's one mark and the seal is not printed.
   */
  lyDo?: string;
  /** The drawn object a real tag chose (`guTheoTag`); absent → the category's object. */
  gu?: string;
  /** Tags the place really carries; the lead's sketch picks at most two props from them. Live rows pass none. */
  tags?: readonly string[];
}

interface CommonProps {
  dd: DiaDiemHienThi;
  daLuu: boolean;
  onOpen: () => void;
  onSave: () => void;
  testID?: string;
  saving?: boolean;
  /** A short viewport puts the name beside the artwork before the fold. */
  compact?: boolean;
}

/**
 * The empty frame's artwork: a paper slot (`card`, hairline `line`, the small
 * radius the photo frames use) with the category drawn in ink and its own one
 * coral detail -- never a stock photo, never a tinted disc. `gu` lets a real
 * tag pick the object (a local dish, the outdoors); without it the category's.
 * The footprint is the same 1.7 × size the disc had, so no layout moves.
 */
export function PlaceGlyph({ glyph, loai, gu, size = 40 }: { glyph: IconName; loai?: string; gu?: string; size?: number }) {
  const { colors, radius } = useRudiTheme();
  const canh = Math.round(size * 1.7);
  const id = gu ?? (loai === undefined ? null : guTheoLoai(loai));
  return (
    <View style={[styles.glyphTo, { width: canh, height: canh, borderRadius: radius.small, backgroundColor: colors.paper, borderColor: colors.line }]}>
      {id === null ? <Ionicons color={colors.ink} name={glyph} size={size} /> : <GuGlyph id={id} size={Math.round(size * 1.15)} tone="ink" />}
    </View>
  );
}

/** Generic category artwork is labelled, never presented as the venue itself. */
export function PlaceArtwork({ loai, width, height = 180 }: { loai?: string; width?: number; height?: number }) {
  const { colors } = useRudiTheme();
  const [measured, setMeasured] = useState(width ?? 280);
  const scene: IllustrationName = loai === "cafe" ? "scene-cafe" : loai === "quan-an-local" ? "scene-street" : loai === "di-choi-dem" ? "scene-lanterns" : loai === "vui-choi" ? "scene-atelier" : "scene-hills";
  return (
    <View onLayout={(e) => setMeasured(e.nativeEvent.layout.width)} style={{ width: width ?? "100%", height, backgroundColor: colors.paper, overflow: "hidden" }}>
      <AssetIllustration name={scene} width={width ?? measured} height={height} />
      <View pointerEvents="none" style={{ position: "absolute", left: width !== undefined && width < 100 ? 6 : 10, bottom: 8, paddingHorizontal: 6, paddingVertical: 2, borderRadius: 4, backgroundColor: colors.card }}>
        <Text style={[typography.caption, { color: colors.ink }]}>Minh hoạ</Text>
      </View>
    </View>
  );
}

/**
 * The one reason a place prints: a small spark and the text, in the AI tone.
 * A fixture reason is one tag; a live reason is the model's sentence, so the
 * line budget follows the surface (`dong`) and grows with large text -- an
 * ellipsis here would hide the one fact the row exists to show (finish
 * review 11/09). The spark is decoration and stays out of the a11y tree.
 */
function LyDo({ text, dong = 2 }: { text: string; dong?: number }) {
  const { colors } = useRudiTheme();
  const { fontScale } = useWindowDimensions();
  return (
    <Inline gap={5}>
      <Ionicons color={colors.ai} importantForAccessibility="no" name="sparkles" size={13} />
      <Text numberOfLines={chuLon(fontScale) ? dong + 1 : dong} style={[typography.label, styles.flex1, { color: colors.ai }]}>{text}</Text>
    </Inline>
  );
}

/** The seal a place shows when it has no reason line: one mark, never two. */
function dauCon(dd: DiaDiemHienThi): string | null {
  return dd.lyDo ? null : dd.badge;
}

/**
 * The lead place, as a card (owner's mockup, 01/10): the picture — the
 * licensed photo, or the sketch of the kind of place when there is none —
 * edge to edge at the top with the heart on it; under it the name, the
 * subtitle on one line, the other facts as one quiet line, and two chips: the
 * reason in the AI tone and the price, whole (`tachGia`). The heart is the
 * card's sibling, not inside its tap, so a screen reader meets two controls.
 */
export function PlaceLead({ dd, daLuu, onOpen, onSave, testID, saving }: CommonProps) {
  const { colors, radius } = useRudiTheme();
  const { sizeClass } = useAdaptiveLayout();
  // 16:10 fills a phone's width at reading height; on a tablet the same ratio
  // is a screenful of photograph before the first name, so the frame widens.
  const tiLe = sizeClass === "compact" ? 16 / 9 : 21 / 9;
  const { gia } = tachGia(dd.facts);
  const dau = dauCon(dd);
  const chip = (mau: string, nen: string, icon: IconName, text: string) => (
    <View style={[styles.chipDan, { backgroundColor: nen, borderRadius: radius.control }]}>
      <Ionicons color={mau} importantForAccessibility="no" name={icon} size={13} />
      <Text style={[typography.label, styles.chuChipDan, { color: mau }]}>{text}</Text>
    </View>
  );
  const chips = dd.lyDo || gia ? (
    <View style={styles.hangChipDan}>
      {dd.lyDo ? chip(colors.ai, colors.aiSoft, "sparkles", dd.lyDo) : null}
      {gia ? chip(colors.inkSoft, colors.ground, "pricetag-outline", gia) : null}
    </View>
  ) : null;
  return (
    <View style={[styles.theDan, { backgroundColor: colors.card, borderColor: colors.line, borderRadius: radius.control }]} testID={testID}>
      <Pressable accessibilityLabel={`Mở ${dd.name}`} accessibilityRole="button" onPress={onOpen} style={({ pressed }) => [pressed && styles.pressed]}>
        <View>
          <View>
            {dd.anh === null ? (
              // No honest picture: the sketch of the KIND of place («ký hoạ trong
              // sổ», review 11/09 A1), never a 16:10 frame of nothing.
              <PlaceArtwork loai={dd.loai} height={sizeClass === "compact" ? 160 : 220} />
            ) : (
              <MediaSlot
                alt={dd.name}
                fallback={<PlaceArtwork loai={dd.loai} height={180} />}
                nguon={{ loai: "danh-muc", anh: dd.anh }}
                overlay={dau ? <View style={styles.badgeOnMedia}><Stamp label={dau} nen tilt={-2} tone="ai" /></View> : null}
                radius={0}
                ratio={tiLe}
              />
            )}
          </View>
          <View style={styles.chuDan}>
            {dd.anh === null && dau ? <Stamp label={dau} style={styles.leadGonDau} tone="ai" /> : null}
            <Text numberOfLines={2} style={[typography.h2, { color: colors.ink }]}>{dd.name}</Text>
            {dd.sub ? (
              <Text style={[typography.body, { color: colors.inkSoft }]}>
                {dd.sub}
              </Text>
            ) : null}
            <DongSuThat facts={dd.facts} kieu={typography.caption} mau={colors.inkFaint} />
            {chips}
          </View>
        </View>
      </Pressable>
      <View style={styles.timTrenAnh}>
        <IconButton
          accessibilityLabel={daLuu ? `Bỏ lưu ${dd.name}` : `Lưu ${dd.name}`}
          icon={daLuu ? "heart" : "heart-outline"}
          onPress={onSave}
          disabled={saving}
          loading={saving}
          selected={daLuu}
          tron
        />
      </View>
    </View>
  );
}

export function PlaceRow({ dd, daLuu, onOpen, onSave, testID, saving }: CommonProps) {
  const { colors, radius } = useRudiTheme();
  const { fontScale } = useWindowDimensions();
  // The facts that decide come first and whole: rating, distance and hours on
  // one line that may wrap, the price band with its unit on its own line, never
  // cut (review 08/09 F04; QA UI-021: found by what it is, not by position).
  // The seal sits beside the name at 1.0 and under the facts once the text is big.
  const { gia: cuoiFact } = tachGia(dd.facts);
  const chuLonHon = chuLon(fontScale);
  // A thumbnail that fails to load shows the category's object, never an
  // empty tinted square (review 08/09 F01). Reset when the picture changes.
  const [hong, setHong] = useState(false);
  const nguon = dd.anh === null ? null : ({ loai: "danh-muc", anh: dd.anh } as const);
  // Keyed on the picture, not on the object: the adapters rebuild that object
  // on every render, so an effect keyed on it cleared this state on the next
  // frame and the failure word could never be seen (F31 follow-up).
  const khoa = khoaNguon(nguon);
  useEffect(() => setHong(false), [khoa]);
  const ve = veKhung(nguon, { hong });
  return (
    <View style={[styles.row, { borderBottomColor: colors.line }]} testID={testID}>
      <Pressable accessibilityLabel={`Mở ${dd.name}`} accessibilityRole="button" onPress={onOpen} style={({ pressed }) => [styles.rowPress, pressed && styles.pressed]}>
        <View style={styles.rowMain}>
        {ve.source !== null ? (
          <View style={[styles.thumb, { borderRadius: radius.small }]}>
            <Image accessibilityLabel={dd.name} contentFit="cover" onError={() => setHong(true)} source={ve.source} style={StyleSheet.absoluteFill} />
          </View>
        ) : (
          <PlaceArtwork loai={dd.loai} width={88} height={88} />
        )}
        <View style={styles.rowText}>
          {/* The seal sits beside the name, so a matched row is as tall as any other. */}
          <View style={styles.rowTen}>
            <Text numberOfLines={2} style={[typography.title, styles.flex1, { color: colors.ink }]}>{dd.name}</Text>
            {dauCon(dd) && !chuLonHon ? <Stamp label={dauCon(dd) as string} tone="ai" /> : null}
          </View>
          {dd.lyDo ? <LyDo text={dd.lyDo} /> : null}
          {dd.sub ? <Text style={[typography.note, { color: colors.inkSoft }]}>{dd.sub}</Text> : null}
          {/* One text node per line: a row of several short texts keeps its
              first measurement when the row wraps and strands one word alone. */}
          <DongSuThat facts={dd.facts} kieu={typography.caption} mau={colors.inkSoft} />
          {/* The price band is never cut: it wraps when it must («200.000đ –
              250.000đ mỗi n…» at 1.3, QA 23/09; 9 rows in 10 at 390, UI-021). */}
          {dauCon(dd) && chuLonHon ? <Stamp label={dauCon(dd) as string} style={styles.rowBadgeDuoi} tone="ai" /> : null}
          {/* The thumbnail is a licensed photograph, so its credit is a line
              of this row (ADR-0017 §2.5) -- two lines, since a long author
              name has to wrap rather than end in an ellipsis. */}
          {/* A failed picture is a state the reader is told about, not only the screen reader (finish review 08/09). */}
          {ve.canhBao !== null ? <Text style={[typography.caption, { color: colors.warn }]}>{ve.canhBao}</Text> : null}
          {ve.ghiCong !== null ? (
            <Text numberOfLines={2} style={[typography.caption, { color: colors.inkFaint }]}>{ve.ghiCong}</Text>
          ) : null}
        </View>
        </View>
        {cuoiFact ? <Text style={[typography.label, styles.rowPrice, { color: colors.inkSoft, backgroundColor: colors.card }]}>{cuoiFact}</Text> : null}
      </Pressable>
      <View style={styles.rowHeart}>
      <IconButton
        accessibilityLabel={daLuu ? `Bỏ lưu ${dd.name}` : `Lưu ${dd.name}`}
        icon={daLuu ? "heart" : "heart-outline"}
        onPress={onSave}
        disabled={saving}
        loading={saving}
        tron
        selected={daLuu}
      />
      </View>
    </View>
  );
}

/**
 * What follows the lead: a pair to compare when there are at least two left,
 * then the rest as rows. With one place left there is nothing to compare and
 * it stays a row. Pure, so the live and the fixture screen split alike.
 */
export function taiSoSanh<T>(sauDan: readonly T[]): { soSanh: readonly [T, T] | null; hang: T[] } {
  if (sauDan.length < 2) return { soSanh: null, hang: [...sauDan] };
  return { soSanh: [sauDan[0], sauDan[1]], hang: sauDan.slice(2) };
}

/**
 * Two candidates on one axis. The same frame, name, line and facts for both,
 * so the eye moves across rather than down; there is no card around either.
 * The pair sits between the lead and the rows (report 07/09 §9.4).
 */
export function PlaceCompare({
  items,
  daLuu,
  onOpen,
  onSave,
  testID,
  saving,
}: {
  items: readonly [DiaDiemHienThi, DiaDiemHienThi];
  daLuu: (id: string) => boolean;
  onOpen: (id: string) => void;
  onSave: (id: string) => void;
  saving?: (id: string) => boolean;
  testID?: string;
}) {
  const { colors } = useRudiTheme();
  const { fontScale, width } = useWindowDimensions();
  // Two columns compare across; once the text is big enough that a half-width
  // column cannot hold a name and a price whole, the two stack and compare
  // down the same fields in the same order (review 08/09 F04).
  const xepDoc = chuLon(fontScale);
  const { sizeClass } = useAdaptiveLayout();
  if (width < 600 || xepDoc) return <View testID={testID}>{items.map((dd) => <PlaceRow key={dd.id} dd={dd} daLuu={daLuu(dd.id)} onOpen={() => onOpen(dd.id)} onSave={() => onSave(dd.id)} saving={saving?.(dd.id)} />)}</View>;
  return (
    <View style={[styles.soSanh, xepDoc && styles.soSanhDoc, { borderBottomColor: colors.line }]} testID={testID}>
      {items.map((dd) => {
        const luu = daLuu(dd.id);
        // The same facts the rows print, so the two really compare; the price
        // band (found by what it is, QA UI-021) gets its own line, never cut.
        const { gia: cuoiFact } = tachGia(dd.facts);
        const chu = (
          <>
            <Text numberOfLines={2} style={[typography.title, { minHeight: typography.title.lineHeight * 2, color: colors.ink }]}>{dd.name}</Text>
            {dd.lyDo ? <LyDo text={dd.lyDo} /> : null}
            {dd.sub ? <Text numberOfLines={2} style={[typography.note, { color: colors.inkSoft }]}>{dd.sub}</Text> : null}
            <DongSuThat facts={dd.facts} kieu={typography.note} mau={colors.inkFaint} soDong={2} />
            {cuoiFact ? <Text style={[typography.note, { color: colors.inkSoft }]}>{cuoiFact}</Text> : null}
          </>
        );

        return (
          <View key={dd.id} style={styles.ungVien}>
            <Pressable accessibilityLabel={`Mở ${dd.name}`} accessibilityRole="button" onPress={() => onOpen(dd.id)} style={({ pressed }) => [styles.ungVienPress, pressed && styles.pressed]}>
              <MediaSlot
                alt={dd.name}
                fallback={<PlaceArtwork loai={dd.loai} height={xepDoc ? 132 : sizeClass === "compact" ? 96 : 180} />}
                overlay={dauCon(dd) ? <View style={styles.badgeOnMedia}><Stamp label={dauCon(dd) as string} nen tilt={-2} tone="ai" /></View> : null}
                // The credit is the frame's own line now, printed under the
                // picture instead of after the facts: it was the one call site
                // that handed this frame a bare address (review 08/09, F31).
                nguon={dd.anh === null ? null : { loai: "danh-muc", anh: dd.anh }}
                ratio={xepDoc ? 2 : sizeClass === "compact" ? 4 / 3 : 16 / 7}
              />
              {chu}
            </Pressable>
            {/* The heart lies on the picture's top corner but is not inside «Mở …»:
                a button within a button (QA UI-114, axe nested-interactive). */}
            <View style={styles.timOnMedia}>
              <IconButton
                accessibilityLabel={luu ? `Bỏ lưu ${dd.name}` : `Lưu ${dd.name}`}
                icon={luu ? "heart" : "heart-outline"}
                onPress={() => onSave(dd.id)}
                disabled={saving?.(dd.id)}
                loading={saving?.(dd.id)}
                selected={luu}
              />
            </View>
          </View>
        );
      })}
    </View>
  );
}

/**
 * A place's quiet facts (`chiaDongSuThat`), as two lines that never end or
 * start on «·» (finish review 03/10): the rating and the distance, with a
 * star before the rating when the line opens on it (`moDauBangSao`), drawn in
 * the words' own ink, inline and held to the rating by a no-break space; then
 * whether it is open, and its hours. A screen reader hears the words alone.
 */
function DongSuThat({ facts, kieu, mau, soDong }: { facts: readonly { icon: string; text: string }[]; kieu: TextStyle; mau: string; soDong?: number }) {
  const { dau, gio } = chiaDongSuThat(facts);
  const sao = moDauBangSao(facts);
  return (
    <>
      {dau ? (
        <Text accessibilityLabel={sao ? dau : undefined} numberOfLines={soDong} style={[kieu, { color: mau }]}>
          {sao ? (
            <>
              <Ionicons color={mau} name="star" size={12} />
              {"\u00a0"}
            </>
          ) : null}
          {dau}
        </Text>
      ) : null}
      {gio ? (
        <Text numberOfLines={soDong} style={[kieu, { color: mau }]}>
          {gio}
        </Text>
      ) : null}
    </>
  );
}

const styles = StyleSheet.create({
  soSanh: { flexDirection: "row", gap: 16, paddingBottom: 12, borderBottomWidth: StyleSheet.hairlineWidth },
  soSanhDoc: { flexDirection: "column", gap: 20 },
  rowBadgeDuoi: { alignSelf: "flex-start", marginTop: 4 },
  ungVien: { flex: 1, minWidth: 0 },
  ungVienPress: { gap: 6 },
  ungVienDau: { flexDirection: "row", alignItems: "center", gap: 8, marginBottom: 4 },
  // The no-photo lead: the sketch sheet and the text stack like the photo lead; only the hairline under it is its own.
  leadGonDau: { alignSelf: "flex-start" },
  leadGonDauHang: { flexDirection: "row", alignItems: "flex-start" },
  leadGonDuoi: { gap: 6, padding: 14, paddingTop: 0 },
  timOnMedia: { position: "absolute", right: 6, top: 6 },
  dauUngVien: { alignSelf: "flex-start", marginBottom: 4 },
  rowTen: { flexDirection: "row", alignItems: "center", gap: 8 },
  flex1: { flex: 1, minWidth: 0 },
  glyphTo: { alignItems: "center", justifyContent: "center", borderWidth: StyleSheet.hairlineWidth, flexShrink: 0 },
  theDan: { borderWidth: StyleSheet.hairlineWidth, overflow: "hidden" },
  // The card is the frame: the sketch drops its own edge and corners.
  kyHoaTrongThe: { borderWidth: 0, borderRadius: 0 },
  chuDan: { gap: 6, padding: 14 },
  hangChipDan: { flexDirection: "row", flexWrap: "wrap", gap: 8, marginTop: 2 },
  chipDan: { flexDirection: "row", alignItems: "center", gap: 6, paddingHorizontal: 10, paddingVertical: 6, maxWidth: "100%" },
  // Money and the reason wrap rather than clip.
  chuChipDan: { flexShrink: 1 },
  timTrenAnh: { position: "absolute", top: 10, right: 10 },
  badgeOnMedia: { position: "absolute", left: 12, top: 12 },
  pressed: { opacity: 0.86 },
  row: { paddingVertical: 16, borderBottomWidth: StyleSheet.hairlineWidth },
  rowMain: { flexDirection: "row", alignItems: "flex-start", gap: 14 },
  rowHeart: { position: "absolute", top: 20, left: 36 },
  rowPrice: { borderRadius: 8, paddingHorizontal: 10, paddingVertical: 8 },
  rowPress: { gap: 10, minHeight: 88 },
  thumb: { width: 88, height: 88, overflow: "hidden", flexShrink: 0 },
  rowText: { flex: 1, gap: 4, minWidth: 0 },
});
