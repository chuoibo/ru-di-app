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
