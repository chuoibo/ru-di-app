import { Redirect, useLocalSearchParams } from "expo-router";

import { GroupWallLiveScreen } from "../../../src/rudi/screens/ky-niem/GroupWallLive";
import { useRudiSession } from "../../../src/rudi/session";
import { CuaDangNhap } from "../../../src/rudi/ui/CuaDangNhap";

function maNhom(id: unknown): string {
  if (typeof id === "string") return id;
  return "";
}

export default function WallRoute() {
  const params = useLocalSearchParams<{ id: string }>();
  const { phien, phienDaDoc } = useRudiSession();
  if (!phienDaDoc) return null;
  if (phien === null) return <CuaDangNhap />;
  const id = maNhom(params.id);
  if (id === "") return <Redirect href="/messages" />;
  return <GroupWallLiveScreen contextId={id} phien={phien} />;
}
