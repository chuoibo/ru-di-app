import { Redirect } from "expo-router";

import { useRudiSession } from "../../../src/rudi/session";
import { CuaDangNhap } from "../../../src/rudi/ui/CuaDangNhap";

/** An old link: assigning items happens inside the live bill split. */
export default function OcrAssignmentRoute() {
  const { phien, phienDaDoc } = useRudiSession();
  if (!phienDaDoc) return null;
  if (phien === null) return <CuaDangNhap tiep="/smart-split/moi/review" />;
  return <Redirect href="/smart-split/moi/review" />;
}
