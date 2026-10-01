

import { PickOutingLiveScreen } from "../../src/rudi/screens/keo/PickOutingLive";
import { useRudiSession } from "../../src/rudi/session";
import { CuaDangNhap } from "../../src/rudi/ui/CuaDangNhap";

export default function PickOutingRoute() {
  const { phien, phienDaDoc } = useRudiSession();
  if (!phienDaDoc) return null;
  if (phien === null) return <CuaDangNhap />;
  return <PickOutingLiveScreen phien={phien} />;
}
