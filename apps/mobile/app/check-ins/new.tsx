import { Redirect } from "expo-router";

import { useRudiSession } from "../../src/rudi/session";
import { CuaDangNhap } from "../../src/rudi/ui/CuaDangNhap";

/** An old check-in link: «Tôi đã tới» lives on the outing, under Lên plan. */
export default function CheckInRoute() {
  const { phien, phienDaDoc } = useRudiSession();
  if (!phienDaDoc) return null;
  if (phien === null) return <CuaDangNhap tiep="/plan" />;
  return <Redirect href="/(tabs)/plan" />;
}
