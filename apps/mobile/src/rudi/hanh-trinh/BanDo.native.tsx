/** Native map host. Web uses BanDo.tsx (maplibre-gl).
 *
 * A static import of `@maplibre/maplibre-react-native` crashes any APK built
 * before the plugin: TurboModuleRegistry.getEnforcing('MLRNCameraModule').
 * Plan tab imports this module, so the whole tab went red. Load MapLibre only
 * when the native binary actually registered the module.
 */

import { useMemo, type ReactElement } from "react";
import { NativeModules, Pressable, StyleSheet, Text, View } from "react-native";

import { typography, useRudiTheme } from "../theme";
import type { BanDoProps } from "./kieu-ban-do";

type BanDoFn = (props: BanDoProps) => ReactElement | null;

function coMapLibreNative(): boolean {
  const n = NativeModules as Record<string, unknown>;
  return Boolean(n.MLRNCameraModule || n.MLRNModule);
}

function layBanDoThat(): BanDoFn | null {
  if (!coMapLibreNative()) return null;
  try {
    return require("./BanDoMapLibre").BanDo as BanDoFn;
  } catch {
    return null;
  }
}

export function BanDo(props: BanDoProps) {
  const That = useMemo(layBanDoThat, []);
  if (That) return <That {...props} />;
  return <BanDoThieu {...props} />;
}

function BanDoThieu({ mauNen, mocs, onNen }: BanDoProps) {
  const { colors } = useRudiTheme();
  return (
    <Pressable
      accessibilityLabel="Bản đồ hành trình — cần bản native có MapLibre"
      onPress={onNen}
      style={[styles.fill, { backgroundColor: mauNen }]}
    >
      <View style={styles.giua}>
        <Text style={[typography.label, { color: colors.ink }]}>Chưa vẽ được bản đồ native</Text>
        <Text style={[typography.caption, { color: colors.inkSoft }]}>
          APK đang chạy chưa gắn MapLibre. Timeline, mốc đánh số và tóm tắt vẫn dùng được. Rebuild dev client rồi bản đồ hiện.
        </Text>
        <Text style={[typography.caption, { color: colors.inkFaint }]}>
          {mocs.length === 0 ? "Chưa có chặng nào có vị trí trên bản đồ" : `${mocs.length} chặng có toạ độ`}
        </Text>
      </View>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  fill: { flex: 1, minHeight: 220 },
  giua: { flex: 1, justifyContent: "center", paddingHorizontal: 24, gap: 8 },
});
