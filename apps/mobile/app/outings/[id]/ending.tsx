import { useLocalSearchParams } from "expo-router";
import { EndingScreen } from "../../../src/rudi/diary/EndingScreen";
import { useRudiSession } from "../../../src/rudi/session";
import { CuaDangNhap } from "../../../src/rudi/ui/CuaDangNhap";
export default function EndingRoute() {
  const { phien, phienDaDoc } = useRudiSession(); const { id } = useLocalSearchParams<{ id: string }>();
  if (!phienDaDoc) return null;
  if (!phien) return <CuaDangNhap />;
  return <EndingScreen key={id} person={phien.person_id} outing={id} />;
}
