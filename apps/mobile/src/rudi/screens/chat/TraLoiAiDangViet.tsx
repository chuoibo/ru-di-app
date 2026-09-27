/**
 * The requester's pending reply under their `@Rủ Đi` message (slice 11):
 * the reading sentence, then the answer's words as the server releases them,
 * then nothing once the published card is in the thread. Other members of
 * the room see the same row, fed by the change feed's `ai` frames instead of
 * the invocation's stream (slice 12, `HangTraLoiAiDangViet` with
 * `nguoiXem="thanh_vien"`): the same state machine, the same Reduce Motion,
 * only the line under the title says whose message it answers.
 *
 * Drawn like `TraLoiAi` so the hand-over does not jump: the quote of the
 * mention on top, a left-aligned paper bubble, the sparkle in the `ai` tone
 * at the foot. No Nếp face on a group answer (ADR-0036 §2.6), no new colour,
 * no animation (a moving indicator waits for `/impeccable`, contract §7);
 * under Reduce Motion the words arrive a sentence at a time.
 */
import { Ionicons } from "@expo/vector-icons";
import { useEffect } from "react";
import { StyleSheet, Text, View } from "react-native";

import { chuHienThi, type TraLoiSong } from "../../ai/tra-loi-song";
import { useAiStream } from "../../ai/useAiStream";
import { hangTraLoiSong, type AiInvocation, type DocMotLoiGoi, type NguoiXem } from "../../chat/ai-invocations";
import type { Tin } from "../../chat/tin-song";
import { typography, useRudiTheme } from "../../theme";

/** How often the fallback re-reads the list the hook keeps (it polls every 2 s itself). */
const NHIP_DOC_DANH_SACH_MS = 2_000;
/** A reply that has not ended after this long stops following; the polled row stays the truth. */
const CHO_TOI_DA_NHOM_MS = 180_000;

export function TraLoiAiDangViet({ request, contextId, personId, trigger, tenNguoi, daCoThe, docMot, khiKetThuc, giamChuyenDong }: {
  request: AiInvocation;
  contextId: string;
  personId: string;
  /** The `@Rủ Đi` message, when it is loaded in the thread. */
  trigger: Tin | null;
  tenNguoi: (id: string | null) => string;
  daCoThe: (messageId: string) => boolean;
  docMot: DocMotLoiGoi;
  /** The stream ended: read the invocation list now so its row catches up. */
  khiKetThuc: () => void;
  giamChuyenDong: boolean;
}) {
  const { traLoi } = useAiStream({
    id: request.id,
    actorId: personId,
    phamVi: { kieu: "nhom", contextId },
    hoi: docMot,
    nhipHoi: () => NHIP_DOC_DANH_SACH_MS,
    choToiDaMs: CHO_TOI_DA_NHOM_MS,
  });
  const ketThuc = traLoi.pha !== "dang_nghi" && traLoi.pha !== "dang_viet";
  useEffect(() => {
    if (ketThuc) khiKetThuc();
  }, [ketThuc, khiKetThuc]);
  return (
    <HangTraLoiAiDangViet
      daCoThe={daCoThe}
      giamChuyenDong={giamChuyenDong}
      request={request}
      tenNguoi={tenNguoi}
      traLoi={traLoi}
      trigger={trigger}
    />
  );
}

/**
 * The row itself, for a given answer state: the requester's (their stream)
 * or another member's (the room's frames); the lab board draws it without a
 * server.
 */
export function HangTraLoiAiDangViet({ request, traLoi, trigger, tenNguoi, daCoThe, giamChuyenDong, nguoiXem = "nguoi_hoi" }: {
  request: AiInvocation;
  traLoi: TraLoiSong;
  trigger: Tin | null;
  tenNguoi: (id: string | null) => string;
  daCoThe: (messageId: string) => boolean;
  giamChuyenDong: boolean;
  nguoiXem?: NguoiXem;
}) {
  const { colors } = useRudiTheme();
  const hang = hangTraLoiSong(request, traLoi, chuHienThi(traLoi, giamChuyenDong), daCoThe, nguoiXem);
  if (hang.kieu === "an") return null;
  return (
    <View style={styles.hang} testID={`${nguoiXem === "thanh_vien" ? "chat-tra-loi-phong" : "chat-tra-loi-song"}-${request.id}`}>
      <View style={styles.khoi}>
        {trigger?.body ? (
          <View
            accessibilityLabel={`Trả lời ${tenNguoi(trigger.author_id)}: ${trigger.body}`}
            style={[styles.trich, { borderLeftColor: colors.ai, backgroundColor: colors.card, borderColor: colors.line }]}
          >
            <Text style={[typography.caption, { color: colors.inkSoft }]}>{tenNguoi(trigger.author_id)}</Text>
            <Text numberOfLines={2} style={[typography.caption, { color: colors.ink }]}>{trigger.body}</Text>
          </View>
        ) : null}
        {hang.kieu === "viet" ? (
          <View style={[styles.bong, { backgroundColor: colors.card, borderColor: colors.line }]}>
            <Text style={[typography.body, { color: colors.ink }]} testID="chat-tra-loi-dang-viet">{hang.chu}</Text>
          </View>
        ) : (
          <View style={[styles.bong, { backgroundColor: colors.card, borderColor: colors.line }]}>
            <Text style={[typography.label, { color: colors.ink }]} testID="chat-tra-loi-dang-doc">{hang.tieuDe}</Text>
            <Text style={[typography.caption, { color: colors.inkSoft }]}>{hang.cau}</Text>
          </View>
        )}
        <View style={styles.chuKy}>
          <Ionicons color={colors.ai} name="sparkles" size={15} />
          <Text style={[typography.caption, { color: colors.ai }]}>Rủ Đi AI</Text>
        </View>
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  hang: { flexDirection: "row", paddingVertical: 4 },
  khoi: { gap: 6, maxWidth: "88%" },
  trich: { borderWidth: 1, borderLeftWidth: 3, borderRadius: 10, paddingHorizontal: 10, paddingVertical: 6, gap: 1 },
  bong: { borderWidth: 1, borderRadius: 18, borderTopLeftRadius: 6, paddingHorizontal: 14, paddingVertical: 10, gap: 2 },
  chuKy: { flexDirection: "row", alignItems: "center", gap: 6 },
});
