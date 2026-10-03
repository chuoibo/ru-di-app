import { Redirect, useLocalSearchParams } from "expo-router";

import { useRudiSession } from "../../../src/rudi/session";
import { CuaDangNhap } from "../../../src/rudi/ui/CuaDangNhap";

/** An old trip link: the trip is an outing now, so it opens there. */
export default function TripRoute() {
  const { phien, phienDaDoc } = useRudiSession();
  const { id } = useLocalSearchParams<{ id?: string }>();
  if (!phienDaDoc) return null;
  if (phien === null) return <CuaDangNhap />;
  return <Redirect href={(typeof id === "string" && id !== "" ? `/outings/${id}` : "/(tabs)/plan") as never} />;
}
