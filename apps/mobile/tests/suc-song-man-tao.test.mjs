/**
 * Sức sống của màn tạo, màn tiền và sổ hai người (ADR-0037, kế hoạch UI v3 §7).
 *
 * QC 24/09 đọc các màn này là «một tờ điền chữ»: toàn ô có viền và chữ. Cổng
 * này đòi mỗi màn trong phạm vi render ÍT NHẤT một thứ của sân khấu giấy -- một
 * sân khấu, Nếp diễn, một vật giấy (hoá đơn, cuống, vé, phong bì, tem), một dấu,
 * hay một ô viết trên dòng kẻ / câu rủ / lịch xé / đĩa xoay -- đọc bằng AST nhẹ:
 * phải có một JSX `<Ten` của primitive được import từ `ui/`.
 *
 * Bắt đầu bằng DANH SÁCH NỢ (các màn chưa làm lại), và danh sách đó chỉ được CO
 * LẠI: màn nào đã có primitive thì phải gạch khỏi nợ, màn nào rơi khỏi phạm vi
 * phải được khai báo lại. Không chứng minh: màn ĐẸP. Chỉ chứng minh màn không
 * còn là tờ điền chữ trơn.
 */
import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import test from "node:test";

const GOC = new URL("../src/rudi/screens/", import.meta.url);

/** Every screen in scope: creation doors, money, the two-person notebook. */
const PHAM_VI = [
  "Create.tsx",
  "LoiMoi.tsx",
  "keo/CreateOutingLive.tsx",
  "groups/New.tsx",
  "groups/Invite.tsx",
  "friends/AddFriend.tsx",
  "chia-bill/ChiaBillLive.tsx",
  "dot-thu/DotThuLive.tsx",
  "Bill.tsx",
  "hai-nguoi/KhongGianGiay.tsx",
  "hai-nguoi/ToLoiRu.tsx",
  "hai-nguoi/DeNghiSua.tsx",
  "hai-nguoi/DongYBac.tsx",
  "hai-nguoi/ChonNguoi.tsx",
];

/** Screens not redone yet. Strike each one off in the slice that redoes it. */
const NO = [
  "Create.tsx",
  "LoiMoi.tsx",
  "keo/CreateOutingLive.tsx",
  "groups/New.tsx",
  "groups/Invite.tsx",
  "friends/AddFriend.tsx",
  "hai-nguoi/KhongGianGiay.tsx",
  "hai-nguoi/ToLoiRu.tsx",
  "hai-nguoi/DeNghiSua.tsx",
  "hai-nguoi/DongYBac.tsx",
  "hai-nguoi/ChonNguoi.tsx",
];

/** The paper stage's primitives, by the module they live in. */
const PRIMITIVE = {
  SanKhau: "SanKhau",
  CanhGap: "CanhGap",
  NepDien: "NepDien",
  HoaDonGiay: "HoaDonGiay",
  CuongPhieu: "CuongPhieu",
  TheVe: "TheVe",
  PhongBi: "PhongBi",
  Tem: "Tem",
  DauLon: "DauLon",
  StampButton: "StampButton",
  ONhapMuc: "ONhapMuc",
  CauRu: "CauRu",
  ChonNgayLich: "ChonNgayLich",
  BanXoay: "BanXoay",
  LatTrang: "LatTrang",
  HinhNhan: "Avatar",
  ToGiay: "ToGiay",
  // S1, the money screens: the ledger page, the bill table, the settlement's arrows.
  TrangSo: "TrangSo",
  BanGanMon: "BanGanMon",
  BanAn: "BanAn",
  SoDoChuyen: "SoDoChuyen",
  DaiTienDo: "DaiTienDo",
};

/** Strip comments and string bodies, so a primitive named in prose does not count. */
function boChuVaChuThich(nguon) {
  return nguon
    .replace(/\/\*[\s\S]*?\*\//g, "")
    .replace(/(^|[^:])\/\/.*$/gm, "$1")
    .replace(/"(?:[^"\\\n]|\\.)*"|'(?:[^'\\\n]|\\.)*'|`(?:[^`\\]|\\.)*`/g, '""');
}

function coPrimitive(tep) {
  const tho = readFileSync(new URL(tep, GOC), "utf8");
  const ma = boChuVaChuThich(tho);
  for (const [ten, modun] of Object.entries(PRIMITIVE)) {
    const nhap = new RegExp(`import\\s*\\{[^}]*\\b${ten}\\b[^}]*\\}\\s*from\\s*["'][./]*(?:rudi/)?ui/${modun}["']`).test(tho);
    const ve = new RegExp(`<${ten}[\\s/>]`).test(ma);
    if (nhap && ve) return ten;
  }
  return null;
}

test("mọi màn trong phạm vi có thật", () => {
  for (const tep of PHAM_VI) assert.ok(existsSync(new URL(tep, GOC)), `${tep} không còn: khai báo lại phạm vi`);
  for (const tep of NO) assert.ok(PHAM_VI.includes(tep), `${tep} trong danh sách nợ mà không trong phạm vi`);
});

test("màn ngoài danh sách nợ render ít nhất một primitive của sân khấu giấy", () => {
  for (const tep of PHAM_VI.filter((t) => !NO.includes(t))) {
    assert.ok(coPrimitive(tep), `${tep}: không render primitive nào của sân khấu giấy (vẫn là tờ điền chữ)`);
  }
});

test("danh sách nợ chỉ co lại: màn đã có primitive phải được gạch khỏi nợ", () => {
  const daXong = NO.filter((t) => coPrimitive(t));
  assert.deepEqual(daXong, [], `đã có primitive, gạch khỏi NO: ${daXong.join(", ")}`);
});

test("bộ đọc: import thật và thẻ JSX thật mới tính; nhắc trong chú thích hay chuỗi thì không", () => {
  assert.equal(boChuVaChuThich('// <HoaDonGiay>\nconst a = "<TheVe />";').includes("<"), false);
});
