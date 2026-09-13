import { StyleSheet, Text, View } from "react-native";

import { typography, useRudiTheme } from "../../theme";
import { Heading, RudiButton } from "../../ui";
import { Sheet } from "../../ui/Sheet";

/**
 * «Việc này làm gì, trước khi nó làm.»
 *
 * Bốn việc trên tờ giấy không lấy lại được — bỏ bản phác, rút lại, nghỉ tuần,
 * huỷ buổi đã chốt — và bản đầu nối thẳng chúng vào một dòng chữ coral dưới
 * tờ giấy: một cú chạm, xong. «Huỷ buổi này» xoá một buổi người kia ĐÃ đồng ý
 * và đang trông; nó hiện trên máy của họ như một buổi biến mất.
 *
 * Luật của chính bản dựng này: việc không lấy lại được phải nói ra nó làm gì
 * TRƯỚC khi làm. `DongSo` đã có khuôn ấy — đếm trước, rồi mới đóng. Tờ này là
 * cùng khuôn cho bốn việc còn lại, và nó nhắc tên người kia vì hậu quả rơi lên
 * họ chứ không chỉ lên người đang bấm.
 */
export function XacNhanViec({
  open,
  onClose,
  onXacNhan,
  tieuDe,
  hauQua,
  nhanLam,
  testID,
}: {
  open: boolean;
  onClose: () => void;
  onXacNhan: () => void;
  tieuDe: string;
  /** Một câu nói việc này làm gì, cho ai. Không phải «bạn có chắc không?». */
  hauQua: string;
  nhanLam: string;
  testID?: string;
}) {
  const { colors, space } = useRudiTheme();
  return (
    <Sheet accessibilityLabel={tieuDe} onClose={onClose} open={open} testID={testID ?? "xac-nhan-viec"}>
      <View style={[styles.noiDung, { gap: space.md }]}>
        <Heading size="h2" title={tieuDe} />
        <Text style={[typography.body, { color: colors.ink }]} testID={`${testID ?? "xac-nhan-viec"}-hau-qua`}>
          {hauQua}
        </Text>
        <RudiButton label={nhanLam} onPress={onXacNhan} variant="outline" />
        <RudiButton label="Để đấy" onPress={onClose} variant="ghost" />
      </View>
    </Sheet>
  );
}

const styles = StyleSheet.create({ noiDung: { paddingBottom: 8 } });
