/** Where the web build keeps its session between page loads.
 *
 * A browser has no keystore. The bearer is not written to localStorage or
 * sessionStorage (see the header of `phien.ts`); it lives in memory, and the
 * server keeps an HttpOnly cookie (`services/core/internal/websession`) that
 * no script can read and that is only ever sent to `/sessions/web/*`. On a
 * fresh page load `doc` asks `/sessions/web/resume` for the session back.
 *
 * What comes back is the minimum a sign-in returns -- token, person, expiry,
 * door, name -- with no group chosen. `khoiPhucPhien` fills the groups from
 * `GET /people/me/contexts` exactly as a sign-in by OTP would.
 *
 * Every call here is best-effort. A server without these routes (older build,
 * or a host whose CORS list is `*`) answers with a refusal, and the result is
 * today's behaviour: signed in until the page reloads, never a thrown error.
 */
import { headerChiBearer } from "./danh-tinh";

/** Where a secret is kept between launches. */
export type KhoAnToan = {
  doc(khoa: string): Promise<string | null>;
  ghi(khoa: string, giaTri: string): Promise<void>;
  xoa(khoa: string): Promise<void>;
};

function phienTuMayChu(wire: unknown): string | null {
  if (!wire || typeof wire !== "object") return null;
  const w = wire as { token?: unknown; person_id?: unknown; expires_at?: unknown; issued_via?: unknown; profile?: unknown };
  if (typeof w.token !== "string" || w.token === "" || typeof w.person_id !== "string" || typeof w.expires_at !== "string") return null;
  const ten = (w.profile as { display_name?: unknown } | null | undefined)?.display_name;
  return JSON.stringify({
    token: w.token,
    person_id: w.person_id,
    expires_at: w.expires_at,
    context_id: null,
    membership_state: null,
    membership_id: null,
    issued_via: typeof w.issued_via === "string" ? w.issued_via : undefined,
    profile: typeof ten === "string" ? { display_name: ten } : undefined,
  });
}

/** Every request goes through the global `fetch` with its path written out, so
 *  scripts/check_api_contract.py can read which routes this file calls. */
export function khoPhienWeb(baseUrl: string): KhoAnToan {
  let giu: string | null = null;
  // The token the cookie was last set for: a group switch rewrites the
  // session record, and that must not cost a request each time.
  let daGanCookie: string | null = null;
  return {
    async doc() {
      if (giu !== null) return giu;
      try {
        const tra = await fetch(`${baseUrl}/sessions/web/resume`, { method: "POST", credentials: "include" });
        if (!tra.ok) return null;
        const phien = phienTuMayChu(await tra.json());
        if (phien === null) return null;
        giu = phien;
        daGanCookie = (JSON.parse(phien) as { token: string }).token;
        return giu;
      } catch {
        return null;
      }
    },
    async ghi(_khoa, giaTri) {
      giu = giaTri;
      let token: string | null = null;
      try {
        token = (JSON.parse(giaTri) as { token?: unknown }).token as string;
      } catch {
        token = null;
      }
      if (typeof token !== "string" || token === "" || token === daGanCookie) return;
      try {
        const tra = await fetch(`${baseUrl}/sessions/web`, {
          method: "POST",
          headers: headerChiBearer(token),
          credentials: "include",
        });
        if (tra.ok) daGanCookie = token;
      } catch {
        // Still signed in for this page; only the reload is not covered.
      }
    },
    async xoa() {
      giu = null;
      daGanCookie = null;
      try {
        await fetch(`${baseUrl}/sessions/web/clear`, { method: "POST", credentials: "include" });
      } catch {
        // The server-side session is what sign-out revokes; a cookie left
        // behind only resumes into a 401, which clears it.
      }
    },
  };
}
