/**
 * Ô nhập viết trên dòng kẻ (UI v3 S0.5, QC 24/09 B7): không có hộp, chỉ một
 * gạch mực; placeholder là chữ của nhà vẽ; tên trợ năng là nhãn hay câu gợi ý;
 * gạch dày lên khi đang viết hoặc sai mà chữ không xê dịch; trên web không có
 * khung focus xanh của trình duyệt.
 *
 * Không chứng minh: gạch có đủ tương phản (đó là test_contrast_floor.py) hay
 * người thật có nhận ra đây là chỗ để viết (đó là ảnh chụp và đọc mù).
 */
import assert from "node:assert/strict";
import test from "node:test";

import React from "react";
import { renderToStaticMarkup } from "react-dom/server";

import { ONhapMuc, kieuGach } from "../dist-test/rudi/ui/ONhapMuc.js";

const CAU = "Tên món, ví dụ Bún chả";

function ve(props) {
  return renderToStaticMarkup(React.createElement(ONhapMuc, { onChangeText: () => undefined, ...props }));
}

test("không có hộp: không viền bốn phía, không nền; chỉ gạch dưới", async () => {
  const { readFileSync } = await import("node:fs");
  const nguon = readFileSync(new URL("../src/rudi/ui/ONhapMuc.tsx", import.meta.url), "utf8");
  assert.doesNotMatch(nguon, /borderWidth:/, "ô viết trên dòng kẻ không được có viền bốn phía");
  assert.doesNotMatch(nguon, /backgroundColor:\s*colors\.card/, "ô viết trên trang không tự trải nền thẻ");
  assert.match(nguon, /borderBottomWidth: day/);
});

test("placeholder là chữ của nhà vẽ; tên trợ năng là nhãn, không thì câu gợi ý", () => {
  const html = ve({ placeholder: CAU, value: "" });
  const input = /<input\b[^>]*>/.exec(html)?.[0] ?? "";
  assert.ok(input, html);
  assert.doesNotMatch(input, /\bplaceholder=/);
  assert.match(html, />Tên món, ví dụ Bún chả</);
  assert.match(input, new RegExp(`aria-label="${CAU}"`));
  const coNhan = /<input\b[^>]*>/.exec(ve({ label: "Tên món", placeholder: CAU, value: "" }))?.[0] ?? "";
  assert.match(coNhan, /aria-label="Tên món"/);
  const nhanRieng = /<input\b[^>]*>/.exec(ve({ accessibilityLabel: "Ô tên món 1", label: "Tên món", value: "" }))?.[0] ?? "";
  assert.match(nhanRieng, /aria-label="Ô tên món 1"/, "nhãn Maestro truyền vào phải giữ nguyên");
});

test("có chữ thì gợi ý biến mất; lỗi thay dòng phụ và được đọc lên", () => {
  const html = ve({ placeholder: CAU, value: "Nem", helper: "Tên như trên hoá đơn", error: "Tên món còn trống" });
  assert.equal(html.includes(`>${CAU}<`), false);
  assert.match(html, />Tên món còn trống</);
  assert.equal(html.includes("Tên như trên hoá đơn"), false, "lỗi thay chỗ dòng phụ");
  assert.match(html, /aria-live="polite"/);
  assert.match(/<input\b[^>]*>/.exec(html)?.[0] ?? "", /aria-invalid="true"/);
});

test("gạch dày 2dp khi viết hay sai mà chữ không xê dịch", () => {
  for (const multiline of [false, true]) {
    const thuong = kieuGach({ day: 1, multiline, dongCao: 24 }).hang;
    const dam = kieuGach({ day: 2, multiline, dongCao: 24 }).hang;
    assert.equal(dam.borderBottomWidth, 2);
    assert.equal(thuong.paddingBottom + thuong.borderBottomWidth, dam.paddingBottom + dam.borderBottomWidth);
  }
  const nhieu = kieuGach({ multiline: true, numberOfLines: 4, dongCao: 24 }).nhap;
  assert.equal(nhieu.minHeight, 96);
  assert.equal(nhieu.textAlignVertical, "top");
  assert.equal(kieuGach({ multiline: true, numberOfLines: 40, dongCao: 24 }).nhap.minHeight, 8 * 24, "có trần rồi cuộn");
  assert.ok(kieuGach({ dongCao: 24 }).nhap.minHeight >= 44, "một dòng vẫn đủ 44dp để chạm");
});
