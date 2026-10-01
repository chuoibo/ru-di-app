/**
 * The lead card's two lines under the name (owner's mockup, 01/10): the price
 * as its own chip, whole — money is never ellipsized — and every other fact
 * as one quiet line. The facts are the ones the compare cards and rows show,
 * so the lead never says less than the cards beneath it. Both catalogues mark
 * the price with the wallet icon (`chiTietNgan`, the demo's `hienThiMau`).
 */
export const ICON_GIA = "wallet-outline";

export function tachTheDan(facts: readonly { icon: string; text: string }[]): { gia: string | null; phu: string } {
  const gia = facts.find((f) => f.icon === ICON_GIA)?.text ?? null;
  const phu = facts
    .filter((f) => f.icon !== ICON_GIA)
    .map((f) => f.text)
    .join(" · ");
  return { gia, phu };
}
