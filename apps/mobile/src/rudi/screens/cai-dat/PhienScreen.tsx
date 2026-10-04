/**
 * «Đăng nhập & phiên» (L5, ADR-0023 §2.5): tài khoản này đang mở ở đâu.
 *
 * Không có tên thiết bị vì máy chủ không lưu tên thiết bị. Hàng nào là phiên
 * đang dùng thì máy chủ nói, không phải màn này đoán — và chính hàng ấy KHÔNG
 * có nút thu hồi: đăng xuất máy mình là việc của mục Tài khoản, và trộn hai
 * việc vào một nút là cách nhanh nhất để ai đó tự khoá mình ra ngoài.
 */
import { useFocusEffect } from "expo-router";
import { useCallback, useState } from "react";
import { StyleSheet, Text, View } from "react-native";

import { ApiError, attemptFor, thongDiepNguoiDoc, type Attempt } from "../../../api";
import { cauPhien, docPhien, thuHoiPhien, type PhienWire } from "../../cai-dat/phien-cai-dat";
import { useRudiSession } from "../../session";
import { typography, useRudiTheme } from "../../theme";
import { NhomHang, RudiButton, RudiScreen, TopBar } from "../../ui";
import { Stamp } from "../../ui/Stamp";
import { EmptyState } from "../../ui/EmptyState";
import { ErrorState } from "../../ui/ErrorState";
import { SkeletonRow } from "../../ui/Skeleton";
import { ngayVN } from "../../ngay-viet";

type Trang =
  | { pha: "dang-doc" }
  | { pha: "xong"; phien: PhienWire[] }
  | { pha: "hong"; loi: string };

export function PhienScreen() {
  const { colors } = useRudiTheme();
  const { phien: sessionCuaToi, phienDaDoc } = useRudiSession();
  const [trang, setTrang] = useState<Trang>({ pha: "dang-doc" });
  const [dangThuHoi, setDangThuHoi] = useState<string | null>(null);
  const [attempts] = useState<Record<string, Attempt>>({});

  const personId = sessionCuaToi?.person_id ?? "";

  const nap = useCallback(async () => {
    if (personId === "") return;
    try {
      const ds = await docPhien(personId);
      setTrang({ pha: "xong", phien: ds.sessions });
    } catch (error) {
      setTrang({
        pha: "hong",
        loi: error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null),
      });
    }
  }, [personId]);

  useFocusEffect(
    useCallback(() => {
      void nap();
    }, [nap]),
  );

  const thuHoi = async (id: string) => {
    if (dangThuHoi !== null) return;
    setDangThuHoi(id);
    try {
      await thuHoiPhien(id, personId, attemptFor(attempts, `thu-hoi:${id}`));
      await nap();
    } catch (error) {
      setTrang({
        pha: "hong",
        loi: error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null),
      });
    } finally {
      setDangThuHoi(null);
    }
  };

  if (!phienDaDoc) return null;

  return (
    <RudiScreen testID="phien-screen">
      <TopBar title="Đăng nhập & phiên" />
      <Text style={[typography.body, { color: colors.inkSoft }]}>
        Mỗi lần đăng nhập là một phiên. Đăng xuất một phiên ở đây thì máy đó phải đăng nhập lại.
      </Text>
      {trang.pha === "dang-doc" ? (
        <View style={styles.khoi}>
          <SkeletonRow />
          <SkeletonRow />
        </View>
      ) : null}
      {trang.pha === "hong" ? (
        <ErrorState body={trang.loi} onRetry={() => void nap()} title="Chưa đọc được danh sách phiên" />
      ) : null}
      {trang.pha === "xong" && trang.phien.length === 0 ? (
        <EmptyState
          body="Phiên hiện tại sẽ hiện ở đây sau khi được ghi nhận."
          kind="first-use"
          title="Chưa có phiên nào"
        />
      ) : null}
      {trang.pha === "xong" ? (
        <NhomHang>
          {/* «Phiên này» first, always: web and Android listed the rows in the
              server's order, so the one to keep moved (B11 critique). */}
          {[...trang.phien].sort((a, b) => Number(b.current) - Number(a.current)).map((row) => (
            <View key={row.id} style={styles.hang}>
              <View style={styles.hangChu}>
                <Text style={[typography.label, { color: colors.ink }]}>{cauPhien(row)}</Text>
                <Text style={[typography.caption, { color: colors.inkFaint }]}>
                  Hết hạn {ngayVN(row.expires_at)}
                </Text>
              </View>
              {row.current ? (
                <Stamp label="PHIÊN NÀY" tilt={-3} tone="accent" variant="ink" />
              ) : (
                <RudiButton
                  // The row is another session: «phiên này» on its button read
                  // as the one in your hand (B11 critique, P1).
                  accessibilityLabel={`Đăng xuất phiên ${cauPhien(row)}`}
                  compact
                  disabled={dangThuHoi !== null}
                  full={false}
                  label="Đăng xuất phiên đó"
                  loading={dangThuHoi === row.id}
                  onPress={() => void thuHoi(row.id)}
                  variant="outline"
                />
              )}
            </View>
          ))}
        </NhomHang>
      ) : null}
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  khoi: { gap: 12 },
  hang: { flexDirection: "row", alignItems: "center", gap: 12, minHeight: 56 },
  hangChu: { flex: 1, gap: 2 },
});
