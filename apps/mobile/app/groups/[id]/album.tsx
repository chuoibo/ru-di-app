import { Redirect, useLocalSearchParams } from "expo-router";

import { AlbumNhomLiveScreen } from "../../../src/rudi/screens/ky-niem/AlbumLive";
import { useRudiSession } from "../../../src/rudi/session";
import { CuaDangNhap } from "../../../src/rudi/ui/CuaDangNhap";

function maNhom(id: unknown): string {
  if (typeof id === "string") return id;
  return "";
}

export default function GroupAlbumRoute() {
  const params = useLocalSearchParams<{ id: string }>();
  const { phien, phienDaDoc } = useRudiSession();
  if (!phienDaDoc) return null;
  const id = maNhom(params.id);
  if (phien !== null && id !== "") return <AlbumNhomLiveScreen contextId={id} phien={phien} />;
  // A cold link without a session goes through the sign-in door (UI-121).
  if (phien === null) return <CuaDangNhap />;
  return <Redirect href="/messages" />;
}
