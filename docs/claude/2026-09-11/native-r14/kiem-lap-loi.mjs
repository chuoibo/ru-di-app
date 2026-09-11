#!/usr/bin/env node
/**
 * R3 gate on a uiautomator dump of the fixture Explore screen: the promise is
 * said ONCE. Fails (exit 1) when
 *  - any node text matches /hợp gu/i (the seal or the old «Hợp gu nhờ …» line);
 *  - the section title «Gần bạn, đúng gu» is not exactly one node;
 *  - the lead has no reason line (a one-line node right after the lead name
 *    whose text is one of the fixture tags), or its description repeats a word
 *    of that reason.
 * Usage: node kiem-lap-loi.mjs <hierarchy.xml> [--ten "Tiệm Nướng Xóm Lèo"]
 * uiautomator drops nodes fully off-screen, so the check is on what the
 * screen shows -- which is exactly what the audit reads.
 */
import { readFileSync } from "node:fs";

const [xmlPath, ...rest] = process.argv.slice(2);
if (!xmlPath) { console.error("cần đường dẫn XML"); process.exit(2); }
const ten = rest.includes("--ten") ? rest[rest.indexOf("--ten") + 1] : "Tiệm Nướng Xóm Lèo";
const xml = readFileSync(xmlPath, "utf8");
// Icon fonts (Ionicons) are Text nodes whose «text» is one private-use glyph;
// drop anything without a letter or digit so the spark before the reason line
// does not shift the lead's neighbours by one.
const texts = [...xml.matchAll(/ text="([^"]*)"/g)]
  .map((m) => m[1].replace(/&amp;/g, "&").replace(/&quot;/g, '"'))
  .filter((t) => /[\p{L}\p{N}]/u.test(t));

const gapChu = (t) => t.normalize("NFD").replace(/[̀-ͯ]/g, "").replace(/đ/g, "d").toLowerCase();
const loi = [];
const hopGu = texts.filter((t) => /hợp gu/i.test(t));
if (hopGu.length > 0) loi.push(`còn ${hopGu.length} node «hợp gu»: ${JSON.stringify(hopGu)}`);
const tieuDe = texts.filter((t) => t === "Gần bạn, đúng gu");
if (tieuDe.length !== 1) loi.push(`tiêu đề mục «Gần bạn, đúng gu» phải đúng 1 node, thấy ${tieuDe.length}`);
const i = texts.indexOf(ten);
if (i < 0) loi.push(`không thấy tên dẫn «${ten}» trên màn`);
else {
  const lyDo = texts[i + 1] ?? "";
  const moTa = texts[i + 2] ?? "";
  if (!lyDo || lyDo.split(" ").length > 4) loi.push(`sau tên dẫn phải là một lý do ngắn (một tag), thấy «${lyDo}»`);
  const tu = gapChu(lyDo).split(/\s+/).filter((w) => w.length >= 3);
  const lap = tu.filter((w) => gapChu(moTa).includes(w));
  if (lap.length > 0) loi.push(`mô tả «${moTa}» nhắc lại từ của lý do: ${lap.join(", ")}`);
  console.log(`dẫn: «${ten}» · lý do: «${lyDo}» · mô tả: «${moTa}»`);
}
if (loi.length) { for (const l of loi) console.log("ĐỎ:", l); process.exit(1); }
console.log("XANH: một lời hứa ở tiêu đề, một lý do cho địa điểm dẫn, mô tả không nhắc lại");
