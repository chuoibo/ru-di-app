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
 * UI v2: rows on the paper with a hairline between them; unread is a mark
 * and a number; loading is the list's own shape; errors keep the list.
 */
import { Ionicons } from "@expo/vector-icons";
import { HoiTaiHang } from "../../ui/HoiTaiHang";
import { useFocusEffect, useRouter } from "expo-router";
import { useCallback, useEffect, useRef, useState } from "react";
import { AppState, Pressable, StyleSheet, View } from "react-native";
import { NativeFontMetricsProvider, Text, useNativeFontMetricsOwner, useNativeStandaloneTextMetrics } from "../../ui/NativeFontMetrics";
import { useNativeFontScale } from "../../chia-bill/useBillFontScale";

import { ApiError, newAttempt, thongDiepNguoiDoc } from "../../../api";
import { docNhomCuaToi, ganDanhSachNhom, chonNhom, vaoNhom, type NhomTomTat, type Phien } from "../../../phien";
import { apiV2 } from "../../chat/e2ee/api-v2";
import { gopTomTatV2 } from "../../chat/e2ee/tom-tat";
import { xemTruocV2 } from "../../chat/e2ee/useTinNhanV2";
import { gioPhut, tacGiaTin, xemTruocTinCuoi } from "../../chat/tin-song";
import { laPair, tenCuocTroChuyen } from "../../nhan-rieng/nhan-rieng";
import { cauLoiMoi, docLoiMoiNhom, loiMoiTruoc, tuChoiLoiMoiNhom, type LoiMoiNhom } from "../../nhom/loi-moi-nhom";
import { useNepNguCanh } from "../../nep/NepProvider";
import { useRudiSession } from "../../session";
import { StoryRail } from "../story/StoryRail";
import { bangMauChat, bongGiay, phuMau, typography, useRudiTheme } from "../../theme";
import { Field, IconButton, RudiButton, RudiScreen } from "../../ui";
import { khopTenCuocTroChuyen } from "../../nhom/tim-cuoc-tro-chuyen";
import { EmptyState } from "../../ui/EmptyState";
import { AssetIllustration } from "../../asset-pack/react-native/AssetIllustration";
import { CauTaiCho } from "../../ui/CauTaiCho";
import { ErrorState } from "../../ui/ErrorState";
import { AvatarNguoi } from "../../ui/AvatarNguoi";
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
  const tacGia = tacGiaTin(tin);
  if (tacGia.loai === "ai") return "Rủ Đi AI";
  if (tacGia.id === toi) return "Bạn";
  return tin.author_display_name ?? "Thành viên";
}

export function ConversationsScreen({ phien }: { phien: Phien }) {
  const owned = useNativeFontMetricsOwner();
  return owned ? <ConversationsContent phien={phien} /> : <ObserveConversationMetrics phien={phien} />;
}

function ObserveConversationMetrics({ phien }: { phien: Phien }) {
  const snapshot = useNativeFontScale();
  return <NativeFontMetricsProvider snapshot={snapshot}><ConversationsContent phien={phien} /></NativeFontMetricsProvider>;
}

function ConversationsContent({ phien }: { phien: Phien }) {
  const router = useRouter();
  const { colors, radius } = useRudiTheme();
  const { datPhien } = useRudiSession();
  const phienMoiNhat = useRef(phien);
  phienMoiNhat.current = phien;
  const conMo = useRef(true);
  useEffect(() => {
    conMo.current = true;
    return () => { conMo.current = false; };
  }, []);
  const [trang, setTrang] = useState<Trang>({ pha: "dang-doc" });
  const [query, setQuery] = useState("");
  const actionLock = useRef(false);
  const [readError, setReadError] = useState<string | null>(null);
  const [dangBam, setDangBam] = useState<string | null>(null);
  // QA UI-080: who invited, read for each open invitation; and an answer that
  // failed is said under its own row, the list stays (QA UI-077 pattern).
  const [loiMoi, setLoiMoi] = useState<Record<string, LoiMoiNhom | null>>({});
  const [hoiTuChoi, setHoiTuChoi] = useState<string | null>(null);
  const [loiHang, setLoiHang] = useState<{ id: string; cau: string } | null>(null);
  useNepNguCanh({ man: "messages", tieuDe: "Tin nhắn", goiY: ["Rủ ai đó đi chơi tuần này", "Cuộc hẹn nào sắp tới?"] });

  const nap = useCallback(async () => {
    const lucBatDau = phienMoiNhat.current;
    try {
      // End-to-end rooms take their last message and unread count from the
      // lane's summary and their words from this device (tom-tat.ts); a host
      // without the lane answers nothing and the list is the legacy one.
      const [nhom, rooms] = await Promise.all([
        docNhomCuaToi(lucBatDau.person_id),
        apiV2.summaries(lucBatDau.person_id).catch(() => []),
      ]);
      const xemTruoc = await xemTruocV2(lucBatDau.person_id, rooms.map((r) => r.conversation_id)).catch(() => ({}));
      if (!conMo.current || phienMoiNhat.current.token !== lucBatDau.token) return;
      setReadError(null);
      setTrang({ pha: "xong", nhom: gopTomTatV2(nhom, rooms, xemTruoc) });
      // An invitation that cannot be read still shows, without a name.
      for (const n of nhom.filter((x) => x.my_state === "invited" && !laPair(x))) {
        docLoiMoiNhom(n.id, phien.person_id)
          .then((lm) => setLoiMoi((cu) => ({ ...cu, [n.id]: lm })))
          .catch(() => setLoiMoi((cu) => ({ ...cu, [n.id]: null })));
      }
      // Keep the session's own copy fresh too: it is what the empty state and
      // the entry decision read on the next cold start.
      // A mounted tab keeps this callback across group selections. Merge into
      // the current session, never the one captured before the user chose.
      const moi = await ganDanhSachNhom(phienMoiNhat.current, nhom);
      if (conMo.current && phienMoiNhat.current.token === lucBatDau.token) datPhien(moi);
    } catch (error) {
      if (!conMo.current || phienMoiNhat.current.token !== lucBatDau.token) return;
      setReadError(loiRaChu(error));
      setTrang((held) => held.pha === "xong" ? held : { pha: "hong", loi: loiRaChu(error) });
    }
    // `phien` changes identity on every `datPhien`; refetching on that would loop.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [phien.person_id]);

  useFocusEffect(
    useCallback(() => {
      void nap();
    }, [nap]),
  );
  // Focus alone missed the common case: the app was already on this tab when
  // the other person sent the invitation, and coming back from the background
  // fires no focus event (QA 23/09: an invitee saw nothing until re-login).
  useEffect(() => {
    const dangKy = AppState.addEventListener("change", (tt) => {
      if (tt === "active") void nap();
    });
    return () => dangKy.remove();
  }, [nap]);

  const moNhom = async (nhom: NhomTomTat) => {
    if (actionLock.current) return;
    actionLock.current = true;
    setDangBam(nhom.id);
    try {
      // A pair is never the current group (ADR-0021 §2.5): the money screens
      // keep the group they had, and only the chat opens.
      if (!laPair(nhom)) {
        const moi = await chonNhom({ ...phien, contexts: trang.pha === "xong" ? trang.nhom : phien.contexts }, nhom.id);
        datPhien(moi);
      }
      router.push(`/groups/${nhom.id}/chat` as never);
    } catch (error) {
      setLoiHang({ id: nhom.id, cau: loiRaChu(error) });
    } finally {
      actionLock.current = false;
      setDangBam(null);
    }
  };

  const dongY = async (nhom: NhomTomTat) => {
    if (actionLock.current) return;
    actionLock.current = true;
    setDangBam(nhom.id);
    setLoiHang(null);
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
      setLoiHang({ id: nhom.id, cau: loiRaChu(error) });
    } finally {
      actionLock.current = false;
      setDangBam(null);
    }
  };

  const tuChoi = async (nhom: NhomTomTat) => {
    if (actionLock.current) return;
    actionLock.current = true;
    setDangBam(nhom.id);
    setLoiHang(null);
    try {
      await tuChoiLoiMoiNhom(nhom.id, phien.person_id, newAttempt());
      setHoiTuChoi(null);
      await nap();
    } catch (error) {
      setLoiHang({ id: nhom.id, cau: loiRaChu(error) });
    } finally {
      actionLock.current = false;
      setDangBam(null);
    }
  };

  const matching = trang.pha === "xong" ? loiMoiTruoc(trang.nhom).filter((nhom) => khopTenCuocTroChuyen(tenCuocTroChuyen(nhom), query)) : [];
  const bodyMetrics = useNativeStandaloneTextMetrics(typography.body);
  // An unusable inherited owner keeps the readable reflow without a child observer.
  const largeText = bodyMetrics === null || (bodyMetrics.fontSize ?? typography.body.fontSize) >= typography.body.fontSize * 1.25;

  return (
    <RudiScreen material="quiet" cot="doc" bottomInset="tab" onRefresh={nap} testID="conversations-screen">
      <View style={styles.dau}>
        <View style={styles.flex}>
          <Text accessibilityRole="header" style={[typography.h1, { color: colors.ink }]}>Tin nhắn</Text>
        </View>
        <IconButton accessibilityLabel="Tạo nhóm" icon="add" onPress={() => router.push("/groups/new")} quiet />
      </View>
      <Field nativeMetrics icon="search-outline" returnKeyType="search" accessibilityLabel="Tìm cuộc trò chuyện" placeholder="Tìm người hoặc nhóm" value={query} onChangeText={setQuery} autoCorrect={false} trailing={query ? <IconButton accessibilityLabel="Xoá tìm kiếm" icon="close" onPress={() => setQuery("")} quiet /> : undefined} />
      {/* L4 (ADR-0022 §2.3): the story rail, drawn from GET /stories on every focus. */}
      <StoryRail nativeMetrics personId={phien.person_id} />
      {readError && trang.pha === "xong" ? <CauTaiCho cau={`Chưa cập nhật được danh sách. ${readError}`} hanhDong={{ label: "Đọc lại", onPress: () => void nap() }} /> : null}
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
          nativeMetrics
          action={{ label: "Tạo nhóm", onPress: () => router.push("/groups/new") }}
          body="Mở một nhóm cho cả hội, nhận lời mời của người đã ở trong nhóm, hoặc kết bạn bằng tên tài khoản để nhắn riêng và rủ một người đi chơi."
          illustration={<AssetIllustration name="spot-keeps" width={220} height={170} />}
          kind="first-use"
          layout="inline"
          title="Chưa có nhóm nào"
        />
      ) : null}
      {trang.pha === "xong" && trang.nhom.length > 0 && matching.length === 0 ? <View style={styles.khongTimThay}>
        <Text style={[typography.title, { color: colors.ink }]}>Chưa tìm thấy cuộc trò chuyện</Text>
        <Text style={[typography.body, { color: colors.inkSoft }]}>Thử tên khác hoặc xoá tìm kiếm để xem tất cả.</Text>
        <RudiButton compact full={false} label="Xoá tìm kiếm" variant="ghost" onPress={() => setQuery("")} />
      </View> : null}
      {trang.pha === "xong" && trang.nhom.length === 0 ? (
        <RudiButton icon="person-add-outline" label="Thêm bạn bằng tên tài khoản" onPress={() => router.push("/friends/add")} variant="ghost" />
      ) : null}
      {trang.pha === "xong" ? (
        <View>
          {matching.map((nhom) => {
            const duocMoi = nhom.my_state === "invited";
            return (
              <View key={nhom.id} style={[styles.hang, { borderBottomColor: colors.line }]}>
                <Pressable
                  accessibilityLabel={laPair(nhom) ? `Mở cuộc trò chuyện với ${tenCuocTroChuyen(nhom)}` : `Mở nhóm ${tenCuocTroChuyen(nhom)}`}
                  accessibilityRole="button"
                  disabled={nhom.my_state !== "active" || dangBam !== null}
                  onPress={() => void moNhom(nhom)}
                  style={({ pressed }) => [styles.hangChinh, pressed && styles.bam]}
                >
                  {/* A pair (ADR-0021 §2.5) is the other person, so their avatar
                      stands where a group shows the roster glyph. */}
                  {laPair(nhom) ? (
                    <AvatarNguoi name={tenCuocTroChuyen(nhom)} personId={nhom.counterpart?.id} size={44} />
                  ) : (
                    duocMoi ? (
                      // An invitation is a letter not yet opened.
                      <View style={[styles.hinh, { backgroundColor: colors.accentSoft, borderRadius: radius.small }]}>
                        <Ionicons color={colors.accent} name="mail-open-outline" size={22} />
                      </View>
                    ) : (
                      // A group is its notebook seen from the shelf (ADR-0037 D1): the
                      // spine in the group's own chat colour, its initial on it.
                      <GaySo ten={tenCuocTroChuyen(nhom)} theme={nhom.theme} />
                    )
                  )}
                  <View style={[styles.hangNoi, largeText && styles.hangNoiDoc]}>
                    <View style={[styles.hangChu, largeText && styles.hangChuDoc]}>
                      <Text numberOfLines={largeText ? undefined : 2} style={[typography.title, { color: colors.ink }]}>{tenCuocTroChuyen(nhom)}</Text>
                      <Text numberOfLines={largeText ? undefined : 1} style={[typography.note, { color: nhom.unread_count > 0 ? colors.ink : colors.inkSoft, fontWeight: nhom.unread_count > 0 ? "700" : "400" }]}>
                        {nhom.last_message
                          ? `${tenTacGia(nhom.last_message, phien.person_id)}: ${xemTruocTinCuoi(nhom.last_message)}`
                          : "Chưa có tin nhắn nào."}
                      </Text>
                      <Text numberOfLines={largeText ? undefined : 1} style={[typography.note, { color: colors.inkFaint }]}>
                        {duocMoi && !laPair(nhom)
                          ? cauLoiMoi(loiMoi[nhom.id] ?? null, nhom.member_count)
                          : nhom.unavailable === true
                            ? "Không còn nhận tin"
                            : laPair(nhom)
                              ? "Nhắn riêng"
                              : `${nhom.member_count} thành viên`}
                        {!duocMoi && !laPair(nhom) && nhom.my_role === "admin" ? " · bạn quản trị" : ""}
                      </Text>
                    </View>
                    <View style={[styles.meta, largeText && styles.metaDoc]}>
                      {nhom.last_message && Number.isFinite(Date.parse(nhom.last_message.created_at)) ? <Text style={[typography.note, styles.gio, { color: colors.inkSoft }]}>{gioPhut(nhom.last_message.created_at)}</Text> : null}
                      {nhom.unread_count > 0 ? (
                        <View accessibilityLabel={`${nhom.unread_count} tin chưa đọc`} style={[styles.chuaDoc, { backgroundColor: colors.accent }]}>
                          <Text style={[typography.caption, styles.soChuaDoc, { color: colors.accentInk }]}>{nhom.unread_count}</Text>
                        </View>
                      ) : null}
                    </View>
                  </View>
                </Pressable>
                {duocMoi && hoiTuChoi !== nhom.id ? (
                  // Two answers side by side (QA UI-080): yes, or no.
                  <View style={styles.traLoi}>
                    <RudiButton
                      accessibilityLabel={`Đồng ý vào nhóm ${tenCuocTroChuyen(nhom)}`}
                      compact
                      disabled={dangBam !== null}
                      full={false}
                      label="Đồng ý vào nhóm"
                      loading={dangBam === nhom.id}
                      onPress={() => void dongY(nhom)}
                    />
                    <RudiButton
                      accessibilityLabel={`Từ chối lời mời vào nhóm ${tenCuocTroChuyen(nhom)}`}
                      compact
                      disabled={dangBam !== null}
                      full={false}
                      label="Từ chối"
                      onPress={() => { setHoiTuChoi(nhom.id); setLoiHang(null); }}
                      variant="ghost"
                    />
                  </View>
                ) : null}
                {duocMoi && hoiTuChoi === nhom.id ? (
                  // Asked in the row, like every answer that removes something.
                  <View style={styles.hoi}>
                    <HoiTaiHang
                      nativeMetrics
                      cau={`Từ chối lời mời vào ${tenCuocTroChuyen(nhom)}? Nếu đổi ý, bạn cần được mời lại.`}
                      dangLam={dangBam === nhom.id}
                      nhan="Từ chối"
                      onDongY={() => void tuChoi(nhom)}
                      onThoi={() => setHoiTuChoi(null)}
                      testID={`tin-nhan-hoi-tu-choi-${nhom.id}`}
                    />
                  </View>
                ) : null}
                {loiHang?.id === nhom.id ? <CauTaiCho cau={loiHang.cau} co="nho" /> : null}
              </View>
            );
          })}
        </View>
      ) : null}
    </RudiScreen>
  );
}

/** A notebook's spine on the shelf: cloth in the group's chat colour, a darker hinge, the initial. */
function GaySo({ ten, theme }: { ten: string; theme?: string }) {
  const { colors, dark } = useRudiTheme();
  const mau = bangMauChat(theme, dark);
  const chu = ten.trim().charAt(0).toLocaleUpperCase("vi") || "?";
  return (
    <View accessibilityElementsHidden importantForAccessibility="no-hide-descendants" style={styles.gayKhung}>
      <View style={[styles.gay, { backgroundColor: mau.bubble, borderColor: colors.lineStrong }, bongGiay(1, dark)]}>
        <View style={[styles.gayBanLe, { backgroundColor: phuMau(colors.ink, dark ? 0.3 : 0.22) }]} />
        <Text allowFontScaling={false} style={[typography.title, { color: mau.bubbleInk }]}>{chu}</Text>
        <View style={[styles.gayNhan, { backgroundColor: colors.card }]} />
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  gayKhung: { width: 44, height: 48, alignItems: "center", justifyContent: "center" },
  gay: { width: 34, height: 48, borderRadius: 3, borderWidth: StyleSheet.hairlineWidth, alignItems: "center", justifyContent: "center", overflow: "hidden", paddingLeft: 4 },
  gayBanLe: { position: "absolute", left: 0, top: 0, bottom: 0, width: 5 },
  gayNhan: { position: "absolute", bottom: 7, left: 10, right: 5, height: 4, borderRadius: 1 },
  flex: { flex: 1, minWidth: 0 },
  dau: { flexDirection: "row", alignItems: "center", justifyContent: "space-between", gap: 12 },
  khongTimThay: { gap: 8, paddingVertical: 24 },
  hang: { gap: 10, paddingVertical: 10, borderBottomWidth: StyleSheet.hairlineWidth },
  hangChinh: { flexDirection: "row", alignItems: "flex-start", gap: 12, minHeight: 56 },
  // Answers start under the text column (avatar 44 + gap 12).
  traLoi: { flexDirection: "row", flexWrap: "wrap", alignItems: "center", gap: 8, paddingLeft: 56 },
  hoi: { gap: 4, paddingLeft: 56 },
  khongThut: { paddingLeft: 0 },
  hinh: { width: 44, height: 44, alignItems: "center", justifyContent: "center" },
  hangNoi: { flex: 1, minWidth: 0, flexDirection: "row", alignItems: "flex-start", gap: 8 },
  hangNoiDoc: { flexDirection: "column" },
  hangChu: { flex: 1, minWidth: 0, gap: 4 },
  hangChuDoc: { flex: 0, alignSelf: "stretch" },
  meta: { alignItems: "flex-end", flexShrink: 0, gap: 6 },
  metaDoc: { alignSelf: "stretch", flexDirection: "row", alignItems: "center", justifyContent: "space-between" },
  gio: { fontVariant: ["tabular-nums"] },
  chuaDoc: { minWidth: 26, minHeight: 26, borderRadius: 13, alignItems: "center", justifyContent: "center", paddingHorizontal: 8, paddingVertical: 4 },
  soChuaDoc: { textAlign: "center", fontVariant: ["tabular-nums"] },
  bam: { opacity: 0.7 },
});
