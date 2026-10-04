/** Sessions issued by the managed account doors (ADR-0053).
 * The server chooses the immutable person UUID. Native tokens live in
 * SecureStore; web tokens stay in memory and reload through an HttpOnly
 * Secure SameSite=Strict cookie. No account proof uses idempotency replay.
 */
import {
  BASE_URL,
  datTokenPhien,
  newAttempt,
  tokenPhienHienTai,
  translatedAnonymous,
  translatedAsActor,
} from "./api";
import { khoPhienWeb, type KhoAnToan } from "./phien-web";

/** What the server hands back once, and what we keep. */
export type TinCuoiTomTat = {
  id: string;
  kind: "text" | "image" | "ai_card" | "sticker" | "deleted";
  preview: string;
  author_id: string | null;
  author_display_name: string | null;
  created_at: string;
};

export type NhomTomTat = {
  id: string;
  display_name: string;
  my_state: "invited" | "active";
  my_role?: "member" | "admin";
  membership_id: string;
  member_count: number;
  unread_count: number;
  last_message?: TinCuoiTomTat | null;
  /** ADR-0021 §2.4: one of five slugs; absent on a server older than L1. */
  theme?: string;
  /** ADR-0021 §2.5: `pair` is a private conversation between two friends;
   *  absent (a group) on a server older than L2. */
  kind?: "group" | "pair";
  /** The other person of a pair, named by the server on every read. */
  counterpart?: { id: string; display_name: string } | null;
  /** ADR-0023 §2.3.2: cặp này không nhận tin mới nữa (bị chặn, hoặc người kia đã xoá tài khoản). */
  unavailable?: boolean;
};

export type Phien = {
  token: string;
  person_id: string;
  expires_at: string;
  /** `null` for a session minted by a door that is not an invitation (OTP,
   *  Google): the person may belong to no group yet. The invite door and
   *  `chonNhomMacDinh` fill it whenever a group is known. */
  context_id: string | null;
  membership_state: "invited" | "active" | "left" | null;
  membership_id: string | null;
  /** Which door minted this session (ADR-0016). Absent on rows stored before
   *  the field existed. */
  issued_via?: "password" | "google" | "invite" | "otp" | "genesis";
  is_new_person?: boolean;
  profile?: { display_name: string };
  /** Every group the person is in or invited to, as the server listed them. */
  contexts?: NhomTomTat[];
};

export type { KhoAnToan } from "./phien-web";

const KHOA = "rudi.phien";

const LOI_DANG_XUAT: Record<string, string> = {
  http_401: "Phiên đã hết hiệu lực rồi.",
};

/** In memory only: the fallback where there is neither SecureStore nor a browser (node). */
export function khoTrongBoNho(): KhoAnToan {
  let giu: string | null = null;
  return {
    async doc() {
      return giu;
    },
    async ghi(_khoa, giaTri) {
      giu = giaTri;
    },
    async xoa() {
      giu = null;
    },
  };
}

let khoMacDinh: KhoAnToan | null = null;

/**
 * SecureStore when the platform has it; in a browser, the cookie-backed web
 * store; memory otherwise.
 *
 * Resolved once and remembered, because the answer cannot change inside one
 * run of the app, and because a failed dynamic import should not be retried on
 * every read.
 */
export async function khoAnToanMacDinh(): Promise<KhoAnToan> {
  if (khoMacDinh !== null) return khoMacDinh;
  try {
    const store = await import("expo-secure-store");
    // Touch the API before committing to it: the module resolves on web and
    // then throws from its methods, and finding that out on the first read
    // would lose a token that had already been issued.
    await store.getItemAsync(KHOA);
    khoMacDinh = {
      doc: (khoa) => store.getItemAsync(khoa),
      ghi: (khoa, giaTri) => store.setItemAsync(khoa, giaTri),
      xoa: (khoa) => store.deleteItemAsync(khoa),
    };
  } catch {
    khoMacDinh =
      typeof document !== "undefined" && typeof fetch === "function"
        ? khoPhienWeb(BASE_URL)
        : khoTrongBoNho();
  }
  return khoMacDinh;
}

/** A non-empty string, or `null`. An empty id is an absent id, not a group named "". */
function chuoiHayNull(gia: unknown): string | null {
  return typeof gia === "string" && gia !== "" ? gia : null;
}

function docPhien(thoLuu: string | null): Phien | null {
  if (thoLuu === null) return null;
  try {
    const parsed = JSON.parse(thoLuu) as Partial<Phien>;
    if (typeof parsed.token !== "string" || parsed.token === "") return null;
    if (typeof parsed.person_id !== "string") return null;
    if (typeof parsed.expires_at !== "string") return null;
    // The group triple is nullable since ADR-0016: a session from the OTP or
    // Google door may belong to no group yet, and that is a person who should
    // land on "chưa có nhóm nào", not be signed out. A row written before the
    // fields existed still carries all three as strings and reads unchanged.
    const state =
      parsed.membership_state === "invited" ||
      parsed.membership_state === "active" ||
      parsed.membership_state === "left"
        ? parsed.membership_state
        : null;
    return {
      token: parsed.token,
      person_id: parsed.person_id,
      context_id: chuoiHayNull(parsed.context_id),
      expires_at: parsed.expires_at,
      membership_state: state,
      membership_id: chuoiHayNull(parsed.membership_id),
      issued_via: parsed.issued_via,
      is_new_person: parsed.is_new_person,
      profile: parsed.profile,
      contexts: Array.isArray(parsed.contexts) ? parsed.contexts : undefined,
    };
  } catch {
    // A corrupted record is a signed-out app, not a crashed one.
    return null;
  }
}

const LOI_VAO_NHOM: Record<string, string> = {
  // The row is gone, or was never this person's. Both read the same from here.
  http_404: "Lời mời này không còn hiệu lực. Nhờ người trong nhóm mời lại.",
  // Somebody already accepted, or the row moved on. Not an error worth a
  // scary sentence -- the next screen will show where they actually stand.
  http_409: "Trạng thái nhóm vừa đổi. Mở lại màn hình để xem hiện tại.",
};

/**
 * Consent to the membership this session was issued for, and remember it.
 *
 * Why the invitee presses this at all: a named invitation carries an existing
 * member's choice of a person, so the remaining question is that person's own
 * consent, not a second approval (ADR-0014 s8, `accept_context_membership`
 * requires `is_invitee`). A link invitation is the other shape and is not this
 * function's business -- there a member who is already in must approve.
 *
 * The stored record is rewritten from the SERVER's answer rather than being
 * patched to `"active"` locally. The two differ whenever the server declined
 * to move the row, and a local patch would leave the phone believing it is in
 * a group it is not -- which `nguon.ts` reads as permission to show group
 * money.
 */
export async function vaoNhom(phien: Phien, kho?: KhoAnToan): Promise<Phien> {
  if (phien.membership_id === null) {
    throw new Error("Phiên này không mang thẻ thành viên nào để đồng ý.");
  }
  const wire = await translatedAsActor<{ state: "invited" | "active" | "left" }>(
    LOI_VAO_NHOM,
    `/memberships/${phien.membership_id}/accept`,
    // No `contexts` claim. The route reads `membership_id` and asks only
    // `is_invitee`; claiming membership of a group this person has not joined
    // yet would be a false sentence on a `dev` host and ignored on a `prod`
    // one, so it is worth nothing and costs a lie.
    { method: "POST", actorId: phien.person_id },
  );
  const moi: Phien = { ...phien, membership_state: wire.state };
  await ghiNho(moi, kho);
  return moi;
}

/**
 * A session with a group to stand in, when the server knows one.
 *
 * The OTP door answers with `context_id: null` and the full `contexts` list;
 * the screens that already read live money (`nguon.ts`) need one group on the
 * session. The first ACTIVE membership is that group -- a person with several
 * picks another from the conversation list (M2). Pure, so it can be tested.
 */
export function chonNhomMacDinh(phien: Phien): Phien {
  if (phien.context_id !== null) return phien;
  // Never a pair (ADR-0021 §2.5): the money screens read the current group,
  // and a private conversation is not where somebody expects to find a bill.
  const active = phien.contexts?.find((nhom) => nhom.my_state === "active" && nhom.kind !== "pair");
  if (active === undefined) return phien;
  return {
    ...phien,
    context_id: active.id,
    membership_state: "active",
    membership_id: active.membership_id,
  };
}

/**
 * Every group this person is in or invited to, as the server lists them.
 *
 * `GET /people/me/contexts` (ADR-0016) is how a session minted by a door that
 * knows no group finds one. Read as the actor only for the `X-Actor-ID` header
 * a dev-mode server still looks at; in `prod` the bearer decides who "me" is.
 */
export async function docNhomCuaToi(personId: string): Promise<NhomTomTat[]> {
  const wire = await translatedAsActor<{ contexts: NhomTomTat[] }>({}, "/people/me/contexts", {
    method: "GET",
    actorId: personId,
  });
  return wire.contexts;
}

/**
 * A fresh group list on an existing session, written back to the disk.
 *
 * Used right after a group is created or accepted: the server already knows,
 * and the phone must not keep saying "chưa có nhóm nào" until the next launch.
 */
export async function ganDanhSachNhom(
  phien: Phien,
  contexts: NhomTomTat[],
  kho?: KhoAnToan,
): Promise<Phien> {
  const moi = chonNhomMacDinh({ ...phien, contexts });
  await ghiNho(moi, kho);
  return moi;
}

/**
 * The display name the session greets with, after `PATCH /people/me` changed it.
 *
 * The session is minted with whatever name the server had at sign-in, often
 * its placeholder; without writing the new one back, a person who just typed
 * their name kept being «Thành viên mới» until the next sign-in (QA 23/09).
 */
export async function doiTenTrongPhien(phien: Phien, ten: string, kho?: KhoAnToan): Promise<Phien> {
  const moi: Phien = { ...phien, profile: { ...phien.profile, display_name: ten } };
  await ghiNho(moi, kho);
  return moi;
}

/**
 * Make one of the listed groups the current one, and remember it.
 *
 * The conversation list is where a person with several groups picks which one
 * the money screens read. Only a group the server listed can be chosen -- an
 * id typed from nowhere would send `nguon.ts` live on a group the server may
 * refuse -- and an `invited` row is not a choice yet: accepting is `vaoNhom`.
 */
export async function chonNhom(phien: Phien, contextId: string, kho?: KhoAnToan): Promise<Phien> {
  const nhom = phien.contexts?.find((ung) => ung.id === contextId);
  if (nhom === undefined) {
    throw new Error("Nhóm này không còn trong danh sách của bạn.");
  }
  if (nhom.my_state !== "active") {
    throw new Error("Bạn chưa đồng ý vào nhóm này.");
  }
  const moi: Phien = {
    ...phien,
    context_id: nhom.id,
    membership_state: "active",
    membership_id: nhom.membership_id,
  };
  await ghiNho(moi, kho);
  return moi;
}

/** The caller's own profile as `GET /people/me` returns it (M2). */
export type HoSoToi = {
  id: string;
  display_name: string;
  bio: string | null;
  city: string | null;
  created_at: string;
  counts: {
    friends: number;
    contexts: number;
    outings: number;
    places_checked_in: number;
    memories: number;
  };
  login_methods: string[];
  /** ADR-0022 §2.2: who may comment on my posts; absent on a server older than L3. */
  wall_comment_policy?: string;
  /** ADR-0023 §2.5: findable by telephone number; absent on a server older than L5. */
  discoverable_by_phone?: boolean;
};

const LOI_HO_SO: Record<string, string> = {
  person_not_found: "Chưa có hồ sơ cho tài khoản này. Đăng nhập lại giúp mình.",
  http_422: "Hồ sơ chưa hợp lệ: tên không được rỗng, giới thiệu tối đa 500 chữ.",
};

export async function docHoSoToi(personId: string): Promise<HoSoToi> {
  return translatedAsActor<HoSoToi>(LOI_HO_SO, "/people/me", { method: "GET", actorId: personId });
}

/** Partial update; `bio`/`city` = "" clears the field. */
export async function suaHoSoToi(
  personId: string,
  thayDoi: {
    display_name?: string;
    bio?: string;
    city?: string;
    discoverable_by_phone?: boolean;
  },
): Promise<HoSoToi> {
  return translatedAsActor<HoSoToi>(LOI_HO_SO, "/people/me", {
    method: "PATCH",
    body: thayDoi,
    actorId: personId,
    attempt: newAttempt(),
  });
}

export async function ghiNho(phien: Phien, kho?: KhoAnToan): Promise<void> {
  datTokenPhien(phien.token);
  const store = kho ?? (await khoAnToanMacDinh());
  await store.ghi(KHOA, JSON.stringify(phien));
}

/**
 * Read the stored session at launch, if there is one.
 *
 * An expired record is dropped here rather than sent: the server would answer
 * 401 and the app would show a person a failure for something it already knew.
 */
export async function khoiPhucPhien(kho?: KhoAnToan): Promise<Phien | null> {
  const store = kho ?? (await khoAnToanMacDinh());
  const phien = docPhien(await store.doc(KHOA));
  if (phien === null) return null;
  if (Date.parse(phien.expires_at) <= Date.now()) {
    await store.xoa(KHOA);
    datTokenPhien(null);
    return null;
  }
  datTokenPhien(phien.token);
  if (phien.contexts !== undefined) return phien;
  // A session resumed on the web (and any record older than the field) knows
  // who but not which groups: ask, the way a sign-in by OTP does. Offline, the
  // person is still signed in; the group list fills on its next refresh.
  try {
    const coNhom = chonNhomMacDinh({ ...phien, contexts: await docNhomCuaToi(phien.person_id) });
    await store.ghi(KHOA, JSON.stringify(coNhom));
    return coNhom;
  } catch {
    return phien;
  }
}

/**
 * Sign out on the server first, then forget locally.
 *
 * That order is the point. A session only the phone forgets is still a live
 * credential on the server, and a phone somebody else is holding is exactly
 * when that matters. The local record is cleared even when the call fails,
 * because a person who pressed sign-out has said what they want and the
 * server-side row will expire on its own.
 */
export async function dangXuat(personId: string, kho?: KhoAnToan): Promise<void> {
  const token = tokenPhienHienTai();
  try {
    if (token !== null) {
      await translatedAsActor<void>(LOI_DANG_XUAT, "/sessions/current", {
        method: "DELETE",
        actorId: personId,
      });
    }
  } finally {
    datTokenPhien(null);
    const store = kho ?? (await khoAnToanMacDinh());
    await store.xoa(KHOA);
  }
}
