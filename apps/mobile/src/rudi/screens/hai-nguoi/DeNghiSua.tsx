import { useEffect, useMemo, useState } from "react";
import { StyleSheet, Text, View } from "react-native";

import { typography, useRudiTheme } from "../../theme";
import { type NoiDungTo, type ToGiay, khacGi, phienBan } from "../../to-giay/to-giay";
import { Heading, RudiButton } from "../../ui";
import { Field } from "../../ui/Field";
import { Sheet } from "../../ui/Sheet";

/**
 * «Đề nghị sửa»: a new version, not an edit (spec §3.3 rule 3). The sheet
 * starts from the current version's words, and before the person sends it
 * shows exactly what changed («Giờ chỗ chính: 18:30 → 19:00») from `khacGi`,
 * the same function the receiving side reads it with. Sending it means the
 * proposer has agreed to it; the other person now answers.
 */
export function DeNghiSua({ to, open, onClose, onGui, testID }: { to: ToGiay; open: boolean; onClose: () => void; onGui: (content: NoiDungTo, lyDo: string | null) => void; testID?: string }) {
  const { colors, space } = useRudiTheme();
  const pb = phienBan(to);
  const chinh = pb?.content.chang[0], tiep = pb?.content.chang[1];
  const [ngay, setNgay] = useState(pb?.content.ngay ?? "");
  const [gio1, setGio1] = useState(chinh?.gio ?? "");
  const [viec1, setViec1] = useState(chinh?.viec ?? "");
  const [gio2, setGio2] = useState(tiep?.gio ?? "");
  const [viec2, setViec2] = useState(tiep?.viec ?? "");
  const [lyDo, setLyDo] = useState("");

  useEffect(() => {
    if (!open) return;
    setNgay(pb?.content.ngay ?? "");
    setGio1(chinh?.gio ?? "");
    setViec1(chinh?.viec ?? "");
    setGio2(tiep?.gio ?? "");
    setViec2(tiep?.viec ?? "");
    setLyDo("");
  }, [open, pb?.version]); // eslint-disable-line react-hooks/exhaustive-deps

  const noiDung = useMemo<NoiDungTo>(() => {
    const chang = [];
    if (gio1.trim() || viec1.trim()) chang.push({ gio: gio1.trim(), viec: viec1.trim(), place_id: chinh?.place_id ?? null, can_kiem: true });
    if (gio2.trim() || viec2.trim()) chang.push({ gio: gio2.trim(), viec: viec2.trim(), place_id: tiep?.place_id ?? null, can_kiem: true });
    return { ngay: ngay.trim(), chang };
  }, [ngay, gio1, viec1, gio2, viec2, chinh?.place_id, tiep?.place_id]);

  const doi = pb
    ? khacGi({ ...pb, version: pb.version + 1, content: noiDung, ly_do: lyDo.trim() || null }, pb)
    : [];
  const guiDuoc = doi.length > 0 && noiDung.chang.length > 0 && noiDung.chang[0].viec.length > 0;

  return (
    <Sheet accessibilityLabel="Đề nghị sửa tờ giấy" onClose={onClose} open={open} testID={testID ?? "de-nghi-sua"}>
      <View style={[styles.noiDung, { gap: space.md }]}>
        <Heading size="h2" subtitle={`Sửa gì thì thành phiên bản ${to.version + 1}. Người ấy sẽ thấy đúng chỗ đổi.`} title="Đề nghị sửa" />
        <Field label="Ngày" onChangeText={setNgay} placeholder="Thứ Bảy 20/09" testID="de-nghi-sua-ngay" value={ngay} />
        <View style={styles.hang}>
          <View style={styles.gio}>
            <Field label="Giờ" onChangeText={setGio1} placeholder="18:30" testID="de-nghi-sua-gio" value={gio1} />
          </View>
          <View style={styles.viec}>
            <Field label="Chỗ chính" onChangeText={setViec1} placeholder="Ăn tối, một quán chưa đi" testID="de-nghi-sua-viec" value={viec1} />
          </View>
        </View>
        <View style={styles.hang}>
          <View style={styles.gio}>
            <Field label="Giờ" onChangeText={setGio2} placeholder="20:00" value={gio2} />
          </View>
          <View style={styles.viec}>
            <Field label="Đi tiếp (tuỳ chọn)" onChangeText={setViec2} placeholder="Đi bộ, rồi chè" value={viec2} />
          </View>
        </View>
        <Field label="Vì sao đổi (tuỳ chọn)" multiline onChangeText={setLyDo} placeholder="Tối thứ Bảy mưa." value={lyDo} />
        {doi.length > 0 ? (
          <View style={[styles.doi, { borderLeftColor: colors.lineStrong }]} testID="de-nghi-sua-khac-gi">
            <Text style={[typography.caption, { color: colors.inkSoft }]}>Người ấy sẽ thấy:</Text>
            {doi.map((d) => (
              <Text key={d} style={[typography.label, { color: colors.ink }]}>· {d}</Text>
            ))}
          </View>
        ) : (
          <Text style={[typography.caption, { color: colors.inkSoft }]}>Chưa đổi gì. Sửa một dòng ở trên thì gửi được.</Text>
        )}
        <RudiButton disabled={!guiDuoc} label={`Gửi phiên bản ${to.version + 1}`} onPress={() => onGui(noiDung, lyDo.trim() || null)} />
        <RudiButton label="Thôi" onPress={onClose} variant="ghost" />
      </View>
    </Sheet>
  );
}

const styles = StyleSheet.create({
  noiDung: { paddingBottom: 8 },
  hang: { flexDirection: "row", gap: 10 },
  gio: { width: 96 },
  viec: { flex: 1 },
  doi: { borderLeftWidth: StyleSheet.hairlineWidth, paddingLeft: 12, gap: 2 },
});
