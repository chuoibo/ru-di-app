/**
 * A field written on the page, not a box to fill (ADR-0037 D1, plan S0.5).
 *
 * The QC of 24/09 read every creation screen as «a form»: a stack of bordered
 * boxes, each asking to be filled. On paper, you write on a ruled line. So
 * this field has no box: its words sit on the page (or on the receipt, the
 * ticket, the envelope it belongs to), and its one boundary is an ink rule
 * under them -- 1 dp of `lineStrong` at rest (a control's boundary, 3:1 on
 * every surface it lands on, `test_contrast_floor.py`), 2 dp of `accent`
 * while written in, 2 dp of `warn` when the value is wrong. The extra dp is
 * taken from the padding, so the words never move.
 *
 * On the web a focused input also draws the browser's focus ring: a blue box
 * around a field that has no box (QC 24/09, B7). The ring is turned off
 * because the rule already says «you are writing here» at 2 dp in the accent;
 * the focus stays visible, only its shape is the product's.
 *
 * The rest follows `Field`: the placeholder is drawn by the kit on one line
 * (Android wraps and clips a native hint, F44), the accessible name is the
 * label or the placeholder, an error replaces the helper line and is
 * announced politely. A multiline field rules its page: faint lines under
 * every line of text, the last one being the control's own rule.
 */
import { useState, type ReactNode } from "react";
import { Platform, StyleSheet, Text, TextInput, View, type StyleProp, type TextInputProps, type TextStyle, type ViewStyle } from "react-native";

import { typography, useRudiTheme } from "../theme";

export type ONhapMucProps = TextInputProps & {
  label?: string;
  /** A quiet line under the rule: format, limit, what the field is for. */
  helper?: string;
  /** What is wrong with the value; replaces `helper` and colours the rule. */
  error?: string | null;
  leading?: ReactNode;
  trailing?: ReactNode;
  /** `lon` writes the value in the heading size (a name on a cover, an amount on a receipt). */
  co?: "vua" | "lon";
  style?: StyleProp<TextStyle>;
  khungStyle?: StyleProp<ViewStyle>;
};

const DEM_DUOI = 8;
/** Past this many lines a multiline field scrolls. */
const DONG_TOI_DA = 8;

export interface KieuGach {
  hang: ViewStyle;
  nhap: TextStyle;
}

/**
 * The geometry of the rule, pure so a node test can hold it: `day` is the
 * rule's width (2 while written in or wrong, else 1), and the bottom padding
 * gives back what the rule takes, so the text never moves.
 */
export function kieuGach({ day = 1, multiline = false, numberOfLines, dongCao }: { day?: number; multiline?: boolean; numberOfLines?: number; dongCao: number }): KieuGach {
  const hang: ViewStyle = { flexDirection: "row", alignItems: multiline ? "flex-start" : "center", gap: 10, borderBottomWidth: day, paddingBottom: DEM_DUOI - (day - 1) };
  if (!multiline) return { hang, nhap: { flex: 1, minHeight: 44, paddingVertical: 0, paddingHorizontal: 0 } };
  const dong = Math.max(2, Math.min(DONG_TOI_DA, Math.floor(numberOfLines ?? 3)));
  return {
    hang,
    nhap: { flex: 1, minHeight: dong * dongCao, maxHeight: DONG_TOI_DA * dongCao, paddingVertical: 0, paddingHorizontal: 0, textAlignVertical: "top" },
  };
}

/** The browser's own focus box, off: the rule shows focus in the product's shape (B7). */
const KHONG_VIEN_WEB = (Platform.OS === "web" ? { outlineStyle: "none", outlineWidth: 0 } : {}) as TextStyle;

export function ONhapMuc({ label, helper, error, leading, trailing, co = "vua", multiline, numberOfLines, placeholder, style, khungStyle, ...inputProps }: ONhapMucProps) {
  const { colors } = useRudiTheme();
  const [goDuoc, setGoDuoc] = useState("");
  const [dangNhap, setDangNhap] = useState(false);
  const rong = (inputProps.value ?? goDuoc) === "";
  const coLoi = typeof error === "string" && error !== "";
  const chu = co === "lon" ? typography.h2 : typography.body;
  const kieu = kieuGach({ day: dangNhap || coLoi ? 2 : 1, multiline, numberOfLines, dongCao: chu.lineHeight });
  const mauGach = coLoi ? colors.warn : dangNhap ? colors.accent : colors.lineStrong;
  const placeholderNha = !multiline && placeholder !== undefined;
  const soDongKe = multiline ? Math.max(2, Math.min(DONG_TOI_DA, Math.floor(numberOfLines ?? 3))) - 1 : 0;
  return (
    <View style={[styles.khoi, khungStyle]}>
      {label ? <Text style={[typography.caption, { color: colors.inkSoft }]}>{label}</Text> : null}
      <View style={[kieu.hang, { borderBottomColor: mauGach }]}>
        {leading}
        <View style={styles.boc}>
          {soDongKe > 0 ? (
            // The ruled page under a long text: decorative lines at every line
            // height but the last, which is the control's own rule.
            <View importantForAccessibility="no-hide-descendants" pointerEvents="none" style={StyleSheet.absoluteFill}>
              {Array.from({ length: soDongKe }, (_, i) => (
                <View key={i} style={[styles.dongKe, { top: (i + 1) * chu.lineHeight - 1, backgroundColor: colors.line }]} />
              ))}
            </View>
          ) : null}
          <TextInput
            {...inputProps}
            accessibilityLabel={inputProps.accessibilityLabel ?? label ?? placeholder}
            aria-invalid={coLoi || undefined}
            multiline={multiline}
            numberOfLines={numberOfLines}
            onBlur={(e) => {
              setDangNhap(false);
              inputProps.onBlur?.(e);
            }}
            onChangeText={(t) => {
              setGoDuoc(t);
              inputProps.onChangeText?.(t);
            }}
            onFocus={(e) => {
              setDangNhap(true);
              inputProps.onFocus?.(e);
            }}
            placeholder={placeholderNha ? undefined : placeholder}
            placeholderTextColor={colors.inkFaint}
            style={[kieu.nhap, chu, { color: colors.ink }, KHONG_VIEN_WEB, style]}
          />
          {placeholderNha && rong ? (
            <View importantForAccessibility="no-hide-descendants" pointerEvents="none" style={styles.goiY}>
              <Text numberOfLines={1} style={[chu, { color: colors.inkFaint }]}>
                {placeholder}
              </Text>
            </View>
          ) : null}
        </View>
        {trailing}
      </View>
      {coLoi ? (
        <Text accessibilityLiveRegion="polite" style={[typography.caption, { color: colors.warn }]}>
          {error}
        </Text>
      ) : helper ? (
        <Text style={[typography.note, { color: colors.inkSoft }]}>{helper}</Text>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  khoi: { gap: 4 },
  boc: { flex: 1, alignSelf: "stretch", justifyContent: "center" },
  goiY: { position: "absolute", left: 0, right: 0, top: 0, bottom: 0, justifyContent: "center" },
  dongKe: { position: "absolute", left: 0, right: 0, height: StyleSheet.hairlineWidth },
});
