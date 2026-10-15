import { Redirect, useLocalSearchParams } from "expo-router";

import { duongChiaBillCu } from "../../../src/rudi/duong-cu";
import { useRudiSession } from "../../../src/rudi/session";
import { CuaDangNhap } from "../../../src/rudi/ui/CuaDangNhap";

/** An old link: assigning items happens inside the live bill split. */
export default function OcrAssignmentRoute() {
  const { phien, phienDaDoc } = useRudiSession();
  const { id, ctx, dip } = useLocalSearchParams<{ id?: string; ctx?: string; dip?: string }>();
  const href = duongChiaBillCu(id, ctx, dip);
  if (!phienDaDoc) return null;
  if (phien === null) return <CuaDangNhap tiep={href} />;
  return <Redirect href={href as never} />;
}
