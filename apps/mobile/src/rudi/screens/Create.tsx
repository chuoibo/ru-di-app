import { useRouter } from "expo-router";
import { useState } from "react";
import { StyleSheet, Text, View } from "react-native";

import { type VatBan, KHUNG_VAT, hinhVat } from "../art/vat-ban";
import { bongGiay, typography, useRudiTheme } from "../theme";
import { DEMO_GROUP } from "../fixtures";
import { laPair } from "../nhan-rieng/nhan-rieng";
import { useRudiSession } from "../session";
import { DemoBadge, Heading } from "../ui";
import { VeLop } from "../ui/art/VeLop";
import { NepDien } from "../ui/NepDien";
import { PressScale } from "../ui/PressScale";
import { Sheet } from "../ui/Sheet";

/** One line per action; a second line only where two of them could be
 *  confused (a memory goes to the group's wall, a story to friends for a day).
 *  Each is the paper object it makes (ADR-0037 D1, plan S3). */
const ACTIONS: { vat: VatBan; title: string; detail?: string; href: string }[] = [
  { vat: "lich", title: "Tạo cuộc hẹn", href: "/outings/new" },
  { vat: "hoa-don", title: "Chia hóa đơn", href: "/smart-split/xom-leo/review" },
  { vat: "anh-in", title: "Đăng kỷ niệm", detail: "Ảnh lên tường nhóm", href: "/moments/new" },
  { vat: "polaroid", title: "Đăng story", detail: "Một tấm 24 giờ, chỉ bạn bè thấy", href: "/stories/new" },
  // The one entry the two-person notebook adds here (spec «Nếp truyền giấy»
  // §20.1, Lead): a sheet to ONE person, into the pair's notebook, not the group's.
  { vat: "thu-gap", title: "Rủ một người đi chơi", detail: "Một tờ giấy cho hai người, mỗi tuần", href: "/hai-nguoi/chon-nguoi" },
];

/** Objects lie on the desk a little askew, the same way every time. */
const NGHIENG = [-3, 2, -2, 3, -1];

const RU_MOT_NGUOI = "/hai-nguoi/chon-nguoi";

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
  const { phien } = useRudiSession();
  const [open, setOpen] = useState(true);
  const currentGroup = phien?.contexts?.find((group) => group.id === phien.context_id);
  // Somebody who already talks one-to-one with a person gets the two-person
  // entry first: it was fifth, under the bill and the story, and a couple read
  // past it (QA 23/09). Everybody else keeps the group order.
  const coCap = (phien?.contexts ?? []).some((nhom) => laPair(nhom) && nhom.my_state === "active");
  const cacViec = coCap ? [...ACTIONS.filter((a) => a.href === RU_MOT_NGUOI), ...ACTIONS.filter((a) => a.href !== RU_MOT_NGUOI)] : ACTIONS;
  const subtitle = phien === null
    ? `Bắt đầu với ${DEMO_GROUP.name}.`
    : currentGroup ? `Đang ở ${currentGroup.display_name}.` : "Chọn hội bạn trong bước tiếp theo.";

  return (
    <View style={styles.man}>
      <Sheet accessibilityLabel="Tạo mới" onClose={() => setOpen(false)} onClosed={() => router.back()} open={open} testID="create-sheet">
        <View style={styles.noiDung}>
          <View style={styles.headingRow}>
            <View style={styles.headingText}>
              <Heading size="h2" subtitle={subtitle} title="Mình làm gì tiếp?" />
              {phien === null ? <DemoBadge /> : null}
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
                style={[styles.vat, i === 0 ? styles.vatDan : styles.vatNho, { backgroundColor: colors.card, borderColor: colors.lineStrong }, bongGiay(1, dark)]}
              >
                <VeLop height={i === 0 ? 72 : 64} khungH={KHUNG_VAT} khungW={KHUNG_VAT} lop={hinhVat(action.vat)} style={{ transform: [{ rotate: `${NGHIENG[i % NGHIENG.length]}deg` }] }} width={i === 0 ? 72 : 64} />
                <View style={i === 0 ? styles.chuDan : styles.chuNho}>
                  <Text style={[typography.title, i === 0 ? null : styles.giua, { color: colors.ink }]}>{action.title}</Text>
                  {action.detail ? <Text style={[typography.note, i === 0 ? null : styles.giua, { color: colors.inkSoft }]}>{action.detail}</Text> : null}
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
  ban: { flexDirection: "row", flexWrap: "wrap", gap: 10 },
  vat: { borderWidth: 1, borderRadius: 8, padding: 12 },
  vatDan: { flexBasis: "100%", flexDirection: "row", alignItems: "center", gap: 14, minHeight: 96 },
  // Two a row: each takes half the desk less half the gap.
  vatNho: { flexGrow: 1, flexBasis: "45%", alignItems: "center", gap: 8, minHeight: 132 },
  chuDan: { flex: 1, gap: 2 },
  chuNho: { gap: 2, alignSelf: "stretch" },
  giua: { textAlign: "center" },
});
