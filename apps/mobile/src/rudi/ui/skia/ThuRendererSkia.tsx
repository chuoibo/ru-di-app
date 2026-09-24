/**
 * The renderer probe of `ui-lab` (Skia side): Nếp standing up out of the page
 * around its feet line in 3D, and an ink route drawing itself -- the two
 * primitives the whole campaign rests on (the pop-up and the draw-on). If this
 * block paints on a device, the dev client has Skia and the 3D transform works
 * there; if the board shows the SVG twin instead, it does not.
 *
 * Reached only through `luoiSkia(() => import("./skia/ThuRendererSkia"))`.
 */
import { Canvas, Group, Path, vec } from "@shopify/react-native-skia";
import { useMemo } from "react";
import { useDerivedValue, type SharedValue } from "react-native-reanimated";

import { CHAN_NEP, KHUNG_NEP, hinhNep } from "../../art/nep";
import { useRudiTheme } from "../../theme";
import { duongCongS } from "../duong-svg";
import { LopSkia, duongSkia } from "./VeSkia";

export interface ThuRendererProps {
  /** 0 = lying flat in the page, 1 = standing. */
  mo: SharedValue<number>;
  /** 0..1 of the route drawn. */
  veToi: SharedValue<number>;
  width: number;
  height: number;
}

export default function ThuRendererSkia({ mo, veToi, width, height }: ThuRendererProps) {
  const { colors } = useRudiTheme();
  const nep = useMemo(() => hinhNep("moi", { chiTiet: true }), []);
  const tiLe = height / KHUNG_NEP;
  const route = useMemo(() => duongSkia(duongCongS(Math.max(40, width - height - 16), height - 16, "down").d), [width, height]);
  const dung = useDerivedValue(() => [{ perspective: 480 }, { rotateX: ((1 - mo.value) * Math.PI) / 2 }]);
  return (
    <Canvas style={{ width, height }}>
      <Group transform={[{ scale: tiLe }]}>
        <Group origin={vec(KHUNG_NEP / 2, CHAN_NEP)} transform={dung}>
          <LopSkia colors={colors} lop={nep} />
        </Group>
      </Group>
      <Group transform={[{ translateX: height + 8 }, { translateY: 8 }]}>
        <Path color={colors.accent} end={veToi} path={route} start={0} strokeCap="round" strokeJoin="round" strokeWidth={3} style="stroke" />
      </Group>
    </Canvas>
  );
}
