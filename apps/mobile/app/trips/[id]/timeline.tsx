import { Redirect, useLocalSearchParams } from "expo-router";

import { TripTimelineScreen } from "../../../src/rudi/screens/Outing";
import { useRudiSession } from "../../../src/rudi/session";

/**
 * The experience build's trip timeline; live, an outing's stops are its
 * timeline. Opened on a real session, this route showed the demo timeline under
 * a real outing's id, with no demo label and no way back (QA UI-035), while its
 * sibling `itinerary` already sent a session to the live outing. Same rule here.
 */
export default function TimelineRoute() {
  const { phien, phienDaDoc } = useRudiSession();
  const { id } = useLocalSearchParams<{ id?: string }>();
  if (!phienDaDoc) return null;
  if (phien !== null) return <Redirect href={(typeof id === "string" && id !== "" ? `/outings/${id}` : "/(tabs)/plan") as never} />;
  return <TripTimelineScreen />;
}
