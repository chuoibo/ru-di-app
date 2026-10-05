/**
 * «Thiết bị nhắn tin mã hoá» (ADR-0057 §1.3–§1.4): which phones hold this
 * account's end-to-end keys.
 *
 * The server vouches for whose device is whose, so this list is where a person
 * catches a device they do not know -- and the key mark is what two phones
 * compare out of band. This phone's own mark is computed from the keys on
 * this phone, never from the server's list: comparing two copies of the
 * server's word would prove nothing (security review 05/10). Where the
 * server holds other keys for this phone, the screen says so. A device taken
 * out here leaves every room at the next
 * commit and reads nothing sent after. This phone's own row has no button:
 * taking it out is signing out, which belongs to «Tài khoản».
 */
import { useFocusEffect } from "expo-router";
import { useCallback, useState } from "react";
import { Platform, StyleSheet, Text, View } from "react-native";

import { ApiError, thongDiepNguoiDoc } from "../../../api";
import type { Card } from "../../chat/e2ee/kieu";
import { cungKhoa, dauKhoa, docThietBi, goThietBi, type ThietBiWire } from "../../chat/e2ee/thiet-bi";
import { coMaHoa, theCuaMay } from "../../chat/e2ee/useTinNhanV2";
import { ngayVN } from "../../ngay-viet";
import { useRudiSession } from "../../session";
import { typography, useRudiTheme } from "../../theme";
import { NhomHang, RudiButton, RudiScreen, TopBar } from "../../ui";
import { EmptyState } from "../../ui/EmptyState";
import { ErrorState } from "../../ui/ErrorState";
import { SkeletonRow } from "../../ui/Skeleton";
import { Stamp } from "../../ui/Stamp";

type Trang =
  | { pha: "dang-doc" }
  | { pha: "xong"; ds: ThietBiWire[]; toiDa: number; mayNay: Card | null }
  | { pha: "hong"; loi: string };

export function ThietBiMaHoaScreen() {
  const { colors } = useRudiTheme();
  const { phien, phienDaDoc } = useRudiSession();
  const [trang, setTrang] = useState<Trang>({ pha: "dang-doc" });
  const [dangGo, setDangGo] = useState<string | null>(null);
  const personId = phien?.person_id ?? "";

  const nap = useCallback(async () => {
    if (personId === "") return;
    try {
      const [r, mayNay] = await Promise.all([docThietBi(personId), theCuaMay(personId)]);
      setTrang({ pha: "xong", ds: r.devices.filter((d) => d.revoked_at === null), toiDa: r.max_devices, mayNay });
    } catch (error) {
      setTrang({ pha: "hong", loi: error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null) });
    }
  }, [personId]);

  useFocusEffect(
    useCallback(() => {
      void nap();
    }, [nap]),
  );

  const go = async (deviceId: string) => {
    if (dangGo !== null) return;
    setDangGo(deviceId);
    try {
      await goThietBi(personId, deviceId);
      await nap();
    } catch (error) {
      setTrang({ pha: "hong", loi: error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null) });
    } finally {
      setDangGo(null);
    }
  };

  if (!phienDaDoc) return null;

  return (
    <RudiScreen testID="thiet-bi-ma-hoa-screen">
      <TopBar title="Thiết bị nhắn tin mã hoá" />
      <Text style={[typography.body, { color: colors.inkSoft }]}>
        Tin trong phòng mã hoá đầu cuối chỉ mở được trên các máy dưới đây. Thấy máy lạ thì gỡ ngay: máy đã gỡ không đọc
        được tin gửi sau đó.
      </Text>
      {!coMaHoa() ? (
        <Text style={[typography.caption, { color: colors.inkSoft }]}>
          Bản ứng dụng này chưa có mã hoá đầu cuối nên máy này không có trong danh sách.
        </Text>
      ) : null}
      {trang.pha === "dang-doc" ? (
        <View style={styles.khoi}>
          <SkeletonRow />
          <SkeletonRow />
        </View>
      ) : null}
      {trang.pha === "hong" ? (
        <ErrorState body={trang.loi} onRetry={() => void nap()} title="Chưa đọc được danh sách thiết bị" />
      ) : null}
      {/* This phone holds keys the server does not list (or lists under
          another id): the list cannot be trusted, and it is said. */}
      {trang.pha === "xong" && trang.mayNay !== null && !trang.ds.some((d) => d.card.device_id === trang.mayNay?.device_id) ? (
        <View style={styles.canhBao}>
          <Text style={[typography.label, { color: colors.warn }]}>Máy chủ không liệt kê máy này</Text>
          <Text style={[typography.caption, { color: colors.warn }]}>
            Máy này giữ khoá mã hoá nhưng không có trong danh sách dưới đây. Đừng tin danh sách này cho tới khi hỏi rõ.
          </Text>
          <Text selectable style={[typography.caption, styles.dau, { color: colors.inkSoft }]}>
            Dấu khoá máy này {dauKhoa(trang.mayNay)}
          </Text>
        </View>
      ) : null}
      {trang.pha === "xong" && trang.ds.length === 0 ? (
        <EmptyState
          body="Mở một cuộc trò chuyện trên máy có bản ứng dụng mới nhất, máy đó sẽ hiện ở đây."
          kind="first-use"
          title="Chưa có thiết bị nào"
        />
      ) : null}
      {trang.pha === "xong" && trang.ds.length > 0 ? (
        <>
          <NhomHang>
            {[...trang.ds]
              .sort((a, b) => Number(b.card.device_id === trang.mayNay?.device_id) - Number(a.card.device_id === trang.mayNay?.device_id))
              .map((d) => {
                const mayNay = trang.mayNay !== null && d.card.device_id === trang.mayNay.device_id;
                // This phone: the mark from its own keys; a server card that
                // disagrees is the attack the mark is there to catch.
                const lech = mayNay && trang.mayNay !== null && !cungKhoa(d.card, trang.mayNay);
                const dau = mayNay && trang.mayNay !== null ? dauKhoa(trang.mayNay) : dauKhoa(d.card);
                return (
                  <View key={d.card.device_id} style={styles.hang}>
                    <View style={styles.hangChu}>
                      <Text style={[typography.label, { color: colors.ink }]}>{d.label}</Text>
                      <Text style={[typography.caption, { color: colors.inkFaint }]}>Thêm ngày {ngayVN(d.created_at)}</Text>
                      <Text selectable style={[typography.caption, styles.dau, { color: colors.inkSoft }]}>
                        Dấu khoá {dau}
                      </Text>
                      {lech ? (
                        <Text style={[typography.caption, { color: colors.warn }]}>
                          Máy chủ đang giữ một khoá khác cho máy này. Đừng nhắn tin mã hoá trên máy này cho tới khi hỏi rõ.
                        </Text>
                      ) : null}
                    </View>
                    {mayNay ? (
                      <Stamp label="MÁY NÀY" tilt={-3} tone="accent" variant="ink" />
                    ) : (
                      <RudiButton
                        accessibilityLabel={`Gỡ thiết bị ${d.label}, thêm ngày ${ngayVN(d.created_at)}`}
                        compact
                        disabled={dangGo !== null}
                        full={false}
                        label="Gỡ"
                        loading={dangGo === d.card.device_id}
                        onPress={() => void go(d.card.device_id)}
                        variant="outline"
                      />
                    )}
                  </View>
                );
              })}
          </NhomHang>
          <Text style={[typography.caption, { color: colors.inkSoft }]}>
            Đang dùng {trang.ds.length} trên tối đa {trang.toiDa} thiết bị. Muốn chắc một máy là của bạn, so dấu khoá của nó ở
            đây với dấu khoá hiện trên chính máy đó.
          </Text>
        </>
      ) : null}
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  khoi: { gap: 12 },
  canhBao: { gap: 4 },
  hang: { flexDirection: "row", alignItems: "center", gap: 12, minHeight: 64 },
  hangChu: { flex: 1, gap: 2 },
  // Monospace so two phones' marks line up group by group when compared.
  dau: { fontFamily: Platform.select({ ios: "Menlo", default: "monospace" }) },
});
