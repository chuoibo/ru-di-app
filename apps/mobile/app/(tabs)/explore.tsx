import { useFocusEffect, useRouter } from "expo-router";
import type { ReactNode } from "react";
import { useCallback } from "react";

import { ExploreLiveScreen } from "../../src/rudi/screens/explore/ExploreLive";
import { useRudiSession } from "../../src/rudi/session";
import { CuaDangNhap } from "../../src/rudi/ui/CuaDangNhap";
import { useNepNguCanh } from "../../src/rudi/nep/NepProvider";
import { DauKhamPha } from "../../src/rudi/ui/DauKhamPha";
import { ghiMucKhamPha } from "../../src/rudi/ui/thanh-tab";

export default function ExploreTab() {
  // Declared, never inferred: Nếp learns the route and the questions that make
  // sense here, and nothing about what the catalogue is showing.
  useNepNguCanh({
    man: "explore",
    tieuDe: "Khám phá",
    goiY: ["Quanh đây có gì hay?", "Chỗ này hợp đi mấy người?", "Gợi ý quán cho tối nay"],
  });
  const router = useRouter();
  const { phien, phienDaDoc } = useRudiSession();
  // Địa điểm is the section in view: the Khám phá column reopens it.
  useFocusEffect(useCallback(() => ghiMucKhamPha("explore"), []));
  // The section switch is a navigation written here, in the route file, so the
  // guide's extractor (tools/rut-huong-dan.mjs) sees explore -> community.
  const dau = (phai?: ReactNode) => <DauKhamPha muc="explore" onDoiMuc={() => router.navigate("/community")} phai={phai} />;
  // The server's catalogue; no session, the sign-in door.
  if (!phienDaDoc) return null;
  if (phien === null) return <CuaDangNhap tiep="/explore" />;
  return <ExploreLiveScreen dau={dau} phien={phien} />;
}
