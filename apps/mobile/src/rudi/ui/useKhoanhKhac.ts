/**
 * Plays one of Nếp's eight moments (ADR-0037 D4, D5) on a clock shared by
 * everything that moves with it -- the puppet, a stage, a big stamp -- so a
 * seal lands on the page on the same frame Nếp's hand does.
 *
 * What to do is decided once per event, in an effect (never during render:
 * the registry remembers what played): play it, show the still frame
 * (already played this session, or Reduce Motion), or show nothing (an
 * error, a state the server has not confirmed). The haptic beats fire as the
 * clock crosses them while it plays -- never when a skip jumps it -- and a
 * tap skips to the still frame (ADR-0037 D3: a performance never holds the
 * person up).
 */
import { useEffect, useMemo, useState } from "react";
import { Easing, cancelAnimation, runOnJS, useAnimatedReaction, useSharedValue, withTiming, type SharedValue } from "react-native-reanimated";

import { TIET_MUC, noiTietMuc, type NhipRung, type TietMuc } from "../art/nep-dien";
import { KHOANH_KHAC, khoaKhoanhKhac, nenDien, type CachDien, type KhoanhKhacId } from "../khoanh-khac";
import { useMotion } from "./useMotion";

export interface KhoanhKhacDangDien {
  /** The joined performance, or null when there is nothing to show. */
  tm: TietMuc | null;
  /** The clock, ms into `tm`. */
  t: SharedValue<number>;
  cach: CachDien | null;
  dangChay: boolean;
  /** Jump to the still frame. */
  boQua: () => void;
}

export function useKhoanhKhac(id: KhoanhKhacId, suKien: string | null, tuyChon: { hopLe?: boolean; coLoi?: boolean } = {}): KhoanhKhacDangDien {
  const motion = useMotion();
  const { hopLe = true, coLoi = false } = tuyChon;
  const tm = useMemo(() => noiTietMuc(KHOANH_KHAC[id].tietMuc.map((x) => TIET_MUC[x])), [id]);
  const tTinh = tm.khoa[tm.khungTinh].t;
  const t = useSharedValue(tTinh);
  const dangChaySV = useSharedValue(false);
  const [cach, setCach] = useState<CachDien | null>(null);
  const [dangChay, setDangChay] = useState(false);
  const khoa = suKien ? khoaKhoanhKhac(id, suKien) : "";

  useEffect(() => {
    if (!khoa) {
      setCach(null);
      return;
    }
    const c = nenDien({ khoa, reduced: motion.reduced, hopLe, coLoi });
    setCach(c);
    if (c !== "dien") {
      t.value = tTinh;
      return;
    }
    t.value = 0;
    dangChaySV.value = true;
    setDangChay(true);
    t.value = withTiming(tm.ms, { duration: tm.ms, easing: Easing.linear, reduceMotion: motion.reanimated }, () => {
      dangChaySV.value = false;
      runOnJS(setDangChay)(false);
    });
    return () => {
      cancelAnimation(t);
      dangChaySV.value = false;
    };
    // The decision is per event key; the motion kit and the state flags are read at that moment.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [khoa, hopLe, coLoi]);

  const rung = (kieu: NhipRung) => {
    if (kieu === "cham") motion.haptic.impact();
    else if (kieu === "xong") motion.haptic.success();
    else motion.haptic.select();
  };
  useAnimatedReaction(
    () => t.value,
    (moi, cu) => {
      if (cu === null || !dangChaySV.value || moi < cu) return;
      for (const n of tm.nhip) if (cu < n.t && moi >= n.t) runOnJS(rung)(n.kieu);
    },
    [tm],
  );

  const boQua = () => {
    cancelAnimation(t);
    dangChaySV.value = false;
    t.value = tTinh;
    setDangChay(false);
  };
  return { tm: cach === "dien" || cach === "tinh" ? tm : null, t, cach, dangChay, boQua };
}
