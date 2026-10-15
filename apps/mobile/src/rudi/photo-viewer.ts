/** Keep the image, caption and accessibility counter on the same page. */
export function viewerIndex(index: number, count: number): number {
  return Math.max(0, Math.min(Number.isFinite(index) ? Math.round(index) : 0, count - 1));
}

/** Worklet-safe bounds are reapplied while pinching, not only while panning. */
export function boundPhotoOffset(offset: number, extent: number, scale: number): number {
  "worklet";
  const limit = Math.max(0, extent * (scale - 1) / 2);
  return Math.max(-limit, Math.min(limit, offset));
}

export type PhotoCommand = "zoom" | "reset" | "left" | "right" | "up" | "down";

/** Buttons obey the same zoom range and pan bounds as native gestures. */
export function photoButtonTransform(command: PhotoCommand, current: { scale: number; x: number; y: number }, width: number, height: number) {
  if (command === "zoom" || command === "reset") return { scale: command === "zoom" ? 2 : 1, x: 0, y: 0 };
  const step = Math.min(width, height) / 4;
  return {
    scale: current.scale,
    x: boundPhotoOffset(current.x + (command === "left" ? -step : command === "right" ? step : 0), width, current.scale),
    y: boundPhotoOffset(current.y + (command === "up" ? -step : command === "down" ? step : 0), height, current.scale),
  };
}
