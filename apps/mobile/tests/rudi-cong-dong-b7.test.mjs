// Community feed decisions and the composer's topic rules (QA N14: UI-133,
// UI-139, UI-140, UI-141, UI-143, UI-144, UI-145, UI-146, UI-147, UI-148),
// pure functions first, then the source rules of the screens that use them.
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";
import ts from "typescript";

import { TOI_DA_CHU_DE, chuanChuDe, loiChuDe, tachChuDe } from "../dist-test/rudi/community/chu-de.js";
import { cauThongBao, coThongBaoMoi, doiTheoDoiTacGia, nhanTrangThaiDuyet, thanBiCat } from "../dist-test/rudi/community/bang-tin.js";
import { COMMUNITY_ERRORS } from "../dist-test/rudi/community/api.js";

const SRC = join(dirname(fileURLToPath(import.meta.url)), "..", "src", "rudi", "community");
const doc = (ten) => readFileSync(join(SRC, ten), "utf8");

/** Locate the visible send error, including the stale-edit recovery branch. */
function sendErrorExpression(source) {
  const sf = ts.createSourceFile("Composer.tsx", source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
  const found = [];
  function visit(node) {
    if (ts.isJsxSelfClosingElement(node) && node.tagName.getText(sf) === "CauTaiCho") {
      const attrs = node.attributes.properties.filter(ts.isJsxAttribute);
      const id = attrs.find((attr) => attr.name.getText(sf) === "testID")?.initializer;
      if (id && ts.isStringLiteral(id) && id.text === "cong-dong-loi-gui") {
        assert.ok(node.attributes.properties.every(ts.isJsxAttribute), "lỗi gửi không bị JSX spread ghi đè");
        const cau = attrs.find((attr) => attr.name.getText(sf) === "cau")?.initializer;
        if (cau && ts.isJsxExpression(cau) && cau.expression) found.push(cau.expression);
      }
    }
    ts.forEachChild(node, visit);
  }
  visit(sf);
  assert.equal(found.length, 1, "một lỗi gửi tại chỗ, không trùng ô thông báo");
  return found[0];
}

/** Follow the refusal's document binding instead of its former inline action. */
function assertNepErrorDocument(source) {
  const sf = ts.createSourceFile("PostDetail.tsx", source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
  function nodes(root, predicate, rendered = false) {
    const found = [];
    function visit(node) {
      // Callback bodies and attributes are not rendered document children.
      if (rendered && (ts.isFunctionLike(node) || ts.isJsxAttribute(node) || ts.isJsxSpreadAttribute(node))) return;
      if (predicate(node)) found.push(node);
      ts.forEachChild(node, visit);
    }
    visit(root);
    return found;
  }
  function variable(name) {
    const found = nodes(sf, (node) => ts.isVariableDeclaration(node) && ts.isIdentifier(node.name) && node.name.text === name);
    assert.equal(found.length, 1, `một binding ${name}`);
    assert.ok(found[0].initializer);
    return found[0].initializer;
  }
  function attribute(element, name) {
    return element.attributes.properties.find((attr) => ts.isJsxAttribute(attr) && attr.name.getText(sf) === name)?.initializer;
  }
  function literalAttribute(element, name, value) {
    const attr = attribute(element, name);
    return attr && ts.isStringLiteral(attr) && attr.text === value;
  }
  const editor = variable("editorContent");
  const errors = nodes(editor, (node) => ts.isJsxSelfClosingElement(node) && node.tagName.getText(sf) === "CauTaiCho" && literalAttribute(node, "testID", "cong-dong-loi-nep"), true);
  assert.equal(errors.length, 1, "một lỗi Nếp trong tài liệu editor");
  assert.ok(errors[0].attributes.properties.every(ts.isJsxAttribute), "lỗi Nếp không bị spread ghi đè");
  const copy = attribute(errors[0], "cau");
  assert.ok(copy && ts.isJsxExpression(copy) && copy.expression && ts.isIdentifier(copy.expression) && copy.expression.text === "loiNep", "lỗi Nếp lấy loiNep tại chỗ (UI-138)");
  const sheets = nodes(sf, (node) => ts.isJsxElement(node) && node.openingElement.tagName.getText(sf) === "Sheet" && literalAttribute(node.openingElement, "accessibilityLabel", "Gọi Nếp"));
  assert.equal(sheets.length, 1, "một sheet Nếp");
  assert.ok(sheets[0].openingElement.attributes.properties.every(ts.isJsxAttribute), "sheet Nếp không bị spread ghi đè");
  const footer = attribute(sheets[0].openingElement, "footer");
  assert.ok(footer && ts.isJsxExpression(footer) && footer.expression && ts.isIdentifier(footer.expression) && footer.expression.text === "editorFooter", "sheet Nếp gắn đúng CTA editorFooter, không gắn tài liệu lỗi vào footer");
  const documentReferences = sheets[0].children.flatMap((child) => nodes(child, (node) => ts.isJsxExpression(node) && node.expression && ts.isIdentifier(node.expression) && node.expression.text === "editorContent", true));
  assert.equal(documentReferences.length, 1, "sheet Nếp render đúng tài liệu có câu lỗi");
  const footerReferences = nodes(variable("editorFooter"), (node) => ts.isIdentifier(node) && node.text === "editorContent");
  assert.equal(footerReferences.length, 0, "tài liệu lỗi không chuyển vào CTA cố định");
}

/** Bind the update affordance to the returned feed, not an unused JSX declaration. */
function assertUpdateRowInFlow(source) {
  const sf = ts.createSourceFile("CommunityScreen.tsx", source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
  function nodes(root, predicate, ownBody = false) {
    const found = [];
    function visit(node) {
      if (ownBody && ts.isFunctionLike(node)) return;
      if (predicate(node)) found.push(node);
      ts.forEachChild(node, visit);
    }
    visit(root);
    return found;
  }
  const tag = (node) => (ts.isJsxElement(node) ? node.openingElement : node).tagName.getText(sf);
  const children = (node) => node.children.filter((child) => !(ts.isJsxText(child) && !child.text.trim()) && !(ts.isJsxExpression(child) && !child.expression));
  function attr(element, name) {
    assert.ok(element.attributes.properties.every(ts.isJsxAttribute), "update document binding has no overriding JSX spread");
    return element.attributes.properties.find((item) => item.name.getText(sf) === name)?.initializer;
  }
  function expression(element, name) {
    const value = attr(element, name);
    assert.ok(value && ts.isJsxExpression(value) && value.expression, `actual update document has ${name}`);
    return value.expression;
  }
  const identifier = (node, name) => ts.isIdentifier(node) && node.text === name;
  const reference = (node, name) => ts.isPropertyAccessExpression(node) && identifier(node.expression, "styles") && node.name.text === name;
  const owners = nodes(sf, (node) => ts.isFunctionDeclaration(node) && node.name?.text === "CommunityScreenContent");
  assert.equal(owners.length, 1, "one feed content owner");
  const returns = nodes(owners[0].body, (node) => ts.isReturnStatement(node) && node.expression && ts.isJsxElement(node.expression) && tag(node.expression) === "RudiScreen" && attr(node.expression.openingElement, "testID")?.text === "community-screen", true);
  assert.equal(returns.length, 1, "one actual returned feed screen");
  const columns = children(returns[0].expression).filter((node) => ts.isJsxElement(node) && tag(node) === "View" && reference(expression(node.openingElement, "style"), "readingColumn"));
  assert.equal(columns.length, 1, "one rendered reading column");
  const document = children(columns[0]);
  const lists = document.filter((node) => ts.isJsxSelfClosingElement(node) && tag(node) === "FlatList" && identifier(expression(node, "ref"), "list") && identifier(expression(node, "data"), "posts"));
  assert.equal(lists.length, 1, "same reader-owned FlatList");
  const updates = document.filter((node) => ts.isJsxExpression(node) && node.expression && ts.isConditionalExpression(node.expression) && identifier(node.expression.condition, "fresh"));
  assert.equal(updates.length, 1, "one fresh affordance in the actual column");
  assert.ok(document[document.indexOf(lists[0]) + 1] === updates[0], "fresh affordance follows the same FlatList in reading order");
  const update = updates[0].expression;
  assert.equal(update.whenFalse.kind, ts.SyntaxKind.NullKeyword, "no reserved update row while fresh is false");
  assert.ok(ts.isJsxElement(update.whenTrue) && tag(update.whenTrue) === "Animated.View", "fresh renders the update row directly");
  const row = update.whenTrue;
  const buttons = children(row);
  assert.equal(buttons.length, 1, "one direct refresh action");
  const button = buttons[0];
  assert.ok(ts.isJsxSelfClosingElement(button) && tag(button) === "RudiButton", "rendered row contains the existing button primitive");
  const label = attr(button, "accessibilityLabel");
  assert.ok(label && ts.isStringLiteral(label) && label.text === "Bảng tin có cập nhật", "refresh keeps its accessible name");
  const press = expression(button, "onPress");
  assert.ok(ts.isArrowFunction(press) && press.parameters.length === 0 && ts.isVoidExpression(press.body) && ts.isCallExpression(press.body.expression) && identifier(press.body.expression.expression, "refresh") && press.body.expression.arguments.length === 0, "actual button calls the existing guarded refresh");
  const style = expression(row.openingElement, "style");
  assert.ok(ts.isArrayLiteralExpression(style), "rendered update row exposes its style bindings");
  const sheets = nodes(sf, (node) => ts.isVariableDeclaration(node) && identifier(node.name, "styles"));
  assert.equal(sheets.length, 1, "one source stylesheet binding");
  const sheet = sheets[0].initializer;
  assert.ok(sheet && ts.isCallExpression(sheet) && ts.isPropertyAccessExpression(sheet.expression) && identifier(sheet.expression.expression, "StyleSheet") && sheet.expression.name.text === "create" && sheet.arguments.length === 1 && ts.isObjectLiteralExpression(sheet.arguments[0]), "actual source stylesheet is inspectable");
  assert.ok(sheet.arguments[0].properties.every(ts.isPropertyAssignment), "stylesheet has no spread override for the update binding");
  const bindings = style.elements.filter((node) => reference(node, "newPosts"));
  assert.equal(bindings.length, 1, "rendered row uses the inspected update style");
  const assignments = sheet.arguments[0].properties.filter((node) => (ts.isIdentifier(node.name) || ts.isStringLiteral(node.name)) && node.name.text === "newPosts");
  assert.equal(assignments.length, 1, "one update style definition");
  const objects = [assignments[0].initializer, ...style.elements.filter((node) => !reference(node, "newPosts"))];
  for (const object of objects) {
    assert.ok(ts.isObjectLiteralExpression(object), "no opaque update style can bypass the flow contract");
    for (const property of object.properties) {
      assert.ok(ts.isPropertyAssignment(property) && (ts.isIdentifier(property.name) || ts.isStringLiteral(property.name)), "update styles have no dynamic or spread overrides");
      const name = property.name.text;
      assert.ok(!["top", "bottom", "left", "right", "start", "end", "transform", "inset", "insetBlock", "insetInline"].includes(name), "update row has no visual offset over the document");
      if (name === "position") assert.ok(ts.isStringLiteral(property.initializer) && property.initializer.text === "relative", "update row participates in normal layout");
    }
  }
}

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
  assert.match(man, /if \(dangHien\.current !== khoa\) \{\s*dangHien\.current = khoa;\s*daNhanTrangDau\.current = false;\s*viTriCuon\.current = 0;\s*setPosts\(\[\]\)/);
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
  // UI-143: updates have a normal-flow place after the actual reader-owned
  // list, keeping the search clear without moving the list's starting edge.
  assertUpdateRowInFlow(man);
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
  assertNepErrorDocument(ct);
  // A different error value or a detached document must still fail UI-138.
  assert.throws(() => assertNepErrorDocument(ct.replace("cau={loiNep}", "cau={error}")), /lỗi Nếp lấy loiNep/);
  assert.throws(() => assertNepErrorDocument(ct.replace("        {editorContent}\n", "        {null}\n")), /sheet Nếp render đúng tài liệu/);
  // A callback-only reference cannot stand in for rendered children.
  assert.throws(() => assertNepErrorDocument(ct.replace("        {editorContent}\n", "        <View onLayout={() => <>{editorContent}</>} />\n")), /sheet Nếp render đúng tài liệu/);
  // Verify the actual Sheet slot, not an unused footer declaration.
  assert.throws(() => assertNepErrorDocument(ct.replace("prioritizeFooter footer={editorFooter}", "prioritizeFooter footer={editorContent}")), /sheet Nếp gắn đúng CTA editorFooter/);
  const bl = doc("Comments.tsx");
  assert.match(bl, /<CauTaiCho cau=\{error\} hanhDong=\{\{ label: "Thử lại", onPress: \(\) => void load\(\) \}\}/, "UI-148");
  assert.doesNotMatch(bl, /setItems\(\[\]\);\s*void load\(\)/);
  const so = doc("Composer.tsx");
  assert.match(so, /const loiChuDeGo = loiChuDe\(topics\)/);
  const sendError = sendErrorExpression(so);
  if (ts.isConditionalExpression(sendError)) {
    assert.ok(ts.isIdentifier(sendError.condition) && sendError.condition.text === "conflict", "chỉ xung đột bản sửa mới thay câu lỗi máy chủ");
    assert.ok(ts.isStringLiteral(sendError.whenTrue) && sendError.whenTrue.text.trim().length > 0, "xung đột có lời giải thích tại chỗ");
  }
  const serverError = ts.isConditionalExpression(sendError) ? sendError.whenFalse : sendError;
  assert.ok(ts.isIdentifier(serverError) && serverError.text === "error", "lỗi máy chủ vẫn hiện tại chỗ khi không có xung đột bản sửa");
  assert.match(so, /params\.edit \? <Text[^>]*testID="cong-dong-nguoi-doc-khoa">Bản sửa giữ người đọc/, "người đọc khoá khi sửa nói vì sao");
  // UI-096: deletion still asks in the row; active waits keep a visible reason.
  // The idle editor uses its placeholder and disabled send without a duplicate helper.
  assert.match(bl, /onPress=\{\(\) => \{ if \(!mounted\.current \|\| !deletableNow\.current\.has\(c\.id\)\) return; setCommentMenu\(null\); setHoiXoa\(c\.id\); setLoiXoa\(null\); \}\}/);
  assert.match(bl, /Xóa bình luận này\? Không lấy lại được\./);
  assert.doesNotMatch(bl, /label="Xóa" variant="ghost" onPress=\{\(\) => \{ void translatedAsActor/);
  assert.match(bl, /const sendReason = busy \? "Đợi gửi xong rồi viết tiếp\." : uploading \? "Đợi ảnh tải xong rồi gửi\." : pendingFile !== null \? "Thử lại hoặc bỏ ảnh đang chờ\." : null/);
  assert.match(bl, /testID="community-comment-send-reason"[^>]*>\{sendReason\}<\/Text>/, "disabled send keeps its visible reason in the editor");
  assert.match(bl, /accessibilityLabel=\{busy \? "Đang gửi…" : "Gửi bình luận"\} disabled=\{busy \|\| uploading \|\| pendingFile !== null \|\| parts\.pending \|\| !body\.trim\(\)\}/);
  const the = doc("PostCard.tsx");
  assert.match(the, /if \(!expanded && biCat\) setExpanded\(true\)/, "UI-139");
  // UI-143: a frame never wider than the album it sits in. Since the five-column
  // strip (owner's choice, 02/10) each page IS the album's measured width.
  assert.match(the, /const kichAnh = kichTrangAlbum\(rongAlbum\)/, "UI-143");
  assert.match(doc("Search.tsx"), /Không tìm thấy chủ đề hay người nào khớp/, "UI-145");
  assert.match(doc("Keeps.tsx"), /Chưa có ghi chép nào/, "UI-145");
  assert.match(doc("Notifications.tsx"), /cauThongBao\(item\)/, "UI-147");
});
