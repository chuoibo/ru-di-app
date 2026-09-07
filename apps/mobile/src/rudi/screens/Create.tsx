import { Ionicons } from "@expo/vector-icons";
import { useRouter } from "expo-router";
import { useState } from "react";
import { StyleSheet, Text, View } from "react-native";

import { type RudiTone, toneColor, toneSoftColor, typography, useRudiTheme } from "../theme";
import { DEMO_GROUP } from "../fixtures";
import { useRudiSession } from "../session";
import { DemoBadge, Heading, IconName } from "../ui";
import { PressScale } from "../ui/PressScale";
import { Sheet } from "../ui/Sheet";

const ACTIONS: { icon: IconName; title: string; detail: string; href: string; tone: RudiTone }[] = [
  { icon: "calendar-outline", title: "Tạo cuộc hẹn", detail: "Chốt thời gian, nơi đi và hội bạn", href: "/outings/new", tone: "accent" },
  { icon: "receipt-outline", title: "Chia hóa đơn", detail: "Xem lại ảnh bill và gán món", href: "/smart-split/xom-leo/review", tone: "split" },
  { icon: "images-outline", title: "Đăng kỷ niệm", detail: "Chia sẻ ảnh vào tường nhóm", href: "/moments/new", tone: "accent" },
  { icon: "aperture-outline", title: "Đăng story", detail: "Một tấm ảnh 24 giờ, chỉ bạn bè thấy", href: "/stories/new", tone: "accent" },
];

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
  const { colors } = useRudiTheme();
  const { phien } = useRudiSession();
  const [open, setOpen] = useState(true);
  const currentGroup = phien?.contexts?.find((group) => group.id === phien.context_id);
  const subtitle = phien === null
    ? `Bắt đầu với ${DEMO_GROUP.name}.`
    : currentGroup ? `Đang ở ${currentGroup.display_name}.` : "Chọn hội bạn trong bước tiếp theo.";

  return (
    <View style={styles.man}>
      <Sheet accessibilityLabel="Tạo mới" onClose={() => setOpen(false)} onClosed={() => router.back()} open={open} testID="create-sheet">
        <View style={styles.noiDung}>
          <View style={styles.headingRow}>
            <Heading size="h2" subtitle={subtitle} title="Mình làm gì tiếp?" />
            {phien === null ? <DemoBadge /> : null}
          </View>
          <View style={styles.actions}>
            {ACTIONS.map((action) => (
              <PressScale
                accessibilityRole="button"
                key={action.title}
                onPress={() => router.replace(action.href as never)}
                pressedScale={0.985}
                style={[styles.action, { borderColor: colors.line }]}
              >
                <View style={[styles.icon, { backgroundColor: toneSoftColor(colors, action.tone) }]}>
                  <Ionicons color={toneColor(colors, action.tone)} name={action.icon} size={24} />
                </View>
                <View style={styles.actionText}>
                  <Text style={[typography.title, { color: colors.ink }]}>{action.title}</Text>
                  <Text style={[typography.caption, { color: colors.inkFaint }]}>{action.detail}</Text>
                </View>
                <Ionicons color={colors.inkFaint} name="arrow-forward" size={20} />
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
  headingRow: { gap: 10 },
  actions: { gap: 4 },
  action: {
    minHeight: 72,
    flexDirection: "row",
    alignItems: "center",
    gap: 13,
    paddingVertical: 10,
    borderBottomWidth: StyleSheet.hairlineWidth,
  },
  icon: { width: 48, height: 48, borderRadius: 16, alignItems: "center", justifyContent: "center" },
  actionText: { flex: 1, gap: 2 },
});
