import { Redirect, useLocalSearchParams } from "expo-router";

import { GroupChatLiveScreen } from "../../../src/rudi/screens/chat/GroupChatLive";
import { useRudiSession } from "../../../src/rudi/session";
import { chatRoute } from "../../../src/rudi/chat/chat-route";
import { CuaDangNhap } from "../../../src/rudi/ui/CuaDangNhap";

export default function GroupChatRoute() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const { phien, phienDaDoc } = useRudiSession();
  if (!phienDaDoc) return null;
  const route = chatRoute(typeof id === "string" ? id : undefined, phien !== null);
  if (route === "messages") return <Redirect href="/messages" />;
  if (route === "login" || phien === null) return <CuaDangNhap tiep={`/groups/${id}/chat`} />;
  // Keyed by conversation: a `rudi://groups/<id>/chat` link opened while
  // another chat is in front changes the param IN PLACE, and a screen that
  // survived that carried its notice, draft and quoted message into the next
  // group. A remount empties all of it; the hook's generations handle what a
  // remount cannot reach, the replies still in flight (audit 09/09, F42).
  return <GroupChatLiveScreen key={`${phien.person_id}:${id}`} contextId={id} />;
}
