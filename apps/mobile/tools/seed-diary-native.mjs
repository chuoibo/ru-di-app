/** Synthetic local-only fixture for the native diary flows. Never logs tokens. */
import { randomUUID } from "node:crypto";
import { writeFileSync } from "node:fs";
import { resolve, relative } from "node:path";
import { pngMau, soDienThoai } from "./seed-rudi-world-lib.mjs";

const base = process.env.DIARY_TEST_API ?? "http://127.0.0.1:8199";
if (!["localhost", "127.0.0.1"].includes(new URL(base).hostname)) throw new Error("Local test API required");
const output = resolve(process.env.DIARY_TEST_OUTPUT ?? "/tmp/rudi-diary-native-v3.json");
const repo = resolve(import.meta.dirname, "../../..");
if (!relative(repo, output).startsWith("..")) throw new Error("Fixture credentials must stay outside the repo");
let session;
async function call(path, body, form = false) {
  const r = await fetch(base + path, {
    method: body === undefined ? "GET" : "POST",
    headers: { ...(session ? { Authorization: `Bearer ${session.token}`, "X-Actor-ID": session.person_id } : {}), ...(form ? {} : { "Content-Type": "application/json" }), "Idempotency-Key": randomUUID() },
    body: body === undefined ? undefined : form ? body : JSON.stringify(body),
  });
  if (!r.ok) throw new Error(`${path}: ${r.status}`);
  return r.json();
}
const phone = soDienThoai(0, Date.now() % 100000);
const challenge = await call("/auth/otp/request", { phone });
session = await call("/auth/otp/verify", { phone, challenge_id: challenge.challenge_id, code: "000000" });
const room = await call("/contexts", { display_name: "Hội thử sổ · dữ liệu giả lập" });
// Match the source API's photo-day projection, including the UTC midnight gap.
const localDay = new Intl.DateTimeFormat("en-CA", { timeZone: "Asia/Ho_Chi_Minh", year: "numeric", month: "2-digit", day: "2-digit" });
const today = localDay.format(new Date());
const yesterday = localDay.format(new Date(Date.now() - 86400000));
const outings = [];
for (const [title, starts_on] of [["Hai ngày gom nắng", yesterday], ["Một chiều có nhau", today]]) {
  const outing = await call(`/contexts/${room.id}/outings`, { title, starts_on, ends_on: today, headcount: 2, budget_per_person_vnd: 0 });
  outings.push(outing.id);
}
const photos = [];
for (const color of [[75, 114, 129], [201, 125, 76], [153, 148, 119]]) {
  const form = new FormData();
  form.append("file", new Blob([pngMau(480, 360, color)], { type: "image/png" }), "synthetic-colour.png");
  photos.push(await call(`/contexts/${room.id}/photos`, form, true));
}
writeFileSync(output, JSON.stringify({ phone, session, room: room.id, outings, photos, day: today }), { mode: 0o600 });
console.log(`Synthetic diary fixture saved outside repo: ${output}`);
