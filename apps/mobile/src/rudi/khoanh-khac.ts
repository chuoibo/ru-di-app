/**
 * The eight moments Nếp performs (ADR-0037 D4, D5).
 *
 * A moment is a thing the PERSON just did or just got -- never a screen
 * opening, never a loop -- and it plays once per event: the event key (the
 * expense id, the plan id, the badge id) is remembered for the session, so a
 * remount, a back-and-forth or a re-render shows the still frame instead of a
 * replay. Reduce Motion shows the still frame from the start. And Nếp is never
 * at an error, a refusal or a conflict (the «Nếp Không Chạm Số» rule and its
 * siblings): a moment asked for on a failed state does not happen at all.
 *
 * Pure: a module-level registry and a decision function, so the node gate can
 * pin the rules; `ui/useKhoanhKhac.ts` is the hook that plays what this says.
 */
import type { TietMucId } from "./art/nep-dien";

export const KHOANH_KHAC_IDS = ["M1", "M2", "M3", "M4", "M5", "M6", "M7", "M8"] as const;
export type KhoanhKhacId = (typeof KHOANH_KHAC_IDS)[number];

export interface KhoanhKhac {
  /** What happened, in the product's words (for the log and the lab). */
  ten: string;
  /** The performances, played one after the other. */
  tietMuc: readonly TietMucId[];
  /** A money moment: the face stays level, the puppet never touches a number. */
  tien: boolean;
}

export const KHOANH_KHAC: Readonly<Record<KhoanhKhacId, KhoanhKhac>> = Object.freeze({
  M1: { ten: "Khay tạo mới mở", tietMuc: ["buoc-vao"], tien: false },
  M2: { ten: "Chụp hoá đơn", tietMuc: ["cam-may"], tien: true },
  M3: { ten: "Ghi sổ xong", tietMuc: ["dong-dau"], tien: true },
  M4: { ten: "Tiền đã về", tietMuc: ["cui-cam-on"], tien: true },
  M5: { ten: "Tạo kèo", tietMuc: ["buoc-di"], tien: false },
  M6: { ten: "Sổ hai người mở", tietMuc: ["keo-tab", "nhay"], tien: false },
  M7: { ten: "Gửi tờ giấy", tietMuc: ["gap-thu"], tien: false },
  M8: { ten: "Huy hiệu mới", tietMuc: ["nang-tem"], tien: false },
});

/** How many event keys a session remembers; the oldest is forgotten first. */
export const TRAN_KHOANH_KHAC = 200;

const daDien = new Set<string>();

/**
 * What a moment does now:
 *   - `dien`  play it (and remember the key);
 *   - `tinh`  show the still frame: already played this session, or Reduce Motion;
 *   - `khong` nothing: the state is not valid (the server has not confirmed it)
 *             or it failed.
 */
export type CachDien = "dien" | "tinh" | "khong";

export function nenDien(vao: { khoa: string; reduced: boolean; hopLe?: boolean; coLoi?: boolean }): CachDien {
  const { khoa, reduced, hopLe = true, coLoi = false } = vao;
  if (coLoi || !hopLe || khoa.trim() === "") return "khong";
  if (daDien.has(khoa)) return "tinh";
  ghiDaDien(khoa);
  return reduced ? "tinh" : "dien";
}

export function ghiDaDien(khoa: string): void {
  daDien.delete(khoa);
  daDien.add(khoa);
  while (daDien.size > TRAN_KHOANH_KHAC) {
    const cuNhat = daDien.values().next().value;
    if (cuNhat === undefined) break;
    daDien.delete(cuNhat);
  }
}

export function daDienRoi(khoa: string): boolean {
  return daDien.has(khoa);
}

/** For tests and the lab's «chạy lại»: forget every played moment. */
export function quenKhoanhKhac(): void {
  daDien.clear();
}

/** The event key of a moment: the moment and the thing it is about. */
export function khoaKhoanhKhac(id: KhoanhKhacId, suKien: string): string {
  return `${id}:${suKien}`;
}
