#!/usr/bin/env node
// Keep the native marker wrapper's bounds in sync with its RN-managed child.
// ReactViewGroup does not request a layout: a visible child can otherwise sit
// inside a 0x0 wrapper and disappear from Android's accessibility tree.
import { createHash } from "node:crypto";
import { readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = join(dirname(fileURLToPath(import.meta.url)), "..");
const library = join(root, "node_modules/@maplibre/maplibre-react-native");
const version = JSON.parse(readFileSync(join(library, "package.json"), "utf8")).version;
if (version !== "11.3.10") throw new Error(`Review the native marker fix for MapLibre ${version}.`);
const target = join(library, "android/src/main/java/org/maplibre/reactnative/components/annotations/markerview/MarkerViewManager.kt");
const originalHash = "50956c5e476d7c2e66e7a2bff4cbe62c968cd10693f8481b3bd5664c13d92e48";
const hash = (source) => createHash("sha256").update(source).digest("hex");
const anchor = "        val (viewWidth, viewHeight) = view.getContentSize()\n";
const addition = `        // RN lays out the child, but never this unregistered ReactViewGroup.
        // Give Android real bounds for accessibility, without relaying out the child.
        val width = viewWidth.toInt()
        val height = viewHeight.toInt()
        if (view.width != width || view.height != height) {
            view.measure(
                View.MeasureSpec.makeMeasureSpec(width, View.MeasureSpec.EXACTLY),
                View.MeasureSpec.makeMeasureSpec(height, View.MeasureSpec.EXACTLY),
            )
            view.layout(view.left, view.top, view.left + width, view.top + height)
        }
`;
const source = readFileSync(target, "utf8");
const patched = source.includes(anchor + addition) && hash(source.replace(addition, "")) === originalHash;
if (patched) {
  console.log("sua-maplibre-marker: bản sửa native đã khớp.");
} else {
  if (hash(source) !== originalHash) throw new Error("MapLibre marker source differs from the reviewed version.");
  if (process.argv.includes("--kiem")) throw new Error("Run sua-maplibre-marker before building the native APK.");
  writeFileSync(target, source.replace(anchor, anchor + addition));
  console.log("sua-maplibre-marker: đã sửa bounds của view bọc native.");
}
