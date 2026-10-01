/**
 * The one door a link goes through when the person opening it is not signed in.
 *
 * Before this, a cold link without a session went to four different places: a
 * chat to `/login` (and, after the code, to Khám phá rather than the chat), an
 * outing or a diary to the Welcome cover, a two-person notebook to an
 * unlabelled demo notebook, a community post to a spinner that never ended
 * (QA UI-121, UI-082, UI-137). Now every such route renders this, which sends
 * the person to the sign-in door with the route they were opening as `?tiep=`,
 * and the door sends them back to it once the session exists.
 *
 * `tiep` may be given when the route knows its own address better than the
 * router does (a query that matters, like `?ru=1&cho=` on the notebook). On the
 * web the browser's own location is exact; on native the router's pathname is
 * the best there is, so a native call site with a meaningful query passes it.
 */
import { Redirect, usePathname } from "expo-router";
import type { ReactNode } from "react";
import { Platform } from "react-native";

import { duongDangNhap } from "../duong-vao";
import { useRudiSession } from "../session";

function duongDangMo(pathname: string): string {
  if (Platform.OS === "web" && typeof window !== "undefined") {
    return `${window.location.pathname}${window.location.search}`;
  }
  return pathname;
}

export function CuaDangNhap({ tiep }: { tiep?: string }) {
  const pathname = usePathname();
  return <Redirect href={duongDangNhap(tiep ?? duongDangMo(pathname)) as never} />;
}

/**
 * A route that means nothing without a session: nothing until the session has
 * been read, the sign-in door when there is none, the screen when there is.
 */
export function CanPhien({ children, tiep }: { children: ReactNode; tiep?: string }) {
  const { phien, phienDaDoc } = useRudiSession();
  if (!phienDaDoc) return null;
  if (phien === null) return <CuaDangNhap tiep={tiep} />;
  return <>{children}</>;
}
