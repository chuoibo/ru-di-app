import { Ionicons } from "@expo/vector-icons";
import { Image, type ImageSource } from "expo-image";
import { Pressable, StyleSheet, Text, View } from "react-native";

import { typography, useRudiTheme } from "../../theme";
import { IconButton, Inline, type IconName } from "../../ui";
import { MediaSlot, cauGhiCong, type Attribution } from "../../ui/MediaSlot";
import { Stamp } from "../../ui/Stamp";
import { useAdaptiveLayout } from "../../ui/useAdaptiveLayout";
import { GuGlyph } from "../../ui/art/Gu";
import { guTheoLoai } from "../../kham-pha/dia-diem";

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
  photo: ImageSource | null;
  attribution?: Attribution;
  /** «Rất hợp gu» from a computed match; null otherwise. */
  badge: string | null;
}

interface CommonProps {
  dd: DiaDiemHienThi;
  daLuu: boolean;
  onOpen: () => void;
  onSave: () => void;
  testID?: string;
}

/** The empty frame's artwork: the category drawn with one pen, never a stock photo. */
export function PlaceGlyph({ glyph, loai, size = 40 }: { glyph: IconName; loai?: string; size?: number }) {
  const { colors } = useRudiTheme();
  return (
    <View style={[styles.glyphDisc, { width: size * 1.7, height: size * 1.7, borderRadius: size * 0.85, backgroundColor: colors.accentSoft }]}>
      {loai === undefined ? <Ionicons color={colors.accent} name={glyph} size={size} /> : <GuGlyph id={guTheoLoai(loai)} size={size * 1.15} tone="accent" />}
    </View>
  );
}

export function PlaceLead({ dd, daLuu, onOpen, onSave, testID }: CommonProps) {
  const { colors } = useRudiTheme();
  const { sizeClass } = useAdaptiveLayout();
  // 16:10 fills a phone's width at reading height; on a tablet the same ratio
  // is a screenful of photograph before the first name, so the frame widens.
  const tiLe = sizeClass === "compact" ? 16 / 10 : 21 / 9;
  return (
    <View style={styles.lead} testID={testID}>
      <Pressable accessibilityLabel={`Mở ${dd.name}`} accessibilityRole="button" onPress={onOpen} style={({ pressed }) => [styles.leadPress, pressed && styles.pressed]}>
        <MediaSlot
          alt={dd.name}
          attribution={dd.attribution}
          fallback={<PlaceGlyph glyph={dd.glyph} loai={dd.loai} size={44} />}
          overlay={dd.badge ? <View style={styles.badgeOnMedia}><Stamp label={dd.badge} nen tilt={-2} tone="ai" /></View> : null}
          ratio={tiLe}
          source={dd.photo}
        />
        <View style={styles.leadText}>
          <Text style={[typography.h2, { color: colors.ink }]}>{dd.name}</Text>
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
        </View>
      </Pressable>
      <View style={styles.leadSave}>
        <IconButton
          accessibilityLabel={daLuu ? `Bỏ lưu ${dd.name}` : `Lưu ${dd.name}`}
          icon={daLuu ? "heart" : "heart-outline"}
          onPress={onSave}
          selected={daLuu}
        />
      </View>
    </View>
  );
}

export function PlaceRow({ dd, daLuu, onOpen, onSave, testID }: CommonProps) {
  const { colors, radius } = useRudiTheme();
  const facts = dd.facts.map((f) => f.text).join(" · ");
  return (
    <View style={[styles.row, { borderBottomColor: colors.line }]} testID={testID}>
      <Pressable accessibilityLabel={`Mở ${dd.name}`} accessibilityRole="button" onPress={onOpen} style={({ pressed }) => [styles.rowPress, pressed && styles.pressed]}>
        <View style={[styles.thumb, { borderRadius: radius.small, backgroundColor: colors.accentSoft }]}>
          {dd.photo ? (
            <Image accessibilityLabel={dd.name} contentFit="cover" source={dd.photo} style={StyleSheet.absoluteFill} />
          ) : dd.loai !== undefined ? (
            <GuGlyph id={guTheoLoai(dd.loai)} size={32} tone="accent" />
          ) : (
            <Ionicons color={colors.accent} name={dd.glyph} size={24} />
          )}
        </View>
        <View style={styles.rowText}>
          <Text numberOfLines={2} style={[typography.title, { color: colors.ink }]}>{dd.name}</Text>
          {dd.sub ? <Text numberOfLines={1} style={[typography.caption, { color: colors.inkSoft }]}>{dd.sub}</Text> : null}
          {/* One text node: a row of several short texts keeps its first
              measurement when the row wraps and strands one word alone. */}
          {facts ? <Text numberOfLines={1} style={[typography.caption, { color: colors.inkFaint }]}>{facts}</Text> : null}
          {/* The thumbnail is a licensed photograph, so its credit is a line
              of this row (ADR-0017 §2.5) -- two lines, since a long author
              name has to wrap rather than end in an ellipsis. */}
          {dd.photo && dd.attribution ? (
            <Text numberOfLines={2} style={[typography.caption, { color: colors.inkFaint }]}>{cauGhiCong(dd.attribution)}</Text>
          ) : null}
          {dd.badge ? <Stamp label={dd.badge} style={styles.rowBadge} tone="ai" /> : null}
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
  return (
    <View style={[styles.soSanh, { borderBottomColor: colors.line }]} testID={testID}>
      {items.map((dd) => {
        const luu = daLuu(dd.id);
        const fact = dd.facts[0]?.text;
        return (
          <View key={dd.id} style={styles.ungVien}>
            <Pressable accessibilityLabel={`Mở ${dd.name}`} accessibilityRole="button" onPress={() => onOpen(dd.id)} style={({ pressed }) => [styles.ungVienPress, pressed && styles.pressed]}>
              <MediaSlot
                alt={dd.name}
                fallback={<PlaceGlyph glyph={dd.glyph} loai={dd.loai} size={34} />}
                overlay={dd.badge ? <View style={styles.badgeOnMedia}><Stamp label={dd.badge} nen tilt={-2} tone="ai" /></View> : null}
                ratio={4 / 3}
                source={dd.photo}
              />
              <Text numberOfLines={2} style={[typography.title, { color: colors.ink }]}>{dd.name}</Text>
              {dd.sub ? <Text numberOfLines={2} style={[typography.note, { color: colors.inkSoft }]}>{dd.sub}</Text> : null}
              {fact ? <Text numberOfLines={1} style={[typography.note, { color: colors.inkFaint }]}>{fact}</Text> : null}
              {dd.photo && dd.attribution ? (
                <Text numberOfLines={2} style={[typography.note, { color: colors.inkFaint }]}>{cauGhiCong(dd.attribution)}</Text>
              ) : null}
            </Pressable>
            <IconButton
              accessibilityLabel={luu ? `Bỏ lưu ${dd.name}` : `Lưu ${dd.name}`}
              icon={luu ? "heart" : "heart-outline"}
              onPress={() => onSave(dd.id)}
              quiet
              selected={luu}
            />
          </View>
        );
      })}
    </View>
  );
}

const styles = StyleSheet.create({
  soSanh: { flexDirection: "row", gap: 16, paddingBottom: 12, borderBottomWidth: StyleSheet.hairlineWidth },
  ungVien: { flex: 1, minWidth: 0, gap: 4 },
  ungVienPress: { gap: 6 },
  glyphDisc: { alignItems: "center", justifyContent: "center" },
  lead: { gap: 4 },
  leadPress: { gap: 12 },
  leadText: { gap: 6, paddingRight: 56 },
  leadSave: { position: "absolute", right: 0, bottom: 0 },
  badgeOnMedia: { position: "absolute", left: 12, top: 12 },
  pressed: { opacity: 0.86 },
  row: { flexDirection: "row", alignItems: "center", gap: 8, paddingVertical: 10, borderBottomWidth: StyleSheet.hairlineWidth },
  rowPress: { flex: 1, flexDirection: "row", alignItems: "center", gap: 12, minHeight: 56 },
  thumb: { width: 56, height: 56, overflow: "hidden", alignItems: "center", justifyContent: "center", flexShrink: 0 },
  rowText: { flex: 1, gap: 2, minWidth: 0 },
  rowBadge: { marginTop: 4 },
});
