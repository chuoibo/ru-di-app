import { Redirect, useLocalSearchParams } from "expo-router";

import { duongKeoCu } from "../../../src/rudi/duong-cu";
import { useRudiSession } from "../../../src/rudi/session";
import { CuaDangNhap } from "../../../src/rudi/ui/CuaDangNhap";

/** An old trip link: the trip is an outing now, so it opens there. */
export default function TripRoute() {
  const { phien, phienDaDoc } = useRudiSession();
  const { id, ctx } = useLocalSearchParams<{ id?: string; ctx?: string }>();
  if (!phienDaDoc) return null;
  if (phien === null) return <CuaDangNhap />;
  return <Redirect href={duongKeoCu(id, ctx) as never} />;
}
