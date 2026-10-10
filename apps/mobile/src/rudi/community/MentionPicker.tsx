import { useEffect, useState } from "react";
import { Pressable, Text, View } from "react-native";
import { docDanhSachBan, type Ban } from "../../screens/ca-nhan/ban-be";
import { typography, useRudiTheme } from "../theme";
import { RudiButton } from "../ui";
import { toggleState } from "../../ui/a11y";
import { SkeletonLines } from "../ui/Skeleton";

export function MentionPicker({ person, selected, onChange, expanded, onExpandedChange, hideTrigger = false }: { person: string; selected: string[]; onChange: (next: string[]) => void; expanded?: boolean; onExpandedChange?: (open: boolean) => void; hideTrigger?: boolean }) {
  const [friends, setFriends] = useState<Ban[]>([]); const [open, setOpen] = useState(false); const [error, setError] = useState<string | null>(null); const { colors } = useRudiTheme();
  const [loading, setLoading] = useState(false); const [retry, setRetry] = useState(0);
  const isOpen = expanded ?? open;
  const changeOpen = (value: boolean) => { setOpen(value); onExpandedChange?.(value); };
  useEffect(() => { if (!isOpen) return; let alive = true; setLoading(true); setError(null); void docDanhSachBan(person, person).then((items) => { if (alive) setFriends(items); }).catch(() => { if (alive) setError("Chưa đọc được danh sách bạn bè."); }).finally(() => { if (alive) setLoading(false); }); return () => { alive = false; }; }, [person, isOpen, retry]);
  return <View style={{ gap: 8 }}>{!hideTrigger ? <RudiButton full={false} compact label={selected.length ? `Cùng ${selected.length} người bạn` : "Tag bạn bè"} icon="at-outline" variant="ghost" onPress={() => changeOpen(!isOpen)} /> : null}{isOpen ? <><Text style={[typography.caption, { color: colors.inkSoft }]}>Tag gửi lời nhắc, không mở quyền xem bài.</Text>{loading ? <SkeletonLines lines={2}/> : null}{!loading && !error ? friends.map((f) => <Pressable {...toggleState("checkbox", selected.includes(f.person_id), () => onChange(selected.includes(f.person_id) ? selected.filter((id) => id !== f.person_id) : [...selected, f.person_id].slice(0, 20)))} key={f.person_id} onPress={() => onChange(selected.includes(f.person_id) ? selected.filter((id) => id !== f.person_id) : [...selected, f.person_id].slice(0, 20))} style={{ minHeight: 48, justifyContent: "center" }}><Text style={[typography.body, { color: selected.includes(f.person_id) ? colors.accent : colors.ink }]}>{f.display_name}{selected.includes(f.person_id) ? " · Đã chọn" : ""}</Text></Pressable>) : null}{error ? <RudiButton label="Thử lại danh sách bạn bè" variant="ghost" onPress={() => setRetry((n) => n + 1)}/> : null}{!loading && !friends.length ? <Text style={[typography.caption, { color: colors.inkSoft }]}>{error ?? "Chưa có bạn bè để tag."}</Text> : null}</> : null}</View>;
}
