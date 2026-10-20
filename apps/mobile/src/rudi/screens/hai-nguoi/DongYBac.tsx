import { Ionicons } from "@expo/vector-icons";
import { useEffect, useRef } from "react";
import { StyleSheet, Text, View, useWindowDimensions } from "react-native";

import { useRudiSession } from "../../session";
import { typography, useRudiTheme } from "../../theme";
import { Heading } from "../../ui";
import { ChuKy } from "../../ui/ChuKy";
import { PressScale } from "../../ui/PressScale";
import { Sheet } from "../../ui/Sheet";
import { useGiuKhiDong } from "../../ui/giu-khi-dong";
import { StampButton } from "../../ui/StampButton";
import { DongSo, TrangSo } from "../../ui/TrangSo";

/**
 * The consent ladder's two rungs a person can climb here (spec §6.1): tier 2
 * «Lập sổ» and tier 3 «Một đôi». Each sheet says in plain words what the
 * rung allows and, in the same breath, what it does NOT pull along -- a lower
 * rung never implies a higher one, and tier 4 (Nếp reads the chat) stays off
 * until slice 2 adds its own switch.
 *
 * Both are proposals: nothing changes until the other person agrees on their
 * own phone.
 *
 * UI v3 (ADR-0037, plan S2): each rung is a pact written by hand on a page of
 * the notebook -- what it allows beside what it does not pull along, both in
 * full (D8: said before the press) -- and two signature lines. Proposing is
 * signing your line; agreeing is signing the other. A signature that appears
 * while the sheet is up draws itself; one already there is simply there.
 */
function BacDongY({
  open,
  onClose,
  tieuDe,
  choPhep,
  khongKeoTheo,
  dangCho: dangChoSong,
  deNghiCuaToi: deNghiCuaToiSong,
  onDeNghi,
  onDongY,
  nhanDeNghi,
  tenNguoiKia,
  testID,
  dangLam = false,
  loi = null,
}: {
  open: boolean;
  onClose: () => void;
  tieuDe: string;
  choPhep: readonly string[];
  khongKeoTheo: readonly string[];
  dangCho: boolean;
  /**
   * Lời đề nghị đang chờ là của TÔI hay của người kia.
   *
   * Không phải chi tiết: người được đề nghị là người duy nhất bấm đồng ý được.
   * Bản đầu hiện cùng một câu «chờ người ấy đồng ý» cho cả hai phía, nên trên
   * máy thật người NHẬN lời đề nghị nhìn thấy một màn không có việc gì để làm
   * (đo ở vòng native 14/09). `true` khi không có lời đề nghị nào.
   */
  deNghiCuaToi: boolean;
  onDeNghi: () => void;
  /** Đồng ý lời đề nghị của người kia. Chỉ gọi khi `dangCho && !deNghiCuaToi`. */
  onDongY: () => void;
  nhanDeNghi: string;
  /** Who is waiting on this, by name; the app knows it, so it says it. */
  tenNguoiKia?: string;
  testID: string;
  dangLam?: boolean;
  loi?: string | null;
}) {
  const { colors, space } = useRudiTheme();
  const { width, fontScale } = useWindowDimensions();
  const xepDoc = width < 600 || fontScale >= 1.3;
  const { phien } = useRudiSession();
  // Closing, the sheet keeps what it said open (QA UI-084).
  const { dangCho, deNghiCuaToi } = useGiuKhiDong(open, { dangCho: dangChoSong, deNghiCuaToi: deNghiCuaToiSong });
  const tenToi = phien?.profile?.display_name?.trim() || "Bạn";
  const ho = tenNguoiKia?.trim() || "Người ấy";
  // Whoever proposed has signed; the other line waits for the other person.
  const toiKy = dangCho && deNghiCuaToi;
  const hoKy = dangCho && !deNghiCuaToi;
  // My signature appearing while the sheet is up (the proposal just landed) draws itself.
  const truoc = useRef<boolean | null>(null);
  const vuaKy = open && truoc.current === false && toiKy;
  useEffect(() => {
    truoc.current = open ? toiKy : null;
  }, [open, toiKy]);
  const actions = <View style={styles.actions}>
    {loi ? <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.warn }]}>{loi}</Text> : null}
    {dangCho ? (
      <Text accessibilityLiveRegion="polite" style={[typography.caption, { color: colors.inkSoft }]} testID={`${testID}-${deNghiCuaToi ? "dang-cho" : "ho-de-nghi"}`}>
        {deNghiCuaToi ? `Đã đề nghị. Chờ ${ho} đồng ý trên máy của họ.` : `${ho} đã đề nghị. ${testID === "lap-so" ? "Bạn đồng ý thì sổ mở." : "Bạn đồng ý thì sổ này trở thành sổ cặp đôi."}`}
      </Text>
    ) : null}
    {dangCho && !deNghiCuaToi ? (
      <StampButton wrapLabel style={styles.dau} disabled={dangLam} loading={dangLam} label="Đồng ý" onPress={onDongY} size="vua" tilt={-1} />
    ) : !dangCho ? (
      <StampButton wrapLabel style={styles.dau} disabled={dangLam} loading={dangLam} label={nhanDeNghi} onPress={onDeNghi} size="vua" tilt={-1} />
    ) : null}
    <PressScale accessibilityRole="button" onPress={onClose} pressedScale={0.98} style={styles.deSau}>
      <Text style={[typography.label, styles.deSauLabel, { color: colors.inkSoft }]}>Để sau</Text>
    </PressScale>
  </View>;
  return (
    <Sheet accessibilityLabel={tieuDe} onClose={onClose} open={open} testID={testID} footer={actions} showsVerticalScrollIndicator>
      <View style={[styles.noiDung, { gap: space.sm }]}>
        <Heading size="h2" title={tieuDe} />
        <TrangSo ke={false} tone="accent" testID={`${testID}-giao-keo`}>
          <View style={[styles.haiCot, xepDoc && styles.doc]}>
            <View style={[styles.cot, xepDoc && styles.cotDoc]}>
              <DongSo dau trai="Cho phép" />
              {choPhep.map((d, i) => (
                <View key={d} style={styles.dieu}>
                  <Ionicons color={colors.ink} name="checkmark" size={16} style={styles.dauDieu} />
                  <Text testID={`${testID}-cho-phep-${i}`} style={[typography.body, styles.flex, { color: colors.ink }]}>{d}</Text>
                </View>
              ))}
            </View>
            <View style={[styles.cot, xepDoc && styles.cotDoc]}>
              <DongSo dau trai="Không kéo theo" />
              {khongKeoTheo.map((d, i) => (
                <View key={d} style={styles.dieu}>
                  <Ionicons color={colors.inkSoft} name="close" size={16} style={styles.dauDieu} />
                  <Text testID={`${testID}-khong-keo-theo-${i}`} style={[typography.body, styles.flex, { color: colors.inkSoft }]}>{d}</Text>
                </View>
              ))}
            </View>
          </View>
          <View style={[styles.chuKy, xepDoc && styles.doc]}>
            <ChuKy wrapName showStatus style={xepDoc ? styles.chuKyDoc : undefined} choChu={hoKy ? "Chờ bạn ký: bấm Đồng ý" : "Bạn ký khi bấm đề nghị"} dong={vuaKy} ten={toiKy ? tenToi : null} vaiTro={hoKy ? "Người đồng ý" : "Người đề nghị"} />
            <ChuKy wrapName showStatus style={xepDoc ? styles.chuKyDoc : undefined} choChu={`Chờ ${ho} ký`} ten={hoKy ? ho : null} vaiTro={hoKy ? "Người đề nghị" : "Người đồng ý"} />
          </View>
          <Text style={[typography.caption, { color: colors.inkSoft, marginTop: 10 }]}>Im lặng không phải đồng ý.</Text>
        </TrangSo>
      </View>
    </Sheet>
  );
}

export function LapSo(props: { open: boolean; onClose: () => void; dangCho: boolean; deNghiCuaToi: boolean; onDeNghi: () => void; onDongY: () => void; tenNguoiKia?: string; dangLam?: boolean; loi?: string | null }) {
  return (
    <BacDongY
      {...props}
      choPhep={["Một chỗ hai bạn truyền giấy cho nhau mỗi tuần.", "Những điều cần tránh: «Không ăn được» và «Đừng».", "Nếp phác một tờ khi tới lượt, bạn sửa rồi gửi."]}
      // Notebook consent grants neither couple status nor access to chat.
      khongKeoTheo={["Không tự thành «Một đôi».", "Nếp không tự đọc tin nhắn của hai bạn.", "Chỉ hai bạn mở được sổ này trong app. Quyền riêng tư của tin nhắn không đổi."]}
      nhanDeNghi="Đề nghị lập sổ"
      testID="lap-so"
      tieuDe="Lập sổ lời hẹn"
    />
  );
}

export function BatMotDoi(props: { open: boolean; onClose: () => void; dangCho: boolean; deNghiCuaToi: boolean; onDeNghi: () => void; onDongY: () => void; tenNguoiKia?: string; dangLam?: boolean; loi?: string | null }) {
  return (
    <BacDongY
      {...props}
      // Only what switching it on does today. The roles («Người lo», «Người
      // chấm») are not built yet, so the sheet no longer promises them.
      choPhep={["Sổ này là sổ đôi: mỗi người chỉ có một.", "Nếp biết đây là sổ của một đôi."]}
      khongKeoTheo={["Đồng ý là cặp đôi không cấp quyền đọc chat cho Nếp. AI chỉ nhận phần bạn chọn và xác nhận gửi.", "Không đăng gì, không ai được báo.", "Tắt được bất cứ lúc nào, sổ vẫn còn."]}
      nhanDeNghi="Đề nghị bật «Một đôi»"
      testID="bat-mot-doi"
      tieuDe="Bật «Một đôi»"
    />
  );
}

const styles = StyleSheet.create({
  noiDung: { paddingBottom: 4 },
  actions: { gap: 4, paddingTop: 8 },
  dau: { maxWidth: "100%" },
  deSau: { minHeight: 44, paddingVertical: 6, paddingHorizontal: 16, alignItems: "center", justifyContent: "center" },
  deSauLabel: { alignSelf: "stretch", textAlign: "center" },
  flex: { flex: 1 },
  // Two columns when there is room for both at reading width, one under the other otherwise.
  haiCot: { flexDirection: "row", flexWrap: "wrap", gap: 10 },
  cot: { flexGrow: 1, flexBasis: 200, gap: 4 },
  doc: { flexDirection: "column", flexWrap: "nowrap", alignItems: "stretch", width: "100%" },
  cotDoc: { flexGrow: 0, flexBasis: "auto", width: "100%" },
  chuKyDoc: { flex: 0, flexBasis: "auto", width: "100%" },
  dieu: { flexDirection: "row", alignItems: "flex-start", gap: 6 },
  dauDieu: { marginTop: 3 },
  chuKy: { flexDirection: "row", flexWrap: "wrap", gap: 12, marginTop: 14 },
});
