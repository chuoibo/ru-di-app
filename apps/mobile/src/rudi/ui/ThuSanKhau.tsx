/**
 * `ui-lab`'s paper-stage board (ADR-0037, plan S0.3): the machine on invented
 * scenes, so its three motions can be seen and filmed on one screen --
 *   1. a scene standing up once on mount (far layers first);
 *   2. a place sketch lifted into three depths, leaned by a sideways drag;
 *   3. a stage raised by the hand through a pull tab, then by a tap.
 * Every stage is built from the art the app already ships, never redrawn.
 */
import { useState } from "react";
import { Text, View, useWindowDimensions } from "react-native";
import { GestureDetector } from "react-native-gesture-handler";
import { useSharedValue, withTiming } from "react-native-reanimated";

import { sanKhauKyHoa, sanKhauTuCanh } from "../art/san-khau";
import { typography, useRudiTheme } from "../theme";
import { KeoTab } from "./KeoTab";
import { SanKhau } from "./SanKhau";
import { useMotion } from "./useMotion";
import { useThiSaiKeo } from "./useThiSai";

const SAN_CANH = sanKhauTuCanh("chua-co-hoi");
const SAN_KY_HOA = sanKhauKyHoa("quan-an-local", ["View đẹp", "Chill", "Nhóm đông"]);
const SAN_TAB = sanKhauTuCanh("chua-co-loi-moi");

function NhanNho({ children }: { children: string }) {
  const { colors } = useRudiTheme();
  return <Text style={{ ...typography.note, color: colors.inkFaint }}>{children}</Text>;
}

export function ThuSanKhau({ lan }: { lan: number }) {
  const { width } = useWindowDimensions();
  const motion = useMotion();
  const cot = Math.min(width - 32, 560);
  const { thiSai, cuChi } = useThiSaiKeo(true);
  const moTab = useSharedValue(0);
  const [daMo, setDaMo] = useState(false);
  const moHet = () => {
    setDaMo(true);
    moTab.value = withTiming(1, { duration: motion.sanKhau.batToiDa(SAN_TAB.tang.length), reduceMotion: motion.reanimated });
    motion.haptic.success();
  };
  const gapLai = () => {
    setDaMo(false);
    moTab.value = withTiming(0, { duration: motion.ms("shared"), reduceMotion: motion.reanimated });
  };
  return (
    <View style={{ gap: 14 }}>
      <NhanNho>{"1 · Cảnh rỗng thành sân khấu hai tầng: đạo cụ rồi Nếp, dựng từ vạch sàn."}</NhanNho>
      <View style={{ alignItems: "center" }}>
        <SanKhau key={`canh-${lan}`} san={SAN_CANH} testID="lab-san-khau-canh" width={Math.min(cot, 240)} />
      </View>
      <NhanNho>{"2 · Ký hoạ tách ba độ sâu theo độ dày nét. Kéo ngang để nghiêng, thả ra là về chỗ."}</NhanNho>
      <GestureDetector gesture={cuChi}>
        <View style={{ alignItems: "center" }}>
          <SanKhau key={`ky-hoa-${lan}`} san={SAN_KY_HOA} testID="lab-san-khau-ky-hoa" thiSai={thiSai} width={cot} />
        </View>
      </GestureDetector>
      <NhanNho>{"3 · Tab kéo: kéo sang phải để dựng phong thư bằng tay, hoặc chạm."}</NhanNho>
      <View style={{ alignItems: "center" }}>
        <SanKhau mo={moTab} san={SAN_TAB} testID="lab-san-khau-tab" width={Math.min(cot, 240)} />
      </View>
      {daMo ? (
        <KeoTab goiY="Gập phong thư vào trang" nhan="Gập lại" onKeo={gapLai} testID="lab-keo-tab-gap" />
      ) : (
        <KeoTab goiY="Dựng phong thư lên khỏi trang" nhan="Kéo để mở thư" onKeo={moHet} testID="lab-keo-tab" tien={moTab} />
      )}
    </View>
  );
}
