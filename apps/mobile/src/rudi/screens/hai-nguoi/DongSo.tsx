import { StyleSheet, Text, View } from "react-native";

import { typography, useRudiTheme } from "../../theme";
import { Heading, RudiButton } from "../../ui";
import { Sheet } from "../../ui/Sheet";

/**
 * «Đóng sổ» with its preview (spec §7.6, Lead's decision in §20.1): before
 * the button, the count of what closing will do, in bullets a person can
 * check against what they see. Destructive action as an `outline` button and
 * the way out as `ghost`, the idiom `CaiDatNhom` uses for leaving a group; no
 * `Alert`. Closing does not bring anything back (§3.3 rule 7).
 */
export function DongSo({ open, onClose, xemTruoc, onDong, testID }: { open: boolean; onClose: () => void; xemTruoc: { so_nhap_bo: number; so_to_huy: number; so_to_khoa: number; so_de_nghi_huy: number }; onDong: () => void; testID?: string }) {
  const { colors, space } = useRudiTheme();
  return (
    <Sheet accessibilityLabel="Đóng sổ hai người" onClose={onClose} open={open} testID={testID ?? "dong-so"}>
      <View style={[styles.noiDung, { gap: space.md }]}>
        <Heading size="h2" subtitle="Đóng là đóng. Tờ chưa mở thì thôi; không gì sống lại." title="Đóng sổ hai người?" />
        <View style={styles.khoi} testID="dong-so-xem-truoc">
          {xemTruoc.so_nhap_bo > 0 ? (
            <Text style={[typography.body, { color: colors.ink }]}>· {xemTruoc.so_nhap_bo} bản phác chỉ bạn thấy sẽ bỏ.</Text>
          ) : null}
          <Text style={[typography.body, { color: colors.ink }]}>
            {xemTruoc.so_to_huy === 0 ? "· Không có tờ nào đang chờ trả lời." : `· ${xemTruoc.so_to_huy} tờ đang chờ trả lời sẽ huỷ.`}
          </Text>
          <Text style={[typography.body, { color: colors.ink }]}>
            {xemTruoc.so_to_khoa === 0 ? "· Không có buổi đã chốt nào." : `· ${xemTruoc.so_to_khoa} buổi đã chốt sẽ khoá, chỉ còn đọc.`}
          </Text>
          <Text style={[typography.body, { color: colors.ink }]}>
            {xemTruoc.so_de_nghi_huy === 0 ? "· Không có lời đề nghị nào đang chờ." : `· ${xemTruoc.so_de_nghi_huy} lời đề nghị đang chờ sẽ huỷ.`}
          </Text>
          <Text style={[typography.body, { color: colors.ink }]}>· Ký ức đã giữ vẫn đọc được, không sửa được.</Text>
        </View>
        <RudiButton label="Đóng sổ" onPress={onDong} variant="outline" />
        <RudiButton label="Giữ sổ" onPress={onClose} variant="ghost" />
      </View>
    </Sheet>
  );
}

const styles = StyleSheet.create({ noiDung: { paddingBottom: 8 }, khoi: { gap: 6 } });
