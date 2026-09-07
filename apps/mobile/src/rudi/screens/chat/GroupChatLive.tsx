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
 *
 * Social v1.1 (ADR-0021): stickers from a closed vocabulary, a reply quote
 * the server builds, taking back one's own message, the group's settings
 * sheet (name, bubble theme, members, leave), and a pair -- two friends'
 * private thread -- drawn by the same screen with the other person's name.
 */
import { Ionicons } from "@expo/vector-icons";
import { Image } from "expo-image";
import { Redirect, useLocalSearchParams, useRouter } from "expo-router";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  FlatList,
  Keyboard,
  KeyboardAvoidingView,
  type NativeScrollEvent,
  type NativeSyntheticEvent,
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
  cauYDinh,
  docTheAi,
  gioPhut,
  glyphPhanUng,
  khoaHang,
  nhomTheoNgay,
  trichTu,
  type HangHienThi,
  type LoaiPhanUng,
  type Tin,
  type TrichDan,
} from "../../chat/tin-song";
import { TAT_KAV_QA } from "../../chat/qa-ban-phim";
import { boAnh, chonAnh, nenVaDung } from "../../ky-niem/chon-anh";
import { nguonAnh } from "../../ky-niem/ky-niem";
import { useTinNhan } from "../../chat/useTinNhan";
import { laPair, tenCuocTroChuyen } from "../../nhan-rieng/nhan-rieng";
import { useRudiSession } from "../../session";
import { bangMauChat, typography, useRudiTheme } from "../../theme";
import { IconButton, TopBar } from "../../ui";
import { Avatar } from "../../ui/Avatar";
import { EmptyState } from "../../ui/EmptyState";
import { Sticker } from "../../ui/stickers/Sticker";
import { Sheet } from "../../ui/Sheet";
import { CaiDatNhomSheet } from "./CaiDatNhom";
import { KhaySticker } from "./KhaySticker";
import { NoiDungBaoCao } from "../nguoi/NoiDungBaoCao";
import { MenuTin } from "./MenuTin";
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
  const { colors, dark, radius, space } = useRudiTheme();
  const insets = useSafeAreaInsets();
  const { phien, datPhien } = useRudiSession();
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
  // Social v1.1 (ADR-0021): the sticker tray, the long-press menu of one
  // message, the message being replied to, and the group settings sheet.
  const [khaySticker, setKhaySticker] = useState(false);
  const [menuTin, setMenuTin] = useState<Tin | null>(null);
  // ADR-0023 §2.4: báo cáo một tin nhắn. Khay riêng, mở sau khi khay menu
  // đóng, để hai khay không chồng lên nhau trên màn nhỏ.
  const [baoCaoTin, setBaoCaoTin] = useState<Tin | null>(null);
  const [traLoi, setTraLoi] = useState<TrichDan | null>(null);
  const [caiDatMo, setCaiDatMo] = useState(false);
  const [dangGuiAnh, setDangGuiAnh] = useState(false);
  const [tenTheoId, setTenTheoId] = useState<Record<string, string>>({});

  const nhom = phien?.contexts?.find((n) => n.id === contextId);
  // ADR-0023 §2.3.2: bị chặn, hoặc người kia đã xoá tài khoản. Tin cũ vẫn
  // đọc được -- chúng cũng là của người kia -- nhưng cửa soạn tin đóng, và
  // câu nói ra KHÔNG cho biết vì lý do nào trong hai lý do.
  const khongNhanTin = nhom?.unavailable === true;
  const tenNhom = tenCuocTroChuyen(nhom);
  // A pair (ADR-0021 §2.5) has no roster to show or invite into; the pill in
  // that place opens the other person's profile instead.
  const nhanRieng = laPair(nhom);
  const nguoiKiaId = nhom?.counterpart?.id;
  // The group's theme colours only the sender's bubble and the reader's own
  // reaction chip; the screen's leading tone stays the brand accent.
  const mauChat = bangMauChat(nhom?.theme, dark);

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
  // The same fact as state, because it decides a prop: the list anchors the
  // reader's place only while they are up in the history.
  const [oCuoi, setOCuoi] = useState(true);
  const ghiViTri = useCallback((e: NativeSyntheticEvent<NativeScrollEvent>) => {
    const gan = e.nativeEvent.contentOffset.y <= 120;
    ganCuoi.current = gan;
    setOCuoi(gan);
  }, []);
  // Back to the end before a row of ours lands, and say so: with the anchor
  // off, the new row and the notice under it sit at offset 0 by construction.
  const veCuoi = useCallback(() => {
    ganCuoi.current = true;
    setOCuoi(true);
    danhSachRef.current?.scrollToOffset({ offset: 0, animated: false });
  }, []);
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
    const traLoiId = traLoi?.id ?? null;
    try {
      const daGui = await chat.gui(body, traLoiId);
      setTraLoi(null);
      veCuoi();
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
      veCuoi();
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

  /** One sticker from the tray, optionally as a reply to the quoted message. */
  const guiStickerChon = async (id: string) => {
    setKhaySticker(false);
    if (dangGui) return;
    setThongBao(null);
    try {
      await chat.guiSticker(id, traLoi?.id ?? null);
      setTraLoi(null);
      veCuoi();
    } catch (error) {
      setThongBao({
        tu: "Rủ Đi",
        cau: error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null),
        luc: new Date().toISOString(),
      });
    }
  };

  /** Take back one's own message; the server's refusal is shown as its sentence. */
  const xoaTinChon = async (tin: Tin) => {
    setMenuTin(null);
    try {
      await chat.xoaTin(tin.id);
    } catch (error) {
      setThongBao({
        tu: "Rủ Đi",
        cau: error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null),
        luc: new Date().toISOString(),
      });
    }
  };

  /** Keep the session's copy of the group current so the header and theme follow at once. */
  const nhomDaDoi = (thay: { display_name?: string; theme?: string }) => {
    if (phien === null || !phien.contexts) return;
    datPhien({ ...phien, contexts: phien.contexts.map((n) => (n.id === contextId ? { ...n, ...thay } : n)) });
  };

  const phanUng = async (tin: Tin, kind: LoaiPhanUng) => {
    setMenuTin(null);
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
    const laSticker = tin.kind === "sticker";
    const daXoa = tin.kind === "deleted";
    const chips = (tin.reactions ?? []).filter((r) => r.count > 0);
    // The theme colours the sender's bubble; a taken-back row is paper again.
    const nenBong = daXoa ? colors.card : cuaToi ? mauChat.bubble : colors.card;
    const vienBong = daXoa ? colors.line : cuaToi ? mauChat.bubble : colors.line;
    const mucBong = cuaToi && !daXoa ? mauChat.bubbleInk : colors.ink;
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
          {/* The quote sits ABOVE the bubble, in the block, never in the
              flex-wrapped row under it: a lone Text at the end of a wrapping
              row keeps one word's width (bài học RN 2026-09-04). A deleted
              row keeps no quote either: what it answered went with it. */}
          {tin.reply_to && !laAi && !daXoa ? (
            <View
              accessibilityLabel={`Trích: ${tin.reply_to.preview}`}
              style={[styles.trich, { borderLeftColor: mauChat.accent, backgroundColor: colors.card, borderColor: colors.line }]}
            >
              <Text style={[typography.caption, { color: colors.inkSoft }]}>{tenNguoi(tin.reply_to.author_id)}</Text>
              <Text numberOfLines={1} style={[typography.caption, { color: colors.ink }]}>
                {tin.reply_to.preview}
              </Text>
            </View>
          ) : null}
          {laAi ? (
            <TheAiView
              the={docTheAi(tin.card)}
              contextId={contextId}
              personId={personId}
              tenNguoi={tenNguoi}
              tacGia={tenNguoi(tin.author_id)}
            />
          ) : laSticker ? (
            <Pressable accessibilityLabel="Tin nhắn: sticker" onLongPress={() => setMenuTin(tin)} style={styles.stickerHang}>
              <Sticker id={tin.body ?? ""} size={120} />
            </Pressable>
          ) : (
            <Pressable
              accessibilityLabel={daXoa ? "Tin nhắn đã bị xoá" : `Tin nhắn: ${tin.body ?? ""}`}
              disabled={daXoa}
              onLongPress={() => setMenuTin(tin)}
              style={[styles.bong, { backgroundColor: nenBong, borderColor: vienBong }]}
            >
              {daXoa ? (
                <Text style={[typography.caption, styles.nghieng, { color: colors.inkFaint }]}>Tin nhắn đã bị xoá</Text>
              ) : tin.kind === "image" ? (
                <View style={styles.anhKhoi}>
                  {anhTin(tin) === null ? (
                    <Text style={[typography.caption, { color: cuaToi ? mauChat.bubbleInk : colors.inkFaint }]}>
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
                  {tin.body ? <Text style={[typography.body, { color: mucBong }]}>{tin.body}</Text> : null}
                </View>
              ) : (
                <Text style={[typography.body, { color: mucBong }]}>{tin.body}</Text>
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
                  { borderColor: r.mine ? mauChat.accent : colors.line, backgroundColor: colors.card },
                ]}
              >
                <Text style={typography.caption}>
                  {glyphPhanUng(r.kind)} {r.count}
                </Text>
              </Pressable>
            ))}
          </View>
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
        <View style={styles.pills}>
          {nhanRieng ? (
            nguoiKiaId !== undefined ? (
              <Pressable
                accessibilityLabel="Xem hồ sơ"
                accessibilityRole="button"
                onPress={() => router.push(`/people/${nguoiKiaId}` as never)}
                style={({ pressed }) => [styles.thanhVien, pressed && styles.mo]}
              >
                <Ionicons color={colors.inkFaint} name="person-outline" size={15} />
                <Text numberOfLines={1} style={[typography.caption, { color: colors.inkSoft }]}>
                  Xem hồ sơ
                </Text>
                <Ionicons color={colors.inkFaint} name="chevron-forward" size={14} />
              </Pressable>
            ) : null
          ) : (
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
          )}
          <Pressable
            accessibilityLabel="Cài đặt nhóm"
            accessibilityRole="button"
            onPress={() => setCaiDatMo(true)}
            style={({ pressed }) => [styles.thanhVien, pressed && styles.mo]}
          >
            <Ionicons color={colors.inkFaint} name="settings-outline" size={15} />
            <Text style={[typography.caption, { color: colors.inkSoft }]}>Cài đặt</Text>
          </Pressable>
        </View>
      </View>
      {/* Drawn outside the inverted list: the list flips its own children
          back upright, and an extra flip here once mirrored this copy. */}
      {!chat.dangNap && chat.tin.length === 0 && dangGuiThan === null ? (
        <View style={[styles.rong, { paddingHorizontal: space.md }]}>
          <EmptyState
            body={nhanRieng ? `Nhắn gì đó cho ${tenNhom}, hoặc gõ / để rủ Rủ Đi AI vào.` : "Nhắn gì đó cho hội, hoặc gõ / để rủ Rủ Đi AI vào."}
            kind="first-use"
            layout="inline"
            title="Chưa có tin nhắn nào"
          />
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
                  <View style={[styles.bong, { backgroundColor: mauChat.bubble, borderColor: mauChat.bubble }]}>
                    <Text style={[typography.body, { color: mauChat.bubbleInk }]}>{dangGuiThan}</Text>
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
        // Hold the reader's place while they are up in the history and rows
        // arrive at the newest end. Off while they are at the end: in an
        // inverted list offset 0 *is* the newest end, so a new row, the
        // pending bubble and the notice header land in view with no scroll at
        // all. Keeping the anchor on there was the flow-30 red: the native
        // helper answers every content change with a smooth scroll of its
        // own, the JS side holds the render window until a scroll event comes
        // back, and the throttle dropped the event that would have said «at
        // the end again» -- the sent bubble ended half under the composer and
        // the notice below it, off screen.
        maintainVisibleContentPosition={oCuoi ? undefined : { minIndexForVisible: 0 }}
        // Content grows at the newest end (a row, the pending bubble, the
        // notice header): within a bubble of the end, stay at the end.
        onContentSizeChange={() => {
          if (ganCuoi.current) danhSachRef.current?.scrollToOffset({ offset: 0, animated: false });
        }}
        onEndReached={() => void chat.napCuHon()}
        onEndReachedThreshold={0.6}
        onMomentumScrollEnd={ghiViTri}
        onScroll={ghiViTri}
        onScrollEndDrag={ghiViTri}
        renderItem={renderItem}
        // Every event: Android drops (not delays) events inside the throttle
        // window, and a dropped last event leaves `ganCuoi` pointing at the
        // wrong end of the list.
        scrollEventThrottle={16}
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
      {traLoi ? (
        <View
          accessibilityLabel={`Đang trả lời ${tenNguoi(traLoi.author_id)}`}
          style={[styles.dangTraLoi, { backgroundColor: colors.card, borderColor: colors.line, borderLeftColor: mauChat.accent, marginHorizontal: space.md }]}
        >
          <View style={styles.dangTraLoiChu}>
            <Text style={[typography.caption, { color: colors.inkSoft }]}>Đang trả lời {tenNguoi(traLoi.author_id)}</Text>
            <Text numberOfLines={1} style={[typography.caption, { color: colors.ink }]}>
              {traLoi.preview}
            </Text>
          </View>
          <IconButton accessibilityLabel="Bỏ trả lời" icon="close" onPress={() => setTraLoi(null)} quiet />
        </View>
      ) : null}
      {khongNhanTin ? (
        <View style={[styles.dungNhan, { backgroundColor: colors.card, borderColor: colors.line, marginHorizontal: space.md }]}>
          <Text style={[typography.caption, { color: colors.inkSoft }]}>Cuộc trò chuyện này không còn nhận tin.</Text>
        </View>
      ) : (
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
            accessibilityLabel="Gửi sticker"
            disabled={dangGuiAnh || dangGui}
            icon="happy-outline"
            onPress={() => setKhaySticker(true)}
            quiet
          />
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
            placeholder={nhanRieng ? `Nhắn cho ${tenNhom}, hoặc gõ /` : "Nhắn cho hội, hoặc gõ /"}
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
      )}
      <KhaySticker onChon={(id) => void guiStickerChon(id)} onClose={() => setKhaySticker(false)} open={khaySticker} />
      <MenuTin
        cuaToi={menuTin !== null && menuTin.author_id === personId}
        onClose={() => setMenuTin(null)}
        onPhanUng={(tin, kind) => void phanUng(tin, kind)}
        onTraLoi={(tin) => {
          setTraLoi(trichTu(tin, tenNguoi));
          setMenuTin(null);
        }}
        onBaoCao={(tin) => {
          setMenuTin(null);
          setBaoCaoTin(tin);
        }}
        onXoa={(tin) => void xoaTinChon(tin)}
        tin={menuTin}
      />
      <Sheet
        accessibilityLabel="Báo cáo tin nhắn"
        onClose={() => setBaoCaoTin(null)}
        open={baoCaoTin !== null}
      >
        {baoCaoTin === null ? null : (
          <NoiDungBaoCao
            actorId={personId}
            loai="message"
            onThoi={() => setBaoCaoTin(null)}
            onXong={() => setBaoCaoTin(null)}
            targetId={baoCaoTin.id}
          />
        )}
      </Sheet>
      <CaiDatNhomSheet
        nhom={{ id: contextId, display_name: tenNhom, theme: nhom?.theme, kind: nhom?.kind }}
        onClose={() => setCaiDatMo(false)}
        onDaDoi={nhomDaDoi}
        open={caiDatMo}
        personId={personId}
      />
    </KeyboardAvoidingView>
  );
}

const styles = StyleSheet.create({
  dungNhan: { borderWidth: StyleSheet.hairlineWidth, borderRadius: 14, padding: 12, marginBottom: 10 },
  man: { flex: 1 },
  anhKhoi: { gap: 6 },
  anh: { width: 208, height: 208 },
  pills: { flexDirection: "row", justifyContent: "center", alignItems: "center", gap: 2, marginTop: -8 },
  thanhVien: { flexDirection: "row", alignItems: "center", gap: 5, minHeight: 40, paddingHorizontal: 8 },
  trich: { borderWidth: 1, borderLeftWidth: 3, borderRadius: 10, paddingHorizontal: 10, paddingVertical: 6, gap: 1, maxWidth: "100%" },
  stickerHang: { paddingVertical: 2 },
  nghieng: { fontStyle: "italic" },
  dangTraLoi: { flexDirection: "row", alignItems: "center", gap: 6, borderWidth: 1, borderLeftWidth: 3, borderRadius: 14, paddingLeft: 12, paddingRight: 2, paddingVertical: 4, marginBottom: 6 },
  dangTraLoiChu: { flex: 1, gap: 1 },
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
