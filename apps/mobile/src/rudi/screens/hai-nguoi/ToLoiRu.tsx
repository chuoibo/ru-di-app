import { StyleSheet, Text, View, useWindowDimensions } from "react-native";

import { chuLon } from "../../adaptive";
import { typography, useRudiTheme } from "../../theme";
import { TRANG_THAI_MO, type ToGiay, cauTrangThai, daDongY, khacGi, nutChoTo, phienBan, phienBanTruoc } from "../../to-giay/to-giay";
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
 * read 12/09). It is the OUTING's reason, the first version's; a later
 * version's reason is the reason for the change and belongs in the «đổi gì»
 * block, which shows only while the sheet is still being decided -- a diff
 * on a memory read as noise (blind read 12/09, Phase 2).
 *
 * The state is a `Stamp` with a readable word, never a colour alone (spec
 * §12.1), and the word is the READER's: the same `da_gui` sheet says «Đã gửi»
 * to its sender and «Gửi cho bạn» to its recipient. Coral on the stamp is for
 * a sheet that still asks something; a plan, a memory, a closed week is a
 * fact and stamps in ink, so the one coral lead on the surface stays with the
 * button that asks (spec §16.4; blind read: a filled coral «KÝ ỨC» beat the
 * footer button to the eye).
 *
 * Buttons come from `nutChoTo`: the first primary is `solid`, the second
 * `outline` -- `soft` lost to `outline` in a blind read, the reverse of what
 * spec §15.3 assumed -- side by side at 143dp on a 360dp phone and stacked
 * under large text. «Tuần này nghỉ», «Đã đi rồi», «Huỷ buổi này», «Bỏ bản
 * phác này» are `ghost` lines: they must stay reachable, and none of them is
 * the thing to do now.
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
  const dauTien = phienBan(to, 1);
  const cau = cauTrangThai(to, toiId);
  const nut = nutChoTo(to, toiId);
  const dangQuyet = ["da_gui", "da_xem", "de_nghi_sua", "dong_y"].includes(to.state);
  const doi = dangQuyet && pb && to.version > 1 ? khacGi(pb, phienBanTruoc(to)) : [];
  const lyDoSua = dangQuyet && to.version > 1 ? pb?.ly_do ?? null : null;
  const hang = pb?.content.chang ?? [];
  const laKeHoach = to.state === "chot" || to.state === "da_di" || to.state === "da_giu";

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
  const PHU = new Set(["nghi_tuan", "bo", "huy", "da_di"]);
  const chinh = nut.filter((n) => !PHU.has(n) && bam[n]);
  const phu = nut.filter((n) => PHU.has(n) && bam[n]);

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
            <Stamp
              label={nhanDau(to, toiId)}
              testID={testID ? `${testID}-stamp` : undefined}
              tone={TRANG_THAI_MO.includes(to.state) ? "accent" : "ink"}
              variant={laKeHoach ? "ink" : "outline"}
            />
          </View>
        </View>
      </ToGiayView>
      {dauTien?.ly_do ? <Text style={[typography.label, styles.lyDo, { color: colors.inkSoft }]}>Vì: {dauTien.ly_do}</Text> : null}
      {doi.length > 0 || lyDoSua ? (
        <View style={[styles.doi, { borderLeftColor: colors.lineStrong }]} testID={testID ? `${testID}-khac-gi` : undefined}>
          <Text style={[typography.caption, { color: colors.inkSoft }]}>Phiên bản {to.version} đổi gì so với trước:</Text>
          {doi.map((d) => (
            <Text key={d} style={[typography.label, { color: colors.ink }]}>· {d}</Text>
          ))}
          {lyDoSua ? <Text style={[typography.label, { color: colors.inkSoft }]}>Vì sao sửa: {lyDoSua}</Text> : null}
        </View>
      ) : null}
      <Text style={[typography.caption, { color: colors.inkSoft }]}>{cau}</Text>
      {to.keeps.length > 0 ? (
        <View style={styles.giu}>
          {to.keeps.map((k) => (
            <Text key={k.id} style={[typography.body, { color: colors.ink }]}>
              <Text style={{ color: colors.inkSoft }}>Giữ lại: </Text>
              {k.line}
            </Text>
          ))}
        </View>
      ) : null}
      {chinh.length > 0 ? (
        <View style={[styles.nutChinh, doc && styles.nutDoc, { gap: space.sm }]}>
          {chinh.map((n, i) => (
            <View key={n} style={doc ? styles.nutFull : styles.nutNua}>
              <RudiButton label={CHU_NUT[n]} onPress={bam[n]!} variant={i === 0 ? "solid" : "outline"} />
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

/**
 * The readable word on the stamp, from the reader's side. Every state has one
 * (spec §12.1); the ones that depend on who sent the current version say so.
 */
export function nhanDau(to: ToGiay, toiId: string): string {
  const pb = phienBan(to);
  const toiGui = pb?.author_type === "human" && pb.sent_by === toiId;
  switch (to.state) {
    case "da_gui":
      return pb?.author_type === "nep" ? "Nếp gửi hộ" : toiGui ? "Đã gửi" : "Gửi cho bạn";
    case "da_xem":
      return toiGui ? "Đã xem" : "Chờ bạn";
    case "dong_y":
      return daDongY(to, "toi", toiId) ? "Bạn đã ừ" : "Người ấy đã ừ";
    case "rut":
      return toiGui ? "Bạn đã rút" : "Đã rút";
    default:
      return NHAN[to.state];
  }
}

/** Role-neutral words, for rows and for the states that read the same from both sides. */
export const NHAN: Record<ToGiay["state"], string> = {
  nhap: "Bản phác",
  da_gui: "Đã gửi",
  da_xem: "Đã xem",
  de_nghi_sua: "Đề nghị sửa",
  dong_y: "Một bên đã ừ",
  chot: "Đã chốt",
  da_di: "Đã đi",
  // «Ký ức», not «Đã giữ»: the sheet has gone to memory, and on the emulator the
  // condensed stamp face drew «ĐÃ GIỮ» as «ĐÃ» with a blank where the second
  // word should be (12/09, fs1.0 light; «ĐÃ CHỐT» and «ĐÃ GỬI» drew whole).
  // The cause is not a missing glyph (the cmap has Ữ); noted as a debt.
  da_giu: "Ký ức",
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
