import { Ionicons } from "@expo/vector-icons";
import { useNhuongChoNep } from "../../nep/NepProvider";
import { useEffect, useRef, useState } from "react";
import { BackHandler, Platform, Pressable, ScrollView, StyleSheet, Text, View, useWindowDimensions } from "react-native";
import { lenhSanSang, type ChatCapabilities, type LenhAi } from "../../chat/ai-invocations";
import { cauBoiCanh, nhanVai, type BoiCanh } from "../../ai/boi-canh";
import { docBanNhapCongCu, ghiBanNhapCongCu, loiBinhChon, loiBinhChonTheoO, type BanNhapCongCu, type LoiBinhChonTheoO } from "../../chat/ban-nhap-cong-cu";
import { chuKhay } from "../../chat/khay-cong-cu";
import { docTheAi, type Tin } from "../../chat/tin-song";
import { KHUNG_VAT, hinhVat, type VatBan } from "../../art/vat-ban";
import { typography, useRudiTheme } from "../../theme";
import { VeLop } from "../../ui/art/VeLop";
import { ONhapMuc } from "../../ui/ONhapMuc";
import { Field, IconButton, RudiButton } from "../../ui";

export type KhayChat = "tools" | "poll" | "plan" | null;

/** The folded margin names the next action; the full text lives in the thread. */
export function ToHen({ tin, onOpen, onVote }: { tin: Tin; onOpen: (tin: Tin) => void; onVote: (tin: Tin) => void }) {
  const { colors } = useRudiTheme();
  const the = docTheAi(tin.card);
  if (the.loai !== "poll" && the.loai !== "itinerary") return null;
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
  const action = poll ? "Xem phiếu" : daThanhKeo ? "Mở lịch trình" : nhapDangMo ? "Sửa cùng hội" : "Sửa tờ hẹn";
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

export function CongCuChat({ personId, contextId, panel, onPanel, onImage, onSticker, onPoll, onPlan, onManual, capabilities, busy, error, initialPrompt, boiCanh, haiNguoi = false, onToGiay, lenh = "plan" }: {
  personId: string; contextId: string;
  /** A two-person conversation: the tray's plan slot opens the pair's paper. */
  haiNguoi?: boolean; onToGiay?: () => void;
  panel: KhayChat; onPanel: (panel: KhayChat) => void; onImage: () => void; onSticker: () => void;
  onPoll: (command: string) => Promise<boolean>; onPlan: (prompt: string, boiCanh?: BoiCanh) => Promise<boolean>; onManual: () => void;
  /** What the screen is showing, already reduced to what would go on the wire. */
  boiCanh: BoiCanh | null;
  capabilities: ChatCapabilities | null; busy: boolean; error: string | null; initialPrompt: string;
  /**
   * What the AI panel asks for. `chia_bill` keeps everything that protects the
   * person (the «Mình đang thấy» preview, «Chỉ gửi lời nhờ», the queue) and
   * changes only the words and the manual fallback.
   */
  lenh?: LenhAi;
}) {
  const chiaBill = lenh === "chia_bill";
  const sanSang = lenhSanSang(capabilities, lenh);
  const { colors } = useRudiTheme();
  const chu = chuKhay(haiNguoi && onToGiay !== undefined);
  const [dinhKem, setDinhKem] = useState(true);
  const [moRong, setMoRong] = useState(false);
  // The tray is a sheet laid over the conversation; Nếp makes room for it.
  useNhuongChoNep(panel !== null);
  const { height } = useWindowDimensions();
  const [draft, setDraft] = useState(() => docBanNhapCongCu(personId, contextId));
  const held = useRef(draft);
  const [restored, setRestored] = useState(() => ({ poll: !!draft.question || draft.choices.some(Boolean), plan: !!draft.prompt }));
  const [pollError, setPollError] = useState<string | null>(null);
  const [fieldErrors, setFieldErrors] = useState<LoiBinhChonTheoO | null>(null);
  const [undo, setUndo] = useState<{ panel: "poll" | "plan"; value: Partial<BanNhapCongCu> } | null>(null);
  const lastPrompt = useRef("");
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
  useEffect(() => {
    if (initialPrompt && initialPrompt !== lastPrompt.current) update({ prompt: initialPrompt });
    lastPrompt.current = initialPrompt;
  }, [initialPrompt]);
  useEffect(() => {
    if (!panel) return;
    if (Platform.OS === "android") {
      const sub = BackHandler.addEventListener("hardwareBackPress", () => { onPanelRef.current(null); return true; });
      return () => sub.remove();
    }
    if (Platform.OS !== "web") return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") { event.preventDefault(); onPanelRef.current(null); }
    };
    document.addEventListener("keydown", onKey, true);
    return () => document.removeEventListener("keydown", onKey, true);
  }, [panel !== null]);
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
    if (panel === "plan") {
      setUndo({ panel: "plan", value: { prompt: held.current.prompt } });
      update({ prompt: "" });
      setRestored((old) => ({ ...old, plan: false }));
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
  const sendPlan = async () => {
    const submitted = held.current.prompt;
    if (await onPlan(submitted.trim(), dinhKem ? boiCanh ?? undefined : undefined)) {
      if (held.current.prompt === submitted) update({ prompt: "" });
      setRestored((old) => ({ ...old, plan: false }));
      onPanel(null);
    }
  };
  // Each tool is the paper thing it puts into the conversation (ADR-0037 D1).
  const tools: { vat: VatBan; label: string; action: () => void }[] = [
    { vat: "anh-in", label: "Ảnh", action: onImage },
    { vat: "sticker", label: "Sticker", action: onSticker },
    { vat: "phieu-bau", label: "Bình chọn", action: () => onPanel("poll") },
    chu.congCuHen.dich === "to-giay"
      ? { vat: "thu-gap", label: chu.congCuHen.label, action: () => { onPanel(null); onToGiay?.(); } }
      : { vat: "lich", label: chu.congCuHen.label, action: () => onPanel("plan") },
  ];
  const hasDraft = panel === "poll" ? !!draft.question || draft.choices.some(Boolean) : panel === "plan" && !!draft.prompt;
  return (
    <View style={[styles.tools, { backgroundColor: colors.card, borderColor: colors.line }]}>
      <View style={styles.titleRow}>
        <Text accessibilityRole="header" style={[typography.title, styles.flex, { color: colors.ink }]}>
          {panel === "tools" ? "Thêm vào cuộc trò chuyện" : panel === "poll" ? chu.tieuDePoll : chiaBill ? "Nhờ AI gom khoản chi" : "Phác một tờ hẹn"}
        </Text>
        <IconButton accessibilityLabel="Đóng khay công cụ" icon="close" quiet onPress={() => onPanel(null)} />
      </View>
      {panel !== "tools" && undo?.panel === panel ? <View style={styles.draftRow}>
        <Text accessibilityLiveRegion="polite" style={[typography.caption, styles.flex, { color: colors.inkSoft }]}>Đã bỏ bản nháp.</Text>
        <RudiButton label="Hoàn tác" variant="ghost" compact full={false} disabled={busy} onPress={undoDiscard} />
      </View> : panel !== "tools" && hasDraft ? <View style={styles.draftRow}>
        <Text accessibilityLiveRegion="polite" style={[typography.caption, styles.flex, { color: colors.inkSoft }]}>{restored[panel] ? "Đã khôi phục bản nháp" : ""}</Text>
        <RudiButton label="Bỏ bản nháp" variant="ghost" compact full={false} disabled={busy} onPress={discard} />
      </View> : null}
      <ScrollView keyboardShouldPersistTaps="handled" style={[styles.scroll, { maxHeight: Math.max(130, Math.min(260, height * 0.25)) }]}>
        {panel === "tools" ? (
          <View style={styles.toolRow}>{tools.map((tool) => (
            <Pressable key={tool.label} accessibilityRole="button" accessibilityLabel={tool.label} disabled={busy} onPress={tool.action}
              style={({ pressed }) => [styles.tool, pressed && styles.pressed]}>
              <View style={[styles.toolIcon, { backgroundColor: colors.ground, borderColor: colors.line }]}><VeLop height={44} khungH={KHUNG_VAT} khungW={KHUNG_VAT} lop={hinhVat(tool.vat)} width={44} /></View>
              {/* Stretched to the column: measured at its own width, Android wrapped
                  «Tờ giấy» after «Tờ» and the second line never showed (24/09). */}
              <Text numberOfLines={2} style={[typography.caption, styles.toolNhan, { color: colors.ink }]}>{tool.label}</Text>
            </Pressable>
          ))}</View>
        ) : panel === "poll" ? (
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
        ) : chiaBill ? <Field label="Lời nhờ gom khoản chi" accessibilityLabel="Lời nhờ gom khoản chi" value={draft.prompt} onChangeText={(prompt) => update({ prompt })} editable={!busy} multiline maxLength={2000} placeholder="Ví dụ: mình trả 300k tiền nước" />
          : <Field label={chu.nhanPlan} accessibilityLabel="Lời nhờ lập kế hoạch" value={draft.prompt} onChangeText={(prompt) => update({ prompt })} editable={!busy} multiline maxLength={2000} placeholder="Ví dụ: tối thứ Sáu, ăn rồi đi dạo quanh hồ" />}
      </ScrollView>
      {panel === "poll" ? <View style={styles.footer}>
        {pollError ? <Text accessibilityLiveRegion="polite" style={[typography.caption, { color: colors.warn }]}>{pollError}</Text> : null}
        <RudiButton label="Gửi bình chọn" loading={busy} disabled={busy} onPress={() => void sendPoll()} />
      </View> : panel === "plan" ? <View style={styles.footer}>
        {capabilities?.ai.share_scope === "caller_attached" ? (
          /* The same block Nếp already uses, and the same promise, so it reads
             as one app rather than two. It renders from the bundle itself, not
             from the message list: a preview rebuilt from the screen would be a
             picture OF the payload instead of the payload. */
          <View style={[styles.thay, { backgroundColor: colors.aiSoft, borderColor: colors.ai }]}>
            <Text style={[typography.label, { color: colors.ai }]}>Mình đang thấy</Text>
            <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.ink }]} testID="chat-boi-canh">
              {cauBoiCanh(dinhKem ? boiCanh : null)}
            </Text>
            {dinhKem && boiCanh !== null && boiCanh.luot.length > 0 ? (
              <>
                <Pressable accessibilityRole="button" accessibilityState={{ expanded: moRong }} onPress={() => setMoRong((cu) => !cu)} testID="chat-boi-canh-mo">
                  <Text style={[typography.caption, { color: colors.ai }]}>{moRong ? "Thu lại" : "Xem đúng thứ sắp gửi"}</Text>
                </Pressable>
                {moRong ? (
                  <View style={styles.luot} testID="chat-boi-canh-luot">
                    <Text style={[typography.caption, { color: colors.inkSoft }]}>
                      Ảnh đi bằng chú thích, sticker đi bằng chữ «Sticker», tin đã xoá đi bằng một dòng nói là đã xoá. Tên hiển thị của các thành viên đi kèm để AI biết ai nói gì, còn chữ trong tin nhắn thì đi nguyên văn.
                    </Text>
                    {boiCanh.luot.map((l) => (
                      <Text key={l.id} style={[typography.caption, { color: colors.ink }]} testID="chat-boi-canh-muc">{`${nhanVai(l)}: ${l.chu}`}</Text>
                    ))}
                  </View>
                ) : null}
              </>
            ) : null}
            {/* The old contract promised the history was never shared. Ship without
                a way back to exactly that and the promise is withdrawn by one
                side, which is not a thing to do quietly. */}
            <RudiButton label={dinhKem ? "Chỉ gửi lời nhờ" : "Gửi kèm tin gần nhất"} variant="outline" compact disabled={busy} onPress={() => setDinhKem((cu) => !cu)} />
          </View>
        ) : (
          <View style={styles.scope}>
            <Ionicons name="hand-left-outline" size={18} color={colors.inkSoft} />
            <Text style={[typography.caption, styles.flex, { color: colors.inkSoft }]}>Chỉ lời nhờ trong ô này được gửi cho AI. Lịch sử chat không được chia sẻ.</Text>
          </View>
        )}
        {chiaBill ? (
          /* AI does not touch money (ADR-0036 §2.9): say so before sending,
             not only on the card that comes back. */
          <Text style={[typography.caption, { color: colors.inkSoft }]} testID="chat-chia-bill-luu-y">
            AI chỉ đề xuất ai đã trả bao nhiêu, không ghi gì vào sổ. Cả hội xem lại và xác nhận ở mục Chia bill.
          </Text>
        ) : null}
        {error ? <Text accessibilityLiveRegion="polite" style={[typography.caption, { color: colors.warn }]}>{error}</Text> : null}
        {sanSang ? <RudiButton label="Gửi lời nhờ cho AI" loading={busy} disabled={busy || !draft.prompt.trim()} onPress={() => void sendPlan()} />
          : <Text style={[typography.caption, { color: colors.inkSoft }]}>{chiaBill ? "AI chưa gom khoản chi được lúc này. Bạn vẫn có thể thêm khoản chi ở mục Chia bill." : "AI chưa sẵn sàng. Bạn vẫn có thể tự tạo kèo."}</Text>}
        {chiaBill ? null : <RudiButton label="Tự tạo kèo" variant="outline" disabled={busy} onPress={onManual} />}
      </View> : null}
    </View>
  );
}

const styles = StyleSheet.create({
  flex: { flex: 1 },
  toHen: { flexDirection: "row", alignItems: "center", gap: 10, minHeight: 52, padding: 12, marginHorizontal: 16, marginTop: 8, marginBottom: 4, borderWidth: 1, borderRadius: 4, borderTopRightRadius: 18, overflow: "hidden" },
  fold: { position: "absolute", top: -1, right: -1, width: 16, height: 16, borderLeftWidth: 1, borderBottomWidth: 1, borderBottomLeftRadius: 4 },
  tools: { borderTopWidth: 1, paddingHorizontal: 16, paddingBottom: 12 },
  titleRow: { flexDirection: "row", alignItems: "center", gap: 8 },
  draftRow: { flexDirection: "row", alignItems: "center", gap: 8 },
  // A box and the sentence about it are one thing, so they move together.
  o: { gap: 4 },
  toolRow: { flexDirection: "row", flexWrap: "wrap", gap: 8, justifyContent: "space-between" },
  tool: { alignItems: "center", justifyContent: "center", minWidth: 62, flex: 1, gap: 7, paddingVertical: 10 },
  toolNhan: { alignSelf: "stretch", textAlign: "center" },
  toolIcon: { width: 56, height: 56, borderWidth: 1, borderRadius: 14, alignItems: "center", justifyContent: "center" },
  giayNho: { borderWidth: 1, borderRadius: 4, padding: 12, marginTop: 4 },
  form: { gap: 12, paddingBottom: 4 },
  scroll: { flexGrow: 0 },
  footer: { flexShrink: 0, gap: 8, paddingTop: 10 },
  scope: { flexDirection: "row", alignItems: "flex-start", gap: 8 },
  thay: { borderWidth: 1, borderRadius: 10, padding: 12, gap: 8 },
  luot: { gap: 6 },
  pressed: { opacity: 0.65 },
});
