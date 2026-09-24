/**
 * `ui-lab`'s puppet board (ADR-0037 D5, plan S0.4): each of the nine
 * performances on demand, each of the eight moments through the real
 * `NepDien` (once per event, still frame after, nothing on an error), and the
 * nine still frames side by side as SVG -- the frames Reduce Motion shows.
 */
import { useEffect, useState } from "react";
import { Text, View } from "react-native";
import { Easing, useSharedValue, withTiming } from "react-native-reanimated";

import { KHUNG_NEP } from "../art/nep";
import { TIET_MUC, TIET_MUC_IDS, type TietMucId } from "../art/nep-dien";
import { tuTheRoi } from "../art/nep-roi";
import { KHOANH_KHAC, KHOANH_KHAC_IDS, type KhoanhKhacId } from "../khoanh-khac";
import { typography, useRudiTheme } from "../theme";
import { Chip, Inline } from "../ui";
import { NepDien } from "./NepDien";
import { NepRoi } from "./NepRoi";
import { VeLop } from "./art/VeLop";
import { useMotion } from "./useMotion";

const TEN: Record<TietMucId, string> = {
  "buoc-vao": "Bước vào",
  "keo-tab": "Kéo tab",
  "cam-may": "Chụp",
  "dong-dau": "Đóng dấu",
  "cui-cam-on": "Cúi cảm ơn",
  "buoc-di": "Lên đường",
  "gap-thu": "Gấp thư",
  nhay: "Nhảy",
  "nang-tem": "Nâng tem",
};

export function ThuNepDien() {
  const { colors } = useRudiTheme();
  const motion = useMotion();
  const [chon, setChon] = useState<TietMucId>("buoc-vao");
  const [lan, setLan] = useState(0);
  const [khoanhKhac, setKhoanhKhac] = useState<{ id: KhoanhKhacId; lan: number } | null>(null);
  const tm = TIET_MUC[chon];
  const t = useSharedValue(tm.khoa[tm.khungTinh].t);
  useEffect(() => {
    if (lan === 0) return;
    t.value = 0;
    t.value = withTiming(tm.ms, { duration: tm.ms, easing: Easing.linear, reduceMotion: motion.reanimated });
    // A run per press; the performance is read at that press.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [lan]);
  return (
    <View style={{ gap: 12 }}>
      <Text style={{ ...typography.note, color: colors.inkFaint }}>
        {"Con rối giấy: tờ thân và khuôn mặt của Nếp tĩnh, tay chân hai khúc ghim ở vai và hông. Chạm một tiết mục để diễn."}
      </Text>
      <View style={{ alignItems: "center" }}>
        <NepRoi t={t} testID="lab-nep-roi" tm={tm} width={144} />
      </View>
      <Inline gap={8} wrap>
        {TIET_MUC_IDS.map((id) => (
          <Chip
            key={id}
            label={TEN[id]}
            onPress={() => {
              setChon(id);
              setLan((n) => n + 1);
            }}
            selected={chon === id}
          />
        ))}
      </Inline>
      <Text style={{ ...typography.note, color: colors.inkFaint }}>
        {"Tám khoảnh khắc qua NepDien thật: mỗi lần chạm là một sự kiện mới. Khoảnh khắc tiền (M2, M3, M4) giữ mặt bình thản."}
      </Text>
      <Inline gap={8} wrap>
        {KHOANH_KHAC_IDS.map((id) => (
          <Chip key={id} label={`${id} · ${KHOANH_KHAC[id].ten}`} onPress={() => setKhoanhKhac((cu) => ({ id, lan: (cu?.lan ?? 0) + 1 }))} selected={khoanhKhac?.id === id} />
        ))}
      </Inline>
      {khoanhKhac ? (
        <View style={{ alignItems: "center" }}>
          <NepDien khoanhKhac={khoanhKhac.id} key={`${khoanhKhac.id}-${khoanhKhac.lan}`} suKien={`lab-${khoanhKhac.lan}`} testID="lab-nep-dien" />
        </View>
      ) : null}
      <Text style={{ ...typography.note, color: colors.inkFaint }}>{"Chín khung tĩnh (Giảm chuyển động, hoặc khoảnh khắc đã diễn):"}</Text>
      <Inline gap={4} wrap>
        {TIET_MUC_IDS.map((id) => (
          <VeLop
            accessibilityLabel={TIET_MUC[id].moTa}
            height={64}
            key={id}
            khungH={KHUNG_NEP}
            khungW={KHUNG_NEP}
            lop={tuTheRoi(TIET_MUC[id].khoa[TIET_MUC[id].khungTinh].tt, { chiTiet: false })}
            testID={`lab-nep-tinh-${id}`}
            width={64}
          />
        ))}
      </Inline>
    </View>
  );
}
