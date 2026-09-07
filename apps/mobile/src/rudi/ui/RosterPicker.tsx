import { Ionicons } from "@expo/vector-icons";
import { Pressable, StyleSheet, Text } from "react-native";
import { ResponsiveRow } from "../ui";
import { typography, useRudiTheme, type RudiTone } from "../theme";

/** The same full-contrast selection contract for fixture and server rosters.
 *  `tone` is the meaning of the choice: `split` (default) for who shares a
 *  bill, `accent` for who is being invited. */
export function RosterPicker({ people, selected, onToggle, disabled = false, tone = "split", nhanCho }: {
  people: readonly { id: string; name: string }[];
  selected: readonly string[];
  onToggle(id: string): void;
  disabled?: boolean;
  tone?: RudiTone;
  /** Accessible name per box when the same roster repeats on several rows («An · Nước»). */
  nhanCho?: (name: string) => string;
}) {
  const { colors } = useRudiTheme();
  const muc = colors[tone];
  const nen = tone === "accent" ? colors.accentSoft : tone === "ai" ? colors.aiSoft : colors.splitSoft;
  return <ResponsiveRow minItemWidth={130} gap={8}>
    {people.map((person) => {
      const checked = selected.includes(person.id);
      return <Pressable key={person.id} accessibilityRole="checkbox" accessibilityLabel={nhanCho ? nhanCho(person.name) : person.name}
        accessibilityState={{ checked, disabled }} aria-checked={checked} disabled={disabled}
        onPress={() => onToggle(person.id)}
        style={({ pressed }) => [styles.person, {
          borderColor: checked ? muc : colors.lineStrong,
          backgroundColor: checked ? nen : colors.card,
          opacity: pressed ? 0.8 : 1,
        }]}>
        <Ionicons name={checked ? "checkmark-circle" : "ellipse-outline"} color={checked ? muc : colors.inkFaint} size={20} />
        <Text style={[typography.label, styles.name, { color: colors.ink }]}>{person.name}</Text>
      </Pressable>;
    })}
  </ResponsiveRow>;
}

const styles = StyleSheet.create({
  person: { flex: 1, minHeight: 48, flexDirection: "row", alignItems: "center", gap: 8, padding: 10, borderWidth: 1, borderRadius: 10 },
  name: { flex: 1, flexShrink: 1 },
});
