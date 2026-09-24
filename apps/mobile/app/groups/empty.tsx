import { Redirect } from "expo-router";

// The stand-alone «Chưa có nhóm nào» screen had no tab bar; `manDau` now sends
// a signed-in person with no group to the Tin nhắn tab, which carries the same
// empty state and invitations. Old links and pushes still resolve.
export default function GroupsEmptyRoute() {
  return <Redirect href="/messages" />;
}
