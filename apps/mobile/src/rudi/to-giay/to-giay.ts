/**
 * The client's model of one sheet of paper («tờ giấy») in the two-person
 * notebook, and the pure rules the screens read it through.
 *
 * Shapes mirror the wire contract of ADR-0027 (`GET /papers/{pid}`): a paper
 * carries its immutable versions, and every version says who sent it and how
 * each of the two people answered it. Nothing here talks to the server, keeps
 * state, or reads the clock: `het_han` is a rule the SERVER applies when it
 * reads (`hieu_luc(now)`), so the state a screen receives is already final for
 * that read, and the client never computes expiry (spec §3.3 rule 6, plan
 * Phase 2a). The fixture build feeds these functions hand-written papers.
 *
 * Why the rules live here and not in the screens: spec §3.3 lists seven rules
 * that «không được nhập nhằng», and ADR-0027 adds two (withdraw only unseen
 * and unanswered; a sheet Nếp sent needs BOTH to agree). Thirteen surfaces
 * read them. One module, one test file (`tests/to-giay-luat.test.mjs`).
 */

/** The 13 states of spec §3.1, spelled the way the server spells them. */
export const TRANG_THAI_TO = [
  "nhap",
  "da_gui",
  "da_xem",
  "de_nghi_sua",
  "dong_y",
  "chot",
  "da_di",
  "da_giu",
  "nghi_tuan",
  "het_han",
  "rut",
  "bo",
  "huy",
] as const;
export type TrangThaiTo = (typeof TRANG_THAI_TO)[number];

/** States a sheet never leaves. `da_giu` is the good ending; the rest are the week's closings. */
export const TRANG_THAI_CUOI: readonly TrangThaiTo[] = ["nghi_tuan", "het_han", "rut", "bo", "huy", "da_giu"];

/** States in which the sheet is still a proposal: nothing is planned yet. */
export const TRANG_THAI_MO: readonly TrangThaiTo[] = ["nhap", "da_gui", "da_xem", "de_nghi_sua", "dong_y"];

export type TacGia = "human" | "nep";
export type LoaiPhanHoi = "dong_y" | "de_nghi_sua";

/**
 * One stop of the outing. `place_id` is null until a real place is chosen,
 * and `can_kiem` stays true for anything Nếp drafted: spec §8 says a draft
 * says «chưa biết», never «chắc hợp».
 */
export interface Chang {
  gio: string;
  viec: string;
  place_id: string | null;
  can_kiem: boolean;
}

/** The letter's content: a day, one main stop, an optional next stop (spec §5.2). */
export interface NoiDungTo {
  ngay: string;
  chang: readonly Chang[];
}

/** One immutable version, seen from the reader's side of the notebook. */
export interface PhienBanTo {
  version: number;
  content: NoiDungTo;
  ly_do: string | null;
  sent_at: string | null;
  /** Who pressed send. `null` while still a draft. */
  sent_by: string | null;
  author_type: TacGia;
  /** How I answered THIS version; `null` when I have not. */
  my_response: LoaiPhanHoi | null;
  /** Whether the other person agreed to THIS version. */
  their_agreed: boolean;
  /** Shown to the sender only: when the recipient first opened it (spec §7.5). */
  viewed_by_recipient_at: string | null;
}

export interface DongGiu {
  id: string;
  line: string;
  created_at: string;
}

export interface ToGiay {
  id: string;
  state: TrangThaiTo;
  /** The current version number; `versions` holds every one that was sent. */
  version: number;
  author_type: TacGia;
  sent_by: string | null;
  versions: readonly PhienBanTo[];
  outing_id: string | null;
  keeps: readonly DongGiu[];
}

export type Ai = "toi" | "nguoi_kia";

export function phienBan(to: ToGiay, v: number = to.version): PhienBanTo | undefined {
  return to.versions.find((x) => x.version === v);
}

export function phienBanTruoc(to: ToGiay, v: number = to.version): PhienBanTo | undefined {
  return to.versions.filter((x) => x.version < v).sort((a, b) => b.version - a.version)[0];
}

/**
 * Whether `ai` has agreed to version `v`.
 *
 * Spec §3.4: a human who presses send has agreed to what they sent -- so the
 * sender of a human-authored version counts without a separate response. A
 * version Nếp sent has NO agreement built in: both people must answer it
 * (ADR-0027). Agreement is per version and never carries over (§3.3 rule 3).
 */
export function daDongY(to: ToGiay, ai: Ai, toiId: string, v: number = to.version): boolean {
  const pb = phienBan(to, v);
  if (!pb) return false;
  const nguoiGui = pb.author_type === "human" ? pb.sent_by : null;
  if (ai === "toi") return nguoiGui === toiId || pb.my_response === "dong_y";
  return (nguoiGui !== null && nguoiGui !== toiId) || pb.their_agreed;
}

/** Both agreed to the SAME current version, and the sheet is still open. */
export function coTheChot(to: ToGiay, toiId: string): boolean {
  if (!TRANG_THAI_MO.includes(to.state)) return false;
  return daDongY(to, "toi", toiId) && daDongY(to, "nguoi_kia", toiId);
}

/** Silence is not consent (§3.3 rule 2): only `chot` plans anything, and only agreement reaches it. */
export function laKeHoach(to: ToGiay): boolean {
  return to.state === "chot" || to.state === "da_di" || to.state === "da_giu";
}

/**
 * ADR-0027: the sender may withdraw only while the sheet is `da_gui`, the
 * recipient has not opened it, and nobody has answered. After a view the way
 * out is a new version, not a withdrawal.
 */
export function coTheRut(to: ToGiay, actorId: string): boolean {
  if (to.state !== "da_gui") return false;
  const pb = phienBan(to);
  if (!pb || pb.author_type !== "human" || pb.sent_by !== actorId) return false;
  return pb.viewed_by_recipient_at === null && !pb.their_agreed && pb.my_response === null;
}

/** «Tuần này nghỉ» is available right up to `chot` (spec §3.3 table, §6.3). */
export function coTheNghiTuan(to: ToGiay): boolean {
  return TRANG_THAI_MO.includes(to.state);
}

/** The current version was sent by someone else (or by Nếp), so I may answer it. */
function laNguoiNhan(to: ToGiay, toiId: string): boolean {
  const pb = phienBan(to);
  if (!pb) return false;
  return pb.author_type === "nep" || pb.sent_by !== toiId;
}

export function coTheDongY(to: ToGiay, toiId: string): boolean {
  if (!["da_gui", "da_xem", "dong_y"].includes(to.state)) return false;
  return laNguoiNhan(to, toiId) && !daDongY(to, "toi", toiId);
}

export function coTheDeNghiSua(to: ToGiay, toiId: string): boolean {
  if (!["da_gui", "da_xem", "dong_y"].includes(to.state)) return false;
  return laNguoiNhan(to, toiId) && !daDongY(to, "toi", toiId);
}

/**
 * What changed in the CONTENT between two versions, as lines a person can
 * read (§3.3 rule 3: «hiện cái gì đã đổi»). Empty when nothing did.
 */
export function khacGi(v: PhienBanTo, vTruoc: PhienBanTo | undefined): string[] {
  if (!vTruoc) return [];
  const ra: string[] = [];
  if (v.content.ngay !== vTruoc.content.ngay) ra.push(`Ngày: ${vTruoc.content.ngay} → ${v.content.ngay}`);
  const n = Math.max(v.content.chang.length, vTruoc.content.chang.length);
  for (let i = 0; i < n; i++) {
    const ten = i === 0 ? "Chỗ chính" : "Đi tiếp";
    const a = vTruoc.content.chang[i], b = v.content.chang[i];
    if (a && !b) ra.push(`Bỏ chặng ${ten.toLowerCase()}: ${a.gio} ${a.viec}`);
    else if (!a && b) ra.push(`Thêm chặng ${ten.toLowerCase()}: ${b.gio} ${b.viec}`);
    else if (a && b) {
      if (a.gio !== b.gio) ra.push(`Giờ ${ten.toLowerCase()}: ${a.gio} → ${b.gio}`);
      if (a.viec !== b.viec) ra.push(`Việc ${ten.toLowerCase()}: ${a.viec} → ${b.viec}`);
    }
  }
  // The reason is not a diff line: the sheet already prints the current
  // version's «Vì: …» under itself, and repeating it here read as two reasons.
  return ra;
}

/**
 * One sentence about where the sheet stands, from my side. Read aloud by the
 * accessibility label of the open sheet and printed under it, so it names the
 * state in words, never by colour alone (spec §12.1).
 */
export function cauTrangThai(to: ToGiay, toiId: string): string {
  const pb = phienBan(to);
  const toiGui = pb?.author_type === "human" && pb.sent_by === toiId;
  const nep = pb?.author_type === "nep";
  switch (to.state) {
    case "nhap":
      return "Bản phác, chỉ bạn thấy. Gửi đi thì người ấy mới nhận.";
    case "da_gui":
      if (nep) return "Nếp gửi hộ vì chưa ai mở lời. Cần cả hai cùng ừ.";
      return toiGui ? "Đã gửi, chờ trả lời. Người ấy chưa xem." : "Người ấy vừa gửi. Bạn ừ, hay đề nghị sửa?";
    case "da_xem":
      return toiGui ? "Người ấy đã xem, chưa trả lời." : "Bạn đã mở. Ừ, hay đề nghị sửa?";
    case "de_nghi_sua":
      return "Có đề nghị sửa. Phiên bản mới đang chờ.";
    case "dong_y":
      return daDongY(to, "toi", toiId) ? "Bạn đã ừ. Chờ người ấy ừ cùng phiên bản này." : "Người ấy đã ừ. Còn bạn.";
    case "chot":
      return `Đã chốt. Hẹn ${pb?.content.ngay ?? "ngày đã ghi"}.`;
    case "da_di":
      return "Đã đi. Giữ lại một điều về buổi này?";
    case "da_giu":
      return "Đã giữ một điều. Tờ này về ký ức.";
    case "nghi_tuan":
      return "Tuần này nghỉ. Không ai phải mở lời.";
    case "het_han":
      return "Hết khung tuần, tờ này khép lại. Không thành kế hoạch.";
    case "rut":
      return toiGui ? "Bạn đã rút lại trước khi người ấy xem." : "Người ấy đã rút lại trước khi bạn xem.";
    case "bo":
      return "Bản phác đã bỏ, không ai từng nhận.";
    case "huy":
      return "Buổi này đã huỷ.";
  }
}

/** The actions a sheet offers me, in the order the screen lays them out. */
export type NutTo = "gui" | "sua_nhap" | "bo" | "dong_y" | "de_nghi_sua" | "rut" | "nghi_tuan" | "da_di" | "giu" | "huy";

export function nutChoTo(to: ToGiay, toiId: string): NutTo[] {
  const nghi: NutTo[] = coTheNghiTuan(to) ? ["nghi_tuan"] : [];
  switch (to.state) {
    case "nhap":
      // A draft is visible to its owner only (§3.3 rule 1), so seeing it means owning it.
      return ["gui", "sua_nhap", "bo", ...nghi];
    case "da_gui":
    case "da_xem":
    case "dong_y": {
      const ra: NutTo[] = [];
      if (coTheDongY(to, toiId)) ra.push("dong_y");
      if (coTheDeNghiSua(to, toiId)) ra.push("de_nghi_sua");
      if (coTheRut(to, toiId)) ra.push("rut");
      return [...ra, ...nghi];
    }
    case "de_nghi_sua":
      return nghi;
    case "chot":
      return ["da_di", "huy"];
    case "da_di":
      return ["giu"];
    default:
      return [];
  }
}

/**
 * Which sheet is THE open sheet on the paper surface (spec §15.1: one sheet,
 * not three cards). Priority: something I must decide → something I sent and
 * am waiting on → the plan that stands → the freshest memory. `undefined`
 * when the notebook has nothing to show but the invitation to write.
 */
export function toUuTien(ds: readonly ToGiay[], toiId: string): ToGiay | undefined {
  const moc = (to: ToGiay) => phienBan(to)?.sent_at ?? "";
  const moiNhat = (xs: ToGiay[]) => xs.sort((a, b) => (moc(b) > moc(a) ? 1 : moc(b) < moc(a) ? -1 : 0))[0];
  const canQuyet = ds.filter((to) => nutChoTo(to, toiId).some((n) => n === "dong_y" || n === "gui"));
  if (canQuyet.length) return moiNhat(canQuyet);
  const dangCho = ds.filter((to) => TRANG_THAI_MO.includes(to.state));
  if (dangCho.length) return moiNhat(dangCho);
  const keHoach = ds.filter((to) => to.state === "chot" || to.state === "da_di");
  if (keHoach.length) return moiNhat(keHoach);
  const kyUc = ds.filter((to) => to.state === "da_giu");
  return kyUc.length ? moiNhat(kyUc) : undefined;
}

/**
 * What closing the notebook would do, for the preview step (spec §7.6, plan
 * `DongSo`): sheets still being decided are cancelled, plans that stand are
 * locked, open consent proposals are cancelled. Counts only; the server's
 * `close/preview` is the source of truth on the live build (its wire adds
 * `so_to_huy` next to `so_to_khoa`; a preview that called a draft «khoá» was
 * read as a lie against the «Đã bỏ» that followed).
 */
export function demHauQuaDongSo(ds: readonly ToGiay[], soDeNghiCho: number): { so_to_huy: number; so_to_khoa: number; so_de_nghi_huy: number } {
  return {
    // Spec §7.6 draws the line the preview must say out loud: a sheet still
    // being decided is CANCELLED; a plan that stands is LOCKED, read only.
    so_to_huy: ds.filter((to) => TRANG_THAI_MO.includes(to.state)).length,
    so_to_khoa: ds.filter((to) => to.state === "chot" || to.state === "da_di").length,
    so_de_nghi_huy: Math.max(0, soDeNghiCho),
  };
}
