import { Redirect } from "expo-router";

import { useRudiSession } from "../src/rudi/session";
import { CuaDangNhap } from "../src/rudi/ui/CuaDangNhap";

/** An old link: place suggestions live on Khám phá. */
export default function AiMatchRoute() {
  const { phien, phienDaDoc } = useRudiSession();
  if (!phienDaDoc) return null;
  if (phien === null) return <CuaDangNhap tiep="/explore" />;
  return <Redirect href="/explore" />;
}
