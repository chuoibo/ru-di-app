import { Redirect } from "expo-router";

import { AiMatchScreen } from "../src/rudi/screens/Discovery";
import { useRudiSession } from "../src/rudi/session";

/**
 * The group taste match of the experience build.
 *
 * B5 (QC 24/09): opened straight on a real session (a deep link, a
 * notification), this route showed the experience build's demo data. With a
 * session it goes to the live screen that owns the same job; without one the
 * experience build is unchanged.
 */
export default function AiMatchRoute() {
  const { phien, phienDaDoc } = useRudiSession();
  if (!phienDaDoc) return null;
  if (phien !== null) return <Redirect href="/explore" />;
  return <AiMatchScreen />;
}
