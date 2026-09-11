/**
 * «Người đã chặn» (L5, ADR-0023 §2.3): danh sách của người chặn, không của
 * người bị chặn.
 *
 * Không có màn nào trong app này nói cho ai biết họ đang bị chặn. Đó không
 * phải sơ suất: `BLOCKED_IS_SILENT` là quyết định, và một danh sách «ai chặn
 * tôi» sẽ biến mỗi lần chặn thành một cuộc đối đầu.
 */
import { useFocusEffect } from "expo-router";
import { useCallback, useState } from "react";
import { StyleSheet, Text, View } from "react-native";

import { ApiError, attemptFor, thongDiepNguoiDoc, type Attempt } from "../../../api";
import { boChan, docDaChan, type NguoiBiChan } from "../../cai-dat/quyen-rieng-tu";
import { useRudiSession } from "../../session";
import { typography, useRudiTheme } from "../../theme";
import { NhomHang, RudiButton, RudiScreen, TopBar } from "../../ui";
import { Avatar } from "../../ui/Avatar";
import { EmptyState } from "../../ui/EmptyState";
import { ErrorState } from "../../ui/ErrorState";
import { SkeletonRow } from "../../ui/Skeleton";

type Trang =
  | { pha: "dang-doc" }
  | { pha: "xong"; nguoi: NguoiBiChan[] }
  | { pha: "hong"; loi: string };

export function DaChanScreen() {
  const { colors } = useRudiTheme();
  const { phien, phienDaDoc } = useRudiSession();
  const [trang, setTrang] = useState<Trang>({ pha: "dang-doc" });
  const [dangGo, setDangGo] = useState<string | null>(null);
  const [attempts] = useState<Record<string, Attempt>>({});
  const personId = phien?.person_id ?? "";

  const nap = useCallback(async () => {
    if (personId === "") return;
    try {
      const ds = await docDaChan(personId);
      setTrang({ pha: "xong", nguoi: ds.blocked });
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

  const go = async (nguoi: NguoiBiChan) => {
    if (dangGo !== null) return;
    setDangGo(nguoi.person_id);
    try {
      await boChan(nguoi.person_id, personId, attemptFor(attempts, `go:${nguoi.person_id}`));
      await nap();
    } catch (error) {
      setTrang({
        pha: "hong",
        loi: error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null),
      });
    } finally {
      setDangGo(null);
    }
  };

  if (!phienDaDoc) return null;

  return (
    <RudiScreen testID="da-chan-screen">
      <TopBar title="Người đã chặn" />
      <Text style={[typography.body, { color: colors.inkSoft }]}>
        Người bạn chặn không đọc được bài và story của bạn, và bạn cũng không đọc được của họ. Nhóm chung vẫn giữ nguyên.
      </Text>
      {trang.pha === "dang-doc" ? (
        <View style={styles.khoi}>
          <SkeletonRow />
        </View>
      ) : null}
      {trang.pha === "hong" ? (
        <ErrorState body={trang.loi} onRetry={() => void nap()} title="Chưa đọc được danh sách" />
      ) : null}
      {trang.pha === "xong" && trang.nguoi.length === 0 ? (
        <EmptyState
          body="Khi bạn chặn ai đó, họ sẽ hiện ở đây để bạn gỡ chặn."
          kind="first-use"
          title="Bạn chưa chặn ai"
        />
      ) : null}
      {trang.pha === "xong" ? (
        <NhomHang>
          {trang.nguoi.map((nguoi) => (
            <View key={nguoi.person_id} style={styles.hang}>
              <Avatar name={nguoi.display_name} size={40} />
              <View style={styles.hangChu}>
                <Text style={[typography.label, { color: colors.ink }]}>{nguoi.display_name}</Text>
                <Text style={[typography.caption, { color: colors.inkFaint }]}>
                  Chặn từ {new Date(nguoi.blocked_at).toLocaleDateString("vi-VN")}
                </Text>
              </View>
              <RudiButton
                accessibilityLabel={`Bỏ chặn ${nguoi.display_name}`}
                compact
                disabled={dangGo !== null}
                full={false}
                label="Bỏ chặn"
                loading={dangGo === nguoi.person_id}
                onPress={() => void go(nguoi)}
                variant="outline"
              />
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
