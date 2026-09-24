/**
 * The group's shared invitation notebook, backed by the legacy Go candidate.
 * Message history and durable invalidations use independent receive cursors.
 * Polls and proposals become a contextual folded sheet; explicit AI requests
 * use a durable job with invocation-only consent. This surface still labels
 * its legacy transport honestly and never substitutes it for E2EE v2.
 */
import { Ionicons } from "@expo/vector-icons";
import { Image } from "expo-image";
import { Redirect, useFocusEffect, useRouter } from "expo-router";
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
  ScrollView,
  type ViewToken,
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
  tinChoHoiThoai,
  type HangHienThi,
  type LoaiPhanUng,
  type Tin,
  type TrichDan,
} from "../../chat/tin-song";
import { TAT_KAV_QA } from "../../chat/qa-ban-phim";
import { boAnh, chonAnh, nenVaDung } from "../../ky-niem/chon-anh";
import { nguonAnh } from "../../ky-niem/ky-niem";
import { CHAT_VIEWABILITY } from "../../chat/viewability";
import { useBanNhap } from "../../chat/useBanNhap";
import { useTinNhan } from "../../chat/useTinNhan";
import { useChatChanges } from "../../chat/useChatChanges";
import { useChatAi } from "../../chat/useChatAi";
import { chuHangLoiGoi, docLenhAi, lenhSanSang, thuLaiDuoc, type LenhAi } from "../../chat/ai-invocations";
import { laPair, tenCuocTroChuyen } from "../../nhan-rieng/nhan-rieng";
import { useRudiSession } from "../../session";
import { HangToGiaySong } from "../hai-nguoi/HangToGiaySong";
import { bangMauChat, typography, useRudiTheme } from "../../theme";
import { IconButton, RudiButton } from "../../ui";
import { useMotion } from "../../ui/useMotion";
import { Avatar } from "../../ui/Avatar";
import { Sticker } from "../../ui/stickers/Sticker";
import type { TinChoGui } from "../../chat/hang-cho";
import { Sheet } from "../../ui/Sheet";
import { CaiDatNhomSheet } from "./CaiDatNhom";
import { KhaySticker } from "./KhaySticker";
import { NoiDungBaoCao } from "../nguoi/NoiDungBaoCao";
import { MenuTin } from "./MenuTin";
import { TheAiView } from "./TheAi";
import { CongCuChat, ToHen, type KhayChat } from "./SoHen";
import { gomBoiCanhChat } from "../../chat/boi-canh-chat";
import { KhayToHenChung } from "./ToHenChungKhay";
import { useToHenChung } from "../../chat/useToHenChung";
import { docKhoiNhap } from "../../chat/to-hen-chung";
import { Nep } from "../../ui/art/Nep";
import { useNepNguCanh } from "../../nep/NepProvider";

const LENH = [
  { nhan: "/plan", goiY: "/plan tối nay đi đâu?", moTa: "Rủ Đi AI phác lịch trình" },
  { nhan: "/vote", goiY: "/vote", moTa: "Viết câu hỏi và lựa chọn" },
  { nhan: "/chia-bill", goiY: "/chia-bill", moTa: "Rủ Đi AI gom khoản chi để cả hội xác nhận" },
  { nhan: "@Rủ Đi", goiY: "@Rủ Đi ", moTa: "Nhờ phác một tờ hẹn" },
] as const;

/** A body that calls on the model (not `/vote`, which the server answers itself). */
function goiMoHinh(body: string): boolean {
  return /^\/(plan|chia-?bill)\b/i.test(body) || /@(rủ đi|ru di|rudi)/i.test(body);
}

/**
 * One send that has not landed yet, drawn where the message will be.
 *
 * The picture is the state: dimmed while it travels, solid again when it
 * failed, with the server's own sentence under it and the house's «Thử lại»
 * beside it. A refusal that pressing again cannot fix (an id this build does
 * not know, a quoted message that is gone) says so and offers only to drop the
 * row -- a retry button that will fail the same way is a worse answer than
 * none (review delta 08/09, F32).
 */
export function HangChoGui({ tin, onThuLai, onBoQua }: { tin: TinChoGui; onThuLai: () => void; onBoQua: () => void }) {
  const { colors, space } = useRudiTheme();
  const hong = tin.trangThai === "that-bai";
  return (
    <View style={styles.choGui}>
      <View style={[styles.hang, styles.hangToi]}>
        <View style={[styles.khoi, styles.khoiToi, hong ? undefined : styles.mo]}>
          {tin.traLoi ? <Text numberOfLines={2} style={[typography.caption, { color: colors.inkSoft }]}>Trả lời: {tin.traLoi.preview}</Text> : null}
          {tin.kind === "sticker" ? (
            <View style={styles.stickerHang}>
              <Sticker id={tin.than} size={120} />
            </View>
          ) : (
            <View style={[styles.bong, { backgroundColor: colors.card, borderColor: colors.line }]}>
              <Text style={[tin.kind === "text" ? typography.body : typography.caption, { color: colors.ink }]}>{tin.kind === "text" ? tin.than : tin.phuDe ?? "Ảnh"}</Text>
            </View>
          )}
          {hong ? (
            <>
              <Text style={[typography.caption, { color: colors.warn }]}>{tin.loi ?? "Chưa gửi được."}</Text>
              <View style={[styles.hang, styles.nutHong, { gap: space.sm }]}>
                {tin.thuLaiDuoc ? <RudiButton compact full={false} label="Thử lại" onPress={onThuLai} variant="outline" /> : null}
                <RudiButton compact full={false} label="Bỏ" onPress={onBoQua} variant="ghost" />
              </View>
            </>
          ) : (
            <Text style={[typography.caption, { color: colors.inkFaint }]}>Đang gửi…</Text>
          )}
        </View>
      </View>
    </View>
  );
}

export function GroupChatLiveScreen({ contextId }: { contextId: string }) {
  const router = useRouter();
  const { colors, dark, radius, space } = useRudiTheme();
  const insets = useSafeAreaInsets();
  const { reduced } = useMotion();
  const { phien, datPhien } = useRudiSession();
  const personId = phien?.person_id ?? "";
  const chat = useTinNhan(contextId, personId);
  const changes = useChatChanges(contextId, personId, chat.nhanAnhChup);
  const ai = useChatAi(contextId, personId);
  const { text: nhap, change: doiNhap, snapshot: nhapRef, clearIfUnchanged: xoaNhapCu } = useBanNhap();
  const [dangGui, setDangGui] = useState(false);
  // A model command gets an additional waiting row; the queue owns its text.
  const [dangGuiThan, setDangGuiThan] = useState<string | null>(null);
  const [banPhimMo, setBanPhimMo] = useState(false);
  // What the server said about the last command, drawn as a row in the thread
  // (where the pending card promised it), signed by who is speaking.
  const [thongBao, setThongBao] = useState<{ tu: string; cau: string; luc: string } | null>(null);
  // Social v1.1 (ADR-0021): the sticker tray, the long-press menu of one
  // message, the message being replied to, and the group settings sheet.
  const [khaySticker, setKhaySticker] = useState(false);
  const [khay, setKhay] = useState<KhayChat>(null);
  const toHenChung = useToHenChung(contextId, personId);
  const [xacNhanBoToHen, setXacNhanBoToHen] = useState(false);
  // Which decided polls already feed an open sheet, so the card can say "mở"
  // instead of offering to create a second one the server would refuse.
  const binhChonDaCoToHen = useMemo(() => {
    const byVote = new Map<string, { ten: string; ban: number }>();
    for (const row of chat.tin) {
      const nhap = docKhoiNhap(row.card);
      if (nhap?.status !== "open" || !nhap.source_vote_id) continue;
      const the = docTheAi(row.card);
      byVote.set(nhap.source_vote_id, {
        ten: the.loai === "itinerary" ? the.the.tieuDe : "",
        ban: nhap.revision,
      });
    }
    return byVote;
  }, [chat.tin]);
  const coToHenChoBinhChon = (voteId: string) => binhChonDaCoToHen.get(voteId) ?? null;
  const [promptAi, setPromptAi] = useState("");
  // Which command the AI tray sends. Only `/chia-bill` sets it; every other way
  // into (or out of) the tray is a plan, so leaving the tray resets it.
  const [lenhAi, setLenhAi] = useState<LenhAi>("plan");
  useEffect(() => { if (khay !== "plan") setLenhAi("plan"); }, [khay]);
  const [menuTin, setMenuTin] = useState<Tin | null>(null);
  // ADR-0023 §2.4: báo cáo một tin nhắn. Khay riêng, mở sau khi khay menu
  // đóng, để hai khay không chồng lên nhau trên màn nhỏ.
  const [baoCaoTin, setBaoCaoTin] = useState<Tin | null>(null);
  const [traLoi, setTraLoi] = useState<TrichDan | null>(null);
  const [caiDatMo, setCaiDatMo] = useState(false);
  const [dangGuiAnh, setDangGuiAnh] = useState(false);
  const songRef = useRef(true);
  const guiRef = useRef(false);
  const guiAnhRef = useRef(false);
  useEffect(() => {
    songRef.current = true;
    return () => { songRef.current = false; };
  }, []);
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
  // What Nếp may know here: the kind of conversation and, for a group, how many
  // are in it. Never a name and never a message -- chat v2 is end to end
  // encrypted, and Nếp does not read chat on its own (ADR-0033 §2.5). Before
  // this, Nếp opened in a couple's conversation said it had no idea where the
  // person was (QA 23/09).
  useNepNguCanh({
    man: "groups/[id]/chat",
    tieuDe: nhanRieng ? "cuộc trò chuyện của hai bạn" : "chat nhóm",
    loaiSo: !nhanRieng ? "hoi" : undefined,
    soLieu: !nhanRieng ? { soNguoi: nhom?.member_count ?? 0 } : undefined,
    goiY: nhanRieng ? ["Tuần này rủ nhau đi đâu?", "Mở tờ giấy của hai mình"] : ["Gợi ý chỗ cho cả nhóm", "Tóm tắt kèo sắp tới"],
  });
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

  // The roster is read on every focus and again when somebody the roster does
  // not know writes: they joined after it was read. Read once on mount, a
  // group went on saying «2 thành viên» and naming the newcomer «Thành viên»
  // until the screen was reopened (QA 23/09).
  const [lanDoc, setLanDoc] = useState(0);
  const [soDangO, setSoDangO] = useState<number | null>(null);
  useFocusEffect(
    useCallback(() => {
      setLanDoc((n) => n + 1);
    }, []),
  );
  const coNguoiLa = chat.tin.some((t) => t.author_id !== null && t.author_id !== personId && !(t.author_id in tenTheoId));
  useEffect(() => {
    if (coNguoiLa) setLanDoc((n) => n + 1);
  }, [coNguoiLa]);
  useEffect(() => {
    if (lanDoc === 0) return;
    let song = true;
    void danhSachThanhVien(contextId, personId)
      .then((ds) => {
        if (!song) return;
        // Names of everyone who was ever here (an old message keeps its
        // author's name after they leave); the count is who is here now.
        const map: Record<string, string> = {};
        for (const tv of ds) if (tv.display_name) map[tv.person_id] = tv.display_name;
        setTenTheoId(map);
        setSoDangO(ds.filter((tv) => tv.state === "active").length);
      })
      .catch(() => undefined);
    return () => {
      song = false;
    };
  }, [contextId, personId, lanDoc]);

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

  const tinHien = useMemo(() => tinChoHoiThoai(chat.tin), [chat.tin]);
  const hang = useMemo(() => nhomTheoNgay(tinHien), [tinHien]);
  // Built from `tinHien`, the list the screen is drawing, not from `chat.tin`.
  // `tinChoHoiThoai` hides a `/vote` command once its poll card exists, so that
  // command is not on screen -- and "this is what you are looking at" has to be
  // true in the literal sense. One place decides what is visible.
  // Members' display names go with the bundle (ADR-0036 §5), the same names
  // `tenNguoi` draws above each bubble. The raw map rather than `tenNguoi`
  // itself: its «Thành viên» placeholder would make every unknown member one
  // speaker, and an unknown member has to fall back to a distinct «Bạn N».
  const boiCanhAi = useMemo(
    () => gomBoiCanhChat({ tin: tinHien, personId, tenCua: (id) => tenTheoId[id] }),
    [tinHien, personId, tenTheoId],
  );
  const toHen = useMemo(() => {
    if (nhanRieng) return null;
    const dangMo = (tin: Tin) => {
      const card = docTheAi(tin.card);
      if (card.loai === "itinerary") return !!card.nhapChung && card.nhapChung.status === "open" && !card.outingId;
      return card.loai === "poll" && !changes.votes[card.vote_id]?.is_closed && !changes.votes[card.vote_id]?.deleted;
    };
    const bat = (tin: Tin) => {
      const card = docTheAi(tin.card);
      return card.loai === "itinerary" || (card.loai === "poll" && !changes.votes[card.vote_id]?.is_closed && !changes.votes[card.vote_id]?.deleted);
    };
    // The slot answers "hội đang chốt cái gì?", so a sheet the group can still
    // edit outranks a finished one even when the finished one is newer. Without
    // this the strip says "Đã thành kèo" over an open sheet in the same screen.
    return chat.tin.find(dangMo) ?? chat.tin.find(bat) ?? null;
  }, [chat.tin, changes.votes, nhanRieng]);
  const coChu = nhap.trim().length > 0;
  const moLenh = (nhap.startsWith("/") && !nhap.includes(" ")) || nhap === "@";
  const lenhPhuHop = LENH.filter((lenh) => lenh.nhan.toLocaleLowerCase().startsWith(nhap.toLocaleLowerCase()));
  const viewabilityConfig = useRef(CHAT_VIEWABILITY).current;
  const baoTinHienThi = useCallback(({ viewableItems }: { viewableItems: ViewToken<HangHienThi>[] }) => {
    chat.danhDauHienThi(viewableItems.flatMap(({ item, isViewable }) => isViewable && item.loai === "tin" ? [item.tin.id] : []));
  }, [chat.danhDauHienThi]);

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
    if (chat.tin.length > soHang.current && ganCuoi.current) danhSachRef.current?.scrollToOffset({ offset: 0, animated: !reduced });
    soHang.current = chat.tin.length;
  }, [chat.tin.length, reduced]);
  // The notice under the newest bubble (why the model stayed quiet, a send
  // error) is a list header, not a row: `maintainVisibleContentPosition` keeps
  // row 0 in place and leaves the header under the composer, so it is pulled
  // into view the same way a new row is.
  useEffect(() => {
    if (thongBao !== null && ganCuoi.current) danhSachRef.current?.scrollToOffset({ offset: 0, animated: !reduced });
  }, [thongBao, reduced]);
  useEffect(() => {
    const sub = Keyboard.addListener("keyboardDidShow", () => {
      if (ganCuoi.current) danhSachRef.current?.scrollToOffset({ offset: 0, animated: false });
    });
    return () => sub.remove();
  }, []);

  const gui = async (command?: string): Promise<boolean> => {
    const body = (command ?? nhap).trim();
    if (!body || guiRef.current) return false;
    if (goiMoHinh(body)) {
      // `/chia-bill` goes through the same tray as `/plan`: the same «Mình
      // đang thấy» preview, the same «Chỉ gửi lời nhờ», the same queue.
      const { lenh, prompt } = docLenhAi(body);
      setLenhAi(lenh); setPromptAi(prompt); setKhay("plan"); doiNhap("");
      return false;
    }
    if (body === "/vote") { setKhay("poll"); doiNhap(""); return false; }
    guiRef.current = true;
    setDangGui(true);
    setDangGuiThan(body);
    if (command === undefined) doiNhap("");
    const traLoiCu = traLoi;
    setTraLoi(null);
    veCuoi();
    try {
      const daGui = await chat.gui(body, traLoiCu);
      // Landed for a conversation that has left the screen: nothing to show here.
      if (daGui === null) return false;
      veCuoi();
      const cau = cauYDinh(daGui);
      if (cau !== null) setThongBao({ tu: "Rủ Đi", cau, luc: new Date().toISOString() });
      return !daGui.intent_error;
    } catch {
      // The failed row owns the exact text, quote and retry key. Leave any
      // newer words in the composer untouched.
      return false;
    } finally {
      guiRef.current = false;
      setDangGui(false);
      setDangGuiThan(null);
    }
  };

  const moToHen = (tin?: Tin) => {
    const card = docTheAi(tin?.card);
    if (card.loai === "itinerary" && card.outingId) { router.push(`/outings/${card.outingId}` as never); return; }
    // A sheet the group still owns opens where it can be edited together; an AI
    // suggestion still goes to the form where one person accepts it.
    const nhap = docKhoiNhap(tin?.card);
    if (nhap && nhap.status === "open") { setKhay(null); void toHenChung.open(nhap.id); return; }
    router.push({ pathname: "/outings/new", params: { contextId, ...(tin ? { sourceMessageId: tin.id } : {}) } } as never);
  };

  /** A decided poll becomes a sheet the whole group can write on. */
  const moToHenTuBinhChon = async (voteId: string, goiY: string) => {
    setKhay(null);
    const existing = chat.tin.find((row) => {
      const nhap = docKhoiNhap(row.card);
      return nhap?.source_vote_id === voteId && nhap.status === "open";
    });
    const nhap = existing ? docKhoiNhap(existing.card) : null;
    if (nhap) { void toHenChung.open(nhap.id); return; }
    await toHenChung.create({
      title: goiY,
      stops: [{ time_text: "19:00", label: goiY }],
      from_vote_id: voteId,
    });
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
    if (guiAnhRef.current || guiRef.current) return;
    guiAnhRef.current = true;
    let daChon = null;
    try {
      daChon = await chonAnh();
    } catch (error) {
      guiAnhRef.current = false;
      if (!songRef.current) return;
      setThongBao({
        tu: "Rủ Đi",
        cau: error instanceof ApiError ? error.message : "Không mở được thư viện ảnh trên máy này.",
        luc: new Date().toISOString(),
      });
      return;
    }
    if (daChon === null || !songRef.current) {
      guiAnhRef.current = false;
      if (daChon !== null) await boAnh(daChon);
      return;
    }
    const draft = nhapRef.current;
    const caption = draft.text.trim();
    setDangGuiAnh(true);
    // Two stages with one press. The upload has no row of its own, so its
    // failure is a notice; the message does, so its failure belongs there and
    // saying it here as well would be the same news twice, further from the
    // picture it is about (F32).
    let daToiTin = false;
    let daGui: Awaited<ReturnType<typeof chat.guiAnhMoi>> = null;
    try {
      await nenVaDung(daChon, async (anh) => {
        if (!songRef.current) return;
        const daTai = await taiAnhNhom(contextId, anh, personId);
        if (!songRef.current) return;
        daToiTin = true;
        daGui = await chat.guiAnhMoi(daTai.url, caption === "" ? null : caption);
      });
      if (daGui === null) return;
      xoaNhapCu(draft.revision);
      veCuoi();
    } catch (error) {
      await boAnh(daChon);
      if (!daToiTin) {
        setThongBao({
          tu: "Rủ Đi",
          cau: error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null),
          luc: new Date().toISOString(),
        });
      }
    } finally {
      guiAnhRef.current = false;
      setDangGuiAnh(false);
    }
  };

  /**
   * One sticker from the tray, optionally as a reply to the quoted message.
   *
   * Not guarded by `dangGui`: that is the flag of the words being typed, and a
   * sticker is a different send. What the person sees while it travels is the
   * picture itself, dimmed, at the newest end of the thread -- and if it fails
   * it stays there with the reason and a way to send it again, instead of
   * vanishing behind a notice that does not say which picture was lost (F32).
   */
  const guiStickerChon = async (id: string) => {
    setKhaySticker(false);
    const tra = traLoi;
    // The queued row carries the quoted message from here on, so the composer
    // strip clears at once and a retry still answers the right message.
    setTraLoi(null);
    try {
      if ((await chat.guiSticker(id, tra)) !== null) veCuoi();
    } catch {
      // The row says what happened, in place. A general notice would say it a
      // second time and further from the picture it is about.
    }
  };

  /** Send a failed row again, with the key it was minted with. */
  const thuLaiGui = async (khoa: string) => {
    try {
      if ((await chat.thuLaiMot(khoa)) !== null) veCuoi();
    } catch {
      // Same as above: the row itself carries the second refusal.
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
    a.tin.author_id !== null && a.tin.author_id === b.tin.author_id &&
    Math.abs(Date.parse(a.tin.created_at) - Date.parse(b.tin.created_at)) <= 5 * 60 * 1000;

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
        testID={`chat-message-${tin.id}`}
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
              vote={(() => { const card = docTheAi(tin.card); return card.loai === "poll" ? changes.votes[card.vote_id] : undefined; })()}
              onOpenPlan={() => moToHen(tin)}
              onMoToHen={(voteId, goiY) => void moToHenTuBinhChon(voteId, goiY)}
              banToHen={(() => { const card = docTheAi(tin.card); return card.loai === "poll" ? (coToHenChoBinhChon(card.vote_id)?.ban ?? null) : null; })()}
              daCoToHen={(() => { const card = docTheAi(tin.card); return card.loai === "poll" && coToHenChoBinhChon(card.vote_id) !== null; })()}
              tenToHen={(() => { const card = docTheAi(tin.card); return card.loai === "poll" ? (coToHenChoBinhChon(card.vote_id)?.ten || null) : null; })()}
            />
          ) : laSticker ? (
            <Pressable
              accessibilityRole="button"
              accessibilityLabel="Tin nhắn: sticker"
              accessibilityActions={[{ name: "activate", label: "Tuỳ chọn tin nhắn" }]}
              onAccessibilityAction={() => setMenuTin(tin)}
              onLongPress={() => setMenuTin(tin)}
              onPress={() => setMenuTin(tin)}
              style={styles.stickerHang}
            >
              <Sticker id={tin.body ?? ""} size={120} />
            </Pressable>
          ) : (
            <Pressable
              accessibilityRole="button"
              accessibilityLabel={daXoa ? "Tin nhắn đã bị xoá" : `Tin nhắn: ${tin.body ?? ""}`}
              disabled={daXoa}
              accessibilityActions={daXoa ? [] : [{ name: "activate", label: "Tuỳ chọn tin nhắn" }]}
              onAccessibilityAction={() => { if (!daXoa) setMenuTin(tin); }}
              onLongPress={() => setMenuTin(tin)}
              onPress={() => { if (!daXoa) setMenuTin(tin); }}
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
            {cuoiChuoi || chips.length > 0 ? <Text style={[typography.caption, { color: colors.inkFaint }]}>{gioPhut(tin.created_at)}</Text> : null}
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
      <View style={[styles.dau, { paddingHorizontal: space.md, borderBottomColor: colors.line }]}>
        <View style={styles.chatHeader}>
          <IconButton accessibilityLabel="Quay lại" icon="chevron-back" quiet onPress={() => router.back()} />
          <Pressable accessibilityRole="button" accessibilityLabel={nhanRieng ? "Xem hồ sơ" : "Thành viên nhóm"}
            onPress={() => router.push((nhanRieng && nguoiKiaId ? `/people/${nguoiKiaId}` : `/groups/${contextId}/members`) as never)} style={styles.headerIdentity}>
            <Text numberOfLines={1} style={[typography.title, { color: colors.ink }]}>{tenNhom}</Text>
            <Text style={[typography.caption, { color: colors.inkSoft }]}>
              {nhanRieng ? "Cuộc trò chuyện của hai mình" : `${soDangO || nhom?.member_count || 1} thành viên · sổ hẹn của hội`}
            </Text>
          </Pressable>
          <IconButton accessibilityLabel="Cài đặt nhóm" icon="ellipsis-horizontal" quiet onPress={() => setCaiDatMo(true)} />
        </View>
        {nhanRieng && phien !== null ? <HangToGiaySong contextId={contextId} tenNguoiKia={tenNhom} toiId={phien.person_id} /> : null}
        <View style={styles.baoMat}>
          <Ionicons name="lock-open-outline" size={13} color={colors.inkSoft} />
          <Text style={[typography.caption, { color: colors.inkSoft }]}>Chưa mã hoá đầu cuối</Text>
          {changes.connection === "recovering" ? <Text accessibilityLiveRegion="polite" style={[typography.caption, { color: colors.inkSoft }]}>· Đang nối lại</Text> : null}
        </View>
      </View>
      {toHen ? <ToHen tin={toHen} onOpen={moToHen} onVote={(tin) => {
        const index = hang.findIndex((row) => row.loai === "tin" && row.tin.id === tin.id);
        if (index >= 0) danhSachRef.current?.scrollToIndex({ index, animated: !reduced, viewPosition: 0.5 });
      }} /> : null}
      {/* Drawn outside the inverted list: the list flips its own children
          back upright, and an extra flip here once mirrored this copy. */}
      {!chat.dangNap && chat.tin.length === 0 && dangGuiThan === null && chat.hangCho.length === 0 ? (
        <View style={[styles.rong, { paddingHorizontal: space.md }]}>
          <View style={styles.moLoi}>
            <Nep pose="moi" size={96} />
            <Text style={[typography.h2, styles.giua, { color: colors.ink }]}>{nhanRieng ? "Một lời mở đầu." : "Có hội rồi. Mở lời thôi."}</Text>
            <Text style={[typography.body, styles.giua, { color: colors.inkSoft }]}>{nhanRieng ? `Một tin nhắn nhỏ cho ${tenNhom}.` : "Từ một câu rủ, thành một buổi cùng đi."}</Text>
            {!nhanRieng ? <RudiButton label="Rủ hội một buổi" variant="outline" full={false} onPress={() => { setLenhAi("plan"); setKhay("plan"); }} /> : null}
          </View>
        </View>
      ) : null}
      <FlatList
        ref={danhSachRef}
        contentContainerStyle={[styles.danhSach, { paddingHorizontal: space.md }]}
        data={hang}
        inverted
        keyExtractor={khoaHang}
        onViewableItemsChanged={baoTinHienThi}
        viewabilityConfig={viewabilityConfig}
        keyboardDismissMode="on-drag"
        keyboardShouldPersistTaps="handled"
        // Inverted, so the header sits at the newest end: what is being sent
        // shows there at once, and a command shows the model is being asked.
        ListHeaderComponent={
          <>
            {ai.error && khay !== "plan" ? <Text accessibilityLiveRegion="polite" style={[typography.caption, { color: colors.warn, paddingVertical: 10 }]}>{ai.error}</Text> : null}
            {ai.requests.filter((request) => request.status !== "succeeded" && request.status !== "cancelled").map((request) => (
              <View key={request.id} style={[styles.invocation, { backgroundColor: colors.card, borderColor: colors.line }]}>
                <View style={styles.dauAi}>
                  <Ionicons name={request.status === "failed" ? "alert-circle-outline" : "time-outline"} size={20} color={colors.inkSoft} />
                  <Text style={[typography.label, { color: colors.ink }]}>{chuHangLoiGoi(request).tieuDe}</Text>
                </View>
                <Text style={[typography.caption, { color: colors.inkSoft }]}>{chuHangLoiGoi(request).cau}</Text>
                {request.status === "failed" ? <View style={styles.requestActions}>
                  {thuLaiDuoc(request) ? <RudiButton label="Thử lại lời nhờ" compact full={false} variant="outline" loading={ai.busy} disabled={ai.busy || !lenhSanSang(ai.capabilities, request.command ?? "plan")} onPress={() => void ai.retry(request.id)} /> : null}
                  {request.command !== "chia_bill" ? <RudiButton label="Tự tạo kèo" compact full={false} variant="ghost" onPress={() => moToHen()} /> : null}
                </View> : null}
              </View>
            ))}
            {/* Every logical send owns its pending and failed row. */}
            {chat.hangCho.map((t) => (
              <HangChoGui key={t.attempt.key} onBoQua={() => chat.boQua(t.attempt.key)} onThuLai={() => void thuLaiGui(t.attempt.key)} tin={t} />
            ))}
            {dangGuiThan === null && thongBao !== null ? (
            <View style={styles.hang}>
              <View style={[styles.khoi, styles.khoiAi]}>
                <View style={[styles.choAi, { backgroundColor: colors.card, borderColor: colors.line, borderRadius: radius.base }]}>
                  <View style={styles.dauAi}>
                    <Ionicons color={colors.ai} name="sparkles" size={15} />
                    <Text style={[typography.caption, { color: colors.ai }]}>{thongBao.tu}</Text>
                  </View>
                  <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.ink }]}>{thongBao.cau}</Text>
                  <RudiButton compact label="Đã hiểu" variant="ghost" onPress={() => setThongBao(null)} />
                </View>
                <Text style={[typography.caption, { color: colors.inkFaint }]}>{gioPhut(thongBao.luc)}</Text>
              </View>
            </View>
          ) : dangGuiThan !== null ? (
            <View style={styles.choGui}>
              {goiMoHinh(dangGuiThan) ? (
                <View style={styles.hang}>
                  <View style={[styles.khoi, styles.khoiAi]}>
                    <View style={[styles.choAi, { backgroundColor: colors.card, borderColor: colors.line, borderRadius: radius.base }]}>
                      <View style={styles.dauAi}>
                        <Ionicons color={colors.ai} name="sparkles" size={15} />
                        <Text style={[typography.caption, { color: colors.ai }]}>Đang hỏi Rủ Đi AI...</Text>
                      </View>
                      <Text style={[typography.caption, { color: colors.inkSoft }]}>
                        Bạn có thể tiếp tục soạn tin trong lúc chờ.
                      </Text>
                    </View>
                  </View>
                </View>
              ) : null}
            </View>
          ) : null}
          </>
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
        onScrollToIndexFailed={({ index, averageItemLength }) => {
          danhSachRef.current?.scrollToOffset({ offset: index * averageItemLength, animated: false });
        }}
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
      {!oCuoi ? (
        <Pressable
          accessibilityRole="button"
          accessibilityLabel="Về tin nhắn mới nhất"
          onPress={veCuoi}
          style={[styles.veCuoi, { backgroundColor: colors.card, borderColor: colors.line }]}
        >
          <Ionicons name="arrow-down" size={18} color={colors.accent} />
          <Text style={[typography.label, { color: colors.ink }]}>Tin mới nhất</Text>
        </Pressable>
      ) : null}
      {chat.loi ? (
        <Text style={[typography.caption, { color: colors.warn, paddingHorizontal: space.md }]}>{chat.loi}</Text>
      ) : null}
      {moLenh && lenhPhuHop.length > 0 ? (
        <ScrollView keyboardShouldPersistTaps="handled" style={[styles.lenh, { backgroundColor: colors.card, borderColor: colors.line, marginHorizontal: space.md }]}>
          {lenhPhuHop.map((l) => (
            <Pressable accessibilityRole="button" key={l.nhan} onPress={() => { setKhay(l.nhan === "/vote" ? "poll" : "plan"); setLenhAi(l.nhan === "/chia-bill" ? "chia_bill" : "plan"); doiNhap(""); }} style={styles.lenhHang}>
              <Text style={[typography.label, { color: colors.accent }]}>{l.nhan}</Text>
              <Text style={[typography.caption, { color: colors.inkSoft }]}>{l.moTa}</Text>
            </Pressable>
          ))}
        </ScrollView>
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
      {/* The shared sheet takes the tray slot while it is open: two forms over
          one thread is exactly the split-screen the reviewer flagged. */}
      {toHenChung.sheet ? (
        <KhayToHenChung
          busy={toHenChung.busy}
          error={toHenChung.error}
          onClose={() => { setXacNhanBoToHen(false); toHenChung.close(); }}
          onConfirm={() => {
            const source = toHenChung.sheet?.message_id;
            const anchor = chat.tin.find((row) => row.id === source);
            const daCoKeo = anchor ? docTheAi(anchor.card) : null;
            // Already a kèo: open it.
            if (daCoKeo?.loai === "itinerary" && daCoKeo.outingId) {
              toHenChung.close();
              router.push(`/outings/${daCoKeo.outingId}` as never);
              return;
            }
            // Still a draft: chốt right here. Sending the person to a second
            // route meant the moment that matters -- this sheet becoming the
            // kèo -- happened off-screen from where they pressed the button.
            void (async () => {
              const keo = await toHenChung.confirm();
              if (!keo) return;
              toHenChung.close();
              await chat.taiLai();
            })();
          }}
          onBoThat={() => { setXacNhanBoToHen(false); void toHenChung.discard(); }}
          onDiscard={() => setXacNhanBoToHen(true)}
          onHuyBo={() => setXacNhanBoToHen(false)}
          onSave={(patch) => void toHenChung.edit(patch)}
          sheet={toHenChung.sheet}
          stale={toHenChung.stale}
          xacNhanBo={xacNhanBoToHen}
        />
      ) : null}
      {!khongNhanTin && !toHenChung.sheet ? <CongCuChat personId={personId} contextId={contextId} panel={khay} onPanel={setKhay} capabilities={ai.capabilities} busy={dangGui || ai.busy} boiCanh={boiCanhAi}
        initialPrompt={promptAi}
        lenh={lenhAi}
        error={ai.error}
        onImage={() => { setKhay(null); void guiAnh(); }}
        onSticker={() => { setKhay(null); setKhaySticker(true); }}
        onPoll={gui}
        onPlan={async (prompt, boiCanh) => { const draft = nhapRef.current; const sent = await ai.send(prompt, boiCanh, lenhAi); if (sent) { setPromptAi(""); if (goiMoHinh(draft.text)) xoaNhapCu(draft.revision); } return sent; }}
        onManual={() => { setKhay(null); moToHen(); }}
        haiNguoi={nhanRieng}
        onToGiay={nhanRieng ? () => router.push(`/groups/${contextId}/to-giay` as never) : undefined} /> : null}
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
            accessibilityLabel={khay ? "Đóng công cụ chat" : "Thêm vào cuộc trò chuyện"}
            disabled={dangGuiAnh || dangGui}
            icon={khay ? "close" : "add"}
            loading={dangGuiAnh}
            onPress={() => setKhay(khay ? null : "tools")}
            quiet
          />
          <TextInput
            accessibilityLabel="Ô soạn tin"
            cursorColor={colors.accent}
            multiline
            onChangeText={doiNhap}
            placeholder={nhanRieng ? `Nhắn cho ${tenNhom}` : "Nhắn cho hội…"}
            placeholderTextColor={colors.inkSoft}
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
  chatHeader: { flexDirection: "row", alignItems: "center", gap: 4, minHeight: 64 },
  headerIdentity: { flex: 1, minHeight: 48, justifyContent: "center", gap: 3, paddingHorizontal: 4 },
  moLoi: { alignItems: "center", gap: 6 },
  invocation: { borderWidth: 1, borderRadius: 12, padding: 14, gap: 8, marginBottom: 10 },
  requestActions: { flexDirection: "row", flexWrap: "wrap", gap: 6 },
  dungNhan: { borderWidth: StyleSheet.hairlineWidth, borderRadius: 14, padding: 12, marginBottom: 10 },
  man: { flex: 1, width: "100%", maxWidth: 820, alignSelf: "center" },
  anhKhoi: { gap: 6 },
  anh: { width: 208, height: 208 },
  pills: { flexDirection: "row", flexWrap: "wrap", justifyContent: "center", alignItems: "center", gap: 2, marginTop: -8 },
  thanhVien: { flexDirection: "row", alignItems: "center", gap: 5, minHeight: 48, paddingHorizontal: 8 },
  trich: { borderWidth: 1, borderRadius: 10, paddingHorizontal: 10, paddingVertical: 6, gap: 1, maxWidth: "100%" },
  stickerHang: { paddingVertical: 2 },
  nghieng: { fontStyle: "italic" },
  dangTraLoi: { flexDirection: "row", alignItems: "center", gap: 6, borderWidth: 1, borderRadius: 14, paddingLeft: 12, paddingRight: 2, paddingVertical: 4, marginBottom: 6 },
  dangTraLoiChu: { flex: 1, gap: 1 },
  danhSach: { paddingTop: 12, paddingBottom: 22, gap: 8 },
  ngay: { flexDirection: "row", alignItems: "center", gap: 10, paddingVertical: 6 },
  duong: { flex: 1, height: StyleSheet.hairlineWidth },
  hang: { flexDirection: "row", alignItems: "flex-end", gap: 8 },
  hangToi: { justifyContent: "flex-end" },
  hangCao: { alignItems: "flex-start" },
  khoi: { maxWidth: "82%", gap: 4 },
  khoiToi: { alignItems: "flex-end" },
  khoiAi: { maxWidth: "100%", flex: 1 },
  choChuDau: { width: 30, height: 30 },
  bong: { borderWidth: 1, borderRadius: 18, paddingHorizontal: 14, paddingVertical: 10 },
  duoiBong: { flexDirection: "row", alignItems: "center", gap: 6, flexWrap: "wrap" },
  chip: { minHeight: 48, justifyContent: "center", borderWidth: 1, borderRadius: 999, paddingHorizontal: 10, paddingVertical: 2 },
  baoMat: { flexDirection: "row", alignItems: "center", justifyContent: "center", gap: 5, flexWrap: "wrap" },
  dau: { paddingBottom: 8, borderBottomWidth: StyleSheet.hairlineWidth },
  rong: { paddingVertical: 24 },
  choGui: { gap: 12 },
  // Two content-sized buttons that may wrap. `RudiButton` is full-width by
  // default, and two full-width buttons in one row pushed «Thử lại» off the
  // left edge of the screen (audit native 09/09, F41). At large text the pair
  // stacks, right-aligned, instead of shrinking or hiding its label.
  nutHong: { flexWrap: "wrap", justifyContent: "flex-end" },
  mo: { opacity: 0.62 },
  choAi: { gap: 6, padding: 14, borderWidth: 1 },
  dauAi: { flexDirection: "row", alignItems: "center", gap: 6 },
  giua: { textAlign: "center", paddingVertical: 8 },
  veCuoi: { alignSelf: "center", flexDirection: "row", alignItems: "center", gap: 8, minHeight: 48, borderWidth: 1, borderRadius: 24, paddingHorizontal: 16, marginVertical: 6 },
  lenh: { maxHeight: 200, flexGrow: 0, borderWidth: 1, borderRadius: 16, padding: 6, gap: 2 },
  lenhHang: { minHeight: 48, paddingHorizontal: 10, paddingVertical: 8, gap: 1 },
  soan: { flexDirection: "row", alignItems: "flex-end", gap: 6, padding: 6, borderWidth: 1, borderRadius: 22 },
  // Centred on purpose, unlike the kit's multiline `Field`: the composer grows
  // with its text, so one line sits in the middle of the pill and a longer
  // message fills it from the top anyway. Said out loud because Android's
  // default is what made every other box start mid-way (QA 23/09).
  oNhap: { flex: 1, minHeight: 48, maxHeight: 120, paddingHorizontal: 10, paddingVertical: 8, textAlignVertical: "center" },
});
