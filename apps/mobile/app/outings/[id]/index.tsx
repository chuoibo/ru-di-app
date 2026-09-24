import { Redirect, useLocalSearchParams } from "expo-router";

import { nguCanhMo } from "../../../src/rudi/ngu-canh-mo";
import { OutingLiveScreen } from "../../../src/rudi/screens/keo/OutingLive";
import { useRudiSession } from "../../../src/rudi/session";

// Outings live on the server only; the fixture build has no route here.
// `?ctx=` opens an outing of a context that is not the current group: the plan
// a two-person sheet became lives in the pair (see `nguCanhMo`).
export default function OutingRoute() {
  const { phien, phienDaDoc } = useRudiSession();
  const params = useLocalSearchParams<{ ctx?: string }>();
  if (!phienDaDoc) return null;
  if (phien === null) return <Redirect href="/welcome" />;
  const mo = nguCanhMo(phien, params.ctx);
  if (mo.kieu === "tu-choi") return <Redirect href="/(tabs)/plan" />;
  if (mo.kieu === "hien-tai") return <OutingLiveScreen phien={phien} />;
  return <OutingLiveScreen key={mo.contextId} phien={{ ...phien, context_id: mo.contextId }} />;
}
