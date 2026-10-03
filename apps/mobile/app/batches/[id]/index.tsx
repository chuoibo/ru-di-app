import { Redirect, useLocalSearchParams } from "expo-router";

import { nguCanhMo } from "../../../src/rudi/ngu-canh-mo";
import { DotThuLiveScreen } from "../../../src/rudi/screens/dot-thu/DotThuLive";
import { useRudiSession } from "../../../src/rudi/session";
import { CuaDangNhap } from "../../../src/rudi/ui/CuaDangNhap";

function maDot(id: unknown): string {
  if (typeof id === "string") return id;
  return "";
}

// `?ctx=` names the context the round belongs to. A pair is never the current
// group (ADR-0021 §2.5), and the screen reads the round's state from its
// context's list of rounds: opened without it, a pair's published round read
// as «Chưa phát» and offered to publish again (S1 evidence, 25/09). Only a
// context the person is active in; any other falls back to the current one.
export default function BatchRoute() {
  const params = useLocalSearchParams<{ id: string; ctx?: string }>();
  const { phien, phienDaDoc } = useRudiSession();
  if (!phienDaDoc) return null;
  const id = maDot(params.id);
  if (phien !== null && id !== "") {
    const mo = nguCanhMo(phien, params.ctx);
    if (mo.kieu === "khac") return <DotThuLiveScreen batchId={id} key={mo.contextId} phien={{ ...phien, context_id: mo.contextId }} />;
    return <DotThuLiveScreen batchId={id} phien={phien} />;
  }
  if (phien === null) return <CuaDangNhap />;
  return <Redirect href="/(tabs)/plan" />;
}
