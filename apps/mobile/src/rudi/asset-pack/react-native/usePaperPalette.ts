import { useMemo } from "react";
import { mauVatLieuAsset, useRudiTheme } from "../../theme";
import type { PaperPalette } from "./PaperPalette";

/** Decor and text-bearing paper use the existing theme's separate roles. */
export function usePaperPalette(textSurface = false): PaperPalette {
  const { colors, dark } = useRudiTheme();
  return useMemo(() => ({ ink: colors.ink, accent: colors.accent, paper: textSurface ? colors.card : colors.paper, shade: colors.paperShade, muted: colors.inkSoft, teal: colors.split, cover: colors.cover, ochre: dark ? mauVatLieuAsset.ochreDark : mauVatLieuAsset.ochre, cream: colors.ground }), [colors, dark, textSurface]);
}
