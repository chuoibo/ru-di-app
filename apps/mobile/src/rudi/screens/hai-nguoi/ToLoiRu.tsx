import { StyleSheet, Text, View, useWindowDimensions } from "react-native";

import { chuLon } from "../../adaptive";
import { typography, useRudiTheme } from "../../theme";
import { type ToGiay, cauTrangThai, khacGi, nutChoTo, phienBan, phienBanTruoc } from "../../to-giay/to-giay";
import { RudiButton } from "../../ui";
import { Stamp } from "../../ui/Stamp";
import { ToGiay as ToGiayView, VetGap } from "../../ui/ToGiay";

/**
 * The open sheet: the letter's rows, its crease between them, the state in
 * words, and the actions this reader may take (spec §15.2, §16).
 *
 * Rows are the content -- hour in `label` with tabular numerals in a fixed
 * column, the stop in `body` -- so the sheet reads as a note, not a form. The
 * reason («Vì: …») stands UNDER the sheet, outside it, in `inkSoft`: inside
 * the third row it made the rows unequal and the fold lose its meaning (blind
 * read 12/09). The state is a `Stamp` with a readable word, never a colour
 * alone (spec §12.1), and the whole sheet is one accessibility element whose
 * label is `cauTrangThai`, so a screen reader hears where things stand before
 * the rows.
 *
 * Buttons come from `nutChoTo`: two primaries side by side at 143dp on a
 * 360dp phone (`soft` «Ừ» · `outline` «Đề nghị sửa»), stacked under large
 * text; «Tuần này nghỉ» is a `ghost` line under them, 48dp tall, because it
 * must stay reachable until `chot` and never compete with the answer.
 *
 * Nothing here is optimistic: every handler is the screen's, and the screen
 * calls it after the notebook has moved (§3.3 rule 6).
 */
export function ToLoiRu({
  to,
  toiId,
  dan,
  onGui,
  onSuaNhap,
  onBoNhap,
  onDongY,
  onDeNghiSua,
  onRut,
  onNghiTuan,
  onDaDi,
  onGiu,
  onHuy,
  testID,
}: {
  to: ToGiay;
  toiId: string;
  /** This sheet is the thing to do now on this surface. */
  dan?: boolean;
  onGui?: () => void;
  onSuaNhap?: () => void;
  onBoNhap?: () => void;
  onDongY?: () => void;
  onDeNghiSua?: () => void;
  onRut?: () => void;
  onNghiTuan?: () => void;
  onDaDi?: () => void;
  onGiu?: () => void;
  onHuy?: () => void;
  testID?: string;
}) {
  const { colors, space } = useRudiTheme();
  const { fontScale } = useWindowDimensions();
  const doc = chuLon(fontScale);
  const pb = phienBan(to);
  const cau = cauTrangThai(to, toiId);
  const nut = nutChoTo(to, toiId);
  const doi = pb ? khacGi(pb, phienBanTruoc(to)) : [];
  const hang = pb?.content.chang ?? [];

  const bam: Record<string, (() => void) | undefined> = {
    gui: onGui,
    sua_nhap: onSuaNhap,
    bo: onBoNhap,
    dong_y: onDongY,
    de_nghi_sua: onDeNghiSua,
    rut: onRut,
    nghi_tuan: onNghiTuan,
    da_di: onDaDi,
    giu: onGiu,
    huy: onHuy,
  };
  const chinh = nut.filter((n) => n !== "nghi_tuan" && n !== "bo" && n !== "huy" && bam[n]);
  const phu = nut.filter((n) => (n === "nghi_tuan" || n === "bo" || n === "huy") && bam[n]);

  return (
    <View style={styles.khoi} testID={testID}>
      <ToGiayView dan={dan} testID={testID ? `${testID}-to` : undefined}>
        <View accessibilityLabel={cau} accessibilityRole="summary" accessible style={styles.thanTo}>
          {hang.length === 0 ? (
            <Text style={[typography.body, { color: colors.inkSoft }]}>Tuần này Nếp chưa có gì để phác. Bạn viết lấy một dòng?</Text>
          ) : null}
          {hang.map((c, i) => (
            <View key={`${c.gio}-${i}`}>
              {i > 0 ? <VetGap /> : null}
              <View style={styles.hang}>
                <Text style={[typography.label, styles.gio, { color: colors.ink }]}>{c.gio}</Text>
                <Text style={[typography.body, styles.viec, { color: colors.ink }]}>{c.viec}</Text>
              </View>
            </View>
          ))}
          <VetGap />
          <View style={styles.hangCuoi}>
            <Text style={[typography.label, { color: colors.inkSoft, flexShrink: 1 }]}>{pb?.content.ngay ?? ""}</Text>
            <Stamp label={NHAN[to.state]} tone="accent" variant={to.state === "chot" || to.state === "da_di" || to.state === "da_giu" ? "ink" : "outline"} testID={testID ? `${testID}-stamp` : undefined} />
          </View>
        </View>
      </ToGiayView>
      {pb?.ly_do ? (
        <Text style={[typography.label, styles.lyDo, { color: colors.inkSoft }]}>Vì: {pb.ly_do}</Text>
      ) : null}
      {doi.length > 0 ? (
        <View style={[styles.doi, { borderLeftColor: colors.lineStrong }]} testID={testID ? `${testID}-khac-gi` : undefined}>
          <Text style={[typography.caption, { color: colors.inkSoft }]}>Phiên bản {to.version} đổi gì so với trước:</Text>
          {doi.map((d) => (
            <Text key={d} style={[typography.label, { color: colors.ink }]}>· {d}</Text>
          ))}
        </View>
      ) : null}
      <Text style={[typography.caption, { color: colors.inkSoft }]}>{cau}</Text>
      {to.keeps.length > 0 ? (
        <View style={styles.giu}>
          {to.keeps.map((k) => (
            <Text key={k.id} style={[typography.note, { color: colors.inkSoft }]}>Giữ lại: {k.line}</Text>
          ))}
        </View>
      ) : null}
      {chinh.length > 0 ? (
        <View style={[styles.nutChinh, doc && styles.nutDoc, { gap: space.sm }]}>
          {chinh.map((n, i) => (
            <View key={n} style={doc ? styles.nutFull : styles.nutNua}>
              <RudiButton label={CHU_NUT[n]} onPress={bam[n]!} variant={i === 0 ? "soft" : "outline"} />
            </View>
          ))}
        </View>
      ) : null}
      {phu.map((n) => (
        <RudiButton key={n} label={CHU_NUT[n]} onPress={bam[n]!} variant="ghost" />
      ))}
    </View>
  );
}

/** The readable word on the stamp. Every state has one (spec §12.1). */
export const NHAN: Record<ToGiay["state"], string> = {
  nhap: "Bản phác",
  da_gui: "Đã gửi",
  da_xem: "Đã xem",
  de_nghi_sua: "Đề nghị sửa",
  dong_y: "Một bên đã ừ",
  chot: "Đã chốt",
  da_di: "Đã đi",
  da_giu: "Đã giữ",
  nghi_tuan: "Tuần nghỉ",
  het_han: "Hết khung",
  rut: "Đã rút",
  bo: "Đã bỏ",
  huy: "Đã huỷ",
};

const CHU_NUT: Record<string, string> = {
  gui: "Gửi cho người ấy",
  sua_nhap: "Sửa trước khi gửi",
  bo: "Bỏ bản phác này",
  dong_y: "Ừ",
  de_nghi_sua: "Đề nghị sửa",
  rut: "Rút lại",
  nghi_tuan: "Tuần này nghỉ",
  da_di: "Đã đi rồi",
  giu: "Giữ lại một điều",
  huy: "Huỷ buổi này",
};

const styles = StyleSheet.create({
  khoi: { gap: 10 },
  thanTo: { gap: 0 },
  hang: { flexDirection: "row", alignItems: "flex-start", gap: 12 },
  gio: { minWidth: 52, fontVariant: ["tabular-nums"], paddingTop: 2 },
  viec: { flex: 1 },
  hangCuoi: { flexDirection: "row", alignItems: "center", justifyContent: "space-between", gap: 12, marginTop: 4 },
  lyDo: { paddingHorizontal: 6 },
  doi: { borderLeftWidth: StyleSheet.hairlineWidth, paddingLeft: 12, gap: 2 },
  giu: { gap: 2, paddingHorizontal: 6 },
  nutChinh: { flexDirection: "row" },
  nutDoc: { flexDirection: "column" },
  nutNua: { flex: 1 },
  nutFull: { alignSelf: "stretch" },
});
