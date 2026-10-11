/**
 * A question asked in the row it is about, before something is taken away
 * (B11): «Xoá bình luận này? Không lấy lại được.», then the action in the
 * `warn` ink and «Thôi». No dialog, no toast: the answer is given where the
 * finger already is (DESIGN.md, «Trạng thái rỗng, tải, lỗi»).
 *
 * Five screens wrote their own copy of this with two type sizes and the
 * action sometimes outlined, sometimes ghost; this is the one shape.
 * When the question appears, focus moves to it (QA UI-101): on the web to
 * «Thôi», the answer that loses nothing; on a phone the screen reader reads
 * the question itself. Otherwise a keyboard or screen-reader user is left on
 * the button that raised it, with the question somewhere below.
 */
import { useEffect, useRef } from "react";
import { AccessibilityInfo, findNodeHandle, Platform, StyleSheet, Text, View } from "react-native";

import { typography, useRudiTheme } from "../theme";
import { RudiButton } from "../ui";
import { Text as NativeMetricText } from "./NativeFontMetrics";

export function HoiTaiHang({ cau, nhan, onDongY, onThoi, dangLam = false, testID, nativeMetrics = false }: {
  /** The question and what it costs, one or two sentences. */
  cau: string;
  /** The action, a verb: «Xoá», «Bỏ quyền», «Từ chối». */
  nhan: string;
  onDongY: () => void;
  onThoi: () => void;
  dangLam?: boolean;
  testID?: string;
  /** Use the surface owner's metrics; no observer is created here. */
  nativeMetrics?: boolean;
}) {
  const { colors } = useRudiTheme();
  const khoi = useRef<View>(null);
  const CopyText = nativeMetrics ? NativeMetricText : Text;
  useEffect(() => {
    const dich = khoi.current;
    if (dich === null) return;
    if (Platform.OS === "web") {
      const nut = (dich as unknown as HTMLElement).querySelectorAll?.<HTMLElement>('[role="button"],button');
      nut?.[nut.length - 1]?.focus();
      return;
    }
    const node = findNodeHandle(dich);
    if (node !== null) AccessibilityInfo.setAccessibilityFocus(node);
  }, []);
  return (
    <View accessibilityLiveRegion="polite" ref={khoi} style={styles.khoi} testID={testID}>
      <CopyText style={[typography.note, { color: colors.ink }]}>{cau}</CopyText>
      <View style={styles.nut}>
        <RudiButton compact full={false} label={nhan} loading={dangLam} onPress={onDongY} tone="warn" variant="outline" />
        <RudiButton compact full={false} label="Thôi" onPress={onThoi} variant="ghost" />
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  khoi: { gap: 6 },
  nut: { flexDirection: "row", flexWrap: "wrap", alignItems: "center", gap: 8 },
});
