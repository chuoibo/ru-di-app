import { useEffect, useState } from "react";
import { StyleSheet, Text, View } from "react-native";

import { typography, useRudiTheme } from "../../theme";
import type { RangBuoc as RangBuocKieu } from "../../to-giay/so-fixture";
import { Heading, RudiButton } from "../../ui";
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
    <Sheet accessibilityLabel="Hai ô ràng buộc" onClose={onClose} open={open} testID={testID ?? "rang-buoc"}>
      <View style={[styles.noiDung, { gap: space.md }]}>
        <Heading size="h2" subtitle="Nếp đọc hai ô này trước khi phác. Không cần lý do." title="Hai ô ràng buộc" />
        <Field label="Không ăn được" onChangeText={setKhongAn} placeholder="Hải sản" testID="rang-buoc-khong-an" value={khongAn} />
        <Field label="Đừng" multiline onChangeText={setDung} placeholder="Đừng rủ sau 21:00 ngày thường." testID="rang-buoc-dung" value={dung} />
        <View style={[styles.cuaNguoiKia, { borderTopColor: colors.line }]}>
          <Text style={[typography.label, { color: colors.ink }]}>{tenNguoiKia} đã ghi</Text>
          <Text style={[typography.body, { color: colors.inkSoft }]}>Không ăn được: {nguoiKia.khong_an_duoc || "chưa ghi"}</Text>
          <Text style={[typography.body, { color: colors.inkSoft }]}>Đừng: {nguoiKia.dung || "chưa ghi"}</Text>
        </View>
        {loi ? <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.warn }]}>{loi}</Text> : null}
        <RudiButton disabled={!doi || dangLuu} label="Lưu hai ô của tôi" loading={dangLuu} onPress={() => onLuu({ khong_an_duoc: khongAn.trim(), dung: dung.trim() })} />
        <RudiButton label="Đóng" onPress={onClose} variant="ghost" />
      </View>
    </Sheet>
  );
}

const styles = StyleSheet.create({ noiDung: { paddingBottom: 8 }, cuaNguoiKia: { borderTopWidth: StyleSheet.hairlineWidth, paddingTop: 12, gap: 4 } });
