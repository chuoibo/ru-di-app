/* Self-check of the detectors in lib/do-dac.mjs and lib/lop-phu.mjs.
 *
 * A detector that cannot go red proves nothing when it stays green, so every
 * signal family gets a CANARY page with the defect planted (must fire) and the
 * IDENTITY page (must stay silent), plus the two look-alikes the audit meets
 * all the time and must not report: a pager's off-screen slide and a chip row
 * that scrolls sideways.
 *
 *   node tu-kiem.mjs              run the table against lib/do-dac.mjs
 *   node tu-kiem.mjs --dot-bien   also run two self-chosen mutants of it; each
 *                                 must turn exactly its predicted rows red
 *
 * Mutants run on a copy written to the OS temp dir; the source is never
 * touched. Equivalence was checked before choosing them: each changes a
 * comparison that some canary crosses, so neither can be a no-op.
 */
import { mkdtempSync, readFileSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

import { moTrinhDuyet } from "./lib/trinh-duyet.mjs";
import { soLuoi } from "./lib/lop-phu.mjs";

const HERE = fileURLToPath(new URL(".", import.meta.url));

const khung = (than) => `<!doctype html><html lang="vi"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<style>body{margin:0;font:16px/1.4 sans-serif}button{font:inherit}</style></head><body><main style="padding:16px">${than}</main></body></html>`;

const IDENTITY = khung(`<h1>Tiêu đề</h1><p>Một đoạn văn bình thường, vừa khung.</p>
<button aria-label="Lưu" style="width:48px;height:48px">✓</button>`);

// name, page, predicate on the summary, and on the full result where needed.
const CA = [
  ["identity: im lặng", IDENTITY, (t) => Object.entries(t).every(([k, v]) => ["renderer", "nepCachTien"].includes(k) || v === 0)],
  ["tràn ngang: phần tử 500px", khung(`<div style="width:500px;height:20px;background:#ddd">rộng quá</div>`), (t) => t.tranNgang >= 1 && t.tranTrangPx > 0],
  ["tràn ngang: hàng chip cuộn ngang KHÔNG báo", khung(`<div style="overflow-x:auto;white-space:nowrap"><span style="display:inline-block;width:900px">chip chip chip</span></div>`), (t) => t.tranNgang === 0 && t.tranTrangPx === 0],
  ["chữ tự cắt không ellipsis", khung(`<div style="width:100px;overflow:hidden;white-space:nowrap">Một câu rất dài bị cắt cụt mất đuôi</div>`), (t, k) => k.chuBiCat.some((c) => c.kieu === "tu-cat") && t.ellipsis === 0],
  ["ellipsis tách riêng", khung(`<div style="width:100px;overflow:hidden;white-space:nowrap;text-overflow:ellipsis">Một câu rất dài có dấu ba chấm</div>`), (t) => t.ellipsis === 1 && t.chuBiCat === 0],
  ["cha cắt một phần", khung(`<div style="height:20px;overflow:hidden"><div style="font-size:30px;line-height:40px">Chữ cao bị cắt</div></div>`), (t, k) => k.chuBiCat.some((c) => c.kieu === "cha-cat")],
  ["slide pager nằm hẳn ngoài KHÔNG báo", khung(`<div style="width:300px;overflow:hidden;white-space:nowrap"><div style="display:inline-block;width:300px">Trang một</div><div style="display:inline-block;width:300px">Trang hai</div></div>`), (t) => t.chuBiCat === 0],
  ["chữ bị che bởi lớp đục", khung(`<div style="position:relative"><p>Chữ nằm dưới</p><div style="position:absolute;inset:0;background:#fff"></div></div>`), (t) => t.cheChu >= 1],
  ["lớp trong suốt chặn chạm", khung(`<div style="position:relative"><p>Chữ nằm dưới lớp vô hình</p><div style="position:absolute;inset:0"></div></div>`), (t) => t.phuTrongSuot >= 1 && t.cheChu === 0],
  ["placeholder vẽ dưới input KHÔNG báo", khung(`<div style="position:relative;height:44px"><div style="position:absolute;inset:0">Số di động của bạn</div><input aria-label="Ô số" style="position:absolute;inset:0;background:transparent;border:0;width:100%"></div>`), (t) => t.phuTrongSuot === 0 && t.cheChu === 0],
  ["vùng bấm 30px", khung(`<button aria-label="Nhỏ" style="width:30px;height:30px;padding:0">x</button>`), (t) => t.vungBamNho48 === 1 && t.vungBamNho44 === 1],
  ["nút không tên", khung(`<button style="width:48px;height:48px"></button>`), (t) => t.khongTen === 1],
  ["dấu gạch dài", khung(`<p>Đi chơi — về sớm</p>`), (t) => t.gachDai === 1],
  ["mã lỗi tiếng Anh", khung(`<p>audit_injected_failure</p>`), (t) => t.maLoi === 1],
];

async function chayBang(browser, duongModule) {
  const { doDac, tomTat, luoiChamTrang } = await import(pathToFileURL(duongModule).href);
  const ctx = await browser.newContext({ viewport: { width: 390, height: 844 }, deviceScaleFactor: 1 });
  const page = await ctx.newPage();
  const ket = [];
  for (const [ten, html, dung] of CA) {
    await page.setContent(html, { waitUntil: "load" });
    const k = await doDac(page);
    const t = tomTat(k);
    let ok = false;
    try {
      ok = !!dung(t, k);
    } catch {
      ok = false;
    }
    ket.push({ ten, ok, t });
  }
  // Residue: a transparent full-screen layer left after "closing" must change the tap grid.
  await page.setContent(khung(`<button aria-label="Mở" style="width:120px;height:48px">Mở</button><p>Nội dung</p>`), { waitUntil: "load" });
  const truoc = await luoiChamTrang(page);
  await page.evaluate(() => {
    const d = document.createElement("div");
    d.setAttribute("data-testid", "tang-vo-hinh");
    d.style.cssText = "position:fixed;inset:0;opacity:0";
    document.body.appendChild(d);
  });
  const sau = await luoiChamTrang(page);
  ket.push({ ten: "lớp sót sau khi đóng đổi lưới chạm", ok: soLuoi(truoc, sau).length > 0, t: null });
  await page.evaluate(() => document.querySelector('[data-testid="tang-vo-hinh"]').remove());
  const sau2 = await luoiChamTrang(page);
  ket.push({ ten: "không sót thì lưới chạm giữ nguyên", ok: soLuoi(truoc, sau2).length === 0, t: null });
  await ctx.close();
  return ket;
}

const DOT_BIEN = [
  {
    ten: "M1 bỏ ngưỡng mép phải của tràn ngang",
    tu: "if (r.right <= vw + 1 && r.left >= -1) continue;",
    thanh: "if (r.right <= vw + 1000 && r.left >= -1) continue;",
    duDoanDo: ["tràn ngang: phần tử 500px"],
  },
  {
    ten: "M2 coi mọi nền là trong suốt khi xét che chữ",
    tu: "if (alpha > 0.3 ||",
    thanh: "if (alpha > 1.1 ||",
    duDoanDo: ["chữ bị che bởi lớp đục"],
  },
];

const browser = await moTrinhDuyet();
let loi = 0;
try {
  const goc = join(HERE, "lib", "do-dac.mjs");
  const ket = await chayBang(browser, goc);
  for (const k of ket) {
    console.log(`${k.ok ? "XANH" : "ĐỎ  "}  ${k.ten}`);
    if (!k.ok) loi++;
  }
  console.log(`bảng gốc: ${ket.length - loi}/${ket.length} xanh`);
  if (process.argv.includes("--dot-bien")) {
    const nguon = readFileSync(goc, "utf8");
    const tam = mkdtempSync(join(tmpdir(), "audit-dot-bien-"));
    for (const db of DOT_BIEN) {
      if (!nguon.includes(db.tu)) throw new Error(`${db.ten}: không thấy đoạn cần đổi`);
      // The mutant imports nothing relative, so it can live outside the tree.
      const file = join(tam, `${db.ten.slice(0, 2)}-do-dac.mjs`);
      writeFileSync(file, nguon.replace(db.tu, db.thanh));
      const kq = await chayBang(browser, file);
      const do_ = kq.filter((k) => !k.ok).map((k) => k.ten);
      const dung = do_.length === db.duDoanDo.length && db.duDoanDo.every((t) => do_.includes(t));
      console.log(`${dung ? "ĐÚNG DỰ ĐOÁN" : "SAI DỰ ĐOÁN"}  ${db.ten}: đỏ = [${do_.join(" | ")}]`);
      if (!dung) loi++;
    }
  }
} finally {
  await browser.close();
}
process.exit(loi ? 1 : 0);
