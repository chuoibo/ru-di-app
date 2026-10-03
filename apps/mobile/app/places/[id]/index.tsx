import { PlaceDetailLiveScreen } from "../../../src/rudi/screens/explore/PlaceDetailLive";
import { useRudiSession } from "../../../src/rudi/session";
import { CuaDangNhap } from "../../../src/rudi/ui/CuaDangNhap";

export default function PlaceDetailRoute() {
  const { phien, phienDaDoc } = useRudiSession();
  if (!phienDaDoc) return null;
  if (phien === null) return <CuaDangNhap />;
  return <PlaceDetailLiveScreen phien={phien} />;
}
