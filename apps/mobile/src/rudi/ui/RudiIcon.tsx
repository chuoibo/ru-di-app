import { Ionicons } from "@expo/vector-icons";
import type { ComponentProps } from "react";
import { View, type StyleProp, type TextStyle } from "react-native";

import { AssetIcon, type AssetIconName } from "../asset-pack/react-native/AssetIcon";

export type IconName = ComponentProps<typeof Ionicons>["name"];

/** Semantic aliases for the shipped paper glyphs, shared by controls and navigation. */
const paperIcons: Partial<Record<IconName, AssetIconName>> = {
  "compass-outline": "compass",
  "map-outline": "map",
  add: "add",
  "add-outline": "add",
  "chatbubble-outline": "chat",
  "chatbubbles-outline": "chat",
  "chatbox-outline": "chat",
  "chatbubble-ellipses-outline": "chat",
  "chatbox-ellipses-outline": "chat",
  "person-outline": "profile",
  "person-circle-outline": "profile",
  search: "search",
  "search-outline": "search",
  "heart-outline": "heart",
  "bookmark-outline": "bookmark",
  "notifications-outline": "bell",
  "calendar-outline": "calendar",
  "time-outline": "clock",
  "location-outline": "pin",
  "image-outline": "photo",
  "camera-outline": "camera",
  "videocam-outline": "video",
  "send-outline": "send",
  "paper-plane-outline": "send",
  "mail-outline": "envelope",
  "people-outline": "people",
  "person-add-outline": "person-add",
  "settings-outline": "settings",
  "options-outline": "settings",
  "lock-closed-outline": "lock",
  "shield-checkmark-outline": "shield",
  "eye-outline": "eye",
  "eye-off-outline": "eye-off",
  close: "close",
  "close-outline": "close",
  "chevron-back": "chevron-left",
  "chevron-back-outline": "chevron-left",
  "chevron-forward": "chevron-right",
  "chevron-forward-outline": "chevron-right",
  "chevron-down": "chevron-down",
  "chevron-down-outline": "chevron-down",
  "arrow-back": "arrow-left",
  "arrow-back-outline": "arrow-left",
  menu: "menu",
  "menu-outline": "menu",
  "ellipsis-vertical": "more",
  checkmark: "check",
  "checkmark-outline": "check",
  "receipt-outline": "receipt",
  "journal-outline": "ledger",
  "push-outline": "upload",
  "download-outline": "download",
  pencil: "edit",
  "pencil-outline": "edit",
  "create-outline": "edit",
  "trash-outline": "trash",
  "filter-outline": "filter",
  "funnel-outline": "filter",
  "globe-outline": "globe",
  "cafe-outline": "coffee",
  "restaurant-outline": "food",
  "play-outline": "play",
  "leaf-outline": "leaf",
  sparkles: "sparkles",
  "sparkles-outline": "sparkles",
  "flag-outline": "report",
  "log-out-outline": "logout",
  "exit-outline": "logout",
  link: "link",
  "link-outline": "link",
  "copy-outline": "copy",
  "star-outline": "star",
  "book-outline": "book",
  "folder-outline": "folder",
  refresh: "refresh",
  "refresh-outline": "refresh",
  "information-circle-outline": "info",
  "warning-outline": "warning",
  "checkmark-circle-outline": "check-circle",
  "remove-circle-outline": "minus-circle",
  "bar-chart-outline": "poll",
  "stats-chart-outline": "poll",
};

/**
 * The host owns the label, hit target, tone and selected state. The pack's
 * silhouettes are outlines, so filled hearts, bookmarks and active tabs keep
 * their existing filled glyph. A missing semantic match keeps Ionicons too.
 */
export function RudiIcon({
  name,
  color,
  size = 24,
  assetIcon,
  assetSize = size,
  style,
}: {
  name: IconName;
  color: string;
  size?: number;
  /** An explicit paper glyph chosen by a host that already owns its meaning. */
  assetIcon?: AssetIconName;
  /** Existing icon-only controls give paper glyphs a little more room. */
  assetSize?: number;
  style?: StyleProp<TextStyle>;
}) {
  const paper = assetIcon ?? paperIcons[name];
  if (!paper) return <Ionicons accessible={false} aria-hidden color={color} name={name} size={size} style={style} />;
  const glyph = <AssetIcon color={color} name={paper} size={assetSize} />;
  // A layout style belongs to a box; the glyph keeps its authored square.
  return style ? <View accessible={false} aria-hidden pointerEvents="none" style={style}>{glyph}</View> : glyph;
}
