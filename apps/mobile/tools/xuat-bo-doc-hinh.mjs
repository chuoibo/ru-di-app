/** Export synthetic art from the compiled production geometry for a blind read.
 * No screenshots are edited; SVG paths/crops are the ones the native renderers
 * consume. This is a review packet, not evidence of native raster or human understanding.
 * Compile with tsc + fixup-esm first. Pass a new, empty output directory.
 */
import { existsSync, mkdirSync, readdirSync, writeFileSync, readFileSync } from "node:fs";
import { resolve, dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { KHUNG_KY_HOA, hinhKyHoa, moTaKyHoa } from "../dist-test/rudi/art/ky-hoa.js";
import { KHUNG_CANH, hinhCanh, hopNgang, moTaCanh } from "../dist-test/rudi/art/canh.js";
import { KHUNG_STICKER, hinhSticker, nhanSticker } from "../dist-test/rudi/chat/sticker.js";

const app = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const tokens = JSON.parse(readFileSync(join(app, "../../packages/shared/tokens.json"), "utf8"));
const out = process.argv[2] && resolve(process.argv[2]);
if (!out || (existsSync(out) && readdirSync(out).length)) {
  throw new Error("Cần một thư mục đầu ra mới/rỗng; không ghi đè bộ đã phát cho người đọc.");
}
mkdirSync(out, { recursive: true });

const sketch = (category, tags, compact) => ({
  kind: "ky-hoa", category, tags, compact,
  answer: moTaKyHoa(category, tags, { gon: compact }),
  layers: hinhKyHoa(category, tags, { gon: compact }),
  view: `0 0 ${KHUNG_KY_HOA.w} ${KHUNG_KY_HOA.h}`,
  width: 360, height: compact ? 90 : 120, paper: true, slice: true,
});
const scene = (id, nep = true) => {
  const layers = hinhCanh(id, { nep });
  const { x0, x1 } = hopNgang(layers);
  const left = Math.max(0, Math.floor(x0 - 4));
  const width = Math.min(KHUNG_CANH.w, Math.ceil(x1 + 4)) - left;
  return { kind: "canh", id, nep, answer: moTaCanh(id), layers,
    view: `${left} 0 ${width} ${KHUNG_CANH.h}`, width, height: KHUNG_CANH.h };
};
const sticker = (id, size) => ({
  kind: "sticker", id, size, answer: nhanSticker(id),
  layers: hinhSticker(id, { chiTiet: size >= 72 }).lop,
  view: `0 0 ${KHUNG_STICKER} ${KHUNG_STICKER}`, width: size, height: size,
});
// Interleave types and variants rather than placing the answer-bearing larger
// reading next to its compact sibling. The facilitator assigns one variant
// per concept/person before the first response; the key stays outside the packet.
const cases = [
  sketch("cafe", ["Chill", "Săn mây"], false),
  sticker("ket-xe", 64),
  scene("chua-co-tin-nhan"),
  sketch("vui-choi", ["Săn mây"], true),
  sketch("di-choi-dem", ["Nhộn nhịp"], false),
  scene("bo-loc-che-het"),
  sketch("quan-an-local", ["View đẹp", "Chill"], true),
  sticker("tra-tien-ne", 120),
  scene("chua-co-loi-moi"),
  sketch("khac", [], false),
  sketch("cafe", ["Chill", "Săn mây"], true),
  scene("chua-doc-duoc"),
  sticker("ket-xe", 120),
  sketch("quan-an-local", ["View đẹp", "Chill"], false),
  scene("chua-co-ky-niem"),
  sketch("vui-choi", ["Săn mây"], false),
  sticker("tra-tien-ne", 64),
  sketch("di-choi-dem", ["Nhộn nhịp"], true),
  scene("chua-co-tin-nhan", false),
  sketch("khac", [], true),
];
const escape = (s) => String(s).replaceAll("&", "&amp;").replaceAll('"', "&quot;").replaceAll("<", "&lt;").replaceAll(">", "&gt;");
const key = [];
for (const theme of ["light", "dark"]) {
  const folder = theme === "light" ? "sang" : "toi";
  const dir = join(out, "nguoi-doc", folder);
  mkdirSync(dir, { recursive: true });
  const colors = tokens.color[theme];
  const links = [];
  for (const [index, c] of cases.entries()) {
    const code = `H${String(index + 1).padStart(2, "0")}`;
    // Exactly the role mapping used by VeLop and Sticker. Geometry retains
    // its neutral role names only in memory; neither role nor concept leaves
    // the facilitator key as SVG metadata, labels, ids or filenames.
    const colorsByRole = c.kind === "sticker"
      ? { accent: colors.accent, ink: c.layers.some(l => l.mau === "coral") ? tokens.color.light.ink : colors.ink,
          split: colors.split, card: colors.paper, coral: tokens.brand.coral, line: colors.paperShade }
      : { giay: colors.paper, bong: colors.paperShade, gap: colors.accent, mo: colors.accentSoft,
          split: colors.split, ai: colors.ai, muc: colors.ink };
    const paths = c.layers.map(l => {
      const color = colorsByRole[l.mau];
      if (!color) throw new Error(`Unknown color role: ${l.mau}`);
      return l.net === undefined
        ? `<path d="${escape(l.d)}" fill="${color}"/>`
        : `<path d="${escape(l.d)}" fill="none" stroke="${color}" stroke-width="${l.net}" stroke-linecap="round" stroke-linejoin="round"/>`;
    }).join("");
    const background = c.paper ? colors.paper : colors.ground;
    const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="${c.width}" height="${c.height}" viewBox="${c.view}" preserveAspectRatio="xMidYMid ${c.slice ? "slice" : "meet"}" style="background:${background}">${paths}</svg>\n`;
    // Keep the participant directory self-contained and text-only: an inline
    // SVG avoids a second file whose name could accidentally acquire a label
    // when a packet is copied or reordered. The role/aria label contains only
    // the neutral code, never the facilitator answer.
    links.push(`<li><a href="${code}.html">${code}</a></li>`);
    const page = `<h1>${code}</h1><div role="img" aria-label="Hình ${code}" class="art">${svg}</div><p>Bạn thấy gì trong hình này? Bạn nghĩ hình muốn nói điều gì?</p>`;
    const forbidden = [c.answer, c.kind, c.id, ...(c.tags ?? [])].filter(Boolean);
    if (forbidden.some((marker) => page.includes(marker))) {
      throw new Error(`Trang ${code} làm lộ nhãn điều phối`);
    }
    writeFileSync(join(dir, `${code}.html`), htmlPage(code, page, colors));
    if (theme === "light") {
      const { layers, view, width, height, paper, slice, ...entry } = c;
      key.push({ code, ...entry });
    }
  }
  writeFileSync(join(dir, "index.html"), htmlPage("Bộ hình", `<h1>Bộ hình</h1><p>Mở mã hình được người điều phối giao.</p><ul>${links.join("")}</ul>`, colors));
}
function htmlPage(title, body, colors) {
  return `<!doctype html><html lang="vi"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>${title}</title><style>body{font:18px/1.5 system-ui;margin:24px;background:${colors.ground};color:${colors.ink}}.art{display:block;max-width:100%;overflow:auto}.art svg{display:block;max-width:100%;height:auto}a{color:${colors.ink}}h1{font-size:20px}</style><body>${body}</body></html>\n`;
}
const keyMarkdown = [
  "# Đáp án điều phối — không phát cho người đọc",
  "",
  "| Mã | Ý nghĩa nội bộ | Biến thể |",
  "|---|---|---|",
  ...key.map(({ code, ...entry }) => `| ${code} | ${entry.answer} | ${entry.compact ? "gọn" : "đủ"}, ${entry.kind} |`),
  "",
  "Tệp này nằm ngoài `nguoi-doc/`; chỉ người điều phối mở sau khi ghi nhận câu trả lời.",
].join("\n") + "\n";
writeFileSync(join(out, "dap-an-dieu-phoi.md"), keyMarkdown);
console.log(`Đã xuất ${cases.length} mã × sáng/tối. Chỉ phát thư mục nguoi-doc; giữ đáp án riêng.`);
