import { useLocalSearchParams } from "expo-router";

import { KhongGianGiayScreen } from "../../../src/rudi/screens/hai-nguoi/KhongGianGiay";
import { useRudiSession } from "../../../src/rudi/session";

/**
 * The paper surface of a two-person notebook. Slice 1 renders the fixture
 * notebook on every build; Phase 4 adds the live screen behind `phien`.
 */
export default function ToGiayRoute() {
  const params = useLocalSearchParams<{ id: string; ru?: string }>();
  const { phienDaDoc } = useRudiSession();
  if (!phienDaDoc) return null;
  const id = typeof params.id === "string" ? params.id : "cap-demo";
  return <KhongGianGiayScreen contextId={id} ruNgay={params.ru === "1"} />;
}
