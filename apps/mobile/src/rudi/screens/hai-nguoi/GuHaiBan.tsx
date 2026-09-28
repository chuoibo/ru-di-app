import { StyleSheet, Text, View } from "react-native";

import { CAU_BAT_LAI_CHO_CHAT, type GuChat, type GuSo, canBatLaiChoChat, cauBatGu, cauGu } from "../../to-giay/gu-doi";
import { typography, useRudiTheme } from "../../theme";
import { Heading, ListRow, RudiButton } from "../../ui";
import { Sheet } from "../../ui/Sheet";

/**
 * «Gu của hai bạn» (ADR-0034 §2.1–2.2): each person's own switch.
 *
 * Turning it on lets the other person see MY taste and lets Nếp use it in this
 * notebook and Rủ Đi AI use it in the two's chat (ADR-0048); it never pulls the
 * other person's taste along, and nobody has to agree to it. What the two have
 * in common shows only once both have shared. A switch turned on before the
 * chat was named covers the notebook only: the sheet offers «Bật lại cho chat».
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
  guChat = null,
  onBatLai,
}: {
  open: boolean;
  onClose: () => void;
  gu: GuSo | null;
  tenNguoiKia: string;
  dangLam: boolean;
  onBat: () => void;
  onTat: () => void;
  onSuaGuCuaToi: () => void;
  /** `gu_chat` of chat-capabilities; null when unknown or outside a couple. */
  guChat?: GuChat | null;
  /** Re-consent for the chat: off, then on again (ADR-0048 §3.2). */
  onBatLai?: () => void;
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
        {onBatLai && canBatLaiChoChat(gu, guChat) ? (
          <>
            <Text style={[typography.caption, { color: colors.inkSoft }]} testID="gu-bat-lai-cau">
              {CAU_BAT_LAI_CHO_CHAT}
            </Text>
            <RudiButton disabled={dangLam} label="Bật lại cho chat" loading={dangLam} onPress={onBatLai} />
          </>
        ) : null}
        {gu?.mine_shared ? (
          <RudiButton disabled={dangLam} label={`Thôi cho ${tenNguoiKia} thấy gu của mình`} loading={dangLam} onPress={onTat} variant="outline" />
        ) : (
          <>
            <Text style={[typography.caption, { color: colors.inkSoft }]}>{cauBatGu(gu, tenNguoiKia)}</Text>
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
