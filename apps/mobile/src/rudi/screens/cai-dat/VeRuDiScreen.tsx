/**
 * «Về Rủ Đi» (L5, ADR-0023 §2.6): điều khoản và chính sách dữ liệu, dạng NHÁP.
 *
 * Cửa hàng ứng dụng đòi một trang như thế này, và người dùng đáng được đọc nó
 * bằng tiếng của mình chứ không phải một bản dịch máy của một mẫu tiếng Anh.
 * Trang này nói đúng những gì mã nguồn làm hôm nay: những câu ở đây đều đối
 * chiếu được với một bảng trong `models.py` hoặc một luật trong `app/domain`.
 *
 * Con dấu «BẢN NHÁP» ở đầu trang là thật chứ không phải trang trí: chưa ai có
 * chuyên môn pháp lý đọc trang này. Gỡ con dấu là việc của Lead sau khi có
 * người đọc, không phải của lát này.
 */
import { StyleSheet, Text, View } from "react-native";

import { typography, useRudiTheme } from "../../theme";
import { Card, Heading, RudiScreen, SectionHeader, TopBar } from "../../ui";
import { Stamp } from "../../ui/Stamp";

const MUC: readonly { tieuDe: string; cau: readonly string[] }[] = [
  {
    tieuDe: "Rủ Đi giữ gì của bạn",
    cau: [
      "Số điện thoại, để bạn đăng nhập và để bạn bè tìm ra bạn. Bạn tắt được việc tìm theo số ở mục Quyền riêng tư.",
      "Tên hiển thị, giới thiệu, thành phố và ảnh đại diện bạn tự nhập.",
      "Tin nhắn, ảnh, bài đăng, story và khoản chi bạn tạo trong các nhóm của mình.",
      "Sở thích bạn chọn, để Rủ Đi AI gợi ý sát hơn.",
    ],
  },
  {
    tieuDe: "Rủ Đi không giữ gì",
    cau: [
      "Không mật khẩu: đăng nhập bằng mã một lần gửi tới số của bạn.",
      "Không một thông tin ngân hàng nào. Rủ Đi nói ai nợ ai bao nhiêu; chuyển tiền là việc giữa hai người.",
      "Không vị trí chạy nền. App chỉ biết nơi bạn tự chọn khi check-in.",
      "Không bán dữ liệu cho ai, và không có quảng cáo trong app.",
    ],
  },
  {
    tieuDe: "Ai đọc được gì",
    cau: [
      "Tin nhắn và ảnh trong một nhóm: chỉ thành viên nhóm ấy.",
      "Bài trên tường: theo mức bạn chọn khi đăng, và bạn đặt được ai bình luận.",
      "Story: chỉ bạn bè, và tự biến mất sau 24 giờ.",
      "Ảnh cá nhân: chỉ mở được khi có một bài hoặc một story bạn cho phép người ấy đọc.",
    ],
  },
  {
    tieuDe: "Khi bạn chặn hoặc báo cáo",
    cau: [
      "Chặn: hai người không đọc được bài và story của nhau, không nhắn riêng được nữa. Nhóm chung giữ nguyên.",
      "Bỏ chặn không tự nối lại tình bạn; hai người kết bạn lại nếu muốn.",
      "Báo cáo đi tới người vận hành. Người bị báo cáo không biết ai đã báo cáo, và bạn không nhận trả lời tự động.",
    ],
  },
  {
    tieuDe: "Khi bạn xoá tài khoản",
    cau: [
      "Tên, giới thiệu, ảnh, bài đăng, story, bình luận và phản ứng của bạn bị xoá.",
      "Bạn rời mọi nhóm. Tin nhắn cũ ở lại với tên «Người dùng đã rời», vì chúng là một phần cuộc trò chuyện của người khác.",
      "Sổ tiền của các nhóm giữ nguyên: xoá tài khoản không xoá một khoản nợ.",
      "Đăng nhập lại bằng cùng số điện thoại sẽ tạo một tài khoản mới, trắng.",
    ],
  },
];

export function VeRuDiScreen() {
  const { colors } = useRudiTheme();
  return (
    <RudiScreen testID="ve-rudi-screen">
      <TopBar title="Về Rủ Đi" />
      <View style={styles.dau}>
        <Stamp label="BẢN NHÁP" tilt={-3} tone="accent" variant="ink" />
      </View>
      <Heading
        subtitle="Trang này nói đúng những gì app đang làm hôm nay. Chưa có người làm luật đọc nó, nên nó chưa phải điều khoản chính thức."
        title="Điều khoản và dữ liệu"
      />
      {MUC.map((muc) => (
        <View key={muc.tieuDe}>
          <SectionHeader title={muc.tieuDe} />
          <Card style={styles.khoi}>
            {muc.cau.map((cau) => (
              <Text key={cau} style={[typography.body, { color: colors.inkSoft }]}>
                {cau}
              </Text>
            ))}
          </Card>
        </View>
      ))}
      <Text style={[typography.caption, { color: colors.inkFaint }]}>
        Có gì sai trong trang này? Nói với người đã mời bạn vào Rủ Đi. Bản này chưa có địa chỉ liên hệ trong app.
      </Text>
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  dau: { flexDirection: "row" },
  khoi: { gap: 10 },
});
