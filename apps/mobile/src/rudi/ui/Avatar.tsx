import { Image, type ImageSource } from "expo-image";
import { useState } from "react";
import { StyleSheet, Text, View, type StyleProp, type ViewStyle } from "react-native";

import { chuDau } from "../../screens/ca-nhan/ban-be";
import { MOTION_MS } from "../motion";
import { anhDaHong, danhDauAnhHong } from "../nguoi/anh-dai-dien-cache";
import { nguonAnhDaiDien } from "../nguoi/anh-ca-nhan";
import { mucNguoi, typography, useRudiTheme, type RudiTone } from "../theme";

export interface AvatarProps {
  name: string;
  /** Resolved by the screen (`nguonAnh` for group photos, the avatar helper for people); null = initials. */
  source?: ImageSource | null;
  size?: number;
  /** A ring marks the person who is speaking, paying, or being pointed at. */
  ring?: boolean;
  tone?: RudiTone;
  /** The frame could not load its photograph; the caller drops back to initials. */
  onError?: () => void;
  /**
   * Who this is (ADR-0037 D6): the ring and the initial take the person's own
   * ink, the same on every screen. Without it the avatar keeps the screen's
   * tone, as before.
   */
  personId?: string | null;
  /**
   * Fetch the person's own avatar as `actorId` (the viewer) when no `source`
   * is given; a 404 is remembered for the session and the initial is drawn
   * straight away after that (`anh-dai-dien-cache.ts`).
   */
  anh?: { actorId: string; lan?: number } | null;
  style?: StyleProp<ViewStyle>;
  testID?: string;
}

/**
 * A person, at any size. Photo when the person uploaded one (M8), initials
 * otherwise -- never a stock face, never a photo of a real person that did not
 * come from them. With `personId` (ADR-0037 D6, which replaced the old «colour
 * is not per person» rule) the ring and the initial are the person's own ink:
 * eight inks, each 30 degrees of hue away from the three meaning tones, so a
 * group of eight stops reading as eight identical coral circles. Without it,
 * the tint is still the screen's tone.
 */
export function Avatar({ name, source = null, size = 44, ring = false, tone = "accent", onError, personId, anh = null, style, testID }: AvatarProps) {
  const { colors, dark } = useRudiTheme();
  const [hong, setHong] = useState(false);
  const mucRieng = personId ? mucNguoi(personId, dark) : null;
  const soft = mucRieng ? colors.card : tone === "accent" ? colors.accentSoft : tone === "split" ? colors.splitSoft : colors.aiSoft;
  const ink = mucRieng ?? colors[tone];
  const nguon = source ?? (personId && anh && !hong && !anhDaHong(personId) ? nguonAnhDaiDien(personId, anh.actorId, anh.lan ?? 0) : null);
  const frame: ViewStyle = {
    width: size,
    height: size,
    borderRadius: size / 2,
    backgroundColor: soft,
    borderWidth: mucRieng ? (ring ? 3 : 2) : 2,
    borderColor: mucRieng ? ink : ring ? ink : colors.card,
  };
  return (
    <View accessibilityLabel={name} testID={testID} style={[styles.center, frame, style]}>
      {nguon ? (
        <Image
          source={nguon}
          onError={() => {
            if (!source && personId) {
              danhDauAnhHong(personId);
              setHong(true);
            }
            onError?.();
          }}
          contentFit="cover"
          transition={MOTION_MS.standard}
          style={[StyleSheet.absoluteFill, { borderRadius: size / 2 }]}
        />
      ) : (
        <Text style={[typography.label, { color: ink, fontSize: Math.max(11, Math.round(size * 0.38)), lineHeight: undefined }]}>
          {chuDau(name)}
        </Text>
      )}
    </View>
  );
}

/**
 * A person as a paper standee (ADR-0037 D1, D6): the avatar as a head on a
 * cut-paper body in the person's ink, standing on a small base -- how people
 * appear around the bill table and in scenes. The name is the label.
 */
export function HinhNhan({ name, personId, size = 44, anh = null, ring = false, style, testID }: { name: string; personId: string; size?: number; anh?: AvatarProps["anh"]; ring?: boolean; style?: StyleProp<ViewStyle>; testID?: string }) {
  const { colors, dark } = useRudiTheme();
  const muc = mucNguoi(personId, dark);
  return (
    <View accessibilityLabel={name} accessible style={[styles.nhan, { width: size * 1.3 }, style]} testID={testID}>
      <Avatar anh={anh} name={name} personId={personId} ring={ring} size={size} style={styles.dauNhan} />
      <View style={[styles.than, { width: size * 0.9, height: size * 0.55, borderTopLeftRadius: size * 0.45, borderTopRightRadius: size * 0.45, borderColor: muc, backgroundColor: colors.card, marginTop: -size * 0.12 }]} />
      <View style={[styles.de, { width: size * 1.2, backgroundColor: colors.paperShade }]} />
    </View>
  );
}

export interface AvatarStackProps {
  people: { name: string; source?: ImageSource | null }[];
  size?: number;
  max?: number;
  tone?: RudiTone;
  style?: StyleProp<ViewStyle>;
}

/** Overlapping heads with a «+N» tail; the count is the honest number, not a mood. */
export function AvatarStack({ people, size = 32, max = 4, tone = "accent", style }: AvatarStackProps) {
  const { colors } = useRudiTheme();
  const shown = people.slice(0, max);
  const rest = people.length - shown.length;
  return (
    <View accessibilityLabel={`${people.length} người`} style={[styles.row, style]}>
      {shown.map((p, i) => (
        <Avatar key={`${p.name}-${i}`} name={p.name} source={p.source} size={size} tone={tone} style={i > 0 ? { marginLeft: -size * 0.3 } : undefined} />
      ))}
      {rest > 0 ? (
        <View
          style={[
            styles.center,
            { width: size, height: size, borderRadius: size / 2, marginLeft: -size * 0.3, backgroundColor: colors.card, borderWidth: 2, borderColor: colors.line },
          ]}
        >
          <Text style={[typography.caption, { color: colors.inkSoft }]}>+{rest}</Text>
        </View>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  center: { alignItems: "center", justifyContent: "center", overflow: "hidden" },
  row: { flexDirection: "row", alignItems: "center" },
  nhan: { alignItems: "center" },
  dauNhan: { zIndex: 1 },
  than: { borderWidth: 2, borderBottomWidth: 0 },
  de: { height: 4, borderRadius: 2 },
});
