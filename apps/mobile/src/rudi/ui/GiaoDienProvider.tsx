/**
 * Đọc lựa chọn sáng/tối trên máy và cấp cho cả cây (L5, ADR-0023 §2.5).
 *
 * Giá trị nằm trong AsyncStorage, không lên máy chủ. Trong lúc đĩa còn đang
 * trả lời, cả cây chạy ở «theo hệ thống» — cùng đúng cái app làm trước lát
 * này, nên không có khung hình nào nhấp nháy sai màu rồi mới đúng.
 */
import { createContext, useCallback, useContext, useEffect, useState, useSyncExternalStore } from "react";

import { Appearance, Platform } from "react-native";

import { toiHay, docCheDoGiaoDien, KHOA_GIAO_DIEN, type CheDoGiaoDien } from "../giao-dien";
import { docGiaoDienAsync, ghiGiaoDienAsync } from "../kho";
import { GiaoDienContext } from "../theme";

/** One external-store snapshot prevents mounted and newly opened panels diverging. */
const readSystemDark = () => Platform.OS === "web" && typeof window !== "undefined"
  ? window.matchMedia("(prefers-color-scheme: dark)").matches
  : Appearance.getColorScheme() === "dark";
const subscribeSystemScheme = (notify: () => void) => {
  if (Platform.OS === "web" && typeof window !== "undefined") {
    const query = window.matchMedia("(prefers-color-scheme: dark)");
    query.addEventListener("change", notify);
    return () => query.removeEventListener("change", notify);
  }
  const subscription = Appearance.addChangeListener(notify);
  return () => subscription.remove();
};
const serverDark = () => false;

type DoiGiaoDien = { cheDo: CheDoGiaoDien; datCheDo: (che: CheDoGiaoDien) => void; daDoc: boolean };

const DoiContext = createContext<DoiGiaoDien>({
  cheDo: "he-thong",
  datCheDo: () => undefined,
  daDoc: false,
});

export function useGiaoDien(): DoiGiaoDien {
  return useContext(DoiContext);
}

export function GiaoDienProvider({ children }: { children: React.ReactNode }) {
  const systemDark = useSyncExternalStore(subscribeSystemScheme, readSystemDark, serverDark);
  const resolved = (choice: CheDoGiaoDien) => toiHay(choice, systemDark) ? "toi" : "sang";
  const [cheDo, setCheDo] = useState<CheDoGiaoDien>("he-thong");
  const [daDoc, setDaDoc] = useState(false);

  useEffect(() => {
    let con = true;
    void (async () => {
      const luu = await docGiaoDienAsync(KHOA_GIAO_DIEN);
      if (!con) return;
      setCheDo(docCheDoGiaoDien(luu));
      setDaDoc(true);
    })();
    return () => {
      con = false;
    };
  }, []);

  const datCheDo = useCallback((che: CheDoGiaoDien) => {
    setCheDo(che);
    void ghiGiaoDienAsync(KHOA_GIAO_DIEN, che);
  }, []);

  return (
    <DoiContext.Provider value={{ cheDo, datCheDo, daDoc }}>
      <GiaoDienContext.Provider value={resolved(cheDo)}>{children}</GiaoDienContext.Provider>
    </DoiContext.Provider>
  );
}
