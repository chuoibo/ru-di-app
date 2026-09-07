import type { StyleProp, ViewStyle } from "react-native";

import { KHUNG_CANH, hinhCanh, hopNgang, moTaCanh, type CanhId } from "../../art/canh";
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
  const lop = hinhCanh(id, { nep });
  // `width` is the width of the full 144 frame; a scene framed tighter keeps
  // the same scale, so with and without the figure the props stay one size.
  const { x0, x1 } = hopNgang(lop);
  const le = 4;
  const tu = Math.max(0, Math.floor(x0 - le));
  const rong = Math.min(KHUNG_CANH.w, Math.ceil(x1 + le)) - tu;
  const tiLe = width / KHUNG_CANH.w;
  return (
    <VeLop
      accessibilityLabel={moTaCanh(id)}
      height={Math.round(KHUNG_CANH.h * tiLe)}
      khungH={KHUNG_CANH.h}
      khungW={KHUNG_CANH.w}
      lop={lop}
      style={style}
      testID={testID ?? `canh-${id}`}
      viewBox={`${tu} 0 ${rong} ${KHUNG_CANH.h}`}
      width={Math.round(rong * tiLe)}
    />
  );
}
