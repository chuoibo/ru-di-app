/**
 * The shape of a print: the photo's own ratio, held between 3:4 portrait and
 * 1.91:1 wide. A fixed frame either put grey bands beside a portrait photo
 * (share screen) or cut its head and feet off (the wall's 4:3 crop), QA 23/09.
 */
export function tiLeKhung(anh: { width: number; height: number } | null | undefined, macDinh = 4 / 3): number {
  if (!anh || !(anh.width > 0) || !(anh.height > 0)) return macDinh;
  return Math.min(1.91, Math.max(0.75, anh.width / anh.height));
}
