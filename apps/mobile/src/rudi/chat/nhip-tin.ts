/**
 * The rhythm of a thread: when a time is printed, which bubbles read as one
 * voice, and how a failed action on one message is said under it.
 *
 * Messenger's conventions, which the requester measured the thread against
 * (QA UI-064, 27/09): a time is a band across the thread where the talk
 * paused, not a caption under every run; consecutive bubbles of one person are
 * one shape, the avatar at the foot of the last; the time of one message is
 * there when you touch it (the message menu). Pure, so
 * `tests/rudi-chat-nhip-tin.test.mjs` holds the rules without a device.
 */
import { ApiError } from "../../api";
import { laTuChoiVinhVien } from "../../cau-loi-theo-ma";
import { gioPhut, type HangHienThi, type Tin } from "./tin-song";

/**
 * A pause this long starts a new stretch with its own time band. Fifteen
 * minutes: a reply after a coffee is the same conversation, one after lunch
 * is not -- and five (the old run rule) printed «13:27» five times on a screen
 * of eleven messages sent within the same minute.
 */
export const NGHI_MS = 15 * 60_000;

/** The calendar day on this phone, not in UTC: a message at 06:30 in Hà Nội is today's. */
export function ngayDiaPhuong(d: Date): string {
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

/** «Hôm nay», «Hôm qua», or dd/MM, for a local `yyyy-mm-dd`. */
export function nhanNgayDiaPhuong(ngay: string, homNay: Date): string {
  if (ngay === ngayDiaPhuong(homNay)) return "Hôm nay";
  const qua = new Date(homNay.getFullYear(), homNay.getMonth(), homNay.getDate() - 1);
  if (ngay === ngayDiaPhuong(qua)) return "Hôm qua";
  const [, mm, dd] = ngay.split("-");
  return `${dd}/${mm}`;
}

/**
 * Newest-first messages into the rows of an inverted list, with ONE band above
 * every stretch: «Hôm nay · 13:27» where a day starts, «15:40» where the talk
 * resumed after `nghiMs`. In list order the band follows the oldest message of
 * its stretch, so it is drawn above it; its key is that message's, which stays
 * put while newer messages arrive.
 */
export function nhomTheoQuang(tin: readonly Tin[], homNay: Date = new Date(), nghiMs: number = NGHI_MS): HangHienThi[] {
  const ra: HangHienThi[] = [];
  for (let i = 0; i < tin.length; i += 1) {
    const t = tin[i];
    ra.push({ loai: "tin", tin: t });
    const cu = tin[i + 1];
    const ngay = ngayDiaPhuong(new Date(t.created_at));
    if (cu === undefined || ngayDiaPhuong(new Date(cu.created_at)) !== ngay) {
      ra.push({ loai: "ngay", nhan: `${nhanNgayDiaPhuong(ngay, homNay)} · ${gioPhut(t.created_at)}`, key: `quang-${t.id}` });
    } else if (Date.parse(t.created_at) - Date.parse(cu.created_at) > nghiMs) {
      ra.push({ loai: "ngay", nhan: gioPhut(t.created_at), key: `quang-${t.id}` });
    }
  }
  return ra;
}

/**
 * Two neighbouring rows are one voice: both messages by the same person, with
 * no band between them. A model card or a poll is an object, never part of a
 * run of bubbles.
 */
export function cungGiong(a: HangHienThi | undefined, b: HangHienThi | undefined): boolean {
  return (
    a !== undefined && b !== undefined && a.loai === "tin" && b.loai === "tin" &&
    a.tin.kind !== "ai_card" && b.tin.kind !== "ai_card" &&
    a.tin.author_id !== null && a.tin.author_id === b.tin.author_id
  );
}

/** Where a bubble sits in its run: alone, first (oldest), in between, or last (newest). */
export type ViTriCum = "mot" | "dau" | "giua" | "cuoi";

/** For the inverted list's `index`: `index + 1` is the older neighbour, `index - 1` the newer. */
export function viTriTrongCum(hang: readonly HangHienThi[], index: number): ViTriCum {
  const voiCu = cungGiong(hang[index], hang[index + 1]);
  const voiMoi = cungGiong(hang[index], hang[index - 1]);
  if (voiCu && voiMoi) return "giua";
  if (voiCu) return "cuoi";
  if (voiMoi) return "dau";
  return "mot";
}

export type GocBong = {
  borderTopLeftRadius: number;
  borderTopRightRadius: number;
  borderBottomLeftRadius: number;
  borderBottomRightRadius: number;
};

/**
 * The corners of one bubble in its run. The outer side stays round; on the
 * side the run hangs from (left for others, right for your own) the corners
 * that touch a neighbour tighten, so the run reads as one shape. The last
 * bubble of someone else's run keeps its foot round: the avatar sits there.
 */
export function gocBong(viTri: ViTriCum, cuaToi: boolean, lon = 18, nho = 6): GocBong {
  const tren = viTri === "giua" || viTri === "cuoi" ? nho : lon;
  const duoi = viTri === "dau" || viTri === "giua" ? nho : lon;
  return cuaToi
    ? { borderTopLeftRadius: lon, borderBottomLeftRadius: lon, borderTopRightRadius: tren, borderBottomRightRadius: duoi }
    : { borderTopRightRadius: lon, borderBottomRightRadius: lon, borderTopLeftRadius: tren, borderBottomLeftRadius: duoi };
}

/**
 * What to say under a message when an action on it failed: the action named,
 * the cause in a clause, and whether pressing again can help. Never the
 * general «việc này» sentence, and never in the model's voice: this is the
 * app talking (QA UI-069).
 *
 * @param viec the action in the infinitive, «thả cảm xúc», «xoá tin này».
 */
export function cauLoiThaoTac(viec: string, error: unknown): { cau: string; thuLai: boolean } {
  if (!(error instanceof ApiError) || error.status === 0) {
    return { cau: `Chưa ${viec}: máy chưa nối được Rủ Đi. Kiểm tra mạng rồi thử lại.`, thuLai: true };
  }
  if (error.status >= 500) {
    return { cau: `Chưa ${viec}: Rủ Đi đang trục trặc. Chưa có gì thay đổi.`, thuLai: true };
  }
  // An ended session (401) is not helped by pressing again either: the way on
  // is signing in, which the screen offers itself.
  return { cau: `Chưa ${viec}. ${error.message}`, thuLai: error.status !== 401 && !laTuChoiVinhVien(error.status, error.code) };
}
