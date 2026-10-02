/** Toggle state that actually survives the trip to the DOM.
 *
 * `accessibilityState={{ checked }}` is the React Native spelling of "this box
 * is ticked", and on web it delivers nothing at all. react-native-web 0.21.2
 * forwards no prop by that name: grepping its shipped `dist` for
 * `accessibilityState` returns five hits, all inside the deprecated
 * `TouchableWithoutFeedback` prop map and `isDisabled`, none on the path
 * `Pressable` and `View` take. Rendered through the real library, a cell that
 * declared the state came out as
 *
 *     <div role="checkbox" tabindex="0" class="...">
 *
 * byte-identical ticked and unticked. A screen reader announced "checkbox" and
 * never said whether it was on, and pressing it announced nothing, on the very
 * cells that decide how much each person owes.
 *
 * `aria-checked` is not a web-only spelling of the same idea. React Native's
 * own `Pressable` and `View` resolve `ariaChecked ?? accessibilityState?.checked`
 * (`Libraries/Components/Pressable/Pressable.js:229`), so one prop serves both
 * platforms and sending both would only create two places to disagree.
 *
 * The role travels together with the state on purpose, because picking the
 * attribute is the half people get wrong. `aria-selected` is invalid on `radio`
 * and on `button`; both of the occurrences this module was written for had
 * paired them by hand -- a chip row that said `role="radio"` with
 * `selected`, and a mode switch that said `role="button"` with `selected`.
 * `checkbox`, `radio` and `switch` all take `aria-checked` and nothing else,
 * so a caller that asks for the role cannot pick the wrong attribute.
 */

import { Platform } from "react-native";

/** Roles whose on/off state is carried by `aria-checked`. */
export type ToggleRole = "checkbox" | "radio" | "switch";

type PhimSpace = { key: string; preventDefault(): void };

export type ToggleProps = {
  accessibilityRole: ToggleRole;
  "aria-checked": boolean;
  onKeyDown?: (event: PhimSpace) => void;
};

/** Spread onto the `Pressable` that is the toggle itself.
 *
 * `on` is required rather than optional: a toggle whose state is unknown is
 * the bug this exists to prevent, so there is no way to ask for the role
 * without also saying which way it is set.
 *
 * Pass `onToggle` (the same function as `onPress`) and Space flips it on the
 * web too: react-native-web's press responder accepts Space only from a
 * `button` or `role="button"` and Enter from anything, so a seat or a list box
 * built from `Pressable` with `role="checkbox"` answered Enter and ignored
 * Space, the key a checkbox is operated with (QA UI-053).
 */
export function toggleState(role: ToggleRole, on: boolean, onToggle?: () => void): ToggleProps {
  return { accessibilityRole: role, "aria-checked": on, ...(onToggle ? phimSpace(onToggle) : {}) };
}

/** Space presses `onToggle` on the web; nothing on native, where there is no key. */
export function phimSpace(onToggle: () => void): { onKeyDown?: (event: PhimSpace) => void } {
  if (Platform.OS !== "web") return {};
  return {
    onKeyDown: (event) => {
      if (event.key !== " " && event.key !== "Spacebar") return;
      // The page would scroll by a screen otherwise.
      event.preventDefault();
      onToggle();
    },
  };
}

export type TabProps = { accessibilityRole: "tab"; "aria-selected": boolean };

/**
 * Spread onto one tab of a `tablist`. A tab's state is `aria-selected` -- the
 * one attribute a radio or a button must not carry, which is why it has its
 * own helper rather than a fourth `ToggleRole` (QA UI-003: four tabs, no
 * `aria-selected`, no `tablist`). The container takes `TABLIST`.
 */
export function tabState(selected: boolean): TabProps {
  return { accessibilityRole: "tab", "aria-selected": selected };
}

/** Spread onto the row that holds the tabs. */
export const TABLIST = { role: "tablist" } as const;

export type GiuProps = { accessibilityState: { selected: boolean }; "aria-pressed"?: boolean };

/**
 * A button that stays down once pressed: a filter chip, a chosen day, a stop
 * picked on the map. `selected` is invalid on `role="button"` on the web, and
 * a toggle button's state there is `aria-pressed`, which React Native does not
 * read; native announces `selected`, which the web never receives. So this is
 * the one helper that sends each platform its own spelling.
 */
export function giuState(pressed: boolean): GiuProps {
  return Platform.OS === "web" ? { accessibilityState: { selected: pressed }, "aria-pressed": pressed } : { accessibilityState: { selected: pressed } };
}

/** A control that opens and closes a region below it. */
export function expandState(expanded: boolean): { "aria-expanded": boolean } {
  return { "aria-expanded": expanded };
}

export type VungSongProps = { accessibilityLiveRegion: "polite" | "none"; "aria-busy"?: boolean };

/**
 * Spread onto the text of an answer that arrives word by word (QA UI-165): it
 * is announced once, whole, when it settles -- never once per tick, which on
 * a screen reader is a stutter of half-sentences.
 *
 * The web has the tool for it: a polite live region held `aria-busy` while the
 * words arrive, read when the flag drops. Native has no busy flag, so there the
 * line becomes a live region only once it has settled. The status line before
 * the first word («đang đọc…», «đang nghĩ…») is a plain polite region.
 */
export function vungSong(dangChay: boolean): VungSongProps {
  if (Platform.OS === "web") return { accessibilityLiveRegion: "polite", "aria-busy": dangChay };
  return { accessibilityLiveRegion: dangChay ? "none" : "polite" };
}
