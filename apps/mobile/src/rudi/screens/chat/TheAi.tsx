/**
 * Server cards inside the chat (M3): text, places, itinerary, poll, expense draft.
 *
 * Every card is read through `docTheAi`, which trusts nothing about the shape.
 * The poll card carries only ids and labels; counts and «phiếu của tôi» come
 * from `GET /votes/{id}` and a tap goes to `POST /votes/{id}/ballots` -- the
 * vote table is the truth, never a card. An expense draft is shown as exactly
 * that: numbers the server read from chat, marked as needing review, with no
 * button that could turn them into a ledger entry from here (M5 owns that).
 *
 * ## A proposal is an object in the thread, not an advert (UI v2, đợt 5)
 *
 * The first cut drew every card violet-on-violet. Here a card is a sheet of
 * paper laid into the conversation: one small violet line says who wrote it
 * («Rủ Đi AI gợi ý»), the content is ordinary ink, an itinerary is drawn with
 * the same pencil route the plan uses (`HangChang`, dashed = draft), a poll
 * is a list of choices with a radio, and a bill draft prints its sums as
 * `Money` in the money tone.
 */
import { Ionicons } from "@expo/vector-icons";
import { useCallback, useEffect, useState } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";
import { cauBiCat } from "../../../screens/chat/ke-hoach";

import { ApiError, boPhieu, docBinhChon, thongDiepNguoiDoc, type CuocBinhChonWire } from "../../../api";
import { moTaDiaDiem, type TheAi } from "../../chat/tin-song";
import { typography, useRudiTheme, type RudiTone } from "../../theme";
import { Money } from "../../ui/Money";
import { HangChang } from "../keo/HangChang";

/**
 * The sheet of paper every card is drawn on, signed at the foot.
 *
 * The author mark used to sit above the content; over an itinerary's title
 * that made it a kicker over a heading, which the craft floor bans outright.
 * The heading now speaks first and the sheet is signed underneath, the way a
 * note in a journal is.
 */
function ToGiay({ nhan, tone = "ai", children }: { nhan: string; tone?: RudiTone; children: React.ReactNode }) {
  const { colors, radius } = useRudiTheme();
  const muc = colors[tone];
  return (
    <View style={[styles.card, { backgroundColor: colors.card, borderColor: colors.line, borderRadius: radius.base }]}>
      {children}
      <View style={styles.chuKy}>
        <Ionicons color={muc} name={tone === "split" ? "receipt-outline" : "sparkles"} size={15} />
        <Text style={[typography.caption, { color: muc }]}>{nhan}</Text>
      </View>
    </View>
  );
}

export function TheAiView({
  the,
  contextId,
  personId,
  tenNguoi,
  tacGia,
}: {
  the: TheAi;
  contextId: string;
  personId: string;
  tenNguoi: (id: string | null) => string;
  /** Who the card speaks for on the poll's author line (roster name or «Bạn»). */
  tacGia: string;
}) {
  const { colors } = useRudiTheme();
  switch (the.loai) {
    case "text":
      return (
        <ToGiay nhan="Rủ Đi AI">
          <Text style={[typography.body, { color: colors.ink }]}>{the.text}</Text>
        </ToGiay>
      );
    case "places":
      return (
        <ToGiay nhan="Rủ Đi AI gợi ý">
          {the.the.intro ? <Text style={[typography.body, { color: colors.ink }]}>{the.the.intro}</Text> : null}
          {the.the.diaDiem.map((d, i) => (
            <View key={d.id} style={[styles.dong, i > 0 && { borderTopColor: colors.line, borderTopWidth: StyleSheet.hairlineWidth }]}>
              <Text style={[typography.label, { color: colors.ink }]}>{d.ten}</Text>
              <Text style={[typography.caption, { color: colors.inkSoft }]}>{moTaDiaDiem(d)}</Text>
            </View>
          ))}
          {the.the.soChoBiCat !== undefined ? (
            <Text style={[typography.caption, { color: colors.inkSoft }]}>{cauBiCat(the.the.soChoBiCat, "chỗ")[0]}</Text>
          ) : null}
        </ToGiay>
      );
    case "itinerary":
      return (
        <ToGiay nhan="Rủ Đi AI phác lịch trình">
          {/* A message-sized heading: `h2` is the screen's voice, not a card's in a thread (re-audit 10/09, R5). */}
          <Text style={[typography.title, { color: colors.ink }]}>{the.the.tieuDe}</Text>
          <View style={styles.duong}>
            {the.the.chang.map((c, i) => (
              <HangChang
                cuoi={i === the.the.chang.length - 1}
                gio={c.gio}
                key={`${c.diaDiem.id}-${i}`}
                phac
                phu={c.ghiChu ?? null}
                tieuDe={c.diaDiem.ten}
              />
            ))}
          </View>
          {the.the.soChangBiCat !== undefined ? (
            <Text style={[typography.caption, { color: colors.inkSoft }]}>{cauBiCat(the.the.soChangBiCat, "chặng")[0]}</Text>
          ) : null}
          <Text style={[typography.caption, { color: colors.inkSoft }]}>
            Nét chì là bản nháp của AI. Nhóm sửa được trước khi chốt; không gì ở đây tự thành kèo.
          </Text>
        </ToGiay>
      );
    case "poll":
      return <ThePoll the={the} contextId={contextId} personId={personId} tacGia={tacGia} />;
    case "expense_draft":
      return (
        <ToGiay nhan="Nháp chia bill từ chat" tone="split">
          {the.drafts.map((d, i) => (
            <View key={`${d.title}-${i}`} style={[styles.dong, i > 0 && { borderTopColor: colors.line, borderTopWidth: StyleSheet.hairlineWidth }]}>
              <View style={styles.hangTien}>
                <Text style={[typography.body, styles.flex, { color: colors.ink }]}>{d.title}</Text>
                <Money size="label" tone="split" vnd={d.amount_vnd} />
              </View>
              <Text style={[typography.caption, { color: colors.inkSoft }]}>
                {tenNguoi(d.paid_by_id)} trả · chia cho {d.shared_by.length} người
                {d.needs_review ? " · cần xem lại" : ""}
              </Text>
            </View>
          ))}
          <Text style={[typography.caption, { color: colors.inkFaint }]}>
            Đây là bản đọc từ tin nhắn, chưa ghi vào sổ. Xác nhận khoản chi ở mục Chia bill.
          </Text>
        </ToGiay>
      );
    default:
      return (
        <ToGiay nhan="Rủ Đi AI">
          <Text style={[typography.caption, { color: colors.inkFaint }]}>Một thẻ bản này chưa hiển thị được.</Text>
        </ToGiay>
      );
  }
}

function ThePoll({
  the,
  contextId,
  personId,
  tacGia,
}: {
  the: Extract<TheAi, { loai: "poll" }>;
  contextId: string;
  personId: string;
  tacGia: string;
}) {
  const { colors, radius } = useRudiTheme();
  const [ketQua, setKetQua] = useState<CuocBinhChonWire | null>(null);
  const [loi, setLoi] = useState<string | null>(null);
  const [dangBo, setDangBo] = useState<string | null>(null);

  const nap = useCallback(async () => {
    try {
      setKetQua(await docBinhChon(the.vote_id, personId, contextId));
    } catch (error) {
      setLoi(error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null));
    }
  }, [the.vote_id, personId, contextId]);

  useEffect(() => {
    void nap();
  }, [nap]);

  const bo = async (optionId: string) => {
    setDangBo(optionId);
    try {
      await boPhieu(the.vote_id, optionId, personId, contextId);
      await nap();
    } catch (error) {
      setLoi(error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null));
    } finally {
      setDangBo(null);
    }
  };

  const dem = new Map<string, number>();
  for (const o of ketQua?.options ?? []) dem.set(o.id, o.ballot_count);
  const tong = ketQua?.total_ballots ?? 0;
  const dong = ketQua?.is_closed === true;

  return (
    <View style={[styles.card, { backgroundColor: colors.card, borderColor: colors.line, borderRadius: radius.base }]}>
      {/* The poll question is the sheet's title at message size, like the itinerary heading (R5). */}
      <Text style={[typography.title, { color: colors.ink }]}>{the.question}</Text>
      {the.options.map((o) => {
        const cuaToi = ketQua?.my_option_id === o.id;
        const so = dem.get(o.id) ?? 0;
        return (
          <Pressable
            accessibilityRole="radio"
            accessibilityState={{ checked: cuaToi, disabled: dangBo !== null || dong }}
            accessibilityLabel={`Bỏ phiếu ${o.label}`}
            disabled={dangBo !== null || dong}
            key={o.id}
            onPress={() => void bo(o.id)}
            style={({ pressed }) => [
              styles.luaChon,
              { borderColor: cuaToi ? colors.accent : colors.lineStrong, backgroundColor: cuaToi ? colors.accentSoft : colors.card, borderRadius: radius.control },
              pressed && styles.bam,
            ]}
          >
            <Ionicons color={cuaToi ? colors.accent : colors.lineStrong} name={cuaToi ? "checkmark-circle" : "ellipse-outline"} size={22} />
            <View style={styles.flex}>
              <Text style={[typography.body, { color: colors.ink }]}>{o.label}</Text>
              <Text style={[typography.caption, { color: cuaToi ? colors.accent : colors.inkSoft }]}>
                {so} phiếu{cuaToi ? " · của bạn" : ""}
              </Text>
            </View>
          </Pressable>
        );
      })}
      <Text style={[typography.caption, { color: colors.inkFaint }]}>
        {tong} phiếu{dong ? " · đã đóng" : ""}
      </Text>
      {/* Signed at the foot like every sheet: the question is the heading, not a label over it. */}
      <View style={styles.chuKy}>
        <Ionicons color={colors.accent} name="stats-chart-outline" size={15} />
        <Text style={[typography.caption, { color: colors.inkSoft }]}>{tacGia} tạo bình chọn</Text>
      </View>
      {loi ? <Text accessibilityLiveRegion="polite" style={[typography.caption, { color: colors.warn }]}>{loi}</Text> : null}
    </View>
  );
}

const styles = StyleSheet.create({
  flex: { flex: 1 },
  card: { gap: 8, padding: 14, borderWidth: 1 },
  chuKy: { flexDirection: "row", alignItems: "center", justifyContent: "flex-end", gap: 6, paddingTop: 2 },
  dong: { gap: 2, paddingVertical: 6 },
  duong: { paddingTop: 4 },
  hangTien: { flexDirection: "row", alignItems: "center", gap: 10 },
  luaChon: { flexDirection: "row", alignItems: "center", gap: 10, borderWidth: 1.5, paddingHorizontal: 12, paddingVertical: 10, minHeight: 56 },
  bam: { opacity: 0.8 },
});
