import { StyleSheet, Text, View } from "react-native";

import { typography, useRudiTheme } from "../../theme";
import { Heading, RudiButton, Segmented } from "../../ui";
import { Sheet } from "../../ui/Sheet";

/**
 * «Loại sổ»: two choices side by side, no default winner (spec §14.2): «Hai
 * người bạn» and «Một đôi». «Người nhà» waits for its own slice. Choosing
 * «Một đôi» PROPOSES tier 3; it is on only when the other agrees. Choosing
 * «Hai người bạn» while a pair is on withdraws it, on the spot, from my side.
 */
export function LoaiSo({ open, onClose, batDoi, dangCho, onChonDoi, onChonBan, nguoiKiaDongY, testID }: { open: boolean; onClose: () => void; batDoi: boolean; dangCho: boolean; onChonDoi: () => void; onChonBan: () => void; nguoiKiaDongY: (() => void) | null; testID?: string }) {
  const { colors, space } = useRudiTheme();
  const chon = batDoi || dangCho ? 1 : 0;
  return (
    <Sheet accessibilityLabel="Loại sổ" onClose={onClose} open={open} testID={testID ?? "loai-so"}>
      <View style={[styles.noiDung, { gap: space.md }]}>
        <Heading size="h2" subtitle="Đổi được bất cứ lúc nào. Tờ giấy đã gửi không đổi theo." title="Sổ này là sổ gì?" />
        <Segmented items={["Hai người bạn", "Một đôi"]} onSelect={(i) => (i === 1 ? onChonDoi() : onChonBan())} selected={chon} testIDs={["loai-so-ban", "loai-so-doi"]} />
        <Text style={[typography.caption, { color: colors.inkSoft }]}>
          {batDoi
            ? "Đang là một đôi. Nếp nói chuyện với hai bạn như với một đôi."
            : dangCho
              ? "Đã đề nghị «Một đôi». Chờ người ấy đồng ý trên máy của người ấy."
              : "Hai người bạn: truyền giấy, hai ô ràng buộc, không gì hơn."}
        </Text>
        {dangCho && nguoiKiaDongY ? <RudiButton label="(Bản trải nghiệm) Người kia đồng ý" onPress={nguoiKiaDongY} variant="outline" /> : null}
        <RudiButton label="Xong" onPress={onClose} variant="ghost" />
      </View>
    </Sheet>
  );
}

const styles = StyleSheet.create({ noiDung: { paddingBottom: 8 } });
