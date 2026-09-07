import type { StyleProp, ViewStyle } from "react-native";

import { KHUNG_GU, hinhGu } from "../../art/gu";
import { useRudiTheme } from "../../theme";
import { VeLop } from "./VeLop";

export interface GuGlyphProps {
  /** A taste id from the server's vocabulary; anything else draws a folded tag. */
  id: string;
  size?: number;
  /** `accent` inks the glyph in the accent for a chosen tile; `ink` is the resting reading. */
  tone?: "ink" | "accent";
  style?: StyleProp<ViewStyle>;
  testID?: string;
}

/**
 * One taste as an object drawn with one pen. Decorative: the tile's label
 * names the taste; the glyph only helps the eye find it.
 */
export function GuGlyph({ id, size = 48, tone = "ink", style, testID }: GuGlyphProps) {
  const { colors } = useRudiTheme();
  return (
    <VeLop
      doiMau={tone === "accent" ? { muc: colors.accent } : undefined}
      height={size}
      khungH={KHUNG_GU}
      khungW={KHUNG_GU}
      lop={hinhGu(id)}
      style={style}
      testID={testID}
      width={size}
    />
  );
}
