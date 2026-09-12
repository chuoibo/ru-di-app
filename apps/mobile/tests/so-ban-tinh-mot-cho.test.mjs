/* Một chỗ khai khác biệt giữa các loại sổ.
 *
 * ## Vì sao cần một cổng, không cần một lời hứa
 *
 * Spec «Nếp truyền giấy» §13.3 nói: không có hai mode, có nhiều sổ, mỗi sổ
 * một loại; và mọi khác biệt giữa các loại (cách quyết định, có vai không,
 * nhịp Nếp nói, Nếp được làm gì, câu chữ, tiền hiện kiểu gì) sống ở ĐÚNG MỘT
 * module `src/rudi/so/ban-tinh.ts`. Lý do là bài học của repo này: một danh
 * sách tên và một thân hàm rẽ nhánh là hai hướng trôi dạt khác nhau, và sau
 * ba lát thì «sync» chỉ còn là chữ trong doc. Cổng này giữ nó thành cơ chế.
 *
 * ## Cổng này đo cái gì, và không đo cái gì
 *
 * Nó đo: không file nào ngoài `so/ban-tinh.ts` được so sánh loại sổ, dù là
 * `kind === "pair"` (loại thô của máy chủ) hay `loaiSo === "doi"` (loại đã
 * suy). Regex cố ý rộng và neo vào ĐỊNH DANH (`kind`, `loaiSo`), không neo vào
 * chuỗi trần: `src/rudi/art/ky-hoa.ts` có `san === "doi"` là sân khấu «đồi»
 * của ký hoạ, không phải loại sổ, và một regex bắt chuỗi trần sẽ đỏ sai ở đó.
 *
 * Nó KHÔNG đo quyền. Đây là chính sách trình bày: cái gì hiện, gọi tên gì.
 * Phân quyền là việc của máy chủ ở mọi biên đọc/ghi (ADR-0027 §4, Codex lượt
 * hai), và một nút hiện ra chưa cấp cho ai điều gì.
 *
 * ## Ba chỗ có sẵn được khai tường minh, và cổng kiểm cả hai chiều
 *
 * Trước module này, ba file đã so `kind === "pair"` cho việc của riêng chúng
 * (đặt tên cuộc nhắn riêng, ẩn «Rời nhóm», không chọn pair làm nhóm mặc định).
 * Chúng được khai vào `CO_SAN` thay vì gom vội — gom là sửa màn hội bạn, và
 * PR này không được đụng hội bạn. Nhưng một danh sách nguồn có thể rỗng dần
 * mà không ai biết (memory «danh sách nguồn RỖNG → cổng tự tháo»), nên cổng
 * đòi HAI chiều: file ngoài danh sách mà so sánh → đỏ; file trong danh sách mà
 * KHÔNG còn so sánh → cũng đỏ, để danh sách này không hoá thành di tích.
 */
import assert from "node:assert/strict";
import test from "node:test";
import { readFileSync, readdirSync } from "node:fs";
import { join, relative } from "node:path";
import { fileURLToPath } from "node:url";

const SRC = fileURLToPath(new URL("../src", import.meta.url));

/** Chỗ duy nhất được phép so loại sổ. */
const CHU_NHA = new Set(["rudi/so/ban-tinh.ts"]);

/** Ba chỗ so `kind === "pair"` có trước module này. Gom về `loaiSoCua` là việc của một PR đụng hội bạn, không phải PR này. */
const CO_SAN = new Set(["phien.ts", "rudi/nhan-rieng/nhan-rieng.ts", "rudi/screens/chat/CaiDatNhom.tsx"]);

// Cố ý rộng, neo vào định danh. `kind` là loại thô của máy chủ (ADR-0021);
// `loaiSo`/`loaiSoCua(...)` là loại đã suy từ module bản tính.
const SO_KIND = /\bkind\s*[!=]==?\s*["'](?:pair|group)["']|["'](?:pair|group)["']\s*[!=]==?\s*\w*\.?kind\b/;
const SO_LOAI = /\bloaiSo(?:Cua\([^)]*\))?\s*[!=]==?\s*["'](?:hoi|hai-nguoi|doi)["']|["'](?:hoi|hai-nguoi|doi)["']\s*[!=]==?\s*loaiSo\b/;

function nguon(dir) {
  const ra = [];
  for (const muc of readdirSync(dir, { withFileTypes: true })) {
    const p = join(dir, muc.name);
    if (muc.isDirectory()) ra.push(...nguon(p));
    else if (/\.tsx?$/.test(muc.name)) ra.push(p);
  }
  return ra.sort();
}

/** Bỏ comment: một comment nhắc tới `kind === "pair"` không phải một nhánh. */
function boComment(text) {
  return text.replace(/\/\*[\s\S]*?\*\//g, "").replace(/^[^\n]*?\/\/[^\n]*$/gm, "");
}

test("máy đọc còn nhìn: sàn số file và regex bắt được cả hai cách viết", () => {
  const files = nguon(SRC);
  assert.ok(files.length >= 60, `chỉ thấy ${files.length} file dưới src/ — máy quét đã mù`);
  assert.ok(SO_KIND.test('if (nhom.kind === "pair") {'));
  assert.ok(SO_KIND.test("const laPair = n?.kind !== 'group';"));
  assert.ok(SO_LOAI.test('loaiSo === "doi"'));
  assert.ok(SO_LOAI.test("if (loaiSoCua(nhom, doi) !== 'hoi')"));
  assert.equal(SO_LOAI.test('const than = san === "doi" && gon'), false, "chuỗi trần không phải loại sổ");
});

test("ngoài so/ban-tinh.ts và ba chỗ có sẵn, không file nào so loại sổ", () => {
  const pham = [];
  for (const duong of nguon(SRC)) {
    const ten = relative(SRC, duong);
    if (CHU_NHA.has(ten) || CO_SAN.has(ten)) continue;
    const than = boComment(readFileSync(duong, "utf8"));
    if (SO_KIND.test(than) || SO_LOAI.test(than)) pham.push(ten);
  }
  assert.deepEqual(
    pham,
    [],
    "những file này rẽ nhánh theo loại sổ thay vì đọc `banTinhCua(loaiSoCua(...))`:\n  " + pham.join("\n  "),
  );
});

test("ba chỗ có sẵn vẫn còn đó: danh sách khai không được rỗng dần trong im lặng", () => {
  for (const ten of CO_SAN) {
    const than = boComment(readFileSync(join(SRC, ten), "utf8"));
    assert.ok(SO_KIND.test(than), `${ten} không còn so kind === "pair": xoá nó khỏi CO_SAN, đừng để danh sách nói dối`);
  }
});

test("so/ban-tinh.ts là LÁ trong src/: không import gì để không kéo màn hình vào chính sách", () => {
  const than = readFileSync(join(SRC, "rudi/so/ban-tinh.ts"), "utf8");
  const noiBo = [...than.matchAll(/^\s*import\s[^;]*?from\s+["'](\.[^"']+)["']/gm)].map((m) => m[1]);
  assert.deepEqual(noiBo, [], `ban-tinh.ts phải là lá, nhưng import: ${noiBo.join(", ")}`);
});
