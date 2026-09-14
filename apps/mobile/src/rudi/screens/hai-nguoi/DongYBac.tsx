import { StyleSheet, Text, View } from "react-native";

import { typography, useRudiTheme } from "../../theme";
import { Heading, RudiButton } from "../../ui";
import { Sheet } from "../../ui/Sheet";

/**
 * The consent ladder's two rungs a person can climb here (spec §6.1): tier 2
 * «Lập sổ» and tier 3 «Một đôi». Each sheet says in plain words what the
 * rung allows and, in the same breath, what it does NOT pull along -- a lower
 * rung never implies a higher one, and tier 4 (Nếp reads the chat) stays off
 * until slice 2 adds its own switch.
 *
 * Both are proposals: nothing changes until the other person agrees on their
 * own phone. The fixture build offers «(Bản trải nghiệm) Người kia đồng ý» so
 * one phone can play both sides; it exists only when `nguoiKia` is not null,
 * which is only a development build with the fixture door open.
 */
function BacDongY({
  open,
  onClose,
  tieuDe,
  choPhep,
  khongKeoTheo,
  dangCho,
  deNghiCuaToi,
  onDeNghi,
  onDongY,
  nhanDeNghi,
  nguoiKiaDongY,
  testID,
}: {
  open: boolean;
  onClose: () => void;
  tieuDe: string;
  choPhep: readonly string[];
  khongKeoTheo: readonly string[];
  dangCho: boolean;
  /**
   * Lời đề nghị đang chờ là của TÔI hay của người kia.
   *
   * Không phải chi tiết: người được đề nghị là người duy nhất bấm đồng ý được.
   * Bản đầu hiện cùng một câu «chờ người ấy đồng ý» cho cả hai phía, nên trên
   * máy thật người NHẬN lời đề nghị nhìn thấy một màn không có việc gì để làm
   * (đo ở vòng native 14/09). `true` khi không có lời đề nghị nào.
   */
  deNghiCuaToi: boolean;
  onDeNghi: () => void;
  /** Đồng ý lời đề nghị của người kia. Chỉ gọi khi `dangCho && !deNghiCuaToi`. */
  onDongY: () => void;
  nhanDeNghi: string;
  nguoiKiaDongY: (() => void) | null;
  testID: string;
}) {
  const { colors, space } = useRudiTheme();
  return (
    <Sheet accessibilityLabel={tieuDe} onClose={onClose} open={open} testID={testID}>
      <View style={[styles.noiDung, { gap: space.md }]}>
        <Heading size="h2" title={tieuDe} />
        <View style={styles.khoi}>
          <Text style={[typography.label, { color: colors.ink }]}>Cho phép</Text>
          {choPhep.map((d) => (
            <Text key={d} style={[typography.body, { color: colors.ink }]}>· {d}</Text>
          ))}
        </View>
        <View style={styles.khoi}>
          <Text style={[typography.label, { color: colors.ink }]}>Không kéo theo</Text>
          {khongKeoTheo.map((d) => (
            <Text key={d} style={[typography.body, { color: colors.inkSoft }]}>· {d}</Text>
          ))}
        </View>
        {dangCho && !deNghiCuaToi ? (
          // Người ấy đề nghị: việc của màn này là một nút, không phải một câu
          // nói rằng đang chờ chính mình.
          <>
            <Text style={[typography.body, { color: colors.ink }]} testID={`${testID}-ho-de-nghi`}>
              Người ấy đã đề nghị. Bạn đồng ý thì sổ mở.
            </Text>
            <RudiButton label="Đồng ý" onPress={onDongY} />
          </>
        ) : dangCho ? (
          <Text style={[typography.caption, { color: colors.inkSoft }]} testID={`${testID}-dang-cho`}>
            Đã đề nghị. Chờ người ấy đồng ý trên máy của người ấy; im lặng không phải đồng ý.
          </Text>
        ) : (
          <RudiButton label={nhanDeNghi} onPress={onDeNghi} />
        )}
        {dangCho && nguoiKiaDongY ? (
          <RudiButton label="(Bản trải nghiệm) Người kia đồng ý" onPress={nguoiKiaDongY} variant="outline" />
        ) : null}
        <RudiButton label="Để sau" onPress={onClose} variant="ghost" />
      </View>
    </Sheet>
  );
}

export function LapSo(props: { open: boolean; onClose: () => void; dangCho: boolean; deNghiCuaToi: boolean; onDeNghi: () => void; onDongY: () => void; nguoiKiaDongY: (() => void) | null }) {
  return (
    <BacDongY
      {...props}
      choPhep={["Một chỗ hai bạn truyền giấy cho nhau mỗi tuần.", "Hai ô ràng buộc: «Không ăn được» và «Đừng».", "Nếp phác một tờ khi tới lượt, bạn sửa rồi gửi."]}
      khongKeoTheo={["Không tự thành «Một đôi».", "Nếp không đọc tin nhắn của hai bạn.", "Không ai ngoài hai bạn thấy sổ này."]}
      nhanDeNghi="Đề nghị lập sổ"
      testID="lap-so"
      tieuDe="Lập sổ hai người"
    />
  );
}

export function BatMotDoi(props: { open: boolean; onClose: () => void; dangCho: boolean; deNghiCuaToi: boolean; onDeNghi: () => void; onDongY: () => void; nguoiKiaDongY: (() => void) | null }) {
  return (
    <BacDongY
      {...props}
      choPhep={["Sổ này là sổ đôi: mỗi người chỉ có một.", "Nếp nói chuyện với hai bạn như với một đôi.", "Mở đường cho vai «Người lo» và «Người chấm»."]}
      khongKeoTheo={["Nếp vẫn không đọc tin nhắn; đó là một công tắc khác.", "Không đăng gì, không ai được báo.", "Tắt được bất cứ lúc nào, sổ vẫn còn."]}
      nhanDeNghi="Đề nghị bật «Một đôi»"
      testID="bat-mot-doi"
      tieuDe="Bật «Một đôi»"
    />
  );
}

const styles = StyleSheet.create({ noiDung: { paddingBottom: 8 }, khoi: { gap: 4 } });
