/**
 * One room on the chat v2 lane (ADR-0057), shaped like `useTinNhan` so the
 * chat screen draws either lane with the same code.
 *
 * - The native module seals every change before its answer; this hook only
 *   orchestrates (`MayMaHoa`) and draws (`so-tin`).
 * - It polls the lane while focused (the WebSocket stream is the next step);
 *   history starts where this device joined -- earlier messages were never
 *   encrypted to it, and the legacy archive stays readable on its own.
 * - What the room shows comes from the device's sealed record (`so-phong`),
 *   read before the network: history survives a restart and works offline.
 * - Nothing here ever writes plaintext anywhere: a failure is said, not
 *   routed around.
 */
import AsyncStorage from "@react-native-async-storage/async-storage";
import { File } from "expo-file-system";
import { useFocusEffect } from "expo-router";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Image, Platform } from "react-native";

import { ApiError, goiNhiPhan, thongDiepNguoiDoc } from "../../../api";
import { makeIdFactory } from "../../../participants";
import { ChatCryptoModule } from "../../../../modules/rudi-chat-crypto";
import type { TinChoGui } from "../hang-cho";
import type { LoaiPhanUng, PhanUngTomTat, Tin, TinDaGui } from "../tin-song";
import { PHAN_UNG } from "../tin-song";
import { danhSachThanhVien } from "../../../screens/vao-cua/cong-api";
import { apiV2 } from "./api-v2";
import type { MediaRef, Operation } from "./kieu";
import { MayMaHoa, type KhoTinPort, type ThietBiMoi } from "./may-ma-hoa";
import { PHONG_TRONG, dungPhong, type Cho, type SoPhong } from "./so-phong";
import { danhSach, moiTruoc, type TinV2 } from "./so-tin";

const NHIP_MS = 3000;

/** Base64 of bytes in chunks: spreading megabytes into one call overflows the stack. */
export function base64TuByte(bytes: Uint8Array): string {
  let s = "";
  for (let i = 0; i < bytes.length; i += 0x8000) s += String.fromCharCode(...bytes.subarray(i, i + 0x8000));
  return btoa(s);
}
const uuid = makeIdFactory();

const kho = {
  doc: (k: string) => AsyncStorage.getItem(k),
  ghi: (k: string, v: string) => AsyncStorage.setItem(k, v),
};

/** One engine per signed-in person on this phone. */
const mayTheoNguoi = new Map<string, Promise<MayMaHoa>>();

/**
 * Everyone listening for new devices, whichever room or screen opened the
 * engine first: a notice is never bound to the first caller's closure
 * (security review 05/10: notices for later rooms were dropped).
 */
const ngheThietBiMoi = new Set<(m: ThietBiMoi) => void>();

/** Everyone drawing a room, told when its record grows (whoever made it grow). */
const ngheDoiPhong = new Set<(room: string) => void>();

export function coMaHoa(): boolean {
  return ChatCryptoModule !== null;
}

export async function mayCua(personId: string): Promise<MayMaHoa> {
  const native = ChatCryptoModule;
  if (native === null) throw new ApiError(0, "chat_v2_native_missing", "Bản ứng dụng này chưa có mã hoá đầu cuối. Cập nhật ứng dụng để nhắn trong phòng này.");
  let co = mayTheoNguoi.get(personId);
  if (co === undefined) {
    co = (async () => {
      const so: KhoTinPort = {
        doc: async (room) => {
          const raw = await native.roomRead(room);
          return raw === null ? null : (JSON.parse(raw) as SoPhong);
        },
        noi: (room, cursor, ban) => native.roomAppend(room, cursor, JSON.stringify(ban)),
      };
      const may = new MayMaHoa({ actorId: personId, crypto: native, api: apiV2, kho, so, uuid, label: Platform.OS === "ios" ? "iPhone" : "Android",
        onThietBiMoi: (m) => { for (const nghe of ngheThietBiMoi) nghe(m); },
        onPhong: (room) => { for (const nghe of ngheDoiPhong) nghe(room); } });
      await may.moThietBi();
      return may;
    })();
    co.catch(() => mayTheoNguoi.delete(personId));
    mayTheoNguoi.set(personId, co);
  }
  return co;
}

const GLYPH: Record<string, LoaiPhanUng> = Object.fromEntries(PHAN_UNG.map((p) => [p.glyph, p.kind]));
const KIND_GLYPH: Record<LoaiPhanUng, string> = Object.fromEntries(PHAN_UNG.map((p) => [p.kind, p.glyph])) as Record<LoaiPhanUng, string>;

/** A v2 message in the legacy `Tin` shape the screen draws. */
export function sangTin(t: TinV2, contextId: string, personId: string, anhDaMo: Record<string, string>): Tin {
  const reactions: PhanUngTomTat[] = Object.keys(t.reactions)
    .filter((g) => Object.hasOwn(GLYPH, g))
    .map((g) => ({ kind: GLYPH[g], count: t.reactions[g].length, mine: t.reactions[g].includes(personId) }));
  // A sticker travels as its id in `body`, exactly as the legacy wire carries
  // one (tin-song.ts guiSticker): the screen draws the picture from that id.
  let body = t.body;
  if (t.sticker !== null) body = t.sticker.stickerId;
  return {
    id: t.id,
    context_id: contextId,
    author_id: t.authorId,
    kind: t.deleted ? "deleted" : t.sticker !== null ? "sticker" : t.media?.type === "image" ? "image" : "text",
    body,
    image_url: t.media?.type === "image" ? (anhDaMo[t.media.media.media_id] ?? null) : null,
    card: null,
    created_at: t.at ?? new Date(0).toISOString(),
    cursor: String(t.sequence),
    reactions,
    reply_to: t.replyTo === null ? null : { id: t.replyTo, kind: "text", author_id: null, preview: "" },
    deleted_at: t.deleted ? (t.at ?? new Date(0).toISOString()) : null,
  };
}

/** An own send still on its way, as the screen's pending row. */
export function sangCho(c: Cho, tinTheoId: Record<string, TinV2>): TinChoGui {
  const op = c.r.operation;
  const goc = op.type === "reply" ? tinTheoId[op.reply_to] : undefined;
  return {
    attempt: { key: c.id, at: c.luc },
    kind: op.type === "sticker" ? "sticker" : op.type === "image" ? "image" : "text",
    than: op.type === "sticker" ? op.sticker_id : op.type === "text" || op.type === "reply" ? op.body : "",
    phuDe: op.type === "image" ? op.caption : null,
    traLoi: op.type === "reply" ? { id: op.reply_to, kind: "text", author_id: goc?.authorId ?? null, preview: goc?.body ?? "" } : null,
    trangThai: c.hong ? "that-bai" : "dang-gui",
    loi: c.loi,
    thuLaiDuoc: c.thuLai,
    luc: new Date(c.luc).toISOString(),
  };
}

export type TrangThaiV2 = {
  tin: Tin[];
  dangNap: boolean;
  loi: string | null;
  /** Devices that joined since this phone last looked: said in the room. */
  thietBiMoi: ThietBiMoi[];
  sanSang: boolean;
};

export function useTinNhanV2(contextId: string, personId: string, tat = false) {
  const [phong, setPhong] = useState<SoPhong>(PHONG_TRONG);
  const [trang, setTrang] = useState<Omit<TrangThaiV2, "tin">>({ dangNap: true, loi: null, thietBiMoi: [], sanSang: false });
  const [anhDaMo, setAnhDaMo] = useState<Record<string, string>>({});
  const theHe = useRef(0);
  const { so, cho, khongMo } = useMemo(() => dungPhong(phong.ban), [phong]);

  /** Draws the room's record as it is now. */
  const ve = useCallback(async () => {
    const lan = theHe.current;
    const p = await (await mayCua(personId)).soPhong(contextId);
    if (lan === theHe.current) setPhong(p);
  }, [contextId, personId]);

  const nap = useCallback(async () => {
    const lan = theHe.current;
    try {
      const may = await mayCua(personId);
      await ve();
      await may.nhanWelcome();
      const sanSang = await may.chuanBi(contextId);
      if (sanSang) {
        await may.dongBo(contextId);
        await may.guiLai(contextId);
      }
      if (lan !== theHe.current) return;
      await ve();
      setTrang((cu) => ({ ...cu, dangNap: false, loi: null, sanSang }));
    } catch (error) {
      if (lan !== theHe.current) return;
      setTrang((cu) => ({ ...cu, dangNap: false, loi: error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null) }));
    }
  }, [contextId, personId, ve]);

  useEffect(() => {
    if (tat) return undefined;
    const nghe = (m: ThietBiMoi) => {
      if (m.room === contextId) setTrang((cu) => ({ ...cu, thietBiMoi: [...cu.thietBiMoi, m] }));
    };
    const doi = (room: string) => {
      if (room === contextId) void ve().catch(() => undefined);
    };
    ngheThietBiMoi.add(nghe);
    ngheDoiPhong.add(doi);
    return () => {
      ngheThietBiMoi.delete(nghe);
      ngheDoiPhong.delete(doi);
    };
  }, [contextId, tat, ve]);

  useEffect(() => {
    theHe.current += 1;
    setPhong(PHONG_TRONG);
    setTrang({ dangNap: !tat, loi: null, thietBiMoi: [], sanSang: false });
    if (!tat) void nap();
  }, [nap, tat]);

  useFocusEffect(
    useCallback(() => {
      if (tat) return undefined;
      const id = setInterval(() => void nap(), NHIP_MS);
      return () => clearInterval(id);
    }, [nap, tat]),
  );

  /** The room's message for a logical send, once the record holds it. */
  const daGui = useCallback(
    async (id: string): Promise<TinDaGui | null> => {
      await ve();
      const p = await (await mayCua(personId)).soPhong(contextId);
      const t = dungPhong(p.ban).so.byId[id];
      return t === undefined ? null : { ...sangTin(t, contextId, personId, {}), intent: null, vote: null, intent_error: null };
    },
    [contextId, personId, ve],
  );

  const guiOp = useCallback(
    async (op: Operation): Promise<TinDaGui | null> => {
      const may = await mayCua(personId);
      const ev = await may.gui(contextId, op);
      const id = ev?.envelope?.logical_send_id;
      return id === undefined ? null : daGui(id);
    },
    [contextId, personId, daGui],
  );

  /** Opens a sealed image once: downloads the ciphertext, opens it on the device. */
  const moAnh = useCallback(
    async (media: MediaRef) => {
      if (Object.hasOwn(anhDaMo, media.media_id) || ChatCryptoModule === null) return;
      const may = await mayCua(personId);
      const ciphertext = base64TuByte(await goiNhiPhan(`/v2/chat/media/${contextId}/${media.media_id}?device_id=${await may.thietBi()}`, personId, "GET"));
      const opened = JSON.parse(await ChatCryptoModule.call("open_media", JSON.stringify({ media, ciphertext }))) as { plaintext: string };
      setAnhDaMo((cu) => ({ ...cu, [media.media_id]: `data:${media.mime};base64,${opened.plaintext}` }));
    },
    [anhDaMo, contextId, personId],
  );

  useEffect(() => {
    for (const t of danhSach(so)) if (t.media?.type === "image") void moAnh(t.media.media).catch(() => undefined);
  }, [so, moAnh]);

  return {
    ...trang,
    // Newest first, as the screen's inverted list (and the legacy hook) has it.
    tin: moiTruoc(so).map((t) => sangTin(t, contextId, personId, anhDaMo)),
    /** Own sends not yet on the lane, newest first: in flight, or failed and waiting for the person. */
    hangCho: cho.map((c) => sangCho(c, so.byId)).reverse(),
    /** Envelopes skipped because they will never open. */
    khongMo,
    taiLai: nap,
    gui: (body: string, traLoi: { id: string } | null = null) =>
      guiOp(traLoi === null ? { type: "text", body } : { type: "reply", reply_to: traLoi.id, body }),
    sua: (messageId: string, body: string) => guiOp({ type: "edit", message_id: messageId, body }),
    xoaTin: (messageId: string) => guiOp({ type: "delete", message_id: messageId }),
    doiPhanUng: (messageId: string, kind: LoaiPhanUng) => guiOp({ type: "reaction", message_id: messageId, emoji: KIND_GLYPH[kind] }),
    guiSticker: (stickerId: string) => guiOp({ type: "sticker", pack_id: "nep", sticker_id: stickerId }),
    thuLai: async (id: string): Promise<TinDaGui | null> => {
      const ev = await (await mayCua(personId)).thuLai(contextId, id);
      return ev === null ? null : daGui(id);
    },
    boQua: (id: string) => {
      void mayCua(personId)
        .then((may) => may.boQua(contextId, id))
        .catch(() => undefined);
    },
    /**
     * A photo already shrunk and re-encoded (`nenVaDung`: no EXIF survives):
     * sealed on the device, the ciphertext uploaded, then the reference sent.
     */
    guiAnhTuTep: async (uri: string, caption: string | null): Promise<TinDaGui | null> => {
      const [width, height] = await new Promise<[number, number]>((resolve) =>
        Image.getSize(uri, (w, h) => resolve([w, h]), () => resolve([1, 1])),
      );
      if (ChatCryptoModule === null) throw new ApiError(0, "chat_v2_native_missing", "Bản ứng dụng này chưa có mã hoá đầu cuối.");
      const may = await mayCua(personId);
      const plaintext = await new File(uri).base64();
      const sealed = JSON.parse(await ChatCryptoModule.call("seal_media", JSON.stringify({ media_id: uuid(), mime: "image/jpeg", plaintext }))) as {
        ciphertext: string;
        media: MediaRef;
      };
      const bytes = Uint8Array.from(atob(sealed.ciphertext), (c) => c.charCodeAt(0));
      await goiNhiPhan(`/v2/chat/media/${contextId}/${sealed.media.media_id}?device_id=${await may.thietBi()}`, personId, "PUT", bytes);
      return guiOp({ type: "image", media: sealed.media, caption, width, height });
    },
  };
}

export type Lan =
  | { lan: "dang-xet" }
  /** The lane could not be established: nothing is read or sent until it is. */
  | { lan: "khong-ro"; thuLai: () => void }
  | { lan: "v2" }
  /** Legacy plaintext, with why the room is not encrypted yet (null: the lane is off here). */
  | { lan: "legacy"; lyDo: string | null };

/**
 * Which lane a room is on (ADR-0057 §8.2). A room on v2 stays on v2. A room
 * not yet on v2 opens it once every active member has an enrolled device;
 * until then it stays legacy and says who it waits for. Where this build has
 * no native crypto, or the server's lane is off, the room is legacy -- and a
 * v2 room refuses legacy writes on the server, so nothing falls back.
 */
export function useLanChat(contextId: string, personId: string): Lan {
  const [lan, setLan] = useState<Lan>({ lan: "dang-xet" });
  const [lanThu, setLanThu] = useState(0);
  useEffect(() => {
    let song = true;
    setLan({ lan: "dang-xet" });
    if (!coMaHoa()) {
      setLan({ lan: "legacy", lyDo: null });
      return;
    }
    void (async () => {
      try {
        const thanhVien = (await danhSachThanhVien(contextId, personId)).filter((tv) => tv.state === "active").map((tv) => tv.person_id);
        const may = await mayCua(personId);
        const device = await may.thietBi();
        const r = await apiV2.roster(personId, contextId, device);
        if (r.exists) {
          if (song) setLan({ lan: "v2" });
          return;
        }
        const coThietBi = new Set(r.expected.map((c) => c.actor_id));
        const thieu = thanhVien.filter((id) => !coThietBi.has(id));
        if (thieu.length > 0) {
          if (song) setLan({ lan: "legacy", lyDo: `${thieu.length} người trong nhóm chưa dùng bản ứng dụng có mã hoá đầu cuối.` });
          return;
        }
        await may.chuanBi(contextId);
        if (song) setLan({ lan: "v2" });
      } catch (error) {
        if (!song) return;
        // Legacy only when that is known: the server's lane is off (404).
        // Anything else is not knowing, and not knowing never opens the
        // plaintext path (security review 05/10: fail closed).
        if (error instanceof ApiError && error.status === 404) setLan({ lan: "legacy", lyDo: null });
        else setLan({ lan: "khong-ro", thuLai: () => setLanThu((n) => n + 1) });
      }
    })();
    return () => {
      song = false;
    };
  }, [contextId, personId, lanThu]);
  return lan;
}

/**
 * The chat screen's `chat` object for a room on the v2 lane: the legacy
 * shape, with v2's messages and writers. What v2 has no use for -- older
 * server pages, a pending-send queue, legacy snapshots, read marks -- answers
 * as nothing to do.
 */
export function hopLanV2<L extends { gui: unknown }>(cu: L, v2: ReturnType<typeof useTinNhanV2>): L {
  return {
    ...cu,
    tin: v2.tin,
    dangNap: v2.dangNap,
    dangNapCu: false,
    hetTinCu: true,
    loi: v2.loi,
    loiCu: null,
    loiLoai: v2.loi === null ? null : "tam",
    hangCho: v2.hangCho,
    napCuHon: async () => undefined,
    napMoi: v2.taiLai,
    taiLai: v2.taiLai,
    gui: (body: string, traLoi: { id: string } | null = null) => v2.gui(body, traLoi),
    guiAnhMoi: async () => null,
    guiSticker: async (stickerId: string) => v2.guiSticker(stickerId),
    thuLaiMot: (key: string) => v2.thuLai(key),
    boQua: (key: string) => v2.boQua(key),
    xoaTin: (id: string) => v2.xoaTin(id),
    doiPhanUng: (id: string, kind: LoaiPhanUng) => v2.doiPhanUng(id, kind),
    danhDauHienThi: () => undefined,
    nhanAnhChup: () => undefined,
  } as L;
}

/**
 * The chat screen's `chat` object while the lane is not yet known: nothing
 * shown, nothing sent -- neither lane may act before the room's lane is
 * established (security review 05/10).
 */
export function khoaLan<L extends { gui: unknown }>(cu: L, dangXet: boolean): L {
  const tuChoi = async (): Promise<never> => {
    throw new ApiError(0, "chat_lane_unknown", dangXet ? "Đang kiểm tra mã hoá của phòng. Thử lại sau giây lát." : "Chưa xác định được phòng có mã hoá hay không. Bấm thử lại.");
  };
  return {
    ...cu,
    tin: [],
    dangNap: dangXet,
    dangNapCu: false,
    hetTinCu: true,
    loi: dangXet ? null : "Chưa xác định được phòng có mã hoá đầu cuối hay không.",
    loiCu: null,
    loiLoai: dangXet ? null : "tam",
    hangCho: [],
    napCuHon: async () => undefined,
    gui: tuChoi,
    guiAnhMoi: tuChoi,
    guiSticker: tuChoi,
    thuLaiMot: tuChoi,
    boQua: () => undefined,
    xoaTin: tuChoi,
    doiPhanUng: tuChoi,
    danhDauHienThi: () => undefined,
    nhanAnhChup: () => undefined,
  } as L;
}
