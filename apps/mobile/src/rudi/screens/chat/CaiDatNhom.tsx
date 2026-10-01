import { useRouter } from "expo-router";
import { useEffect, useRef, useState } from "react";
import { Ionicons } from "@expo/vector-icons";
import { Pressable, StyleSheet, Switch, Text, View } from "react-native";

import { ApiError, attemptFor, thongDiepNguoiDoc, type Attempt } from "../../../api";
import { coGiDeDoi, doiNhom, roiNhom } from "../../chat/nhom-cai-dat";
import { THEME_CHAT, bangMauChat, nhanTheme, type ThemeChat } from "../../mau-chat";
import { typography, useRudiTheme } from "../../theme";
import { Field, Heading, ListRow, RudiButton } from "../../ui";
import { Sheet } from "../../ui/Sheet";
import { toggleState } from "../../../ui/a11y";

export type NhomCaiDat = {
  id: string;
  display_name: string;
  theme?: string;
  kind?: "group" | "pair";
  ai_auto_suggest?: boolean;
};

/**
 * Group settings (ADR-0021 §2.4) as a sheet over the chat: rename, choose a
 * theme, the roster door, and «Rời nhóm» behind a confirmation step. A pair
 * (nhắn riêng, L2) has no name of its own and no roster to manage, so those
 * rows are hidden and only the theme stays.
 *
 * Every write is its own attempt (`attemptFor`), held in a ref so a re-render
 * cannot turn a retry into a second write.
 */
export function CaiDatNhomSheet({
  open,
  onClose,
  personId,
  nhom,
  onDaDoi,
  aiTuGoiY,
}: {
  open: boolean;
  onClose: () => void;
  personId: string;
  nhom: NhomCaiDat;
  onDaDoi: (thay: Partial<NhomCaiDat>) => void;
  /** L6 wires this; until then the switch is not drawn. */
  aiTuGoiY?: { bat: boolean; onDoi: (bat: boolean) => void };
}) {
  const router = useRouter();
  const { colors, dark, radius } = useRudiTheme();
  const [ten, setTen] = useState(nhom.display_name);
  const [dangLuu, setDangLuu] = useState<"ten" | "theme" | "roi" | null>(null);
  const [loi, setLoi] = useState<string | null>(null);
  const [xacNhanRoi, setXacNhanRoi] = useState(false);
  const attempts = useRef<Record<string, Attempt>>({});
  const laPair = nhom.kind === "pair";
  const mauChon = bangMauChat(nhom.theme, dark);

  useEffect(() => {
    if (open) {
      setTen(nhom.display_name);
      setLoi(null);
      setXacNhanRoi(false);
    }
  }, [open, nhom.display_name]);

  const baoLoi = (error: unknown) =>
    setLoi(error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null));

  const luuTen = async () => {
    if (!coGiDeDoi({ display_name: ten }) || dangLuu !== null) return;
    setDangLuu("ten");
    setLoi(null);
    try {
      const moi = await doiNhom(nhom.id, personId, { display_name: ten }, attemptFor(attempts.current, `ten:${ten.trim()}`));
      onDaDoi({ display_name: moi.display_name, theme: moi.theme });
    } catch (error) {
      baoLoi(error);
    } finally {
      setDangLuu(null);
    }
  };

  const chonTheme = async (slug: ThemeChat) => {
    if (dangLuu !== null || slug === nhom.theme) return;
    setDangLuu("theme");
    setLoi(null);
    try {
      const moi = await doiNhom(nhom.id, personId, { theme: slug }, attemptFor(attempts.current, `theme:${slug}`));
      onDaDoi({ theme: moi.theme, display_name: moi.display_name });
    } catch (error) {
      baoLoi(error);
    } finally {
      setDangLuu(null);
    }
  };

  const roi = async () => {
    if (dangLuu !== null) return;
    setDangLuu("roi");
    setLoi(null);
    try {
      await roiNhom(nhom.id, personId, personId, attemptFor(attempts.current, "roi"));
      // The chat screen goes away with the group; the sheet goes with it. Not
      // `onClose()` first: closing a sheet while its screen is being replaced
      // is the sequence that crashed Fabric (see `ui/Sheet.tsx`).
      router.replace("/messages" as never);
    } catch (error) {
      baoLoi(error);
    } finally {
      setDangLuu(null);
    }
  };

  return (
    <Sheet accessibilityLabel="Cài đặt nhóm" onClose={onClose} open={open} testID="cai-dat-nhom">
      <Heading size="h2" title={laPair ? "Cuộc trò chuyện" : "Cài đặt nhóm"} />
      {!laPair ? (
        <View style={styles.khoi}>
          <Field accessibilityLabel="Ô tên nhóm" label="Tên nhóm" onChangeText={setTen} value={ten} />
          {/* ADR-0038 §2.2: the save appears once there is something to
              save, instead of a greyed button under an unchanged name. */}
          {ten.trim() !== nhom.display_name ? (
            <RudiButton
              compact
              disabled={!coGiDeDoi({ display_name: ten })}
              full={false}
              label="Lưu tên"
              loading={dangLuu === "ten"}
              lyDo="Tên nhóm không để trống được."
              onPress={() => void luuTen()}
              variant="soft"
            />
          ) : null}
        </View>
      ) : null}
      <Text style={[typography.label, { color: colors.ink }]}>Màu bong bóng</Text>
      {/* A corner of the chat page in the chosen theme, drawn by the same
          palette the chat itself reads (`bangMauChat`): what the swatch picks
          is what the group will see, their bubble beside yours. */}
      <View accessibilityLabel={`Xem trước: ${nhanTheme(nhom.theme ?? "mac-dinh")}`} accessible style={[styles.xemTruoc, { backgroundColor: colors.ground, borderColor: colors.line, borderRadius: radius.control }]} testID="xem-truoc-bong-bong">
        <View style={[styles.bong, styles.bongHo, { backgroundColor: colors.card, borderColor: colors.line }]}>
          <Text style={[typography.note, { color: colors.ink }]}>Tối nay đi đâu?</Text>
        </View>
        <View style={[styles.bong, styles.bongToi, { backgroundColor: mauChon.bubble, borderColor: mauChon.bubble }]} testID="bong-bong-mau">
          <Text style={[typography.note, { color: mauChon.bubbleInk }]}>Phố cổ nhé!</Text>
        </View>
      </View>
      <View accessibilityRole="radiogroup" style={styles.themes}>
        {THEME_CHAT.map((slug) => {
          const mau = bangMauChat(slug, dark);
          const chon = (nhom.theme ?? "mac-dinh") === slug;
          return (
            <Pressable
              accessibilityLabel={`Theme ${nhanTheme(slug)}`}
              {...toggleState("radio", chon, () => void chonTheme(slug))}
              key={slug}
              onPress={() => void chonTheme(slug)}
              style={[
                styles.oTheme,
                {
                  borderRadius: radius.control,
                  backgroundColor: mau.bubble,
                  // The theme's own accent is often the bubble's colour, so a
                  // border in it vanished (blind read, S9): the chosen swatch
                  // is ringed in ink and carries a tick.
                  borderColor: chon ? colors.ink : colors.lineStrong,
                  borderWidth: chon ? 3 : 1,
                },
              ]}
            >
              {chon ? <Ionicons color={mau.bubbleInk} name="checkmark" size={16} /> : null}
              <Text style={[typography.label, { color: mau.bubbleInk }]}>Aa</Text>
            </Pressable>
          );
        })}
      </View>
      <Text style={[typography.caption, { color: colors.inkFaint }]}>
        {nhanTheme(nhom.theme ?? "mac-dinh")}. Cả nhóm thấy cùng một màu.
      </Text>
      {aiTuGoiY ? (
        <View style={styles.hangCongTac}>
          <View style={styles.hangChu}>
            <Text style={[typography.label, { color: colors.ai }]}>Rủ Đi AI tự gợi ý</Text>
            <Text style={[typography.caption, { color: colors.inkSoft }]}>
              Khi thấy nhóm hỏi ăn gì, đi đâu, Rủ Đi AI gợi ý mà không cần gọi.
            </Text>
          </View>
          <Switch
            accessibilityLabel="Rủ Đi AI tự gợi ý"
            onValueChange={aiTuGoiY.onDoi}
            thumbColor={colors.card}
            trackColor={{ true: colors.ai, false: colors.line }}
            value={aiTuGoiY.bat}
          />
        </View>
      ) : null}
      {laPair ? (
        <ListRow
          icon="document-text-outline"
          onPress={() => {
            onClose();
            router.push(`/groups/${nhom.id}/to-giay` as never);
          }}
          subtitle="Loại sổ, hai ô ràng buộc, đóng sổ."
          title="Tờ giấy của hai mình"
        />
      ) : null}
      {!laPair ? (
        <ListRow
          icon="people-outline"
          onPress={() => {
            onClose();
            router.push(`/groups/${nhom.id}/members` as never);
          }}
          subtitle="Xem ai đang ở trong nhóm, mời thêm, đổi vai trò."
          title="Thành viên"
        />
      ) : null}
      {loi ? <Text style={[typography.caption, { color: colors.warn }]}>{loi}</Text> : null}
      {/* Leaving sits apart, under a rule, well away from the rows above. */}
      {!laPair ? <View style={[styles.ke, { backgroundColor: colors.line }]} /> : null}
      {!laPair ? (
        xacNhanRoi ? (
          <View style={styles.khoi}>
            <Text style={[typography.body, { color: colors.ink }]}>Rời nhóm này? Bạn sẽ không đọc được tin và sổ của nhóm nữa.</Text>
            <RudiButton label="Rời nhóm" loading={dangLuu === "roi"} onPress={() => void roi()} variant="outline" />
            <RudiButton label="Ở lại" onPress={() => setXacNhanRoi(false)} variant="ghost" />
          </View>
        ) : (
          <RudiButton icon="exit-outline" label="Rời nhóm" onPress={() => setXacNhanRoi(true)} variant="ghost" />
        )
      ) : null}
    </Sheet>
  );
}

const styles = StyleSheet.create({
  khoi: { gap: 8, paddingBottom: 8 },
  themes: { flexDirection: "row", flexWrap: "wrap", gap: 10, paddingVertical: 8 },
  oTheme: { width: 52, height: 52, alignItems: "center", justifyContent: "center" },
  hangCongTac: { flexDirection: "row", alignItems: "center", gap: 12, paddingVertical: 8 },
  hangChu: { flex: 1, gap: 2 },
  xemTruoc: { borderWidth: StyleSheet.hairlineWidth, padding: 12, gap: 6 },
  bong: { borderWidth: 1, borderRadius: 16, paddingHorizontal: 12, paddingVertical: 7, maxWidth: "78%" },
  bongHo: { alignSelf: "flex-start", borderBottomLeftRadius: 4 },
  bongToi: { alignSelf: "flex-end", borderBottomRightRadius: 4 },
  ke: { height: StyleSheet.hairlineWidth, marginTop: 24, marginBottom: 4 },
});
