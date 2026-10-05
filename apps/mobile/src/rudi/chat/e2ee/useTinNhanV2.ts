/**
 * One room on the chat v2 lane (ADR-0057), shaped like `useTinNhan` so the
 * chat screen draws either lane with the same code.
 *
 * - The native module seals every change before its answer; this hook only
 *   orchestrates (`MayMaHoa`) and draws (`so-tin`).
 * - It polls the lane while focused (the WebSocket stream is the next step);
 *   history starts where this device joined -- earlier messages were never
 *   encrypted to it, and the legacy archive stays readable on its own.
 * - Nothing here ever writes plaintext anywhere: a failure is said, not
 *   routed around.
 */
import AsyncStorage from "@react-native-async-storage/async-storage";
import { File } from "expo-file-system";
import { useFocusEffect } from "expo-router";
import { useCallback, useEffect, useRef, useState } from "react";
import { Platform } from "react-native";

import { ApiError, BASE_URL, thongDiepNguoiDoc, tokenPhienHienTai } from "../../../api";
import { makeIdFactory } from "../../../participants";
import { ChatCryptoModule } from "../../../../modules/rudi-chat-crypto";
import type { LoaiPhanUng, PhanUngTomTat, Tin } from "../tin-song";
import { PHAN_UNG } from "../tin-song";
import { apiV2 } from "./api-v2";
import type { MediaRef, Operation } from "./kieu";
import { MayMaHoa, type ThietBiMoi } from "./may-ma-hoa";
import { SO_TRONG, apDung, danhSach, type SoTin, type TinV2 } from "./so-tin";

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

export function coMaHoa(): boolean {
  return ChatCryptoModule !== null;
}

export async function mayCua(personId: string, onThietBiMoi?: (m: ThietBiMoi) => void): Promise<MayMaHoa> {
  const native = ChatCryptoModule;
  if (native === null) throw new ApiError(0, "chat_v2_native_missing", "Bản ứng dụng này chưa có mã hoá đầu cuối. Cập nhật ứng dụng để nhắn trong phòng này.");
  let co = mayTheoNguoi.get(personId);
  if (co === undefined) {
    co = (async () => {
      const may = new MayMaHoa({ actorId: personId, crypto: native, api: apiV2, kho, uuid, label: Platform.OS === "ios" ? "iPhone" : "Android", onThietBiMoi });
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
  return {
    id: t.id,
    context_id: contextId,
    author_id: t.authorId,
    kind: t.deleted ? "deleted" : t.sticker !== null ? "sticker" : t.media?.type === "image" ? "image" : "text",
    body: t.sticker !== null ? t.sticker.stickerId : t.body,
    image_url: t.media?.type === "image" ? (anhDaMo[t.media.media.media_id] ?? null) : null,
    card: null,
    created_at: new Date().toISOString(),
    cursor: String(t.sequence),
    reactions,
    reply_to: t.replyTo === null ? null : { id: t.replyTo, kind: "text", author_id: null, preview: "" },
    deleted_at: t.deleted ? new Date().toISOString() : null,
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

export function useTinNhanV2(contextId: string, personId: string) {
  const [so, setSo] = useState<SoTin>(SO_TRONG);
  const soRef = useRef<SoTin>(SO_TRONG);
  const [trang, setTrang] = useState<Omit<TrangThaiV2, "tin">>({ dangNap: true, loi: null, thietBiMoi: [], sanSang: false });
  const [anhDaMo, setAnhDaMo] = useState<Record<string, string>>({});
  const theHe = useRef(0);

  const nap = useCallback(async () => {
    const lan = theHe.current;
    try {
      const may = await mayCua(personId, (m) => {
        if (m.room === contextId) setTrang((cu) => ({ ...cu, thietBiMoi: [...cu.thietBiMoi, m] }));
      });
      await may.nhanWelcome();
      const sanSang = await may.chuanBi(contextId);
      let moi = soRef.current;
      await may.dongBo(contextId, (t) => {
        moi = apDung(moi, t.received, t.sequence);
      });
      if (lan !== theHe.current) return;
      soRef.current = moi;
      setSo(moi);
      setTrang((cu) => ({ ...cu, dangNap: false, loi: null, sanSang }));
    } catch (error) {
      if (lan !== theHe.current) return;
      setTrang((cu) => ({ ...cu, dangNap: false, loi: error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null) }));
    }
  }, [contextId, personId]);

  useEffect(() => {
    theHe.current += 1;
    soRef.current = SO_TRONG;
    setSo(SO_TRONG);
    setTrang({ dangNap: true, loi: null, thietBiMoi: [], sanSang: false });
    void nap();
  }, [nap]);

  useFocusEffect(
    useCallback(() => {
      const id = setInterval(() => void nap(), NHIP_MS);
      return () => clearInterval(id);
    }, [nap]),
  );

  const guiOp = useCallback(
    async (op: Operation) => {
      const may = await mayCua(personId);
      await may.gui(contextId, op);
      await nap();
    },
    [contextId, personId, nap],
  );

  /** Opens a sealed image once: downloads the ciphertext, opens it on the device. */
  const moAnh = useCallback(
    async (media: MediaRef) => {
      if (Object.hasOwn(anhDaMo, media.media_id) || ChatCryptoModule === null) return;
      const may = await mayCua(personId);
      const res = await fetch(`${BASE_URL}/v2/chat/media/${contextId}/${media.media_id}?device_id=${await may.thietBi()}`, {
        headers: { Authorization: `Bearer ${tokenPhienHienTai() ?? ""}` },
      });
      if (!res.ok) return;
      const ciphertext = base64TuByte(new Uint8Array(await res.arrayBuffer()));
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
    tin: danhSach(so).map((t) => sangTin(t, contextId, personId, anhDaMo)),
    taiLai: nap,
    gui: (body: string, traLoi: { id: string } | null = null) =>
      guiOp(traLoi === null ? { type: "text", body } : { type: "reply", reply_to: traLoi.id, body }),
    sua: (messageId: string, body: string) => guiOp({ type: "edit", message_id: messageId, body }),
    xoaTin: (messageId: string) => guiOp({ type: "delete", message_id: messageId }),
    doiPhanUng: (messageId: string, kind: LoaiPhanUng) => guiOp({ type: "reaction", message_id: messageId, emoji: KIND_GLYPH[kind] }),
    guiSticker: (stickerId: string) => guiOp({ type: "sticker", pack_id: "nep", sticker_id: stickerId }),
    /**
     * A photo already shrunk and re-encoded (`nenVaDung`: no EXIF survives):
     * sealed on the device, the ciphertext uploaded, then the reference sent.
     */
    guiAnhTuTep: async (uri: string, caption: string | null, width: number, height: number) => {
      if (ChatCryptoModule === null) throw new ApiError(0, "chat_v2_native_missing", "Bản ứng dụng này chưa có mã hoá đầu cuối.");
      const may = await mayCua(personId);
      const plaintext = await new File(uri).base64();
      const sealed = JSON.parse(await ChatCryptoModule.call("seal_media", JSON.stringify({ media_id: uuid(), mime: "image/jpeg", plaintext }))) as {
        ciphertext: string;
        media: MediaRef;
      };
      const bytes = Uint8Array.from(atob(sealed.ciphertext), (c) => c.charCodeAt(0));
      const res = await fetch(`${BASE_URL}/v2/chat/media/${contextId}/${sealed.media.media_id}?device_id=${await may.thietBi()}`, {
        method: "PUT",
        headers: { Authorization: `Bearer ${tokenPhienHienTai() ?? ""}`, "Content-Type": "application/octet-stream" },
        body: bytes,
      });
      if (!res.ok) throw new ApiError(res.status, "chat_v2_media_upload", "Chưa tải được ảnh lên. Thử lại.");
      await guiOp({ type: "image", media: sealed.media, caption, width, height });
    },
  };
}
