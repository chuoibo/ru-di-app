import { Image } from "expo-image";
import { useEffect, useState, type ReactNode } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";

import { guTheoLoai } from "../../kham-pha/dia-diem";
import { typography, useRudiTheme } from "../../theme";
import { GuGlyph } from "../../ui/art/Gu";
import { veKhung, type AnhCoGhiCong, type KhungDaVe } from "../../ui/ghi-cong";

/**
 * One stop on the ink route: the hour on the left axis, a node on the line,
 * the stop on the right. The line is the plan itself -- solid ink for what
 * the group holds, a pencil dash for a draft nobody has confirmed -- so a
 * timeline reads as one evening in sequence rather than a stack of equal
 * cards. Every state is also a word (`phu`), never the stroke alone.
 */
export interface HangChangProps {
  gio: string;
  tieuDe: string;
  /** The line under the title: the place, or what is still missing. */
  phu?: string | null;
  phuTone?: "ink" | "inkSoft" | "inkFaint" | "accent";
  /** A third, quieter line: who has arrived, a note. */
  ghiChu?: string | null;
  /** Filled node: the group has been here, or this stop is the one now. */
  daToi?: boolean;
  /** Pencil dash instead of ink: a draft or an AI proposal. */
  phac?: boolean;
  /** The last stop draws no line below its node. */
  cuoi?: boolean;
  onPress?: () => void;
  accessibilityLabel?: string;
  /** Right-hand slot: a stamp, a button, a menu. */
  phai?: ReactNode;
  /**
   * The photograph beside a main stop, with the credit it may be shown under.
   * The row draws both -- the 44dp thumbnail at the right and the credit as
   * the stop's last line -- so a screen cannot pass the picture and forget the
   * words (review 08/09 vòng 2, F21: the vote and the itinerary did exactly
   * that). A picture that fails to load gives way to the category's object in
   * the same frame; `loai` names that category.
   */
  anh?: { anh: AnhCoGhiCong; alt: string; loai?: string } | null;
  children?: ReactNode;
}

/**
 * The photograph beside a main stop of the route. A stop with a place the
 * group has a picture of reads as a destination; a stop without one stays a
 * compact line, and that difference is the timeline's rhythm (report §5.2 B).
 * Not exported: the only way to put a picture on a stop is `HangChang.anh`,
 * which carries the credit with it.
 */
function AnhChang({ ve, alt, loai, onHong }: { ve: KhungDaVe; alt: string; loai?: string; onHong: () => void }) {
  const { colors, radius } = useRudiTheme();
  return (
    <View style={[styles.khungAnhChang, { backgroundColor: colors.card, borderColor: colors.line, borderRadius: radius.small }]}>
      {ve.source === null ? (
        <View accessible accessibilityLabel={`${ve.canhBao ?? "Chưa tải được ảnh"}: ${alt}`} style={[styles.anhChang, styles.anhChangVe, { borderRadius: radius.small - 2 }]}>
          <GuGlyph id={guTheoLoai(loai ?? "")} size={28} tone="accent" />
        </View>
      ) : (
        <Image
          accessibilityLabel={alt}
          contentFit="cover"
          onError={onHong}
          source={ve.source}
          style={[styles.anhChang, { borderRadius: radius.small - 2, backgroundColor: colors.line }]}
        />
      )}
    </View>
  );
}

export function HangChang({ gio, tieuDe, phu, phuTone = "inkSoft", ghiChu, daToi = false, phac = false, cuoi = false, onPress, accessibilityLabel, phai, anh = null, children }: HangChangProps) {
  const { colors } = useRudiTheme();
  // The picture's failure is the stop's state, not the thumbnail's: the frame
  // shows the drawn object, and the stop says why in words (a state is always
  // also a word). Reset when the picture changes.
  const [hong, setHong] = useState(false);
  const nguonAnh = anh?.anh ?? null;
  useEffect(() => setHong(false), [nguonAnh]);
  // One decision for the picture, the credit and the failure word, so the
  // thumbnail below and the lines here cannot disagree (F31).
  const ve = veKhung(anh === null ? null : { loai: "danh-muc", anh: anh.anh }, { hong });
  const muc = phac ? colors.inkFaint : colors.lineStrong;
  const body = (
    <>
      <Text style={[typography.label, { color: colors.ink }]}>{tieuDe}</Text>
      {phu ? <Text style={[typography.caption, { color: colors[phuTone] }]}>{phu}</Text> : null}
      {ghiChu ? <Text style={[typography.caption, { color: colors.inkSoft }]}>{ghiChu}</Text> : null}
      {children}
      {/* The credit is a line of the stop, beside the thumbnail it qualifies,
          so it scrolls with the picture and never ends up a screen away from
          it (ADR-0017 §2.5). No line cap: the column beside the hour and the
          thumbnail is narrow, and at font 1.3 a long author name has to wrap
          rather than end in an ellipsis; the stop simply grows. */}
      {ve.canhBao !== null ? <Text style={[typography.caption, { color: colors.warn }]}>{ve.canhBao}</Text> : null}
      {ve.ghiCong !== null ? <Text style={[typography.caption, { color: colors.inkFaint }]}>{ve.ghiCong}</Text> : null}
    </>
  );
  return (
    <View style={styles.row}>
      <Text numberOfLines={1} style={[typography.label, styles.gio, { color: colors.ink }]}>{gio}</Text>
      <View style={styles.axis}>
        <View
          style={[
            styles.node,
            { borderColor: phac ? colors.inkFaint : colors.ink, backgroundColor: daToi ? colors.split : colors.ground },
            daToi && { borderColor: colors.split },
          ]}
        />
        {cuoi ? null : (
          <View
            style={[
              styles.line,
              phac ? { borderLeftWidth: 2, borderColor: muc, borderStyle: "dashed", backgroundColor: "transparent" } : { backgroundColor: muc },
            ]}
          />
        )}
      </View>
      {onPress ? (
        <Pressable accessibilityLabel={accessibilityLabel ?? tieuDe} accessibilityRole="button" onPress={onPress} style={({ pressed }) => [styles.body, pressed && styles.pressed]}>
          {body}
        </Pressable>
      ) : (
        <View style={styles.body}>{body}</View>
      )}
      {anh ? (
        <View style={styles.phai}>
          <AnhChang alt={anh.alt} loai={anh.loai} onHong={() => setHong(true)} ve={ve} />
        </View>
      ) : phai ? (
        <View style={styles.phai}>{phai}</View>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  row: { flexDirection: "row", alignItems: "stretch", gap: 10, minHeight: 64 },
  // Intrinsic width: every hour is five tabular digits, so the column lines up
  // without a fixed width, and at font 1.3 «07:00» stays on one line.
  gio: { minWidth: 46, flexShrink: 0, textAlign: "right", paddingTop: 2, fontVariant: ["tabular-nums"] },
  khungAnhChang: { padding: 2, borderWidth: StyleSheet.hairlineWidth },
  anhChang: { width: 44, height: 44 },
  anhChangVe: { alignItems: "center", justifyContent: "center" },
  axis: { width: 14, alignItems: "center" },
  node: { width: 12, height: 12, borderRadius: 6, borderWidth: 2, marginTop: 4 },
  line: { flex: 1, width: 2, marginTop: 4, marginBottom: -4, minHeight: 20 },
  body: { flex: 1, gap: 2, paddingBottom: 18, minHeight: 48 },
  pressed: { opacity: 0.7 },
  phai: { justifyContent: "flex-start", paddingTop: 0, flexShrink: 0 },
});
