/**
 * Where a screen's group data comes from, decided in one place.
 *
 * `live` names the person and the group every request is made as. Anything
 * else is `chua-co-nhom` with the reason: no session, no group yet, a group
 * that has not accepted this person, or a malformed id. There is no demo
 * story to fall back to; a screen in that state shows its own empty state or
 * the sign-in door.
 *
 * ## Why `invited` is not live
 *
 * Signing in is not joining. A first invitation lands `invited` and a member
 * still has to accept; the server refuses that person's group data, and the
 * screen must say the same thing rather than render an empty group as though
 * it were an empty trip.
 *
 * The `EXPO_PUBLIC_RUDI_*` pair is the DEV door -- it is how the native gate
 * drives live screens without minting an invitation every run -- and
 * `tests/cau-hinh-ban-dung.test.mjs` refuses it in any shippable profile.
 */

/** UUID as the server writes them. Same shape `navigation/lien-ket.ts` enforces. */
const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export type Nguon =
  /** No group data to read yet. `viSao` says why, and a screen may print it. */
  | { kieu: "chua-co-nhom"; viSao: string }
  /** Every number came from the server, as this person, in this group. */
  | { kieu: "live"; actorId: string; contextId: string };

/**
 * Dev-actor mode: an operator pins both halves of an identity at bundle time.
 *
 * Written as plain `process.env.X` member reads because Expo's inliner
 * pattern-matches the syntax tree -- `process?.env?.X` is an
 * OptionalMemberExpression, the guard returns false, and the read survives into
 * the bundle unreplaced. That exact mistake pinned every build to the
 * developer's own localhost once already; see `tests/env-inlining.test.mjs`.
 */
declare const process: { env: Record<string, string | undefined> };
const ACTOR_DEV = process.env.EXPO_PUBLIC_RUDI_ACTOR;
const CONTEXT_DEV = process.env.EXPO_PUBLIC_RUDI_CONTEXT;

/**
 * @param coPhien whether `src/api.ts` is holding a session bearer.
 *
 * Taken as an argument rather than imported so this stays a pure function of
 * its inputs and can be exercised without a device or a server.
 */
/** What the session module knows, reduced to what this decision needs. */
export type PhienToiThieu = {
  person_id: string;
  context_id: string | null;
  membership_state: "invited" | "active" | "left" | null;
};

export function nguonHienTai(
  phien: PhienToiThieu | null,
  moiTruong: { actor?: string; context?: string } = { actor: ACTOR_DEV, context: CONTEXT_DEV },
): Nguon {
  const { actor, context } = moiTruong;
  if (actor !== undefined || context !== undefined) {
    // Half a configuration is a mistake worth naming. Falling back quietly
    // would show nothing to somebody who believes they pinned a group.
    if (actor === undefined || context === undefined) {
      return {
        kieu: "chua-co-nhom",
        viSao: "Thiếu một nửa cấu hình dev: cần cả EXPO_PUBLIC_RUDI_ACTOR lẫn EXPO_PUBLIC_RUDI_CONTEXT.",
      };
    }
    if (!UUID_RE.test(actor) || !UUID_RE.test(context)) {
      // These go straight into a request path. A malformed one is a 404 storm
      // that reads on screen as "the server is broken".
      return {
        kieu: "chua-co-nhom",
        viSao: "Cấu hình dev sai hình dạng: actor và context phải là UUID.",
      };
    }
    return { kieu: "live", actorId: actor, contextId: context };
  }
  if (phien !== null) {
    if (phien.context_id === null) {
      return {
        kieu: "chua-co-nhom",
        viSao:
          "Đã đăng nhập nhưng chưa ở nhóm nào. Tạo nhóm hoặc nhận lời mời để xem dữ liệu thật.",
      };
    }
    if (phien.membership_state !== "active") {
      // The server will refuse this person's group data, and a screen that
      // rendered the group anyway would show an empty trip where the truth is
      // "nobody has accepted you yet".
      return {
        kieu: "chua-co-nhom",
        viSao: "Đã đăng nhập, nhưng nhóm còn phải duyệt thì bạn mới xem được dữ liệu nhóm.",
      };
    }
    if (!UUID_RE.test(phien.context_id)) {
      return { kieu: "chua-co-nhom", viSao: "Phiên mang một mã nhóm không đọc được." };
    }
    return { kieu: "live", actorId: phien.person_id, contextId: phien.context_id };
  }
  return { kieu: "chua-co-nhom", viSao: "Chưa đăng nhập." };
}
