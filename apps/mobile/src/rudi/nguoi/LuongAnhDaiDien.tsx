import { useEffect } from "react";
import { AppState } from "react-native";

import { BASE_URL } from "../../api";
import { tokenPhienHienTai } from "../../danh-tinh";
import { useRudiSession } from "../session";
import { moLuongAnhDaiDien, type SocketToiThieu } from "./luong-anh-dai-dien";

/**
 * Keeps the avatar stream open for the signed-in account while the app is in
 * the foreground. Renders nothing; mounted once, inside the session provider.
 */
export function LuongAnhDaiDien() {
  const { phien } = useRudiSession();
  const personId = phien?.person_id ?? null;
  const token = phien?.token ?? null;
  useEffect(() => {
    if (personId === null || token === null || typeof WebSocket === "undefined") return;
    const luong = moLuongAnhDaiDien({
      url: `${BASE_URL.replace(/^http/, "ws")}/people/avatars/stream`,
      actorId: personId,
      token: tokenPhienHienTai,
      // Only the four handlers, send and close are used; their event arguments are ignored.
      taoSocket: (url) => new WebSocket(url) as unknown as SocketToiThieu,
    });
    if (AppState.currentState !== "active") luong.tamDung();
    const theoDoi = AppState.addEventListener("change", (state) => (state === "active" ? luong.tiepTuc() : luong.tamDung()));
    return () => {
      theoDoi.remove();
      luong.dung();
    };
  }, [personId, token]);
  return null;
}
