/* Nếp vẽ một tấm ảnh, đi hết đường thật: app -> API -> proxy -> Celery -> agy.
 *
 * Bốn tầng, mỗi tầng đã có test riêng xanh. File này tồn tại vì hai nửa xanh
 * nối lại vẫn hỏng được: job id do API đúc phải là id proxy nhận, `tenant` phải
 * là thứ proxy đếm hạn mức được, và bytes phải quay về qua đúng cổng kiểm quyền
 * sở hữu chứ không phải một URL trỏ thẳng vào chỗ vẽ.
 *
 * Chạy được khi có:
 *     cd services/api && NEP_PROXY_URL=http://127.0.0.1:20129 \
 *       NEP_PROXY_TOKEN=$(cat ~/.agy-proxy/api.token) \
 *       MOBILE_PERSON_ID_KEY=<>=32 ký tự> MOBILE_AUTH_MODE=dev \
 *       uvicorn app.api.main:app --port 8098
 *     và một worker: celery -A agy_proxy.media.celery_app worker -Q nep_anh
 *
 * Một lượt vẽ mất ~99 giây đo thật và ĐỐT MỘT LƯỢT QUOTA SEAT. Nên mặc định
 * file này dừng ở lúc nhận `job_id`; `NEP_E2E_CHO_ANH=1` mới chờ tới lúc có
 * bytes. Một cổng CI không được phép tiêu tiền thật mỗi lần ai đó mở PR.
 *
 * `MOBILE_REQUIRE_E2E=1` biến «không có máy chủ» thành một lần hỏng, đúng quy
 * ước của `vertical-slice.test.mjs`. «Có máy chủ nhưng chưa nối đường media» là
 * một chuyện khác, và chỉ hỏng khi `NEP_REQUIRE_MEDIA=1`.
 */
import assert from "node:assert/strict";
import test from "node:test";

import { BASE_URL } from "../../dist-test/api.js";
import { conPhaiHoi, duongFile, nhipHoiMs } from "../../dist-test/rudi/nep/media.js";
import { batPhienE2E } from "./phien-e2e.mjs";

/* Hai cờ, hai câu hỏi khác nhau, và gộp chúng là cách file này làm đỏ CI.
 *
 * `MOBILE_REQUIRE_E2E` nghĩa là «phải có MỘT MÁY CHỦ», và `scripts/gate.sh e2e`
 * đặt nó vì nó tự dựng PostgreSQL + uvicorn. Nó KHÔNG có nghĩa là phải có
 * đường media: đường đó cần thêm một proxy, một worker Celery và một tài khoản
 * agy, và bắt CI có đủ ba thứ đó là bắt CI đốt quota thật mỗi lần ai mở PR.
 *
 * Nên vắng máy chủ là HỎNG khi `MOBILE_REQUIRE_E2E=1`, còn vắng đường media chỉ
 * hỏng khi ai đó nói rõ `NEP_REQUIRE_MEDIA=1` — tức là trên máy đã dựng sẵn cả
 * chuỗi và muốn biết nó còn thông.
 */
const REQUIRED = Boolean(process.env.MOBILE_REQUIRE_E2E);
const REQUIRED_MEDIA = Boolean(process.env.NEP_REQUIRE_MEDIA);
const CHO_ANH = Boolean(process.env.NEP_E2E_CHO_ANH);
const MO_TA = "một chú gấu tím cầm tờ giấy gấp, vector phẳng, nền trắng";

/* Máy chủ của slice chạy `prod`, nên nó KHÔNG tin `X-Actor-ID` (ADR-0014).
 * `batPhienE2E` đọc `MOBILE_E2E_SESSIONS` mà `scripts/e2e_slice.sh` dựng rồi
 * gắn bearer thật theo actor. Bản đầu của file này tự bịa header actor và ăn
 * 401 trên CI; một e2e không tự xác thực được thì nó đang đo một máy chủ khác
 * với máy chủ người dùng gặp. */
// `null` nghĩa là đang chạy với máy chủ `dev`, thứ vẫn tin `X-Actor-ID` — đó là
// một đường chạy hợp lệ, không phải lỗi (xem `vertical-slice.test.mjs` §49).
const NGUOI = batPhienE2E() ?? "a1b2c3d4-e5f6-4a0b-8c1d-2e3f4a5b6c7d";

function headers() {
  return {
    "X-Actor-ID": NGUOI,
    "X-Actor-Roles": "member",
    "Content-Type": "application/json",
  };
}

async function serverIsUp() {
  try {
    const r = await fetch(`${BASE_URL}/healthz`, { signal: AbortSignal.timeout(2000) });
    return r.ok;
  } catch {
    return false;
  }
}

/**
 * Đường media có dùng được ở máy chủ này không.
 *
 * Thăm dò bằng GET trên một id ĐÚNG DẠNG nhưng không phải của ai. Bản đầu của
 * hàm này thăm dò bằng POST, và mỗi lần chạy e2e đẩy bốn job thật vào hàng rồi
 * đốt bốn lượt quota. Một phép thăm dò không được phép có tác dụng phụ, nhất là
 * tác dụng phụ mất tiền.
 *
 * Ba trạng thái, gộp lại là cách một cổng xanh giả:
 *   404 + "Not Found"    -> máy chủ không phục vụ route đó (Go core ở 8099:
 *                           route Nếp còn ở trạng thái PY trong manifest);
 *   404 + khong_thay_job -> CÓ route, và cổng sở hữu đang làm việc;
 *   503                  -> có route nhưng NEP_PROXY_URL/TOKEN chưa cấu hình.
 */
async function mediaSan() {
  const idDungDang = "0123456789abcdef-ThamDoKhongPhaiCuaAi";
  const r = await fetch(`${BASE_URL}/me/nep/media/${idDungDang}`, {
    headers: headers(),
    signal: AbortSignal.timeout(8000),
  });
  const than = await r.json().catch(() => ({}));
  // Fail-closed: CHỈ một câu trả lời duy nhất nghĩa là đường thông, còn lại đều
  // là không dùng được. Bản đầu làm ngược (mọi status lạ coi như thông) và 401
  // của máy chủ prod lọt qua, rồi ba bài kiểm phía sau đỏ vì một lý do chẳng
  // liên quan gì tới thứ chúng đo.
  if (r.status === 404 && than.detail === "khong_thay_job") return { san: true, vi_sao: null };
  if (r.status === 503) return { san: false, vi_sao: "chua-cau-hinh" };
  if (r.status === 404) return { san: false, vi_sao: "may-chu-khong-co-route" };
  if (r.status === 401 || r.status === 403) return { san: false, vi_sao: "khong-co-phien" };
  return { san: false, vi_sao: `status-la-${r.status}` };
}

async function boQua(t) {
  if (!(await serverIsUp())) {
    if (REQUIRED) assert.fail(`MOBILE_REQUIRE_E2E đặt rồi nhưng không có server tại ${BASE_URL}`);
    t.skip(`không có server tại ${BASE_URL}`);
    return true;
  }
  const { san, vi_sao } = await mediaSan();
  if (!san) {
    if (REQUIRED_MEDIA) assert.fail(`NEP_REQUIRE_MEDIA đặt rồi nhưng đường media không dùng được: ${vi_sao}`);
    t.skip(`đường media không dùng được ở server này: ${vi_sao}`);
    return true;
  }
  return false;
}

async function xinVe() {
  const r = await fetch(`${BASE_URL}/me/nep/media`, {
    method: "POST",
    headers: headers(),
    body: JSON.stringify({ loai: "anh", mo_ta: MO_TA }),
    signal: AbortSignal.timeout(20_000),
  });
  return { status: r.status, than: await r.json().catch(() => ({})) };
}

test("Nếp xin vẽ và nhận lại job id có bằng chứng sở hữu", async (t) => {
  if (await boQua(t)) return;

  const { status, than } = await xinVe();
  assert.equal(status, 202, `mong 202, nhận ${status}: ${JSON.stringify(than)}`);
  assert.equal(than.loai, "anh");
  assert.equal(than.trang_thai, "dang-cho");
  // Tiền tố HMAC 16 hex + phần ngẫu nhiên: bằng chứng sở hữu nằm trong chính id.
  assert.match(than.job_id, /^[a-f0-9]{16}-[A-Za-z0-9_-]{8,}$/, than.job_id);

  // Và chủ job đọc được job của mình.
  const doc = await fetch(`${BASE_URL}/me/nep/media/${than.job_id}`, { headers: headers() });
  assert.equal(doc.status, 200, "chủ job phải đọc được job của chính mình");
});

test("job của người khác trả 404, và không nói job đó có tồn tại hay không", async (t) => {
  if (await boQua(t)) return;
  const cuaNguoiKhac = "0123456789abcdef-KhongPhaiCuaToi";
  const r = await fetch(`${BASE_URL}/me/nep/media/${cuaNguoiKhac}`, { headers: headers() });
  assert.equal(r.status, 404, "403 là một câu trả lời; ở đây nó là câu trả lời sai người");
});

test("id bịa không qua được cổng sở hữu", async (t) => {
  if (await boQua(t)) return;
  for (const xau of ["abc", "khong-co-gach-dung-dang", "zzzzzzzzzzzzzzzz-aaaaaaaa"]) {
    const r = await fetch(`${BASE_URL}/me/nep/media/${encodeURIComponent(xau)}`, { headers: headers() });
    assert.equal(r.status, 404, xau);
  }
});

test("vẽ xong thì bytes quay về qua đúng cổng của app", { timeout: 300_000 }, async (t) => {
  if (await boQua(t)) return;
  if (!CHO_ANH) {
    t.skip("đặt NEP_E2E_CHO_ANH=1 để chờ tới lúc có ảnh (tốn một lượt quota)");
    return;
  }

  const { status, than } = await xinVe();
  assert.equal(status, 202);
  const job = than.job_id;

  let lan = 0;
  let tin = { trang_thai: "dang-cho" };
  while (conPhaiHoi(tin.trang_thai) && lan < 60) {
    await new Promise((r) => setTimeout(r, nhipHoiMs(lan)));
    const r = await fetch(`${BASE_URL}/me/nep/media/${job}`, { headers: headers() });
    assert.equal(r.status, 200);
    tin = await r.json();
    lan += 1;
  }
  assert.equal(tin.trang_thai, "xong", `vẽ không xong: ${JSON.stringify(tin)}`);

  const anh = await fetch(duongFile(BASE_URL, job), { headers: headers() });
  assert.equal(anh.status, 200);
  assert.match(anh.headers.get("content-type") ?? "", /^image\//);
  const bytes = new Uint8Array(await anh.arrayBuffer());
  assert.ok(bytes.length > 10_000, `ảnh quá nhỏ: ${bytes.length} byte`);
  // JPEG bắt đầu bằng ff d8 ff. Kiểm bytes thật chứ không tin header.
  assert.deepEqual([...bytes.slice(0, 3)], [0xff, 0xd8, 0xff]);
});
