import { Redirect, useLocalSearchParams } from "expo-router";

import { nguCanhMo } from "../../../src/rudi/ngu-canh-mo";
import { ChiaBillLiveScreen } from "../../../src/rudi/screens/chia-bill/ChiaBillLive";
import { useRudiSession } from "../../../src/rudi/session";
import { CuaDangNhap } from "../../../src/rudi/ui/CuaDangNhap";

// The whole bill flow runs on the server in one stepper; no session, the
// sign-in door.
// `?ctx=` splits the bill in the context an outing belongs to (a pair's plan is
// split in the pair, see `nguCanhMo`); `?dip=` names the occasion after it.
export default function ReceiptReviewRoute() {
  const { phien, phienDaDoc } = useRudiSession();
  const params = useLocalSearchParams<{ ctx?: string; dip?: string }>();
  if (!phienDaDoc) return null;
  if (phien === null) return <CuaDangNhap />;
  const dip = typeof params.dip === "string" ? params.dip.slice(0, 120) : undefined;
  const mo = nguCanhMo(phien, params.ctx);
  if (mo.kieu === "tu-choi") return <Redirect href="/(tabs)/plan" />;
  if (mo.kieu === "hien-tai") return <ChiaBillLiveScreen dip={dip} phien={phien} />;
  return <ChiaBillLiveScreen dip={dip} key={mo.contextId} phien={{ ...phien, context_id: mo.contextId }} />;
}
