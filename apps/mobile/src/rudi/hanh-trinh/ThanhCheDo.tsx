import { Segmented } from "../ui";
import type { CheDoXem } from "./che-do";

const MUC = ["Lịch trình", "Hành trình"] as const;

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
