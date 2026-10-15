import { Image } from "expo-image";
import { PixelRatio, type StyleProp, type ImageStyle } from "react-native";
import metadata from "../source/raster-metadata.json";
import { illustrationResolution, type AssetResolution } from "../resolution";

const assets = {
  "scene-hills": { small: require("../illustrations/scene-hills-384.webp"), medium: require("../illustrations/scene-hills-768.webp"), master: require("../illustrations/scene-hills.webp") },
  "scene-street": { small: require("../illustrations/scene-street-384.webp"), medium: require("../illustrations/scene-street-768.webp"), master: require("../illustrations/scene-street.webp") },
  "scene-lanterns": { small: require("../illustrations/scene-lanterns-384.webp"), medium: require("../illustrations/scene-lanterns-768.webp"), master: require("../illustrations/scene-lanterns.webp") },
  "scene-coast": { small: require("../illustrations/scene-coast-384.webp"), medium: require("../illustrations/scene-coast-768.webp"), master: require("../illustrations/scene-coast.webp") },
  "scene-atelier": { small: require("../illustrations/scene-atelier-384.webp"), medium: require("../illustrations/scene-atelier-768.webp"), master: require("../illustrations/scene-atelier.webp") },
  "scene-cafe": { small: require("../illustrations/scene-cafe-384.webp"), medium: require("../illustrations/scene-cafe-768.webp"), master: require("../illustrations/scene-cafe.webp") },
  "decor-branch": { small: require("../decor/decor-branch-384.webp"), medium: require("../decor/decor-branch-768.webp"), master: require("../decor/decor-branch.webp") },
  "decor-pines": { small: require("../decor/decor-pines-384.webp"), medium: require("../decor/decor-pines-768.webp"), master: require("../decor/decor-pines.webp") },
  "decor-cover": { small: require("../decor/decor-cover-384.webp"), medium: require("../decor/decor-cover-768.webp"), master: require("../decor/decor-cover.webp") },
  "spot-keeps": { small: require("../illustrations/spot-keeps-384.webp"), medium: require("../illustrations/spot-keeps-768.webp"), master: require("../illustrations/spot-keeps.webp") },
  "spot-memory": { small: require("../illustrations/spot-memory-384.webp"), medium: require("../illustrations/spot-memory-768.webp"), master: require("../illustrations/spot-memory.webp") },
  "spot-map": { small: require("../illustrations/spot-map-384.webp"), medium: require("../illustrations/spot-map-768.webp"), master: require("../illustrations/spot-map.webp") },
} as const;

export type IllustrationName = keyof typeof assets;

/** Local artwork only; user and venue media continue to come from their API. */
export function AssetIllustration({ name, width, height, resolution = "auto", fit, style, contentPosition }: {
  name: IllustrationName; width: number; height: number;
  resolution?: "auto" | AssetResolution; fit?: "contain" | "cover";
  contentPosition?: { left: `${number}%`; top: `${number}%` };
  style?: StyleProp<ImageStyle>;
}) {
  const info = metadata[name];
  const contentFit = fit ?? info.fit as "contain" | "cover";
  const selected = resolution === "auto" ? illustrationResolution(width, height, info.size[0] / info.size[1], PixelRatio.get(), contentFit) : resolution;
  const focal = contentPosition ?? { left: `${info.focal[0] * 100}%` as const, top: `${info.focal[1] * 100}%` as const };
  return <Image source={assets[name][selected]} contentFit={contentFit} contentPosition={focal} transition={0} accessible={false} accessibilityElementsHidden pointerEvents="none" style={[{ width, height }, style]} />;
}
