/**
 * The colours of a `Switch`, one shape everywhere (B11 critique, 04/10).
 *
 * Off: a `lineStrong` track, the 3:1 floor of a control's edge on paper -- the
 * `line` hairline beside the cream ground all but vanished. On: the tone of
 * what the switch turns on (`accent`, or `ai` for the AI's own switches). The
 * thumb is paper on both platforms: react-native-web paints its own teal
 * thumb unless `activeThumbColor` is given, and teal is the money's ink.
 */
import { Platform, type SwitchProps } from "react-native";

import type { RudiPalette } from "../theme";

export function congTac(colors: RudiPalette, tone: string): Pick<SwitchProps, "trackColor" | "thumbColor"> & { activeThumbColor?: string } {
  return {
    trackColor: { false: colors.lineStrong, true: tone },
    thumbColor: colors.card,
    ...(Platform.OS === "web" ? { activeThumbColor: colors.card } : {}),
  };
}
