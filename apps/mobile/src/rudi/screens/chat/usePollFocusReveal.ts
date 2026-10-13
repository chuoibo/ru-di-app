import { createRef, useCallback, useLayoutEffect, useRef, type RefObject } from "react";
import { Platform, ScrollView, TextInput, View, type NativeScrollEvent, type NativeSyntheticEvent } from "react-native";

import { typography } from "../../theme";
import { useNativeStandaloneTextMetrics, useNativeTextInputMetrics } from "../../ui/NativeFontMetrics";

type FieldRefs = { input: RefObject<TextInput | null>; group: RefObject<View | null> };
type Rect = { top: number; height: number };

function measuredRect(top: number, width: number, height: number): Rect | null {
  return Number.isFinite(top) && Number.isFinite(width) && width > 0 && Number.isFinite(height) && height > 0
    ? { top, height } : null;
}

/** Reveal only the current poll input inside its measured native scroll body. */
export function usePollFocusReveal(active: boolean, fieldCount = 7) {
  const caption = useNativeStandaloneTextMetrics(typography.caption);
  const body = useNativeTextInputMetrics(typography.body, {});
  const fields = useRef<FieldRefs[] | null>(null);
  if (fields.current === null) {
    // Fixed slots per editor; changing the current choice count never replaces refs.
    fields.current = Array.from({ length: Math.max(1, Math.min(9, fieldCount)) }, () => ({ input: createRef<TextInput>(), group: createRef<View>() }));
  }
  const slots = fields.current;
  const scrollRef = useRef<ScrollView>(null);
  const scrollY = useRef(0);
  const focused = useRef<number | null>(null);
  const mounted = useRef(false);
  const enabled = useRef(false);
  const manualDrag = useRef(false);
  const generation = useRef(0);
  const viewportGeneration = useRef(0);
  const frames = useRef<[number | null, number | null]>([null, null]);

  const cancel = useCallback(() => {
    generation.current += 1;
    for (const frame of frames.current) if (frame !== null) cancelAnimationFrame(frame);
    frames.current = [null, null];
  }, []);

  const request = useCallback((resume = false) => {
    if (manualDrag.current && !resume) return;
    cancel();
    if (resume) manualDrag.current = false;
    if (!mounted.current || !enabled.current || Platform.OS === "web") return;
    const index = focused.current;
    const slot = index === null ? undefined : slots[index];
    const input = slot?.input.current;
    const scroll = scrollRef.current;
    const viewport = scroll?.getNativeScrollRef();
    if (!slot || !input || !scroll || !viewport || TextInput.State.currentlyFocusedInput() !== input) return;
    const group = slot.group.current;
    const token = generation.current;
    const layoutToken = viewportGeneration.current;
    const offset = scrollY.current;
    const current = () => mounted.current && enabled.current && !manualDrag.current &&
      generation.current === token && viewportGeneration.current === layoutToken && focused.current === index &&
      slot.input.current === input && TextInput.State.currentlyFocusedInput() === input &&
      scrollRef.current === scroll && scroll.getNativeScrollRef() === viewport && scrollY.current === offset;
    frames.current[0] = requestAnimationFrame(() => {
      if (!current()) return;
      frames.current[0] = null;
      frames.current[1] = requestAnimationFrame(() => {
        if (!current()) return;
        frames.current[1] = null;
        viewport.measureInWindow((_x, top, width, height) => {
          if (!current()) return;
          const bodyRect = measuredRect(top, width, height);
          if (bodyRect === null) return;
          input.measureInWindow((_inputX, inputTop, inputWidth, inputHeight) => {
            if (!current()) return;
            const inputRect = measuredRect(inputTop, inputWidth, inputHeight);
            if (inputRect === null) return;
            const finish = (groupRect: Rect | null) => {
              if (!current() || (groupRect !== null && slot.group.current !== group)) return;
              // Keep the label when it fits. Errors are outside this group.
              const target = groupRect !== null && groupRect.height <= bodyRect.height &&
                groupRect.top <= inputRect.top && groupRect.top + groupRect.height >= inputRect.top + inputRect.height
                ? groupRect : inputRect;
              const clearance = Math.min(8, Math.max(0, (bodyRect.height - target.height) / 2));
              const bottomDelta = target.top + target.height - (bodyRect.top + bodyRect.height - clearance);
              const topDelta = target.top - (bodyRect.top + clearance);
              // An oversized input cannot satisfy both edges; keep its bottom stable.
              const delta = bottomDelta > 0 ? bottomDelta : target.height <= bodyRect.height && topDelta < 0 ? topDelta : 0;
              const y = Math.max(0, offset + delta);
              if (delta !== 0 && y !== offset && current()) scroll.scrollTo({ y, animated: false });
            };
            if (group === null || slot.group.current !== group) { finish(null); return; }
            group.measureInWindow((_groupX, groupTop, groupWidth, groupHeight) => {
              if (!current() || slot.group.current !== group) return;
              finish(measuredRect(groupTop, groupWidth, groupHeight));
            });
          });
        });
      });
    });
  }, [cancel, slots]);

  useLayoutEffect(() => {
    mounted.current = true;
    return () => { mounted.current = false; enabled.current = false; cancel(); };
  }, [cancel]);
  useLayoutEffect(() => {
    enabled.current = active;
    if (active) request(true); else cancel();
    return cancel;
  }, [active, caption?.fontSize, caption?.lineHeight, caption?.letterSpacing,
    body?.style.fontSize, body?.style.lineHeight, body?.style.letterSpacing, body?.linePitch, cancel, request]);

  const onFocus = useCallback((index: number) => {
    if (!slots[index]) return;
    focused.current = index;
    request(true);
  }, [request, slots]);
  const onBlur = useCallback((index: number) => {
    if (focused.current !== index) return;
    focused.current = null;
    cancel();
  }, [cancel]);
  const onBodyLayout = useCallback(() => {
    viewportGeneration.current += 1;
    request(true);
  }, [request]);
  const onFieldLayout = useCallback(() => { request(); }, [request]);
  const onScroll = useCallback((event: NativeSyntheticEvent<NativeScrollEvent>) => {
    const y = event.nativeEvent.contentOffset.y;
    if (!Number.isFinite(y)) return;
    const changed = scrollY.current !== y;
    scrollY.current = y;
    // Native focus/programmatic scrolling can settle after layout. Remeasure
    // only on a changed native offset, and never fight a manual drag.
    if (changed && !manualDrag.current) request();
  }, [request]);
  const onScrollBeginDrag = useCallback(() => {
    manualDrag.current = true;
    cancel();
  }, [cancel]);

  return { fields: slots, scrollRef, onFocus, onBlur, onBodyLayout, onFieldLayout,
    onContentSizeChange: onFieldLayout, onScroll, onScrollBeginDrag };
}
