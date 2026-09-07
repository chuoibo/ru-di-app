/**
 * «Thêm hành động» trên hồ sơ người khác (L5, ADR-0023 §2.3–2.4): chặn, bỏ
 * chặn, báo cáo.
 *
 * Ba việc này nằm sau một nút phụ chứ không nằm cạnh «Nhắn tin»: chúng hiếm,
 * và một nút chặn to bằng nút nhắn tin là một lời mời bấm nhầm. Báo cáo hỏi
 * lý do trong một bộ đóng rồi mới gửi; không có ô nào bắt buộc phải viết.
 *
 * Không dùng `Alert` của hệ thống: mọi xác nhận trong app này là một bước
 * trong màn, đọc được bằng máy đọc màn hình và lái được bằng Maestro.
 */
import { useState } from "react";
import { StyleSheet, Text, View } from "react-native";

import { ApiError, newAttempt, thongDiepNguoiDoc } from "../../../api";
import {
  baoCao,
  boChan,
  chan,
  LY_DO_BAO_CAO,
  type LoaiBaoCao,
  type LyDoBaoCao,
} from "../../cai-dat/quyen-rieng-tu";
import { typography, useRudiTheme } from "../../theme";
import { Field, RudiButton } from "../../ui";
import { Sheet } from "../../ui/Sheet";

type Buoc = "menu" | "xac-nhan-chan" | "bao-cao" | "da-bao-cao";

export function HanhDongHoSoSheet({
  open,
  onClose,
  actorId,
  personId,
  displayName,
  daChan,
  onDoiChan,
}: {
  open: boolean;
  onClose: () => void;
  actorId: string;
  personId: string;
  displayName: string;
  daChan: boolean;
  onDoiChan: (daChan: boolean) => void;
}) {
  const { colors } = useRudiTheme();
  const [buoc, setBuoc] = useState<Buoc>("menu");
  const [lyDo, setLyDo] = useState<LyDoBaoCao>("spam");
  const [ghiChu, setGhiChu] = useState("");
  const [dangGui, setDangGui] = useState(false);
  const [loi, setLoi] = useState<string | null>(null);

  const baoLoi = (error: unknown) =>
    setLoi(error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null));

  const doiChan = async (bat: boolean) => {
    if (dangGui) return;
    setDangGui(true);
    setLoi(null);
    try {
      if (bat) await chan(personId, actorId, newAttempt());
      else await boChan(personId, actorId, newAttempt());
      onDoiChan(bat);
      setBuoc("menu");
      onClose();
    } catch (error) {
      baoLoi(error);
    } finally {
      setDangGui(false);
    }
  };

  const gui = async () => {
    if (dangGui) return;
    setDangGui(true);
    setLoi(null);
    try {
      await baoCao("person" as LoaiBaoCao, personId, lyDo, ghiChu, actorId, newAttempt());
      setGhiChu("");
      setBuoc("da-bao-cao");
    } catch (error) {
      baoLoi(error);
    } finally {
      setDangGui(false);
    }
  };

  return (
    <Sheet
      accessibilityLabel="Hành động với hồ sơ"
      onClose={() => {
        setBuoc("menu");
        setLoi(null);
        onClose();
      }}
      open={open}
      testID="hanh-dong-ho-so"
    >
      {buoc === "menu" ? (
        <View style={styles.khoi}>
          {daChan ? (
            <RudiButton
              icon="hand-left-outline"
              label={`Bỏ chặn ${displayName}`}
              loading={dangGui}
              onPress={() => void doiChan(false)}
              variant="outline"
            />
          ) : (
            <RudiButton
              icon="hand-left-outline"
              label={`Chặn ${displayName}`}
              onPress={() => setBuoc("xac-nhan-chan")}
              variant="outline"
            />
          )}
          <RudiButton
            icon="flag-outline"
            label="Báo cáo"
            onPress={() => setBuoc("bao-cao")}
            variant="ghost"
          />
        </View>
      ) : null}
      {buoc === "xac-nhan-chan" ? (
        <View style={styles.khoi}>
          <Text style={[typography.body, { color: colors.ink }]}>
            Chặn {displayName}? Hai người sẽ không đọc được bài và story của nhau. Nhóm chung vẫn giữ nguyên.
          </Text>
          <RudiButton label="Chặn" loading={dangGui} onPress={() => void doiChan(true)} variant="outline" />
          <RudiButton label="Thôi" onPress={() => setBuoc("menu")} variant="ghost" />
        </View>
      ) : null}
      {buoc === "bao-cao" ? (
        <View style={styles.khoi}>
          <Text style={[typography.label, { color: colors.ink }]}>Vì sao bạn báo cáo?</Text>
          <View accessibilityRole="radiogroup" style={styles.lyDo}>
            {LY_DO_BAO_CAO.map((muc) => (
              <RudiButton
                compact
                full={false}
                key={muc.ma}
                label={muc.nhan}
                onPress={() => setLyDo(muc.ma)}
                variant={lyDo === muc.ma ? "soft" : "ghost"}
              />
            ))}
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
          <RudiButton label="Gửi báo cáo" loading={dangGui} onPress={() => void gui()} variant="outline" />
          <RudiButton label="Thôi" onPress={() => setBuoc("menu")} variant="ghost" />
        </View>
      ) : null}
      {buoc === "da-bao-cao" ? (
        <View style={styles.khoi}>
          <Text style={[typography.body, { color: colors.ink }]}>Đã gửi báo cáo</Text>
          <Text style={[typography.caption, { color: colors.inkFaint }]}>
            Người vận hành sẽ đọc. Bạn không nhận được trả lời tự động, và người kia không biết ai báo cáo.
          </Text>
          <RudiButton label="Xong" onPress={() => { setBuoc("menu"); onClose(); }} variant="ghost" />
        </View>
      ) : null}
      {loi ? <Text style={[typography.caption, { color: colors.warn }]}>{loi}</Text> : null}
    </Sheet>
  );
}

const styles = StyleSheet.create({
  khoi: { gap: 10 },
  lyDo: { flexDirection: "row", flexWrap: "wrap", gap: 8 },
});
