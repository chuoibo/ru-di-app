/* Mọi mã từ chối của đường gọi AI có một câu người đọc, và câu đó không nói dối.
 *
 * ## Vì sao file này tồn tại
 *
 * Bất biến này từng nằm ở `cau-chu-im-lang.test.mjs`, gác từ vựng `reason` của
 * `POST /contexts/{id}/ai-turn`. ADR-0036 §2.1 xoá route đó cùng mọi đường AI
 * tự kích hoạt; đường duy nhất còn lại là hàng đợi lời gọi của
 * `services/core/internal/chatassist`. ADR-0036 §3b ghi rõ: đừng xoá một cổng
 * chất lượng dưới danh nghĩa dọn dẹp, bất biến phải chuyển sang chỗ mới TRƯỚC.
 * Đây là chỗ mới.
 *
 * ## Cổng này đo cái gì
 *
 * 1. **Từ vựng đọc từ mã Go, không viết tay.** Ca này đọc mọi file không phải
 *    test trong `chatassist/`, gom mọi mã ở ba dạng phát ra có trong gói
 *    (`refuse(w, NNN, "mã")`, `&denied{NNN, "mã"}`, `invalid("mã")`), rồi đi đồ
 *    thị gọi hàm theo tên từ đúng các handler mà client gọi qua `LOI_GOI_AI`.
 *    Danh sách handler cũng không viết tay: nó đọc đường dẫn trong
 *    `src/rudi/chat/ai-invocations.ts` (những lời gọi dùng bảng `LOI_GOI_AI`)
 *    rồi khớp với `h.mux.HandleFunc(...)` trong `handler.go`. Máy chủ thêm một
 *    mã trên đường đó thì file này đỏ, chứ không phải người dùng đọc câu chung
 *    nói rằng lỗi là của app.
 * 2. **Miễn trừ có tên và có lý do.** Mã nào client không bao giờ thấy thì nằm
 *    trong `MIEN_TRU` kèm một câu vì sao. Miễn trừ cho một mã không còn phát ra,
 *    hoặc cho một mã đã có câu, cũng làm cổng đỏ: một lối thoát không ai đối
 *    chiếu thì không còn là lối thoát có tên.
 * 3. **Không câu chết.** Mỗi khoá của `LOI_GOI_AI` phải là một mã đường ấy thật
 *    sự phát ra.
 * 4. **Giọng người**, giữ nguyên luật của file cũ: không chữ máy, không mã,
 *    không «lỗi», không mã HTTP, không gạch dài, đủ dài để nói được việc gì,
 *    và không hai mã nào đọc ra cùng một câu trừ khi chúng là cùng một sự kiện
 *    với người đọc (xem `CUNG_SU_KIEN`).
 *
 * ## Nó KHÔNG chứng minh
 *
 * Rằng câu tới được màn hình, rằng phân tích theo tên hàm không bỏ sót một lời
 * gọi động (nó xấp xỉ TRÊN: hai hàm cùng tên đều bị coi là với tới được), hay
 * rằng người thật hiểu câu chữ.
 *
 * Chạy từ apps/mobile:
 *
 *     tsc -p tsconfig.test.json && node tools/fixup-esm.mjs
 *     node --test tests/cau-chu-goi-ai.test.mjs
 */
import assert from "node:assert/strict";
import { readFileSync, readdirSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";

import { LOI_GOI_AI } from "../dist-test/rudi/chat/ai-invocations.js";

const HERE = dirname(fileURLToPath(import.meta.url));
const GOC_APP = join(HERE, "..");
const GOC_REPO = join(GOC_APP, "..", "..");
const CHATASSIST = join(GOC_REPO, "services", "core", "internal", "chatassist");

/**
 * Mã phát ra trên đường client gọi nhưng client không bao giờ thấy. Mỗi dòng
 * một lý do; câu chung của `thongDiepNguoiDoc` («đây là lỗi của app») là câu
 * đúng cho chúng, vì chỉ một bản app hỏng mới gửi được yêu cầu như vậy.
 */
const MIEN_TRU = {
  json_required:
    "translatedAsActor luôn gửi Content-Type: application/json; chỉ một client khác mới chạm được mã này.",
  invalid_body:
    "Thân yêu cầu dựng từ đúng các trường máy chủ đọc (logical_id, command, prompt, boi_canh); thân thừa trường hay sai JSON là app hỏng.",
  invalid_context:
    "contextId lấy từ chính nhóm đang mở, luôn là UUID; không có gì người dùng gõ đi vào đường dẫn.",
  invocation_already_published:
    "Chỉ nhánh cancel của mutate phát ra mã này; client không có lời gọi cancel nào. Đồ thị gọi theo tên xấp xỉ trên nên mới kéo nó vào qua retry.",
};

/**
 * Hai mã chung một câu là được khi, với người đọc, chúng là cùng một sự kiện.
 * `provider_unavailable` (không gọi được bên mô hình) và `chat_ai_unavailable`
 * (máy chủ không phân loại được lỗi) đều nghĩa là «AI chưa sẵn sàng, việc của
 * bạn vẫn làm được», và máy chủ cố ý không nói thêm.
 */
const CUNG_SU_KIEN = [["chat_ai_unavailable", "provider_unavailable"]];

/* ------------------------------------------------ đọc mã Go -------------- */

function nguonGo() {
  const tep = readdirSync(CHATASSIST).filter((ten) => ten.endsWith(".go") && !ten.endsWith("_test.go"));
  assert.ok(tep.length >= 3, `chỉ thấy ${tep.length} file Go trong chatassist, cổng đang đọc sai chỗ`);
  return tep.map((ten) => ({ ten, nguon: readFileSync(join(CHATASSIST, ten), "utf8") }));
}

/** Top-level functions, by the gofmt shape: `func` at column 0, `}` at column 0. */
function cacHam(nguon) {
  const dong = nguon.split("\n");
  const ham = [];
  for (let i = 0; i < dong.length; i++) {
    const dau = /^func\s+(\([^)]*\)\s*)?([A-Za-z_]\w*)\s*\(/.exec(dong[i]);
    if (!dau) continue;
    let cuoi = i;
    // A one-line body ends on its own line; anything else ends at column-0 `}`.
    if (!/\{.*\}\s*$/.test(dong[i]) || /\{\s*$/.test(dong[i])) {
      while (cuoi < dong.length && dong[cuoi] !== "}") cuoi++;
    }
    ham.push({ ten: dau[2], laMethod: Boolean(dau[1]), than: dong.slice(i, cuoi + 1).join("\n") });
    i = cuoi;
  }
  return ham;
}

const DANG_PHAT = [
  /\brefuse\(\s*w\s*,\s*\d{3}\s*,\s*"([a-z_]+)"\s*\)/g,
  /&denied\{\s*\d{3}\s*,\s*"([a-z_]+)"\s*\}/g,
  /\binvalid\(\s*"([a-z_]+)"\s*\)/g,
];

function maTrong(than) {
  const ma = new Set();
  for (const dang of DANG_PHAT) for (const m of than.matchAll(dang)) ma.add(m[1]);
  return ma;
}

/**
 * Methods and plain functions live in two name spaces: `x.name(` can only be
 * a method, a bare `name(` only a function. Without that split the local
 * `cancel()` of a `context.WithTimeout` resolves to the `cancel` HANDLER and
 * drags every code of `mutate` onto the create path.
 */
function doThiGo() {
  const doThi = { method: new Map(), ham: new Map() };
  for (const { nguon } of nguonGo()) {
    for (const ham of cacHam(nguon)) {
      const bang = ham.laMethod ? doThi.method : doThi.ham;
      const cu = bang.get(ham.ten);
      // Same name twice in one space: merge, i.e. over-approximate.
      bang.set(ham.ten, cu ? { ...ham, than: `${cu.than}\n${ham.than}` } : ham);
    }
  }
  return doThi;
}

function maVoiToiDuoc(doThi, goc) {
  const daThay = new Set();
  const hang = goc.map((ten) => `method:${ten}`);
  const ma = new Set();
  while (hang.length) {
    const khoa = hang.pop();
    if (daThay.has(khoa)) continue;
    daThay.add(khoa);
    const [loai, ten] = khoa.split(":");
    const ham = doThi[loai].get(ten);
    assert.ok(ham, `không thấy ${loai} ${ten} trong chatassist`);
    for (const m of maTrong(ham.than)) ma.add(m);
    for (const goi of ham.than.matchAll(/(\.)?\b([A-Za-z_]\w*)\s*\(/g)) {
      const loaiGoi = goi[1] ? "method" : "ham";
      if (doThi[loaiGoi].has(goi[2])) hang.push(`${loaiGoi}:${goi[2]}`);
    }
  }
  return ma;
}

/* ------------------------------------------------ nối client với máy chủ - */

const chuanHoa = (duong) => duong.replace(/\$\{[^}]*\}|\{[^}]*\}/g, "{}");

/** Handler names the client reaches through `LOI_GOI_AI`, read from both sides. */
function gocTuClient() {
  const client = readFileSync(join(GOC_APP, "src", "rudi", "chat", "ai-invocations.ts"), "utf8");
  const duongClient = [...client.matchAll(/translatedAsActor<[^>]*>\(\s*LOI_GOI_AI\s*,\s*`([^`]+)`/g)].map((m) => chuanHoa(m[1]));
  assert.ok(duongClient.length >= 1, "không thấy lời gọi nào dùng LOI_GOI_AI trong ai-invocations.ts");
  const handler = readFileSync(join(CHATASSIST, "handler.go"), "utf8");
  const dangKy = new Map();
  for (const m of handler.matchAll(/h\.mux\.HandleFunc\("POST (\S+)",\s*h\.(\w+)\)/g)) dangKy.set(chuanHoa(m[1]), m[2]);
  return duongClient.map((duong) => {
    const ten = dangKy.get(duong);
    assert.ok(ten, `client gọi POST ${duong} qua LOI_GOI_AI nhưng handler.go không đăng ký route đó`);
    return ten;
  });
}

function tuVung() {
  const goc = gocTuClient();
  const doThi = doThiGo();
  const moiMa = new Set();
  for (const bang of [doThi.method, doThi.ham]) for (const ham of bang.values()) for (const m of maTrong(ham.than)) moiMa.add(m);
  return { goc, moiMa, voiToi: maVoiToiDuoc(doThi, goc) };
}

/* ------------------------------------------------ 1. đủ câu -------------- */

test("mọi mã từ chối trên đường gọi AI đều có câu, hoặc có tên trong miễn trừ", () => {
  const { goc, moiMa, voiToi } = tuVung();
  console.log(`  handler gốc: ${goc.join(", ")}; gói phát ${moiMa.size} mã, đường gọi AI với tới ${voiToi.size}`);
  assert.ok(moiMa.size >= 20, `chỉ đọc được ${moiMa.size} mã trong cả gói, bộ đọc đang hỏng`);
  const thieu = [...voiToi].filter((ma) => !(ma in LOI_GOI_AI) && !(ma in MIEN_TRU)).sort();
  assert.deepEqual(
    thieu,
    [],
    `máy chủ phát ${thieu.join(", ")} trên đường gọi AI mà LOI_GOI_AI không có câu nào. ` +
      `Thêm câu vào src/rudi/chat/ai-invocations.ts, đừng để nó rơi vào câu chung.`,
  );
});

test("miễn trừ chỉ dành cho mã còn phát ra và chưa có câu", () => {
  const { voiToi } = tuVung();
  for (const [ma, lyDo] of Object.entries(MIEN_TRU)) {
    assert.ok(voiToi.has(ma), `miễn trừ cho ${ma} nhưng đường gọi AI không còn phát mã đó`);
    assert.equal(ma in LOI_GOI_AI, false, `${ma} vừa có câu vừa được miễn trừ`);
    assert.ok(lyDo.trim().length > 20, `miễn trừ ${ma} không nói vì sao`);
  }
});

test("không có câu chết: mỗi khoá của LOI_GOI_AI là một mã thật sự phát ra", () => {
  const { voiToi } = tuVung();
  const thua = Object.keys(LOI_GOI_AI).filter((ma) => !voiToi.has(ma));
  assert.deepEqual(thua, [], `câu chữ cho mã đường gọi AI không phát: ${thua.join(", ")}`);
});

/* ------------------------------------------------ 2. phân biệt được ------ */

test("không hai mã nào dùng chung một câu, trừ khi là cùng một sự kiện", () => {
  const choPhep = new Set(CUNG_SU_KIEN.map((nhom) => [...nhom].sort().join("|")));
  const theoCau = new Map();
  for (const [ma, cau] of Object.entries(LOI_GOI_AI)) theoCau.set(cau, [...(theoCau.get(cau) ?? []), ma]);
  const dungChung = [...theoCau.values()]
    .filter((ds) => ds.length > 1)
    .filter((ds) => !choPhep.has([...ds].sort().join("|")));
  assert.deepEqual(dungChung, [], `những mã này đọc ra cùng một câu: ${JSON.stringify(dungChung)}`);
});

/* ------------------------------------------------ 3. giọng người --------- */

test("không câu nào lộ chữ của máy hay viết như báo lỗi", () => {
  const tenMay = [...Object.keys(LOI_GOI_AI), ...Object.keys(MIEN_TRU), "code", "status", "invocation"];
  for (const cau of Object.values(LOI_GOI_AI)) {
    for (const ten of tenMay) {
      assert.equal(cau.includes(ten), false, `câu chữ lộ tên máy "${ten}": ${cau}`);
    }
    assert.doesNotMatch(cau, /[a-z]+_[a-z_]+/, `câu chữ chứa một mã máy: ${cau}`);
    assert.doesNotMatch(cau, /lỗi/i, `câu viết như báo lỗi: ${cau}`);
    assert.doesNotMatch(cau, /\b(4\d\d|5\d\d)\b/, `câu in mã trạng thái HTTP: ${cau}`);
    assert.doesNotMatch(cau, /HTTP/i, `câu nhắc HTTP: ${cau}`);
    assert.doesNotMatch(cau, /[—–]/, `câu dùng gạch dài: ${cau}`);
    assert.ok(cau.trim().length > 20, `câu quá ngắn để nói được việc gì: ${cau}`);
  }
});

/* ------------------------------------------------ 4. lệnh cũ không đi lên - */

test("lệnh AI cũ gõ trong khung chat mở khay AI, không đi lên POST /messages", () => {
  // ADR-0036 §2.1 turned `/plan`, `@Rủ Đi` and `/chia-bill` into ordinary text
  // on the server. The client must keep catching them locally: sent as text,
  // the group would read a bare command where the person meant to ask the AI.
  const nguon = readFileSync(join(GOC_APP, "src", "rudi", "screens", "chat", "GroupChatLive.tsx"), "utf8");
  const ham = /function goiMoHinh\(body: string\): boolean \{\n([\s\S]*?)\n\}/.exec(nguon);
  assert.ok(ham, "không thấy goiMoHinh trong GroupChatLive.tsx");
  const goiMoHinh = new Function("body", ham[1]);
  for (const lenh of ["/plan tối nay đi đâu", "/PLAN", "/chia-bill", "/chiabill", "@Rủ Đi gợi ý quán", "@rudi ơi", "@ru di"]) {
    assert.equal(goiMoHinh(lenh), true, `«${lenh}» sẽ đi lên máy chủ như tin thường`);
  }
  for (const thuong of ["/vote Ăn gì? Phở | Bún", "tối nay ăn gì", "/planning"]) {
    assert.equal(goiMoHinh(thuong), false, `«${thuong}» bị chặn nhầm như lệnh AI`);
  }
  const gui = nguon.slice(nguon.indexOf("const gui = async (command?: string)"));
  const chan = gui.indexOf("if (goiMoHinh(body)) {");
  const diLen = gui.indexOf("chat.gui(body");
  assert.ok(chan > 0 && diLen > 0 && chan < diLen, "gui() phải chặn lệnh AI trước khi gọi chat.gui");
  assert.match(gui.slice(chan, diLen), /return false;/, "nhánh lệnh AI phải dừng, không rơi xuống chat.gui");
});
