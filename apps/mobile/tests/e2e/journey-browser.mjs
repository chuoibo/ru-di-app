// Real exported app and real Go HTTP, with disposable managed accounts only.
import assert from "node:assert/strict";
import { randomUUID } from "node:crypto";
import { existsSync, readFileSync, mkdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { findChrome, launch, serve } from "../chrome-cdp.mjs";
import { noiDungGui } from "../../dist-test/rudi/hanh-trinh/ke-hoach.js";

export const enabled = !!process.env.RUDI_TEST_ACCOUNT_WORLD;
export const required = process.env.MOBILE_REQUIRE_E2E === "1";

export async function openJourney({ webgl2 = true } = {}) {
  const worldFile = process.env.RUDI_TEST_ACCOUNT_WORLD;
  assert.ok(worldFile?.startsWith("/tmp/"), "Disposable account world required");
  const world = JSON.parse(readFileSync(worldFile, "utf8"));
  const person = world.minh;
  assert.equal(person.username, "fixture_minh");
  const base = process.env.EXPO_PUBLIC_API_URL;
  const api = new URL(base);
  assert.equal(api.hostname, "127.0.0.1");
  assert.equal(api.protocol, "http:");
  const tokens = JSON.parse(readFileSync(process.env.MOBILE_E2E_SESSIONS, "utf8"));
  const token = tokens[person.person_id];
  assert.ok(token);
  async function request(path, body) {
    const response = await fetch(base + path, {
      method: "POST", headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}`, "Idempotency-Key": randomUUID() }, body: JSON.stringify(body),
    });
    assert.ok(response.ok, `Journey fixture refused: ${path} ${response.status}`);
    return response.json();
  }
  const groupName = "Nhóm QA bản đồ " + randomUUID().slice(0, 8);
  const group = await request("/contexts", { display_name: groupName });
  const title = "Lịch trình QA " + randomUUID().slice(0, 8);
  const outing = await request(`/contexts/${group.id}/outings`, { title, starts_on: "2027-04-20", ends_on: "2027-04-20", headcount: 1, budget_per_person_vnd: 500000 });
  const stops = [
    { id: "tmp-map-a", position: 0, day: "2027-04-20", at: "12:30", label: "Điểm QA thứ nhất", place_id: null, place_name: null, duration_minutes: 30, time_locked: true, meeting_point: { label: "Điểm QA thứ nhất", lat: 11.94, lng: 108.43 } },
    { id: "tmp-map-b", position: 1, day: "2027-04-20", at: "14:00", label: "Điểm QA thứ hai", place_id: null, place_name: null, duration_minutes: 30, time_locked: true, meeting_point: { label: "Điểm QA thứ hai", lat: 11.98, lng: 108.45 } },
  ];
  const kept = await fetch(`${base}/outings/${outing.id}/itinerary`, { method: "PUT", headers: { "Content-Type": "application/json", Authorization: `Bearer ${token}`, "Idempotency-Key": randomUUID() }, body: JSON.stringify(noiDungGui({ expected_revision: outing.timeline_revision, days: [{ day: "2027-04-20", transport_mode: "motorbike", start_at: "07:00", start_stop_id: null, end_stop_id: null, return_to_start: false }], stops })) });
  const keptBody = await kept.json();
  assert.equal(kept.status, 200, "Coordinate itinerary must persist through Go: " + JSON.stringify(keptBody));
  assert.equal(keptBody.stops.length, 2);
  assert.equal(keptBody.stops[0].day, "2027-04-20");
  assert.equal(keptBody.stops[0].meeting_point.lat, 11.94);
  const reread = await fetch(`${base}/contexts/${group.id}/outings`, { headers: { Authorization: `Bearer ${token}` } });
  const rereadBody = await reread.json();
  assert.equal(reread.status, 200);
  const listed = rereadBody.outings.find(row => row.id === outing.id);
  assert.deepEqual(listed.stops, keptBody.stops, "Itinerary and group reads must carry the same stops");
  const root = new URL("../../", import.meta.url).pathname;
  const exported = process.env.MOBILE_WEB_EXPORT ?? join(root, ".expo-build-check");
  assert.ok(existsSync(join(exported, "index.html")), "Build the real web export first");
  const chrome = findChrome();
  assert.ok(chrome, "Browser evidence needs Chrome");
  const server = await serve(exported);
  const page = await launch(chrome, { webgl: webgl2, webgl2 });
  try {
    await page.viewport(390, 844);
    // The standard build-check export has an intentionally invalid API URL.
    // Redirect only that placeholder; bodies and responses remain real HTTP.
    await page.call("Page.addScriptToEvaluateOnNewDocument", { source: `const originalFetch=window.fetch;window.qaOutingReads=[];window.fetch=async(input,options)=>{const url=typeof input==='string'&&input.startsWith('http://api.build-check.invalid')?${JSON.stringify(base)}+input.slice('http://api.build-check.invalid'.length):input;const response=await originalFetch(url,options);if(typeof url==='string'&&url.includes('/contexts/')&&url.endsWith('/outings')&&(!options?.method||options.method==='GET')){const data=await response.clone().json();window.qaOutingReads.push({url,outings:data.outings?.map(o=>({id:o.id,stops:o.stops,days:o.days}))});}return response;};` });
    await page.goto(server.url + "login", () => !!document.querySelector('[data-testid="account-username"]'));
    await page.typeInto("Tên tài khoản", person.username);
    await page.typeInto("Mật khẩu", "isolated synthetic credential for minh");
    await page.clickChu("Đăng nhập");
    await page.waitFor(() => !document.querySelector('[data-testid="account-username"]'), { label: "managed login completed", timeout: 30000 });
    // Select the fixture group through the same conversation list as a user.
    await page.goto(server.url + "messages", () => !!document.querySelector('[data-testid="conversations-screen"]'));
    await page.waitFor(name => [...document.querySelectorAll('[aria-label]')].some(el => el.getAttribute('aria-label') === 'Mở nhóm ' + name), { label: "journey group listed", timeout: 30000, diagnose: () => document.body.innerText.slice(0, 1500) }, groupName);
    await page.clickLabel("Mở nhóm " + groupName);
    await page.waitFor(id => location.pathname.includes('/groups/' + id + '/chat'), { label: "fixture group selected" }, group.id);
    await page.clickLabel("Quay lại");
    await page.waitFor(() => location.pathname.includes('/messages'), { label: "back to conversation list" });
    await page.clickLabel("Lên plan");
    await page.waitFor(title => document.body.innerText.includes(title), { label: "journey in selected group", timeout: 30000, diagnose: () => ({ text:document.body.innerText.slice(-1600), reads:window.qaOutingReads }) }, title);
    await page.clickLabel("Mở kèo " + title);
    await page.waitFor(id => location.pathname === '/outings/' + id, { label: "exact fixture outing opened" }, outing.id);
    await page.waitFor(() => document.body.innerText.includes("Lịch trình QA") && !!document.querySelector('[data-testid="che-do-lich-trinh"]'), { label: "authenticated itinerary", timeout: 30000, diagnose: () => document.body.innerText.slice(0, 1500) });
    return { page, outing, close: async () => { await page.close(); await server.close(); } };
  } catch (error) {
    await page.close(); await server.close(); throw error;
  }
}

export async function capture(page, name) {
  const directory = process.env.MOBILE_TEST_ARTIFACTS;
  if (!directory) return;
  assert.ok(directory.startsWith("/tmp/"), "Synthetic browser artifacts must stay in /tmp");
  mkdirSync(directory, { recursive: true, mode: 0o700 });
  const { data } = await page.call("Page.captureScreenshot", { format: "png" });
  writeFileSync(join(directory, name + ".png"), Buffer.from(data, "base64"), { mode: 0o600 });
}
