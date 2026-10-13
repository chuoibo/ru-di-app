import { Ionicons } from "@expo/vector-icons";
import { useEffect, useState } from "react";
import { Pressable, ScrollView, StyleSheet, View, useWindowDimensions } from "react-native";

import type { RandomMode, RandomOption, RandomRound } from "../../chat/e2ee/random-v2";
import { typography, useRudiTheme } from "../../theme";
import { IconButton, RudiButton } from "../../ui";
import { Text, useNativeStandaloneTextMetrics } from "../../ui/NativeFontMetrics";
import { ONhapMuc } from "../../ui/ONhapMuc";
import { useDongKhay } from "../../ui/useDongKhay";
import { RandomGame } from "./RandomGame";
import { usePollFocusReveal } from "./usePollFocusReveal";

export type RandomDraft = { request: string; mode: RandomMode; options: RandomOption[] };
export const EMPTY_RANDOM_DRAFT: RandomDraft = { request: "", mode: "wheel", options: [{ label: "" }, { label: "" }] };

export function RandomPicker({ draft, onChange, onClose, onSearch, onPlay, onNew, onRetry, onCreateOuting, round,
  published, searching, publishing, error, searchMessage, searchReady, keyboardOpen, sources, onOpenPlace }: {
  draft: RandomDraft; onChange: (draft: RandomDraft) => void; onClose: () => void;
  onSearch: () => void; onPlay: () => void; onNew: () => void; onRetry: () => void; onCreateOuting: () => void;
  round: RandomRound | null; published: boolean; searching: boolean; publishing: boolean;
  error: string | null; searchMessage: string | null; searchReady: boolean; keyboardOpen: boolean;
  sources: Record<string, string>;
  onOpenPlace: (id: string) => void;
}) {
  const { colors } = useRudiTheme();
  const { width } = useWindowDimensions();
  const metrics = useNativeStandaloneTextMetrics(typography.label);
  const reflow = width < 360 || metrics === null || (metrics.fontSize ?? 14) > 20;
  const reveal = usePollFocusReveal(keyboardOpen, 9);
  const [complete, setComplete] = useState(false);
  const [showSearch, setShowSearch] = useState(() => !!draft.request.trim());
  const [showSearchMessage, setShowSearchMessage] = useState(false);
  const locked = round !== null || publishing;
  useDongKhay(true, onClose);
  useEffect(() => {
    if (round && published) reveal.scrollRef.current?.scrollTo({ y: 0, animated: false });
  }, [round?.id, published, reveal.scrollRef]);
  const setOption = (index: number, label: string) => onChange({ ...draft, options: draft.options.map((option, i) => i === index ? { label } : option) });
  const mode = round?.mode ?? draft.mode, options = round?.options ?? draft.options;
  return <View style={[styles.root, { backgroundColor: colors.ground }]} testID="random-picker">
    <View style={styles.header}>
      <Text accessibilityRole="header" style={[typography.h2, styles.flex, { color: colors.ink }]}>Nếp chọn giúp</Text>
      <IconButton accessibilityLabel="Đóng Nếp chọn giúp" icon="close" quiet onPress={onClose} />
    </View>
    <ScrollView ref={reveal.scrollRef} keyboardShouldPersistTaps="handled" style={styles.body} contentContainerStyle={styles.content}
      onLayout={reveal.onBodyLayout} onScroll={reveal.onScroll} scrollEventThrottle={16}
      onScrollBeginDrag={reveal.onScrollBeginDrag} onContentSizeChange={reveal.onContentSizeChange}>
      {!round ? <>
        <Pressable accessibilityRole="button" accessibilityLabel="Nhờ Nếp tìm quán" accessibilityState={{ expanded: showSearch }}
          onPress={() => setShowSearch(!showSearch)} style={styles.searchDoor}>
          <Ionicons name="sparkles-outline" size={20} color={colors.accent} />
          <Text style={[typography.label, styles.flex, { color: colors.inkSoft }]}>Nhờ Nếp tìm quán</Text>
          <Ionicons name={showSearch ? "chevron-up" : "chevron-down"} size={18} color={colors.inkSoft} />
        </Pressable>
        {showSearch ? <View style={styles.searchArea}>
        <View ref={reveal.fields[0].group} onLayout={reveal.onFieldLayout}>
          <ONhapMuc oRef={reveal.fields[0].input} accessibilityLabel="Lời nhờ Nếp tìm quán"
            value={draft.request} maxLength={300} placeholder="Ăn tối, quán yên tĩnh…" editable={!locked && !searching}
            onChangeText={(request) => onChange({ ...draft, request })}
            onFocus={() => reveal.onFocus(0)} onBlur={() => reveal.onBlur(0)} />
        </View>
        <View style={styles.searchRow}>
          <Text style={[typography.note, styles.flex, { color: colors.inkSoft }]}>Chỉ gửi lời nhờ này cho AI.</Text>
          <RudiButton compact full={false} variant="ghost" label="Tìm quán" icon="sparkles-outline" loading={searching}
            disabled={locked || searching || !searchReady || !draft.request.trim()} lyDo={!searchReady ? "Tìm kiếm chưa sẵn sàng; bạn vẫn có thể tự thêm." : !draft.request.trim() ? "Viết lời nhờ trước khi tìm." : "Đang xử lý lượt này."} onPress={onSearch} />
        </View>
        {!searchReady ? <Text style={[typography.note, { color: colors.inkSoft }]}>Tìm kiếm chưa sẵn sàng. Bạn vẫn có thể tự thêm món hoặc quán.</Text> : null}
        </View> : null}
        {searchMessage ? <View style={styles.response}>
          <Pressable accessibilityRole="button" accessibilityState={{ expanded: showSearchMessage }} onPress={() => setShowSearchMessage(!showSearchMessage)} style={styles.responseHeading}>
            <Text style={[typography.label, styles.flex, { color: colors.inkSoft }]}>Phản hồi tìm quán</Text>
            <Ionicons name={showSearchMessage ? "chevron-up" : "chevron-down"} size={18} color={colors.inkSoft} />
          </Pressable>
          <Text accessibilityLiveRegion="polite" numberOfLines={showSearchMessage ? undefined : 2} style={[typography.note, { color: colors.inkSoft }]}>{searchMessage}</Text>
        </View> : null}
        <View accessibilityRole="tablist" style={[styles.modes, { borderColor: colors.lineStrong, backgroundColor: colors.card }]}>
          {([ ["wheel", "Vòng quay", "disc-outline"], ["ducks", "Đua vịt", "flag-outline"] ] as const).map(([value, label, icon]) =>
            <Pressable key={value} accessibilityRole="tab" accessibilityState={{ selected: mode === value, disabled: locked }} disabled={locked}
              onPress={() => onChange({ ...draft, mode: value })} style={({ pressed }) => [styles.mode, { backgroundColor: mode === value ? colors.accentSoft : "transparent" }, pressed && styles.pressed]}>
              <Ionicons name={icon} size={20} color={mode === value ? colors.accent : colors.inkSoft} />
              <Text style={[typography.label, { color: mode === value ? colors.accent : colors.inkSoft, flexShrink: 1 }]}>{label}</Text>
            </Pressable>)}
        </View>
      </> : <Text accessibilityLiveRegion="polite" style={[typography.note, { color: colors.inkSoft }]}>{published ? "Lượt này đã được gửi vào nhóm." : publishing ? "Đang gửi lượt đã khóa vào nhóm…" : "Lượt chưa gửi được. Kết quả được giữ nguyên khi thử lại."}</Text>}
      {(!keyboardOpen || !!round) && (!round || published) ? <RandomGame mode={mode} options={options}
        round={round} animate={!!round && published} width={Math.min(width - 64, round ? 280 : options.filter((option) => option.label.trim()).length < 2 ? 160 : 220)} onComplete={() => setComplete(true)} /> : null}
      {round && published && complete ? <View accessibilityLiveRegion="polite" style={styles.result}>
        <Text style={[typography.note, { color: colors.inkSoft }]}>{mode === "wheel" ? "Vòng quay chọn" : "Về đích đầu tiên"}</Text>
        <Text style={[typography.h2, { color: colors.ink }]}>{round.options[round.winner].label}</Text>
      </View> : null}
      <View style={styles.listHeading}>
        <Text style={[typography.title, styles.flex, { color: colors.ink }]}>{round ? "Các lựa chọn của lượt này" : "Trên bàn chọn"}</Text>
        <Text style={[typography.note, { color: colors.inkSoft }]}>{options.length}/8</Text>
      </View>
      {options.map((option, index) => <View key={index} style={styles.option}>
        <Text style={[typography.label, styles.number, { color: colors.inkSoft }]}>{index + 1}</Text>
        <View style={styles.flex} ref={reveal.fields[index + 1].group} onLayout={reveal.onFieldLayout}>
          {round ? <Text style={[typography.body, { color: colors.ink }]}>{option.label}</Text> : <ONhapMuc oRef={reveal.fields[index + 1].input}
            accessibilityLabel={`Lựa chọn ngẫu nhiên ${index + 1}`} value={option.label} maxLength={80} editable={!locked && !searching}
            placeholder={index === 0 ? "Một món hoặc một quán" : "Thêm một lựa chọn"} onChangeText={(label) => setOption(index, label)}
            onFocus={() => reveal.onFocus(index + 1)} onBlur={() => reveal.onBlur(index + 1)} />}
          {option.placeId ? <View style={styles.sourceRow}>
            <Text style={[typography.note, styles.flex, { color: colors.inkSoft }]}>{sources[option.placeId] || "Dẫn nguồn địa điểm"}</Text>
          </View> : null}
          {option.placeId ? <RudiButton compact full={false} variant="ghost" label="Xem quán" icon="open-outline" onPress={() => onOpenPlace(option.placeId!)} /> : null}
        </View>
        {!round ? <IconButton icon="remove" quiet accessibilityLabel={`Bỏ lựa chọn ${index + 1}`} disabled={locked || searching || options.length <= 2}
          onPress={() => onChange({ ...draft, options: draft.options.filter((_, i) => i !== index) })} /> : null}
      </View>)}
      {!round && options.length < 8 ? <RudiButton compact full={false} icon="add" variant="ghost" label="Thêm lựa chọn" disabled={locked || searching}
        lyDo={searching ? "Đang tìm quán; thêm lựa chọn sau khi tìm xong." : "Lượt đã khóa; mở lượt mới để sửa."}
        onPress={() => onChange({ ...draft, options: [...draft.options, { label: "" }] })} /> : null}
      {error && (keyboardOpen || round && published) ? <Text accessibilityLiveRegion="polite" style={[typography.note, { color: colors.warn }]}>{error}</Text> : null}
      {round && published && complete ? <View style={styles.actions}>
          <View style={[styles.resultActions, reflow && styles.column]}>
            <RudiButton full={false} label="Tạo kèo" onPress={onCreateOuting} />
            <RudiButton full={false} variant="outline" label="Lượt mới" onPress={() => { setComplete(false); onNew(); }} />
          </View>
        <Text style={[typography.note, { color: colors.inkSoft }]}>Người tổ chức tạo lượt chọn này.</Text>
      </View> : null}
    </ScrollView>
    {!keyboardOpen && (!round || !published) ? <View style={[styles.footer, { borderTopColor: colors.line, backgroundColor: colors.ground }]}>
      {!round ? <RudiButton label={mode === "wheel" ? "Quay cùng Nếp" : "Cho vịt xuất phát"} icon={mode === "wheel" ? "refresh" : "flag-outline"}
        disabled={publishing || searching} lyDo={searching ? "Đang tìm quán; bạn có thể chơi khi tìm xong." : "Đang gửi lượt này vào nhóm."} loading={publishing} onPress={onPlay} />
        : <RudiButton label={publishing ? "Đang gửi lượt…" : "Thử gửi lại lượt này"} disabled={publishing} lyDo="Đang gửi lượt này vào nhóm." loading={publishing} onPress={onRetry} />}
      {error ? <Text accessibilityLiveRegion="polite" style={[typography.note, { color: colors.warn }]}>{error}</Text>
        : <Text style={[typography.note, { color: colors.inkSoft }]}>Cơ hội bằng nhau · Kết quả lưu vào chat.</Text>}
    </View> : null}
  </View>;
}

const styles = StyleSheet.create({
  root: { flex: 1, minHeight: 0 }, header: { flexDirection: "row", alignItems: "center", paddingLeft: 16, paddingRight: 8, minHeight: 56, gap: 8 },
  body: { flex: 1, minHeight: 0 }, content: { paddingHorizontal: 16, paddingBottom: 20, gap: 10 }, flex: { flex: 1, minWidth: 0 },
  searchRow: { flexDirection: "row", alignItems: "center", gap: 8, flexWrap: "wrap" },
  searchDoor: { flexDirection: "row", alignItems: "center", gap: 10, minHeight: 48 }, searchArea: { gap: 8 },
  modes: { flexDirection: "row", borderWidth: 1, borderRadius: 14, padding: 4, gap: 4, flexWrap: "wrap" },
  mode: { flexGrow: 1, flexBasis: 118, minHeight: 48, paddingHorizontal: 10, paddingVertical: 8, borderRadius: 10, flexDirection: "row", alignItems: "center", justifyContent: "center", gap: 8 },
  pressed: { opacity: 0.7 }, listHeading: { flexDirection: "row", alignItems: "center", gap: 8, marginTop: 6 },
  option: { flexDirection: "row", alignItems: "center", gap: 8, paddingVertical: 2 }, number: { minWidth: 16, textAlign: "center" },
  footer: { borderTopWidth: StyleSheet.hairlineWidth, paddingHorizontal: 16, paddingTop: 12, paddingBottom: 8, gap: 6 },
  sourceRow: { flexDirection: "row", paddingTop: 4 }, actions: { gap: 10, paddingTop: 8 },
  result: { gap: 4, alignItems: "flex-start", paddingBottom: 8 }, resultActions: { flexDirection: "row", flexWrap: "wrap", gap: 10 }, column: { flexDirection: "column", alignItems: "stretch" },
  response: { gap: 4 }, responseHeading: { minHeight: 48, flexDirection: "row", alignItems: "center", gap: 8 },
});
