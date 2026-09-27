import { Segmented } from "../ui";
import type { CheDoXem } from "./che-do";

// ADR-0038 §2.4: «Lịch trình» and «Hành trình» read as one word; the second
// view is the route on a map, and says so. testIDs keep the old names.
const MUC = ["Lịch trình", "Bản đồ"] as const;

export function ThanhCheDo({ cheDo, onDoi }: { cheDo: CheDoXem; onDoi: (c: CheDoXem) => void }) {
  return (
    <Segmented
      items={[...MUC]}
      onSelect={(i) => onDoi(i === 0 ? "lich-trinh" : "hanh-trinh")}
      selected={cheDo === "lich-trinh" ? 0 : 1}
      testIDs={["che-do-lich-trinh", "che-do-hanh-trinh"]}
    />
  );
}
