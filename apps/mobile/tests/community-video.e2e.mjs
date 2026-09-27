// Run against an exported web app with an approved synthetic video post.
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { setTimeout as delay } from "node:timers/promises";
import puppeteer from "puppeteer-core";

const [base, fixture, post] = process.argv.slice(2);
const url = new URL(base);
assert(["127.0.0.1", "localhost"].includes(url.hostname), "Loopback QA app only");
assert(/^[a-f0-9-]{36}$/.test(post), "Synthetic post ID required");
const { actors } = JSON.parse(await readFile(fixture, "utf8"));
assert(actors[0].token.startsWith("synthetic-"), "Synthetic session required");
const browser = await puppeteer.launch({
  executablePath: process.env.CHROME_BIN || "/usr/bin/google-chrome",
  headless: true,
  args: ["--no-sandbox"],
});
try {
  const page = await browser.newPage();
  await page.setViewport({ width: 390, height: 844 });
  const errors = [];
  let authorizedMedia = false;
  page.on("pageerror", (error) => errors.push(error.message));
  page.on("response", (response) => {
    if (response.url().includes("/v2/community/media/")) {
      authorizedMedia ||= response.status() === 200 && Boolean(response.request().headers().authorization);
    }
  });
  await page.setCookie({
    name: "rudi_web_session", value: actors[0].token, domain: url.hostname,
    path: "/sessions/web", httpOnly: true, sameSite: "Strict",
  });
  await page.goto(`${base}/community/posts/${post}`, { waitUntil: "networkidle2" });
  await page.waitForFunction(() => {
    const video = document.querySelector("video");
    return video && video.readyState >= 2 && video.duration > 0;
  }, { timeout: 30000 });
  await page.evaluate(async () => {
    const video = document.querySelector("video");
    video.muted = true;
    await video.play();
  });
  await delay(600);
  const playback = await page.evaluate(() => {
    const video = document.querySelector("video");
    video.pause();
    return { time: video.currentTime, duration: video.duration, blob: video.src.startsWith("blob:") };
  });
  assert(authorizedMedia, "Video bytes must be fetched with authorization");
  assert(playback.time > 0 && playback.blob, "Video must actually decode and advance");
  assert.deepEqual(errors, []);
  console.log(JSON.stringify({ result: "PASS", authorizedMedia, ...playback }));
} finally {
  await browser.close();
}
