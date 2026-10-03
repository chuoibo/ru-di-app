import { Redirect, useLocalSearchParams } from "expo-router";

import { KhongGianGiayScreen } from "../../../src/rudi/screens/hai-nguoi/KhongGianGiay";
import { useRudiSession } from "../../../src/rudi/session";
import { tenCuocTroChuyen } from "../../../src/rudi/nhan-rieng/nhan-rieng";
import { SoDoiSongProvider } from "../../../src/rudi/to-giay/SoDoiSong";
import { CuaDangNhap } from "../../../src/rudi/ui/CuaDangNhap";

/**
 * The paper surface of a two-person notebook.
 *
 * A real session wraps it in the live provider, which answers `useSoDoi()`
 * from the server. Without a session there is no notebook to show.
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
  // Signed out: the sign-in door, then back to this notebook.
  const query = [ruNgay ? "ru=1" : null, cho ? `cho=${encodeURIComponent(cho)}` : null].filter(Boolean).join("&");
  return <CuaDangNhap tiep={`/groups/${String(params.id)}/to-giay${query ? `?${query}` : ""}`} />;
}
