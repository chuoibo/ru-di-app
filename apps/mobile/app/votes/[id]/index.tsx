import { Redirect } from "expo-router";

import { useRudiSession } from "../../../src/rudi/session";
import { CuaDangNhap } from "../../../src/rudi/ui/CuaDangNhap";

/**
 * An old vote link. Votes are cards in the group chat now, so a session goes
 * to the conversation list and no session goes through the sign-in door.
 */
export default function VoteRoute() {
  const { phien, phienDaDoc } = useRudiSession();
  if (!phienDaDoc) return null;
  if (phien === null) return <CuaDangNhap tiep="/messages" />;
  return <Redirect href="/messages" />;
}
