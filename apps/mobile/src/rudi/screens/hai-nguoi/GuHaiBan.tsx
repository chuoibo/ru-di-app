import { StyleSheet, Text, View } from "react-native";

import { type GuSo, cauGu } from "../../to-giay/gu-doi";
import { typography, useRudiTheme } from "../../theme";
import { Heading, ListRow, RudiButton } from "../../ui";
import { Sheet } from "../../ui/Sheet";

/**
 * «Gu của hai bạn» (ADR-0034 §2.1–2.2): each person's own switch.
 *
 * Turning it on lets the other person see MY taste and lets Nếp use it in this
 * notebook; it never pulls the other person's taste along, and nobody has to
 * agree to it. What the two have in common shows only once both have shared.
 * The server decides what is visible; this sheet only words it.
 */
export function GuHaiBan({
  open,
  onClose,
  gu,
  tenNguoiKia,
  dangLam,
  onBat,
  onTat,
  onSuaGuCuaToi,
}: {
  open: boolean;
  onClose: () => void;
  gu: GuSo | null;
  tenNguoiKia: string;
  dangLam: boolean;
  onBat: () => void;
  onTat: () => void;
  onSuaGuCuaToi: () => void;
}) {
  const { colors, space } = useRudiTheme();
  const cau = cauGu(gu, tenNguoiKia);
  return (
    <Sheet accessibilityLabel="Gu của hai bạn" onClose={onClose} open={open} testID="gu-hai-ban">
      <View style={[styles.noiDung, { gap: space.md }]}>
        <Heading size="h2" subtitle="Mỗi người tự bật cho riêng mình. Tắt là thôi ngay." title="Gu của hai bạn" />
        {cau?.chung ? (
          <Text accessibilityLiveRegion="polite" style={[typography.title, { color: colors.ink }]} testID="gu-chung">
            {cau.chung}
          </Text>
        ) : null}
        {cau?.cuaHo ? (
          <Text style={[typography.body, { color: colors.ink }]} testID="gu-cua-ho">
            {cau.cuaHo}
          </Text>
        ) : (
          <Text style={[typography.body, { color: colors.inkSoft }]}>{`${tenNguoiKia} chưa chia gu của mình.`}</Text>
        )}
        <Text style={[typography.caption, { color: colors.inkSoft }]} testID="gu-cua-toi">
          {cau?.cuaToi ?? "Gu của bạn đang để riêng."}
        </Text>
        {gu?.mine_shared ? (
          <RudiButton disabled={dangLam} label={`Thôi cho ${tenNguoiKia} thấy gu của mình`} loading={dangLam} onPress={onTat} variant="outline" />
        ) : (
          <>
            <Text style={[typography.caption, { color: colors.inkSoft }]}>
              {gu?.theirs_shared
                ? `Bật thì ${tenNguoiKia} thấy gu của bạn, hai bạn thấy mình cùng thích gì, và Nếp phác tờ theo đó.`
                : `Bật thì ${tenNguoiKia} thấy gu của bạn và Nếp dùng nó khi phác tờ. Gu của ${tenNguoiKia} chỉ hiện khi chính họ bật.`}
            </Text>
            <RudiButton disabled={dangLam} label={`Cho ${tenNguoiKia} thấy gu của mình`} loading={dangLam} onPress={onBat} />
          </>
        )}
        <ListRow icon="options-outline" onPress={onSuaGuCuaToi} subtitle="Chọn lại những gì bạn thích" title="Sửa gu của mình" />
        <RudiButton label="Xong" onPress={onClose} variant="ghost" />
      </View>
    </Sheet>
  );
}

const styles = StyleSheet.create({ noiDung: { paddingBottom: 8 } });
