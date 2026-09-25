import { Redirect } from "expo-router";

import { VotingScreen } from "../../../src/rudi/screens/Group";
import { useRudiSession } from "../../../src/rudi/session";

/**
 * A vote of the experience build; live votes are cards in the group chat.
 *
 * B5 (QC 24/09): opened straight on a real session (a deep link, a
 * notification), this route showed the experience build's demo data. With a
 * session it goes to the live screen that owns the same job; without one the
 * experience build is unchanged.
 */
export default function VoteRoute() {
  const { phien, phienDaDoc } = useRudiSession();
  if (!phienDaDoc) return null;
  if (phien !== null) return <Redirect href="/messages" />;
  return <VotingScreen />;
}
