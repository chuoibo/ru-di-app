import { ExploreScreen } from "../../src/rudi/screens/Discovery";
import { ExploreLiveScreen } from "../../src/rudi/screens/explore/ExploreLive";
import { useRudiSession } from "../../src/rudi/session";
import { useNepNguCanh } from "../../src/rudi/nep/NepProvider";

export default function ExploreTab() {
  // Declared, never inferred: Nếp learns the route and the questions that make
  // sense here, and nothing about what the catalogue is showing.
  useNepNguCanh({
    man: "explore",
    tieuDe: "Khám phá",
    goiY: ["Quanh đây có gì hay?", "Chỗ này hợp đi mấy người?", "Gợi ý quán cho tối nay"],
  });
  const { phien, phienDaDoc } = useRudiSession();
  // A real session reads the server's catalogue; the fixture build keeps the
  // fixture places, which is what the default Maestro table drives.
  if (!phienDaDoc) return null;
  if (phien !== null) return <ExploreLiveScreen phien={phien} />;
  return <ExploreScreen />;
}
