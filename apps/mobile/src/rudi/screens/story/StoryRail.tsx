/**
 * The story rail at the top of «Tin nhắn» (L4, ADR-0022 §2.3).
 *
 * Reads `GET /stories` on every focus and draws exactly what came back, in
 * the server's order: one ring per author, an accent ring while something is
 * unseen, a quiet one once it is all seen. The first tile is always «Đăng
 * story»: the composer must stay reachable when the feed is empty or when the
 * request failed, so a broken rail never hides the way to post.
 */
import { Ionicons } from "@expo/vector-icons";
import { useFocusEffect, useRouter } from "expo-router";
import { useCallback, useRef, useState } from "react";
import { Pressable, ScrollView, StyleSheet, Text as RNText, View } from "react-native";
import { Text as NativeMetricText, useNativeStandaloneTextMetrics } from "../../ui/NativeFontMetrics";

import { ApiError, thongDiepNguoiDoc } from "../../../api";
import { docStories, laCuaToi, nhanVong, type NhomStoryWire } from "../../story/story";
import { typography, useRudiTheme } from "../../theme";
import { AvatarNguoi } from "../../ui/AvatarNguoi";
import { RudiButton } from "../../ui";
import { Skeleton } from "../../ui/Skeleton";

type Trang = { pha: "dang-doc" } | { pha: "xong"; nhom: NhomStoryWire[] } | { pha: "hong"; loi: string };

const CO = 56;

export function StoryRail({ personId, nativeMetrics = false }: { personId: string; nativeMetrics?: boolean }) {
  const router = useRouter();
  const { colors } = useRudiTheme();
  // Shared callers keep RN scaling; the Inbox opts into its existing owner.
  const Text = nativeMetrics ? NativeMetricText : RNText;
  const labelMetrics = useNativeStandaloneTextMetrics(typography.caption);
  const largeText = nativeMetrics && (labelMetrics === null || (labelMetrics.fontSize ?? typography.caption.fontSize) >= typography.caption.fontSize * 1.25);
  const tileStyle = [styles.o, largeText && styles.oLarge];
  const labelStyle = nativeMetrics ? styles.label : undefined;
  const live = useRef(true);
  const [loiDoc, setLoiDoc] = useState<string | null>(null);
  const [trang, setTrang] = useState<Trang>({ pha: "dang-doc" });

  const nap = useCallback(async () => {
    try {
      const dai = await docStories(personId);
      if (!live.current) return;
      setLoiDoc(null);
      setTrang({ pha: "xong", nhom: dai.authors });
    } catch (error) {
      if (!live.current) return;
      const loi = error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null);
      setLoiDoc(loi);
      setTrang((held) => held.pha === "xong" ? held : { pha: "hong", loi });
    }
  }, [personId]);

  useFocusEffect(
    useCallback(() => {
      live.current = true;
      void nap();
      return () => { live.current = false; };
    }, [nap]),
  );

  return (
    <View accessibilityLabel="Dải story" style={styles.khung} testID="story-rail">
      <ScrollView contentContainerStyle={styles.hang} horizontal showsHorizontalScrollIndicator={false}>
        <Pressable
          // Not the same words as the caption under it: a driver (and a screen
          // reader) must find exactly one node for this tile.
          accessibilityLabel="Đăng story mới"
          accessibilityRole="button"
          onPress={() => router.push("/stories/new" as never)}
          style={tileStyle}
        >
          <View style={[styles.vong, { width: CO + 8, height: CO + 8, borderRadius: (CO + 8) / 2, borderColor: colors.lineStrong, borderStyle: "dashed" }]}>
            <View style={[styles.them, { backgroundColor: colors.accentSoft, width: CO, height: CO, borderRadius: CO / 2 }]}>
              <Ionicons color={colors.accent} name="add" size={26} />
            </View>
          </View>
          <Text numberOfLines={nativeMetrics ? undefined : 1} style={[typography.caption, labelStyle, { color: colors.ink }]}>Đăng story</Text>
        </Pressable>
        {trang.pha === "dang-doc"
          ? [0, 1, 2].map((i) => (
              <View key={i} style={tileStyle}>
                <Skeleton height={CO + 8} radius={(CO + 8) / 2} width={CO + 8} />
                <Skeleton height={12} width={48} />
              </View>
            ))
          : null}
        {trang.pha === "xong"
          ? trang.nhom.map((nhom) => {
              const cuaToi = laCuaToi(nhom, personId);
              const chuaXem = !nhom.all_seen && !cuaToi;
              return (
                <Pressable
                  accessibilityLabel={nhanVong(nhom, personId)}
                  accessibilityRole="button"
                  key={nhom.author.id}
                  onPress={() => router.push(`/stories/${nhom.author.id}` as never)}
                  style={tileStyle}
                >
                  <View
                    style={[
                      styles.vong,
                      {
                        width: CO + 8,
                        height: CO + 8,
                        borderRadius: (CO + 8) / 2,
                        borderColor: chuaXem ? colors.accent : colors.lineStrong,
                        borderWidth: chuaXem ? 3 : 1.5,
                      },
                    ]}
                  >
                    <AvatarNguoi name={cuaToi ? "Bạn" : nhom.author.display_name} personId={nhom.author.id} size={CO} />
                  </View>
                  <Text numberOfLines={nativeMetrics ? undefined : 1} style={[typography.caption, labelStyle, { color: colors.ink }]}>
                    {cuaToi ? "Bạn" : nhom.author.display_name}
                  </Text>
                </Pressable>
              );
            })
          : null}
      </ScrollView>
      {loiDoc ? <><Text accessibilityLiveRegion="polite" style={[typography.caption, { color: colors.inkSoft }]}>Chưa cập nhật được story: {loiDoc}</Text><RudiButton label="Đọc lại story" variant="ghost" compact full={false} onPress={() => void nap()} /></> : null}
    </View>
  );
}

const styles = StyleSheet.create({
  khung: { gap: 4 },
  hang: { flexDirection: "row", gap: 12, paddingVertical: 4 },
  o: { alignItems: "center", gap: 4, width: CO + 16 },
  oLarge: { width: 112 },
  label: { alignSelf: "stretch", minWidth: 0, textAlign: "center" },
  vong: { alignItems: "center", justifyContent: "center", borderWidth: 1.5 },
  them: { alignItems: "center", justifyContent: "center" },
});
