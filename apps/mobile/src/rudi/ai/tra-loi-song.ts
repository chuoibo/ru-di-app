/**
 * One AI answer while it is being written, as the screen shows it
 * (contract docs/architecture/03-ai-engine-hop-dong.md §4.1, slice 11).
 *
 * Two halves, both pure so they run under bare node
 * (`tests/tra-loi-song.test.mjs`):
 *
 *   - `buocTraLoi` folds one stream event into the state the screen draws:
 *     «thinking» with the status code, the text so far, the sealed end, a
 *     failure code, a cancel, or a revoked read;
 *   - `theoDoiTraLoi` keeps one invocation followed until it ends: the stream
 *     first, and the invocation read (polling) when the stream is not usable.
 *     Polling is the truth anyway: the stream only shows the answer early.
 *
 * What it refuses, each on purpose:
 *   - an event already seen (same or older Redis id: a reconnect that replays)
 *     and a delta for a part before the newest one (the writer keeps parts in
 *     order, so an older part arriving late is a replay, not new text);
 *   - anything after the end: the end is final, a late delta cannot reopen it;
 *   - words of its own: codes stay codes here, and each screen turns them into
 *     the sentences it already has (Nếp: `nep/hoi.ts`, group:
 *     `chat/ai-invocations.ts`). This file is shared by both and imports
 *     neither, so the group answer never borrows Nếp's voice.
 *
 * `p` is the answer PART, not a sequence number (§4.1: Nếp only has part 0).
 * Order and duplicates are therefore decided by the event id, and `p` only
 * says which part a piece of text belongs to.
 */
import { moLuong, type LyDoChuyenSangHoi, type SuKienSSE } from "./sse";

export type PhaTraLoi = "dang_nghi" | "dang_viet" | "xong" | "loi" | "huy" | "thu_hoi";

export interface KetThucTraLoi {
  /** Nếp: the sealed answer, equal to the deltas joined. Null when the end did not carry it. */
  text: string | null;
  chips: string[];
  nguon: string[];
  /** Group: the published `ai_card` message. */
  messageId: string | null;
}

export interface TraLoiSong {
  pha: PhaTraLoi;
  /** The last `trang_thai{cau}` code, for the «thinking» sentence. */
  trangThai: string | null;
  /** Text by part, in part order. */
  phan: string[];
  /** Highest part a delta has written to; -1 before the first delta. */
  phanCuoi: number;
  /** Last Redis id folded in: the dedupe mark and the resume position. */
  idCuoi: string | null;
  ketThuc: KetThucTraLoi | null;
  /** `that_bai{code}`, or the polled row's code. */
  ma: string | null;
  /** Polling took over from the stream (for the record; the screen looks the same). */
  hoiThay: LyDoChuyenSangHoi | "luong-dong" | null;
}

export const TRA_LOI_DAU: TraLoiSong = Object.freeze({
  pha: "dang_nghi",
  trangThai: null,
  phan: [],
  phanCuoi: -1,
  idCuoi: null,
  ketThuc: null,
  ma: null,
  hoiThay: null,
}) as TraLoiSong;

/** Parts an answer can have; anything past this is not an answer we draw. */
export const SO_PHAN_TOI_DA = 8;

/** Whether nothing more will change this answer. */
export function daKetThuc(s: TraLoiSong): boolean {
  return s.pha === "xong" || s.pha === "loi" || s.pha === "huy" || s.pha === "thu_hoi";
}

/**
 * Redis stream ids are `ms-seq`. Returns true when `a` is strictly after `b`.
 * An id that does not parse compares as «not after», so it is dropped rather
 * than trusted to be new.
 */
export function idSau(a: string, b: string): boolean {
  const pa = /^(\d+)-(\d+)$/.exec(a);
  const pb = /^(\d+)-(\d+)$/.exec(b);
  if (!pa || !pb) return false;
  const c = soSanhSo(pa[1], pb[1]);
  return c !== 0 ? c > 0 : soSanhSo(pa[2], pb[2]) > 0;
}

/** Compares two decimal strings of any length without losing digits to a float. */
function soSanhSo(a: string, b: string): number {
  const x = a.replace(/^0+(?=\d)/, "");
  const y = b.replace(/^0+(?=\d)/, "");
  if (x.length !== y.length) return x.length - y.length;
  return x < y ? -1 : x > y ? 1 : 0;
}

const laChuoi = (v: unknown): v is string => typeof v === "string";
const dsChuoi = (v: unknown): string[] => (Array.isArray(v) ? v.filter(laChuoi) : []);

/** The text so far, parts joined by a blank line (group answers can have several). */
export function chuDaViet(s: TraLoiSong): string {
  return s.phan.filter((t) => t !== "").join("\n\n");
}

/** Folds one event into the answer. Returns the same object when nothing changed. */
export function buocTraLoi(s: TraLoiSong, e: SuKienSSE): TraLoiSong {
  if (daKetThuc(s)) return s;
  if (e.id !== null && s.idCuoi !== null && !idSau(e.id, s.idCuoi)) return s;
  const moi = e.id !== null ? { idCuoi: e.id } : {};
  const data = (e.data ?? {}) as Record<string, unknown>;
  switch (e.loai) {
    case "hello":
      return s;
    case "trang_thai": {
      const cau = laChuoi(data.cau) ? data.cau : null;
      // A status after text started changes nothing on screen: the words stay.
      if (s.pha !== "dang_nghi") return { ...s, ...moi };
      return { ...s, ...moi, trangThai: cau };
    }
    case "lam_lai":
      // Back to thinking; whatever showed is dropped (§4.1: in practice nothing).
      return { ...s, ...moi, pha: "dang_nghi", phan: [], phanCuoi: -1 };
    case "delta": {
      const p = data.p;
      const text = data.text;
      if (typeof p !== "number" || !Number.isInteger(p) || p < 0 || p >= SO_PHAN_TOI_DA || !laChuoi(text)) {
        return { ...s, ...moi };
      }
      if (p < s.phanCuoi) return { ...s, ...moi };
      const phan = s.phan.slice();
      while (phan.length <= p) phan.push("");
      phan[p] += text;
      return { ...s, ...moi, pha: "dang_viet", phan, phanCuoi: p };
    }
    case "xong": {
      const ketThuc: KetThucTraLoi = {
        text: laChuoi(data.text) ? data.text : null,
        chips: dsChuoi(data.chips),
        nguon: dsChuoi(data.nguon),
        messageId: laChuoi(data.message_id) ? data.message_id : null,
      };
      // The sealed text replaces what was streamed (it is the same words).
      const phan = ketThuc.text !== null ? [ketThuc.text] : s.phan;
      return { ...s, ...moi, pha: "xong", phan, phanCuoi: Math.max(0, phan.length - 1), ketThuc };
    }
    case "that_bai":
      // What showed stays (only `worker_interrupted` after release has any),
      // and the screen adds the sentence for the code.
      return { ...s, ...moi, pha: "loi", ma: laChuoi(data.code) ? data.code : null };
    case "huy":
      return { ...s, ...moi, pha: "huy" };
    case "thu_hoi":
      // The read was taken away: hide what showed.
      return { ...s, ...moi, pha: "thu_hoi", phan: [], phanCuoi: -1 };
    default:
      // `phan` (not sent in this slice) and anything else: position only.
      return { ...s, ...moi };
  }
}

/** One read of the invocation row, the polling side's view of the same answer. */
export interface DocLoiGoi {
  status: "queued" | "running" | "succeeded" | "failed" | "cancelled";
  code: string | null;
  /** Nếp: the sealed answer. */
  text?: string | null;
  /** Group: the published card. */
  message_id?: string | null;
}

/** Folds a polled row in. Only an ended row changes anything. */
export function buocTuHang(s: TraLoiSong, hang: DocLoiGoi): TraLoiSong {
  if (daKetThuc(s)) return s;
  if (hang.status === "succeeded") {
    return buocTraLoi(s, {
      id: null,
      loai: "xong",
      data: { text: hang.text ?? null, message_id: hang.message_id ?? null, chips: [], nguon: [] },
    });
  }
  if (hang.status === "failed") return buocTraLoi(s, { id: null, loai: "that_bai", data: { code: hang.code } });
  if (hang.status === "cancelled") return buocTraLoi(s, { id: null, loai: "huy", data: {} });
  return s;
}

/**
 * The text to draw. Under Reduce Motion the words do not jump in 16-rune
 * steps: they arrive a sentence at a time (up to the last full stop, question
 * mark, exclamation, ellipsis or line end), and the whole of it at the end.
 * The answer still appears while it is written; it only stops flickering.
 */
export function chuHienThi(s: TraLoiSong, giamChuyenDong: boolean): string {
  const chu = chuDaViet(s);
  if (!giamChuyenDong || s.pha !== "dang_viet") return chu;
  let cat = -1;
  const mau = /[.!?…](?=\s|$)|\n/g;
  for (let m = mau.exec(chu); m; m = mau.exec(chu)) cat = m.index + m[0].length;
  return cat < 0 ? "" : chu.slice(0, cat).trimEnd();
}

/* ------------------------------------------------------------------ follow */

export interface TuyChonTheoDoi {
  /** The SSE URL, or null when this platform cannot stream a fetch body. */
  url: string | null;
  headers: Record<string, string>;
  viTriQua?: "header" | "query";
  /** Consecutive empty stream failures before polling takes over. The owner's rule: two. */
  hongToiDa?: number;
  /** Reads the invocation row. Throws on a missed read; the next tick reads again. */
  hoi(): Promise<DocLoiGoi | null>;
  /** Delay before poll n (0-based). */
  nhipHoi(n: number): number;
  /** After this long without an end, give up reading (the screen says so). */
  choToiDaMs: number;
  khiDoi(s: TraLoiSong): void;
  /** Polling ran out of time without an end. */
  khiHetGio?(): void;
  /** Whether reading now is worth it (the app is in the foreground). */
  duocDoc?(): boolean;
  fetchImpl?: typeof fetch;
  hen?: (fn: () => void, ms: number) => unknown;
  boHen?: (h: unknown) => void;
  bayGio?: () => number;
  ngauNhien?: () => number;
}

export const HONG_TOI_DA_TRA_LOI = 2;

/**
 * Follows one invocation until it ends or `dong` is called. Never throws;
 * every outcome reaches `khiDoi` (or `khiHetGio`).
 */
export function theoDoiTraLoi(o: TuyChonTheoDoi): { dong(): void; trangThai(): TraLoiSong } {
  const hen = o.hen ?? ((fn: () => void, ms: number) => setTimeout(fn, ms));
  const boHen = o.boHen ?? ((h: unknown) => clearTimeout(h as ReturnType<typeof setTimeout>));
  const bayGio = o.bayGio ?? Date.now;
  const batDau = bayGio();
  let s = TRA_LOI_DAU;
  let daDong = false;
  let luong: { dong(): void } | null = null;
  let cho: unknown = null;
  let dangHoi = false;

  const doi = (moi: TraLoiSong) => {
    if (daDong || moi === s) return;
    s = moi;
    o.khiDoi(s);
    if (daKetThuc(s)) dung();
  };
  const dung = () => {
    daDong = true;
    if (cho !== null) boHen(cho);
    cho = null;
    luong?.dong();
    luong = null;
  };

  const hoiLan = (n: number) => {
    if (daDong) return;
    cho = hen(() => void docMot(n), o.nhipHoi(n));
  };
  const docMot = async (n: number) => {
    cho = null;
    if (daDong || dangHoi) return;
    if (bayGio() - batDau > o.choToiDaMs) {
      dung();
      o.khiHetGio?.();
      return;
    }
    if (o.duocDoc?.() ?? true) {
      dangHoi = true;
      try {
        const hang = await o.hoi();
        if (hang) doi(buocTuHang(s, hang));
      } catch {
        // One missed read is not a failed answer: read again next tick.
      } finally {
        dangHoi = false;
      }
    }
    hoiLan(n + 1);
  };
  const sangHoi = (lyDo: TraLoiSong["hoiThay"]) => {
    if (daDong) return;
    luong = null;
    doi({ ...s, hoiThay: lyDo });
    hoiLan(0);
  };

  if (o.url === null) {
    sangHoi("khong-ho-tro");
  } else {
    luong = moLuong({
      url: o.url,
      headers: o.headers,
      viTriQua: o.viTriQua,
      hongToiDa: o.hongToiDa ?? HONG_TOI_DA_TRA_LOI,
      khiSuKien: (e) => doi(buocTraLoi(s, e)),
      khiChuyenSangHoi: (lyDo) => sangHoi(lyDo),
      // The stream ended without an end we saw (a 401/403/404): the row
      // still says how the answer ended, so read it.
      khiDong: () => {
        if (!daKetThuc(s)) sangHoi("luong-dong");
      },
      fetchImpl: o.fetchImpl,
      hen: o.hen,
      boHen: o.boHen,
      ngauNhien: o.ngauNhien,
    });
  }
  return { dong: dung, trangThai: () => s };
}
