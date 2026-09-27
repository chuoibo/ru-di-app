import { Redirect } from "expo-router";
import { useEffect, useState } from "react";
import { ActivityIndicator, Text } from "react-native";
import { ApiError, translatedAsActor } from "../../src/api";
import { COMMUNITY_ERRORS } from "../../src/rudi/community/api";
import { DangBaiScreen } from "../../src/rudi/screens/nguoi/DangBaiScreen";
import { useRudiSession } from "../../src/rudi/session";
import { RudiButton, RudiScreen } from "../../src/rudi/ui";

/** Keep the established composer available while the community rollout is off. */
export default function NewWallPost() {
  const { phien } = useRudiSession();
  const [enabled, setEnabled] = useState<boolean | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [retry, setRetry] = useState(0);
  useEffect(() => {
    let current = true;
    setError(null);
    if (!phien) { setEnabled(false); return; }
    void translatedAsActor(COMMUNITY_ERRORS, "/v2/community/preferences", { actorId: phien.person_id, method: "GET" })
      .then(() => { if (current) setEnabled(true); })
      .catch((problem) => {
        if (!current) return;
        if (problem instanceof ApiError && problem.status === 404) setEnabled(false);
        else setError("Chưa mở được trang viết. Kiểm tra kết nối rồi thử lại nhé.");
      });
    return () => { current = false; };
  }, [phien, retry]);
  if (error) return <RudiScreen><Text accessibilityRole="alert">{error}</Text><RudiButton label="Thử lại" onPress={() => setRetry((value) => value + 1)} /></RudiScreen>;
  return enabled === null ? <ActivityIndicator accessibilityLabel="Đang mở trang viết" /> : enabled
    ? <Redirect href={{ pathname: "/community/new", params: { wall: "1" } } as never} />
    : <DangBaiScreen />;
}
