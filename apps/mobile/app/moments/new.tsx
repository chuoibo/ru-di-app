import { Redirect, useLocalSearchParams } from "expo-router";

import { nguCanhMo } from "../../src/rudi/ngu-canh-mo";
import { ShareMomentScreen } from "../../src/rudi/screens/Memories";
import { ShareMomentLiveScreen } from "../../src/rudi/screens/ky-niem/ShareMomentLive";
import { useRudiSession } from "../../src/rudi/session";

// `?ctx=` posts into the wall the person came from -- a group that is not the
// current one, or a pair keeping a photo of their evening (ADR-0021 §2.5 lets
// memories into a pair). Without it the photo went to the CURRENT group
// whatever wall the button was pressed on.
export default function ShareMomentRoute() {
  const { phien, phienDaDoc } = useRudiSession();
  const params = useLocalSearchParams<{ ctx?: string }>();
  if (!phienDaDoc) return null;
  if (phien === null) return <ShareMomentScreen />;
  const mo = nguCanhMo(phien, params.ctx);
  if (mo.kieu === "tu-choi") return <Redirect href="/(tabs)/messages" />;
  if (mo.kieu === "hien-tai") return <ShareMomentLiveScreen phien={phien} />;
  return <ShareMomentLiveScreen key={mo.contextId} phien={{ ...phien, context_id: mo.contextId }} />;
}
