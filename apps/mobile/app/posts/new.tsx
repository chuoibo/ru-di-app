import { Redirect } from "expo-router";
import { useEffect, useState } from "react";
import { ApiError, translatedAsActor } from "../../src/api";
import { SkeletonLines } from "../../src/rudi/ui/Skeleton";
import { ErrorState } from "../../src/rudi/ui/ErrorState";
import { COMMUNITY_ERRORS } from "../../src/rudi/community/api";
import { DangBaiScreen } from "../../src/rudi/screens/nguoi/DangBaiScreen";
import { useRudiSession } from "../../src/rudi/session";
import { RudiScreen, TopBar } from "../../src/rudi/ui";

/** Keep the established composer available while the community rollout is off. */
export default function NewWallPost() {
  const { phien } = useRudiSession();
  const [enabled, setEnabled] = useState<boolean | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [retry, setRetry] = useState(0);
  useEffect(() => {
    let current = true;
    setError(null); setEnabled(null);
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
  if (error) return <RudiScreen material="quiet" cot="form"><TopBar title="Kể một khoảnh khắc"/><ErrorState title="Chưa mở được trang viết" body={error} onRetry={() => setRetry((n) => n + 1)}/></RudiScreen>;
  return enabled === null ? <RudiScreen material="quiet" cot="form"><TopBar title="Kể một khoảnh khắc"/><SkeletonLines lines={5}/></RudiScreen> : enabled
    ? <Redirect href={{ pathname: "/community/new", params: { wall: "1" } } as never} />
    : <DangBaiScreen />;
}
