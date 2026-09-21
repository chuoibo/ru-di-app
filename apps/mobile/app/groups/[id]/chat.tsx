import { Redirect, useLocalSearchParams } from "expo-router";

import { GroupChatScreen } from "../../../src/rudi/screens/Group";
import { GroupChatLiveScreen } from "../../../src/rudi/screens/chat/GroupChatLive";
import { useRudiSession } from "../../../src/rudi/session";

export default function GroupChatRoute() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const { phien, phienDaDoc } = useRudiSession();
  if (!phienDaDoc) return null;
  // A real session gets the real conversation; the fixture build keeps the
  // fixture chat the default Maestro table drives.
  if (phien !== null) {
    if (typeof id !== "string") return <Redirect href="/messages" />;
    // Keyed by conversation: a `rudi://groups/<id>/chat` link opened while
    // another chat is in front changes the param IN PLACE, and a screen that
    // survived that carried its notice, draft and quoted message into the next
    // group. A remount empties all of it; the hook's generations handle what a
    // remount cannot reach, the replies still in flight (audit 09/09, F42).
    return <GroupChatLiveScreen key={`${phien.person_id}:${id}`} contextId={id} />;
  }
  // The fixture chat is Team Đà Lạt for every id but one: the fixture pair
  // notebook, which puts its own pinned line under the title.
  return <GroupChatScreen contextId={typeof id === "string" ? id : undefined} />;
}
