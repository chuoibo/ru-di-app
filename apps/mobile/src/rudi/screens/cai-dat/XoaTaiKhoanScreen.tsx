/**
 * Xoá tài khoản (L5, ADR-0023 §2.1): hai bước, một từ gõ tay, không hộp thoại.
 *
 * Bước một nói thẳng cái gì mất và cái gì ở lại — kể cả câu khó nghe rằng sổ
 * tiền của nhóm KHÔNG mất, vì người đọc màn này có thể đang mong nó mất. Bước
 * hai bắt gõ đúng một từ; nút xoá mờ cho tới lúc đó, và nó là `outline` chứ
 * không phải nút chính: không màn nào trong app này biến hành động không lấy
 * lại được thành cái đẹp nhất trên màn.
 */
import { useRouter } from "expo-router";
import { useState } from "react";
import { StyleSheet, Text, View } from "react-native";

import { ApiError, newAttempt, thongDiepNguoiDoc } from "../../../api";
import {
  DIEU_SE_XAY_RA,
  TU_XAC_NHAN,
  xacNhanHopLe,
  xoaTaiKhoan,
} from "../../cai-dat/xoa-tai-khoan";
import { useRudiSession } from "../../session";
import { typography, useRudiTheme } from "../../theme";
import { Card, Field, RudiButton, RudiScreen, TopBar } from "../../ui";

export function XoaTaiKhoanScreen() {
  const router = useRouter();
  const { colors } = useRudiTheme();
  const { phien, phienDaDoc, resetSession } = useRudiSession();
  const [buoc, setBuoc] = useState<1 | 2>(1);
  const [daGo, setDaGo] = useState("");
  const [dangXoa, setDangXoa] = useState(false);
  const [loi, setLoi] = useState<string | null>(null);

  if (!phienDaDoc) return null;

  const xoa = async () => {
    if (phien === null || !xacNhanHopLe(daGo) || dangXoa) return;
    setDangXoa(true);
    setLoi(null);
    try {
      await xoaTaiKhoan(phien.person_id, newAttempt());
      // The session is dead on the server; drop it here too before the next
      // screen can try to use it.
      resetSession();
      router.replace("/welcome");
    } catch (error) {
      setLoi(error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null));
    } finally {
      setDangXoa(false);
    }
  };

  return (
    <RudiScreen testID="xoa-tai-khoan-screen">
      <TopBar title="Xoá tài khoản" />
      {buoc === 1 ? (
        <>
          <Card style={styles.khoi}>
            <Text style={[typography.title, { color: colors.ink }]}>Xoá tài khoản là vĩnh viễn.</Text>
            {DIEU_SE_XAY_RA.map((cau) => (
              <View key={cau} style={styles.dong}>
                <Text style={[typography.body, { color: colors.inkSoft }]}>{cau}</Text>
              </View>
            ))}
          </Card>
          <Text style={[typography.caption, { color: colors.inkFaint }]}>
            Đăng nhập lại bằng cùng số điện thoại sẽ tạo một tài khoản mới, trắng: không nhóm cũ, không tin cũ.
          </Text>
          <RudiButton label="Tôi hiểu, tiếp tục" onPress={() => setBuoc(2)} variant="outline" />
          <RudiButton label="Ở lại" onPress={() => router.back()} variant="ghost" />
        </>
      ) : (
        <>
          <Card style={styles.khoi}>
            <Text style={[typography.body, { color: colors.ink }]}>
              Gõ {TU_XAC_NHAN} vào ô dưới để xác nhận.
            </Text>
            <Field
              accessibilityLabel="Ô xác nhận xoá"
              autoCapitalize="characters"
              onChangeText={setDaGo}
              placeholder={TU_XAC_NHAN}
              value={daGo}
            />
          </Card>
          {loi ? (
            <Card>
              <Text style={[typography.body, { color: colors.warn }]}>{loi}</Text>
            </Card>
          ) : null}
          <RudiButton
            disabled={!xacNhanHopLe(daGo) || dangXoa}
            label="Xoá vĩnh viễn"
            loading={dangXoa}
            onPress={() => void xoa()}
            variant="outline"
          />
          <RudiButton disabled={dangXoa} label="Ở lại" onPress={() => router.back()} variant="ghost" />
        </>
      )}
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  khoi: { gap: 10 },
  dong: { flexDirection: "row", gap: 8 },
});
