import type { StyleProp, ViewStyle } from "react-native";

import { type GapNep, KHUNG_NEP, hinhNep, type PoseNep } from "../../art/nep";
import { VeLop } from "./VeLop";

export interface NepProps {
  pose: PoseNep;
  /** Edge of the square, in dp. Below 72 the 48dp reading is drawn instead of shrinking the 96dp one. */
  size?: number;
  /** Which sheet: the group page (default) or the pocket-folded one of the two-person notebook. */
  gap?: GapNep;
  style?: StyleProp<ViewStyle>;
  testID?: string;
}

/**
 * Nếp on its own, for a first-use moment or a cover. Decorative: it never
 * carries information, so it is hidden from the accessibility tree; the words
 * beside it do the talking. Never place it next to money, an error or a
 * conflict (report 07/09 §6.4).
 */
export function Nep({ pose, size = 96, gap = "trang", style, testID }: NepProps) {
  const lop = hinhNep(pose, { chiTiet: size >= 72, gap });
  return <VeLop height={size} khungH={KHUNG_NEP} khungW={KHUNG_NEP} lop={lop} style={style} testID={testID} width={size} />;
}
