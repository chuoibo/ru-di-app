import { StyleSheet, Text, View } from "react-native";

import { typography, useRudiTheme } from "../../theme";
import { Heading, RudiButton, Segmented } from "../../ui";
import { Stamp } from "../../ui/Stamp";
import { CauTaiCho } from "../../ui/CauTaiCho";
import { Sheet } from "../../ui/Sheet";
import { useGiuKhiDong } from "../../ui/giu-khi-dong";

/**
 * Friendship includes two people. Couple mode still requires both people
 * to agree; switching back withdraws only this person's couple consent.
 */
export function LoaiSo({ open, onClose, batDoi: batDoiSong, dangCho: dangChoSong, deNghiCuaToi: deNghiCuaToiSong = true, tenNguoiKia, onChonDoi, onChonBan, onDongY, dangLam = false, loi = null, testID }: { open: boolean; onClose: () => void; batDoi: boolean; dangCho: boolean; deNghiCuaToi?: boolean; tenNguoiKia?: string; onChonDoi: () => void; onChonBan: () => void; onDongY?: () => void; dangLam?: boolean; loi?: string | null; testID?: string }) {
  const { colors, space } = useRudiTheme();
  // Closing, the sheet keeps what it said open (QA UI-084).
  const { batDoi, dangCho, deNghiCuaToi } = useGiuKhiDong(open, { batDoi: batDoiSong, dangCho: dangChoSong, deNghiCuaToi: deNghiCuaToiSong });
  // Highlight follows what IS, never what is proposed: a lit «Một đôi» while
  // the other person had not agreed read as already on (blind read 12/09).
  const chon = batDoi ? 1 : 0;
  return (
    <Sheet accessibilityLabel="Loại sổ" onClose={onClose} open={open} showsVerticalScrollIndicator testID={testID ?? "loai-so"} footer={
      <View style={{ gap: space.xs }}>
        <CauTaiCho cau={loi} />
        {dangLam ? <Text accessibilityLiveRegion="polite" style={[typography.note, { color: colors.inkSoft }]}>Đang cập nhật loại sổ…</Text> : null}
        {/* The receiver's answer. Before 23/09 the receiver read «Chờ người
            ấy đồng ý» about their own decision and had no button; the only
            thing to press was «Một đôi», which filed a SECOND proposal. */}
        {dangCho && !batDoi && !deNghiCuaToi && onDongY ? <RudiButton label="Đồng ý là một đôi" disabled={dangLam} loading={dangLam} onPress={onDongY} /> : null}
        <RudiButton label="Xong" onPress={onClose} variant="ghost" />
      </View>
    }>
      <View style={[styles.noiDung, { gap: space.md }]}>
        <Heading size="h2" subtitle="Đổi được bất cứ lúc nào. Tờ giấy đã gửi không đổi theo." title="Loại sổ" />
        <Segmented disabled={dangLam} items={["Hội bạn", "Cặp đôi"]} onSelect={(i) => (i === 1 ? onChonDoi() : onChonBan())} selected={chon} testIDs={["loai-so-ban", "loai-so-doi"]} />
        {dangCho && !batDoi && deNghiCuaToi ? <Stamp label="Đã đề nghị" tone="ink" /> : null}
        <Text style={[typography.caption, { color: colors.inkSoft }]}>
          {batDoi
            ? "Một góc dành cho hai mình. Những buổi hẹn nhỏ, những chuyến đi xa."
            : dangCho && !deNghiCuaToi
              ? `${tenNguoiKia ?? "Người ấy"} đề nghị hai bạn là «Một đôi». Bạn đồng ý thì bật cho cả hai.`
              : dangCho
                ? `Đã đề nghị «Một đôi». Chờ ${tenNguoiKia ?? "người ấy"} đồng ý trên máy của họ.`
                : "Hội bạn bắt đầu từ hai người. Cùng rủ nhau, chọn chỗ và giữ lại những cuộc đi."}
        </Text>
      </View>
    </Sheet>
  );
}

const styles = StyleSheet.create({ noiDung: { paddingBottom: 8 } });
