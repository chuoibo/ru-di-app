import { useLocalSearchParams } from "expo-router";
import { useEffect, useState } from "react";
import { BaiChiTietScreen } from "../../src/rudi/screens/tuong/BaiChiTietScreen";
import { PostDetail } from "../../src/rudi/community/PostDetail";
import { getPost, COMMUNITY_ERRORS } from "../../src/rudi/community/api";
import { ApiError, newAttempt, translatedAsActor } from "../../src/api";
import { useRudiSession } from "../../src/rudi/session";
import { RudiScreen, TopBar } from "../../src/rudi/ui";
import { SkeletonLines } from "../../src/rudi/ui/Skeleton";
import { ErrorState } from "../../src/rudi/ui/ErrorState";

/** Both walls open the same conversation; legacy posts retain their reader. */
export default function PostRoute() {
  const { id } = useLocalSearchParams<{ id: string }>(); const { phien } = useRudiSession(); const [managed, setManaged] = useState<boolean | null>(null);
  const [communityAvailable, setCommunityAvailable] = useState(false);
  const [error, setError] = useState<string | null>(null); const [retry, setRetry] = useState(0);
  useEffect(() => {
    let current = true; setManaged(null); setCommunityAvailable(false); setError(null);
    if (!phien || !id) { setManaged(false); return; }
    void getPost(phien.person_id, id).then((post) => { if (current) { setManaged(post.revision > 0); setCommunityAvailable(true); } }).catch((problem) => {
      if (!current) return;
      if (problem instanceof ApiError && problem.status === 404) setManaged(false);
      else setError(problem instanceof Error ? problem.message : "Chưa mở được câu chuyện. Bạn thử lại nhé.");
    });
    return () => { current = false; };
  }, [phien, id, retry]);
  const submitLegacy = communityAvailable && phien && id ? async () => {
    await translatedAsActor(COMMUNITY_ERRORS, `/v2/community/posts/${id}/submit`, { actorId: phien.person_id, method: "POST", attempt: newAttempt() });
    setManaged(true);
  } : undefined;
  if (managed === null) return <RudiScreen material="quiet" cot="doc"><TopBar title="Một câu chuyện"/>{error ? <ErrorState title="Chưa mở được câu chuyện" body={error} onRetry={() => setRetry((n) => n + 1)}/> : <SkeletonLines lines={5}/>}</RudiScreen>;
  return managed ? <PostDetail/> : <BaiChiTietScreen onShareCommunity={submitLegacy}/>;
}
