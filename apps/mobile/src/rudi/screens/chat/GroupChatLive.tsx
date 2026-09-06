/**
 * The group chat, on the real API (M3).
 *
 * Messenger shape: an inverted FlatList (newest at the bottom, older pages on
 * pull-up), one composer pinned above the keyboard, day dividers, bubbles
 * with the author's name from the roster, reaction chips under a bubble and a
 * quick bar on long-press. The companion is a member of the roster called
 * «Rủ Đi AI»: its cards render inline, and a slash command or `@Rủ Đi` is how
 * a person calls on it -- the server answers in the same POST, so what it did
 * (or why it stayed quiet) is shown right under the composer.
 *
 * Keyboard: `KeyboardAvoidingView` with `padding`; the geometry is measured on
 * the emulator (flow 30 + `scripts/do_ban_phim.py`), not assumed from a prop.
 *
 * UI v2 (đợt 5): consecutive messages from one person read as one run --
 * the name once at the top of the run, the initial once at its foot -- so a
 * conversation is a conversation and not a column of identical rows. The
 * header is a title and one line of roster; the AI answers as a sheet of
 * paper in the thread (`TheAi.tsx`), never as a violet advert. Reaction
 * targets are 48dp. The inverted list, paging, pending row and position
 * holding are unchanged: they are the behaviour the review said to keep.
 */
import { Ionicons } from "@expo/vector-icons";
import { Image } from "expo-image";
import { Redirect, useLocalSearchParams, useRouter } from "expo-router";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  FlatList,
  Keyboard,
  KeyboardAvoidingView,
  Platform,
  Pressable,
  StyleSheet,
  Text,
  TextInput,
  View,
} from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";

import { ApiError, taiAnhNhom, thongDiepNguoiDoc } from "../../../api";
import { danhSachThanhVien } from "../../../screens/vao-cua/cong-api";
import {
  PHAN_UNG,
  cauYDinh,
  docTheAi,
  gioPhut,
  glyphPhanUng,
  khoaHang,
  nhomTheoNgay,
  type HangHienThi,
  type LoaiPhanUng,
  type Tin,
} from "../../chat/tin-song";
import { TAT_KAV_QA } from "../../chat/qa-ban-phim";
import { boAnh, chonAnh, nenVaDung } from "../../ky-niem/chon-anh";
import { nguonAnh } from "../../ky-niem/ky-niem";
import { useTinNhan } from "../../chat/useTinNhan";
import { useRudiSession } from "../../session";
import { typography, useRudiTheme } from "../../theme";
import { IconButton, TopBar } from "../../ui";
import { Avatar } from "../../ui/Avatar";
import { EmptyState } from "../../ui/EmptyState";
import { TheAiView } from "./TheAi";

const LENH = [
  { nhan: "/plan", goiY: "/plan tối nay đi đâu?", moTa: "Rủ Đi AI phác lịch trình" },
  { nhan: "/vote", goiY: "/vote Ăn gì? Bún bò | Phở", moTa: "Mở bình chọn: câu hỏi? A | B" },
  { nhan: "/chia-bill", goiY: "/chia-bill", moTa: "Đọc các khoản chi trong tin gần đây" },
  { nhan: "@Rủ Đi", goiY: "@Rủ Đi ", moTa: "Hỏi Rủ Đi AI một câu" },
] as const;

/** A body that calls on the model (not `/vote`, which the server answers itself). */
function goiMoHinh(body: string): boolean {
  return /^\/(plan|chia-?bill)\b/i.test(body) || /@(rủ đi|ru di|rudi)/i.test(body);
}

export function GroupChatLiveScreen({ contextId }: { contextId: string }) {
  const router = useRouter();
  const { colors, radius, space } = useRudiTheme();
  const insets = useSafeAreaInsets();
  const { phien } = useRudiSession();
  const personId = phien?.person_id ?? "";
  const chat = useTinNhan(contextId, personId);
  const [nhap, setNhap] = useState("");
  const [dangGui, setDangGui] = useState(false);
  // The words in flight: drawn as a pending own bubble (and a pending AI row
  // for a command) until the server's rows replace them.
  const [dangGuiThan, setDangGuiThan] = useState<string | null>(null);
  const [banPhimMo, setBanPhimMo] = useState(false);
  // What the server said about the last command, drawn as a row in the thread
  // (where the pending card promised it), signed by who is speaking.
  const [thongBao, setThongBao] = useState<{ tu: string; cau: string; luc: string } | null>(null);
  const [dangChonPhanUng, setDangChonPhanUng] = useState<string | null>(null);
  const [dangGuiAnh, setDangGuiAnh] = useState(false);
  const [tenTheoId, setTenTheoId] = useState<Record<string, string>>({});

  const tenNhom = phien?.contexts?.find((n) => n.id === contextId)?.display_name ?? "Nhóm";

  useEffect(() => {
    const hien = Keyboard.addListener("keyboardDidShow", () => setBanPhimMo(true));
    const an = Keyboard.addListener("keyboardDidHide", () => setBanPhimMo(false));
    return () => {
      hien.remove();
      an.remove();
    };
  }, []);

  useEffect(() => {
    let song = true;
    void danhSachThanhVien(contextId, personId)
      .then((ds) => {
        if (!song) return;
        const map: Record<string, string> = {};
        for (const tv of ds) if (tv.display_name) map[tv.person_id] = tv.display_name;
        setTenTheoId(map);
      })
      .catch(() => undefined);
    return () => {
      song = false;
    };
  }, [contextId, personId]);

  /**
   * The frame for one image message: the read route is permission-checked, so
   * the source carries this reader's headers rather than a bare url.
   */
  const anhTin = useCallback(
    (tin: Tin) => nguonAnh(tin.image_url, personId, contextId),
    [personId, contextId],
  );

  const tenNguoi = useCallback(
    (id: string | null) => (id === null ? "Rủ Đi AI" : id === personId ? "Bạn" : tenTheoId[id] ?? "Thành viên"),
    [tenTheoId, personId],
  );

  const hang = useMemo(() => nhomTheoNgay(chat.tin), [chat.tin]);
  const coChu = nhap.trim().length > 0;
  const moLenh = nhap.startsWith("/") && !nhap.includes(" ") || nhap === "@";

  // The list only auto-scrolls to new rows when the reader is already at the
  // newest end (see autoscrollToTopThreshold); a message you just sent must
  // always come into view, so the send jumps there explicitly.
  const danhSachRef = useRef<FlatList<HangHienThi>>(null);
  // Whether the reader is at the newest end (within a bubble of offset 0 of
  // the inverted list). A new row (the model's answer, a friend's message) and
  // the keyboard opening both pull the list back to the end only then; a
  // reader up in the history keeps their place.
  const ganCuoi = useRef(true);
  const soHang = useRef(chat.tin.length);
  useEffect(() => {
    if (chat.tin.length > soHang.current && ganCuoi.current) danhSachRef.current?.scrollToOffset({ offset: 0, animated: true });
    soHang.current = chat.tin.length;
  }, [chat.tin.length]);
  // The notice under the newest bubble (why the model stayed quiet, a send
  // error) is a list header, not a row: `maintainVisibleContentPosition` keeps
  // row 0 in place and leaves the header under the composer, so it is pulled
  // into view the same way a new row is.
  useEffect(() => {
    if (thongBao !== null && ganCuoi.current) danhSachRef.current?.scrollToOffset({ offset: 0, animated: true });
  }, [thongBao]);
  useEffect(() => {
    const sub = Keyboard.addListener("keyboardDidShow", () => {
      if (ganCuoi.current) danhSachRef.current?.scrollToOffset({ offset: 0, animated: false });
    });
    return () => sub.remove();
  }, []);

  const gui = async () => {
    const body = nhap.trim();
    if (!body || dangGui) return;
    setDangGui(true);
    setDangGuiThan(body);
    setNhap("");
    setThongBao(null);
    try {
      const daGui = await chat.gui(body);
      danhSachRef.current?.scrollToOffset({ offset: 0, animated: true });
      const cau = cauYDinh(daGui);
      const tuAi = daGui.companion !== null && daGui.companion !== undefined && !daGui.companion.spoke;
      setThongBao(cau === null ? null : { tu: tuAi ? "Rủ Đi AI" : "Rủ Đi", cau, luc: new Date().toISOString() });
    } catch (error) {
      // Give the words back: a failed send must not eat what was typed.
      setNhap(body);
      setThongBao({
        tu: "Rủ Đi",
        cau: error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null),
        luc: new Date().toISOString(),
      });
    } finally {
      setDangGui(false);
      setDangGuiThan(null);
    }
  };

  /**
   * Photo into the group, then into the feed.
   *
   * Same two steps the memory wall uses (`nenVaDung` shrinks and cleans up the
   * temp files either way), and the same order as everywhere else: bytes
   * first, message second, so a failed upload leaves nothing behind. Whatever
   * is in the composer rides along as the caption -- one message, not two.
   */
  const guiAnh = async () => {
    if (dangGuiAnh || dangGui) return;
    let daChon = null;
    try {
      daChon = await chonAnh();
    } catch (error) {
      setThongBao({
        tu: "Rủ Đi",
        cau: error instanceof ApiError ? error.message : "Không mở được thư viện ảnh trên máy này.",
        luc: new Date().toISOString(),
      });
      return;
    }
    if (daChon === null) return;
    const caption = nhap.trim();
    setDangGuiAnh(true);
    setThongBao(null);
    try {
      await nenVaDung(daChon, async (anh) => {
        const daTai = await taiAnhNhom(contextId, anh, personId);
        await chat.guiAnhMoi(daTai.url, caption === "" ? null : caption);
      });
      setNhap("");
      danhSachRef.current?.scrollToOffset({ offset: 0, animated: true });
    } catch (error) {
      await boAnh(daChon);
      setThongBao({
        tu: "Rủ Đi",
        cau: error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null),
        luc: new Date().toISOString(),
      });
    } finally {
      setDangGuiAnh(false);
    }
  };

  const phanUng = async (tin: Tin, kind: LoaiPhanUng) => {
    setDangChonPhanUng(null);
    const cuaToi = tin.reactions?.some((r) => r.kind === kind && r.mine) ?? false;
    try {
      await chat.doiPhanUng(tin.id, kind, cuaToi);
    } catch (error) {
      setThongBao({
        tu: "Rủ Đi",
        cau: error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null),
        luc: new Date().toISOString(),
      });
    }
  };

  /** Same human author as the neighbour row (not the AI, not a day divider). */
  const cungNguoi = (a: HangHienThi | undefined, b: HangHienThi | undefined) =>
    a !== undefined && b !== undefined && a.loai === "tin" && b.loai === "tin" &&
    a.tin.kind !== "ai_card" && b.tin.kind !== "ai_card" &&
    a.tin.author_id !== null && a.tin.author_id === b.tin.author_id;

  const renderItem = ({ item, index }: { item: HangHienThi; index: number }) => {
    if (item.loai === "ngay") {
      return (
        <View style={styles.ngay}>
          <View style={[styles.duong, { backgroundColor: colors.line }]} />
          <Text style={[typography.caption, { color: colors.inkFaint }]}>{item.nhan}</Text>
          <View style={[styles.duong, { backgroundColor: colors.line }]} />
        </View>
      );
    }
    const tin = item.tin;
    const cuaToi = tin.author_id === personId;
    const laAi = tin.kind === "ai_card";
    const chips = (tin.reactions ?? []).filter((r) => r.count > 0);
    // Inverted list: index + 1 is the older neighbour, index - 1 the newer.
    const dauChuoi = !cungNguoi(item, hang[index + 1]);
    const cuoiChuoi = !cungNguoi(item, hang[index - 1]);
    return (
      <View
        style={[
          styles.hang,
          cuaToi && !laAi && styles.hangToi,
          // A photo makes the row tall; an initial pinned to the bottom of it
          // floats away from the name it belongs to.
          tin.kind === "image" && styles.hangCao,
        ]}
      >
        {!cuaToi && !laAi ? (
          cuoiChuoi ? <Avatar name={tenNguoi(tin.author_id)} size={30} /> : <View style={styles.choChuDau} />
        ) : null}
        <View style={[styles.khoi, cuaToi && !laAi && styles.khoiToi, laAi && styles.khoiAi]}>
          {!cuaToi && !laAi && dauChuoi ? (
            <Text style={[typography.caption, { color: colors.inkSoft }]}>{tenNguoi(tin.author_id)}</Text>
          ) : null}
          {laAi ? (
            <TheAiView
              the={docTheAi(tin.card)}
              contextId={contextId}
              personId={personId}
              tenNguoi={tenNguoi}
              tacGia={tenNguoi(tin.author_id)}
            />
          ) : (
            <Pressable
              accessibilityLabel={`Tin nhắn: ${tin.body ?? ""}`}
              onLongPress={() => setDangChonPhanUng(tin.id)}
              style={[
                styles.bong,
                {
                  backgroundColor: cuaToi ? colors.accent : colors.card,
                  borderColor: cuaToi ? colors.accent : colors.line,
                },
              ]}
            >
              {tin.kind === "image" ? (
                <View style={styles.anhKhoi}>
                  {anhTin(tin) === null ? (
                    <Text style={[typography.caption, { color: cuaToi ? colors.accentInk : colors.inkFaint }]}>
                      Ảnh này máy không mở được.
                    </Text>
                  ) : (
                    <Image
                      accessibilityLabel={tin.body ? `Ảnh: ${tin.body}` : "Ảnh trong nhóm"}
                      contentFit="cover"
                      source={anhTin(tin)}
                      style={[styles.anh, { borderRadius: radius.small }]}
                    />
                  )}
                  {tin.body ? (
                    <Text style={[typography.body, { color: cuaToi ? colors.accentInk : colors.ink }]}>
                      {tin.body}
                    </Text>
                  ) : null}
                </View>
              ) : (
                <Text style={[typography.body, { color: cuaToi ? colors.accentInk : colors.ink }]}>{tin.body}</Text>
              )}
            </Pressable>
          )}
          <View style={styles.duoiBong}>
            <Text style={[typography.caption, { color: colors.inkFaint }]}>{gioPhut(tin.created_at)}</Text>
            {chips.map((r) => (
              <Pressable
                accessibilityLabel={`${r.count} ${glyphPhanUng(r.kind)}`}
                hitSlop={8}
                key={r.kind}
                onPress={() => void phanUng(tin, r.kind)}
                style={[
                  styles.chip,
                  { borderColor: r.mine ? colors.accent : colors.line, backgroundColor: colors.card },
                ]}
              >
                <Text style={typography.caption}>
                  {glyphPhanUng(r.kind)} {r.count}
                </Text>
              </Pressable>
            ))}
          </View>
          {dangChonPhanUng === tin.id ? (
            <View style={[styles.thanhPhanUng, { backgroundColor: colors.card, borderColor: colors.line }]}>
              {PHAN_UNG.map((p) => (
                <Pressable
                  accessibilityLabel={p.nhan}
                  key={p.kind}
                  onPress={() => void phanUng(tin, p.kind)}
                  style={styles.nutPhanUng}
                >
                  <Text style={styles.glyph}>{p.glyph}</Text>
                </Pressable>
              ))}
            </View>
          ) : null}
        </View>
      </View>
    );
  };

  if (phien === null) return <Redirect href="/welcome" />;

  return (
    <KeyboardAvoidingView
      behavior={Platform.OS === "ios" ? "padding" : "height"}
      // Off only under the QA negative control; see `qa-ban-phim.ts`.
      enabled={!TAT_KAV_QA}
      style={[styles.man, { backgroundColor: colors.ground, paddingTop: insets.top }]}
    >
      {/* A bar with an edge: scrolled rows disappear under a defined line, not
          into bare ground. */}
      <View style={[styles.dau, { paddingHorizontal: space.md, borderBottomColor: colors.line }]}>
        <TopBar title={tenNhom} />
        {/* The roster lives under the title, not in the top-right corner: on a
            development build the dev-launcher's floating gear covers that
            corner, and a target nobody can reach on the build we test on is a
            target nobody has tested. Messenger puts group info here too. */}
        <Pressable
          accessibilityLabel="Thành viên nhóm"
          accessibilityRole="button"
          onPress={() => router.push(`/groups/${contextId}/members` as never)}
          style={({ pressed }) => [styles.thanhVien, pressed && styles.mo]}
        >
          <Ionicons color={colors.inkFaint} name="people-outline" size={15} />
          <Text style={[typography.caption, { color: colors.inkSoft }]}>
            {Object.keys(tenTheoId).length || 1} thành viên · xem và mời
          </Text>
          <Ionicons color={colors.inkFaint} name="chevron-forward" size={14} />
        </Pressable>
      </View>
      {/* Drawn outside the inverted list: the list flips its own children
          back upright, and an extra flip here once mirrored this copy. */}
      {!chat.dangNap && chat.tin.length === 0 && dangGuiThan === null ? (
        <View style={[styles.rong, { paddingHorizontal: space.md }]}>
          <EmptyState body="Nhắn gì đó cho hội, hoặc gõ / để rủ Rủ Đi AI vào." kind="first-use" layout="inline" title="Chưa có tin nhắn nào" />
        </View>
      ) : null}
      <FlatList
        ref={danhSachRef}
        contentContainerStyle={[styles.danhSach, { paddingHorizontal: space.md }]}
        data={hang}
        inverted
        keyExtractor={khoaHang}
        // Inverted, so the header sits at the newest end: what is being sent
        // shows there at once, and a command shows the model is being asked.
        ListHeaderComponent={
          dangGuiThan === null && thongBao !== null ? (
            <View style={styles.hang}>
              <View style={[styles.khoi, styles.khoiAi]}>
                <View style={[styles.choAi, { backgroundColor: colors.card, borderColor: colors.line, borderRadius: radius.base }]}>
                  <View style={styles.dauAi}>
                    <Ionicons color={colors.ai} name="sparkles" size={15} />
                    <Text style={[typography.caption, { color: colors.ai }]}>{thongBao.tu}</Text>
                  </View>
                  <Text style={[typography.body, { color: colors.ink }]}>{thongBao.cau}</Text>
                </View>
                <Text style={[typography.caption, { color: colors.inkFaint }]}>{gioPhut(thongBao.luc)}</Text>
              </View>
            </View>
          ) : dangGuiThan !== null ? (
            <View style={styles.choGui}>
              <View style={[styles.hang, styles.hangToi]}>
                <View style={[styles.khoi, styles.khoiToi, styles.mo]}>
                  <View style={[styles.bong, { backgroundColor: colors.accent, borderColor: colors.accent }]}>
                    <Text style={[typography.body, { color: colors.accentInk }]}>{dangGuiThan}</Text>
                  </View>
                  <Text style={[typography.caption, { color: colors.inkFaint }]}>Đang gửi...</Text>
                </View>
              </View>
              {goiMoHinh(dangGuiThan) ? (
                <View style={styles.hang}>
                  <View style={[styles.khoi, styles.khoiAi]}>
                    <View style={[styles.choAi, { backgroundColor: colors.card, borderColor: colors.line, borderRadius: radius.base }]}>
                      <View style={styles.dauAi}>
                        <Ionicons color={colors.ai} name="sparkles" size={15} />
                        <Text style={[typography.caption, { color: colors.ai }]}>Đang hỏi Rủ Đi AI...</Text>
                      </View>
                      <Text style={[typography.caption, { color: colors.inkSoft }]}>
                        Câu trả lời sẽ hiện ở đây trong vài giây, hoặc lý do nó không trả lời.
                      </Text>
                    </View>
                  </View>
                </View>
              ) : null}
            </View>
          ) : null
        }
        ListFooterComponent={
          chat.dangNapCu ? (
            <Text style={[typography.caption, styles.giua, { color: colors.inkFaint }]}>Đang tải tin cũ...</Text>
          ) : null
        }
        // Hold the reader's place while older pages load above, but when they
        // are within a bubble of the newest end, new rows (own sends, the AI's
        // answer, a friend's message) scroll into view instead of landing
        // under the composer.
        maintainVisibleContentPosition={{ minIndexForVisible: 0, autoscrollToTopThreshold: 120 }}
        // Content grows at the newest end (a row, the pending bubble, the
        // notice header): while the reader is at the end, stay at the end.
        onContentSizeChange={() => {
          if (ganCuoi.current) danhSachRef.current?.scrollToOffset({ offset: 0, animated: false });
        }}
        onEndReached={() => void chat.napCuHon()}
        onEndReachedThreshold={0.6}
        onScroll={(e) => {
          ganCuoi.current = e.nativeEvent.contentOffset.y <= 120;
        }}
        renderItem={renderItem}
        scrollEventThrottle={64}
        testID="chat-list"
      />
      {chat.loi ? (
        <Text style={[typography.caption, { color: colors.warn, paddingHorizontal: space.md }]}>{chat.loi}</Text>
      ) : null}
      {moLenh ? (
        <View style={[styles.lenh, { backgroundColor: colors.card, borderColor: colors.line, marginHorizontal: space.md }]}>
          {LENH.map((l) => (
            <Pressable accessibilityRole="button" key={l.nhan} onPress={() => setNhap(l.goiY)} style={styles.lenhHang}>
              <Text style={[typography.label, { color: colors.accent }]}>{l.nhan}</Text>
              <Text style={[typography.caption, { color: colors.inkSoft }]}>{l.moTa}</Text>
            </Pressable>
          ))}
        </View>
      ) : null}
      <View
        style={[
          styles.soan,
          {
            backgroundColor: colors.card,
            borderColor: colors.line,
            marginHorizontal: space.md,
            // With the keyboard up the IME covers the navigation bar, so the
            // bottom inset would only float the composer above the keys.
            marginBottom: banPhimMo ? 6 : Math.max(insets.bottom, 8),
          },
        ]}
      >
        <IconButton
          accessibilityLabel="Gửi ảnh"
          disabled={dangGuiAnh || dangGui}
          icon="image-outline"
          loading={dangGuiAnh}
          onPress={() => void guiAnh()}
          quiet
        />
        <TextInput
          accessibilityLabel="Ô soạn tin"
          cursorColor={colors.accent}
          multiline
          onChangeText={setNhap}
          placeholder="Nhắn cho hội, hoặc gõ /"
          placeholderTextColor={colors.inkFaint}
          selectionColor={colors.accentSoft}
          style={[typography.body, styles.oNhap, { color: colors.ink }]}
          value={nhap}
        />
        <IconButton
          accessibilityLabel="Gửi tin nhắn"
          dim={!coChu && !dangGui}
          disabled={!coChu && !dangGui}
          icon="arrow-up"
          loading={dangGui}
          onPress={() => void gui()}
          solid={coChu || dangGui}
        />
      </View>
    </KeyboardAvoidingView>
  );
}

const styles = StyleSheet.create({
  man: { flex: 1 },
  anhKhoi: { gap: 6 },
  anh: { width: 208, height: 208 },
  thanhVien: { alignSelf: "center", flexDirection: "row", alignItems: "center", gap: 5, minHeight: 40, paddingHorizontal: 8, marginTop: -8 },
  danhSach: { paddingVertical: 12, gap: 12 },
  ngay: { flexDirection: "row", alignItems: "center", gap: 10, paddingVertical: 6 },
  duong: { flex: 1, height: StyleSheet.hairlineWidth },
  hang: { flexDirection: "row", alignItems: "flex-end", gap: 8 },
  hangToi: { justifyContent: "flex-end" },
  hangCao: { alignItems: "flex-start" },
  khoi: { maxWidth: "82%", gap: 4 },
  khoiToi: { alignItems: "flex-end" },
  khoiAi: { maxWidth: "100%", flex: 1 },
  choChuDau: { width: 30, height: 30 },
  bong: { borderWidth: 1, borderRadius: 17, paddingHorizontal: 13, paddingVertical: 10 },
  duoiBong: { flexDirection: "row", alignItems: "center", gap: 6, flexWrap: "wrap" },
  chip: { minHeight: 36, justifyContent: "center", borderWidth: 1, borderRadius: 999, paddingHorizontal: 10, paddingVertical: 2 },
  thanhPhanUng: { flexDirection: "row", gap: 4, borderWidth: 1, borderRadius: 999, padding: 4, alignSelf: "flex-start" },
  nutPhanUng: { width: 48, height: 48, alignItems: "center", justifyContent: "center" },
  glyph: { fontSize: 20 },
  dau: { paddingBottom: 10, borderBottomWidth: StyleSheet.hairlineWidth },
  rong: { paddingVertical: 24 },
  choGui: { gap: 12 },
  mo: { opacity: 0.62 },
  choAi: { gap: 6, padding: 14, borderWidth: 1 },
  dauAi: { flexDirection: "row", alignItems: "center", gap: 6 },
  giua: { textAlign: "center", paddingVertical: 8 },
  lenh: { borderWidth: 1, borderRadius: 16, padding: 6, gap: 2 },
  lenhHang: { paddingHorizontal: 10, paddingVertical: 8, gap: 1 },
  soan: { flexDirection: "row", alignItems: "flex-end", gap: 6, padding: 6, borderWidth: 1, borderRadius: 22 },
  oNhap: { flex: 1, maxHeight: 120, paddingHorizontal: 10, paddingVertical: 8 },
});
