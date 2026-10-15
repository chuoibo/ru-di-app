import { useRef, useState } from "react";
import { StyleSheet, Text, View } from "react-native";

import { typography, useRudiTheme } from "../../theme";
import { Heading, RudiButton } from "../../ui";
import { Field } from "../../ui/Field";
import { Sheet } from "../../ui/Sheet";

/**
 * After «Đã đi»: one line to keep about the outing (spec §7.4). One field,
 * one button. The first kept line turns the sheet into a memory (`da_giu`);
 * the notebook does not ask for a rating, a photo, or a second line.
 */
export function GiuMotDieu({ open, onClose, onGiu, dangLam = false, loi = null, testID }: { open: boolean; onClose: () => void; onGiu: (line: string) => Promise<boolean>; dangLam?: boolean; loi?: string | null; testID?: string }) {
  const { colors, space } = useRudiTheme();
  const [dong, setDong] = useState("");
  const lock = useRef(false);
  const sach = dong.trim();
  const giu = async () => {
    if (!sach || dangLam || lock.current) return;
    lock.current = true;
    try { if (await onGiu(sach)) setDong(""); } finally { lock.current = false; }
  };
  return (
    <Sheet accessibilityLabel="Giữ lại một điều" onClose={onClose} open={open} testID={testID ?? "giu-mot-dieu"}>
      <View style={[styles.noiDung, { gap: space.md }]}>
        <Heading size="h2" subtitle="Viết một điều bạn muốn nhớ về buổi đi này." title="Giữ lại một điều" />
        {/* One line, one field, and the keyboard's own Done key keeps it: the
            sheet has no keyboard avoidance, so the button under the field can
            sit beneath the keyboard while it is up. */}
        <Field
          label="Điều muốn giữ"
          onChangeText={setDong}
          onSubmitEditing={() => void giu()}
          editable={!dangLam}
          placeholder="Hàng chè đầu hẻm, lần sau lại."
          returnKeyType="done"
          testID="giu-mot-dieu-o"
          value={dong}
        />
        <Text style={[typography.caption, { color: colors.inkSoft }]}>Chỉ hai bạn đọc được dòng này.</Text>
        {loi ? <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.warn }]}>{loi}</Text> : null}
        <RudiButton loading={dangLam} disabled={dangLam || sach.length === 0} label="Giữ lại" lyDo={sach.length === 0 ? "Viết một điều trước." : undefined} onPress={() => void giu()} />
      </View>
    </Sheet>
  );
}

const styles = StyleSheet.create({ noiDung: { paddingBottom: 8 } });
