// Community feed decisions and the composer's topic rules (QA N14: UI-133,
// UI-139, UI-140, UI-141, UI-143, UI-144, UI-145, UI-146, UI-147, UI-148),
// pure functions first, then the source rules of the screens that use them.
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";

import { TOI_DA_CHU_DE, chuanChuDe, loiChuDe, tachChuDe } from "../dist-test/rudi/community/chu-de.js";
import { cauThongBao, coThongBaoMoi, doiTheoDoiTacGia, nhanTrangThaiDuyet, thanBiCat } from "../dist-test/rudi/community/bang-tin.js";
import { COMMUNITY_ERRORS } from "../dist-test/rudi/community/api.js";

const SRC = join(dirname(fileURLToPath(import.meta.url)), "..", "src", "rudi", "community");
const doc = (ten) => readFileSync(join(SRC, ten), "utf8");

test("chủ đề: kiểm như máy chủ (posts.go normalizeTopics), nói điều cần sửa", () => {
  assert.equal(TOI_DA_CHU_DE, 5);
  assert.deepEqual(tachChuDe(" cà phê, ,Đà Lạt ,"), ["cà phê", "Đà Lạt"]);
  assert.equal(chuanChuDe("  #Cà   Phê "), "cà phê");
  assert.equal(loiChuDe(""), null);
  assert.equal(loiChuDe("cà phê, đi bộ, Đà Lạt"), null);
  assert.equal(loiChuDe("a1, b2, c3, d4, e5"), null, "đúng 5 chủ đề vẫn gửi được");
  assert.match(loiChuDe("a, b2, c3, d4, e5, f6"), /^Tối đa 5 chủ đề\. Bỏ bớt 1 chủ đề/);
  assert.match(loiChuDe("cà phê, x"), /«x» ngắn quá/);
  assert.match(loiChuDe("#a"), /ngắn quá/, "dấu # không tính");
  assert.match(loiChuDe("x".repeat(41)), /dài quá: tối đa 40/);
  assert.equal(loiChuDe("x".repeat(40)), null);
  assert.match(loiChuDe("ăn/uống"), /bỏ ký tự đó/);
  assert.match(loiChuDe("hỏi @lan"), /bỏ ký tự đó/);
  // The server's own refusals have their words too, and never «lỗi của app».
  assert.match(COMMUNITY_ERRORS.too_many_topics, /Tối đa 5 chủ đề/);
  assert.match(COMMUNITY_ERRORS.invalid_topic, /2 đến 40 ký tự/);
});

test("theo dõi đổi mọi thẻ cùng tác giả, không chỉ thẻ vừa chạm", () => {
  const ds = [{ id: "1", author_id: "a", following: true }, { id: "2", author_id: "a", following: true }, { id: "3", author_id: "b", following: true }];
  assert.deepEqual(doiTheoDoiTacGia(ds, "a", false).map((p) => p.following), [false, false, true]);
});

test("thân bị cắt mới tốn lần chạm đầu để mở hết", () => {
  assert.equal(thanBiCat("Một câu ngắn."), false);
  assert.equal(thanBiCat("x".repeat(241)), true);
  assert.equal(thanBiCat("a\nb\nc\nd\ne\nf\ng"), true, "bảy dòng: bị cắt ở sáu");
});

test("thông báo nói ai nhắc và nhắc ở đâu; chấm đỏ khi có cái mới hơn lần xem", () => {
  assert.equal(cauThongBao({ id: "1", post_id: "p", kind: "mention", created_at: "", actor: "Lan" }), "Lan nhắc bạn trong một câu chuyện");
  assert.equal(cauThongBao({ id: "1", post_id: "p", kind: "mention", created_at: "", actor: "Lan", from_comment: true }), "Lan nhắc bạn trong một bình luận");
  assert.equal(cauThongBao({ id: "1", post_id: "p", kind: "mention", created_at: "", actor: null }), "Bạn được nhắc trong một câu chuyện");
  const ds = [{ id: "1", post_id: "p", kind: "mention", created_at: "2030-09-27T10:00:00Z" }];
  assert.equal(coThongBaoMoi([], null), false);
  assert.equal(coThongBaoMoi(ds, null), true);
  assert.equal(coThongBaoMoi(ds, "2030-09-27T11:00:00Z"), false);
  assert.equal(coThongBaoMoi(ds, "2030-09-27T09:00:00Z"), true);
});

test("hàng duyệt nói trạng thái bằng chữ, không in mã", () => {
  assert.equal(nhanTrangThaiDuyet("pending"), "Chờ duyệt");
  assert.equal(nhanTrangThaiDuyet("review"), "Cần người xem lại");
  assert.equal(nhanTrangThaiDuyet("something_new"), "Đang xử lý");
  assert.match(doc("Review.tsx"), /nhanTrangThaiDuyet\(item\.status\)/);
  assert.doesNotMatch(doc("Review.tsx"), /\} · \{item\.status\}/);
});

test("bảng tin: quay lại giữ danh sách, ẩn có hoàn tác, xoá lịch sử hỏi trước, trang chủ đề có lối về", () => {
  const man = doc("CommunityScreen.tsx");
  // UI-135: the same feed is not cleared on focus; a different one starts over.
  assert.match(man, /if \(dangHien\.current !== khoa\) \{\s*dangHien\.current = khoa;\s*viTriCuon\.current = 0;\s*setPosts\(\[\]\)/);
  assert.doesNotMatch(man, /useFocusEffect\(useCallback\(\(\) => \{ fetching\.current = false; setPosts\(\[\]\)/);
  // UI-140, UI-141.
  assert.match(man, /doiTheoDoiTacGia\(items, p\.author_id, !p\.following\)/);
  assert.match(man, /label="Hoàn tác" onPress=\{\(\) => void hoanTacAn\(item\)\}/);
  assert.match(man, /label="Bài đã ẩn"/);
  assert.match(man, /label="Xoá lịch sử" tone="warn"/);
  // The header's own line never shares a row with the three actions (it broke
  // at 390dp); below 360dp the actions rise above the large title.
  assert.match(man, /<\/View>\s*<\/View>\s*\{\/\* Under the row/);
  assert.match(man, /const hep = useWindowDimensions\(\)\.width < 360/);
  // UI-143: the update band follows the header's measured height.
  assert.match(man, /top: caoDau \+ 8/);
  // The stream's first sync opens the connection: no «Bảng tin có cập nhật» for it.
  assert.match(man, /else if \(!lanDau\) setFresh\(true\)/);
  // UI-135 on the web: the place in the list is put back when the reader returns.
  assert.match(man, /list\.current\?\.scrollToOffset\(\{ offset: y, animated: false \}\)/);
  // UI-146: a topic page has the way back and follows its topic in place.
  assert.match(man, /<TopBar title=\{topic\} \/>/);
  assert.match(man, /"Theo dõi chủ đề"/);
  // UI-147: a way to the notifications outside the settings sheet.
  assert.match(man, /accessibilityLabel=\{coMoi \? "Thông báo, có điều mới" : "Thông báo"\}/);
});

test("chi tiết bài, bình luận, ô soạn: lỗi tại chỗ, nối lại không mất chữ", () => {
  const ct = doc("PostDetail.tsx");
  assert.doesNotMatch(ct, /setPost\(null\);\s*setDraft\(""\)/, "nối lại không xoá bài và bản nháp (UI-134)");
  assert.match(ct, /<CauTaiCho cau=\{loiNep\} hanhDong=\{\{ label: "Thử lại"/, "lỗi Nếp nằm trong sheet Nếp (UI-138)");
  const bl = doc("Comments.tsx");
  assert.match(bl, /<CauTaiCho cau=\{error\} hanhDong=\{\{ label: "Thử lại", onPress: \(\) => void load\(\) \}\}/, "UI-148");
  assert.doesNotMatch(bl, /setItems\(\[\]\);\s*void load\(\)/);
  const so = doc("Composer.tsx");
  assert.match(so, /const loiChuDeGo = loiChuDe\(topics\)/);
  assert.match(so, /<CauTaiCho cau=\{error\} testID="cong-dong-loi-gui" \/>/, "lỗi máy chủ ngay trên nút gửi");
  assert.match(so, /params\.edit \? <Text[^>]*testID="cong-dong-nguoi-doc-khoa">Bản sửa giữ người đọc/, "người đọc khoá khi sửa nói vì sao");
  // UI-096 on Cộng đồng: «Xóa» asks in the comment's row; the send button says why it is off.
  assert.match(bl, /onPress=\{\(\) => \{ setHoiXoa\(c\.id\); setLoiXoa\(null\); \}\}/);
  assert.match(bl, /Xóa bình luận này\? Không lấy lại được\./);
  assert.doesNotMatch(bl, /label="Xóa" variant="ghost" onPress=\{\(\) => \{ void translatedAsActor/);
  assert.match(bl, /lyDo=\{uploading \? "Đợi ảnh tải xong rồi gửi\." : !body\.trim\(\) \? "Viết vài chữ trước khi gửi\."/);
  const the = doc("PostCard.tsx");
  assert.match(the, /if \(!expanded && biCat\) setExpanded\(true\)/, "UI-139");
  assert.match(the, /Math\.min\(296, rongAlbum\)/, "UI-143");
  assert.match(doc("Search.tsx"), /Không tìm thấy chủ đề hay người nào khớp/, "UI-145");
  assert.match(doc("Keeps.tsx"), /Chưa có ghi chép nào/, "UI-145");
  assert.match(doc("Notifications.tsx"), /cauThongBao\(item\)/, "UI-147");
});
