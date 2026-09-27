import { Ionicons } from "@expo/vector-icons";
import { useEffect, useState } from "react";
import { ScrollView, StyleSheet, Text, View } from "react-native";
import { ngayKieuViet, ngayVeISO, type ToHenChung } from "../../chat/to-hen-chung";
import { typography, useRudiTheme } from "../../theme";
import { IconButton, RudiButton } from "../../ui";
import { ONhapMuc } from "../../ui/ONhapMuc";

/**
 * The tray where the group edits its shared sheet.
 *
 * What makes this different from the manual outing form it sits next to: the
 * sheet already exists on the server, several people can be looking at it, and
 * every save carries the revision this editor started from. When somebody else
 * saved first, the person is shown the newer sheet and told so — their typing
 * is not silently replayed onto a sheet they have not read.
 */
export function KhayToHenChung({
  sheet,
  busy,
  error,
  stale,
  onSave,
  onDiscard,
  xacNhanBo,
  onHuyBo,
  onBoThat,
  onConfirm,
  onClose,
}: {
  sheet: ToHenChung;
  busy: boolean;
  error: string | null;
  stale: boolean;
  onSave: (patch: {
    title?: string;
    starts_on?: string | null;
    ends_on?: string | null;
    headcount?: number | null;
    budget_per_person_vnd?: number | null;
    stops?: { time_text: string; label: string }[];
  }) => void;
  onDiscard: () => void;
  /** True while the person is being asked to confirm the discard. */
  xacNhanBo: boolean;
  onHuyBo: () => void;
  onBoThat: () => void;
  onConfirm: () => void;
  onClose: () => void;
}) {
  const { colors } = useRudiTheme();
  const [title, setTitle] = useState(sheet.title);
  const [starts, setStarts] = useState(ngayKieuViet(sheet.starts_on));
  const [ends, setEnds] = useState(ngayKieuViet(sheet.ends_on));
  const [headcount, setHeadcount] = useState(sheet.headcount ? String(sheet.headcount) : "");
  const [budget, setBudget] = useState(
    sheet.budget_per_person_vnd ? String(sheet.budget_per_person_vnd) : "",
  );
  const [stops, setStops] = useState(sheet.stops.map((stop) => ({ ...stop })));

  // When the server hands back a newer sheet, the boxes have to follow it or
  // the person would keep editing a copy nobody else can see.
  useEffect(() => {
    setTitle(sheet.title);
    setStarts(ngayKieuViet(sheet.starts_on));
    setEnds(ngayKieuViet(sheet.ends_on));
    setHeadcount(sheet.headcount ? String(sheet.headcount) : "");
    setBudget(sheet.budget_per_person_vnd ? String(sheet.budget_per_person_vnd) : "");
    setStops(sheet.stops.map((stop) => ({ ...stop })));
  }, [sheet.revision, sheet.id]);

  const open = sheet.status === "open";
  const digits = (value: string) => value.replace(/\D+/g, "");

  const save = () => {
    onSave({
      title: title.trim(),
      starts_on: ngayVeISO(starts),
      ends_on: ngayVeISO(ends),
      headcount: headcount ? Number(digits(headcount)) : null,
      budget_per_person_vnd: budget ? Number(digits(budget)) : null,
      stops: stops.map((stop) => ({ time_text: stop.time_text, label: stop.label })),
    });
  };

  return (
    <View style={[styles.khay, { backgroundColor: colors.card, borderColor: colors.line }]}>
      <View style={styles.hangTieuDe}>
        <Text accessibilityRole="header" style={[typography.title, styles.flex, { color: colors.ink }]}>
          Tờ hẹn chung của hội
        </Text>
        <IconButton accessibilityLabel="Đóng tờ hẹn chung" icon="close" quiet onPress={onClose} />
      </View>

      <View style={styles.hangTrangThai}>
        <Ionicons name="people-outline" size={18} color={colors.inkSoft} />
        <Text style={[typography.caption, styles.flex, { color: colors.inkSoft }]}>
          {open
            ? `Cả hội sửa được tờ này · bản ${sheet.revision}`
            : sheet.status === "promoted"
              ? "Tờ này đã thành kèo."
              : "Tờ này đã được bỏ."}
        </Text>
      </View>

      {stale ? (
        <Text accessibilityLiveRegion="polite" style={[typography.caption, { color: colors.warn }]}>
          Đây là bản mới nhất. Xem lại rồi sửa tiếp nhé.
        </Text>
      ) : null}
      {error ? (
        <Text accessibilityLiveRegion="polite" style={[typography.caption, { color: colors.warn }]}>
          {error}
        </Text>
      ) : null}

      <ScrollView keyboardShouldPersistTaps="handled" style={styles.cuon}>
        {/* The group's shared sheet is written on ruled paper, a pen line per
            field, the stops in pencil order (ADR-0037 D1, plan S5). */}
        <View style={[styles.form, styles.giayKe, { backgroundColor: colors.card, borderColor: colors.lineStrong }]}>
          <ONhapMuc
            accessibilityLabel="Tên tờ hẹn"
            editable={open && !busy}
            label="Tên tờ hẹn"
            maxLength={200}
            onChangeText={setTitle}
            value={title}
          />
          <View style={styles.hang}>
            <View style={styles.flex}>
              <ONhapMuc
                accessibilityLabel="Ô ngày đi của tờ hẹn"
                editable={open && !busy}
                keyboardType="numbers-and-punctuation"
                label="Ngày đi"
                onChangeText={setStarts}
                placeholder="20/09/2026"
                value={starts}
              />
            </View>
            <View style={styles.flex}>
              <ONhapMuc
                accessibilityLabel="Ô ngày về của tờ hẹn"
                editable={open && !busy}
                keyboardType="numbers-and-punctuation"
                label="Ngày về"
                onChangeText={setEnds}
                placeholder="21/09/2026"
                value={ends}
              />
            </View>
          </View>
          <View style={styles.hang}>
            <View style={styles.flex}>
              <ONhapMuc
                accessibilityLabel="Ô số người của tờ hẹn"
                editable={open && !busy}
                keyboardType="number-pad"
                label="Số người"
                onChangeText={setHeadcount}
                value={headcount}
              />
            </View>
            <View style={styles.flex}>
              <ONhapMuc
                accessibilityLabel="Ô ngân sách một người của tờ hẹn"
                editable={open && !busy}
                keyboardType="number-pad"
                label="Ngân sách một người"
                onChangeText={setBudget}
                value={budget}
              />
            </View>
          </View>

          <Text style={[typography.label, { color: colors.ink }]}>Các chặng</Text>
          {stops.map((stop, index) => (
            <View key={index} style={styles.hang}>
              <View style={styles.gio}>
                <ONhapMuc
                  accessibilityLabel={`Giờ chặng ${index + 1}`}
                  editable={open && !busy}
                  keyboardType="numbers-and-punctuation"
                  label="Giờ"
                  onChangeText={(value) =>
                    setStops((rows) => rows.map((row, i) => (i === index ? { ...row, time_text: value } : row)))
                  }
                  placeholder="08:00"
                  value={stop.time_text}
                />
              </View>
              <View style={styles.flex}>
                <ONhapMuc
                  accessibilityLabel={`Tên chặng ${index + 1}`}
                  editable={open && !busy}
                  label={`Chặng ${index + 1}`}
                  maxLength={200}
                  onChangeText={(value) =>
                    setStops((rows) => rows.map((row, i) => (i === index ? { ...row, label: value } : row)))
                  }
                  value={stop.label}
                />
              </View>
            </View>
          ))}
          {open && stops.length < 12 ? (
            <RudiButton
              compact
              disabled={busy}
              label="Thêm chặng"
              onPress={() => setStops((rows) => [...rows, { time_text: "", label: "" }])}
              variant="ghost"
            />
          ) : null}
        </View>
      </ScrollView>

      {open && xacNhanBo ? (
        // Several people wrote this sheet. One tap must not end it.
        <View style={styles.chan}>
          <Text accessibilityLiveRegion="polite" style={[typography.caption, { color: colors.ink }]}>
            Bỏ tờ hẹn này? Cả hội sẽ không sửa tiếp được.
          </Text>
          <View style={styles.hangPhu}>
            <RudiButton compact disabled={busy} full={false} label="Giữ lại" onPress={onHuyBo} variant="ghost" />
            <RudiButton compact disabled={busy} full={false} label="Bỏ tờ hẹn" loading={busy} onPress={onBoThat} variant="outline" />
          </View>
        </View>
      ) : open ? (
        <View style={styles.chan}>
          <RudiButton disabled={busy} label="Lưu cho cả hội" loading={busy} onPress={save} />
          <View style={styles.hangPhu}>
            <RudiButton
              compact
              disabled={busy}
              full={false}
              label="Bỏ tờ hẹn"
              onPress={onDiscard}
              variant="ghost"
            />
            <RudiButton
              compact
              disabled={busy}
              full={false}
              label="Chốt thành kèo"
              onPress={onConfirm}
              variant="outline"
            />
          </View>
        </View>
      ) : (
        <View style={styles.chan}>
          <RudiButton label="Mở kèo của hội" onPress={onConfirm} variant="outline" />
        </View>
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  giayKe: { borderWidth: 1, borderRadius: 4, padding: 12 },
  khay: { borderTopWidth: StyleSheet.hairlineWidth, gap: 10, paddingHorizontal: 16, paddingVertical: 10 },
  hangTieuDe: { flexDirection: "row", alignItems: "center", gap: 8 },
  hangTrangThai: { flexDirection: "row", alignItems: "center", gap: 8 },
  flex: { flex: 1 },
  cuon: { maxHeight: 300 },
  form: { gap: 10 },
  hang: { flexDirection: "row", gap: 10 },
  gio: { width: 96 },
  chan: { gap: 8 },
  hangPhu: { flexDirection: "row", gap: 8, justifyContent: "space-between" },
});
