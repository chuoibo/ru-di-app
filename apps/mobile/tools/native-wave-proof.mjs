/** Synthetic QA only; this serial is reserved for the campaign's own AVD. */
import { execFileSync } from "node:child_process";
import { mkdirSync, writeFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

import { homedir } from "node:os";
import { join } from "node:path";

// Any machine with an SDK, not one person's home: the repo test pins this.
const sdk = process.env.ANDROID_SDK_ROOT ?? process.env.ANDROID_HOME ?? join(homedir(), "Android", "Sdk");
const adb = join(sdk, "platform-tools", "adb");
const root = fileURLToPath(new URL("../../../.impeccable/review/wave1/", import.meta.url));
const call = (...args) => execFileSync(adb, ["-s", "emulator-5560", ...args], {
  timeout: 20000, maxBuffer: 32 * 1024 * 1024,
});
const [command, value, expected] = process.argv.slice(2);
if (command === "route") {
  if (!/^[a-z0-9/-]+$/.test(value ?? "")) throw Error("Invalid fixture route");
  console.log(call("shell", "am", "start", "-a", "android.intent.action.VIEW", "-d", `rudi://${value}`, "com.lakiet.rudi").toString());
} else if (command === "inspect" || command === "capture") {
  const xml = call("exec-out", "uiautomator", "dump", "/dev/tty").toString();
  if (command === "capture") {
    if (!/^[a-z0-9-]+$/.test(value ?? "") || !expected) throw Error("capture needs a safe name and expected visible text");
    if (!xml.includes(`text="${expected}"`) && !xml.includes(`content-desc="${expected}"`)) {
      throw Error("Expected screen was not found; no screenshot written");
    }
    mkdirSync(root, { recursive: true });
    writeFileSync(`${root}${value}.png`, call("exec-out", "screencap", "-p"));
    console.log(`${root}${value}.png`);
  } else {
    for (const node of xml.matchAll(/<node\b[^>]+>/g)) {
      const get = key => node[0].match(new RegExp(`${key}="([^"]*)"`))?.[1] || "";
      if (get("text") || get("content-desc")) console.log(JSON.stringify({ text: get("text"), label: get("content-desc"), bounds: get("bounds") }));
    }
  }
} else throw Error("Use route, inspect, or capture");
