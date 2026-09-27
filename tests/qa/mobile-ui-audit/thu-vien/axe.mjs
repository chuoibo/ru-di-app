/* axe-core on the live page: WCAG 2 A/AA rules, summarised per rule.
 *
 * What it adds to do-dac.mjs: computed colour contrast against the real
 * cascade, and the ARIA structure rules (required children/parents, allowed
 * attributes, nested interactive). What it cannot do here: text over the
 * paper textures and illustrations is an image background, which axe reports
 * as INCOMPLETE rather than guessing; those are listed separately so they are
 * looked at in the screenshot instead of being counted as a pass.
 */
import { AxeBuilder } from "@axe-core/playwright";

export async function chayAxe(page) {
  const kq = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"]).analyze();
  const tom = (ds) =>
    ds.map((v) => ({
      rule: v.id,
      impact: v.impact,
      so: v.nodes.length,
      vd: v.nodes.slice(0, 4).map((n) => ({
        target: String(n.target?.[0] ?? "").slice(0, 80),
        tomTat: String(n.failureSummary ?? n.any?.[0]?.message ?? "").replace(/\s+/g, " ").slice(0, 200),
      })),
    }));
  return { vi: tom(kq.violations), chuaRo: tom(kq.incomplete.filter((i) => i.id === "color-contrast")) };
}
