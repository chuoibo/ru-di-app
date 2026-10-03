import { AchievementsLiveScreen } from "../src/rudi/screens/ky-niem/AchievementsLive";
import { useRudiSession } from "../src/rudi/session";
import { CuaDangNhap } from "../src/rudi/ui/CuaDangNhap";

export default function AchievementsRoute() {
  const { phien, phienDaDoc } = useRudiSession();
  if (!phienDaDoc) return null;
  if (phien === null) return <CuaDangNhap />;
  return <AchievementsLiveScreen phien={phien} />;
}
