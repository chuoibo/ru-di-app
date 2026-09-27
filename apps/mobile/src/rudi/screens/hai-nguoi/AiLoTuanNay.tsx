import { StyleSheet, Text, View } from "react-native";

import type { ChonLo, VaiTuan } from "../../to-giay/to-giay-song";
import { cauVaiTuan } from "../../to-giay/vai-tuan";
import { typography, useRudiTheme } from "../../theme";
import { Heading, RudiButton } from "../../ui";
import { Sheet } from "../../ui/Sheet";

/**
 * «Ai lo tuần này?» (ADR-0034 §2.4): «Để tôi lo», «Để <tên> lo», «Hôm nay
 * mình share». Without a choice the notebook infers it from what the two did here;
 * choosing only decides whose turn the week reads as -- it grants nothing, and
 * either of the two may change it.
 */
export function AiLoTuanNay({
  open,
  onClose,
  vai,
  toiId,
  tenNguoiKia,
  dangLam,
  onChon,
}: {
  open: boolean;
  onClose: () => void;
  vai: VaiTuan | null;
  toiId: string;
  tenNguoiKia: string;
  dangLam: boolean;
  onChon: (lo: ChonLo) => void;
}) {
  const { colors, space } = useRudiTheme();
  const cau = cauVaiTuan(vai, toiId, tenNguoiKia);
  const dangChon: ChonLo | null =
    !vai || vai.cach !== "chon" ? null : vai.nguoi_lo.length > 1 ? "ca_hai" : vai.nguoi_lo[0] === toiId ? "toi" : "nguoi_kia";
  const nut = (lo: ChonLo, label: string) => (
    <RudiButton
      accessibilityLabel={dangChon === lo ? `${label}, đang chọn` : label}
      disabled={dangLam}
      label={dangChon === lo ? `${label} ✓` : label}
      onPress={() => onChon(lo)}
      variant={dangChon === lo ? "solid" : "outline"}
    />
  );
  return (
    <Sheet accessibilityLabel="Ai lo tuần này" onClose={onClose} open={open} testID="ai-lo-tuan-nay">
      <View style={[styles.noiDung, { gap: space.md }]}>
        <Heading size="h2" subtitle="Chỉ để biết tuần này lượt ai mở lời. Không ai được quyền gì hơn." title="Ai lo tuần này?" />
        {cau ? (
          <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.ink }]} testID="ai-lo-hien-tai">
            {`${cau.nhan}. ${cau.vi}`}
          </Text>
        ) : null}
        {/* «Để tôi lo», not «Mình lo»: beside a partner called Minh the two
            read as one word (seen on device 25/09). */}
        {nut("toi", "Để tôi lo")}
        {nut("nguoi_kia", `Để ${tenNguoiKia} lo`)}
        {nut("ca_hai", "Hôm nay mình share")}
        <RudiButton label="Xong" onPress={onClose} variant="ghost" />
      </View>
    </Sheet>
  );
}

const styles = StyleSheet.create({ noiDung: { paddingBottom: 8 } });
