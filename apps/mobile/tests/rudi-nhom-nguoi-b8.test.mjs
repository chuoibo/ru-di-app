// Groups, people and the two-person notebook (QA F06, F07, N26: UI-071, UI-073,
// UI-074, UI-075, UI-076, UI-077, UI-078, UI-080, UI-081, UI-083, UI-084,
// UI-085, UI-086, UI-090, UI-092, UI-126, UI-127, UI-130): the invitation's
// words first, then the source rules of the screens that use them.
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";

import { LOI_LOI_MOI, cauLoiMoi, loiMoiTruoc } from "../dist-test/rudi/nhom/loi-moi-nhom.js";

const SRC = join(dirname(fileURLToPath(import.meta.url)), "..", "src", "rudi");
const doc = (ten) => readFileSync(join(SRC, ten), "utf8");

test("lời mời vào nhóm nói ai mời và nhóm có mấy người; lời mời lên đầu danh sách (UI-080)", () => {
  assert.equal(cauLoiMoi({ invited_by: { id: "a", display_name: "Chat Test 01" }, member_count: 2 }, 9), "Chat Test 01 mời bạn · 2 người trong nhóm");
  assert.equal(cauLoiMoi({ invited_by: null, member_count: 3 }, 9), "Bạn được mời · 3 người trong nhóm", "người mời đã xoá tài khoản");
  assert.equal(cauLoiMoi(null, 4), "Bạn được mời · 4 người trong nhóm", "chưa đọc được lời mời: vẫn là lời mời");
  assert.deepEqual(loiMoiTruoc([{ id: "1", my_state: "active" }, { id: "2", my_state: "invited" }, { id: "3", my_state: "active" }]).map((n) => n.id), ["2", "1", "3"]);
  assert.match(LOI_LOI_MOI.invitation_not_found, /không còn/);
  const man = doc("screens/groups/Conversations.tsx");
  assert.match(man, /label="Từ chối"/);
  assert.match(man, /Từ chối lời mời vào \$\{tenCuocTroChuyen\(nhom\)\}\? Nếu đổi ý, bạn cần được mời lại\./, "từ chối hỏi tại hàng");
  assert.match(man, /<HoiTaiHang[\s\S]*?nhan="Từ chối"/, "hỏi tại hàng bằng primitive chung (B11)");
  assert.doesNotMatch(man, /setTrang\(\{ pha: "hong", loi: loiRaChu\(error\) \}\);\s*\} finally \{\s*setDangBam/, "lỗi trả lời không thành màn lỗi (UI-077 pattern)");
});

test("lập nhóm xong vào chính phòng chat của nhóm; nhóm một người mời bạn trước (UI-071)", () => {
  const moi = doc("screens/groups/New.tsx");
  assert.match(moi, /router\.replace\(`\/groups\/\$\{lap\.id\}\/chat`/);
  assert.doesNotMatch(moi, /manDau\(moi\)/);
  const chat = doc("screens/chat/GroupChatLive.tsx");
  assert.match(chat, /const chiMinhToi = !nhanRieng && \(soDangO \?\? nhom\?\.member_count \?\? 2\) <= 1/);
  assert.match(chat, /label="Mời bạn vào nhóm"/);
});

test("thành viên: hỏi trước khi tự bỏ quyền, nút có tên người, hàng mở hồ sơ, «Người lập nhóm» theo người lập (UI-074, 075, 076, 081)", () => {
  const man = doc("screens/groups/Members.tsx");
  assert.match(man, /tv\.person_id === phien\.person_id && vaiTro === "member" && !daHoi/);
  assert.match(man, /không tự lấy lại được/);
  assert.match(man, /`Đặt \$\{ten\} làm quản trị`/);
  assert.match(man, /"Bỏ quyền quản trị của bạn"/);
  assert.match(man, /router\.push\(`\/people\/\$\{tv\.person_id\}`/);
  assert.match(man, /if \(nguoiLap === tv\.person_id\) return "Người lập nhóm"/);
  assert.doesNotMatch(man, /tv\.role === "admin" && tv\.state === "active" \? "Người lập nhóm"/, "không gán «Người lập nhóm» cho mọi quản trị");
  assert.match(man, /<RudiScreen cot="doc"/);
  assert.match(doc("screens/friends/Friends.tsx"), /cot="doc"/);
  // Two columns where 280dp each fit: the action stays beside its name on a tablet.
  assert.match(doc("ui/LuoiNguoi.tsx"), /gridFor\(rong, 280, 16, 2\)/);
  assert.match(man, /<LuoiNguoi vachTrong=\{false\} hang=/);
  assert.match(doc("screens/friends/Friends.tsx"), /<LuoiNguoi hang=/);
});

test("bạn bè: lỗi của một hàng nằm dưới hàng đó, danh sách giữ (UI-077)", () => {
  const man = doc("screens/friends/Friends.tsx");
  assert.match(man, /setLoiHang\(\{ id: lm\.id, cau: loiRaChu\(error\) \}\)/);
  assert.match(man, /setLoiHang\(\{ id: b\.person_id, cau: loiRaChu\(error\) \}\)/);
  assert.doesNotMatch(man, /catch \(error\) \{\s*setTrang\(\{ pha: "hong", loi: loiRaChu\(error\) \}\);\s*\} finally \{\s*setDang/);
});

test("hồ sơ người đã chặn: đọc danh sách chặn, không mời kết bạn, không gọi «Cùng nhóm» (UI-078)", () => {
  const man = doc("screens/nguoi/HoSoNguoiScreen.tsx");
  assert.match(man, /docDaChan\(phien\.person_id\)/);
  assert.match(man, /setDaChan\(dsChan\.blocked\.some\(\(n\) => n\.person_id === personId\)\)/);
  assert.match(man, /\{daChan \? <Chip label="Đã chặn" selected \/> :/);
  assert.match(man, /hoSo\.hoSo\.relation === "groupmate" && !daChan/);
});

test("người vào bằng lời mời thấy tên nhóm đặt cho mình và sửa được (UI-073)", () => {
  const man = doc("screens/Onboarding.tsx");
  assert.match(man, /const quaLoiMoi = laVaoQuaLoiMoi\(/, "đọc từ phiên, không từ URL");
  assert.match(man, /Nhóm mời bạn đang gọi bạn là «\$\{tenNhomGoi\}»/);
  assert.match(man, /ten\.trim\(\) === tenNhomGoi\) return/, "giữ tên cũ thì không ghi gì");
});

test("sổ hai người: lỗi đọc không vẽ thành «chưa có sổ»; sheet đóng khi sổ mở; M6 một lần; câu chờ trên màn (UI-083, 084, 086, 126, 127)", () => {
  const man = doc("screens/hai-nguoi/KhongGianGiay.tsx");
  assert.match(man, /if \(so\.loiDoc !== null\) \{\s*\/\/[^\n]*\n[\s\S]*?than = <ErrorState body=\{so\.loiDoc\} onRetry=\{so\.lamMoi\} title="Chưa đọc được sổ của hai bạn" \/>/);
  assert.match(doc("to-giay/SoDoiSong.tsx"), /loiDoc: song\.pha === "loi" \? song\.loi : null/);
  assert.match(man, /useLayoutEffect\(\(\) => \{\s*if \(!so\.daNap\) return;\s*if \(lapSoTruoc\.current === false && so\.lapSo\) \{\s*setVuaMoSo\(true\);\s*setMo\(\(m\) => \(m === "lap-so" \? null : m\)\)/);
  assert.match(man, /setMo\(\(m\) => \(m === "bat-doi" \|\| m === "loai-so" \? null : m\)\)/);
  assert.match(doc("screens/hai-nguoi/DongYBac.tsx"), /useGiuKhiDong\(open, \{ dangCho: dangChoSong, deNghiCuaToi: deNghiCuaToiSong \}\)/);
  assert.match(doc("screens/hai-nguoi/LoaiSo.tsx"), /useGiuKhiDong\(open, \{ batDoi: batDoiSong/);
  assert.match(man, /\{khoanhKhacM6\}\s*<Heading title="Hai người cũng thành một hội"/, "M6 diễn ở cả nhánh «hội»");
  assert.match(man, /Đã đề nghị lập sổ\. Chờ \$\{so\.tenNguoiKia \|\| "người ấy"\} đồng ý trên máy của họ\./);
  assert.match(man, /"Xem lời đề nghị của bạn"/);
  assert.match(man, /đề nghị hai bạn là «Một đôi»\./);
  assert.match(man, /so\.toGiay\.length > 0 \? "Hẹn nhau như mọi hội bạn\. Những tờ giấy cũ vẫn nằm ở đây\." : "Hẹn nhau như mọi hội bạn\."/);
});

test("«Rủ … tới đây» không bỏ chỗ: cặp chưa «Một đôi» tạo kèo có chỗ làm chặng đầu; tuần đã chốt thêm vào buổi (UI-085, UI-130)", () => {
  const man = doc("screens/hai-nguoi/KhongGianGiay.tsx");
  assert.match(man, /setChoVaoKeo\(\{ id: goiYCho, lam: "tao-keo" \}\)/);
  assert.match(man, /\/outings\/new\?contextId=\$\{contextId\}&place=/);
  assert.match(man, /\/outings\/chon\?place=/);
  const tao = doc("screens/keo/CreateOutingLive.tsx");
  assert.match(tao, /themChang\(\[\], \{ at: gioTiepTheo\(\), label: choDau\.ten, place_name: choDau\.ten, place_id: choDau\.id \}\)/);
  assert.match(tao, /Kèo đã tạo, nhưng chưa thêm được \$\{choDau\.ten\} làm chặng\./, "thêm chặng hỏng thì nói, không lặng lẽ bỏ");
});

test("bìa sổ: tên xuống hai dòng; dải ngày cuộn tới lá đang chọn; việc phá huỷ mang tông warn (UI-090, UI-092)", () => {
  assert.match(doc("ui/SoBia.tsx"), /<Text numberOfLines=\{2\} style=\{\[typography\.label, styles\.tenBia/);
  assert.match(doc("screens/hai-nguoi/DeNghiSua.tsx"), /daiNgay\.current\?\.scrollTo\(\{ x: Math\.max\(0, giua\), animated: false \}\)/);
  assert.match(doc("screens/hai-nguoi/ToLoiRu.tsx"), /tone=\{n === "bo" \|\| n === "huy" \? "warn" : undefined\}/);
  assert.match(doc("screens/hai-nguoi/KhongGianGiay.tsx"), /nguyHiem=\{viec === "bo" \|\| viec === "huy"\}/);
});
