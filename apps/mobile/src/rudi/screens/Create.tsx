import { useLocalSearchParams, useRouter } from "expo-router";
import { useState } from "react";
import { StyleSheet, Text, View, useWindowDimensions } from "react-native";
import { chuLon } from "../adaptive";
import Svg, { Circle, Path, Rect } from "react-native-svg";

import { PaperObject, type PaperName } from "../asset-pack/react-native/PaperObject";
import { usePaperPalette } from "../asset-pack/react-native/usePaperPalette";
import { bongGiay, typography, useRudiTheme } from "../theme";
import { laPair } from "../nhan-rieng/nhan-rieng";
import { useRudiSession } from "../session";
import { luiVeVe } from "../lui-ve";
import { tabTu, thuTuViec, type ViecTao } from "../tao-moi";
import { Heading } from "../ui";
import { NepDien } from "../ui/NepDien";
import { PressScale } from "../ui/PressScale";
import { RudiIcon } from "../ui/RudiIcon";
import { Sheet } from "../ui/Sheet";

/** One line per action; a second line only where two of them could be
 *  confused (a memory goes to the group's wall, a story to friends for a day).
 *  Each is the paper object it makes (ADR-0037 D1, plan S3). This is the
 *  desk's stable order; `tao-moi.ts` moves the one card that fits the tab. */
export const ACTIONS: ViecTao[] = [
  { vat: "lich", title: "Tạo cuộc hẹn", href: "/outings/new" },
  { vat: "hoa-don", title: "Chia hóa đơn", href: "/smart-split/moi/review" },
  { vat: "anh-in", title: "Đăng kỷ niệm", detail: "Ảnh lên tường nhóm", href: "/moments/new" },
  { vat: "polaroid", title: "Đăng story", detail: "Một tấm 24 giờ, chỉ bạn bè thấy", href: "/stories/new" },
  // The one entry the two-person notebook adds here (spec «Nếp truyền giấy»
  // §20.1, Lead): a sheet to ONE person, into the pair's notebook, not the group's.
  { vat: "thu-gap", title: "Hẹn người thương", detail: "Một lời hẹn trong sổ cặp đôi", href: "/hai-nguoi/chon-nguoi" },
  { vat: "bai-viet", title: "Viết bài", detail: "Kể một điều hay với cộng đồng", href: "/community/new" },
];

/** Objects lie on the desk a little askew, the same way every time. */
const NGHIENG = [-3, 2, -2, 3, -1];
const VAT_GIAY: Record<ViecTao["vat"], PaperName> = { lich: "ticket", "hoa-don": "receipt", "anh-in": "photo-stack", polaroid: "postcard", "thu-gap": "envelope", "bai-viet": "document", sticker: "note", "phieu-bau": "document" };

/** A readable receipt splits into two shares; no floating arrow on a blank page. */
function BillShareIllustration() {
  const { colors } = useRudiTheme();
  return <Svg width={76} height={72} viewBox="0 0 76 72" accessible={false} aria-hidden>
    <Path d="M18 4H56V45L52 42L48 45L44 42L40 45L36 42L32 45L28 42L24 45L18 42Z" fill={colors.card} stroke={colors.lineStrong} strokeWidth={1.5} strokeLinejoin="round" />
    <Rect x={25} y={11} width={23} height={5} rx={1} fill={colors.accent} />
    <Path d="M25 23H36M43 23H49M25 29H38M43 29H49M25 35H49" stroke={colors.inkSoft} strokeWidth={1.5} strokeLinecap="round" />
    <Path d="M38 46V50M38 50H18V56M38 50H58V56" fill="none" stroke={colors.accent} strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" />
    {[18, 58].map((x) => <Circle key={x} cx={x} cy={63} r={7} fill={colors.accentSoft} stroke={colors.accent} strokeWidth={1.5} />)}
  </Svg>;
}

/**
 * Same sheet for the tab FAB (`router.push("/create")`) and the `/create` route.
 *
 * Hosted in a transparent route that only fades: the kit `Sheet` does the
 * panel's own motion (spring in, drag or Back or scrim to close, then the
 * route goes back once the panel has left). One sheet in the app, one
 * behaviour; the hand-rolled copy this file used to be had a handle that
 * did nothing.
 */
export function CreateSheet() {
  const router = useRouter();
  const { colors, dark } = useRudiTheme();
  const singleColumn = chuLon(useWindowDimensions().fontScale);
  const palette = usePaperPalette();
  const { phien } = useRudiSession();
  const [open, setOpen] = useState(true);
  const currentGroup = phien?.contexts?.find((group) => group.id === phien.context_id);
  // The tab the stamp was pressed on puts its own kind of thing first
  // (`tao-moi.ts`); somebody who already talks one-to-one with a person gets
  // the two-person entry first where no tab says otherwise (QA 23/09).
  const tu = tabTu(useLocalSearchParams<{ tu?: string }>().tu);
  const coCap = (phien?.contexts ?? []).some((nhom) => laPair(nhom) && nhom.my_state === "active");
  const { viec: cacViec } = thuTuViec({ viec: ACTIONS, tu, coCap, coCongDong: true });
  const subtitle = phien === null
    ? "Đăng nhập để bắt đầu."
    : currentGroup ? `Đang ở ${currentGroup.display_name}.` : "Chọn hội bạn trong bước tiếp theo.";

  return (
    <View style={styles.man}>
      <Sheet accessibilityLabel="Tạo mới" onClose={() => setOpen(false)} onClosed={() => luiVeVe(router as never, "/explore")} open={open} testID="create-sheet">
        <View style={styles.noiDung}>
          <View style={styles.headingRow}>
            <View style={styles.headingText}>
              <Heading size="h2" subtitle={subtitle} title="Mình làm gì tiếp?" />
            </View>
            {/* M1: Nếp walks onto the desk, once a session. */}
            <NepDien khoanhKhac="M1" suKien="khay-tao" />
          </View>
          {/* The desk: the first thing to do lies across it, the rest two by two. */}
          <View style={styles.ban}>
            {cacViec.map((action, i) => (
              <PressScale
                accessibilityHint={action.detail}
                accessibilityLabel={action.title}
                accessibilityRole="button"
                key={action.title}
                onPress={() => router.replace(action.href as never)}
                pressedScale={0.985}
                style={[styles.vat, i === 0 ? styles.vatDan : styles.vatNho, singleColumn && styles.vatMotCot, { backgroundColor: colors.card, borderColor: colors.line }, bongGiay(1, dark)]}
              >
                <View style={[styles.object, { backgroundColor: colors.ground, transform: [{ rotate: `${NGHIENG[i % NGHIENG.length]}deg` }] }]}>
                  {action.vat === "hoa-don" ? <BillShareIllustration /> : <PaperObject name={VAT_GIAY[action.vat]} palette={{ ...palette, paper: colors.card, shade: colors.lineStrong }} width={i === 0 ? 88 : 64} />}
                  {action.vat === "lich" ? <View pointerEvents="none" style={[StyleSheet.absoluteFill, styles.objectMark, styles.ticketMark]}>
                    <RudiIcon color={colors.inkSoft} name="calendar-outline" size={24} />
                  </View> : null}
                </View>
                <View style={i === 0 || singleColumn ? styles.chuDan : styles.chuNho}>
                  <Text style={[typography.title, i === 0 || singleColumn ? null : styles.giua, { color: colors.ink }]}>{action.title}</Text>
                  {action.detail ? <Text style={[typography.note, i === 0 || singleColumn ? null : styles.giua, { color: colors.inkSoft }]}>{action.detail}</Text> : null}
                </View>
              </PressScale>
            ))}
          </View>
        </View>
      </Sheet>
    </View>
  );
}

const styles = StyleSheet.create({
  man: { flex: 1 },
  noiDung: { width: "100%", maxWidth: 560, alignSelf: "center", gap: 18, paddingTop: 2 },
  headingRow: { flexDirection: "row", alignItems: "flex-end", gap: 8 },
  headingText: { flex: 1, gap: 10 },
  object: { padding: 8, borderRadius: 4 },
  objectMark: { alignItems: "center", justifyContent: "center" },
  ticketMark: { paddingLeft: 20 },
  ban: { flexDirection: "row", flexWrap: "wrap", gap: 10 },
  vat: { borderWidth: StyleSheet.hairlineWidth, borderRadius: 12, padding: 12 },
  vatDan: { flexBasis: "100%", flexDirection: "row", alignItems: "center", gap: 14, minHeight: 112 },
  // Two a row: each takes half the desk less half the gap.
  vatNho: { flexGrow: 1, flexBasis: "45%", alignItems: "center", gap: 8, minHeight: 132 },
  vatMotCot: { flexBasis: "100%", flexDirection: "row", alignItems: "center", gap: 14, minHeight: 104 },
  chuDan: { flex: 1, gap: 2 },
  chuNho: { gap: 2, alignSelf: "stretch" },
  giua: { textAlign: "center" },
});
