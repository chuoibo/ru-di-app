import type { StyleProp, ViewStyle } from "react-native";

import { KHUNG_CANH, hinhCanh, moTaCanh, type CanhId } from "../../art/canh";
import { VeLop } from "./VeLop";

export interface CanhProps {
  id: CanhId;
  /** Width in dp; the height follows the scene's 144:112 frame. */
  width?: number;
  /** Off, the props stand alone; the same scene, for the with/without comparison. */
  nep?: boolean;
  style?: StyleProp<ViewStyle>;
  testID?: string;
}

/**
 * An empty-state scene for `EmptyState`'s `illustration` slot. It says one
 * thing about the silence (a seat kept, a page the ink starts on) and is read
 * to a screen reader as one short sentence.
 */
export function Canh({ id, width = 144, nep = true, style, testID }: CanhProps) {
  const height = Math.round((width * KHUNG_CANH.h) / KHUNG_CANH.w);
  return (
    <VeLop
      accessibilityLabel={moTaCanh(id)}
      height={height}
      khungH={KHUNG_CANH.h}
      khungW={KHUNG_CANH.w}
      lop={hinhCanh(id, { nep })}
      style={style}
      testID={testID ?? `canh-${id}`}
      width={width}
    />
  );
}
