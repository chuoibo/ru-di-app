/** A cold protected route must wait for the real HttpOnly session to resume. */
import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";
import test from "node:test";
import { findChrome, launch, serve } from "../chrome-cdp.mjs";
import { capture, enabled, required } from "./journey-browser.mjs";

test("web account reload waits for a slow cookie resume and keeps the same identity", { skip: !enabled && !required }, async () => {
  const file = process.env.RUDI_TEST_ACCOUNT_WORLD;
  assert.ok(file?.startsWith("/tmp/"), "Disposable account world required");
  const world = JSON.parse(readFileSync(file, "utf8"));
  const person = world.duc;
  assert.equal(person.username, "fixture_duc");
  const base = process.env.EXPO_PUBLIC_API_URL;
  assert.equal(new URL(base).hostname, "127.0.0.1");
  const root = new URL("../../", import.meta.url).pathname;
  const exported = process.env.MOBILE_WEB_EXPORT ?? join(root, ".expo-build-check");
  assert.ok(existsSync(join(exported, "index.html")), "Build the real export first");
  const chrome = findChrome();
  assert.ok(chrome, "Cookie regression requires Chrome");
  const server = await serve(exported);
  const page = await launch(chrome);
  try {
    await page.viewport(390, 844);
    // Only the build-check origin changes. All bodies and responses are real
    // Go HTTP. Delay resume so the initial signed-out render cannot win a race.
    await page.call("Page.addScriptToEvaluateOnNewDocument", { source: `
      window.qaSessionPaths = [location.pathname];
      for (const method of ["pushState", "replaceState"]) {
        const original = history[method].bind(history);
        history[method] = (...args) => {
          const result = original(...args);
          window.qaSessionPaths.push(location.pathname);
          return result;
        };
      }
      const originalFetch = window.fetch;
      window.fetch = async (input, options) => {
        const url = typeof input === "string" && input.startsWith("http://api.build-check.invalid")
          ? ${JSON.stringify(base)} + input.slice("http://api.build-check.invalid".length) : input;
        if (typeof url === "string" && url.endsWith("/sessions/web/resume"))
          await new Promise(resolve => setTimeout(resolve, 1000));
        return originalFetch(url, options);
      };
    ` });
    await page.goto(server.url + "login", () => !!document.querySelector('[data-testid="account-username"]'));
    await page.typeInto("Tên tài khoản", person.username);
    await page.typeInto("Mật khẩu", "isolated synthetic credential for duc");
    await page.clickChu("Đăng nhập");
    await page.waitFor(() => !document.querySelector('[data-testid="account-username"]'), { label: "managed login", timeout: 30000 });
    const { cookies } = await page.call("Network.getCookies", { urls: [base + "/sessions/web/resume"] });
    const cookie = cookies.find(row => row.name === "rudi_web_session");
    assert.ok(cookie, "Real login must persist its web session");
    assert.equal(cookie.httpOnly, true);
    assert.equal(cookie.secure, true);
    assert.equal(cookie.sameSite, "Strict");
    assert.equal(cookie.path, "/sessions/web");
    for (let lap = 0; lap < 2; lap++) {
      await page.goto(server.url + "settings/account", () => document.body.innerText.includes("@fixture_duc"));
      assert.equal(await page.evaluate(() => location.pathname), "/settings/account");
      assert.equal(await page.evaluate(() => window.qaSessionPaths.some(path => path === "/login")), false,
        "Cold account route redirected before the cookie resume completed");
    }
    await capture(page, "account-cookie-resumed");
    await page.goto(server.url + "login", () => !document.querySelector('[data-testid="account-username"]') && document.body.innerText.includes("Tin nhắn"));
    assert.notEqual(await page.evaluate(() => location.pathname), "/login", "A resumed session must leave the login door");
  } catch (error) {
    await capture(page, "account-cookie-failure");
    throw error;
  } finally {
    await page.close();
    await server.close();
  }
});
