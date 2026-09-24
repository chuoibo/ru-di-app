import { Ionicons } from "@expo/vector-icons";
import { useEffect, useRef } from "react";
import { StyleSheet, Text, View, useWindowDimensions } from "react-native";

import { chuLon } from "../../adaptive";
import { typography, useRudiTheme } from "../../theme";
import { TRANG_THAI_MO, type ToGiay, cauTrangThai, daDongY, khacGi, nutChoTo, phienBan, phienBanTruoc, tenNgan } from "../../to-giay/to-giay";
import { ngayDocDuoc } from "../../to-giay/ngay";
import { useTenCho } from "../../to-giay/useTenCho";
import { cauLanHen, demNgay } from "../../to-giay/moc-hen";
import { homNay } from "../../keo/nhip-keo";
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
 * footer button to the eye). Always the outline seal: a filled ink block with
 * white capitals read as a button (blind read, round 2), and a rubber stamp
 * is a ring of ink anyway.
 *
 * Buttons come from `nutChoTo`: the first primary is `solid`, the second
 * `outline` -- `soft` lost to `outline` in a blind read, the reverse of what
 * spec §15.3 assumed -- side by side at 143dp on a 360dp phone and stacked
 * under large text. «Tuần này nghỉ», «Đã đi rồi», «Huỷ buổi này», «Bỏ bản
 * phác này», «Rút lại» are `ghost` lines: they must stay reachable, and none
 * of them is the thing to do now. A sender who is waiting has no primary at
 * all; the sentence under the sheet is the answer.
 *
 * Nothing here is optimistic: every handler is the screen's, and the screen
 * calls it after the notebook has moved (§3.3 rule 6).
 */
export function ToLoiRu({
  to,
  toiId,
  tenNguoiKia,
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
  onXemKeo,
  tatCa,
  testID,
}: {
  to: ToGiay;
  toiId: string;
  /** The other person's name, so the turn can be stated with it rather than «người ấy». */
  tenNguoiKia?: string;
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
  /** The outing this agreed sheet became; shown with the affirmative actions,
   *  above the escapes -- under «Huỷ buổi này» it read as an afterthought. */
  onXemKeo?: () => void;
  /** Every sheet of the notebook, to say which of their evenings this one is. */
  tatCa?: readonly ToGiay[];
  testID?: string;
}) {
  const { colors, space } = useRudiTheme();
  const { fontScale } = useWindowDimensions();
  const doc = chuLon(fontScale);
  const pb = phienBan(to);
  const dauTien = phienBan(to, 1);
  const cau = cauTrangThai(to, toiId, tenNguoiKia);
  const nut = nutChoTo(to, toiId);
  // Lượt của TÔI, không phải «tờ đang mở». Góc gấp coral và con tem coral là
  // hai dấu to nhất trên khung, và bản đầu bật cả hai theo trạng thái mở — nên
  // một tờ đã gửi đang chờ NGƯỜI KIA vẫn hét lên «làm gì đi», giống hệt khung
  // đang chờ chính mình. Luật «nhìn một cái biết ai đang chờ ai» hỏng đúng ở
  // trạng thái nó sinh ra để phục vụ.
  const luotCuaToi = nut.includes("gui") || nut.includes("dong_y");
  // Con dấu chỉ rơi khi trạng thái đổi TRONG LÚC tờ này đang ở trên màn. Mở
  // lại một tờ đã chốt từ hôm qua thì nó đã ở đó rồi, và một con dấu rơi lúc
  // ấy là kể lại một khoảnh khắc không thuộc về người đang nhìn.
  const truoc = useRef<string | null>(null);
  const vuaDoi = truoc.current !== null && truoc.current !== `${to.id}:${to.state}` && truoc.current.startsWith(`${to.id}:`);
  useEffect(() => {
    truoc.current = `${to.id}:${to.state}`;
  }, [to.id, to.state]);
  const dangQuyet = ["da_gui", "da_xem", "de_nghi_sua", "dong_y"].includes(to.state);
  const truocDo = dangQuyet && to.version > 1 ? phienBanTruoc(to) : undefined;
  const hangCu = truocDo?.content.chang ?? [];
  const lyDoSua = dangQuyet && to.version > 1 ? pb?.ly_do ?? null : null;
  const hang = pb?.content.chang ?? [];
  const tenCho = useTenCho([...hang, ...hangCu].map((c) => c.place_id));
  const doi = dangQuyet && pb && truocDo ? khacGi(pb, truocDo, (id) => tenCho[id]) : [];

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
  // Withdrawing is an escape, not the sender's job while they wait: as the
  // only primary it came out as a solid coral «Rút lại», the loudest thing on
  // a frame whose honest answer is «chờ». Ghost, with the other escapes.
  // `da_di` ra khỏi nhóm thoát: nó là việc KHẲNG ĐỊNH của một buổi đã chốt
  // («hai bạn đã đi rồi»), không phải một lối ra. Để nó ở đó thì một việc
  // KHÔNG hỏi lại đứng cạnh ba việc có hỏi, và nét kẻ phân nhóm nói sai.
  const PHU = new Set(["nghi_tuan", "bo", "huy", "rut"]);
  const chinh = nut.filter((n) => !PHU.has(n) && bam[n]);
  const phu = nut.filter((n) => PHU.has(n) && bam[n]);

  return (
    <View style={styles.khoi} testID={testID}>
      <ToGiayView dan={dan && luotCuaToi} testID={testID ? `${testID}-to` : undefined}>
        <View accessibilityLabel={cau} accessibilityRole="summary" accessible style={styles.thanTo}>
          {hang.length === 0 ? (
            <Text style={[typography.body, { color: colors.inkSoft }]}>Tuần này Nếp chưa có gì để phác. Bạn viết lấy một dòng?</Text>
          ) : null}
          {hang.map((c, i) => (
            <View key={`${c.gio}-${i}`}>
              {i > 0 ? <VetGap /> : null}
              <View style={styles.hang}>
                <Text style={[typography.label, styles.gio, { color: colors.ink }]}>{c.gio}</Text>
                <View style={styles.viec}>
                  <Text style={[typography.body, { color: colors.ink }]}>{c.viec}</Text>
                  {c.place_id && tenCho[c.place_id] ? (
                    <Text style={[typography.caption, { color: colors.inkSoft }]} testID={testID ? `${testID}-cho-${i}` : undefined}>
                      <Ionicons color={colors.inkSoft} name="location-outline" size={13} /> {tenCho[c.place_id]}
                    </Text>
                  ) : null}
                </View>
              </View>
            </View>
          ))}
          <VetGap />
          <View style={styles.hangCuoi}>
            <Text style={[typography.label, { color: colors.inkSoft, flexShrink: 1 }]}>{pb ? ngayDocDuoc(pb.content.ngay) : ""}</Text>
            <Stamp
              // «Con dấu rơi xuống» là khoảnh khắc ký của thế giới này, và nó
              // chỉ đúng khi trạng thái vừa đổi DƯỚI NGÓN TAY người đang nhìn.
              dong={vuaDoi}
              label={nhanDau(to, toiId, tenNguoiKia)}
              testID={testID ? `${testID}-stamp` : undefined}
              tilt={vuaDoi ? -2 : 0}
              tone={luotCuaToi ? "accent" : "ink"}
            />
          </View>
        </View>
      </ToGiayView>
      {dauTien?.ly_do && to.state !== "da_giu" ? <Text style={[typography.label, styles.lyDo, { color: colors.inkSoft }]}>Vì: {dauTien.ly_do}</Text> : null}
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
      {/* The agreed evening, as a moment rather than a status: how far it is,
          and which of their evenings it is (QA 23/09: after «Đã chốt» the
          screen held one red button and nothing else). */}
      {to.state === "chot" && (demNgay(to, homNay()) !== null || tatCa) ? (
        <View style={styles.moc} testID={testID ? `${testID}-moc` : undefined}>
          {demNgay(to, homNay()) !== null ? <Stamp dong={vuaDoi} label={demNgay(to, homNay()) ?? ""} tilt={-3} tone="ink" /> : null}
          {tatCa ? <Text style={[typography.label, styles.mocChu, { color: colors.ink }]}>{cauLanHen(to, tatCa)}</Text> : null}
        </View>
      ) : null}
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
              <RudiButton label={chuNut(n, pb)} onPress={bam[n]!} variant={i === 0 ? "solid" : "outline"} />
            </View>
          ))}
        </View>
      ) : null}
      {onXemKeo ? <RudiButton icon="calendar-outline" label="Xem kèo" onPress={onXemKeo} variant={chinh.length === 0 ? "solid" : "outline"} /> : null}
      {phu.length > 0 ? (
        // Tách khỏi nhóm trên bằng một nét: đây là các lối THOÁT, và «Bỏ bản
        // phác này» vứt đi thứ vừa viết còn «Huỷ buổi này» xoá một buổi người
        // kia đang trông. Bảng màu không có tông «phá huỷ» nên nét kẻ mang việc
        // ấy, và tờ xác nhận ở dưới mới là thứ nói ra hậu quả.
        <View style={[styles.thoat, { borderTopColor: colors.line, gap: space.sm }]}>
          {phu.map((n) => (
            <RudiButton key={n} label={chuNut(n, pb)} onPress={bam[n]!} variant="ghost" />
          ))}
        </View>
      ) : null}
    </View>
  );
}

/**
 * The readable word on the stamp, from the reader's side. Every state has one
 * (spec §12.1); the ones that depend on who sent the current version say so.
 */
export function nhanDau(to: ToGiay, toiId: string, tenNguoiKia?: string): string {
  const pb = phienBan(to);
  const toiGui = pb?.author_type === "human" && pb.sent_by === toiId;
  const ho = tenNguoiKia?.trim() ? tenNgan(tenNguoiKia) : "Người ấy";
  switch (to.state) {
    case "da_gui":
      // «GỬI CHO BẠN» in hoa đọc ra như một MỆNH LỆNH («gửi cho một người
      // bạn») trước khi đọc ra như một trạng thái. Nói ai gửi thì hết nhập
      // nhằng, và hàng dưới đó đã nói phải làm gì.
      return pb?.author_type === "nep" ? "Nếp gửi hộ" : toiGui ? "Đã gửi" : `${ho} gửi`;
    case "da_xem":
      return toiGui ? "Đã xem" : "Chờ bạn";
    case "dong_y":
      return daDongY(to, "toi", toiId) ? "Bạn đã ừ" : `${ho} đã ừ`;
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

/**
 * Chữ trên nút, và «Ừ» mang theo cái nó cam kết.
 *
 * Một âm tiết trên một nút coral kín có thể là cú bấm biến một lời đề nghị
 * thành một buổi đã chốt mà đường ra duy nhất là «Huỷ buổi này». Nút phải nói
 * nó làm gì.
 */
function chuNut(n: string, pb: { content: { ngay: string } } | undefined): string {
  if (n === "dong_y" && pb) return `Ừ, hẹn ${ngayDocDuoc(pb.content.ngay)}`;
  return CHU_NUT[n];
}

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
  // Flush with the status line under it: the 6dp inset read as a stray indent (QA 23/09).
  lyDo: {},
  moc: { flexDirection: "row", alignItems: "center", gap: 12, flexWrap: "wrap", paddingVertical: 4 },
  mocChu: { flexShrink: 1 },
  thoat: { borderTopWidth: StyleSheet.hairlineWidth, paddingTop: 12, marginTop: 6 },
  doi: { borderLeftWidth: StyleSheet.hairlineWidth, paddingLeft: 12, gap: 2 },
  giu: { gap: 2, paddingHorizontal: 6 },
  nutChinh: { flexDirection: "row" },
  nutDoc: { flexDirection: "column" },
  nutNua: { flex: 1 },
  nutFull: { alignSelf: "stretch" },
});
