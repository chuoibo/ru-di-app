import { Ionicons } from "@expo/vector-icons";
import { Image } from "expo-image";
import { useEffect, useState } from "react";
import { Pressable, StyleSheet, Text as RNText, View, useWindowDimensions, type TextStyle } from "react-native";

import { AssetIllustration } from "../../asset-pack/react-native/AssetIllustration";
import { chuLon } from "../../adaptive";
import { typography, useRudiTheme } from "../../theme";
import { Text as NativeMetricText } from "../../ui/NativeFontMetrics";
import { IconButton, Inline, type IconName } from "../../ui";
import { MediaSlot } from "../../ui/MediaSlot";
import { khoaNguon, veKhung, type AnhCoGhiCong } from "../../ui/ghi-cong";
import { Stamp } from "../../ui/Stamp";
import { useAdaptiveLayout } from "../../ui/useAdaptiveLayout";
import { GuGlyph } from "../../ui/art/Gu";
import { chiaDongSuThat, guTheoLoai, moDauBangSao, tachGia } from "../../kham-pha/dia-diem";
import { canhChoDiaDiem, type TenCanh, type Vung } from "../../kham-pha/canh-ve";

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
  /** The painted stand-in the list chose for this card (`phanCanhDanhSach`); absent, the category's region-free scene. */
  canh?: TenCanh | null;
}

interface CommonProps {
  dd: DiaDiemHienThi;
  daLuu: boolean;
  onOpen: () => void;
  onSave: () => void;
  testID?: string;
  saving?: boolean;
  /** A short phone opens with the place name; the full-width media and facts follow. */
  compact?: boolean;
  /** Use only the existing surface owner; keep other callers on their original RN path. */
  nativeMetrics?: boolean;
  /** Supplied by that owner; absent preserves the original window-based layout. */
  largeText?: boolean;
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

/**
 * Generic category artwork stands in when a place has no photograph. It carries
 * no text label (owner decision 2026-10-20): the drawn scene reads as a drawing,
 * and a real photograph always arrives through MediaSlot with its credit.
 *
 * Because there is no label, a painting of another city reads as this place
 * (finish review 21/10): the scene comes from `canh-ve.ts`, by category first
 * and only where the painting is of the place's own region. `canh` is what the
 * caller chose for this card (a list varies neighbours); left out, the
 * category's scene that is true of anywhere. `null` -- nothing painted is
 * honest here -- draws the category's object on the paper instead.
 * `nativeMetrics` is accepted for call-site symmetry; there is no text left to measure.
 */
export function PlaceArtwork({ loai, canh, width, height = 180, fillFrame = false }: { loai?: string; canh?: TenCanh | null; width?: number; height?: number; nativeMetrics?: boolean; fillFrame?: boolean }) {
  const { colors } = useRudiTheme();
  const [measured, setMeasured] = useState({ width: width ?? 280, height });
  const scene: TenCanh | null = canh === undefined ? canhChoDiaDiem({ loai, vung: KHONG_VUNG }) : canh;
  return (
    <View
      onLayout={(e) => {
        const { width: nextWidth, height: nextHeight } = e.nativeEvent.layout;
        if (fillFrame) {
          if (!Number.isFinite(nextWidth) || nextWidth <= 0 || !Number.isFinite(nextHeight) || nextHeight <= 0) return;
          setMeasured((current) => current.width === nextWidth && current.height === nextHeight ? current : { width: nextWidth, height: nextHeight });
        } else {
          setMeasured((current) => Object.is(current.width, nextWidth) ? current : { ...current, width: nextWidth });
        }
      }}
      style={fillFrame ? [StyleSheet.absoluteFill, { backgroundColor: colors.paper, overflow: "hidden" }] : { width: width ?? "100%", height, backgroundColor: colors.paper, overflow: "hidden" }}
    >
      {scene === null ? (
        // The category's object in ink on the paper: a drawing of the kind of place, of nowhere in particular.
        <View style={[StyleSheet.absoluteFill, styles.giua]}>
          <GuGlyph id={guTheoLoai(loai ?? "")} size={Math.max(28, Math.min(72, Math.round((fillFrame ? measured.height : height) * 0.42)))} tone="ink" />
        </View>
      ) : (
        /* The frame owns layout; numeric dimensions choose raster resolution only. */
        <AssetIllustration
          name={scene}
          width={fillFrame ? measured.width : width ?? measured.width}
          height={fillFrame ? measured.height : height}
          style={fillFrame ? [StyleSheet.absoluteFill, { width: "100%", height: "100%" }] : undefined}
        />
      )}
    </View>
  );
}

/** No region known: only scenes true of anywhere. */
const KHONG_VUNG: ReadonlySet<Vung> = new Set();

/**
 * The one reason a place prints: a small spark and the text, in the AI tone.
 * A fixture reason is one tag; a live reason is the model's sentence, so the
 * line budget follows the surface (`dong`) and grows with large text -- an
 * ellipsis here would hide the one fact the row exists to show (finish
 * review 11/09). The spark is decoration and stays out of the a11y tree.
 */
function LyDo({ text, dong, nativeMetrics = false, largeText }: { text: string; dong?: number; nativeMetrics?: boolean; largeText?: boolean }) {
  const { colors } = useRudiTheme();
  const { fontScale } = useWindowDimensions();
  const CopyText = nativeMetrics ? NativeMetricText : RNText;
  const large = largeText ?? chuLon(fontScale);
  return (
    <Inline gap={5}>
      <Ionicons color={colors.ai} importantForAccessibility="no" name="sparkles" size={13} />
      <CopyText numberOfLines={dong === undefined ? undefined : large ? dong + 1 : dong} style={[typography.label, styles.flex1, { color: colors.ai }]}>{text}</CopyText>
    </Inline>
  );
}

/** The seal a place shows when it has no reason line: one mark, never two. */
function dauCon(dd: DiaDiemHienThi): string | null {
  return dd.lyDo ? null : dd.badge;
}

/**
 * The lead place, as a card: a short phone groups the full name and
 * subtitle above the picture; a larger viewport keeps them below it.
 * The licensed photo or labelled category sketch stays full width, followed
 * by the complete facts, grounded reason and price (`tachGia`). The heart
 * is the card's sibling, not inside its tap, so there are two controls.
 */
export function PlaceLead({ dd, daLuu, onOpen, onSave, testID, saving, compact = false, nativeMetrics = false }: CommonProps) {
  const { colors, radius } = useRudiTheme();
  const CopyText = nativeMetrics ? NativeMetricText : RNText;
  const { sizeClass } = useAdaptiveLayout();
  // 16:10 fills a phone's width at reading height; on a tablet the same ratio
  // is a screenful of photograph before the first name, so the frame widens.
  const tiLe = sizeClass === "compact" ? 16 / 9 : 21 / 9;
  const { gia } = tachGia(dd.facts);
  const dau = dauCon(dd);
  const name = <CopyText numberOfLines={compact ? undefined : 2} style={[typography.h2, { color: colors.ink }]}>{dd.name}</CopyText>;
  const sub = dd.sub ? <CopyText style={[typography.body, { color: colors.inkSoft }]}>{dd.sub}</CopyText> : null;
  const chip = (mau: string, nen: string, icon: IconName, text: string) => (
    <View style={[styles.chipDan, { backgroundColor: nen, borderRadius: radius.control }]}>
      <Ionicons color={mau} importantForAccessibility="no" name={icon} size={13} />
      <CopyText style={[typography.label, styles.chuChipDan, { color: mau }]}>{text}</CopyText>
    </View>
  );
  const chips = dd.lyDo || gia ? (
    <View style={styles.hangChipDan}>
      {dd.lyDo ? chip(colors.ai, colors.aiSoft, "sparkles", dd.lyDo) : null}
      {gia ? chip(colors.inkSoft, colors.ground, "pricetag-outline", gia) : null}
    </View>
  ) : null;
  // Once identity moves to the header, an empty metadata slot needs no frame.
  const hasDetails = !compact || Boolean(dd.anh === null && dau) || dd.facts.length > 0 || chips !== null;
  return (
    <View style={[styles.theDan, { backgroundColor: colors.card, borderColor: colors.line, borderRadius: radius.control }]} testID={testID}>
      <Pressable accessibilityLabel={`Mở ${dd.name}`} accessibilityRole="button" onPress={onOpen} style={({ pressed }) => [pressed && styles.pressed]}>
        <View>
          {/* On a short phone, identity arrives before another picture. The
              save control remains a sibling and owns its reserved corner. */}
          {compact ? <View style={styles.tenDanGon}>
            <View style={styles.nhanDanGon}>{name}</View>
            {sub}
          </View> : null}
          <View>
            {dd.anh === null ? (
              // No honest picture: the sketch of the KIND of place («ký hoạ trong
              // sổ», review 11/09 A1), never a 16:10 frame of nothing.
              <PlaceArtwork canh={dd.canh} nativeMetrics={nativeMetrics} loai={dd.loai} height={sizeClass === "compact" ? 160 : 220} />
            ) : (
              <MediaSlot
                alt={dd.name}
                fallback={<PlaceArtwork canh={dd.canh} nativeMetrics={nativeMetrics} loai={dd.loai} height={180} />}
                nguon={{ loai: "danh-muc", anh: dd.anh }}
                overlay={dau ? <View style={styles.badgeOnMedia}><Stamp label={dau} nen tilt={-2} tone="ai" /></View> : null}
                radius={0}
                ratio={tiLe}
              />
            )}
          </View>
          {hasDetails ? <View style={styles.chuDan}>
            {dd.anh === null && dau ? <Stamp label={dau} style={styles.leadGonDau} tone="ai" /> : null}
            {compact ? null : name}
            {compact ? null : sub}
            <DongSuThat nativeMetrics={nativeMetrics} facts={dd.facts} kieu={typography.caption} mau={colors.inkFaint} />
            {chips}
          </View> : null}
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

export function PlaceRow({ dd, daLuu, onOpen, onSave, testID, saving, nativeMetrics = false, largeText }: CommonProps) {
  const { colors, radius } = useRudiTheme();
  const CopyText = nativeMetrics ? NativeMetricText : RNText;
  const { fontScale } = useWindowDimensions();
  // The facts that decide come first and whole: rating, distance and hours on
  // one line that may wrap, the price band with its unit on its own line, never
  // cut (review 08/09 F04; QA UI-021: found by what it is, not by position).
  // The seal sits beside the name at 1.0 and under the facts once the text is big.
  const { gia: cuoiFact } = tachGia(dd.facts);
  const chuLonHon = largeText ?? chuLon(fontScale);
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
        <View style={[styles.rowMain, chuLonHon && styles.rowMainLarge]}>
        {ve.source !== null ? (
          <View style={[styles.thumb, chuLonHon && styles.thumbLarge, { borderRadius: radius.small }]}>
            <Image accessibilityLabel={dd.name} contentFit="cover" onError={() => setHong(true)} source={ve.source} style={StyleSheet.absoluteFill} />
          </View>
        ) : (
          <PlaceArtwork canh={dd.canh} nativeMetrics={nativeMetrics} loai={dd.loai} width={chuLonHon ? undefined : 88} height={chuLonHon ? 132 : 88} />
        )}
        <View style={[styles.rowText, chuLonHon && styles.rowTextLarge]}>
          {/* The seal sits beside the name, so a matched row is as tall as any other. */}
          <View style={styles.rowTen}>
            <CopyText style={[typography.title, styles.flex1, { color: colors.ink }]}>{dd.name}</CopyText>
            {dauCon(dd) && !chuLonHon ? <Stamp label={dauCon(dd) as string} tone="ai" /> : null}
          </View>
          {dd.sub ? <CopyText style={[typography.note, { color: colors.inkSoft }]}>{dd.sub}</CopyText> : null}
          {/* One text node per line: a row of several short texts keeps its
              first measurement when the row wraps and strands one word alone. */}
          <DongSuThat nativeMetrics={nativeMetrics} facts={dd.facts} kieu={typography.caption} mau={colors.inkSoft} />
          {/* The price band is never cut: it wraps when it must («200.000đ –
              250.000đ mỗi n…» at 1.3, QA 23/09; 9 rows in 10 at 390, UI-021). */}
          {dauCon(dd) && chuLonHon ? <Stamp label={dauCon(dd) as string} style={styles.rowBadgeDuoi} tone="ai" /> : null}
          {/* The thumbnail is a licensed photograph, so its credit is a line
              of this row (ADR-0017 §2.5) -- two lines, since a long author
              name has to wrap rather than end in an ellipsis. */}
          {/* A failed picture is a state the reader is told about, not only the screen reader (finish review 08/09). */}
          {ve.canhBao !== null ? <CopyText style={[typography.caption, { color: colors.warn }]}>{ve.canhBao}</CopyText> : null}
          {ve.ghiCong !== null ? (
            <CopyText numberOfLines={2} style={[typography.caption, { color: colors.inkFaint }]}>{ve.ghiCong}</CopyText>
          ) : null}
        </View>
        </View>
        {cuoiFact ? <CopyText style={[typography.label, styles.rowPrice, { color: colors.inkSoft, backgroundColor: colors.card, borderColor: colors.line }]}>{cuoiFact}</CopyText> : null}
        {dd.lyDo ? <LyDo nativeMetrics={nativeMetrics} largeText={largeText} text={dd.lyDo} /> : null}
      </Pressable>
      <View style={chuLonHon ? styles.rowHeartLarge : styles.rowHeart}>
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
  nativeMetrics = false,
  largeText,
}: {
  items: readonly [DiaDiemHienThi, DiaDiemHienThi];
  daLuu: (id: string) => boolean;
  onOpen: (id: string) => void;
  onSave: (id: string) => void;
  saving?: (id: string) => boolean;
  testID?: string;
  nativeMetrics?: boolean;
  largeText?: boolean;
}) {
  const { colors } = useRudiTheme();
  const { fontScale, width } = useWindowDimensions();
  const CopyText = nativeMetrics ? NativeMetricText : RNText;
  // Two columns compare across; once the text is big enough that a half-width
  // column cannot hold a name and a price whole, the two stack and compare
  // down the same fields in the same order (review 08/09 F04).
  const xepDoc = largeText ?? chuLon(fontScale);
  const { sizeClass } = useAdaptiveLayout();
  // An opted-in wide pair changes direction rather than replacing its place controllers.
  if (width < 600 || (!nativeMetrics && xepDoc)) return <View testID={testID}>{items.map((dd) => <PlaceRow key={dd.id} dd={dd} daLuu={daLuu(dd.id)} onOpen={() => onOpen(dd.id)} onSave={() => onSave(dd.id)} saving={saving?.(dd.id)} nativeMetrics={nativeMetrics} largeText={largeText} />)}</View>;
  const mediaRatio = xepDoc ? 2 : sizeClass === "compact" ? 4 / 3 : 16 / 7;
  return (
    <View style={[styles.soSanh, xepDoc && styles.soSanhDoc, { borderBottomColor: colors.line }]} testID={testID}>
      {items.map((dd) => {
        const luu = daLuu(dd.id);
        // The same facts the rows print, so the two really compare; the price
        // band (found by what it is, QA UI-021) gets its own line, never cut.
        const { gia: cuoiFact } = tachGia(dd.facts);
        // Names use their natural height; facts stay together before the full reason.
        const chu = (
          <View style={styles.ungVienChu}>
            <CopyText numberOfLines={nativeMetrics && xepDoc ? undefined : 2} style={[typography.title, { color: colors.ink }]}>{dd.name}</CopyText>
            {dd.sub ? <CopyText numberOfLines={nativeMetrics && xepDoc ? undefined : 2} style={[typography.note, { color: colors.inkSoft }]}>{dd.sub}</CopyText> : null}
            <DongSuThat nativeMetrics={nativeMetrics} facts={dd.facts} kieu={typography.note} mau={colors.inkFaint} soDong={nativeMetrics && xepDoc ? undefined : 2} />
            {cuoiFact ? <CopyText style={[typography.note, { color: colors.inkSoft }]}>{cuoiFact}</CopyText> : null}
            {dd.lyDo ? <View style={styles.ungVienLyDo}><LyDo nativeMetrics={nativeMetrics} largeText={largeText} text={dd.lyDo} /></View> : null}
          </View>
        );

        return (
          <View key={dd.id} style={[styles.ungVien, xepDoc && styles.ungVienDoc]}>
            <Pressable accessibilityLabel={`Mở ${dd.name}`} accessibilityRole="button" onPress={() => onOpen(dd.id)} style={({ pressed }) => [styles.ungVienPress, pressed && styles.pressed]}>
              <MediaSlot
                alt={dd.name}
                fallback={<PlaceArtwork canh={dd.canh} nativeMetrics={nativeMetrics} fillFrame={nativeMetrics} loai={dd.loai} height={xepDoc ? 132 : sizeClass === "compact" ? 96 : 180} />}
                overlay={dauCon(dd) ? <View style={styles.badgeOnMedia}><Stamp label={dauCon(dd) as string} nen tilt={-2} tone="ai" /></View> : null}
                // The credit is the frame's own line now, printed under the
                // picture instead of after the facts: it was the one call site
                // that handed this frame a bare address (review 08/09, F31).
                nguon={dd.anh === null ? null : { loai: "danh-muc", anh: dd.anh }}
                ratio={mediaRatio}
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
function DongSuThat({ facts, kieu, mau, soDong, nativeMetrics = false }: { facts: readonly { icon: string; text: string }[]; kieu: TextStyle; mau: string; soDong?: number; nativeMetrics?: boolean }) {
  const CopyText = nativeMetrics ? NativeMetricText : RNText;
  const { dau, gio } = chiaDongSuThat(facts);
  const sao = moDauBangSao(facts);
  return (
    <>
      {dau ? nativeMetrics && sao ? (
        // Keep decoration outside the paragraph so its factual copy can use owner metrics.
        <View style={styles.factLine}>
          <Ionicons accessible={false} allowFontScaling={false} importantForAccessibility="no" color={mau} name="star" size={12} />
          <CopyText accessibilityLabel={dau} numberOfLines={soDong} style={[kieu, styles.factCopy, { color: mau }]}>{dau}</CopyText>
        </View>
      ) : (
        <CopyText accessibilityLabel={sao ? dau : undefined} numberOfLines={soDong} style={[kieu, { color: mau }]}>
          {sao ? (
            <>
              <Ionicons color={mau} name="star" size={12} />
              {"\u00a0"}
            </>
          ) : null}
          {dau}
        </CopyText>
      ) : null}
      {gio ? (
        <CopyText numberOfLines={soDong} style={[kieu, { color: mau }]}>
          {gio}
        </CopyText>
      ) : null}
    </>
  );
}

const styles = StyleSheet.create({
  giua: { alignItems: "center", justifyContent: "center" },
  factLine: { flexDirection: "row", alignItems: "baseline", gap: 4 },
  factCopy: { flex: 1, minWidth: 0 },
  soSanh: { flexDirection: "row", gap: 16, paddingBottom: 12, borderBottomWidth: StyleSheet.hairlineWidth },
  soSanhDoc: { flexDirection: "column", gap: 20 },
  rowBadgeDuoi: { alignSelf: "flex-start", marginTop: 4 },
  // Definite widths with auto basis avoid carrying a column height into the next row.
  ungVien: { width: "50%", flexGrow: 0, flexShrink: 1, flexBasis: "auto", minWidth: 0 },
  ungVienDoc: { width: "100%", flexShrink: 0 },
  ungVienPress: { gap: 6 },
  // Facts read together; an uncapped match explanation follows at its own pace.
  ungVienChu: { gap: 4 },
  ungVienLyDo: { marginTop: 4 },
  ungVienDau: { flexDirection: "row", alignItems: "center", gap: 8, marginBottom: 4 },
  // The no-photo lead: the sketch sheet and the text stack like the photo lead; only the hairline under it is its own.
  leadGonDau: { alignSelf: "flex-start" },
  timOnMedia: { position: "absolute", right: 6, top: 6 },
  dauUngVien: { alignSelf: "flex-start", marginBottom: 4 },
  rowTen: { flexDirection: "row", alignItems: "center", gap: 8 },
  flex1: { flex: 1, minWidth: 0 },
  glyphTo: { alignItems: "center", justifyContent: "center", borderWidth: StyleSheet.hairlineWidth, flexShrink: 0 },
  theDan: { borderWidth: StyleSheet.hairlineWidth, overflow: "hidden" },
  // The card is the frame: the sketch drops its own edge and corners.
  kyHoaTrongThe: { borderWidth: 0, borderRadius: 0 },
  // The title row shares the heart's 48dp height. The subtitle spans the
  // full inner width below both, so it never runs underneath the control.
  tenDanGon: { minHeight: 68, paddingHorizontal: 14, paddingVertical: 10, gap: 6 },
  nhanDanGon: { minHeight: 48, justifyContent: "center", paddingRight: 60 },
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
  rowMainLarge: { flexDirection: "column" },
  rowHeart: { position: "absolute", top: 20, left: 36 },
  rowHeartLarge: { position: "absolute", right: 10, top: 24 },
  rowPrice: { alignSelf: "flex-start", borderRadius: 8, borderWidth: StyleSheet.hairlineWidth, paddingHorizontal: 10, paddingVertical: 6 },
  rowPress: { gap: 10, minHeight: 88 },
  thumb: { width: 88, height: 88, overflow: "hidden", flexShrink: 0 },
  thumbLarge: { width: "100%", height: 132 },
  rowText: { flex: 1, gap: 4, minWidth: 0 },
  rowTextLarge: { flex: 0, alignSelf: "stretch" },
});
