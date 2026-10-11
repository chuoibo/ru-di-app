import { useEffect, useState } from "react";

import {
  observeFontScale,
  readFontScale,
  sameNativeFontMetrics,
  type FontScaleSnapshot,
} from "../../../modules/rudi-font-scale";

function sameSnapshot(a: FontScaleSnapshot, b: FontScaleSnapshot): boolean {
  if (a.status === "available" && b.status === "available") {
    return Object.is(a.fontScale, b.fontScale) && sameNativeFontMetrics(a.fontMetrics, b.fontMetrics);
  }
  return a.status === "unavailable" && b.status === "unavailable" && a.reason === b.reason;
}

/** M alone observes configuration without replacing its draft or window hooks. */
export function useBillFontScale(): FontScaleSnapshot {
  const [snapshot, setSnapshot] = useState<FontScaleSnapshot>(readFontScale);

  useEffect(() => {
    let active = true;
    const reread = () => {
      if (!active) return;
      const next = readFontScale();
      setSnapshot((previous) => sameSnapshot(previous, next) ? previous : next);
    };
    // Subscribe first, then close the render-to-subscription gap with a fresh read.
    const observation = observeFontScale(reread);
    if (observation.status === "unavailable") {
      const next: FontScaleSnapshot = { status: "unavailable", reason: observation.reason };
      setSnapshot((previous) => sameSnapshot(previous, next) ? previous : next);
      return () => { active = false; };
    }
    reread();
    return () => {
      active = false;
      observation.remove();
    };
  }, []);

  return snapshot;
}
