import { Redirect, useLocalSearchParams } from "expo-router";
import { DiaryScreen } from "../../src/rudi/diary/DiaryScreen";
import { useRudiSession } from "../../src/rudi/session";
export default function DiaryRoute() {
  const { phien, phienDaDoc } = useRudiSession(); const { id } = useLocalSearchParams<{ id: string }>();
  if (!phienDaDoc) return null;
  if (!phien) return <Redirect href="/welcome" />;
  return <DiaryScreen key={id} person={phien.person_id} id={id} />;
}
