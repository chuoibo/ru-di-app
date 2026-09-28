import { Redirect, useLocalSearchParams } from "expo-router";
import { EndingScreen } from "../../../src/rudi/diary/EndingScreen";
import { useRudiSession } from "../../../src/rudi/session";
export default function EndingRoute() {
  const { phien, phienDaDoc } = useRudiSession(); const { id } = useLocalSearchParams<{ id: string }>();
  if (!phienDaDoc) return null;
  if (!phien) return <Redirect href="/welcome" />;
  return <EndingScreen key={id} person={phien.person_id} outing={id} />;
}
