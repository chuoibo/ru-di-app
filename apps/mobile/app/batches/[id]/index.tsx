import { useLocalSearchParams } from "expo-router";

import { nguCanhMo } from "../../../src/rudi/ngu-canh-mo";
import { DotThuLiveScreen } from "../../../src/rudi/screens/dot-thu/DotThuLive";
import { useRudiSession } from "../../../src/rudi/session";
import { RudiButton, RudiScreen, Heading, TopBar } from "../../../src/rudi/ui";
import { useRouter } from "expo-router";

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
  const router = useRouter();
  if (!phienDaDoc) return null;
  const id = maDot(params.id);
  if (phien !== null && id !== "") {
    const mo = nguCanhMo(phien, params.ctx);
    if (mo.kieu === "khac") return <DotThuLiveScreen batchId={id} key={mo.contextId} phien={{ ...phien, context_id: mo.contextId }} />;
    return <DotThuLiveScreen batchId={id} phien={phien} />;
  }
  // No fixture round exists: the settlement fixture never opens one. Say so.
  return (
    <RudiScreen tone="split" testID="collection-batch-screen">
      <TopBar title="Đợt thu" />
      <Heading title="Cần đăng nhập" subtitle="Đợt thu là của một nhóm thật; bản trải nghiệm không có đợt thu nào." />
      <RudiButton label="Quay lại" onPress={() => router.back()} tone="split" variant="outline" />
    </RudiScreen>
  );
}
