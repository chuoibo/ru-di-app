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
import { useCallback, useEffect, useRef, useState } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";
import { cauBiCat } from "../../../screens/chat/ke-hoach";

import { ApiError, boPhieu, docBinhChon, dongBinhChon, thongDiepNguoiDoc, type CuocBinhChonWire } from "../../../api";
import { moTaDiaDiem, type TheAi } from "../../chat/tin-song";
import { typography, useRudiTheme, type RudiTone } from "../../theme";
import { Money } from "../../ui/Money";
import { HangChang } from "../keo/HangChang";
import type { BinhChonSong } from "../../chat/thay-doi";
import { RudiButton } from "../../ui";

/**
 * The sheet of paper every card is drawn on, signed at the foot.
 *
 * The author mark used to sit above the content; over an itinerary's title
 * that made it a kicker over a heading, which the craft floor bans outright.
 * The heading now speaks first and the sheet is signed underneath, the way a
 * note in a journal is.
 */
function ToGiay({ nhan, tone = "ai", bieuTuong, children }: { nhan: string; tone?: RudiTone; bieuTuong?: keyof typeof Ionicons.glyphMap; children: React.ReactNode }) {
  const { colors } = useRudiTheme();
  const muc = colors[tone];
  return (
    <View style={[styles.card, { backgroundColor: colors.card, borderColor: colors.line, borderRadius: 4, borderTopRightRadius: 18 }]}>
      {children}
      <View style={styles.chuKy}>
        {/* The signature says who wrote the sheet. A sparkle means the model
            suggested it; the group's own sheet must not borrow that mark. */}
        <Ionicons color={muc} name={bieuTuong ?? (tone === "split" ? "receipt-outline" : "sparkles")} size={15} />
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
  vote,
  onOpenPlan,
  onMoToHen,
  daCoToHen,
  tenToHen,
  banToHen,
}: {
  the: TheAi;
  contextId: string;
  personId: string;
  tenNguoi: (id: string | null) => string;
  /** Who the card speaks for on the poll's author line (roster name or «Bạn»). */
  tacGia: string;
  vote?: BinhChonSong;
  onOpenPlan?: () => void;
  /** Opens the group's shared sheet from a decided poll. */
  onMoToHen?: (voteId: string, goiY: string) => void;
  /** True when this poll already feeds an open sheet. */
  daCoToHen?: boolean;
  /** That sheet's title and revision, so the card can name what it feeds. */
  tenToHen?: string | null;
  banToHen?: number | null;
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
    case "itinerary": {
      // Same card shape, two different objects: an AI suggestion the group
      // accepts, and the group's own sheet they are still writing. The label
      // and the action have to say which one this is, or "sửa" means nothing.
      const nhap = the.nhapChung;
      const daThanhKeoTruoc = !!the.outingId || nhap?.status === "promoted";
      const dangMo = nhap?.status === "open" && !daThanhKeoTruoc;
      // One source for "is this already a kèo". The card must not describe a
      // state in its status line that its own button disagrees with.
      const daThanhKeo = daThanhKeoTruoc;
      return (
        <ToGiay
          bieuTuong={nhap ? "people-outline" : undefined}
          nhan={nhap ? (dangMo ? "Tờ hẹn chung của hội" : "Tờ hẹn chung đã chốt") : "Rủ Đi AI phác lịch trình"}
          tone={nhap ? "accent" : "ai"}
        >
          {/* A message-sized heading: `h2` is the screen's voice, not a card's in a thread (re-audit 10/09, R5). */}
          <Text style={[typography.title, { color: colors.ink }]}>{the.the.tieuDe}</Text>
          {daThanhKeo && !nhap ? <Text style={[typography.caption, { color: colors.inkSoft }]}>{the.the.chang.length} chặng · đã thành kèo</Text> : <View style={styles.duong}>
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
          </View>}
          {the.the.soChangBiCat !== undefined ? (
            <Text style={[typography.caption, { color: colors.inkSoft }]}>{cauBiCat(the.the.soChangBiCat, "chặng")[0]}</Text>
          ) : null}
          {nhap ? (
            <Text style={[typography.caption, { color: colors.inkSoft }]}>
              {dangMo
                ? `Cả hội sửa được tờ này · bản ${nhap.revision}`
                : daThanhKeo
                  ? `${the.the.chang.length} chặng · đã thành kèo`
                  : "Tờ này đã được bỏ."}
            </Text>
          ) : null}
          {!daThanhKeo && !nhap ? <Text style={[typography.caption, { color: colors.inkSoft }]}>Xem lại ngày, ngân sách và chặng trước khi tạo kèo.</Text> : null}
          {onOpenPlan ? <RudiButton label={daThanhKeo ? "Mở kèo của hội" : dangMo ? "Sửa cùng hội" : "Sửa tờ hẹn này"} variant="outline" onPress={onOpenPlan} /> : null}
        </ToGiay>
      );
    }
    case "poll":
      return <ThePoll the={the} contextId={contextId} personId={personId} tacGia={tacGia} live={vote} onMoToHen={onMoToHen} daCoToHen={daCoToHen} tenToHen={tenToHen} banToHen={banToHen} />;
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
  live,
  onMoToHen,
  daCoToHen,
  tenToHen,
  banToHen,
}: {
  the: Extract<TheAi, { loai: "poll" }>;
  contextId: string;
  personId: string;
  tacGia: string;
  live?: BinhChonSong;
  onMoToHen?: (voteId: string, goiY: string) => void;
  daCoToHen?: boolean;
  tenToHen?: string | null;
  banToHen?: number | null;
}) {
  const { colors } = useRudiTheme();
  const [ketQua, setKetQua] = useState<CuocBinhChonWire | null>(null);
  const [loi, setLoi] = useState<string | null>(null);
  const [dangBo, setDangBo] = useState<string | null>(null);
  const [confirmClose, setConfirmClose] = useState(false);
  const [showClosedVotes, setShowClosedVotes] = useState(false);
  const mounted = useRef(true);
  const readVersion = useRef(0);
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; readVersion.current += 1; }; }, [the.vote_id, contextId, personId]);

  useEffect(() => {
    if (live) {
      readVersion.current += 1;
      setKetQua(live);
      setLoi(null);
    }
  }, [live]);

  const nap = useCallback(async () => {
    const version = ++readVersion.current;
    try {
      const response = await docBinhChon(the.vote_id, personId, contextId);
      if (mounted.current && version === readVersion.current) { setKetQua(response); setLoi(null); }
    } catch (error) {
      if (mounted.current && version === readVersion.current) setLoi(error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null));
    }
  }, [the.vote_id, personId, contextId]);

  useEffect(() => {
    if (!live) void nap();
  }, [nap]);

  const bo = async (optionId: string) => {
    setDangBo(optionId);
    try {
      await boPhieu(the.vote_id, optionId, personId, contextId);
      await nap();
    } catch (error) {
      if (mounted.current) setLoi(error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null));
    } finally {
      if (mounted.current) setDangBo(null);
    }
  };

  const closePoll = async () => {
    setDangBo("close");
    try { await dongBinhChon(the.vote_id, personId, contextId); await nap(); if (mounted.current) setConfirmClose(false); }
    catch (error) { if (mounted.current) setLoi(error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null)); }
    finally { if (mounted.current) setDangBo(null); }
  };

  const dem = new Map<string, number>();
  for (const o of ketQua?.options ?? []) dem.set(o.id, o.ballot_count);
  const tong = ketQua?.total_ballots ?? 0;
  const dong = ketQua?.is_closed === true;
  const highest = Math.max(0, ...dem.values());
  const leading = the.options.filter((option) => dem.get(option.id) === highest);
  const summary = highest === 0 ? "Chưa có phiếu" : leading.length === 1 ? `${leading[0].label} · ${highest} phiếu` : `${leading.length} lựa chọn ngang phiếu`;

  if (live?.deleted) return <View style={[styles.card, { borderColor: colors.line, backgroundColor: colors.card }]}><Text style={[typography.caption, { color: colors.inkSoft }]}>Bình chọn này không còn.</Text></View>;

  return (
    <View style={[styles.card, { backgroundColor: colors.card, borderColor: colors.line, borderRadius: 4, borderTopRightRadius: 18 }]}>
      {/* The poll question is the sheet's title at message size, like the itinerary heading (R5). */}
      <Text style={[typography.title, { color: colors.ink }]}>{the.question}</Text>
      {dong ? <View style={styles.summary}>
        <Ionicons name="checkmark-circle-outline" size={20} color={colors.accent} />
        <Text style={[typography.label, styles.flex, { color: colors.ink }]}>{summary}</Text>
      </View> : null}
      {(!dong || showClosedVotes) ? the.options.map((o) => {
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
            // Each choice is a sticky note; every ballot is an ink thumbprint
            // on it, so the count is seen before it is read (ADR-0037 D1).
            style={({ pressed }) => [
              styles.luaChon,
              styles.giayNho,
              { borderColor: cuaToi ? colors.accent : colors.lineStrong, borderWidth: cuaToi ? 2 : 1, backgroundColor: cuaToi ? colors.accentSoft : colors.card },
              pressed && styles.bam,
            ]}
          >
            <View style={styles.flex}>
              <Text style={[typography.body, { color: colors.ink }]}>{o.label}</Text>
              {so > 0 ? (
                <View accessibilityElementsHidden importantForAccessibility="no-hide-descendants" style={styles.vanTay}>
                  {Array.from({ length: Math.min(so, 12) }, (_, i) => (
                    <View key={i} style={[styles.dauVanTay, { backgroundColor: colors.ink, opacity: 0.72, transform: [{ rotate: `${(i * 37) % 60 - 30}deg` }] }]} />
                  ))}
                  {so > 12 ? <Text style={[typography.caption, { color: colors.inkSoft }]}>+{so - 12}</Text> : null}
                </View>
              ) : null}
              <Text style={[typography.caption, { color: cuaToi ? colors.accent : colors.inkSoft }]}>
                {dangBo === o.id ? "Đang gửi phiếu…" : `${so} phiếu${cuaToi ? " · của bạn" : ""}`}
              </Text>
            </View>
            {cuaToi ? <Ionicons color={colors.accent} name="checkmark-circle" size={22} /> : null}
          </Pressable>
        );
      }) : null}
      <Text style={[typography.caption, { color: colors.inkSoft }]}>
        {tong} phiếu{dong ? " · đã đóng" : ""}
      </Text>
      {dong ? <RudiButton label={showClosedVotes ? "Thu gọn phiếu" : "Xem các phiếu"} variant="ghost" compact onPress={() => setShowClosedVotes((value) => !value)} /> : null}
      {/* Where a decision turns into a plan. Without this the vote ends and the
          group is back to one person filling a private form (reviewer C1/C6). */}
      {dong && onMoToHen ? (
        <View style={styles.tiepTheo}>
          <Text style={[typography.caption, { color: colors.inkSoft }]}>
            {tenToHen
              ? `Lựa chọn này đang ở tờ «${tenToHen}»${banToHen ? ` · bản ${banToHen}` : ""}.`
              : "Đưa lựa chọn này vào một tờ hẹn cả hội sửa được."}
          </Text>
          <RudiButton
            label={daCoToHen ? "Mở tờ hẹn chung" : "Mở tờ hẹn chung cho lựa chọn này"}
            variant="outline"
            onPress={() => onMoToHen(the.vote_id, leading.length === 1 ? leading[0].label : the.question)}
          />
        </View>
      ) : null}
      {/* Signed at the foot like every sheet: the question is the heading, not a label over it. */}
      <View style={styles.chuKy}>
        <Ionicons color={colors.accent} name="stats-chart-outline" size={15} />
        <Text style={[typography.caption, { color: colors.inkSoft }]}>{tacGia} tạo bình chọn</Text>
      </View>
      {!dong && ketQua?.created_by_id === personId ? (
        confirmClose ? <View style={styles.dong}>
          <Text style={[typography.caption, { color: colors.inkSoft }]}>Sau khi đóng, mọi người không thể đổi phiếu.</Text>
          <RudiButton label="Đóng bình chọn" compact variant="outline" loading={dangBo === "close"} disabled={dangBo !== null} onPress={() => void closePoll()} />
          <RudiButton label="Tiếp tục bình chọn" compact variant="ghost" disabled={dangBo !== null} onPress={() => setConfirmClose(false)} />
        </View> : <RudiButton label="Chốt bình chọn" compact variant="ghost" disabled={dangBo !== null} onPress={() => setConfirmClose(true)} />
      ) : null}
      {loi ? <View style={styles.dong}><Text accessibilityLiveRegion="polite" style={[typography.caption, { color: colors.warn }]}>{loi}</Text><RudiButton label="Tải lại bình chọn" variant="ghost" compact onPress={() => void nap()} /></View> : null}
    </View>
  );
}

const styles = StyleSheet.create({
  giayNho: { borderRadius: 3, borderTopRightRadius: 12 },
  vanTay: { flexDirection: "row", flexWrap: "wrap", alignItems: "center", gap: 4, paddingVertical: 4 },
  dauVanTay: { width: 10, height: 13, borderRadius: 6 },
  flex: { flex: 1 },
  card: { gap: 8, padding: 14, borderWidth: 1 },
  chuKy: { flexDirection: "row", alignItems: "center", justifyContent: "flex-end", gap: 6, paddingTop: 2 },
  dong: { gap: 2, paddingVertical: 6 },
  duong: { paddingTop: 4 },
  hangTien: { flexDirection: "row", alignItems: "center", gap: 10 },
  summary: { flexDirection: "row", alignItems: "center", gap: 8 },
  luaChon: { flexDirection: "row", alignItems: "center", gap: 10, borderBottomWidth: StyleSheet.hairlineWidth, paddingHorizontal: 8, paddingVertical: 8, minHeight: 52 },
  bam: { opacity: 0.8 },
  // The step after a decision sits apart from the ballots, on the 4pt scale.
  tiepTheo: { gap: 6, paddingTop: 6 },
});
