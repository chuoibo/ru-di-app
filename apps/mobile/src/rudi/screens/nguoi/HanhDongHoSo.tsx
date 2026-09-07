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
import { boChan, chan } from "../../cai-dat/quyen-rieng-tu";
import { typography, useRudiTheme } from "../../theme";
import { RudiButton } from "../../ui";
import { Sheet } from "../../ui/Sheet";
import { NoiDungBaoCao } from "./NoiDungBaoCao";

type Buoc = "menu" | "xac-nhan-chan" | "bao-cao";

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
        <NoiDungBaoCao
          actorId={actorId}
          loai="person"
          onThoi={() => setBuoc("menu")}
          onXong={() => {
            setBuoc("menu");
            onClose();
          }}
          targetId={personId}
        />
      ) : null}
      {loi ? <Text style={[typography.caption, { color: colors.warn }]}>{loi}</Text> : null}
    </Sheet>
  );
}

const styles = StyleSheet.create({
  khoi: { gap: 10 },
});
