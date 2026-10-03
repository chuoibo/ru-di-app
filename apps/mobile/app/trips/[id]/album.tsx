import { Redirect, useLocalSearchParams } from "expo-router";

import { TripAlbumLiveScreen } from "../../../src/rudi/screens/ky-niem/AlbumLive";
import { useRudiSession } from "../../../src/rudi/session";
import { CuaDangNhap } from "../../../src/rudi/ui/CuaDangNhap";

function chuoi(x: unknown): string {
  if (typeof x === "string") return x;
  return "";
}

export default function TripAlbumRoute() {
  const params = useLocalSearchParams<{ id: string; ctx?: string }>();
  const { phien, phienDaDoc } = useRudiSession();
  if (!phienDaDoc) return null;
  if (phien === null) return <CuaDangNhap />;
  const outingId = chuoi(params.id);
  const ctxParam = chuoi(params.ctx);
  // The album is the group's: the shelf passes `ctx`; a deep link without it
  // falls back to the session's group.
  const contextId = ctxParam !== "" ? ctxParam : phien.context_id;
  if (outingId === "" || contextId === null) return <Redirect href="/(tabs)/plan" />;
  return <TripAlbumLiveScreen contextId={contextId} outingId={outingId} phien={phien} />;
}
