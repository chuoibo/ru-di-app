import { Redirect, useLocalSearchParams } from "expo-router";

import { KhongGianGiayScreen } from "../../../src/rudi/screens/hai-nguoi/KhongGianGiay";
import { useRudiSession } from "../../../src/rudi/session";
import { tenCuocTroChuyen } from "../../../src/rudi/nhan-rieng/nhan-rieng";
import { SoDoiSongProvider } from "../../../src/rudi/to-giay/SoDoiSong";

/**
 * The paper surface of a two-person notebook.
 *
 * One screen, two sources. A real session wraps it in the live provider, which
 * answers the same `useSoDoi()` the fixture store answers; the fixture build
 * falls through to the store mounted in `_layout`. The screen itself is the
 * same file in both cases, which is the whole point of Phase 4 -- the blind
 * reads that shaped it in Phase 2 still describe what ships.
 */
export default function ToGiayRoute() {
  const params = useLocalSearchParams<{ id: string; ru?: string; cho?: string }>();
  // `?cho=` is the catalogue place the invitation starts from («Rủ … tới đây»).
  const cho = typeof params.cho === "string" && params.cho !== "" ? params.cho : undefined;
  const { phien, phienDaDoc } = useRudiSession();
  if (!phienDaDoc) return null;
  const ruNgay = params.ru === "1";
  if (phien !== null) {
    if (typeof params.id !== "string") return <Redirect href="/messages" />;
    return (
      // Keyed by conversation for the same reason the chat route is: opening
      // another pair's link while this one is in front changes the param in
      // place, and a screen that survived that would carry one notebook's
      // sheet into the other's.
      <SoDoiSongProvider
        contextId={params.id}
        key={params.id}
        tenNguoiKia={tenCuocTroChuyen(phien.contexts?.find((n) => n.id === params.id))}
        toiId={phien.person_id}
      >
        <KhongGianGiayScreen choGoiY={cho} contextId={params.id} ruNgay={ruNgay} />
      </SoDoiSongProvider>
    );
  }
  return <KhongGianGiayScreen contextId={typeof params.id === "string" ? params.id : "cap-demo"} ruNgay={ruNgay} />;
}
