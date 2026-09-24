import { Ionicons } from "@expo/vector-icons";
import { useNhuongChoNep } from "../../nep/NepProvider";
import { useEffect, useRef, useState } from "react";
import { BackHandler, Platform, Pressable, ScrollView, StyleSheet, Text, View, useWindowDimensions } from "react-native";
import type { ChatCapabilities } from "../../chat/ai-invocations";
import { docBanNhapCongCu, ghiBanNhapCongCu, loiBinhChon, loiBinhChonTheoO, type BanNhapCongCu, type LoiBinhChonTheoO } from "../../chat/ban-nhap-cong-cu";
import { docTheAi, type Tin } from "../../chat/tin-song";
import { typography, useRudiTheme } from "../../theme";
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

export function CongCuChat({ personId, contextId, panel, onPanel, onImage, onSticker, onPoll, onPlan, onManual, capabilities, busy, error, initialPrompt }: {
  personId: string; contextId: string;
  panel: KhayChat; onPanel: (panel: KhayChat) => void; onImage: () => void; onSticker: () => void;
  onPoll: (command: string) => Promise<boolean>; onPlan: (prompt: string) => Promise<boolean>; onManual: () => void;
  capabilities: ChatCapabilities | null; busy: boolean; error: string | null; initialPrompt: string;
}) {
  const { colors } = useRudiTheme();
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
    if (await onPlan(submitted.trim())) {
      if (held.current.prompt === submitted) update({ prompt: "" });
      setRestored((old) => ({ ...old, plan: false }));
      onPanel(null);
    }
  };
  const tools: { icon: keyof typeof Ionicons.glyphMap; label: string; action: () => void }[] = [
    { icon: "image-outline", label: "Ảnh", action: onImage },
    { icon: "happy-outline", label: "Sticker", action: onSticker },
    { icon: "stats-chart-outline", label: "Bình chọn", action: () => onPanel("poll") },
    { icon: "trail-sign-outline", label: "Tờ hẹn", action: () => onPanel("plan") },
  ];
  const hasDraft = panel === "poll" ? !!draft.question || draft.choices.some(Boolean) : panel === "plan" && !!draft.prompt;
  return (
    <View style={[styles.tools, { backgroundColor: colors.card, borderColor: colors.line }]}>
      <View style={styles.titleRow}>
        <Text accessibilityRole="header" style={[typography.title, styles.flex, { color: colors.ink }]}>
          {panel === "tools" ? "Thêm vào cuộc trò chuyện" : panel === "poll" ? "Hội mình chọn gì?" : "Phác một tờ hẹn"}
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
              <View style={[styles.toolIcon, { backgroundColor: colors.ground, borderColor: colors.line }]}><Ionicons name={tool.icon} size={25} color={colors.ink} /></View>
              <Text style={[typography.caption, { color: colors.ink }]}>{tool.label}</Text>
            </Pressable>
          ))}</View>
        ) : panel === "poll" ? (
          <View style={styles.form}>
            <Field label="Câu hỏi" accessibilityLabel="Câu hỏi bình chọn" value={draft.question} onChangeText={(question) => update({ question })} editable={!busy} maxLength={180} placeholder="Tối nay hội mình ăn gì?" />
            {/* The message sits under the box it belongs to. One sentence under
                the whole form said something was wrong but not where, so fixing
                it meant re-reading every box (reviewer C4, heuristic 9). */}
            {fieldErrors?.question ? <Text accessibilityLiveRegion="polite" style={[typography.caption, { color: colors.warn }]}>{fieldErrors.question}</Text> : null}
            {draft.choices.map((choice, index) => <View key={index} style={styles.o}>
              <Field label={`Lựa chọn ${index + 1}`} value={choice} editable={!busy} onChangeText={(value) => update({ choices: held.current.choices.map((old, i) => index === i ? value : old) })} maxLength={100} />
              {fieldErrors?.choices[index] ? <Text accessibilityLiveRegion="polite" style={[typography.caption, { color: colors.warn }]}>{fieldErrors.choices[index]}</Text> : null}
            </View>)}
            {draft.choices.length < 6 ? <RudiButton label="Thêm lựa chọn" variant="ghost" compact disabled={busy} onPress={() => update({ choices: [...held.current.choices, ""] })} /> : null}
          </View>
        ) : <Field label="Bạn muốn rủ hội đi đâu?" accessibilityLabel="Lời nhờ lập kế hoạch" value={draft.prompt} onChangeText={(prompt) => update({ prompt })} editable={!busy} multiline maxLength={2000} placeholder="Ví dụ: tối thứ Sáu, ăn rồi đi dạo quanh hồ" />}
      </ScrollView>
      {panel === "poll" ? <View style={styles.footer}>
        {pollError ? <Text accessibilityLiveRegion="polite" style={[typography.caption, { color: colors.warn }]}>{pollError}</Text> : null}
        <RudiButton label="Gửi bình chọn" loading={busy} disabled={busy} onPress={() => void sendPoll()} />
      </View> : panel === "plan" ? <View style={styles.footer}>
        <View style={styles.scope}>
          <Ionicons name="hand-left-outline" size={18} color={colors.inkSoft} />
          <Text style={[typography.caption, styles.flex, { color: colors.inkSoft }]}>Chỉ lời nhờ trong ô này được gửi cho AI. Lịch sử chat không được chia sẻ.</Text>
        </View>
        {error ? <Text accessibilityLiveRegion="polite" style={[typography.caption, { color: colors.warn }]}>{error}</Text> : null}
        {capabilities?.ai.plan.available ? <RudiButton label="Gửi lời nhờ cho AI" loading={busy} disabled={busy || !draft.prompt.trim()} onPress={() => void sendPlan()} />
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
  tools: { borderTopWidth: 1, paddingHorizontal: 16, paddingBottom: 12 },
  titleRow: { flexDirection: "row", alignItems: "center", gap: 8 },
  draftRow: { flexDirection: "row", alignItems: "center", gap: 8 },
  // A box and the sentence about it are one thing, so they move together.
  o: { gap: 4 },
  toolRow: { flexDirection: "row", flexWrap: "wrap", gap: 8, justifyContent: "space-between" },
  tool: { alignItems: "center", justifyContent: "center", minWidth: 62, flex: 1, gap: 7, paddingVertical: 10 },
  toolIcon: { width: 48, height: 48, borderWidth: 1, borderRadius: 14, alignItems: "center", justifyContent: "center" },
  form: { gap: 12, paddingBottom: 4 },
  scroll: { flexGrow: 0 },
  footer: { flexShrink: 0, gap: 8, paddingTop: 10 },
  scope: { flexDirection: "row", alignItems: "flex-start", gap: 8 },
  pressed: { opacity: 0.65 },
});
