import type { StyleProp, ViewStyle } from "react-native";

import { duongChuyen, gocGap, nepGoc, thuGapBa, vongHo } from "../../art/motif";
import { VeLop } from "./VeLop";

interface Chung {
  style?: StyleProp<ViewStyle>;
  testID?: string;
}

/** The open ring, decoration only: never a progress ring, never animated. */
export function VongHo({ size = 96, net = 3.5, style, testID }: Chung & { size?: number; net?: number }) {
  const r = size / 2 - net;
  return <VeLop height={size} khungH={size} khungW={size} lop={vongHo(size / 2, size / 2, r, { net })} style={style} testID={testID} width={size} />;
}

/** The passed line across a box, the pen's first touch in coral. */
export function DuongChuyen({ width, height, huong = "down", style, testID }: Chung & { width: number; height: number; huong?: "down" | "up" }) {
  return <VeLop height={height} khungH={height} khungW={width} lop={duongChuyen(0, 0, width, height, huong)} style={style} testID={testID} width={width} />;
}

/** The fold alone, laid on a corner of something else: the kept mark on a print. */
export function NepGoc({ size = 18, style, testID }: Chung & { size?: number }) {
  return <VeLop height={size} khungH={size} khungW={size} lop={nepGoc(size, 0, size)} style={style} testID={testID} width={size} />;
}

/** A sheet with its top-right corner folded down: the kept-corner mark. */
export function GocGap({ width, height, style, testID }: Chung & { width: number; height: number }) {
  return <VeLop height={height} khungH={height} khungW={width} lop={gocGap(1.5, 1.5, width - 3, height - 3)} style={style} testID={testID} width={width} />;
}

/** A letter folded in thirds: the two-person notebook's sheet, creases and all, no coral. */
export function ThuGapBa({ width, height, style, testID }: Chung & { width: number; height: number }) {
  return <VeLop height={height} khungH={height} khungW={width} lop={thuGapBa(1.5, 1.5, width - 3, height - 3)} style={style} testID={testID} width={width} />;
}
