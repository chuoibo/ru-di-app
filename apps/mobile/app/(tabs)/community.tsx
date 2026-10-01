import { useFocusEffect, useRouter } from "expo-router";
import { useCallback } from "react";

import { CommunityScreen } from "../../src/rudi/community/CommunityScreen";
import { DauKhamPha } from "../../src/rudi/ui/DauKhamPha";
import { ghiMucKhamPha } from "../../src/rudi/ui/thanh-tab";

/**
 * Cộng đồng on the tab: Khám phá's second section (owner's mockup, 01/10), a
 * route with no column of its own. Opened on a topic it is a stack route
 * (`app/community/topic.tsx`) and keeps its own heading.
 */
export default function CommunityTab() {
  const router = useRouter();
  // Cộng đồng is the section in view: the Khám phá column reopens it.
  useFocusEffect(useCallback(() => ghiMucKhamPha("community"), []));
  // In the route file on purpose: the guide's extractor reads community -> explore here.
  return <CommunityScreen dau={(phai) => <DauKhamPha muc="community" onDoiMuc={() => router.navigate("/explore")} phai={phai} />} />;
}
