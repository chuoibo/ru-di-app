import { PlanLiveScreen } from "../../src/rudi/screens/keo/PlanLive";
import { useRudiSession } from "../../src/rudi/session";
import { CuaDangNhap } from "../../src/rudi/ui/CuaDangNhap";
import { useNepNguCanh } from "../../src/rudi/nep/NepProvider";

export default function PlanTab() {
  useNepNguCanh({
    man: "plan",
    tieuDe: "Lên plan",
    goiY: ["Sắp tới có kèo nào?", "Giúp mình phác lịch trình", "Nhắc mình trước một ngày"],
  });
  const { phien, phienDaDoc } = useRudiSession();
  // The group's outings from the server; no session, the sign-in door.
  if (!phienDaDoc) return null;
  if (phien === null) return <CuaDangNhap tiep="/plan" />;
  return <PlanLiveScreen phien={phien} />;
}
