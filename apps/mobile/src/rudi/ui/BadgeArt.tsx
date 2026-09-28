import { Ionicons } from "@expo/vector-icons";
import { Image } from "expo-image";
import { StyleSheet, View, type StyleProp, type ViewStyle } from "react-native";

import { useRudiTheme } from "../theme";

/** Stable reward IDs from the Go achievement contract. */
export const BADGE_ART = {
  first_checkin: require("../../../assets/rudi/badges/first_checkin.png"),
  first_photo: require("../../../assets/rudi/badges/first_photo.png"),
  first_story: require("../../../assets/rudi/badges/first_story.png"),
  first_together: require("../../../assets/rudi/badges/first_together.png"),
  many_turns: require("../../../assets/rudi/badges/many_turns.png"),
  open_map: require("../../../assets/rudi/badges/open_map.png"),
  photos_remain: require("../../../assets/rudi/badges/photos_remain.png"),
  storyteller: require("../../../assets/rudi/badges/storyteller.png"),
  again_together: require("../../../assets/rudi/badges/again_together.png"),
  full_house: require("../../../assets/rudi/badges/full_house.png"),
  map_becomes_page: require("../../../assets/rudi/badges/map_becomes_page.png"),
  shared_memory: require("../../../assets/rudi/badges/shared_memory.png"),
  whole_journey: require("../../../assets/rudi/badges/whole_journey.png"),
} as const;

export type AchievementBadgeId = keyof typeof BADGE_ART;
export type BadgeArtState = "locked" | "progress" | "unlocked" | "unknown";

export interface BadgeArtProps {
  badgeId: AchievementBadgeId | string;
  state: BadgeArtState;
  label: string;
  size?: number;
  /** Badge art is static; the caller may animate its surrounding grant seal. */
  reduceMotion?: boolean;
  style?: StyleProp<ViewStyle>;
  testID?: string;
}

const STATE_LABEL: Record<BadgeArtState, string> = {
  locked: "chưa đạt",
  progress: "đang chinh phục",
  unlocked: "đã đạt",
  unknown: "chưa đo được",
};

export function BadgeArt({ badgeId, state, label, size = 56, style, testID }: BadgeArtProps) {
  const { colors } = useRudiTheme();
  const art = Object.prototype.hasOwnProperty.call(BADGE_ART, badgeId)
    ? BADGE_ART[badgeId as AchievementBadgeId]
    : undefined;
  const muted = state === "locked" || state === "unknown";
  const overlay = state === "locked" ? "lock-closed" : state === "unknown" || art === undefined ? "help" : undefined;

  return (
    <View
      accessible
      accessibilityRole="image"
      accessibilityLabel={`${label}, ${STATE_LABEL[state]}`}
      testID={testID}
      style={[
        styles.frame,
        { width: size, height: size, borderRadius: size / 2, backgroundColor: colors.paper, borderColor: muted ? colors.line : colors.paper },
        style,
      ]}
    >
      {art !== undefined ? (
        <Image
          accessible={false}
          source={art}
          contentFit="contain"
          style={{ width: size, height: size, opacity: state === "unlocked" ? 1 : state === "progress" ? 0.78 : 0.32 }}
        />
      ) : null}
      {overlay !== undefined ? (
        <View pointerEvents="none" style={[styles.overlay, { borderRadius: size / 2 }]}>
          <View style={[styles.status, { backgroundColor: colors.paper, borderColor: colors.line }]}>
            <Ionicons accessible={false} name={overlay} size={Math.max(13, Math.round(size * 0.25))} color={colors.inkSoft} />
          </View>
        </View>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  frame: {
    alignItems: "center",
    justifyContent: "center",
    overflow: "hidden",
    borderWidth: 1,
  },
  overlay: {
    ...StyleSheet.absoluteFill,
    alignItems: "center",
    justifyContent: "center",
  },
  status: {
    width: 28,
    height: 28,
    alignItems: "center",
    justifyContent: "center",
    borderRadius: 14,
    borderWidth: 1,
  },
});
