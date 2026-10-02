/**
 * Nội dung của một lần báo cáo (L5, ADR-0023 §2.4) — dùng chung cho cả bốn
 * loại đối tượng.
 *
 * Một bộ câu chữ cho mọi chỗ, không phải một bản sao cho mỗi màn: lý do, ô ghi
 * chú và câu cảm ơn phải giống nhau khi báo cáo một người, một bài hay một tin,
 * và hai bản sao là hai chỗ để chúng trôi ra khác nhau.
 *
 * Đây là một khối nội dung chứ không phải một khay: mỗi màn tự mở khay của
 * mình (hồ sơ mở sau «Thêm hành động», chat mở sau khi nhấn giữ một tin), và
 * một khay lồng trong một khay là hai lớp che nhau trên màn nhỏ.
 *
 * Không ô nào bắt buộc phải viết, và không bước nào cho người bị báo cáo biết
 * ai đã báo cáo.
 */
import { useState } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";

import { ApiError, newAttempt, thongDiepNguoiDoc } from "../../../api";
import { baoCao, LY_DO_BAO_CAO, type LoaiBaoCao, type LyDoBaoCao } from "../../cai-dat/quyen-rieng-tu";
import { typography, useRudiTheme } from "../../theme";
import { Field, RudiButton } from "../../ui";
import { CauTaiCho } from "../../ui/CauTaiCho";
import { toggleState } from "../../../ui/a11y";

export function NoiDungBaoCao({
  actorId,
  loai,
  targetId,
  onXong,
  onThoi,
}: {
  actorId: string;
  loai: LoaiBaoCao;
  targetId: string;
  /** Người dùng bấm «Xong» sau khi gửi. */
  onXong: () => void;
  /** Người dùng đổi ý trước khi gửi. */
  onThoi: () => void;
}) {
  const { colors, space } = useRudiTheme();
  const [lyDo, setLyDo] = useState<LyDoBaoCao>("spam");
  const [ghiChu, setGhiChu] = useState("");
  const [dangGui, setDangGui] = useState(false);
  const [daGui, setDaGui] = useState(false);
  const [loi, setLoi] = useState<string | null>(null);

  const gui = async () => {
    if (dangGui) return;
    setDangGui(true);
    setLoi(null);
    try {
      await baoCao(loai, targetId, lyDo, ghiChu, actorId, newAttempt());
      setGhiChu("");
      setDaGui(true);
    } catch (error) {
      setLoi(error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null));
    } finally {
      setDangGui(false);
    }
  };

  if (daGui) {
    return (
      <View style={styles.khoi}>
        <Text style={[typography.body, { color: colors.ink }]}>Đã gửi báo cáo</Text>
        <Text style={[typography.caption, { color: colors.inkFaint }]}>
          Người vận hành sẽ đọc. Bạn không nhận được trả lời tự động, và người kia không biết ai báo cáo.
        </Text>
        <RudiButton label="Xong" onPress={onXong} variant="ghost" />
      </View>
    );
  }

  return (
    <View style={styles.khoi}>
      <Text style={[typography.label, { color: colors.ink }]}>Vì sao bạn báo cáo?</Text>
      {/* One choice of five, drawn as one: a ring that fills, not a fill that
          only changes colour (QA UI-067, where four reasons read as red links
          and the chosen one had no state a screen reader could hear). */}
      <View accessibilityLabel="Vì sao bạn báo cáo?" accessibilityRole="radiogroup" style={styles.lyDo}>
        {LY_DO_BAO_CAO.map((muc, i) => {
          const chon = lyDo === muc.ma;
          return (
            <Pressable
              {...toggleState("radio", chon, () => setLyDo(muc.ma))}
              accessibilityLabel={muc.nhan}
              key={muc.ma}
              onPress={() => setLyDo(muc.ma)}
              style={({ pressed }) => [styles.hangLyDo, { gap: space.md }, i > 0 && { borderTopColor: colors.line, borderTopWidth: StyleSheet.hairlineWidth }, pressed && styles.bam]}
            >
              <View style={[styles.vong, { borderColor: chon ? colors.ink : colors.lineStrong }]}>
                {chon ? <View style={[styles.cham, { backgroundColor: colors.ink }]} /> : null}
              </View>
              <Text style={[typography.body, styles.flex, { color: colors.ink }]}>{muc.nhan}</Text>
            </Pressable>
          );
        })}
      </View>
      <Field
        accessibilityLabel="Ô ghi chú báo cáo"
        label="Thêm gì đó, nếu muốn"
        maxLength={500}
        multiline
        numberOfLines={3}
        onChangeText={setGhiChu}
        value={ghiChu}
      />
      <CauTaiCho cau={loi} />
      <RudiButton label="Gửi báo cáo" loading={dangGui} onPress={() => void gui()} variant="outline" />
      <RudiButton label="Thôi" onPress={onThoi} variant="ghost" />
    </View>
  );
}

const styles = StyleSheet.create({
  khoi: { gap: 10 },
  lyDo: { gap: 0 },
  hangLyDo: { flexDirection: "row", alignItems: "center", minHeight: 48, paddingVertical: 6 },
  vong: { width: 20, height: 20, borderRadius: 10, borderWidth: 2, alignItems: "center", justifyContent: "center" },
  cham: { width: 10, height: 10, borderRadius: 5 },
  flex: { flex: 1 },
  bam: { opacity: 0.7 },
});
