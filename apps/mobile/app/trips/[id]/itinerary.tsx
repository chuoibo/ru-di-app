import { Redirect, useLocalSearchParams } from "expo-router";

import { AiItineraryScreen } from "../../../src/rudi/screens/Group";
import { useRudiSession } from "../../../src/rudi/session";

/**
 * The experience build's AI itinerary; live, an outing's stops are its itinerary.
 *
 * B5 (QC 24/09): opened straight on a real session (a deep link, a
 * notification), this route showed the experience build's demo data. With a
 * session it goes to the live screen that owns the same job; without one the
 * experience build is unchanged.
 */
export default function ItineraryRoute() {
  const { phien, phienDaDoc } = useRudiSession();
  const { id } = useLocalSearchParams<{ id?: string }>();
  if (!phienDaDoc) return null;
  if (phien !== null) return <Redirect href={(typeof id === "string" && id !== "" ? `/outings/${id}` : "/(tabs)/plan") as never} />;
  return <AiItineraryScreen />;
}
