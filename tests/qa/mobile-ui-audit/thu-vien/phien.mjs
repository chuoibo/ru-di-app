/* Sessions for the audit personas, without spending an OTP per screenshot.
 *
 * The OTP door is audited on its own (feature F01) through the real UI. Every
 * other capture needs a signed-in page at many window sizes, and OTP is rate
 * limited (5 challenges per 15 min per number, 10 requests per minute per IP),
 * so each persona signs in ONCE, by the same two API calls the app makes, and
 * the bearer is cached outside git with mode 0600.
 *
 * A page is then signed in the way the web build keeps its own session across
 * reloads (src/phien-web.ts): `POST /sessions/web` with the bearer sets the
 * HttpOnly cookie, and the app's boot calls `/sessions/web/resume`. Nothing is
 * injected into the app's memory and nothing is written to its storage; the
 * app signs itself in from the cookie exactly as it does after a reload.
 *
 * Phone numbers are never written here: Team Đà Lạt numbers come from
 * `soDienThoai(i)` (the seed's own function) and the chat-test numbers from the
 * sessions file the chat seed wrote.
 */
import { randomUUID } from "node:crypto";
import { chmodSync, existsSync, mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";

import { ROSTER, soDienThoai } from "../../../../apps/mobile/tools/seed-rudi-world-lib.mjs";

const cho = (ms) => new Promise((ok) => setTimeout(ok, ms));

async function goi(api, method, path, body, token) {
  const headers = { "Content-Type": "application/json", "Idempotency-Key": randomUUID() };
  if (token) headers.Authorization = `Bearer ${token}`;
  for (let lan = 0; lan < 3; lan++) {
    const r = await fetch(api + path, { method, headers, body: body === undefined ? undefined : JSON.stringify(body) });
    const text = await r.text();
    let json;
    try {
      json = JSON.parse(text);
    } catch {
      json = text;
    }
    if (r.status === 429 && lan < 2) {
      await cho(61_000);
      continue;
    }
    if (!r.ok) throw new Error(`${method} ${path} → ${r.status} ${typeof json === "string" ? json.slice(0, 120) : JSON.stringify(json).slice(0, 200)}`);
    return json;
  }
  throw new Error(`${method} ${path}: hết lượt thử`);
}

export { goi as goiApi };

/** Index in ROSTER of a Team Đà Lạt persona by display name. */
export function chiSoTrongRoster(ten) {
  const i = ROSTER.findIndex((p) => (typeof p === "string" ? p : p?.ten ?? p?.name) === ten);
  if (i < 0) throw new Error(`không có ${ten} trong ROSTER`);
  return i;
}

/**
 * A persona is `{ khoa, phone }`. `khoa` names the cache file; `phone` is only
 * ever held in memory.
 */
export function personaDaLat(i) {
  return { khoa: `dalat-${i}`, phone: soDienThoai(i) };
}

export function personaChatTest(sessionsFile, i) {
  const s = JSON.parse(readFileSync(sessionsFile, "utf8"));
  const u = s.users.find((x) => x.index === i);
  if (!u) throw new Error(`không có người chat-test số ${i}`);
  return { khoa: `chat-${i}`, phone: u.phone, tokenSan: u.token, hetHan: u.expiresAt };
}

/** A persona with a number no seed uses: a brand-new account on first sign-in. */
export function personaMoi(offset) {
  return { khoa: `moi-${offset}`, phone: soDienThoai(0, 1000 + offset) };
}

/** `dalat-<i>`, `chat-<i>`, `moi-<n>`, or "none" (signed out → null). */
export function personaTheoTen(ten, chatSessions) {
  if (!ten || ten === "none") return null;
  const [loai, so] = ten.split("-");
  if (loai === "dalat") return personaDaLat(Number(so));
  if (loai === "chat") return personaChatTest(chatSessions, Number(so));
  if (loai === "moi") return personaMoi(Number(so));
  throw new Error(`persona lạ: ${ten}`);
}

export async function layPhien(api, persona, thuMuc, { code = "000000" } = {}) {
  mkdirSync(thuMuc, { recursive: true, mode: 0o700 });
  const file = join(thuMuc, `${persona.khoa}.json`);
  if (existsSync(file)) {
    const p = JSON.parse(readFileSync(file, "utf8"));
    if (Date.parse(p.expires_at) > Date.now() + 3_600_000) return p;
  }
  if (persona.tokenSan && Date.parse(persona.hetHan) > Date.now() + 3_600_000) {
    const me = await goi(api, "GET", "/people/me", undefined, persona.tokenSan);
    const p = { token: persona.tokenSan, person_id: me.id ?? me.person_id, expires_at: persona.hetHan };
    writeFileSync(file, JSON.stringify(p), { mode: 0o600 });
    return p;
  }
  const challenge = await goi(api, "POST", "/auth/otp/request", { phone: persona.phone });
  const s = await goi(api, "POST", "/auth/otp/verify", { phone: persona.phone, challenge_id: challenge.challenge_id, code });
  const p = { token: s.token, person_id: s.person_id, expires_at: s.expires_at };
  writeFileSync(file, JSON.stringify(p), { mode: 0o600 });
  chmodSync(file, 0o600);
  return p;
}

/**
 * Sign a page in through the web session cookie, then open `path` cold. The
 * page must already be on the app's origin so the cookie is set for the site
 * the app will call from.
 */
export async function ganPhien(page, api, phien) {
  const kq = await page.evaluate(
    async ({ api, token }) => {
      const r = await fetch(`${api}/sessions/web`, { method: "POST", headers: { Authorization: `Bearer ${token}` }, credentials: "include" });
      return r.status;
    },
    { api, token: phien.token },
  );
  if (kq < 200 || kq >= 300) throw new Error(`POST /sessions/web → ${kq}`);
}

export async function boPhien(page, api) {
  await page.context().clearCookies();
  await page.evaluate(() => {
    try {
      localStorage.clear();
      sessionStorage.clear();
    } catch {
      /* storage may be unavailable; nothing to clear then */
    }
  });
  void api;
}
