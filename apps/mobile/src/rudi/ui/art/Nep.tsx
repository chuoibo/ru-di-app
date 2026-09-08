import type { StyleProp, ViewStyle } from "react-native";

import { KHUNG_NEP, hinhNep, type PoseNep } from "../../art/nep";
import { VeLop } from "./VeLop";

export interface NepProps {
  pose: PoseNep;
  /** Edge of the square, in dp. Below 72 the 48dp reading is drawn instead of shrinking the 96dp one. */
  size?: number;
  style?: StyleProp<ViewStyle>;
  testID?: string;
}

/**
 * Nếp on its own, for a first-use moment or a cover. Decorative: it never
 * carries information, so it is hidden from the accessibility tree; the words
 * beside it do the talking. Never place it next to money, an error or a
 * conflict (report 07/09 §6.4).
 */
export function Nep({ pose, size = 96, style, testID }: NepProps) {
  const lop = hinhNep(pose, { chiTiet: size >= 72 });
  return <VeLop height={size} khungH={KHUNG_NEP} khungW={KHUNG_NEP} lop={lop} style={style} testID={testID} width={size} />;
}
