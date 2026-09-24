/**
 * The paper puppet with react-native-svg: the fallback renderer and the frame
 * shown while Skia loads (ADR-0037 D10).
 *
 * SVG cannot move a dozen parts every frame without rebuilding paths on the
 * JS thread, so this renderer does what paper puppets did before film ran
 * smooth: stop-motion. It draws the performance's KEY poses, switching when
 * the shared clock crosses a key -- five or six drawings for a performance,
 * each exactly the frame Skia passes through at that moment (`tuTheRoi` of the
 * same key). The clock is the caller's, so a hand-over to Skia mid-performance
 * continues rather than restarts.
 */
import { useMemo, useState } from "react";
import { runOnJS, useAnimatedReaction, type SharedValue } from "react-native-reanimated";

import { KHUNG_NEP, type GapNep } from "../../art/nep";
import type { TietMuc } from "../../art/nep-dien";
import { tuTheRoi } from "../../art/nep-roi";
import { VeLop } from "./VeLop";

export interface NepRoiSvgProps {
  tm: TietMuc;
  t: SharedValue<number>;
  width: number;
  chiTiet?: boolean;
  gap?: GapNep;
}

/** The last key at or before `t`. */
function khoaTai(tm: TietMuc, t: number): number {
  "worklet";
  let i = 0;
  while (i < tm.khoa.length - 1 && t >= tm.khoa[i + 1].t) i += 1;
  return i;
}

export function NepRoiSvg({ tm, t, width, chiTiet = true, gap = "trang" }: NepRoiSvgProps) {
  const cacKhung = useMemo(() => tm.khoa.map((k) => tuTheRoi(k.tt, { chiTiet, gap })), [tm, chiTiet, gap]);
  const [i, setI] = useState(() => khoaTai(tm, t.value));
  useAnimatedReaction(
    () => khoaTai(tm, t.value),
    (moi, cu) => {
      if (moi !== cu) runOnJS(setI)(moi);
    },
    [tm],
  );
  return <VeLop height={width} khungH={KHUNG_NEP} khungW={KHUNG_NEP} lop={cacKhung[Math.min(i, cacKhung.length - 1)]} width={width} />;
}
