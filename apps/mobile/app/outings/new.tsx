import { CreateOutingScreen } from "../../src/rudi/screens/Outing";
import { CreateOutingLiveScreen } from "../../src/rudi/screens/keo/CreateOutingLive";
import { useRudiSession } from "../../src/rudi/session";
import { Redirect, useLocalSearchParams } from "expo-router";

export default function NewOutingRoute() {
  const { phien, phienDaDoc } = useRudiSession();
  const params = useLocalSearchParams<{ contextId?: string; sourceMessageId?: string }>();
  if (!phienDaDoc) return null;
  if (phien !== null) {
    const contextId = typeof params.contextId === "string" ? params.contextId : phien.context_id;
    if (contextId !== phien.context_id && !phien.contexts?.some((context) => context.id === contextId && context.my_state === "active")) return <Redirect href="/(tabs)/plan" />;
    return <CreateOutingLiveScreen key={`${contextId}:${params.sourceMessageId ?? "new"}`} phien={{ ...phien, context_id: contextId }} sourceMessageId={typeof params.sourceMessageId === "string" ? params.sourceMessageId : undefined} />;
  }
  return <CreateOutingScreen />;
}
