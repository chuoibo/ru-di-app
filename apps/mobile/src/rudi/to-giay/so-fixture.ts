/**
 * The two-person notebook as the FIXTURE build runs it: pure transitions over
 * a list of papers, standing in for the server until Phase 4 wires the routes.
 *
 * Every function returns a new list and never mutates. They enforce the same
 * rules the server will (`to-giay.ts` decides what is allowed; this file only
 * moves the paper), so a Maestro table that drives the fixture drives the real
 * state machine's shape: send = the sender agreed, a counter-proposal is a new
 * version sent by the responder, `chot` needs both on one version and links
 * exactly one outing, `da_di` needs a recorder, a kept line ends the week well.
 *
 * The clock is a parameter. Nothing here reads `Date.now()`.
 */
import {
  type Chang,
  type NoiDungTo,
  type PhienBanTo,
  type ToGiay,
  coTheChot,
  coTheDeNghiSua,
  coTheDongY,
  coTheNghiTuan,
  coTheRut,
  phienBan,
} from "./to-giay";

export interface RangBuoc {
  khong_an_duoc: string;
  dung: string;
}

/** What the notebook knows about the two people, for Nếp's draft. */
export interface NepDuLieuPhac {
  ngay: string;
  /** A place the two have NOT been to; `null` when the routine has nothing to offer. */
  choMoi: { viec: string; gio: string } | null;
  diTiep: { viec: string; gio: string } | null;
  lyDo: string;
}

/**
 * The week a fixture sheet belongs to, and when it would stop being answerable.
 *
 * Written down rather than computed from `Date.now()`: the experience build
 * must render the same thing on every screenshot, and a sheet whose deadline
 * moved with the wall clock would make one pinned capture expire and the next
 * one not.
 */
const TUAN_MAU = "2026-09-14";
const HAN_TUAN_MAU = "2026-09-20T17:00:00Z";

const thay = (ds: readonly ToGiay[], moi: ToGiay): ToGiay[] => ds.map((to) => (to.id === moi.id ? moi : to));

/**
 * Nếp drafts a sheet for the turn holder: a deterministic template, no model
 * call (plan Phase 3a `phac_to_giay`). One main stop, an optional next stop,
 * every stop `can_kiem: true` because a draft says «chưa biết» (spec §8).
 */
export function phacToGiay(id: string, du: NepDuLieuPhac): ToGiay {
  const chang: Chang[] = [];
  if (du.choMoi) chang.push({ gio: du.choMoi.gio, viec: du.choMoi.viec, place_id: null, can_kiem: true });
  if (du.diTiep) chang.push({ gio: du.diTiep.gio, viec: du.diTiep.viec, place_id: null, can_kiem: true });
  const content: NoiDungTo = { ngay: du.ngay, chang };
  return {
    id,
    state: "nhap",
    version: 1,
    author_type: "human",
    sent_by: null,
    versions: [{ version: 1, content, ly_do: du.lyDo, sent_at: null, sent_by: null, author_type: "human", my_response: null, their_agreed: false, viewed_by_recipient_at: null }],
    outing_id: null,
    keeps: [],
    tuan: TUAN_MAU,
    expires_at: HAN_TUAN_MAU,
    // The experience build has no clock: a fixture sheet is never «today», so
    // the button the server gates stays hidden until `daDi` is pressed through
    // the fixture's own path. The live build reads the server's answer.
    co_the_ghi_da_di: true,
  };
}

/** Editing the draft rewrites version 1 in place: a draft has no history yet (§3.3 rule 1). */
export function suaNhap(ds: readonly ToGiay[], id: string, content: NoiDungTo, lyDo: string | null): ToGiay[] {
  const to = ds.find((x) => x.id === id);
  if (!to || to.state !== "nhap") return [...ds];
  const v1 = phienBan(to, 1);
  if (!v1) return [...ds];
  return thay(ds, { ...to, versions: [{ ...v1, content, ly_do: lyDo }] });
}

/** The turn holder presses send: `da_gui`, and the sender has agreed to v1 by sending (§3.4). */
export function guiTo(ds: readonly ToGiay[], id: string, toiId: string, now: string): ToGiay[] {
  const to = ds.find((x) => x.id === id);
  if (!to || to.state !== "nhap") return [...ds];
  const versions = to.versions.map((v) => (v.version === 1 ? { ...v, sent_at: now, sent_by: toiId } : v));
  return thay(ds, { ...to, state: "da_gui", sent_by: toiId, versions });
}

/** The recipient opened it: a view mark, shown to the sender only, and never an agreement (§3.3 rule 4). */
export function nguoiNhanXem(ds: readonly ToGiay[], id: string, now: string): ToGiay[] {
  const to = ds.find((x) => x.id === id);
  if (!to || to.state !== "da_gui") return [...ds];
  const versions = to.versions.map((v) => (v.version === to.version && v.viewed_by_recipient_at === null ? { ...v, viewed_by_recipient_at: now } : v));
  return thay(ds, { ...to, state: "da_xem", versions });
}

function chotNeuDu(to: ToGiay, toiId: string, outingId: string): ToGiay {
  if (!coTheChot(to, toiId)) return to;
  // K3: one sheet, one outing. A second call finds the link already there and
  // leaves it alone -- written as a branch, not `??`, because the id-default
  // gate reads `x ?? id` as a display value falling back to a raw id, and this
  // is a link nobody shows.
  if (to.outing_id !== null) return { ...to, state: "chot" };
  return { ...to, state: "chot", outing_id: outingId };
}

/** I agree to the current version. Both agreed to the same version ⇒ `chot` with exactly one outing. */
export function toiDongY(ds: readonly ToGiay[], id: string, toiId: string, outingId: string): ToGiay[] {
  const to = ds.find((x) => x.id === id);
  if (!to || !coTheDongY(to, toiId)) return [...ds];
  const versions = to.versions.map((v) => (v.version === to.version ? { ...v, my_response: "dong_y" as const } : v));
  return thay(ds, chotNeuDu({ ...to, state: "dong_y", versions }, toiId, outingId));
}

/** The other person agrees to the current version (the fixture's «(Bản trải nghiệm) Người kia đồng ý»). */
export function nguoiKiaDongY(ds: readonly ToGiay[], id: string, toiId: string, outingId: string): ToGiay[] {
  const to = ds.find((x) => x.id === id);
  if (!to || !["da_gui", "da_xem", "dong_y"].includes(to.state)) return [...ds];
  const pb = phienBan(to);
  // The other cannot answer a version they themselves sent (paper_self_response).
  if (!pb || (pb.author_type === "human" && pb.sent_by !== toiId)) return [...ds];
  const versions = to.versions.map((v) => (v.version === to.version ? { ...v, their_agreed: true } : v));
  return thay(ds, chotNeuDu({ ...to, state: "dong_y", versions }, toiId, outingId));
}

/**
 * A counter-proposal: version v+1, sent by the responder, who has thereby
 * agreed to it. The sheet returns to `da_gui` for the other side to answer.
 */
export function deNghiSua(ds: readonly ToGiay[], id: string, byId: string, toiId: string, content: NoiDungTo, lyDo: string | null, now: string): ToGiay[] {
  const to = ds.find((x) => x.id === id);
  if (!to) return [...ds];
  const boiToi = byId === toiId;
  if (boiToi ? !coTheDeNghiSua(to, toiId) : !["da_gui", "da_xem", "dong_y"].includes(to.state)) return [...ds];
  const pb = phienBan(to);
  if (!pb) return [...ds];
  if (!boiToi && pb.author_type === "human" && pb.sent_by === byId) return [...ds];
  const truoc = to.versions.map((v) => (v.version === to.version && boiToi ? { ...v, my_response: "de_nghi_sua" as const } : v));
  const moi: PhienBanTo = {
    version: to.version + 1,
    content,
    ly_do: lyDo,
    sent_at: now,
    sent_by: byId,
    author_type: "human",
    my_response: null,
    their_agreed: false,
    viewed_by_recipient_at: null,
  };
  return thay(ds, { ...to, state: "da_gui", version: moi.version, sent_by: byId, versions: [...truoc, moi] });
}

export function rutTo(ds: readonly ToGiay[], id: string, actorId: string): ToGiay[] {
  const to = ds.find((x) => x.id === id);
  if (!to || !coTheRut(to, actorId)) return [...ds];
  return thay(ds, { ...to, state: "rut" });
}

/** «Tuần này nghỉ»: a draft is dropped, a sent sheet is cancelled (§3.3 table). */
export function nghiTuan(ds: readonly ToGiay[], id: string): ToGiay[] {
  const to = ds.find((x) => x.id === id);
  if (!to || !coTheNghiTuan(to)) return [...ds];
  return thay(ds, { ...to, state: to.state === "nhap" ? "nghi_tuan" : "huy" });
}

export function boNhap(ds: readonly ToGiay[], id: string): ToGiay[] {
  const to = ds.find((x) => x.id === id);
  if (!to || to.state !== "nhap") return [...ds];
  return thay(ds, { ...to, state: "bo" });
}

/** One person records that the outing happened (`recorded_by`); nothing is inferred from the date. */
export function ghiDaDi(ds: readonly ToGiay[], id: string, recordedBy: string): ToGiay[] {
  const to = ds.find((x) => x.id === id);
  if (!to || to.state !== "chot" || !recordedBy) return [...ds];
  return thay(ds, { ...to, state: "da_di" });
}

/** The first kept line turns `da_di` into `da_giu`; later lines just accumulate. */
export function giuMotDieu(ds: readonly ToGiay[], id: string, line: string, now: string): ToGiay[] {
  const to = ds.find((x) => x.id === id);
  const sach = line.trim();
  if (!to || !sach || !["da_di", "da_giu"].includes(to.state)) return [...ds];
  const keeps = [...to.keeps, { id: `${id}-giu-${to.keeps.length + 1}`, line: sach, created_at: now }];
  return thay(ds, { ...to, state: "da_giu", keeps });
}

export function huyBuoi(ds: readonly ToGiay[], id: string): ToGiay[] {
  const to = ds.find((x) => x.id === id);
  if (!to || to.state !== "chot") return [...ds];
  return thay(ds, { ...to, state: "huy" });
}

/**
 * Closing the notebook closes it (§3.3 rule 7, §7.6): every open sheet is
 * cancelled (a draft is dropped), plans and memories stay as they are, read only.
 */
export function dongSo(ds: readonly ToGiay[]): ToGiay[] {
  return ds.map((to) => {
    if (to.state === "nhap") return { ...to, state: "bo" as const };
    if (["da_gui", "da_xem", "de_nghi_sua", "dong_y"].includes(to.state)) return { ...to, state: "huy" as const };
    return to;
  });
}
