import { Ionicons } from "@expo/vector-icons";
import { Image } from "expo-image";
import { useEffect, useState } from "react";
import { Pressable, StyleSheet, Text, View, useWindowDimensions } from "react-native";

import { chuLon } from "../../adaptive";
import { typography, useRudiTheme } from "../../theme";
import { IconButton, Inline, type IconName } from "../../ui";
import { MediaSlot } from "../../ui/MediaSlot";
import { khoaNguon, veKhung, type AnhCoGhiCong } from "../../ui/ghi-cong";
import { Stamp } from "../../ui/Stamp";
import { useAdaptiveLayout } from "../../ui/useAdaptiveLayout";
import { GuGlyph } from "../../ui/art/Gu";
import { guTheoLoai } from "../../kham-pha/dia-diem";

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
}

interface CommonProps {
  dd: DiaDiemHienThi;
  daLuu: boolean;
  onOpen: () => void;
  onSave: () => void;
  testID?: string;
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

export function PlaceLead({ dd, daLuu, onOpen, onSave, testID }: CommonProps) {
  const { colors } = useRudiTheme();
  const { sizeClass } = useAdaptiveLayout();
  // 16:10 fills a phone's width at reading height; on a tablet the same ratio
  // is a screenful of photograph before the first name, so the frame widens.
  const tiLe = sizeClass === "compact" ? 16 / 10 : 21 / 9;
  const chu = (
    <>
      <Text style={[typography.h2, { color: colors.ink }]}>{dd.name}</Text>
      {dd.lyDo ? <LyDo text={dd.lyDo} /> : null}
      {dd.sub ? <Text style={[typography.body, { color: colors.inkSoft }]}>{dd.sub}</Text> : null}
      {dd.facts.length > 0 ? (
        <Inline gap={12} wrap>
          {dd.facts.map((f) => (
            <Inline gap={5} key={f.icon + f.text}>
              <Ionicons color={f.icon === "star" ? colors.accent : colors.inkFaint} name={f.icon} size={15} />
              <Text style={[typography.label, { color: colors.inkSoft }]}>{f.text}</Text>
            </Inline>
          ))}
        </Inline>
      ) : null}
    </>
  );
  const tim = (
    <IconButton
      accessibilityLabel={daLuu ? `Bỏ lưu ${dd.name}` : `Lưu ${dd.name}`}
      icon={daLuu ? "heart" : "heart-outline"}
      onPress={onSave}
      selected={daLuu}
    />
  );
  if (dd.anh === null) {
    // No honest picture: an editorial header of object, name and facts, not a
    // 16:10 frame with an icon in the middle (review 08/09 F01, report §9.4).
    return (
      <View style={[styles.leadGon, { borderBottomColor: colors.line }]} testID={testID}>
        <Pressable accessibilityLabel={`Mở ${dd.name}`} accessibilityRole="button" onPress={onOpen} style={({ pressed }) => [styles.leadGonPress, pressed && styles.pressed]}>
          <PlaceGlyph glyph={dd.glyph} gu={dd.gu} loai={dd.loai} size={34} />
          <View style={[styles.leadText, styles.flex1, styles.leadGonChu]}>
            {dauCon(dd) ? <Stamp label={dauCon(dd) as string} style={styles.leadGonDau} tone="ai" /> : null}
            {chu}
          </View>
        </Pressable>
        {tim}
      </View>
    );
  }
  return (
    <View style={styles.lead} testID={testID}>
      <Pressable accessibilityLabel={`Mở ${dd.name}`} accessibilityRole="button" onPress={onOpen} style={({ pressed }) => [styles.leadPress, pressed && styles.pressed]}>
        <MediaSlot
          alt={dd.name}
          fallback={<PlaceGlyph glyph={dd.glyph} gu={dd.gu} loai={dd.loai} size={44} />}
          nguon={{ loai: "danh-muc", anh: dd.anh }}
          overlay={dauCon(dd) ? <View style={styles.badgeOnMedia}><Stamp label={dauCon(dd) as string} nen tilt={-2} tone="ai" /></View> : null}
          ratio={tiLe}
        />
        <View style={styles.leadText}>{chu}</View>
      </Pressable>
      <View style={styles.leadSave}>{tim}</View>
    </View>
  );
}

export function PlaceRow({ dd, daLuu, onOpen, onSave, testID }: CommonProps) {
  const { colors, radius } = useRudiTheme();
  const { fontScale } = useWindowDimensions();
  // The facts that decide come first and whole: rating and distance on one
  // line, the price band with its unit on its own, so a large font truncates
  // the prose and never the price (review 08/09 F04). The seal sits beside
  // the name at 1.0 and under the facts once the text is big.
  const facts = dd.facts.map((f) => f.text);
  const dauFacts = facts.slice(0, -1).join(" · ");
  const cuoiFact = facts.length > 0 ? facts[facts.length - 1] : "";
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
        {ve.source !== null ? (
          <View style={[styles.thumb, { borderRadius: radius.small }]}>
            <Image accessibilityLabel={dd.name} contentFit="cover" onError={() => setHong(true)} source={ve.source} style={StyleSheet.absoluteFill} />
          </View>
        ) : (
          <PlaceGlyph glyph={dd.glyph} gu={dd.gu} loai={dd.loai} size={33} />
        )}
        <View style={styles.rowText}>
          {/* The seal sits beside the name, so a matched row is as tall as any other. */}
          <View style={styles.rowTen}>
            <Text numberOfLines={2} style={[typography.title, styles.flex1, { color: colors.ink }]}>{dd.name}</Text>
            {dauCon(dd) && !chuLonHon ? <Stamp label={dauCon(dd) as string} tone="ai" /> : null}
          </View>
          {dd.lyDo ? <LyDo text={dd.lyDo} /> : null}
          {dd.sub ? <Text numberOfLines={1} style={[typography.caption, { color: colors.inkSoft }]}>{dd.sub}</Text> : null}
          {/* One text node per line: a row of several short texts keeps its
              first measurement when the row wraps and strands one word alone. */}
          {dauFacts ? <Text numberOfLines={1} style={[typography.caption, { color: colors.inkFaint }]}>{dauFacts}</Text> : null}
          {cuoiFact ? <Text numberOfLines={1} style={[typography.caption, { color: colors.inkFaint }]}>{cuoiFact}</Text> : null}
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
      </Pressable>
      <IconButton
        accessibilityLabel={daLuu ? `Bỏ lưu ${dd.name}` : `Lưu ${dd.name}`}
        icon={daLuu ? "heart" : "heart-outline"}
        onPress={onSave}
        quiet
        selected={daLuu}
      />
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
}: {
  items: readonly [DiaDiemHienThi, DiaDiemHienThi];
  daLuu: (id: string) => boolean;
  onOpen: (id: string) => void;
  onSave: (id: string) => void;
  testID?: string;
}) {
  const { colors } = useRudiTheme();
  const { fontScale } = useWindowDimensions();
  // Two columns compare across; once the text is big enough that a half-width
  // column cannot hold a name and a price whole, the two stack and compare
  // down the same fields in the same order (review 08/09 F04).
  const xepDoc = chuLon(fontScale);
  // One axis means one frame. When neither candidate has a picture, both take
  // the compact header and the pair costs less height; when one of them does,
  // BOTH keep the 4:3 frame and the photoless one draws its object inside that
  // frame. A photo tile beside a glyph strip is two shapes and the eye stops
  // comparing (finish review of this batch).
  const khongAnhNao = items.every((dd) => !dd.anh);
  return (
    <View style={[styles.soSanh, xepDoc && styles.soSanhDoc, { borderBottomColor: colors.line }]} testID={testID}>
      {items.map((dd) => {
        const luu = daLuu(dd.id);
        // The same facts the rows print, so the two really compare; the last
        // fact (the price band, with its unit) gets its own line so a half-width
        // tile never breaks «80K/người» across two lines (finish review 08/09).
        const facts = dd.facts.map((f) => f.text);
        const dauFacts = facts.slice(0, -1).join(" · ");
        const cuoiFact = facts.length > 0 ? facts[facts.length - 1] : "";
        const chu = (
          <>
            <Text numberOfLines={2} style={[typography.title, { color: colors.ink }]}>{dd.name}</Text>
            {dd.lyDo ? <LyDo text={dd.lyDo} /> : null}
            {dd.sub ? <Text numberOfLines={2} style={[typography.note, { color: colors.inkSoft }]}>{dd.sub}</Text> : null}
            {dauFacts ? <Text numberOfLines={1} style={[typography.note, { color: colors.inkFaint }]}>{dauFacts}</Text> : null}
            {cuoiFact ? <Text numberOfLines={1} style={[typography.note, { color: colors.inkFaint }]}>{cuoiFact}</Text> : null}
          </>
        );
        if (khongAnhNao) {
          // No honest picture on one side: BOTH tiles take the object, the seal
          // and the heart on one line, then the same words, so the two still
          // compare on one axis. A photo tile beside a glyph strip does not.
          return (
            <View key={dd.id} style={styles.ungVien}>
              <View style={styles.ungVienDau}>
                <PlaceGlyph glyph={dd.glyph} gu={dd.gu} loai={dd.loai} size={24} />
                {dauCon(dd) ? <Stamp label={dauCon(dd) as string} tone="ai" /> : null}
                <View style={styles.flex1} />
                <IconButton
                  accessibilityLabel={luu ? `Bỏ lưu ${dd.name}` : `Lưu ${dd.name}`}
                  icon={luu ? "heart" : "heart-outline"}
                  onPress={() => onSave(dd.id)}
                  quiet
                  selected={luu}
                />
              </View>
              <Pressable accessibilityLabel={`Mở ${dd.name}`} accessibilityRole="button" onPress={() => onOpen(dd.id)} style={({ pressed }) => [styles.ungVienPress, pressed && styles.pressed]}>
                {chu}
              </Pressable>
            </View>
          );
        }
        return (
          <View key={dd.id} style={styles.ungVien}>
            <Pressable accessibilityLabel={`Mở ${dd.name}`} accessibilityRole="button" onPress={() => onOpen(dd.id)} style={({ pressed }) => [styles.ungVienPress, pressed && styles.pressed]}>
              <MediaSlot
                alt={dd.name}
                fallback={<PlaceGlyph glyph={dd.glyph} gu={dd.gu} loai={dd.loai} size={34} />}
                overlay={
                  <>
                    {dauCon(dd) ? <View style={styles.badgeOnMedia}><Stamp label={dauCon(dd) as string} nen tilt={-2} tone="ai" /></View> : null}
                    {/* The heart lives on the picture's corner, as on the lead; no orphan row under the facts. */}
                    <View style={styles.timOnMedia}>
                      <IconButton
                        accessibilityLabel={luu ? `Bỏ lưu ${dd.name}` : `Lưu ${dd.name}`}
                        icon={luu ? "heart" : "heart-outline"}
                        onPress={() => onSave(dd.id)}
                        selected={luu}
                      />
                    </View>
                  </>
                }
                // The credit is the frame's own line now, printed under the
                // picture instead of after the facts: it was the one call site
                // that handed this frame a bare address (review 08/09, F31).
                nguon={dd.anh === null ? null : { loai: "danh-muc", anh: dd.anh }}
                ratio={4 / 3}
              />
              {chu}
            </Pressable>
          </View>
        );
      })}
    </View>
  );
}

const styles = StyleSheet.create({
  soSanh: { flexDirection: "row", gap: 16, paddingBottom: 12, borderBottomWidth: StyleSheet.hairlineWidth },
  soSanhDoc: { flexDirection: "column", gap: 20 },
  rowBadgeDuoi: { alignSelf: "flex-start", marginTop: 4 },
  ungVien: { flex: 1, minWidth: 0 },
  ungVienPress: { gap: 6 },
  ungVienDau: { flexDirection: "row", alignItems: "center", gap: 8, marginBottom: 4 },
  leadGon: { flexDirection: "row", alignItems: "flex-start", gap: 8, paddingBottom: 12, borderBottomWidth: StyleSheet.hairlineWidth },
  leadGonPress: { flex: 1, flexDirection: "row", alignItems: "flex-start", gap: 12, minWidth: 0 },
  leadGonChu: { paddingRight: 0 },
  leadGonDau: { alignSelf: "flex-start" },
  timOnMedia: { position: "absolute", right: 6, bottom: 6 },
  rowTen: { flexDirection: "row", alignItems: "center", gap: 8 },
  flex1: { flex: 1, minWidth: 0 },
  glyphTo: { alignItems: "center", justifyContent: "center", borderWidth: StyleSheet.hairlineWidth, flexShrink: 0 },
  lead: { gap: 4 },
  leadPress: { gap: 12 },
  leadText: { gap: 6, paddingRight: 56 },
  leadSave: { position: "absolute", right: 0, bottom: 0 },
  badgeOnMedia: { position: "absolute", left: 12, top: 12 },
  pressed: { opacity: 0.86 },
  row: { flexDirection: "row", alignItems: "center", gap: 8, paddingVertical: 10, borderBottomWidth: StyleSheet.hairlineWidth },
  rowPress: { flex: 1, flexDirection: "row", alignItems: "center", gap: 12, minHeight: 56 },
  thumb: { width: 56, height: 56, overflow: "hidden", flexShrink: 0 },
  rowText: { flex: 1, gap: 2, minWidth: 0 },
});
