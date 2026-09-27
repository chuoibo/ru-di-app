import { Ionicons } from "@expo/vector-icons";
import type { ReactNode } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";
import { ResponsiveRow } from "../ui";
import { typography, useRudiTheme, type RudiTone } from "../theme";
import { HinhNhan } from "./Avatar";

/** The same full-contrast selection contract for fixture and server rosters.
 *  `tone` is the meaning of the choice: `split` (default) for who shares a
 *  bill, `accent` for who is being invited.
 *
 *  `kieu="nhan"` draws the roster as people rather than boxes (ADR-0037 D6):
 *  each person a small paper standee in their own ink, a row of them per
 *  dish. Checked is the figure standing forward with a filled seal; unchecked
 *  steps back (faded) with an empty ring -- two differences, not a colour
 *  alone. Ids must be person ids there, since the ink is keyed on them. */
export function RosterPicker({ people, selected, onToggle, disabled = false, tone = "split", nhanCho, kieu = "o", them }: {
  people: readonly { id: string; name: string }[];
  selected: readonly string[];
  onToggle(id: string): void;
  disabled?: boolean;
  tone?: RudiTone;
  /** Accessible name per box when the same roster repeats on several rows («An · Nước»). */
  nhanCho?: (name: string) => string;
  kieu?: "o" | "nhan";
  /** `nhan` only: shortcuts laid in the same row after the people («Cả hai»), so a couple's row is one line. */
  them?: ReactNode;
}) {
  const { colors } = useRudiTheme();
  const muc = colors[tone];
  const nen = tone === "accent" ? colors.accentSoft : tone === "ai" ? colors.aiSoft : colors.splitSoft;
  if (kieu === "nhan") {
    return <View style={styles.hangNhan}>
      {people.map((person) => {
        const checked = selected.includes(person.id);
        return <Pressable key={person.id} accessibilityRole="checkbox" accessibilityLabel={nhanCho ? nhanCho(person.name) : person.name}
          accessibilityState={{ checked, disabled }} aria-checked={checked} disabled={disabled}
          onPress={() => onToggle(person.id)}
          style={({ pressed }) => [styles.nhan, { opacity: pressed ? 0.8 : 1 }]}>
          <View>
            <HinhNhan name={person.name} personId={person.id} ring={checked} size={28} style={checked ? undefined : styles.vang} />
            <View style={[styles.dau, { backgroundColor: checked ? muc : colors.card, borderColor: checked ? muc : colors.lineStrong }]}>
              {checked ? <Ionicons color={colors.card} name="checkmark" size={11} /> : null}
            </View>
          </View>
          <Text numberOfLines={1} style={[typography.caption, styles.tenNhan, { color: checked ? colors.ink : colors.inkSoft }]}>{person.name}</Text>
        </Pressable>;
      })}
      {them ? <View style={styles.them}>{them}</View> : null}
    </View>;
  }
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
  hangNhan: { flexDirection: "row", flexWrap: "wrap", alignItems: "center", gap: 4 },
  them: { flexDirection: "row", flexWrap: "wrap", alignItems: "center", gap: 6, marginLeft: 4 },
  nhan: { width: 72, minHeight: 56, alignItems: "center", paddingTop: 2 },
  vang: { opacity: 0.45 },
  dau: { position: "absolute", right: -2, top: -2, width: 16, height: 16, borderRadius: 8, borderWidth: 1.5, alignItems: "center", justifyContent: "center" },
  tenNhan: { maxWidth: 70, textAlign: "center" },
});
