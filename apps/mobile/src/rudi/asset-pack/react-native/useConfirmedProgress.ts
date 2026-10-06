import { useEffect, useRef } from "react";
import { cancelAnimation, useSharedValue, withTiming, type WithTimingConfig } from "react-native-reanimated";
import { nenDien } from "../../khoanh-khac";
import { useMotion } from "../../ui/useMotion";
import { useScreenActive } from "../../ui/useScreenActive";

/** A visual clock, keyed by a real response; it owns no domain transition. */
export function useConfirmedProgress(eventKey: string | null, timing?: WithTimingConfig, kind = "arrival", focused = true, withoutEvent = 0) {
  const motion = useMotion();
  const active = useScreenActive(focused);
  const progress = useSharedValue(eventKey ? 1 : withoutEvent);
  const config = useRef(timing);
  config.current = timing;
  const key = eventKey ? `asset:${kind}:${eventKey}` : "";
  useEffect(() => {
    const settle = () => { cancelAnimation(progress); progress.value = key ? 1 : withoutEvent; };
    if (!key || nenDien({ khoa: key, reduced: motion.reduced || !active }) !== "dien") settle();
    else {
      progress.value = 0;
      progress.value = withTiming(1, { ...motion.timing("standard"), ...config.current, reduceMotion: motion.reanimated });
    }
    return settle;
  }, [key, active, motion.reduced, motion.reanimated, progress, withoutEvent]);
  return progress;
}
