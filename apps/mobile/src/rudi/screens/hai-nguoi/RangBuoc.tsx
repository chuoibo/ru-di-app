import { useEffect, useState } from "react";
import { StyleSheet, Text, View, useWindowDimensions } from "react-native";

import { typography, useRudiTheme } from "../../theme";
import type { RangBuoc as RangBuocKieu } from "../../to-giay/so-fixture";
import { Heading, RudiButton } from "../../ui";
import { CauTaiCho } from "../../ui/CauTaiCho";
import { Field } from "../../ui/Field";
import { Sheet } from "../../ui/Sheet";

/**
 * The two shared constraints (spec §6.4, ADR-0027 K7): «Không ăn được» and
 * «Đừng». Mine are two fields; the other person's are read-only lines, in
 * their words, because a constraint is something its owner writes. They live
 * in the shared area of the notebook so Nếp's draft can honour them without
 * reading anything private.
 */
export function RangBuoc({ open, onClose, toi, nguoiKia, tenNguoiKia, onLuu, dangLuu = false, loi = null, testID }: { open: boolean; onClose: () => void; toi: RangBuocKieu; nguoiKia: RangBuocKieu; tenNguoiKia: string; onLuu: (rb: RangBuocKieu) => void; dangLuu?: boolean; loi?: string | null; testID?: string }) {
  const { colors, space } = useRudiTheme();
  const { fontScale } = useWindowDimensions();
  const caoToiThieu = 3 * typography.body.lineHeight * fontScale;
  const caoToiDa = 8 * typography.body.lineHeight * fontScale;
  const [caoKhongAn, setCaoKhongAn] = useState(0);
  const [caoDung, setCaoDung] = useState(0);
  const kieuNhap = (cao: number) => ({ minHeight: caoToiThieu, maxHeight: caoToiDa, height: Math.min(caoToiDa, Math.max(caoToiThieu, Math.ceil(cao))) });
  const [khongAn, setKhongAn] = useState(toi.khong_an_duoc);
  const [dung, setDung] = useState(toi.dung);
  useEffect(() => {
    if (open) {
      setKhongAn(toi.khong_an_duoc);
      setDung(toi.dung);
    }
  }, [open, toi.khong_an_duoc, toi.dung]);
  const doi = khongAn.trim() !== toi.khong_an_duoc || dung.trim() !== toi.dung;
  return (
    <Sheet accessibilityLabel="Những điều cần tránh" onClose={onClose} open={open} avoidKeyboard prioritizeFooter showsVerticalScrollIndicator testID={testID ?? "rang-buoc"} footer={
      <View style={{ gap: space.xs, paddingTop: space.sm }}>
        <CauTaiCho cau={loi} />
        {doi || dangLuu ? <RudiButton disabled={dangLuu} label="Lưu điều cần tránh" loading={dangLuu} onPress={() => onLuu({ khong_an_duoc: khongAn.trim(), dung: dung.trim() })} /> : null}
        <RudiButton label={doi ? "Bỏ thay đổi" : "Xong"} onPress={onClose} variant="ghost" />
      </View>
    }>
      <View style={[styles.noiDung, { gap: space.md }]}>
        <Heading size="h2" subtitle="Người kia đọc được những điều bạn ghi ở đây. Không cần giải thích lý do." title="Những điều cần tránh" />
        <Field label="Không ăn được" multiline numberOfLines={3} style={kieuNhap(caoKhongAn)} onContentSizeChange={(event) => setCaoKhongAn(event.nativeEvent.contentSize.height)} onChangeText={setKhongAn} placeholder="Hải sản" testID="rang-buoc-khong-an" value={khongAn} />
        <Field label="Đừng" multiline numberOfLines={3} style={kieuNhap(caoDung)} onContentSizeChange={(event) => setCaoDung(event.nativeEvent.contentSize.height)} onChangeText={setDung} placeholder="Đừng rủ sau 21:00 ngày thường." testID="rang-buoc-dung" value={dung} />
        <View style={[styles.cuaNguoiKia, { borderTopColor: colors.line }]}>
          <Text style={[typography.label, { color: colors.ink }]}>{tenNguoiKia} đã ghi</Text>
          <Text style={[typography.body, { color: colors.inkSoft }]}>Không ăn được: {nguoiKia.khong_an_duoc || "chưa ghi"}</Text>
          <Text style={[typography.body, { color: colors.inkSoft }]}>Đừng: {nguoiKia.dung || "chưa ghi"}</Text>
        </View>
      </View>
    </Sheet>
  );
}

const styles = StyleSheet.create({ noiDung: { paddingBottom: 8 }, cuaNguoiKia: { borderTopWidth: StyleSheet.hairlineWidth, paddingTop: 12, gap: 4 } });
