import { Ionicons } from "@expo/vector-icons";
import { useNhuongChoNep } from "../../nep/NepProvider";
import { useEffect, useRef, useState } from "react";
import { Pressable, ScrollView, StyleSheet, Text, View, useWindowDimensions } from "react-native";
import { lenhSanSang, type ChatCapabilities } from "../../chat/ai-invocations";
import { docBanNhapCongCu, ghiBanNhapCongCu, loiBinhChon, loiBinhChonTheoO, type BanNhapCongCu, type LoiBinhChonTheoO } from "../../chat/ban-nhap-cong-cu";
import { CONG_CU_TO_GIAY, KHOANG_CONG_CU, boCucKhay, chuKhay, tranKhay } from "../../chat/khay-cong-cu";
import { docTheAi, lichTrinhTrongThe, type Tin } from "../../chat/tin-song";
import { KHUNG_VAT, hinhVat, type VatBan } from "../../art/vat-ban";
import { typography, useRudiTheme } from "../../theme";
import { VeLop } from "../../ui/art/VeLop";
import { ONhapMuc } from "../../ui/ONhapMuc";
import { Field, IconButton, RudiButton } from "../../ui";
import { useDongKhay } from "../../ui/useDongKhay";

export type KhayChat = "tools" | "poll" | "plan" | null;

/** The folded margin names the next action; the full text lives in the thread. */
export function ToHen({ tin, onOpen, onVote, haiNguoi = false }: {
  tin: Tin; onOpen: (tin: Tin) => void; onVote: (tin: Tin) => void;
  /** A two-person conversation: the open shared plan is edited «cùng nhau», not «cùng hội». */
  haiNguoi?: boolean;
}) {
  const { colors } = useRudiTheme();
  // A poll, an itinerary card, or the itinerary part of an answer in the thread.
  const doc = docTheAi(tin.card);
  const the = doc.loai === "poll" ? doc : lichTrinhTrongThe(doc);
  if (the === null) return null;
  const poll = the.loai === "poll";
  // The group's own sheet must carry the same name and mark here as it does on
  // the card. Borrowing the AI sheet's words at the top of the viewport is
  // exactly where "which sheet is this?" gets answered wrongly.
  const nhap = the.loai === "itinerary" ? the.nhapChung : undefined;
  const daThanhKeo = the.loai === "itinerary" && (!!the.outingId || nhap?.status === "promoted");
  // Same precedence as the card: a sheet that is already a kèo is never
  // described as still open, whatever its draft block says.
  const nhapDangMo = nhap?.status === "open" && !daThanhKeo;
  const label = poll
    ? "Cùng chọn"
    : daThanhKeo
      ? "Đã thành kèo"
      : nhapDangMo
        ? `Tờ hẹn chung · bản ${nhap.revision}`
        : "Tờ hẹn đang phác";
  const action = poll ? "Xem phiếu" : daThanhKeo ? "Mở lịch trình" : nhapDangMo ? chuKhay(haiNguoi).suaChung : "Sửa tờ hẹn";
  const icon = poll
    ? "stats-chart-outline"
    : daThanhKeo
      ? "checkmark-circle-outline"
      : nhapDangMo
        ? "people-outline"
        : "trail-sign-outline";
  return (
    <Pressable accessibilityRole="button" accessibilityLabel={poll ? `Xem bình chọn: ${the.question}` : daThanhKeo ? "Mở lịch trình đã tạo" : nhapDangMo ? "Mở tờ hẹn chung để sửa" : "Mở tờ hẹn để sửa"}
      onPress={() => poll ? onVote(tin) : onOpen(tin)}
      style={({ pressed }) => [styles.toHen, { borderColor: colors.lineStrong, backgroundColor: colors.card }, pressed && styles.pressed]}>
      <View style={[styles.fold, { borderColor: colors.accent, backgroundColor: colors.ground }]} />
      <Ionicons name={icon} size={21} color={colors.accent} />
      <Text numberOfLines={1} style={[typography.label, styles.flex, { color: colors.ink }]}>{label}</Text>
      <Text style={[typography.caption, { color: colors.inkSoft }]}>{action}</Text>
      <Ionicons name="chevron-forward" size={16} color={colors.inkSoft} />
    </Pressable>
  );
}

/**
 * The group's next outing, pinned where the tờ hẹn is when nothing is being
 * decided. An outing made from the chat with «Tự tạo kèo» has no card in the
 * thread, so the chat showed no trace of it to anybody (QA UI-118). This is
 * the plan tab's own data -- the outing, not a message -- so it reads the same
 * on every member's phone and puts nothing in the conversation.
 */
export function DaiKeoSapToi({ ten, nhip, onOpen, gon = false }: {
  ten: string; nhip: string; onOpen: () => void;
  /** Above an open tờ hẹn or poll: a slim ruled line, so the band grows by one row, not by a second card. */
  gon?: boolean;
}) {
  const { colors } = useRudiTheme();
  if (gon) {
    return (
      <Pressable accessibilityRole="button" accessibilityLabel={`Mở kèo ${ten}${nhip ? `, ${nhip}` : ""}`} onPress={onOpen} testID="dai-keo-sap-toi"
        style={({ pressed }) => [styles.keoGon, pressed && styles.pressed]}>
        <Ionicons name="calendar-outline" size={18} color={colors.accent} />
        <Text numberOfLines={1} style={[typography.label, styles.flex, { color: colors.ink }]}>
          <Text style={[typography.caption, { color: colors.inkSoft }]}>Kèo sắp tới · </Text>
          {ten}
        </Text>
        {nhip ? <Text style={[typography.caption, { color: colors.inkSoft }]}>{nhip}</Text> : null}
        <Ionicons name="chevron-forward" size={16} color={colors.inkSoft} />
      </Pressable>
    );
  }
  return (
    <Pressable accessibilityRole="button" accessibilityLabel={`Mở kèo ${ten}${nhip ? `, ${nhip}` : ""}`} onPress={onOpen} testID="dai-keo-sap-toi"
      style={({ pressed }) => [styles.toHen, { borderColor: colors.lineStrong, backgroundColor: colors.card }, pressed && styles.pressed]}>
      <View style={[styles.fold, { borderColor: colors.accent, backgroundColor: colors.ground }]} />
      <Ionicons name="calendar-outline" size={21} color={colors.accent} />
      <View style={styles.flex}>
        <Text style={[typography.caption, { color: colors.inkSoft }]}>Kèo sắp tới</Text>
        <Text numberOfLines={1} style={[typography.label, { color: colors.ink }]}>{ten}</Text>
      </View>
      {nhip ? <Text style={[typography.caption, { color: colors.inkSoft }]}>{nhip}</Text> : null}
      <Ionicons name="chevron-forward" size={16} color={colors.inkSoft} />
    </Pressable>
  );
}

export function CongCuChat({ personId, contextId, panel, onPanel, onImage, onSticker, onPoll, onHoiAi, onManual, capabilities, busy, error, haiNguoi = false, onToGiay }: {
  personId: string; contextId: string;
  /** A two-person conversation: the same tools, worded for two. */
  haiNguoi?: boolean;
  /**
   * A couple only (`cap_doi`): the tray adds «Tờ giấy», the pair's paper,
   * beside «Tờ hẹn». Absent in a group and in a friends' two-person chat.
   */
  onToGiay?: () => void;
  panel: KhayChat; onPanel: (panel: KhayChat) => void; onImage: () => void; onSticker: () => void;
  onPoll: (command: string) => Promise<boolean>; onManual: () => void;
  /**
   * «Hỏi Rủ Đi AI»: the tray no longer sends to the AI itself. Since ADR-0046
   * an AI request is an ordinary `@Rủ Đi` message, written in the composer
   * with the preview chip above its send button, so this puts `/plan ` there
   * (`MO_DAU_HOI_AI`), in every room.
   */
  onHoiAi: () => void;
  capabilities: ChatCapabilities | null; busy: boolean; error: string | null;
}) {
  const { colors } = useRudiTheme();
  const chu = chuKhay(haiNguoi);
  // Every room's tray offers a plan, so it reads the plan's readiness.
  const sanSang = lenhSanSang(capabilities, "plan");
  // The tray is a sheet laid over the conversation; Nếp makes room for it.
  useNhuongChoNep(panel !== null);
  const { height, width, fontScale } = useWindowDimensions();
  // The tool row's real width once laid out; before that, the window's less
  // the tray's side padding (the chat column is at most 820 wide).
  const [rongHang, setRongHang] = useState<number | null>(null);
  // The photo note's own height, so the tray's cap counts it with the grid.
  const [caoGhiChu, setCaoGhiChu] = useState(0);
  const [draft, setDraft] = useState(() => docBanNhapCongCu(personId, contextId));
  const held = useRef(draft);
  const [restored, setRestored] = useState(() => ({ poll: !!draft.question || draft.choices.some(Boolean) }));
  const [pollError, setPollError] = useState<string | null>(null);
  const [fieldErrors, setFieldErrors] = useState<LoiBinhChonTheoO | null>(null);
  const [undo, setUndo] = useState<{ panel: "poll"; value: Partial<BanNhapCongCu> } | null>(null);
  const onPanelRef = useRef(onPanel);
  onPanelRef.current = onPanel;
  const update = (patch: Partial<BanNhapCongCu>) => {
    const next = { ...held.current, ...patch };
    held.current = next;
    ghiBanNhapCongCu(personId, contextId, next);
    setDraft(next);
    if (pollError && (patch.question !== undefined || patch.choices !== undefined)) {
      setPollError(loiBinhChon(next.question, next.choices));
      setFieldErrors(loiBinhChonTheoO(next.question, next.choices));
    }
  };
  useDongKhay(panel !== null, () => onPanelRef.current(null));
  if (!panel) return null;
  // Discarding is deliberate, so it does not need a confirmation box in front
  // of it -- but a 2000-character request can die on one mistap, and until now
  // there was no way back (reviewer C2). Keep the discarded text until the
  // person types again, closes the tray, or sends something.
  const discard = () => {
    if (panel === "poll") {
      setUndo({ panel: "poll", value: { question: held.current.question, choices: [...held.current.choices] } });
      update({ question: "", choices: ["", ""] });
      setPollError(null);
      setFieldErrors(null);
      setRestored((old) => ({ ...old, poll: false }));
    }
  };
  const undoDiscard = () => {
    if (!undo) return;
    update(undo.value);
    setUndo(null);
  };
  const sendPoll = async () => {
    const issue = loiBinhChon(draft.question, draft.choices);
    setPollError(issue);
    setFieldErrors(issue ? loiBinhChonTheoO(draft.question, draft.choices) : null);
    if (issue) return;
    const submitted = held.current;
    const title = draft.question.trim().replace(/\?+$/, "");
    if (await onPoll(`/vote ${title}? ${draft.choices.map((choice) => choice.trim()).filter(Boolean).join(" | ")}`)) {
      if (held.current === submitted) update({ question: "", choices: ["", ""] });
      setRestored((old) => ({ ...old, poll: false }));
      onPanel(null);
    } else setPollError("Bình chọn chưa được gửi. Bản nháp vẫn ở đây; bạn có thể thử lại.");
  };
  // Each tool is the paper thing it puts into the conversation (ADR-0037 D1).
  const tools: { vat: VatBan; label: string; action: () => void }[] = [
    { vat: "anh-in", label: "Ảnh", action: onImage },
    { vat: "sticker", label: "Sticker", action: onSticker },
    { vat: "phieu-bau", label: "Bình chọn", action: () => onPanel("poll") },
    { vat: "lich", label: "Tờ hẹn", action: () => onPanel("plan") },
    // A couple keeps every friends' tool and adds its paper (owner decision 2026-09-28).
    ...(onToGiay ? [{ vat: "thu-gap" as VatBan, label: CONG_CU_TO_GIAY, action: () => { onPanel(null); onToGiay(); } }] : []),
  ];
  const hasDraft = panel === "poll" && (!!draft.question || draft.choices.some(Boolean));
  const boCuc = boCucKhay(rongHang ?? Math.min(width, 820) - 32, tools.length, fontScale);
  return (
    <View style={[styles.tools, { backgroundColor: colors.card, borderColor: colors.line }]}>
      <View style={styles.titleRow}>
        <Text accessibilityRole="header" style={[typography.title, styles.flex, { color: colors.ink }]}>
          {panel === "tools" ? "Thêm vào cuộc trò chuyện" : panel === "poll" ? chu.tieuDePoll : "Phác một tờ hẹn"}
        </Text>
        <IconButton accessibilityLabel="Đóng khay công cụ" icon="close" quiet onPress={() => onPanel(null)} />
      </View>
      {panel === "poll" && undo?.panel === panel ? <View style={styles.draftRow}>
        <Text accessibilityLiveRegion="polite" style={[typography.caption, styles.flex, { color: colors.inkSoft }]}>Đã bỏ bản nháp.</Text>
        <RudiButton label="Hoàn tác" variant="ghost" compact full={false} disabled={busy} onPress={undoDiscard} />
      </View> : panel === "poll" && hasDraft ? <View style={styles.draftRow}>
        <Text accessibilityLiveRegion="polite" style={[typography.caption, styles.flex, { color: colors.inkSoft }]}>{restored.poll ? "Đã khôi phục bản nháp" : ""}</Text>
        <RudiButton label="Bỏ bản nháp" tone="warn" variant="ghost" compact full={false} disabled={busy} onPress={discard} />
      </View> : null}
      <ScrollView keyboardShouldPersistTaps="handled" style={[styles.scroll, {
        // The cap holds the grid and the photo note under it; when a short
        // window squeezes the tray it may shrink, but never below one whole
        // row of tools, so a tile is never cut in half.
        maxHeight: tranKhay(height, panel === "tools" ? boCuc.caoNoiDung + caoGhiChu : null),
        minHeight: panel === "tools" ? Math.min(boCuc.caoNoiDung, (boCuc.caoNoiDung - (boCuc.hang - 1) * KHOANG_CONG_CU) / boCuc.hang) : 0,
      }]}>
        {panel === "tools" ? (
          // One row while every tool still holds its widest word, else balanced
          // rows of equal tools (`boCucKhay`): five never wrap 4 + 1 (lab 28/09).
          <View onLayout={(e) => { const w = Math.floor(e.nativeEvent.layout.width); if (w > 0 && w !== rongHang) setRongHang(w); }} style={styles.toolRow} testID="khay-hang-cong-cu">{tools.map((tool) => (
            <Pressable key={tool.label} accessibilityRole="button" accessibilityLabel={tool.label} disabled={busy} onPress={tool.action}
              style={({ pressed }) => [styles.tool, { width: boCuc.oRong }, pressed && styles.pressed]}>
              <View style={[styles.toolIcon, { width: boCuc.icon, height: boCuc.icon, backgroundColor: colors.ground, borderColor: colors.line }]}><VeLop height={boCuc.icon - 12} khungH={KHUNG_VAT} khungW={KHUNG_VAT} lop={hinhVat(tool.vat)} width={boCuc.icon - 12} /></View>
              {/* Stretched to the column: measured at its own width, Android wrapped
                  «Tờ giấy» after «Tờ» and the second line never showed (24/09). */}
              <Text numberOfLines={2} style={[typography.caption, styles.toolNhan, { color: colors.ink }]}>{tool.label}</Text>
            </Pressable>
          ))}</View>
        ) : null}
        {/* The photo note scrolls under the tools rather than holding its
            lines above them: when a short window squeezes the tray, the note
            is what gets cut, never a tool (QA UI-124). */}
        {panel === "tools" ? <Text onLayout={(e) => { const h = Math.ceil(e.nativeEvent.layout.height); if (h !== caoGhiChu) setCaoGhiChu(h); }} style={[typography.caption, styles.ghiChu, { color: colors.inkSoft }]}>{chu.ghiChuAnh}</Text> : panel === "poll" ? (
          // Written on a sticky note, one pen line per choice (plan S5).
          <View style={[styles.form, styles.giayNho, { backgroundColor: colors.card, borderColor: colors.lineStrong }]}>
            <ONhapMuc label="Câu hỏi" accessibilityLabel="Câu hỏi bình chọn" value={draft.question} onChangeText={(question) => update({ question })} editable={!busy} maxLength={180} placeholder={chu.goiYPoll} />
            {/* The message sits under the box it belongs to. One sentence under
                the whole form said something was wrong but not where, so fixing
                it meant re-reading every box (reviewer C4, heuristic 9). */}
            {fieldErrors?.question ? <Text accessibilityLiveRegion="polite" style={[typography.caption, { color: colors.warn }]}>{fieldErrors.question}</Text> : null}
            {draft.choices.map((choice, index) => <View key={index} style={styles.o}>
              <ONhapMuc label={`Lựa chọn ${index + 1}`} value={choice} editable={!busy} onChangeText={(value) => update({ choices: held.current.choices.map((old, i) => index === i ? value : old) })} maxLength={100} />
              {fieldErrors?.choices[index] ? <Text accessibilityLiveRegion="polite" style={[typography.caption, { color: colors.warn }]}>{fieldErrors.choices[index]}</Text> : null}
            </View>)}
            {draft.choices.length < 6 ? <RudiButton label="Thêm lựa chọn" variant="ghost" compact disabled={busy} onPress={() => update({ choices: [...held.current.choices, ""] })} /> : null}
          </View>
        ) : (
          /* The words that replaced the tray's own prompt box (ADR-0046): an
             AI request is a message in the thread now, so the tray points at
             the composer instead of sending on the person's behalf. */
          <Text style={[typography.body, { color: colors.ink }]} testID="chat-khay-hoi-ai">
            {chu.loiHoiAi}
          </Text>
        )}
      </ScrollView>
      {panel === "poll" ? <View style={styles.footer}>
        {pollError ? <Text accessibilityLiveRegion="polite" style={[typography.caption, { color: colors.warn }]}>{pollError}</Text> : null}
        <RudiButton label="Gửi bình chọn" loading={busy} disabled={busy} onPress={() => void sendPoll()} />
      </View> : panel === "plan" ? <View style={styles.footer}>
        {error ? <Text accessibilityLiveRegion="polite" style={[typography.caption, { color: colors.warn }]}>{error}</Text> : null}
        {sanSang ? <RudiButton label="Hỏi Rủ Đi AI" disabled={busy} onPress={onHoiAi} />
          : <Text style={[typography.caption, { color: colors.inkSoft }]}>AI chưa sẵn sàng. Bạn vẫn có thể tự tạo kèo.</Text>}
        <RudiButton label="Tự tạo kèo" variant="outline" disabled={busy} onPress={onManual} />
      </View> : null}
    </View>
  );
}

const styles = StyleSheet.create({
  flex: { flex: 1 },
  toHen: { flexDirection: "row", alignItems: "center", gap: 10, minHeight: 52, padding: 12, marginHorizontal: 16, marginTop: 8, marginBottom: 4, borderWidth: 1, borderRadius: 4, borderTopRightRadius: 18, overflow: "hidden" },
  fold: { position: "absolute", top: -1, right: -1, width: 16, height: 16, borderLeftWidth: 1, borderBottomWidth: 1, borderBottomLeftRadius: 4 },
  // The tray gives way in a short window: it shrinks and its body scrolls, so
  // the composer under it never leaves the screen (QA UI-124: a couple's room
  // at 390×460 with the tray open pushed the send button 60dp under the edge).
  tools: { borderTopWidth: 1, paddingHorizontal: 16, paddingBottom: 12, flexShrink: 1, minHeight: 0 },
  titleRow: { flexDirection: "row", alignItems: "center", gap: 8 },
  ghiChu: { paddingTop: 2, paddingBottom: 4 },
  draftRow: { flexDirection: "row", alignItems: "center", gap: 8 },
  // A box and the sentence about it are one thing, so they move together.
  o: { gap: 4 },
  toolRow: { flexDirection: "row", flexWrap: "wrap", gap: KHOANG_CONG_CU, justifyContent: "center" },
  // Squares on one line: a two-line label («Bình / chọn» at 360) hangs below its square.
  tool: { alignItems: "center", justifyContent: "flex-start", gap: 7, paddingVertical: 10 },
  toolNhan: { alignSelf: "stretch", textAlign: "center" },
  toolIcon: { borderWidth: 1, borderRadius: 14, alignItems: "center", justifyContent: "center" },
  giayNho: { borderWidth: 1, borderRadius: 4, padding: 12, marginTop: 4 },
  form: { gap: 12, paddingBottom: 4 },
  scroll: { flexGrow: 0, flexShrink: 1, minHeight: 0 },
  footer: { flexShrink: 0, gap: 8, paddingTop: 10 },
  scope: { flexDirection: "row", alignItems: "flex-start", gap: 8 },
  thay: { borderWidth: 1, borderRadius: 10, padding: 12, gap: 8 },
  luot: { gap: 6 },
  pressed: { opacity: 0.65 },
  keoGon: { flexDirection: "row", alignItems: "center", gap: 8, minHeight: 48, paddingHorizontal: 20, marginTop: 4 },
});
