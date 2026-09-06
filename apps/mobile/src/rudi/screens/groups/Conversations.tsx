/**
 * The «Tin nhắn» tab for a real session: the groups this person is in.
 *
 * Read from `GET /people/me/contexts` (ADR-0016) on every focus, so a group
 * created on another phone, or an invitation a friend just sent, shows up
 * without a restart. Rows carry what the server knows -- member count, unread
 * count, the newest message -- and nothing invented.
 *
 * An `invited` row is not a conversation yet: it carries «Đồng ý», and only the
 * press makes the person a member (`vaoNhom`). Tapping an active row makes
 * that group the current one (`chonNhom`, so the money screens read it) and
 * opens its chat (M3); the roster and invite tools sit behind the chat header.
 *
 * On the fixture build (`cheDo !== "live"`) the tab still renders the fixture
 * chat, unchanged, so the default Maestro table keeps its ground.
 *
 * UI v2: rows on the paper with a hairline between them; unread is a mark
 * and a number; loading is the list's own shape; errors keep the list.
 */
import { Ionicons } from "@expo/vector-icons";
import { useFocusEffect, useRouter } from "expo-router";
import { useCallback, useState } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";

import { ApiError, thongDiepNguoiDoc } from "../../../api";
import { docNhomCuaToi, ganDanhSachNhom, chonNhom, vaoNhom, type NhomTomTat, type Phien } from "../../../phien";
import { useRudiSession } from "../../session";
import { typography, useRudiTheme } from "../../theme";
import { Heading, RudiButton, RudiScreen } from "../../ui";
import { EmptyState } from "../../ui/EmptyState";
import { ErrorState } from "../../ui/ErrorState";
import { SkeletonGroup, SkeletonRow } from "../../ui/Skeleton";

type Trang =
  | { pha: "dang-doc" }
  | { pha: "xong"; nhom: NhomTomTat[] }
  | { pha: "hong"; loi: string };

function loiRaChu(error: unknown): string {
  return error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null);
}

/** «Bạn» for the reader's own last message, the roster name for anyone else,
 *  «Rủ Đi AI» for a card with no author -- the way a messenger reads. */
function tenTacGia(tin: { author_id: string | null; author_display_name: string | null }, toi: string): string {
  if (tin.author_id === null) return "Rủ Đi AI";
  if (tin.author_id === toi) return "Bạn";
  return tin.author_display_name ?? "Thành viên";
}

export function ConversationsScreen({ phien }: { phien: Phien }) {
  const router = useRouter();
  const { colors, radius } = useRudiTheme();
  const { datPhien } = useRudiSession();
  const [trang, setTrang] = useState<Trang>({ pha: "dang-doc" });
  const [dangBam, setDangBam] = useState<string | null>(null);

  const nap = useCallback(async () => {
    try {
      const nhom = await docNhomCuaToi(phien.person_id);
      setTrang({ pha: "xong", nhom });
      // Keep the session's own copy fresh too: it is what the empty state and
      // the entry decision read on the next cold start.
      datPhien(await ganDanhSachNhom(phien, nhom));
    } catch (error) {
      setTrang({ pha: "hong", loi: loiRaChu(error) });
    }
    // `phien` changes identity on every `datPhien`; refetching on that would loop.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [phien.person_id]);

  useFocusEffect(
    useCallback(() => {
      void nap();
    }, [nap]),
  );

  const moNhom = async (nhom: NhomTomTat) => {
    setDangBam(nhom.id);
    try {
      const moi = await chonNhom({ ...phien, contexts: trang.pha === "xong" ? trang.nhom : phien.contexts }, nhom.id);
      datPhien(moi);
      router.push(`/groups/${nhom.id}/chat` as never);
    } catch (error) {
      setTrang({ pha: "hong", loi: loiRaChu(error) });
    } finally {
      setDangBam(null);
    }
  };

  const dongY = async (nhom: NhomTomTat) => {
    setDangBam(nhom.id);
    try {
      const daVao = await vaoNhom({
        ...phien,
        context_id: nhom.id,
        membership_state: "invited",
        membership_id: nhom.membership_id,
      });
      datPhien(daVao);
      await nap();
    } catch (error) {
      setTrang({ pha: "hong", loi: loiRaChu(error) });
    } finally {
      setDangBam(null);
    }
  };

  return (
    <RudiScreen bottomInset={112} testID="conversations-screen">
      <View style={styles.dau}>
        <View style={styles.flex}>
          <Heading title="Tin nhắn" subtitle="Nhóm của bạn trên máy chủ" />
        </View>
        {/* Not in the top-right corner: on the development build the
            dev-launcher's floating gear covers it. A compact button beside the
            title is reachable on the build we test on. */}
        <RudiButton compact full={false} icon="add" label="Tạo nhóm" onPress={() => router.push("/groups/new")} variant="outline" />
      </View>
      {trang.pha === "dang-doc" ? (
        <SkeletonGroup>
          <SkeletonRow leading={44} />
          <SkeletonRow leading={44} />
          <SkeletonRow leading={44} />
        </SkeletonGroup>
      ) : null}
      {trang.pha === "hong" ? <ErrorState body={trang.loi} onRetry={() => void nap()} title="Chưa đọc được danh sách nhóm" /> : null}
      {trang.pha === "xong" && trang.nhom.length === 0 ? (
        <EmptyState
          action={{ label: "Tạo nhóm", onPress: () => router.push("/groups/new") }}
          body="Mở một nhóm mới, hoặc nhận lời mời của người đã ở trong nhóm."
          kind="first-use"
          layout="inline"
          secondary={{ label: "Tôi có lời mời", onPress: () => router.push("/moi") }}
          title="Chưa có nhóm nào"
        />
      ) : null}
      {trang.pha === "xong" ? (
        <View>
          {trang.nhom.map((nhom) => {
            const duocMoi = nhom.my_state === "invited";
            return (
              <View key={nhom.id} style={[styles.hang, { borderBottomColor: colors.line }]}>
                <Pressable
                  accessibilityLabel={`Mở nhóm ${nhom.display_name}`}
                  accessibilityRole="button"
                  disabled={nhom.my_state !== "active" || dangBam !== null}
                  onPress={() => void moNhom(nhom)}
                  style={({ pressed }) => [styles.hangChinh, pressed && styles.bam]}
                >
                  <View style={[styles.hinh, { backgroundColor: colors.accentSoft, borderRadius: radius.small }]}>
                    <Ionicons color={colors.accent} name={duocMoi ? "mail-open-outline" : "people-outline"} size={22} />
                  </View>
                  <View style={styles.hangChu}>
                    <Text numberOfLines={1} style={[typography.title, { color: colors.ink }]}>{nhom.display_name}</Text>
                    <Text numberOfLines={1} style={[typography.caption, { color: colors.inkFaint }]}>
                      {nhom.member_count} thành viên
                      {nhom.my_role === "admin" ? " · bạn quản trị" : ""}
                      {duocMoi ? " · bạn được mời" : ""}
                    </Text>
                    <Text numberOfLines={1} style={[typography.caption, { color: nhom.unread_count > 0 ? colors.ink : colors.inkSoft, fontWeight: nhom.unread_count > 0 ? "700" : "600" }]}>
                      {nhom.last_message
                        ? `${tenTacGia(nhom.last_message, phien.person_id)}: ${nhom.last_message.preview}`
                        : "Chưa có tin nhắn nào."}
                    </Text>
                  </View>
                  {nhom.unread_count > 0 ? (
                    <View accessibilityLabel={`${nhom.unread_count} tin chưa đọc`} style={[styles.chuaDoc, { backgroundColor: colors.accent }]}>
                      <Text style={[typography.caption, { color: colors.accentInk }]}>{nhom.unread_count}</Text>
                    </View>
                  ) : null}
                </Pressable>
                {duocMoi ? (
                  <RudiButton
                    compact
                    disabled={dangBam !== null}
                    label="Đồng ý vào nhóm"
                    loading={dangBam === nhom.id}
                    onPress={() => void dongY(nhom)}
                  />
                ) : null}
              </View>
            );
          })}
        </View>
      ) : null}
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  flex: { flex: 1 },
  dau: { flexDirection: "row", alignItems: "flex-start", justifyContent: "space-between", gap: 12 },
  hang: { gap: 10, paddingVertical: 10, borderBottomWidth: StyleSheet.hairlineWidth },
  hangChinh: { flexDirection: "row", alignItems: "center", gap: 12, minHeight: 56 },
  hinh: { width: 44, height: 44, alignItems: "center", justifyContent: "center" },
  hangChu: { flex: 1, gap: 2 },
  chuaDoc: { minWidth: 26, height: 26, borderRadius: 13, alignItems: "center", justifyContent: "center", paddingHorizontal: 8 },
  bam: { opacity: 0.7 },
});
