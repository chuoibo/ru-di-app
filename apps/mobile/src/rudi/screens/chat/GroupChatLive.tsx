/**
 * The group's shared invitation notebook, backed by the legacy Go candidate.
 * Message history and durable invalidations use independent receive cursors.
 * Polls and proposals become a contextual folded sheet; explicit AI requests
 * use a durable job with invocation-only consent. This surface still labels
 * its legacy transport honestly and never substitutes it for E2EE v2.
 */
import { Ionicons } from "@expo/vector-icons";
import { Image } from "expo-image";
import { useFocusEffect, useRouter } from "expo-router";
import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
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
  type TextStyle,
  View,
  useWindowDimensions,
} from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";

import { ApiError, newAttempt, taiAnhNhom, thongDiepNguoiDoc } from "../../../api";
import { danhSachThanhVien } from "../../../screens/vao-cua/cong-api";
import {
  cauYDinh,
  docTheAi,
  lichTrinhTrongThe,
  tacGiaTin,
  glyphPhanUng,
  khoaHang,
  trichTu,
  tinChoHoiThoai,
  type HangHienThi,
  type LoaiPhanUng,
  type Tin,
  type TrichDan,
} from "../../chat/tin-song";
import { TAT_KAV_QA } from "../../chat/qa-ban-phim";
import { cauLoiThaoTac, gocBong, nhomTheoQuang, viTriTrongCum } from "../../chat/nhip-tin";
import { docDaChan } from "../../cai-dat/quyen-rieng-tu";
import { CauTaiCho } from "../../ui/CauTaiCho";
import { boAnh, chonAnh, nenVaDung } from "../../ky-niem/chon-anh";
import { nguonAnh } from "../../ky-niem/ky-niem";
import { CHAT_VIEWABILITY } from "../../chat/viewability";
import { useBanNhap } from "../../chat/useBanNhap";
import { useTinNhan } from "../../chat/useTinNhan";
import { useChatChanges } from "../../chat/useChatChanges";
import { useChatAi } from "../../chat/useChatAi";
import { chuHangLoiGoi, laCapDoi, laTraLoiDangCho, lenhSanSang, loiGoiCuaPhong, thuLaiDuoc, type LenhAi } from "../../chat/ai-invocations";
import { luotChoNguoiXem } from "../../ai/phong-ai";
import { useRoomAi } from "../../ai/useRoomAi";
import { timNhacAi } from "../../chat/nhac-ai";
import { goiSeGui } from "../../chat/chip-boi-canh";
import { MO_DAU_HOI_AI, lenhGoiY } from "../../chat/khay-cong-cu";
import type { BoiCanh } from "../../ai/boi-canh";
import { laPair, tenCuocTroChuyen } from "../../nhan-rieng/nhan-rieng";
import { loaiSoCua } from "../../so/ban-tinh";
import { useRudiSession } from "../../session";
import { HangToGiaySong } from "../hai-nguoi/HangToGiaySong";
import { bangMauChat, mucNguoi, typography, useRudiTheme } from "../../theme";
import { IconButton, RudiButton } from "../../ui";
import { useMotion } from "../../ui/useMotion";
import { AvatarNguoi } from "../../ui/AvatarNguoi";
import { Sticker } from "../../ui/stickers/Sticker";
import type { TinChoGui } from "../../chat/hang-cho";
import { Sheet } from "../../ui/Sheet";
import { CaiDatNhomSheet } from "./CaiDatNhom";
import { KhaySticker } from "./KhaySticker";
import { NoiDungBaoCao } from "../nguoi/NoiDungBaoCao";
import { MenuTin } from "./MenuTin";
import { TheAiView } from "./TheAi";
import { TraLoiAi } from "./TraLoiAi";
import { HangTraLoiAiDangViet, TraLoiAiDangViet } from "./TraLoiAiDangViet";
import { ChipBoiCanh, TamXemBoiCanh } from "./ChipBoiCanh";
import { CongCuChat, DaiKeoSapToi, ToHen, type KhayChat } from "./SoHen";
import { docKeoCuaNhom } from "../../keo/keo";
import { chiaKeo, homNay, nhanNhip, nhipKeo } from "../../keo/nhip-keo";
import { gomBoiCanhChat } from "../../chat/boi-canh-chat";
import { KhayToHenChung } from "./ToHenChungKhay";
import { useToHenChung } from "../../chat/useToHenChung";
import { docKhoiNhap } from "../../chat/to-hen-chung";
import { Nep } from "../../ui/art/Nep";
import { useNepNguCanh } from "../../nep/NepProvider";
import { KHONG_VIEN_WEB } from "../../ui/khong-vien-web";
import { CuaDangNhap } from "../../ui/CuaDangNhap";
import { luiVeVe } from "../../lui-ve";
import { duongDangNhap } from "../../duong-vao";

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
            <View style={[styles.bong, gocBong("mot", true), { backgroundColor: colors.card, borderColor: colors.line }]}>
              <Text style={[tin.kind === "text" ? typography.body : typography.caption, BE_CHU, { color: colors.ink }]}>{tin.kind === "text" ? tin.than : tin.phuDe ?? "Ảnh"}</Text>
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

/**
 * One shape for a request to Rủ Đi AI that is waiting or did not go through:
 * the call that never reached the server and the invocation that failed are
 * the same news to the person who asked, so they read the same way -- a mark,
 * a title, the reason, and only the actions that can help (QA 27/09: three
 * kinds of failed-request card stacked under one message).
 */
function HangLoiNho({ title, cau, dangCho = false, testID, children }: { title: string; cau: string; dangCho?: boolean; testID?: string; children?: React.ReactNode }) {
  const { colors } = useRudiTheme();
  const nut = Array.isArray(children) ? children.some(Boolean) : Boolean(children);
  return (
    <View accessibilityLiveRegion="polite" style={[styles.invocation, { backgroundColor: colors.card, borderColor: colors.line }]} testID={testID}>
      <View style={styles.dauAi}>
        <Ionicons name={dangCho ? "time-outline" : "alert-circle-outline"} size={20} color={colors.inkSoft} />
        <Text style={[typography.label, styles.flex1, { color: colors.ink }]}>{title}</Text>
      </View>
      <Text style={[typography.note, { color: colors.inkSoft }]}>{cau}</Text>
      {nut ? <View style={styles.requestActions}>{children}</View> : null}
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
  // The room's answers as other members watch them: `ai` frames on the
  // feed's own socket (slice 12).
  const phongAi = useRoomAi(contextId);
  const changes = useChatChanges(contextId, personId, chat.nhanAnhChup, phongAi.nhan);
  // A two-person room re-reads what it is (friends or a couple) while open.
  const ai = useChatAi(contextId, personId, { haiNguoi: laPair(phien?.contexts?.find((n) => n.id === contextId)) });
  const { text: nhap, change: doiNhap, snapshot: nhapRef, clearIfUnchanged: xoaNhapCu } = useBanNhap();
  const [dangGui, setDangGui] = useState(false);
  // A model command gets an additional waiting row; the queue owns its text.
  const [dangGuiThan, setDangGuiThan] = useState<string | null>(null);
  const [banPhimMo, setBanPhimMo] = useState(false);
  // A composer action that failed (a photo, a malformed /vote): one sentence
  // at the newest end of the thread, just above the composer where the finger
  // was. The app's voice, never the AI's sparkle (QA UI-069).
  const [loiCuoi, setLoiCuoi] = useState<{ cau: string; hanhDong?: { label: string; onPress: () => void } } | null>(null);
  // An action on one message that failed (a reaction, a delete): said under
  // THAT message, which is where the reader is looking (QA UI-069).
  const [loiTin, setLoiTin] = useState<{ id: string; cau: string; thuLai: (() => void) | null } | null>(null);
  // Social v1.1 (ADR-0021): the sticker tray, the long-press menu of one
  // message, the message being replied to, and the group settings sheet.
  const [khaySticker, setKhaySticker] = useState(false);
  // The chip's «Xem» sheet, mounted at this screen's root like the others.
  const [xemBoiCanh, setXemBoiCanh] = useState(false);
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
  // «Chỉ gửi lời nhờ» on the chip above the send button. Back on after every
  // send: the choice is about one message, not a setting.
  const [kemTin, setKemTin] = useState(true);
  // The AI half of an `@Rủ Đi` send whose message has not landed yet, by the
  // send's attempt key: a message retried from its failed row still gets its
  // answer, with the bundle frozen at the first press.
  const capChoTin = useRef(new Map<string, { lenh: LenhAi; loiNho: string; goi: BoiCanh | undefined }>());
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
  // Who the roster last read as here now, and which messages were already in
  // the thread at that read.
  const [dangOIds, setDangOIds] = useState<ReadonlySet<string>>(new Set());
  const tinLucDoc = useRef<ReadonlySet<string>>(new Set());

  const nhom = phien?.contexts?.find((n) => n.id === contextId);
  // ADR-0023 §2.3.2: bị chặn, hoặc người kia đã xoá tài khoản. Tin cũ vẫn
  // đọc được -- chúng cũng là của người kia -- nhưng cửa soạn tin đóng, và
  // câu nói ra KHÔNG cho biết vì lý do nào trong hai lý do.
  const khongNhanTin = nhom?.unavailable === true;
  // The person who blocked is told so, and where to undo it; the person who
  // was blocked is not (ADR-0023 §2.3.2), so this reads the reader's OWN block
  // list and nothing about the other side (QA UI-079).
  const [toiDaChan, setToiDaChan] = useState(false);
  useEffect(() => {
    const kiaId = nhom?.counterpart?.id;
    if (!khongNhanTin || !kiaId || !personId) { setToiDaChan(false); return; }
    let song = true;
    docDaChan(personId)
      .then((ds) => { if (song) setToiDaChan(ds.blocked.some((b) => b.person_id === kiaId)); })
      .catch(() => undefined);
    return () => { song = false; };
  }, [khongNhanTin, nhom?.counterpart?.id, personId]);
  const tenNhom = tenCuocTroChuyen(nhom);
  // A pair (ADR-0021 §2.5) has no roster to show or invite into; the pill in
  // that place opens the other person's profile instead.
  const nhanRieng = laPair(nhom);
  const nguoiKiaId = nhom?.counterpart?.id;
  // Owner decision 2026-09-28: two classes. A group and an ordinary
  // two-person chat are friends, with the same AI and tools; a two-person chat
  // where both turned «Một đôi» on is a couple, which adds the pair's paper
  // and the couple stickers. Only the server knows the second fact
  // (`cap_doi`); an older server, or capabilities not read yet, is friends.
  const capDoi = nhanRieng && laCapDoi(ai.capabilities);
  // What Nếp may know here: the kind of notebook (friends `hoi`, or a
  // couple's `doi`, decided in one place by `loaiSoCua`) and how many are in
  // the room. Never a name and never a message -- chat v2 is end to end
  // encrypted, and Nếp does not read chat on its own (ADR-0033 §2.5).
  useNepNguCanh({
    man: "groups/[id]/chat",
    tieuDe: nhanRieng ? "cuộc trò chuyện của hai bạn" : "chat nhóm",
    loaiSo: nhom ? loaiSoCua(nhom, { bat: capDoi }) : undefined,
    soLieu: nhom ? { soNguoi: nhom.member_count ?? 0 } : undefined,
    goiY: capDoi
      ? ["Tuần này rủ nhau đi đâu?", "Mở tờ giấy của hai mình"]
      : nhanRieng ? ["Tuần này rủ nhau đi đâu?", "Tóm tắt kèo sắp tới"] : ["Gợi ý chỗ cho cả nhóm", "Tóm tắt kèo sắp tới"],
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
  const chiMinhToi = !nhanRieng && (soDangO ?? nhom?.member_count ?? 2) <= 1;
  useFocusEffect(
    useCallback(() => {
      setLanDoc((n) => n + 1);
    }, []),
  );
  // Read again when a message arrives from someone the roster does not count
  // as here: a stranger to it, or someone it read as only invited. The second
  // half is QA UI-122 -- the invitee's name was already known from the
  // invitation, so their first message changed nothing and the header kept
  // «2 thành viên» until the chat was reopened. Only messages newer than the
  // last read count, so a member who has left does not start a loop.
  const tacGiaChuaDem = useMemo(() => {
    const ids = new Set<string>();
    for (const t of chat.tin) {
      if (t.author_id === null || t.author_id === personId || tacGiaTin(t).loai === "ai") continue;
      if (!(t.author_id in tenTheoId) || (!dangOIds.has(t.author_id) && !tinLucDoc.current.has(t.id))) ids.add(t.author_id);
    }
    return [...ids].sort().join(",");
  }, [chat.tin, tenTheoId, dangOIds, personId]);
  useEffect(() => {
    if (tacGiaChuaDem !== "") setLanDoc((n) => n + 1);
  }, [tacGiaChuaDem]);
  const tinRef = useRef(chat.tin);
  tinRef.current = chat.tin;
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
        tinLucDoc.current = new Set(tinRef.current.map((t) => t.id));
        setTenTheoId(map);
        setDangOIds(new Set(ds.filter((tv) => tv.state === "active").map((tv) => tv.person_id)));
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
    (id: string | null) => {
      const tacGia = tacGiaTin({ author_id: id });
      return tacGia.loai === "ai" ? "Rủ Đi AI" : tacGia.id === personId ? "Bạn" : tenTheoId[tacGia.id] ?? "Thành viên";
    },
    [tenTheoId, personId],
  );

  const tinHien = useMemo(() => tinChoHoiThoai(chat.tin), [chat.tin]);
  // One time band per stretch of talk, not a time under every run (QA UI-064).
  const hang = useMemo(() => nhomTheoQuang(tinHien), [tinHien]);
  // Which messages are in the thread, for the streamed reply's hand-over: its
  // row gives way the moment the published card is here.
  const tinTheoId = useMemo(() => new Map(chat.tin.map((t) => [t.id, t])), [chat.tin]);
  const daCoThe = useCallback((messageId: string) => tinTheoId.has(messageId), [tinTheoId]);
  // Other members' answers being written, in the thread. The viewer's own
  // (their invocation, or an answer to their own message) is left to their
  // requester row, which follows the invocation itself.
  const traLoiPhong = useMemo(() => {
    const cuaToi = new Set(ai.requests.map((r) => r.id));
    return luotChoNguoiXem(phongAi.kho, (inv, tin) => cuaToi.has(inv) || tinTheoId.get(tin)?.author_id === personId);
  }, [phongAi.kho, ai.requests, tinTheoId, personId]);
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
  // The group's next outing, read on every focus with the roster: coming back
  // from making one shows it (QA UI-118). A failed read keeps what was shown.
  const [keoToi, setKeoToi] = useState<{ id: string; title: string; starts_on: string; ends_on: string } | null>(null);
  useEffect(() => {
    if (lanDoc === 0) return;
    let song = true;
    docKeoCuaNhom(contextId, personId)
      .then((ds) => { if (song) setKeoToi(chiaKeo(ds, homNay()).sapToi[0] ?? null); })
      .catch(() => undefined);
    return () => { song = false; };
  }, [lanDoc, contextId, personId]);
  // Every room pins its open tờ hẹn: a friends' two-person chat plans like a group.
  const toHenTimDuoc = useMemo(() => {
    const dangMo = (tin: Tin) => {
      const card = docTheAi(tin.card);
      if (card.loai === "itinerary") return !!card.nhapChung && card.nhapChung.status === "open" && !card.outingId;
      return card.loai === "poll" && !changes.votes[card.vote_id]?.is_closed && !changes.votes[card.vote_id]?.deleted;
    };
    // An answer in the thread that proposes an itinerary is a tờ hẹn too.
    const bat = (tin: Tin) => {
      const card = docTheAi(tin.card);
      return lichTrinhTrongThe(card) !== null || (card.loai === "poll" && !changes.votes[card.vote_id]?.is_closed && !changes.votes[card.vote_id]?.deleted);
    };
    // The slot answers "hội đang chốt cái gì?", so a sheet the group can still
    // edit outranks a finished one even when the finished one is newer. Without
    // this the strip says "Đã thành kèo" over an open sheet in the same screen.
    const mo = chat.tin.find(dangMo);
    if (mo) return { tin: mo, daThanhKeo: false };
    const tin = chat.tin.find(bat);
    if (!tin) return null;
    const the = lichTrinhTrongThe(docTheAi(tin.card));
    return { tin, daThanhKeo: !!the?.outingId || the?.nhapChung?.status === "promoted" };
  }, [chat.tin, changes.votes]);
  // The band shows what the group is deciding and what it has coming. A card
  // that already became an outing only points at one, so the next outing
  // takes its place. Both an open decision and an outing: the outing is a
  // slim line above the decision, except in a short window, where the band
  // keeps one item and the decision wins (a group whose poll is never closed
  // would otherwise never see its outing here).
  const toHen = toHenTimDuoc && (!toHenTimDuoc.daThanhKeo || keoToi === null) ? toHenTimDuoc.tin : null;
  const { height: caoCuaSo, width: rongCuaSo } = useWindowDimensions();
  // Below 360dp the header says the count only: «· sổ hẹn của hội» wrapped to
  // a line of its own and the header took four lines of a 640 window.
  const hepCuaSo = rongCuaSo < 360;
  const keoTrenBang = keoToi !== null && (toHen === null || caoCuaSo >= 600) ? keoToi : null;
  const coChu = nhap.trim().length > 0;
  const rongTrang = !chat.dangNap && chat.tin.length === 0 && dangGuiThan === null && chat.hangCho.length === 0;
  const thapCuaSo = caoCuaSo < 600;
  // While the tray is open in a short window the pinned rows fold away.
  const gonDau = khay !== null && thapCuaSo;
  // The composer grows with what is typed up to 120dp -- four lines, the
  // ceiling QA's acceptance pins (UI-062) and the most a phone can give the
  // box before the thread above it stops being the conversation -- and never
  // past 30% of a short window; then it scrolls inside itself.
  const caoOToiDa = Math.min(4 * DONG_O + 2 * DEM_O, Math.max(CAO_O + DONG_O, Math.round(caoCuaSo * 0.3)));
  const oNhapRef = useRef<TextInput>(null);
  // Web only: a textarea neither grows with its text nor starts at one row
  // (react-native-web leaves `rows` unset, and the browser draws two). Measure
  // the text at no height, then give the box exactly that, between one line
  // and the ceiling. Native multiline inputs already grow on their own.
  useLayoutEffect(() => {
    if (Platform.OS !== "web") return;
    const o = oNhapRef.current as unknown as HTMLTextAreaElement | null;
    if (!o?.style || typeof o.scrollHeight !== "number") return;
    o.style.height = "0px";
    const can = o.scrollHeight;
    o.style.height = `${Math.min(caoOToiDa, Math.max(CAO_O, can))}px`;
    o.style.overflowY = can > caoOToiDa ? "auto" : "hidden";
  }, [nhap, caoOToiDa]);
  // Enter sends where there is a hardware keyboard (a fine pointer is the
  // web's tell); Shift+Enter breaks the line, and a composing IME -- Telex on
  // a phone keyboard -- keeps its Enter.
  const enterGui = Platform.OS === "web" && typeof window !== "undefined" && window.matchMedia?.("(pointer: fine)").matches === true;
  // The message being typed also asks the AI (ADR-0046), the same way in
  // every room: the chip reads that command's readiness from the server, so
  // it never promises what the server refuses.
  const nhacDangGo = timNhacAi(nhap);
  // What would go with it: nothing at all when this server takes no bundle.
  const goiChip = ai.capabilities?.ai.share_scope === "caller_attached" ? boiCanhAi : null;
  const moLenh = (nhap.startsWith("/") && !nhap.includes(" ")) || nhap === "@";
  const lenhPhuHop = lenhGoiY(nhanRieng).filter((lenh) => lenh.nhan.toLocaleLowerCase().startsWith(nhap.toLocaleLowerCase()));
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
  // The composer's notice is a list header, not a row: `maintainVisibleContentPosition`
  // keeps row 0 in place and leaves the header under the composer, so it is
  // pulled into view. Always, not only at the end: it answers the press the
  // person just made down there (QA UI-069 measured it 1,500px off screen).
  useEffect(() => {
    if (loiCuoi !== null) {
      ganCuoi.current = true;
      setOCuoi(true);
      danhSachRef.current?.scrollToOffset({ offset: 0, animated: !reduced });
    }
  }, [loiCuoi, reduced]);
  useEffect(() => {
    const sub = Keyboard.addListener("keyboardDidShow", () => {
      if (ganCuoi.current) danhSachRef.current?.scrollToOffset({ offset: 0, animated: false });
    });
    return () => sub.remove();
  }, []);

  /** Ask the AI to answer a message that has just been stored. */
  const hoiAiVeTin = (khoa: string, trigger: string) => {
    const cap = capChoTin.current.get(khoa);
    if (!cap) return;
    capChoTin.current.delete(khoa);
    void ai.goiCap({ khoa, trigger, ...cap });
  };

  const gui = async (command?: string): Promise<boolean> => {
    const body = (command ?? nhap).trim();
    if (!body || guiRef.current) return false;
    if (body === "/vote") { setKhay("poll"); doiNhap(""); return false; }
    // An `@Rủ Đi`, `/plan` or `/chia-bill` message is an ordinary message
    // (ADR-0046): it is sent exactly like any other, and only once the server
    // has stored it is the AI asked, naming it. The key is the send's own, so
    // retrying either half can never double the other; the bundle is frozen
    // here, at the press, before the question joins the list it reads.
    const nhac = command === undefined ? timNhacAi(body) : null;
    const attempt = newAttempt();
    if (nhac !== null && lenhSanSang(ai.capabilities, nhac.lenh)) {
      capChoTin.current.set(attempt.key, { lenh: nhac.lenh, loiNho: nhac.loiNho, goi: goiSeGui(goiChip, kemTin) });
    }
    guiRef.current = true;
    setLoiCuoi(null);
    setDangGui(true);
    setDangGuiThan(body);
    if (command === undefined) doiNhap("");
    const traLoiCu = traLoi;
    setTraLoi(null);
    setKemTin(true);
    veCuoi();
    try {
      const daGui = await chat.gui(body, traLoiCu, attempt);
      // Landed for a conversation that has left the screen: nothing to show here.
      if (daGui === null) return false;
      hoiAiVeTin(attempt.key, daGui.id);
      veCuoi();
      const cau = cauYDinh(daGui);
      if (cau !== null) setLoiCuoi({ cau });
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
    const card = lichTrinhTrongThe(docTheAi(tin?.card));
    if (card?.outingId) { router.push(`/outings/${card.outingId}` as never); return; }
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
      setLoiCuoi({ cau: error instanceof ApiError ? error.message : "Chưa mở được thư viện ảnh trên máy này." });
      return;
    }
    if (daChon === null || !songRef.current) {
      guiAnhRef.current = false;
      if (daChon !== null) await boAnh(daChon);
      return;
    }
    const draft = nhapRef.current;
    const caption = draft.text.trim();
    setLoiCuoi(null);
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
        const loi = cauLoiThaoTac("gửi được ảnh", error);
        setLoiCuoi({ cau: loi.cau, hanhDong: loi.thuLai ? { label: "Chọn lại ảnh", onPress: () => void guiAnh() } : undefined });
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
      const daGui = await chat.thuLaiMot(khoa);
      if (daGui !== null) {
        hoiAiVeTin(khoa, daGui.id);
        veCuoi();
      }
    } catch {
      // Same as above: the row itself carries the second refusal.
    }
  };

  /** Take back one's own message; the server's refusal is shown as its sentence. */
  const xoaTinChon = async (tin: Tin) => {
    setMenuTin(null);
    setLoiTin(null);
    try {
      await chat.xoaTin(tin.id);
    } catch (error) {
      const loi = cauLoiThaoTac("xoá được tin này", error);
      setLoiTin({ id: tin.id, cau: loi.cau, thuLai: loi.thuLai ? () => void xoaTinChon(tin) : null });
    }
  };

  /** Keep the session's copy of the group current so the header and theme follow at once. */
  const nhomDaDoi = (thay: { display_name?: string; theme?: string }) => {
    if (phien === null || !phien.contexts) return;
    datPhien({ ...phien, contexts: phien.contexts.map((n) => (n.id === contextId ? { ...n, ...thay } : n)) });
  };

  const phanUng = async (tin: Tin, kind: LoaiPhanUng) => {
    setMenuTin(null);
    setLoiTin(null);
    const cuaToi = tin.reactions?.some((r) => r.kind === kind && r.mine) ?? false;
    try {
      await chat.doiPhanUng(tin.id, kind, cuaToi);
    } catch (error) {
      const loi = cauLoiThaoTac(cuaToi ? `bỏ ${glyphPhanUng(kind)}` : `thả ${glyphPhanUng(kind)}`, error);
      setLoiTin({ id: tin.id, cau: loi.cau, thuLai: loi.thuLai ? () => void phanUng(tin, kind) : null });
    }
  };

  const renderItem = ({ item, index }: { item: HangHienThi; index: number }) => {
    if (item.loai === "ngay") {
      // A band where the talk paused: the day where it starts, then the time
      // it resumed. The only times on the thread; one message's own time is
      // in its menu (QA UI-064).
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
    const theCuaTin = docTheAi(tin.card);
    // A poll is a member's message: it hangs from its sender like any other,
    // name above and avatar beside, and is not a run's bubble.
    const laPoll = laAi && theCuaTin.loai === "poll" && tacGiaTin(tin).loai !== "ai";
    const cuaNguoi = !laAi || laPoll;
    const benKia = cuaNguoi && !cuaToi;
    const benToi = cuaNguoi && cuaToi;
    const laSticker = tin.kind === "sticker";
    const daXoa = tin.kind === "deleted";
    const chips = (tin.reactions ?? []).filter((r) => r.count > 0);
    // The theme colours the sender's bubble; a taken-back row is paper again.
    const nenBong = daXoa ? colors.card : cuaToi ? mauChat.bubble : colors.card;
    const vienBong = daXoa ? colors.line : cuaToi ? mauChat.bubble : colors.line;
    const mucBong = cuaToi && !daXoa ? mauChat.bubbleInk : colors.ink;
    const viTri = viTriTrongCum(hang, index);
    const dauChuoi = viTri === "mot" || viTri === "dau";
    const cuoiChuoi = viTri === "mot" || viTri === "cuoi";
    const loiCuaTin = loiTin !== null && loiTin.id === tin.id ? loiTin : null;
    return (
      // The name, the quote and the reactions sit around the line, not in it:
      // the line holds only the avatar and the bubble, so the avatar's foot is
      // the last bubble's foot (QA UI-064 measured it 22px low, beside the
      // time). Runs sit 2dp apart, separate voices 12dp.
      <View style={[styles.tinKhoi, { paddingTop: dauChuoi ? 12 : 2 }]}>
        {benKia && dauChuoi ? (
          // The sender in their own ink (ADR-0037 D6): the same colour as their avatar ring.
          <Text style={[typography.caption, styles.thut, { color: tin.author_id ? mucNguoi(tin.author_id, dark) : colors.inkSoft }]}>{tenNguoi(tin.author_id)}</Text>
        ) : null}
        <View testID={`chat-message-${tin.id}`} style={[styles.hang, benToi && styles.hangToi, laAi && !laPoll && styles.hangGiua]}>
          {benKia ? (
            cuoiChuoi ? <AvatarNguoi name={tenNguoi(tin.author_id)} personId={tin.author_id} size={30} /> : <View style={styles.choChuDau} />
          ) : null}
          <View style={[styles.khoi, benToi && styles.khoiToi, laPoll ? styles.khoiPoll : laAi && styles.khoiAi]}>
            {/* The quote sits ABOVE the bubble, in the bubble's column: the
                avatar beside the column still meets the bubble's foot, and the
                quote stays part of the message it introduces. Never in a
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
            {laAi && theCuaTin.loai === "tra_loi" && tacGiaTin(tin).loai === "ai" ? (
              <TraLoiAi
                contextId={contextId}
                onMenu={() => setMenuTin(tin)}
                onOpenPlan={() => moToHen(tin)}
                personId={personId}
                tenNguoi={tenNguoi}
                the={theCuaTin}
                tin={tin}
              />
            ) : laAi ? (
              <TheAiView
                the={theCuaTin}
                contextId={contextId}
                personId={personId}
                tenNguoi={tenNguoi}
                tacGia={tenNguoi(tin.author_id)}
                vote={theCuaTin.loai === "poll" ? changes.votes[theCuaTin.vote_id] : undefined}
                onOpenPlan={() => moToHen(tin)}
                onMoToHen={(voteId, goiY) => void moToHenTuBinhChon(voteId, goiY)}
                banToHen={theCuaTin.loai === "poll" ? (coToHenChoBinhChon(theCuaTin.vote_id)?.ban ?? null) : null}
                daCoToHen={theCuaTin.loai === "poll" && coToHenChoBinhChon(theCuaTin.vote_id) !== null}
                tenToHen={theCuaTin.loai === "poll" ? (coToHenChoBinhChon(theCuaTin.vote_id)?.ten || null) : null}
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
                style={[styles.bong, gocBong(viTri, cuaToi), { backgroundColor: nenBong, borderColor: vienBong }]}
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
                    {tin.body ? <Text style={[typography.body, BE_CHU, { color: mucBong }]}>{tin.body}</Text> : null}
                  </View>
                ) : (
                  <Text style={[typography.body, BE_CHU, { color: mucBong }]}>{tin.body}</Text>
                )}
              </Pressable>
            )}
          </View>
        </View>
        {chips.length > 0 ? (
          <View style={[styles.duoiBong, benKia && styles.thut, benToi && styles.duoiToi]}>
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
        ) : null}
        {loiCuaTin ? (
          <CauTaiCho
            cau={loiCuaTin.cau}
            co="nho"
            hanhDong={loiCuaTin.thuLai ? { label: "Thử lại", onPress: loiCuaTin.thuLai } : undefined}
            style={[benKia && styles.thut, benToi && styles.loiTinToi, styles.loiTin]}
            testID="chat-loi-tin"
          />
        ) : null}
      </View>
    );
  };

  if (phien === null) return <CuaDangNhap />;

  return (
    <KeyboardAvoidingView
      behavior={Platform.OS === "ios" ? "padding" : "height"}
      // Off only under the QA negative control; see `qa-ban-phim.ts`.
      enabled={!TAT_KAV_QA}
      style={[styles.man, { backgroundColor: colors.ground, paddingTop: insets.top }]}
    >
      <View style={[styles.dau, { paddingHorizontal: space.md, borderBottomColor: colors.line }]}>
        <View style={styles.chatHeader}>
          <IconButton accessibilityLabel="Quay lại" icon="chevron-back" quiet onPress={() => luiVeVe(router as never, "/messages")} />
          <Pressable accessibilityRole="button" accessibilityLabel={nhanRieng ? "Xem hồ sơ" : "Thành viên nhóm"}
            onPress={() => router.push((nhanRieng && nguoiKiaId ? `/people/${nguoiKiaId}` : `/groups/${contextId}/members`) as never)} style={styles.headerIdentity}>
            <Text numberOfLines={1} style={[typography.title, { color: colors.ink }]}>{tenNhom}</Text>
            <Text style={[typography.caption, { color: colors.inkSoft }]}>
              {nhanRieng ? "Cuộc trò chuyện của hai mình" : hepCuaSo ? `${soDangO || nhom?.member_count || 1} thành viên` : `${soDangO || nhom?.member_count || 1} thành viên · sổ hẹn của hội`}
            </Text>
          </Pressable>
          <IconButton accessibilityLabel={nhanRieng ? "Cài đặt cuộc trò chuyện" : "Cài đặt nhóm"} icon="ellipsis-horizontal" quiet onPress={() => setCaiDatMo(true)} />
        </View>
        {/* The pinned paper is a couple's; a friends' pair reaches the paper
            (and «Một đôi») from the settings row «Tờ giấy của hai mình», and
            sees a slim line here only while the other's proposal waits for
            an answer (`hangGhimChat`). Pairs only: a group has no notebook. */}
        {nhanRieng && phien !== null && !khongNhanTin && !gonDau ? <HangToGiaySong capDoi={capDoi} contextId={contextId} tenNguoiKia={tenNhom} toiId={phien.person_id} /> : null}
        <View style={styles.baoMat}>
          <Ionicons name="lock-open-outline" size={13} color={colors.inkSoft} />
          <Text style={[typography.caption, { color: colors.inkSoft }]}>Chưa mã hoá đầu cuối</Text>
          {changes.connection === "recovering" ? <Text accessibilityLiveRegion="polite" style={[typography.caption, { color: colors.inkSoft }]}>· Đang nối lại</Text> : null}
        </View>
      </View>
      {/* B2 (QC 24/09): the pinned sheet sits on its own solid band with a rule
          under it, so the thread visibly starts below it. On the bare ground
          with margins round it, a bubble clipped at the list's top edge read as
          a bubble cut by the bar. */}
      {/* The pinned rows give way to an open tray in a short window and come
          back when it closes (`gonDau`): at 390×460 a couple's room kept its
          paper row and band, and the tray had too little left for its labels. */}
      {(toHen || keoTrenBang) && !gonDau ? (
        <View style={[styles.dayGhim, { backgroundColor: colors.ground, borderBottomColor: colors.line }]} testID="day-ghim">
          {keoTrenBang ? (
            <DaiKeoSapToi gon={toHen !== null} nhip={nhanNhip(nhipKeo(keoTrenBang.starts_on, keoTrenBang.ends_on, homNay()))} onOpen={() => router.push(`/outings/${keoTrenBang.id}` as never)} ten={keoTrenBang.title} />
          ) : null}
          {toHen ? (
            <ToHen haiNguoi={nhanRieng} tin={toHen} onOpen={moToHen} onVote={(tin) => {
              const index = hang.findIndex((row) => row.loai === "tin" && row.tin.id === tin.id);
              if (index >= 0) danhSachRef.current?.scrollToIndex({ index, animated: !reduced, viewPosition: 0.5 });
            }} />
          ) : null}
        </View>
      ) : null}
      {/* Drawn outside the inverted list: the list flips its own children
          back upright, and an extra flip here once mirrored this copy. It is
          the flexible part of the column now, and scrolls inside itself: as a
          fixed block it pushed the composer under the bottom edge of a short
          window (QA UI-124, 390×460 and 375×667 with the tray open). */}
      {rongTrang ? (
        <ScrollView contentContainerStyle={[styles.rongNoi, { paddingHorizontal: space.md }]} keyboardShouldPersistTaps="handled" style={styles.rong} testID="chat-rong">
          {chat.loi ? (
            // The first page did not load and nothing came by the feed either.
            <CauTaiCho
              cau={chat.loi}
              hanhDong={chat.loiLoai === "phien"
                ? { label: "Đăng nhập lại", onPress: () => router.push(duongDangNhap(`/groups/${contextId}/chat`) as never) }
                : chat.loiLoai === "vinh-vien"
                  ? { label: "Về Tin nhắn", onPress: () => luiVeVe(router as never, "/messages") }
                  : { label: "Thử lại", onPress: () => void chat.taiLai() }}
              testID="chat-loi-dau"
            />
          ) : khongNhanTin || gonDau ? null : (
            <View style={styles.moLoi}>
              {/* The sketch is the first thing to give way: a short window or
                  an open tray keeps the words and the one action. */}
              {thapCuaSo || khay !== null ? null : <Nep pose="moi" size={96} />}
              {/* A group of one (just opened, QA UI-071) asks for friends first:
                  a plan for nobody is not the next step. */}
              <Text style={[typography.h2, styles.giua, { color: colors.ink }]}>{nhanRieng ? "Một lời mở đầu." : chiMinhToi ? "Hội mới, mới có mình bạn." : "Có hội rồi. Mở lời thôi."}</Text>
              <Text style={[typography.body, styles.giua, { color: colors.inkSoft }]}>{nhanRieng ? `Một tin nhắn nhỏ cho ${tenNhom}.` : chiMinhToi ? "Mời vài người bạn vào, rồi cùng rủ nhau một buổi." : "Từ một câu rủ, thành một buổi cùng đi."}</Text>
              {chiMinhToi ? <RudiButton icon="person-add-outline" label="Mời bạn vào nhóm" full={false} onPress={() => router.push(`/groups/${contextId}/invite` as never)} /> : null}
              {/* Every room, a friends' pair included, can start a plan from here. */}
              <RudiButton label={nhanRieng ? "Rủ đi một buổi" : "Rủ hội một buổi"} variant={chiMinhToi ? "ghost" : "outline"} full={false} onPress={() => setKhay("plan")} />
            </View>
          )}
        </ScrollView>
      ) : null}
      {/* The list and the jump pill share one box: the pill floats over the
          newest end instead of taking 60dp from the list, which made the
          thread jump every time it came and went. */}
      <View style={rongTrang ? styles.dsRong : styles.ds}>
        <FlatList
          ref={danhSachRef}
          // Empty, the list keeps only its header's height and the empty page
          // above takes the room.
          style={rongTrang ? styles.dsRongTrong : styles.ds}
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
              <CauTaiCho cau={ai.error} style={styles.loiCuoi} />
              {/* The AI half of an `@Rủ Đi` send that failed after the message
                  landed. Only the person who asked sees it; «Thử lại» replays the
                  same key with the same frozen bundle. */}
              {ai.cho.filter((cap) => cap.loi !== null).map((cap) => (
                <HangLoiNho cau={cap.loi ?? ""} key={cap.khoa} testID="chat-loi-nho-hong" title="Rủ Đi AI chưa nhận lời nhờ">
                  <RudiButton label="Thử lại" compact full={false} variant="outline" onPress={() => ai.thuLaiCap(cap.khoa)} />
                  <RudiButton label="Bỏ" compact full={false} variant="ghost" onPress={() => ai.boCap(cap.khoa)} />
                </HangLoiNho>
              ))}
              {/* The requester's own answer in the thread while it is written:
                  the reading sentence, the words as they come, then the real
                  card (slice 11). */}
              {ai.requests.filter(laTraLoiDangCho).map((request) => (
                <TraLoiAiDangViet
                  contextId={contextId}
                  daCoThe={daCoThe}
                  docMot={ai.docMot}
                  giamChuyenDong={reduced}
                  key={`song-${request.id}`}
                  khiKetThuc={ai.lamMoi}
                  personId={personId}
                  request={request}
                  tenNguoi={tenNguoi}
                  trigger={request.trigger_message_id ? tinTheoId.get(request.trigger_message_id) ?? null : null}
                />
              ))}
              {/* Everyone else in the room watches the same answer: the same row
                  and state machine, fed by the feed socket's `ai` frames, until
                  the card arrives (slice 12). */}
              {traLoiPhong.map((l) => (
                <HangTraLoiAiDangViet
                  daCoThe={daCoThe}
                  giamChuyenDong={reduced}
                  key={`phong-${l.inv}`}
                  nguoiXem="thanh_vien"
                  request={loiGoiCuaPhong(l.inv, l.tin, l.soTin)}
                  tenNguoi={tenNguoi}
                  traLoi={l.traLoi}
                  trigger={tinTheoId.get(l.tin) ?? null}
                />
              ))}
              {ai.requests.filter((request) => request.status !== "succeeded" && request.status !== "cancelled" && !laTraLoiDangCho(request)).map((request) => (
                <HangLoiNho cau={chuHangLoiGoi(request).cau} dangCho={request.status !== "failed"} key={request.id} title={chuHangLoiGoi(request).tieuDe}>
                  {request.status === "failed" && thuLaiDuoc(request) ? <RudiButton label="Thử lại lời nhờ" compact full={false} variant="outline" loading={ai.busy} disabled={ai.busy || !lenhSanSang(ai.capabilities, request.command ?? "plan")} lyDo={lenhSanSang(ai.capabilities, request.command ?? "plan") ? undefined : "Rủ Đi AI chưa sẵn sàng trong cuộc trò chuyện này."} onPress={() => void ai.retry(request.id)} /> : null}
                  {request.status === "failed" && (request.command ?? "plan") === "plan" ? <RudiButton label="Tự tạo kèo" compact full={false} variant="ghost" onPress={() => moToHen()} /> : null}
                </HangLoiNho>
              ))}
              {/* Every logical send owns its pending and failed row. */}
              {chat.hangCho.map((t) => (
                <HangChoGui key={t.attempt.key} onBoQua={() => { capChoTin.current.delete(t.attempt.key); chat.boQua(t.attempt.key); }} onThuLai={() => void thuLaiGui(t.attempt.key)} tin={t} />
              ))}
              {dangGuiThan === null && loiCuoi !== null ? (
              <CauTaiCho cau={loiCuoi.cau} hanhDong={loiCuoi.hanhDong} style={styles.loiCuoi} testID="chat-loi-cuoi" />
            ) : dangGuiThan !== null ? (
              <View style={styles.choGui}>
                {/* Only when the AI will really be asked: a command the server
                    is not ready for goes as an ordinary message, as the chip
                    above the send button just said (QA UI-164). */}
                {(() => { const nhac = timNhacAi(dangGuiThan); return nhac !== null && lenhSanSang(ai.capabilities, nhac.lenh); })() ? (
                  <View style={styles.hang}>
                    <View style={[styles.khoi, styles.khoiAi]}>
                      <View style={[styles.choAi, { backgroundColor: colors.card, borderColor: colors.line, borderRadius: radius.base }]}>
                        <View accessibilityLiveRegion="polite" style={styles.dauAi}>
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
            ) : chat.loiCu ? (
              // Where the older messages would have appeared: the top of the thread.
              <CauTaiCho cau={chat.loiCu} hanhDong={{ label: "Thử lại", onPress: () => void chat.napCuHon() }} style={styles.loiCuoi} testID="chat-loi-cu" />
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
      </View>
      {moLenh && lenhPhuHop.length > 0 ? (
        <ScrollView keyboardShouldPersistTaps="handled" style={[styles.lenh, { backgroundColor: colors.card, borderColor: colors.line, marginHorizontal: space.md }]}>
          {lenhPhuHop.map((l) => (
            <Pressable accessibilityRole="button" key={l.nhan} onPress={() => { if (l.nhan === "/vote") { setKhay("poll"); doiNhap(""); } else doiNhap(l.goiY); }} style={styles.lenhHang}>
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
      {!khongNhanTin && !toHenChung.sheet ? <CongCuChat personId={personId} contextId={contextId} panel={khay} onPanel={setKhay} capabilities={ai.capabilities} busy={dangGui || ai.busy}
        error={ai.error}
        onImage={() => { setKhay(null); void guiAnh(); }}
        onSticker={() => { setKhay(null); setKhaySticker(true); }}
        onPoll={gui}
        // The tray no longer asks the AI itself: it starts the message.
        onHoiAi={() => { setKhay(null); if (timNhacAi(nhapRef.current.text) === null) doiNhap((MO_DAU_HOI_AI + nhapRef.current.text).trimEnd() + " "); }}
        onManual={() => { setKhay(null); moToHen(); }}
        haiNguoi={nhanRieng}
        onToGiay={capDoi ? () => router.push(`/groups/${contextId}/to-giay` as never) : undefined} /> : null}
      {!khongNhanTin && nhacDangGo !== null ? (
        <View style={{ marginHorizontal: space.md }}>
          <ChipBoiCanh goi={goiChip} haiNguoi={nhanRieng} kemTin={kemTin} onDoi={setKemTin} onXem={() => setXemBoiCanh(true)} sanSang={lenhSanSang(ai.capabilities, nhacDangGo.lenh)} />
        </View>
      ) : null}
      {khongNhanTin ? (
        <View style={[styles.dungNhan, { backgroundColor: colors.card, borderColor: colors.line, marginHorizontal: space.md, marginBottom: Math.max(insets.bottom, 10) }]} testID="chat-dung-nhan">
          <Text style={[typography.body, { color: colors.ink }]}>
            {toiDaChan ? `Bạn đã chặn ${tenNhom}.` : "Cuộc trò chuyện này không còn nhận tin."}
          </Text>
          {toiDaChan || chat.tin.length > 0 ? (
            <Text style={[typography.note, { color: colors.inkSoft }]}>
              {[toiDaChan ? "Hai bạn không nhắn cho nhau được nữa." : null, chat.tin.length > 0 ? "Tin cũ vẫn đọc được ở trên." : null].filter(Boolean).join(" ")}
            </Text>
          ) : null}
          {toiDaChan ? <RudiButton compact full={false} label="Xem danh sách đã chặn" onPress={() => router.push("/settings/da-chan" as never)} variant="ghost" /> : null}
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
            onKeyPress={enterGui ? (e) => {
              const phim = e as unknown as { key?: string; shiftKey?: boolean; nativeEvent: { isComposing?: boolean }; preventDefault: () => void };
              if (phim.key === "Enter" && !phim.shiftKey && !phim.nativeEvent.isComposing) {
                phim.preventDefault();
                void gui();
              }
            } : undefined}
            ref={oNhapRef}
            {...(Platform.OS === "web" ? { rows: 1 } : null)}
            placeholder={nhanRieng ? `Nhắn cho ${tenNhom}` : "Nhắn cho hội…"}
            placeholderTextColor={colors.inkSoft}
            selectionColor={colors.accentSoft}
            style={[typography.body, styles.oNhap, { color: colors.ink, maxHeight: caoOToiDa }, KHONG_VIEN_WEB]}
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
      <KhaySticker capDoi={capDoi} onChon={(id) => void guiStickerChon(id)} onClose={() => setKhaySticker(false)} open={khaySticker} />
      <TamXemBoiCanh goi={goiChip} haiNguoi={nhanRieng} onClose={() => setXemBoiCanh(false)} open={xemBoiCanh && !khongNhanTin && nhacDangGo !== null} />
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

/** The composer: one line of body text, its padding, and the 48dp it adds up to. */
const DONG_O = typography.body.lineHeight;
const DEM_O = (48 - DONG_O) / 2;
const CAO_O = DONG_O + 2 * DEM_O;

/**
 * Web only: a word longer than the bubble breaks anywhere. The browser's
 * default `overflow-wrap: break-word` wraps it but still sizes the bubble to
 * the whole word, so a long link pushed it out of its column (QA UI-063).
 */
const BE_CHU: TextStyle | null = Platform.OS === "web" ? ({ wordBreak: "break-word" } as unknown as TextStyle) : null;

const styles = StyleSheet.create({
  dayGhim: { borderBottomWidth: StyleSheet.hairlineWidth, paddingBottom: 4, zIndex: 1 },
  chatHeader: { flexDirection: "row", alignItems: "center", gap: 4, minHeight: 64 },
  headerIdentity: { flex: 1, minHeight: 48, justifyContent: "center", gap: 3, paddingHorizontal: 4 },
  moLoi: { alignItems: "center", gap: 6 },
  invocation: { borderWidth: 1, borderRadius: 12, padding: 14, gap: 8, marginBottom: 10 },
  requestActions: { flexDirection: "row", flexWrap: "wrap", gap: 6 },
  dungNhan: { borderWidth: StyleSheet.hairlineWidth, borderRadius: 14, padding: 12, gap: 4 },
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
  danhSach: { paddingTop: 12, paddingBottom: 22 },
  ngay: { flexDirection: "row", alignItems: "center", gap: 10, paddingTop: 16, paddingBottom: 4 },
  duong: { flex: 1, height: StyleSheet.hairlineWidth },
  hang: { flexDirection: "row", alignItems: "flex-end", gap: 8 },
  hangToi: { justifyContent: "flex-end" },
  // A card nobody in the room sent (the AI's, the group's shared sheet) is an
  // object laid across the thread: full width on a phone, centred in its
  // reading column on a tablet, never hanging from the avatar gutter.
  hangGiua: { justifyContent: "center" },
  // `minWidth: 0` lets the column be narrower than its longest word, so a
  // link breaks inside the bubble instead of pushing it past 82% and off the
  // left edge (QA UI-063: 346px at every width).
  khoi: { maxWidth: "82%", minWidth: 0, flexShrink: 1, gap: 4 },
  khoiToi: { alignItems: "flex-end" },
  // A card is an object in the thread: the full line on a phone, a reading
  // column on a tablet.
  khoiAi: { maxWidth: 640, flex: 1 },
  khoiPoll: { maxWidth: 560, flex: 1 },
  choChuDau: { width: 30, height: 30 },
  bong: { borderWidth: 1, paddingHorizontal: 14, paddingVertical: 10, minWidth: 0 },
  duoiBong: { flexDirection: "row", alignItems: "center", gap: 6, flexWrap: "wrap" },
  chip: { minHeight: 48, justifyContent: "center", borderWidth: 1, borderRadius: 999, paddingHorizontal: 10, paddingVertical: 2 },
  baoMat: { flexDirection: "row", alignItems: "center", justifyContent: "center", gap: 5, flexWrap: "wrap" },
  dau: { paddingBottom: 8, borderBottomWidth: StyleSheet.hairlineWidth },
  rong: { flex: 1 },
  rongNoi: { flexGrow: 1, justifyContent: "center", paddingVertical: 16 },
  ds: { flex: 1 },
  dsRong: { flexGrow: 0, flexShrink: 1 },
  dsRongTrong: { flexGrow: 0 },
  tinKhoi: { gap: 3 },
  // The name, quote and reactions start where the bubble does, past the avatar.
  thut: { marginLeft: 38 },
  duoiToi: { justifyContent: "flex-end" },
  loiTin: { marginTop: 2 },
  // Under your own message the sentence hangs from the right, like the bubble.
  loiTinToi: { alignSelf: "flex-end", justifyContent: "flex-end", maxWidth: "82%" },
  loiCuoi: { paddingVertical: 8 },
  choGui: { gap: 12 },
  // Two content-sized buttons that may wrap. `RudiButton` is full-width by
  // default, and two full-width buttons in one row pushed «Thử lại» off the
  // left edge of the screen (audit native 09/09, F41). At large text the pair
  // stacks, right-aligned, instead of shrinking or hiding its label.
  nutHong: { flexWrap: "wrap", justifyContent: "flex-end" },
  mo: { opacity: 0.62 },
  choAi: { gap: 6, padding: 14, borderWidth: 1 },
  dauAi: { flexDirection: "row", alignItems: "center", gap: 6 },
  flex1: { flex: 1 },
  giua: { textAlign: "center", paddingVertical: 8 },
  veCuoi: { position: "absolute", bottom: 8, alignSelf: "center", flexDirection: "row", alignItems: "center", gap: 8, minHeight: 48, borderWidth: 1, borderRadius: 24, paddingHorizontal: 16 },
  lenh: { maxHeight: 200, flexGrow: 0, borderWidth: 1, borderRadius: 16, padding: 6, gap: 2 },
  lenhHang: { minHeight: 48, paddingHorizontal: 10, paddingVertical: 8, gap: 1 },
  soan: { flexDirection: "row", alignItems: "flex-end", gap: 6, padding: 6, borderWidth: 1, borderRadius: 22 },
  // Centred on purpose, unlike the kit's multiline `Field`: the composer grows
  // with its text, so one line sits in the middle of the pill and a longer
  // message fills it from the top anyway. Said out loud because Android's
  // default is what made every other box start mid-way (QA 23/09).
  // One line is exactly the 48dp of «+» and the send button beside it, so the
  // three share a centre line; more lines grow upward from the same foot.
  oNhap: { flex: 1, minHeight: CAO_O, paddingHorizontal: 10, paddingVertical: DEM_O, textAlignVertical: "center" },
});
