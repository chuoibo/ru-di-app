import { Redirect } from "expo-router";

import { CheckInScreen } from "../../src/rudi/screens/Outing";
import { useRudiSession } from "../../src/rudi/session";

/**
 * The experience build's check-in; live, «Tôi đã tới» lives on the outing.
 *
 * B5 (QC 24/09): opened straight on a real session (a deep link, a
 * notification), this route showed the experience build's demo data. With a
 * session it goes to the live screen that owns the same job; without one the
 * experience build is unchanged.
 */
export default function CheckInRoute() {
  const { phien, phienDaDoc } = useRudiSession();
  if (!phienDaDoc) return null;
  if (phien !== null) return <Redirect href="/(tabs)/plan" />;
  return <CheckInScreen />;
}
