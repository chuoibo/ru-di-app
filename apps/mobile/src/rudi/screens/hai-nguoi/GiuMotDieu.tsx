import { useState } from "react";
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
export function GiuMotDieu({ open, onClose, onGiu, testID }: { open: boolean; onClose: () => void; onGiu: (line: string) => void; testID?: string }) {
  const { colors, space } = useRudiTheme();
  const [dong, setDong] = useState("");
  const sach = dong.trim();
  return (
    <Sheet accessibilityLabel="Giữ lại một điều" onClose={onClose} open={open} testID={testID ?? "giu-mot-dieu"}>
      <View style={[styles.noiDung, { gap: space.md }]}>
        <Heading size="h2" subtitle="Một dòng thôi. Tờ này về ký ức khi có nó." title="Giữ lại một điều" />
        <Field
          label="Điều muốn giữ"
          multiline
          onChangeText={setDong}
          placeholder="Hàng chè đầu hẻm, lần sau lại."
          testID="giu-mot-dieu-o"
          value={dong}
        />
        <Text style={[typography.caption, { color: colors.inkSoft }]}>Chỉ hai bạn đọc được dòng này.</Text>
        <RudiButton
          disabled={sach.length === 0}
          label="Giữ lại"
          onPress={() => {
            onGiu(sach);
            setDong("");
          }}
        />
      </View>
    </Sheet>
  );
}

const styles = StyleSheet.create({ noiDung: { paddingBottom: 8 } });
