import { Image } from "expo-image";
import { useEffect, useState, type ReactNode } from "react";
import { Pressable, StyleSheet, Text, View, type LayoutChangeEvent } from "react-native";

import { guTheoLoai } from "../../kham-pha/dia-diem";
import { typography, useRudiTheme } from "../../theme";
import { GuGlyph } from "../../ui/art/Gu";
import { CAU_ANH_HONG, khoaNguon, veKhung, type AnhCoGhiCong, type KhungDaVe } from "../../ui/ghi-cong";
import { giuState } from "../../../ui/a11y";

/**
 * One stop on the ink route: the hour on the left axis, a node on the line,
 * the stop on the right. The line is the plan itself -- solid ink for what
 * the group holds, a pencil dash for a draft nobody has confirmed -- so a
 * timeline reads as one evening in sequence rather than a stack of equal
 * cards. Every state is also a word (`phu`), never the stroke alone.
 */
export interface HangChangProps {
  gio: string;
  /**
   * Set inside a card in the chat: the hour starts flush with the card's text
   * instead of right-aligned in its column, so «19:00» sits under the title's
   * first letter (finish review B6). The plan screens keep the column.
   */
  sat?: boolean;
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
  /** Shared selection with the journey map: the node takes the accent. */
  chon?: boolean;
  /** Right-hand slot: a stamp, a button, a menu. */
  phai?: ReactNode;
  /**
   * The right-hand control's own inner left padding, when its words are its
   * only visible mark (a ghost text action): stacked under the stop, the words
   * then start on the stop's text column, not 14 dp right of it.
   */
  phaiLeChu?: number;
  /**
   * A line under the stop, in its text column and outside its pressable: an
   * offer that follows what was just done here («Thêm khoảnh khắc ở đây»),
   * which as a child of the row would be a button inside a button.
   */
  duoi?: ReactNode;
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
        <View accessible accessibilityLabel={`${ve.canhBao ?? CAU_ANH_HONG}: ${alt}`} style={[styles.anhChang, styles.anhChangVe, { borderRadius: radius.small - 2 }]}>
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

export function HangChang({ gio, sat = false, tieuDe, phu, phuTone = "inkSoft", ghiChu, daToi = false, phac = false, cuoi = false, onPress, accessibilityLabel, chon = false, phai, phaiLeChu = 0, duoi, anh = null, children }: HangChangProps) {
  const { colors } = useRudiTheme();
  // The picture's failure is the stop's state, not the thumbnail's: the frame
  // shows the drawn object, and the stop says why in words (a state is always
  // also a word). Reset when the picture changes.
  const [hong, setHong] = useState(false);
  const nguon = anh ? ({ loai: "danh-muc", anh: anh.anh } as const) : null;
  // Keyed on the picture, not on the object the itinerary rebuilds every
  // render, or the failure word is cleared on the next frame (F31 follow-up).
  const khoa = khoaNguon(nguon);
  useEffect(() => setHong(false), [khoa]);
  // One decision for the picture, the credit and the failure word, so the
  // thumbnail below and the lines here cannot disagree (F31).
  const ve = veKhung(nguon, { hong });
  const muc = phac ? colors.inkFaint : colors.lineStrong;
  // The right-hand control («Tôi đã tới») steps under the stop when beside it
  // the stop's name would get less than `COT_CHU_TOI_THIEU`: at 320 dp, with
  // the hour, the axis and the reorder handle, the name had 53 px and wrapped
  // over nine lines (QA UI-045). Both widths are measured, so a larger font or
  // a longer label moves the threshold with it.
  const [rongHang, setRongHang] = useState(0);
  const [rongPhai, setRongPhai] = useState(0);
  const coPhai = !anh && phai !== undefined && phai !== null;
  const xepDuoi = coPhai && rongHang > 0 && rongPhai > 0 && rongHang - CHO_TRAI - rongPhai < COT_CHU_TOI_THIEU;
  const doPhai = (e: LayoutChangeEvent) => {
    const w = Math.ceil(e.nativeEvent.layout.width);
    setRongPhai((cu) => (cu === w ? cu : w));
  };
  const duongTruc = (
    <View
      style={[
        styles.line,
        phac ? { borderLeftWidth: 2, borderColor: muc, borderStyle: "dashed", backgroundColor: "transparent" } : { backgroundColor: muc },
      ]}
    />
  );
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
    <View onLayout={(e) => setRongHang(Math.round(e.nativeEvent.layout.width))}>
    <View style={[styles.row, cuoi && !onPress && styles.rowCuoiTinh]}>
      <Text numberOfLines={1} style={[typography.label, styles.gio, sat && styles.gioSat, { color: colors.ink }]}>{gio}</Text>
      <View style={styles.axis}>
        <View
          style={[
            styles.node,
            { borderColor: phac ? colors.inkFaint : colors.ink, backgroundColor: daToi ? colors.split : colors.ground },
            daToi && { borderColor: colors.split },
            chon && { borderColor: colors.accent, backgroundColor: colors.accent },
          ]}
        />
        {cuoi && !xepDuoi ? null : duongTruc}
      </View>
      {onPress ? (
        <Pressable accessibilityLabel={accessibilityLabel ?? tieuDe} accessibilityRole="button" {...giuState(chon)} onPress={onPress} style={({ pressed }) => [styles.body, cuoi && styles.bodyCuoi, xepDuoi && styles.bodyTrenPhai, pressed && styles.pressed]}>
          {body}
        </Pressable>
      ) : (
        <View style={[styles.body, cuoi && styles.bodyCuoi, cuoi && styles.bodyCuoiTinh, xepDuoi && styles.bodyTrenPhai]}>{body}</View>
      )}
      {anh ? (
        <View style={styles.phai}>
          <AnhChang alt={anh.alt} loai={anh.loai} onHong={() => setHong(true)} ve={ve} />
        </View>
      ) : coPhai && !xepDuoi ? (
        <View onLayout={doPhai} style={styles.phai}>{phai}</View>
      ) : null}
    </View>
    {/* Stacked: the control on its own line under the stop, the ink line running on beside it. */}
    {xepDuoi ? (
      <View style={styles.row}>
        <View style={styles.gioTrong} />
        <View style={styles.axis}>{cuoi ? null : duongTruc}</View>
        <View onLayout={doPhai} style={[styles.phaiDuoi, cuoi && styles.bodyCuoi, phaiLeChu > 0 && { marginLeft: -phaiLeChu }]}>{phai}</View>
      </View>
    ) : null}
    {duoi ? (
      <View style={styles.row}>
        <View style={styles.gioTrong} />
        <View style={styles.axis}>{cuoi ? null : duongTruc}</View>
        <View style={[styles.duoi, cuoi && styles.bodyCuoi]}>{duoi}</View>
      </View>
    ) : null}
    </View>
  );
}

/** Hour column, axis and the three gaps of a row: what is left of it goes to the stop and the control. */
const CHO_TRAI = 46 + 14 + 10 * 3;
/** Narrowest the stop's name may get beside the control before the control steps under it. */
const COT_CHU_TOI_THIEU = 120;

const styles = StyleSheet.create({
  row: { flexDirection: "row", alignItems: "stretch", gap: 10, minHeight: 64 },
  rowCuoiTinh: { minHeight: 0 },
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
  // Last stop: no connector below it, so no room to leave for one. The
  // 48dp minimum stays -- `body` is the Pressable when `onPress` is given.
  bodyCuoi: { paddingBottom: 0 },
  // Only a row nobody can tap may drop the floor as well.
  bodyCuoiTinh: { minHeight: 0 },
  pressed: { opacity: 0.7 },
  phai: { justifyContent: "flex-start", paddingTop: 0, flexShrink: 0 },
  gioTrong: { minWidth: 46 },
  gioSat: { textAlign: "left" },
  // The stop above keeps less bottom room when its control sits under it.
  bodyTrenPhai: { paddingBottom: 6 },
  phaiDuoi: { alignSelf: "flex-start", paddingBottom: 18 },
  duoi: { flex: 1, minWidth: 0, paddingBottom: 18 },
});
