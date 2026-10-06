export type AssetResolution = "small" | "medium" | "master";

/** Resolve the pixels actually occupied by the image, rather than the screen. */
export function illustrationResolution(width: number, height: number, ratio: number, density: number, fit: "contain" | "cover"): AssetResolution {
  const valid = [width, height, ratio, density].every(n => Number.isFinite(n) && n > 0);
  if (!valid) return "small";
  const renderedWidth = fit === "cover" ? Math.max(width, height * ratio) : Math.min(width, height * ratio);
  const pixels = renderedWidth * density;
  return pixels <= 384 ? "small" : pixels <= 768 ? "medium" : "master";
}
