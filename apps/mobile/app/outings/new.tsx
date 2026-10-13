import { CreateOutingLiveScreen } from "../../src/rudi/screens/keo/CreateOutingLive";
import { useRudiSession } from "../../src/rudi/session";
import { CuaDangNhap } from "../../src/rudi/ui/CuaDangNhap";
import { Redirect, useLocalSearchParams } from "expo-router";

export default function NewOutingRoute() {
  const { phien, phienDaDoc } = useRudiSession();
  const params = useLocalSearchParams<{ contextId?: string; sourceMessageId?: string; place?: string; initialTitle?: string }>();
  if (!phienDaDoc) return null;
  if (phien === null) return <CuaDangNhap />;
  const contextId = typeof params.contextId === "string" ? params.contextId : phien.context_id;
  if (contextId !== phien.context_id && !phien.contexts?.some((context) => context.id === contextId && context.my_state === "active")) return <Redirect href="/(tabs)/plan" />;
  const initialTitle = typeof params.initialTitle === "string" && params.initialTitle.length <= 80 && !/[\u0000-\u001f\u007f\r\n]/.test(params.initialTitle) ? params.initialTitle.trim().normalize("NFC") : undefined;
  return <CreateOutingLiveScreen key={`${contextId}:${params.sourceMessageId ?? "new"}`} phien={{ ...phien, context_id: contextId }} initialTitle={initialTitle} placeId={typeof params.place === "string" && params.place !== "" ? params.place : undefined} sourceMessageId={typeof params.sourceMessageId === "string" ? params.sourceMessageId : undefined} />;
}
