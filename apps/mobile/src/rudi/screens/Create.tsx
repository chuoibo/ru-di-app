import { useLocalSearchParams, useRouter } from "expo-router";
import { useState } from "react";
import { StyleSheet, View } from "react-native";

import { useNativeFontScale } from "../chia-bill/useBillFontScale";
import { typography, useRudiTheme } from "../theme";
import { laPair } from "../nhan-rieng/nhan-rieng";
import { useRudiSession } from "../session";
import { luiVeVe } from "../lui-ve";
import { tabTu, thuTuViec, type ViecTao } from "../tao-moi";
import { NativeFontMetricsProvider, Text, useNativeFontMetricsOwner, useNativeStandaloneTextMetrics } from "../ui/NativeFontMetrics";
import { NepDien } from "../ui/NepDien";
import { PressScale } from "../ui/PressScale";
import { RudiIcon, type IconName } from "../ui/RudiIcon";
import { Sheet } from "../ui/Sheet";

/** Stable actions; the current tab moves only its own action to the front. */
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

const BIEU_TUONG: Record<ViecTao["vat"], IconName> = {
  lich: "calendar-outline",
  "hoa-don": "receipt-outline",
  "anh-in": "image-outline",
  polaroid: "camera-outline",
  "thu-gap": "mail-outline",
  "bai-viet": "create-outline",
  sticker: "happy-outline",
  "phieu-bau": "bar-chart-outline",
};

/** One owner includes native fallback, so inherited surfaces never subscribe twice. */
export function CreateSheet() {
  const owned = useNativeFontMetricsOwner();
  return owned ? <CreateSheetContent /> : <ObserveCreateMetrics />;
}

function ObserveCreateMetrics() {
  const snapshot = useNativeFontScale();
  return <NativeFontMetricsProvider snapshot={snapshot}><CreateSheetContent /></NativeFontMetricsProvider>;
}

/**
 * The transparent route keeps the kit Sheet's drag, Back, scrim and close
 * animation. Font changes update the open surface without replacing it.
 */
function CreateSheetContent() {
  const router = useRouter();
  const { colors } = useRudiTheme();
  const labelMetrics = useNativeStandaloneTextMetrics(typography.label);
  // A decorative cue yields its width when labels grow or metrics are unavailable.
  const spacious = labelMetrics === null ||
    (labelMetrics.fontSize ?? typography.label.fontSize) >= typography.label.fontSize * 1.25;
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
      <Sheet accessibilityLabel="Tạo mới" onClose={() => setOpen(false)} onClosed={() => luiVeVe(router as never, "/explore")} open={open} showsVerticalScrollIndicator testID="create-sheet">
        <View style={styles.noiDung}>
          <View style={styles.heading}>
            <View style={styles.headingRow}>
              <Text accessibilityRole="header" style={[typography.h2, styles.headingText, { color: colors.ink }]}>Tạo mới</Text>
              {/* The existing once-per-session moment keeps a small, 48dp skip target. */}
              <NepDien khoanhKhac="M1" suKien="khay-tao" co={48} />
            </View>
            <Text style={[typography.note, { color: colors.inkSoft }]}>{subtitle}</Text>
          </View>
          <View style={[styles.ban, { backgroundColor: colors.card, borderColor: colors.line }]}>
            {cacViec.map((action, i) => (
              <PressScale
                accessibilityHint={action.detail}
                accessibilityLabel={action.title}
                accessibilityRole="button"
                key={action.title}
                onPress={() => router.replace(action.href as never)}
                pressedScale={0.985}
                style={[styles.vat, i > 0 && styles.vatTiep, { borderColor: colors.line }]}
              >
                <View style={[styles.icon, { backgroundColor: i === 0 ? colors.accentSoft : colors.ground }]}>
                  <RudiIcon color={i === 0 ? colors.accent : colors.inkSoft} name={BIEU_TUONG[action.vat]} size={22} />
                </View>
                <View style={styles.chu}>
                  <Text style={[typography.label, { color: colors.ink }]}>{action.title}</Text>
                  {action.detail ? <Text style={[typography.note, { color: colors.inkSoft }]}>{action.detail}</Text> : null}
                </View>
                {!spacious ? <RudiIcon color={colors.inkSoft} name="chevron-forward" size={16} /> : null}
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
  noiDung: { width: "100%", maxWidth: 560, alignSelf: "center", gap: 12, paddingTop: 2 },
  heading: { gap: 2 },
  headingRow: { flexDirection: "row", alignItems: "center", gap: 12 },
  headingText: { flex: 1, minWidth: 0 },
  ban: { borderWidth: StyleSheet.hairlineWidth, borderRadius: 12 },
  vat: { width: "100%", minHeight: 56, flexDirection: "row", alignItems: "center", gap: 12, paddingHorizontal: 12, paddingVertical: 8 },
  vatTiep: { borderTopWidth: StyleSheet.hairlineWidth },
  icon: { width: 36, height: 36, flexShrink: 0, alignItems: "center", justifyContent: "center", borderRadius: 8 },
  chu: { flex: 1, minWidth: 0, gap: 2 },
});
