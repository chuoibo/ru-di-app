/**
 * One reason per place, and the drawn object a place stands behind when it has
 * no honest photo. Pure: strings in, strings out, no React.
 *
 * Re-audit 10/09 (R3): the Explore lead said the same promise four times --
 * «Gần bạn, đúng gu» → the «HỢP GU» seal → «Hợp gu nhờ Chill và View đẹp» →
 * a subtitle that repeated «view … chill». Two rules replace that:
 *
 * 1. `chonLyDo` picks ONE matched tag, and never one the subtitle already
 *    says: the reason must add a fact the line under it does not.
 * 2. `guTheoTag` lets a tag the place really carries choose the drawn object
 *    («Món local» → the local dish, «Ngoài trời» → outdoor) before the
 *    category does, so three eateries in a row are not three identical bowls
 *    -- and stay identical when nothing honest tells them apart.
 */

/** Lower-case, no diacritics, so «View đẹp» finds «view đồi». */
function gapChu(text: string): string {
  return text
    .normalize("NFD")
    .replace(/[̀-ͯ]/g, "")
    .replace(/đ/g, "d")
    .replace(/Đ/g, "D")
    .toLowerCase();
}

/**
 * The first tag whose words do not already appear in the subtitle. When every
 * tag is echoed, the first tag still stands (the subtitle is the fixture's
 * problem, not the reader's). No tags → no reason.
 */
export function chonLyDo(tags: readonly string[], sub: string | undefined): string | undefined {
  if (tags.length === 0) return undefined;
  // Whole words, not substrings: «san» (Săn mây) is not said by «sáng» (finish
  // review 11/09 -- the substring test dropped the strongest reason).
  const nen = new Set(gapChu(sub ?? "").split(/[^a-z0-9]+/).filter(Boolean));
  const chuaNoi = tags.find((tag) => {
    const tu = gapChu(tag).split(/[^a-z0-9]+/).filter((t) => t.length >= 3);
    return tu.length === 0 ? true : !tu.some((t) => nen.has(t));
  });
  return chuaNoi ?? tags[0];
}

const GU_THEO_TAG: readonly (readonly [RegExp, string])[] = [
  [/^mon local$/, "mon-local"],
  [/^(ngoai troi|outdoor|san may)$/, "outdoor"],
  [/^karaoke$/, "karaoke"],
  [/^(game|board ?game|vui choi)$/, "game"],
  [/^(mua sam|shopping)$/, "shopping"],
  [/^(ca phe|cafe|coffee|tra)$/, "cafe"],
  [/^(di dem|nightlife|nhon nhip)$/, "nightlife"],
];

/**
 * The drawn object a tag names, or null when no tag names one. Only tags the
 * catalogue actually carries decide; a category fallback is the caller's
 * (`guTheoLoai`), so an eatery with no telling tag stays the bowl.
 */
export function guTheoTag(tags: readonly string[]): string | null {
  for (const tag of tags) {
    const t = gapChu(tag).trim();
    const hit = GU_THEO_TAG.find(([re]) => re.test(t));
    if (hit) return hit[1];
  }
  return null;
}
