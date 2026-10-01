import { useLocalSearchParams } from "expo-router";
import { DiaryScreen } from "../../src/rudi/diary/DiaryScreen";
import { useRudiSession } from "../../src/rudi/session";
import { CuaDangNhap } from "../../src/rudi/ui/CuaDangNhap";
export default function DiaryRoute() {
  const { phien, phienDaDoc } = useRudiSession(); const { id } = useLocalSearchParams<{ id: string }>();
  if (!phienDaDoc) return null;
  if (!phien) return <CuaDangNhap />;
  return <DiaryScreen key={id} person={phien.person_id} id={id} />;
}
