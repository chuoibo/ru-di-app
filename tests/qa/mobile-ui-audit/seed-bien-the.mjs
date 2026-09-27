/* Variant data for the audit, created through the product's own API as a
 * seeded persona. Synthetic text only; nothing here names a real person.
 *
 *   node seed-bien-the.mjs
 *
 * Idempotent: an outing whose title already exists in the group is reused, so a
 * second run creates nothing. Prints the ids the feature scripts read back
 * (they look the outings up by title, never by a remembered id).
 *
 * F03 needs what the seed world does not have:
 *  - KEO_DAI: a long title, one day, 12 stops with long labels, one label that is
 *    a single unbroken word, place names long and short.
 *  - KEO_RONG: two days and no stop at all (the empty-day states).
 * Scripts that write to these outings (attach a place, check in, add a stop)
 * call `datLaiChang` first, so every run starts from the same stops.
 */
import { pathToFileURL } from "node:url";

import { docMoiTruong } from "./thu-vien/moi-truong.mjs";
import { goiApi, layPhien, personaTheoTen } from "./thu-vien/phien.mjs";

export const KEO_DAI = "Chuyến săn mây Cầu Đất rồi về chợ đêm của cả nhóm, lịch dày từ sáng tới khuya để thử chữ dài";
export const KEO_RONG = "Kèo chưa có chặng nào";

export const CHANG_DAI = [
  ["06:00", "Tập trung ở sảnh khách sạn, kiểm tra áo mưa và sạc dự phòng", null, null],
  ["06:45", "Săn mây ở đồi chè Cầu Đất, chờ mặt trời lên khỏi dãy núi phía đông", null, null],
  ["08:15", "Cà phê sáng", "Lưng Chừng Cafe", "p-lung-chung-cafe"],
  ["09:30", "ChụpẢnhTậpThểỞCổngĐồiChèKhôngCóKhoảngTrắngNàoĐểNgắtDòng", null, null],
  ["10:30", "Làm gốm", "Sống Màu Workshop", "p-song-mau-workshop"],
  ["12:00", "Ăn trưa món địa phương, gọi thêm một nồi lẩu gà lá é cho cả nhóm tám người", "Lẩu Gà Lá É Tao Ngộ", null],
  ["13:30", "Nghỉ trưa", null, null],
  ["15:00", "Khu vui chơi", "Khu vui chơi DREAMpark", null],
  ["17:00", "Ngắm hoàng hôn", null, null],
  ["18:30", "Tối nướng", "Tiệm Nướng Xóm Lào", "p-tiem-nuong-xom-lao"],
  ["20:30", "Dạo chợ đêm, mua đặc sản mang về cho người ở nhà và uống sữa đậu nành nóng", "Chợ đêm Đà Lạt", null],
  ["22:00", "Về khách sạn", null, null],
];

const guiChang = (chang) => chang.map(([at, label, place_name, place_id]) => ({ at, label, place_name, place_id }));

/** Session, group id and the outings of the group, as the seeded persona. */
export async function moNhom(mt = docMoiTruong()) {
  const phien = await layPhien(mt.api, personaTheoTen("dalat-0", mt.chatSessions), mt.out);
  const token = phien.token;
  const toi = await goiApi(mt.api, "GET", "/people/me/contexts", undefined, token);
  const nhom = (toi.contexts ?? []).find((c) => c.kind === "group" && c.display_name === "Team Đà Lạt");
  const nhomId = process.env.AUDIT_NHOM_ID ?? nhom?.id;
  if (!nhomId) throw new Error("không tìm thấy nhóm của dalat-0; đặt AUDIT_NHOM_ID");
  const ds = await goiApi(mt.api, "GET", `/contexts/${nhomId}/outings`, undefined, token);
  return { mt, token, nhomId, keo: ds.outings ?? [] };
}

/** The variant outings, created when missing. */
export async function datKeoBienThe(mt = docMoiTruong()) {
  const n = await moNhom(mt);
  const tao = async (ten, tu, den, chang) => {
    let o = n.keo.find((k) => k.title === ten);
    if (!o) {
      o = await goiApi(mt.api, "POST", `/contexts/${n.nhomId}/outings`, { title: ten, starts_on: tu, ends_on: den, headcount: 8, budget_per_person_vnd: 450000 }, n.token);
      if (chang.length) o = await goiApi(mt.api, "PUT", `/outings/${o.id}/timeline`, { stops: guiChang(chang) }, n.token);
      console.log(`tạo ${o.id} «${ten.slice(0, 30)}…» ${chang.length} chặng`);
    }
    return o.id;
  };
  const dai = await tao(KEO_DAI, "2026-10-24", "2026-10-24", CHANG_DAI);
  const rong = await tao(KEO_RONG, "2026-10-31", "2026-11-01", []);
  const goc = n.keo.find((k) => k.title === "Đà Lạt cuối tuần")?.id ?? null;
  return { nhomId: n.nhomId, dai, rong, goc };
}

/** One SQL statement on the LOCAL stack; null when there is no database URL. */
async function chayPsql(sql) {
  const url = (process.env.MOBILE_DATABASE_URL ?? "").replace("postgresql+psycopg", "postgresql");
  if (!url) return null;
  const { spawnSync } = await import("node:child_process");
  const r = spawnSync("psql", [url, "-Atc", sql], { encoding: "utf8" });
  return r.status === 0 ? String(r.stdout) : null;
}

/**
 * Put a variant outing's stops back to the seeded list.
 *
 * Measured 27/09: replacing the timeline keeps the ids of unchanged stops, so
 * their check-ins survive it. There is no route that removes a check-in, so
 * those of the variant outing are deleted on the local stack (when its
 * database URL is set); elsewhere the check-in case reports what it finds.
 */
export async function datLaiChang(mt, id, chang) {
  const n = await moNhom(mt);
  await goiApi(mt.api, "PUT", `/outings/${id}/timeline`, { stops: guiChang(chang) }, n.token);
  const sach = id.replace(/[^0-9a-f-]/g, "");
  await chayPsql(`delete from outing_stop_checkins where stop_id in (select id from outing_stops where outing_id = '${sach}')`);
}

/**
 * Remove the outings the create-outing case made («Kèo thử…», by the audit
 * persona), so the plan list stays the seeded one for the next screenshots.
 * There is no delete route for outings, so this speaks SQL to the LOCAL stack
 * only, and only when MOBILE_DATABASE_URL is set; elsewhere it says so and
 * leaves the rows. Checked before use: nothing references these outings (no
 * stop, invite, vote, pair sheet or chat promotion) and creating one posts no
 * chat message.
 */
export async function donKeoThu(mt = docMoiTruong()) {
  const phien = await layPhien(mt.api, personaTheoTen("dalat-0", mt.chatSessions), mt.out);
  const ra = await chayPsql(`delete from outings where title like 'Kèo thử%' and created_by_id = '${phien.person_id.replace(/[^0-9a-f-]/g, "")}' and not exists (select 1 from outing_stops s where s.outing_id = outings.id)`);
  if (ra === null) {
    console.log("bỏ qua dọn kèo thử: không có MOBILE_DATABASE_URL hoặc psql lỗi");
    return 0;
  }
  const so = Number(ra.match(/DELETE (\d+)/)?.[1] ?? 0);
  console.log(`đã dọn ${so} kèo thử`);
  return so;
}

/**
 * F05: a long history in the chat-test group's legacy thread, which the chat
 * seed leaves empty (its messages went to the v2 lab). Forty synthetic lines
 * from four of its members, among them the shapes that break bubbles: a long
 * paragraph, an unbroken URL, several lines, emoji only. Idempotent: nothing
 * is sent when the thread already has forty legacy messages.
 */
export const CHAT_DAI = [
  "Tối nay ai rảnh không?",
  "Mình rảnh sau 8 giờ.",
  "Đi ăn lẩu nhé, quán cũ gần hồ.",
  "Ok, để mình gọi đặt bàn.",
  "Nhớ gọi thêm rau.",
  "Mình kể cho cả nhóm nghe chuyện hôm qua: đi từ sáng sớm, trời mưa lất phất suốt đoạn đèo, xe chết máy hai lần, cả bọn đẩy xe qua một con dốc dài rồi ngồi uống trà nóng ở một quán nhỏ ven đường, bà chủ quán còn cho thêm một đĩa khoai lang nướng, xong trời tạnh thì cả nhóm đi tiếp tới tận chiều mới tới nơi, mệt nhưng vui.",
  "https://example.com/chia-se/album/2026/chuyen-di-da-lat-cuoi-tuan-cua-ca-nhom-kiem-thu-khong-co-khoang-trang-nao-de-ngat-dong-abcdefghijklmnopqrstuvwxyz0123456789",
  "Dòng một\nDòng hai\nDòng ba\nDòng bốn",
  "😂😂😂",
  "Mai mấy giờ xuất phát?",
];

export async function datLichSuChat(mt = docMoiTruong()) {
  const { readFileSync } = await import("node:fs");
  const s = JSON.parse(readFileSync(mt.chatSessions, "utf8"));
  const phien = [];
  for (let i = 0; i < 4; i += 1) phien.push(await layPhien(mt.api, personaTheoTen(`chat-${i}`, mt.chatSessions), mt.out));
  const trang = await goiApi(mt.api, "GET", `/contexts/${s.groupId}/messages?limit=50`, undefined, phien[0].token);
  const co = (trang.messages ?? []).length;
  if (co >= 40) {
    console.log(`lịch sử chat đã có ${co} tin, không gửi thêm`);
    return { groupId: s.groupId, co };
  }
  for (let i = co; i < 40; i += 1) {
    const body = CHAT_DAI[i % CHAT_DAI.length];
    await goiApi(mt.api, "POST", `/contexts/${s.groupId}/messages`, { kind: "text", body, image_url: null, card: null }, phien[i % 4].token);
  }
  console.log(`đã gửi ${40 - co} tin vào nhóm chat-test`);
  return { groupId: s.groupId, co: 40 };
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  if (process.argv.includes("--don-keo-thu")) await donKeoThu();
  if (process.argv.includes("--chat")) await datLichSuChat();
  console.log(JSON.stringify(await datKeoBienThe()));
}
