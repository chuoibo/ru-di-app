import Svg, { Circle, Path } from "react-native-svg";
import geometry from "../source/icon-geometry.json";

export type AssetIconName = keyof typeof geometry;
type Shape = { kind: "path"; d: string; fill: boolean; strokeWidth: number } | { kind: "circle"; cx: number; cy: number; r: number; fill: boolean; strokeWidth: number };

/** Decorative glyph. The enclosing control owns its label, hit target and state. */
export function AssetIcon({ name, size = 24, color }: { name: AssetIconName; size?: number; color: string }) {
  return <Svg width={size} height={size} viewBox="0 0 24 24" accessible={false} focusable={false} aria-hidden>
    {(geometry[name] as Shape[]).map((shape, i) => {
      const style = { fill: shape.fill ? color : "none", stroke: shape.fill ? "none" : color, strokeWidth: shape.strokeWidth, strokeLinecap: "round" as const, strokeLinejoin: "round" as const };
      return shape.kind === "path" ? <Path key={i} d={shape.d} {...style} /> : <Circle key={i} cx={shape.cx} cy={shape.cy} r={shape.r} {...style} />;
    })}
  </Svg>;
}
