/* Cài đặt: phiên, quyền riêng tư, xoá tài khoản, giao diện (L5, ADR-0023).
 *
 * Chạy từ apps/mobile:
 *     npx tsc -p tsconfig.test.json && node --test tests/rudi-cai-dat.test.mjs
 *
 * Bốn luật đáng ghim, cả bốn đều là luật PHỦ ĐỊNH hoặc luật câu chữ: danh
 * sách phiên không được bịa ra tên thiết bị; thân báo cáo chỉ mang khoá có
 * giá trị; chỉ đúng một từ mở được cửa xoá tài khoản; và mọi giá trị lạ trên
 * đĩa đều rơi về «theo hệ thống» chứ không làm màn trắng.
 */
import assert from "node:assert/strict";
import test from "node:test";

import { BASE_URL, datTokenPhien, newAttempt } from "../dist-test/api.js";
import {
  cauPhien,
  cauTaiKhoanVuaTao,
  docPhien,
  nhanCua,
  thuHoiPhien,
} from "../dist-test/rudi/cai-dat/phien-cai-dat.js";
import {
  LY_DO_BAO_CAO,
  baoCao,
  boChan,
  chan,
  docDaChan,
  thanBaoCao,
} from "../dist-test/rudi/cai-dat/quyen-rieng-tu.js";
import {
  DIEU_SE_XAY_RA,
  TU_XAC_NHAN,
  xacNhanHopLe,
  xoaTaiKhoan,
} from "../dist-test/rudi/cai-dat/xoa-tai-khoan.js";
import { NHAN_GIAO_DIEN, docCheDoGiaoDien, toiHay } from "../dist-test/rudi/giao-dien.js";

const ME = "4dd00000-dddd-4ddd-8ddd-0000d0000021";
const KIA = "5ee00000-eeee-4eee-8eee-0000e0000021";

function traLoi(body, status = 200) {
  if (body === null) return new Response(null, { status });
  return new Response(JSON.stringify(body), { status, headers: { "content-type": "application/json" } });
}

async function batFetch(traVe, chay) {
  const daGoi = [];
  const truoc = globalThis.fetch;
  globalThis.fetch = async (url, init) => {
    daGoi.push({ url: String(url), init });
    return traVe(daGoi.length);
  };
  try {
    await chay();
  } finally {
    globalThis.fetch = truoc;
  }
  return daGoi;
}

test.afterEach(() => datTokenPhien(null));

test("danh sách phiên: đúng đường, mang bearer, và không bịa tên thiết bị", async () => {
  datTokenPhien("tok-cai-dat");
  const phien = {
    id: "p1",
    issued_via: "otp",
    created_at: "2030-08-27T12:00:00Z",
    expires_at: "2030-09-26T12:00:00Z",
    current: true,
  };
  const goi = await batFetch(
    () => traLoi({ sessions: [phien] }),
    async () => {
      const ds = await docPhien(ME);
      assert.equal(ds.sessions.length, 1);
    },
  );
  assert.equal(goi[0].url, `${BASE_URL}/sessions`);
  assert.equal(goi[0].init.method, "GET");
  assert.equal(goi[0].init.headers.Authorization, "Bearer tok-cai-dat");
  assert.equal(cauPhien(phien), "Phiên này, đang dùng");
  const khac = cauPhien({ ...phien, current: false });
  assert.ok(khac.startsWith("Số điện thoại · từ "), khac);
  assert.ok(!/iphone|android|máy/i.test(khac), "máy chủ không lưu tên thiết bị nên màn không nói");
});

test("nhãn cửa cấp phiên có đủ bốn cửa và một câu cho cửa lạ", () => {
  assert.equal(nhanCua("otp"), "Số điện thoại");
  assert.equal(nhanCua("google"), "Google");
  assert.equal(nhanCua("invite"), "Lời mời");
  assert.equal(nhanCua("genesis"), "Bản dựng");
  assert.equal(nhanCua("qua-cua-nao-do"), "Cách khác");
});

test("thu hồi phiên là DELETE đúng id và có khoá thử lại", async () => {
  datTokenPhien("tok");
  const goi = await batFetch(
    () => traLoi(null, 204),
    () => thuHoiPhien("p9", ME, newAttempt()),
  );
  assert.equal(goi[0].url, `${BASE_URL}/sessions/p9`);
  assert.equal(goi[0].init.method, "DELETE");
  assert.ok(goi[0].init.headers["Idempotency-Key"]);
});

test("thân báo cáo chỉ mang ghi chú khi có chữ", () => {
  assert.deepEqual(thanBaoCao("person", KIA, "spam", ""), {
    target_type: "person",
    target_id: KIA,
    reason: "spam",
  });
  assert.deepEqual(thanBaoCao("post", "p1", "other", "  vì sao  "), {
    target_type: "post",
    target_id: "p1",
    reason: "other",
    note: "vì sao",
  });
});

test("chặn, bỏ chặn, đọc danh sách và báo cáo đi đúng bốn đường", async () => {
  datTokenPhien("tok");
  const goi = await batFetch(
    (n) =>
      n === 3
        ? traLoi({ blocked: [] })
        : n === 4
          ? traLoi({ id: "r1", created_at: "2030-08-27T12:00:00Z" }, 201)
          : traLoi({ person_id: KIA, state: n === 1 ? "blocked" : "declined" }),
    async () => {
      await chan(KIA, ME, newAttempt());
      await boChan(KIA, ME, newAttempt());
      await docDaChan(ME);
      await baoCao("person", KIA, "harassment", "", ME, newAttempt());
    },
  );
  assert.deepEqual(
    goi.map((g) => [g.init.method, g.url.slice(BASE_URL.length)]),
    [
      ["POST", `/people/${KIA}/block`],
      ["DELETE", `/people/${KIA}/block`],
      ["GET", "/people/me/blocked"],
      ["POST", "/reports"],
    ],
  );
  assert.equal(JSON.parse(goi[3].init.body).note, undefined, "ghi chú rỗng không lên máy chủ");
});

test("lý do báo cáo là bộ đóng năm mã, khớp máy chủ", () => {
  assert.deepEqual(
    LY_DO_BAO_CAO.map((muc) => muc.ma),
    ["spam", "harassment", "inappropriate", "impersonation", "other"],
  );
  for (const muc of LY_DO_BAO_CAO) assert.ok(!muc.nhan.includes("—"), muc.ma);
});

test("cửa xoá tài khoản chỉ mở với đúng một từ", () => {
  assert.equal(TU_XAC_NHAN, "XOA");
  assert.equal(xacNhanHopLe("XOA"), true);
  assert.equal(xacNhanHopLe(" xoa "), true, "gõ thường và thừa khoảng trắng vẫn là từ ấy");
  for (const sai of ["", "XO", "XOAA", "DELETE", "xoá"]) {
    assert.equal(xacNhanHopLe(sai), false, sai);
  }
});

test("màn xoá nói cả điều khó nghe: sổ tiền không mất", () => {
  const cauTien = DIEU_SE_XAY_RA.find((cau) => cau.includes("Sổ tiền"));
  assert.ok(cauTien, "phải có một câu về sổ tiền");
  assert.ok(cauTien.includes("giữ nguyên"));
  for (const cau of DIEU_SE_XAY_RA) assert.ok(!cau.includes("—"), cau);
});

test("xoá tài khoản gửi confirm true và có khoá thử lại", async () => {
  datTokenPhien("tok");
  const goi = await batFetch(
    () => traLoi(null, 204),
    () => xoaTaiKhoan(ME, newAttempt()),
  );
  assert.equal(goi[0].url, `${BASE_URL}/people/me`);
  assert.equal(goi[0].init.method, "DELETE");
  assert.deepEqual(JSON.parse(goi[0].init.body), { confirm: true });
  assert.ok(goi[0].init.headers["Idempotency-Key"]);
});

test("giao diện: rác trên đĩa về «theo hệ thống», và sáu tổ hợp của toiHay", () => {
  assert.equal(docCheDoGiaoDien("toi"), "toi");
  assert.equal(docCheDoGiaoDien("sang"), "sang");
  assert.equal(docCheDoGiaoDien("he-thong"), "he-thong");
  for (const rac of [null, undefined, "", "dark", 7, {}]) {
    assert.equal(docCheDoGiaoDien(rac), "he-thong", String(rac));
  }
  assert.equal(toiHay("toi", false), true);
  assert.equal(toiHay("toi", true), true);
  assert.equal(toiHay("sang", true), false);
  assert.equal(toiHay("sang", false), false);
  assert.equal(toiHay("he-thong", true), true);
  assert.equal(toiHay("he-thong", false), false);
  assert.deepEqual(NHAN_GIAO_DIEN.map((m) => m.ma), ["sang", "toi", "he-thong"]);
});

test("câu «tài khoản vừa tạo» nói đúng cửa đã tạo nó, không mặc định là số điện thoại", () => {
  // Câu này chỉ hiện đúng một lần trong đời một tài khoản, ngay sau khi đăng
  // nhập lần đầu. Trước khi cửa Google chạy được thật thì mọi tài khoản đều
  // tới bằng số, nên câu ghi cứng «số điện thoại» chưa ai thấy sai; đo trên
  // máy 2026-09-09 với một tài khoản Google mới thì nó nói sai ngay dòng đầu.
  const google = cauTaiKhoanVuaTao("google");
  assert.match(google, /Google/);
  assert.doesNotMatch(google, /số điện thoại/i);

  const otp = cauTaiKhoanVuaTao("otp");
  assert.match(otp, /số điện thoại/i);
  assert.doesNotMatch(otp, /Google/);

  const loiMoi = cauTaiKhoanVuaTao("invite");
  assert.match(loiMoi, /lời mời/i);

  // Cửa mà bản này chưa biết: nói rằng tài khoản vừa được tạo, và KHÔNG
  // khẳng định cửa nào. Bịa một cửa còn tệ hơn im lặng về nó.
  const la = cauTaiKhoanVuaTao("qua-cua-nao-do");
  assert.doesNotMatch(la, /số điện thoại/i);
  assert.doesNotMatch(la, /Google/);

  // Mọi biến thể đều phải chỉ chỗ đổi tên hiển thị, vì đó là việc câu này làm.
  for (const cau of [google, otp, loiMoi, la]) assert.match(cau, /Cá nhân/);
});
