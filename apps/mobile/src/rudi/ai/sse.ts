/**
 * Reading an AI answer while it is being written.
 *
 * The server streams one invocation as Server-Sent Events
 * (`GET …/ai-invocations/{id}/events`, docs/claude/2026-09-25/thiet-ke-ai/02
 * §5). Expo's global `fetch` exposes the body as a ReadableStream, so this is
 * a plain reader over it: no EventSource polyfill, no dependency.
 *
 * Three rules, each one a refusal:
 *   - an event outside the closed vocabulary is dropped, never guessed at;
 *   - a stream that keeps failing gives up and says so (`khiChuyenSangHoi`),
 *     so the screen falls back to polling the invocation, which is the truth
 *     anyway: the stream only shows the answer early;
 *   - a refused stream (401, 403, 404) is not retried: asking again cannot
 *     change who may read it.
 *
 * Resuming sends the last event id back (`Last-Event-ID`), so a reconnect
 * continues where it stopped instead of replaying the answer.
 */

export const SU_KIEN = [
  "hello",
  "trang_thai",
  "phan",
  "delta",
  "lam_lai",
  "xong",
  "that_bai",
  "huy",
  "thu_hoi",
  "ket_noi_lai",
] as const;
export type LoaiSuKien = (typeof SU_KIEN)[number];

export interface SuKienSSE {
  id: string | null;
  loai: LoaiSuKien;
  data: unknown;
}

/** Events after which nothing more will come for this invocation. */
export const KET_THUC: ReadonlySet<LoaiSuKien> = new Set<LoaiSuKien>(["xong", "that_bai", "huy", "thu_hoi"]);

const LA_SU_KIEN: ReadonlySet<string> = new Set<string>(SU_KIEN);

export interface BoDocSSE {
  /** Feed decoded text; returns the events it completed. */
  doc(chu: string): SuKienSSE[];
  /** The last `retry:` the server sent, if any. */
  retryMs(): number | null;
}

/** An incremental SSE parser (WHATWG HTML §9.2.6), tolerant of any chunking. */
export function taoBoDoc(): BoDocSSE {
  let dem = "";
  let loai = "";
  let data: string[] = [];
  let id: string | null = null;
  let retry: number | null = null;
  const xong = (): SuKienSSE | null => {
    const tenLoai = loai || "message";
    const du = data;
    loai = "";
    data = [];
    if (du.length === 0 || !LA_SU_KIEN.has(tenLoai)) return null;
    let giaTri: unknown;
    try {
      giaTri = JSON.parse(du.join("\n"));
    } catch {
      return null;
    }
    return { id, loai: tenLoai as LoaiSuKien, data: giaTri };
  };
  return {
    doc(chu: string): SuKienSSE[] {
      dem += chu;
      const ra: SuKienSSE[] = [];
      for (;;) {
        const m = /\r\n|\r|\n/.exec(dem);
        if (!m) break;
        // A lone "\r" at the very end may be the first half of "\r\n".
        if (m[0] === "\r" && m.index === dem.length - 1) break;
        const dong = dem.slice(0, m.index);
        dem = dem.slice(m.index + m[0].length);
        if (dong === "") {
          const e = xong();
          if (e) ra.push(e);
          continue;
        }
        if (dong.startsWith(":")) continue;
        const hai = dong.indexOf(":");
        const truong = hai === -1 ? dong : dong.slice(0, hai);
        let giaTri = hai === -1 ? "" : dong.slice(hai + 1);
        if (giaTri.startsWith(" ")) giaTri = giaTri.slice(1);
        if (truong === "event") loai = giaTri;
        else if (truong === "data") data.push(giaTri);
        else if (truong === "id" && !giaTri.includes("\0")) id = giaTri || null;
        else if (truong === "retry" && /^\d+$/.test(giaTri)) retry = Number(giaTri);
      }
      return ra;
    },
    retryMs: () => retry,
  };
}

/** Backoff before reconnect n (0-based): 500 ms doubling to 15 s, ±20 %. */
export function nhipNoiLai(lanThu: number, ngauNhien: () => number = Math.random): number {
  const n = Number.isFinite(lanThu) ? Math.max(0, Math.floor(lanThu)) : 0;
  const goc = Math.min(15_000, 500 * 2 ** Math.min(n, 10));
  return Math.round(goc * (0.8 + 0.4 * ngauNhien()));
}

export type LyDoChuyenSangHoi = "khong-ho-tro" | "loi-lap-lai" | "may-chu-tu-choi";

export interface TuyChonLuong {
  url: string;
  headers: Record<string, string>;
  /** Where to resume from, when the screen already saw part of the answer. */
  sauId?: string | null;
  khiSuKien(e: SuKienSSE): void;
  /** The stream is not usable; poll the invocation instead. */
  khiChuyenSangHoi(lyDo: LyDoChuyenSangHoi): void;
  /** The stream ended for good (a terminal event, or the reader was refused). */
  khiDong?(): void;
  fetchImpl?: typeof fetch;
  hen?: (fn: () => void, ms: number) => unknown;
  boHen?: (h: unknown) => void;
  ngauNhien?: () => number;
}

/** Consecutive failures after which the stream gives way to polling. */
export const SO_LAN_HONG_TOI_DA = 3;

/**
 * Opens and keeps an invocation's stream until it ends, is refused, or `dong`
 * is called. Never throws: every outcome is one of the three callbacks.
 */
export function moLuong(o: TuyChonLuong): { dong(): void } {
  const fetcher = o.fetchImpl ?? fetch;
  const hen = o.hen ?? ((fn: () => void, ms: number) => setTimeout(fn, ms));
  const boHen = o.boHen ?? ((h: unknown) => clearTimeout(h as ReturnType<typeof setTimeout>));
  let sauId = o.sauId ?? null;
  let daDong = false;
  let hong = 0;
  let cho: unknown = null;
  let dieuKhien: AbortController | null = null;

  const ketThuc = () => {
    if (daDong) return;
    daDong = true;
    o.khiDong?.();
  };
  const chuyenSangHoi = (lyDo: LyDoChuyenSangHoi) => {
    if (daDong) return;
    daDong = true;
    o.khiChuyenSangHoi(lyDo);
  };
  const henLai = (ms: number) => {
    if (daDong) return;
    cho = hen(() => void chay(), ms);
  };

  const chay = async (): Promise<void> => {
    if (daDong) return;
    dieuKhien = new AbortController();
    let nhanDuoc = false;
    let noiLaiSau: number | null = null;
    try {
      const headers: Record<string, string> = { ...o.headers, Accept: "text/event-stream" };
      if (sauId) headers["Last-Event-ID"] = sauId;
      const res = await fetcher(o.url, { headers, signal: dieuKhien.signal });
      if (res.status === 401 || res.status === 403 || res.status === 404) {
        ketThuc();
        return;
      }
      if (res.status === 503) {
        chuyenSangHoi("may-chu-tu-choi");
        return;
      }
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const reader = res.body?.getReader?.();
      if (!reader) {
        chuyenSangHoi("khong-ho-tro");
        return;
      }
      const giaiMa = new TextDecoder("utf-8");
      const boDoc = taoBoDoc();
      for (;;) {
        const { done, value } = await reader.read();
        if (daDong) {
          await reader.cancel().catch(() => undefined);
          return;
        }
        const chu = done ? giaiMa.decode() : giaiMa.decode(value, { stream: true });
        for (const e of boDoc.doc(chu)) {
          nhanDuoc = true;
          if (e.id) sauId = e.id;
          if (e.loai === "ket_noi_lai") {
            const sau = (e.data as { sau_ms?: unknown } | null)?.sau_ms;
            noiLaiSau = typeof sau === "number" && sau >= 0 ? sau : 0;
            continue;
          }
          o.khiSuKien(e);
          if (KET_THUC.has(e.loai)) {
            await reader.cancel().catch(() => undefined);
            ketThuc();
            return;
          }
        }
        if (done) break;
        if (noiLaiSau !== null) {
          await reader.cancel().catch(() => undefined);
          break;
        }
      }
    } catch {
      if (daDong) return;
    }
    if (daDong) return;
    if (noiLaiSau !== null) {
      hong = 0;
      henLai(noiLaiSau);
      return;
    }
    hong = nhanDuoc ? 0 : hong + 1;
    if (hong >= SO_LAN_HONG_TOI_DA) {
      chuyenSangHoi("loi-lap-lai");
      return;
    }
    henLai(nhipNoiLai(hong, o.ngauNhien));
  };

  void chay();
  return {
    dong() {
      if (daDong) return;
      daDong = true;
      if (cho !== null) boHen(cho);
      dieuKhien?.abort();
    },
  };
}
