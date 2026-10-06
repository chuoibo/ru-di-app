import { useMemo } from "react";
import { SvgXml } from "react-native-svg";
import geometry from "../source/paper-geometry.json";
import type { PaperPalette } from "./PaperPalette";
import { usePaperPalette } from "./usePaperPalette";

export type PaperName = keyof typeof geometry;
export type { PaperPalette } from "./PaperPalette";

/** A paper surface/layer only. Keep actual words and controls outside its SVG. */
export function PaperObject({ name, width, palette: providedPalette, layer }: { name: PaperName; width: number; palette?: PaperPalette; layer?: string }) {
  const themePalette = usePaperPalette();
  const palette = providedPalette ?? themePalette;
  const object = geometry[name];
  const layers = object.layers as Record<string, string>;
  const template = layer ? layers[layer] : object.xml;
  if (!template) throw new Error(`Unknown paper layer: ${name}/${layer}`);
  const xml = useMemo(() => template.replace(/\{\{([a-z]+)\}\}/g, (_match, role: keyof PaperPalette) => palette[role]), [template, palette]);
  return <SvgXml xml={xml} width={width} height={width * object.height / object.width} accessible={false} focusable={false} aria-hidden />;
}
