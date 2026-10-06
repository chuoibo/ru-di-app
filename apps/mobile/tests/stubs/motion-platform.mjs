/**
 * Device boundary for real React lifecycle tests. This is deliberately not a
 * Reanimated implementation: tests hand-advance a pending animation and can
 * observe cancellation, completion and haptic requests. No native frame-rate,
 * worklet threading or OS delivery claim follows from this scheduler.
 */
import { createContext, useEffect, useRef } from "react";

export const NavigationContext = createContext(undefined);
export const DEVICE = { initialReduced: false };
export const CLOCK = { starts: [], cancels: [], haptics: [] };
const pending = new Map();
const reactions = new Set();
const ANIMATION = Symbol("device timing");

function notify() {
  for (const reaction of reactions) {
    const next = reaction.prepare();
    const previous = reaction.previous;
    reaction.previous = next;
    if (next !== previous) reaction.react(next, previous);
  }
}

export function useSharedValue(initial) {
  const ref = useRef(null);
  if (!ref.current) {
    let value = initial;
    ref.current = {
      get value() { return value; },
      set value(next) {
        if (next?.[ANIMATION]) {
          const entry = { ...next, shared: ref.current };
          pending.set(ref.current, entry);
          CLOCK.starts.push(entry);
        } else {
          value = next;
          notify();
        }
      },
    };
  }
  return ref.current;
}

export function withTiming(target, config, complete) {
  return { [ANIMATION]: true, target, config, complete };
}

export function cancelAnimation(shared) {
  const animation = pending.get(shared);
  if (!animation) return;
  pending.delete(shared);
  CLOCK.cancels.push(animation);
  animation.complete?.(false);
}

export function advance(shared, value) {
  if (!pending.has(shared)) throw new Error("No animation to advance");
  shared.value = value;
}

export function finish(shared) {
  const animation = pending.get(shared);
  if (!animation) throw new Error("No animation to finish");
  pending.delete(shared);
  shared.value = animation.target;
  animation.complete?.(true);
}

export function isAnimating(shared) { return pending.has(shared); }
export function resetClock() {
  if (pending.size || reactions.size) throw new Error("Previous hook did not clean up");
  CLOCK.starts.length = CLOCK.cancels.length = CLOCK.haptics.length = 0;
}

export function useAnimatedReaction(prepare, react, dependencies) {
  const latest = useRef({ prepare, react });
  latest.current = { prepare, react };
  useEffect(() => {
    const reaction = {
      prepare: () => latest.current.prepare(),
      react: (next, previous) => latest.current.react(next, previous),
      previous: latest.current.prepare(),
    };
    reactions.add(reaction);
    return () => reactions.delete(reaction);
  }, dependencies);
}

export function runOnJS(callback) { return callback; }
export function useReducedMotion() { return DEVICE.initialReduced; }
export const ReduceMotion = { Always: "always", Never: "never", System: "system" };
export const Easing = { linear: value => value, bezier: (...points) => Object.assign(value => value, { points }) };
export const ImpactFeedbackStyle = { Medium: "medium" };
export const NotificationFeedbackType = { Success: "success" };
export async function selectionAsync() { CLOCK.haptics.push("selection"); }
export async function impactAsync() { CLOCK.haptics.push("impact"); }
export async function notificationAsync() { CLOCK.haptics.push("success"); }
