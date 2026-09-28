import { useLocalSearchParams } from "expo-router";
import { useEffect, useState } from "react";
import { ActivityIndicator } from "react-native";
import { BaiChiTietScreen } from "../../src/rudi/screens/tuong/BaiChiTietScreen";
import { PostDetail } from "../../src/rudi/community/PostDetail";
import { getPost } from "../../src/rudi/community/api";
import { COMMUNITY_ERRORS } from "../../src/rudi/community/api";
import { newAttempt, translatedAsActor } from "../../src/api";
import { useRudiSession } from "../../src/rudi/session";

/** Both walls open the same conversation; legacy posts retain their reader. */
export default function PostRoute() {
  const { id } = useLocalSearchParams<{ id: string }>(); const { phien } = useRudiSession(); const [managed, setManaged] = useState<boolean | null>(null);
  const [communityAvailable, setCommunityAvailable] = useState(false);
  useEffect(() => { let current = true; setManaged(null); setCommunityAvailable(false); if (!phien || !id) { setManaged(false); return; } void getPost(phien.person_id, id).then((p) => { if (current) { setManaged(p.revision > 0); setCommunityAvailable(true); } }).catch(() => { if (current) setManaged(false); }); return () => { current = false; }; }, [phien, id]);
  const submitLegacy = communityAvailable && phien && id ? async () => {
    await translatedAsActor(COMMUNITY_ERRORS, `/v2/community/posts/${id}/submit`, {
      actorId: phien.person_id, method: "POST", attempt: newAttempt(),
    });
    setManaged(true);
  } : undefined;
  return managed === null ? <ActivityIndicator accessibilityLabel="Đang mở bài" /> : managed ? <PostDetail /> : <BaiChiTietScreen onShareCommunity={submitLegacy} />;
}
