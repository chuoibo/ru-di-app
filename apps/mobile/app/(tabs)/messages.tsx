import { ConversationsScreen } from "../../src/rudi/screens/groups/Conversations";
import { useRudiSession } from "../../src/rudi/session";
import { CuaDangNhap } from "../../src/rudi/ui/CuaDangNhap";

export default function MessagesTab() {
  const { phien, phienDaDoc } = useRudiSession();
  if (!phienDaDoc) return null;
  if (phien === null) return <CuaDangNhap tiep="/messages" />;
  return <ConversationsScreen phien={phien} />;
}
