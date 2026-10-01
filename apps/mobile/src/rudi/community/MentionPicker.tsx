import { useEffect, useState } from "react";
import { Pressable, Text, View } from "react-native";
import { docDanhSachBan, type Ban } from "../../screens/ca-nhan/ban-be";
import { typography, useRudiTheme } from "../theme";
import { RudiButton } from "../ui";
import { toggleState } from "../../ui/a11y";

export function MentionPicker({ person, selected, onChange }: { person: string; selected: string[]; onChange: (next: string[]) => void }) {
  const [friends, setFriends] = useState<Ban[]>([]); const [open, setOpen] = useState(false); const [error, setError] = useState<string | null>(null); const { colors } = useRudiTheme();
  useEffect(() => { if (!open) return; let alive = true; void docDanhSachBan(person, person).then((items) => { if (alive) setFriends(items); }).catch(() => { if (alive) setError("Chưa đọc được danh sách bạn bè."); }); return () => { alive = false; }; }, [person, open]);
  return <View style={{ gap: 8 }}><RudiButton full={false} compact label={selected.length ? `Cùng ${selected.length} người bạn` : "Tag bạn bè"} icon="at-outline" variant="ghost" onPress={() => setOpen(!open)} />{open ? <><Text style={[typography.caption, { color: colors.inkSoft }]}>Tag gửi lời nhắc, không mở quyền xem bài.</Text>{friends.map((f) => <Pressable {...toggleState("checkbox", selected.includes(f.person_id), () => onChange(selected.includes(f.person_id) ? selected.filter((id) => id !== f.person_id) : [...selected, f.person_id].slice(0, 20)))} key={f.person_id} onPress={() => onChange(selected.includes(f.person_id) ? selected.filter((id) => id !== f.person_id) : [...selected, f.person_id].slice(0, 20))} style={{ minHeight: 48, justifyContent: "center" }}><Text style={[typography.body, { color: selected.includes(f.person_id) ? colors.accent : colors.ink }]}>{f.display_name}{selected.includes(f.person_id) ? " · Đã chọn" : ""}</Text></Pressable>)}{!friends.length ? <Text style={[typography.caption, { color: colors.inkFaint }]}>{error ?? "Chưa có bạn bè để tag."}</Text> : null}</> : null}</View>;
}
