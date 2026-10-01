import { Ionicons } from "@expo/vector-icons";
import { useEffect, useRef, type ReactNode } from "react";
import { AccessibilityInfo, Platform, StyleSheet, Text, View } from "react-native";
import { Gesture, GestureDetector } from "react-native-gesture-handler";
import Animated, { runOnJS, useAnimatedStyle, useSharedValue, withTiming } from "react-native-reanimated";

import { typography, useRudiTheme } from "../theme";
import { useMotion } from "./useMotion";

type RowBox = { y: number; height: number };

/** Variable-height rows: a larger font changes drop targets as well as drawing. */
export function ReorderList<T>({ items, itemKey, label, renderItem, onChange, disabled, onDragging }: {
  items: readonly T[];
  itemKey(item: T): string;
  label(item: T): string;
  renderItem(item: T, index: number): ReactNode;
  onChange(items: T[]): void;
  disabled?: boolean;
  onDragging?(dragging: boolean): void;
}) {
  const boxes = useRef<Record<string, RowBox>>({});
  // A keyboard move re-orders the rows, and the browser may drop focus from a
  // node React moves; the moved stop's handle gets it back, so the next arrow
  // keeps moving the same stop.
  const tayNam = useRef<Record<string, unknown>>({});
  const traFocus = useRef<string | null>(null);
  useEffect(() => {
    const khoa = traFocus.current;
    if (khoa === null) return;
    traFocus.current = null;
    const el = tayNam.current[khoa] as { focus?: () => void } | undefined;
    el?.focus?.();
  }, [items]);
  const move = (from: number, to: number, banPhim = false) => {
    if (disabled || from === to || to < 0 || to >= items.length) return;
    const next = [...items];
    const [item] = next.splice(from, 1);
    next.splice(to, 0, item);
    if (banPhim) traFocus.current = itemKey(item);
    onChange(next);
    AccessibilityInfo.announceForAccessibility(`${label(item)}: vị trí ${to + 1} trên ${items.length}`);
  };
  const drop = (index: number, translation: number) => {
    const box = boxes.current[itemKey(items[index])];
    if (!box) return;
    const center = box.y + box.height / 2 + translation;
    let target = index;
    let distance = Infinity;
    items.forEach((item, i) => {
      const position = boxes.current[itemKey(item)];
      if (!position) return;
      const delta = Math.abs(center - position.y - position.height / 2);
      if (delta < distance) { distance = delta; target = i; }
    });
    move(index, target);
  };
  return <View style={styles.list}>{items.map((item, index) => (
    <View key={itemKey(item)} onLayout={({ nativeEvent }) => { boxes.current[itemKey(item)] = nativeEvent.layout; }}>
      <ReorderRow label={label(item)} index={index} count={items.length} disabled={disabled}
        drop={(dy) => drop(index, dy)} move={(to, banPhim) => move(index, to, banPhim)} onDragging={onDragging}
        tayNamRef={(el) => { tayNam.current[itemKey(item)] = el; }}>
        {renderItem(item, index)}
      </ReorderRow>
    </View>
  ))}</View>;
}

function ReorderRow({ children, label, index, count, disabled, drop, move, onDragging, tayNamRef }: {
  children: ReactNode; label: string; index: number; count: number; disabled?: boolean;
  drop(dy: number): void; move(to: number, banPhim?: boolean): void; onDragging?(value: boolean): void;
  tayNamRef(el: unknown): void;
}) {
  const { colors } = useRudiTheme();
  const motion = useMotion();
  const settle = motion.timing("standard");
  const y = useSharedValue(0);
  const held = useSharedValue(false);
  const dragState = (value: boolean) => { onDragging?.(value); if (value) motion.haptic.select(); };
  const gesture = Gesture.Pan().enabled(!disabled && count > 1).activateAfterLongPress(250)
    .onStart(() => { held.value = true; runOnJS(dragState)(true); })
    .onUpdate((event) => { y.value = event.translationY; })
    .onEnd((event) => { runOnJS(drop)(event.translationY); })
    .onFinalize(() => {
      held.value = false;
      y.value = withTiming(0, settle);
      runOnJS(dragState)(false);
    });
  const style = useAnimatedStyle(() => ({ transform: [{ translateY: y.value }], zIndex: held.value ? 10 : 0 }));
  // The web has no adjustable role and drops `accessibilityValue` and the
  // increment actions, so the handle was a slider nobody could focus or move
  // without a mouse, and axe flagged its missing value (QA UI-036, UI-042). On
  // the web it is a real slider: focusable, valued, moved with the arrows
  // (down and right move it later), Home and End.
  const banPhim =
    Platform.OS === "web"
      ? {
          role: "slider",
          "aria-valuemin": 1,
          "aria-valuemax": count,
          "aria-valuenow": index + 1,
          "aria-valuetext": `Vị trí ${index + 1} trên ${count}`,
          "aria-orientation": "vertical",
          "aria-disabled": !!disabled,
          tabIndex: disabled ? -1 : 0,
          onKeyDown: (event: { key: string; preventDefault(): void }) => {
            const toi =
              event.key === "ArrowDown" || event.key === "ArrowRight" ? index + 1
              : event.key === "ArrowUp" || event.key === "ArrowLeft" ? index - 1
              : event.key === "Home" ? 0
              : event.key === "End" ? count - 1
              : null;
            if (toi === null) return;
            event.preventDefault();
            move(toi, true);
          },
        }
      : {};
  return <Animated.View style={[styles.row, { backgroundColor: colors.ground }, style]}>
    <View style={styles.content}>{children}</View>
    <GestureDetector gesture={gesture}>
      <View accessible accessibilityRole="adjustable" accessibilityLabel={`Thứ tự ${label}`} ref={tayNamRef as never}
        accessibilityHint="Giữ rồi kéo để đổi thứ tự. Hoặc dùng thao tác tăng giảm vị trí."
        accessibilityValue={{ min: 1, max: count, now: index + 1 }}
        accessibilityState={{ disabled: !!disabled }}
        accessibilityActions={[{ name: "increment", label: "Xuống một chặng" }, { name: "decrement", label: "Lên một chặng" }]}
        onAccessibilityAction={({ nativeEvent }) => move(index + (nativeEvent.actionName === "increment" ? 1 : -1))}
        {...(banPhim as object)}
        style={styles.handle}>
        <Ionicons name="reorder-two-outline" size={24} color={colors.inkFaint} />
        <Text style={[typography.caption, { color: colors.inkFaint }]}>{index + 1}</Text>
      </View>
    </GestureDetector>
  </Animated.View>;
}

const styles = StyleSheet.create({
  list: { gap: 8 },
  row: { flexDirection: "row", alignItems: "center", gap: 8 },
  content: { flex: 1, minWidth: 0 },
  handle: { minWidth: 48, minHeight: 56, justifyContent: "center", alignItems: "center" },
});
