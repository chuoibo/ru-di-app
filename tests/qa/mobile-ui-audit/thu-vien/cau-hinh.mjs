/* The configuration matrix of the 2026-09-27 mobile UI audit.
 *
 * Sizes are logical CSS px, which react-native-web renders 1:1 as dp. Every
 * context is a touch phone to the page (`isMobile`, `hasTouch`), including the
 * tablet widths: the app ships to iPad (`supportsTablet`) and adapts by window
 * size class, not by device, so a 768 or 1024 window is what an iPad shows.
 *
 * DPR 2 everywhere: enough to read 12sp text in a screenshot, and half the
 * bytes of DPR 3. Nothing measured here depends on the pixel ratio.
 *
 * What this matrix cannot express, and the report says so instead of pretending:
 * font scale (react-native-web pins fontScale to 1.0), safe-area insets (the
 * page has no viewport-fit=cover, so env() insets are 0), and a software
 * keyboard (headless Chromium has none). C8 is a SHORT WINDOW, a proxy for an
 * IME-resized or split-screen window, never evidence about a keyboard.
 */
export const DPR = 2;

const tao = (id, width, height, colorScheme, reducedMotion, nhan) =>
  Object.freeze({ id, width, height, colorScheme, reducedMotion, nhan });

export const CAU_HINH = Object.freeze({
  C1: tao("C1", 390, 844, "light", "no-preference", "điện thoại chuẩn"),
  C2: tao("C2", 320, 640, "light", "no-preference", "hẹp nhất"),
  C3: tao("C3", 360, 800, "dark", "no-preference", "Android phổ biến, tối"),
  C4: tao("C4", 375, 667, "light", "no-preference", "iPhone SE 2/3, thấp"),
  C5: tao("C5", 430, 932, "light", "no-preference", "máy lớn"),
  C6: tao("C6", 768, 1024, "light", "no-preference", "tablet medium, rail"),
  C7: tao("C7", 1024, 1366, "dark", "no-preference", "tablet expanded, hai pane"),
  C8: tao("C8", 390, 460, "light", "no-preference", "cửa sổ thấp (proxy IME/split-screen)"),
  C9: tao("C9", 390, 844, "light", "reduce", "giảm chuyển động"),
  // Size-class boundaries of src/rudi/adaptive.ts, used by the shell checks only.
  B599: tao("B599", 599, 900, "light", "no-preference", "biên compact"),
  B600: tao("B600", 600, 900, "light", "no-preference", "biên medium"),
  B839: tao("B839", 839, 1100, "light", "no-preference", "biên medium trên"),
  B840: tao("B840", 840, 1100, "light", "no-preference", "biên expanded"),
});

export function cauHinh(id) {
  const ch = CAU_HINH[id];
  if (!ch) throw new Error(`cấu hình không có: ${id} (có: ${Object.keys(CAU_HINH).join(", ")})`);
  return ch;
}

export function danhSach(chuoi) {
  return String(chuoi)
    .split(",")
    .map((s) => s.trim())
    .filter(Boolean)
    .map(cauHinh);
}
