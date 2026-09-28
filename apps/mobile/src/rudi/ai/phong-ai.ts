/**
 * Other members of a group watching a Rủ Đi AI answer being written (slice
 * 12; design 02 §5.3–5.4, contract §4.5). The server sends the room's answers
 * as `ai` frames on the change feed's WebSocket the screen already holds:
 *
 *     {"type":"ai","inv":"…","tin":"…","so_tin":n,"id":"<redis id>","e":"…","d":{…}}
 *
 * Pure, so it runs under bare node (`tests/phong-ai.test.mjs`):
 *
 *   - `docKhungAi` reads one frame or refuses it (closed event list, ids that
 *     look like ids, nothing else trusted);
 *   - `buocPhong` folds a frame into the room's answers, each through the very
 *     state machine the requester's screen runs (`buocTraLoi`): same phases,
 *     same dedupe by id, same Reduce Motion text;
 *   - `donPhong` forgets what nobody should still see: an answer that failed
 *     or was cancelled, one that ended a while ago, one that went quiet.
 *
 * What the room keeps is only what the frames said. A reconnect replays the
 * room from the server's side (every answer still running, its text so far in
 * one piece), so the screen starts from `KHO_PHONG_TRONG` on every new socket
 * (`useChatChanges` calls `moi`) instead of stitching two replays together.
 */
import { buocTraLoi, daKetThuc, TRA_LOI_DAU, type TraLoiSong } from "./tra-loi-song";
import type { LoaiSuKien } from "./sse";

/** One `ai` frame as the screen reads it. */
export interface KhungAi {
  inv: string;
  /** The `@Rủ Đi` message the answer goes under; null for an answer outside the thread. */
  tin: string | null;
  /** How many shared messages the server confirmed the answer reads; 0 when unknown. */
  soTin: number;
  id: string;
  e: LoaiSuKien;
  d: unknown;
}

/** The events a room frame may carry: never hello, thu_hoi or ket_noi_lai. */
const SU_KIEN_PHONG: ReadonlySet<string> = new Set(["trang_thai", "phan", "delta", "lam_lai", "xong", "that_bai", "huy"]);
const LA_ID = /^[0-9a-fA-F-]{8,64}$/;
const LA_ID_LUONG = /^\d{1,20}-\d{1,20}$/;

/** Whether a WebSocket message is an `ai` frame rather than a feed page. */
export function laKhungAi(raw: unknown): boolean {
  return typeof raw === "object" && raw !== null && (raw as { type?: unknown }).type === "ai";
}

/** Reads one frame, or null for anything that is not a room frame of the contract. */
export function docKhungAi(raw: unknown): KhungAi | null {
  if (!laKhungAi(raw)) return null;
  const k = raw as Record<string, unknown>;
  if (typeof k.inv !== "string" || !LA_ID.test(k.inv)) return null;
  if (typeof k.id !== "string" || !LA_ID_LUONG.test(k.id)) return null;
  if (typeof k.e !== "string" || !SU_KIEN_PHONG.has(k.e)) return null;
  const tin = typeof k.tin === "string" && LA_ID.test(k.tin) ? k.tin : null;
  const soTin = typeof k.so_tin === "number" && Number.isInteger(k.so_tin) && k.so_tin > 0 ? k.so_tin : 0;
  return { inv: k.inv, tin, soTin, id: k.id, e: k.e as LoaiSuKien, d: k.d ?? {} };
}

/** One answer the room is watching. */
export interface LuotPhong {
  inv: string;
  tin: string | null;
  soTin: number;
  traLoi: TraLoiSong;
  /** When its first and its last frame arrived (ms). */
  dau: number;
  luc: number;
}

export type KhoPhongAi = Readonly<Record<string, LuotPhong>>;

export const KHO_PHONG_TRONG: KhoPhongAi = Object.freeze({});

/** At most this many answers are watched at once: the server runs three per room. */
export const SO_LUOT_TOI_DA = 6;

/** Folds one frame in. Returns the same object when nothing changed. */
export function buocPhong(kho: KhoPhongAi, k: KhungAi, bayGio: number): KhoPhongAi {
  const cu = kho[k.inv];
  if (!cu && Object.keys(kho).length >= SO_LUOT_TOI_DA) return kho;
  const truoc = cu?.traLoi ?? TRA_LOI_DAU;
  const traLoi = buocTraLoi(truoc, { id: k.id, loai: k.e, data: k.d });
  if (cu && traLoi === truoc) return kho;
  return {
    ...kho,
    [k.inv]: { inv: k.inv, tin: cu?.tin ?? k.tin, soTin: Math.max(cu?.soTin ?? 0, k.soTin), traLoi, dau: cu?.dau ?? bayGio, luc: bayGio },
  };
}

/** An answer with no frame for this long is dropped (design 02 §5.4). */
export const IM_LANG_TOI_DA_MS = 30_000;
/** An answer that ended stays this long for its card to arrive, then goes. */
export const GIU_SAU_XONG_MS = 10_000;

/**
 * Forgets answers nobody should still see: failed or cancelled at once (the
 * requester alone reads the reason, on their own row), ended after
 * `GIU_SAU_XONG_MS`, and any after `IM_LANG_TOI_DA_MS` without a frame.
 */
export function donPhong(kho: KhoPhongAi, bayGio: number): KhoPhongAi {
  let doi = false;
  const giu: Record<string, LuotPhong> = {};
  for (const [inv, l] of Object.entries(kho)) {
    const pha = l.traLoi.pha;
    const bo =
      pha === "loi" || pha === "huy" || pha === "thu_hoi" ||
      (daKetThuc(l.traLoi) && bayGio - l.luc > GIU_SAU_XONG_MS) ||
      bayGio - l.luc > IM_LANG_TOI_DA_MS;
    if (bo) doi = true;
    else giu[inv] = l;
  }
  return doi ? giu : kho;
}

/**
 * The answers another member's thread draws: in the thread (a `tin`), not
 * the viewer's own (their row follows the invocation itself), oldest first.
 */
export function luotChoNguoiXem(kho: KhoPhongAi, cuaToi: (inv: string, tin: string) => boolean): Array<LuotPhong & { tin: string }> {
  return Object.values(kho)
    .filter((l): l is LuotPhong & { tin: string } => l.tin !== null && !cuaToi(l.inv, l.tin))
    .sort((a, b) => a.dau - b.dau || a.inv.localeCompare(b.inv));
}
