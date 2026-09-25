/**
 * The group AI's answer inside the thread (ADR-0039, proposed): a reply to
 * the `@Rủ Đi` message, drawn like a member's reply rather than a card on its
 * own.
 *
 * The quote of the mention sits on top, the words are a left-aligned bubble,
 * a place list or an itinerary is the same sheet of paper `TheAiView` draws
 * (a sibling, never a card inside a card), and the answer is signed at the
 * foot with the sparkle in the `ai` tone -- «Rủ Đi AI · đọc {n} tin» or «Rủ Đi
 * AI · chỉ đọc lời nhờ», from the count the server confirmed. No Nếp face on a
 * group answer (ADR-0036 §2.6), no new colour: tokens only. The finished look
 * is a later slice.
 */
import { Ionicons } from "@expo/vector-icons";
import { Pressable, StyleSheet, Text, View } from "react-native";

import { chuKyTraLoi, type TheAi, type Tin } from "../../chat/tin-song";
import { typography, useRudiTheme } from "../../theme";
import { TheAiView } from "./TheAi";

export function TraLoiAi({ tin, the, contextId, personId, tenNguoi, onOpenPlan, onMenu }: {
  tin: Tin;
  the: Extract<TheAi, { loai: "tra_loi" }>;
  contextId: string;
  personId: string;
  tenNguoi: (id: string | null) => string;
  /** Opens the itinerary part the way any tờ hẹn opens. */
  onOpenPlan: () => void;
  /** The message menu: react, and «Trả lời» to ask a follow-up. */
  onMenu: () => void;
}) {
  const { colors } = useRudiTheme();
  return (
    <View style={styles.khoi} testID={`chat-tra-loi-${tin.id}`}>
      {tin.reply_to ? (
        <View
          accessibilityLabel={`Trả lời ${tenNguoi(tin.reply_to.author_id)}: ${tin.reply_to.preview}`}
          style={[styles.trich, { borderLeftColor: colors.ai, backgroundColor: colors.card, borderColor: colors.line }]}
        >
          <Text style={[typography.caption, { color: colors.inkSoft }]}>{tenNguoi(tin.reply_to.author_id)}</Text>
          <Text numberOfLines={2} style={[typography.caption, { color: colors.ink }]}>{tin.reply_to.preview}</Text>
        </View>
      ) : null}
      {the.phan.map((phan, i) =>
        phan.loai === "text" ? (
          <Pressable
            accessibilityActions={[{ name: "activate", label: "Tuỳ chọn tin nhắn" }]}
            accessibilityLabel={`Rủ Đi AI trả lời: ${phan.text}`}
            accessibilityRole="button"
            key={`text-${i}`}
            onAccessibilityAction={onMenu}
            onLongPress={onMenu}
            onPress={onMenu}
            style={[styles.bong, { backgroundColor: colors.card, borderColor: colors.line }]}
          >
            <Text style={[typography.body, { color: colors.ink }]}>{phan.text}</Text>
          </Pressable>
        ) : (
          <TheAiView
            contextId={contextId}
            key={`${phan.loai}-${i}`}
            onOpenPlan={phan.loai === "itinerary" ? onOpenPlan : undefined}
            personId={personId}
            tacGia="Rủ Đi AI"
            tenNguoi={tenNguoi}
            the={phan}
          />
        ),
      )}
      <View style={styles.chuKy}>
        <Ionicons color={colors.ai} name="sparkles" size={15} />
        <Text style={[typography.caption, { color: colors.ai }]} testID="chat-tra-loi-chu-ky">{chuKyTraLoi(the)}</Text>
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  khoi: { gap: 6, maxWidth: "88%" },
  trich: { borderWidth: 1, borderLeftWidth: 3, borderRadius: 10, paddingHorizontal: 10, paddingVertical: 6, gap: 1 },
  bong: { borderWidth: 1, borderRadius: 18, borderTopLeftRadius: 6, paddingHorizontal: 14, paddingVertical: 10 },
  chuKy: { flexDirection: "row", alignItems: "center", gap: 6 },
});
