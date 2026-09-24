import { Redirect, useLocalSearchParams } from "expo-router";
import { Text, View } from "react-native";

import { sanKhauKyHoa, sanKhauTuCanh } from "../../src/rudi/art/san-khau";
import { CUA_FIXTURE_DEV } from "../../src/rudi/cua-fixture";
import { typography, useRudiTheme } from "../../src/rudi/theme";
import { RudiScreen, SectionHeader } from "../../src/rudi/ui";
import { CanhGap, ThanhCanh } from "../../src/rudi/ui/CanhGap";

/**
 * Synthetic probe of a staged screen head (ADR-0037, plan S0.3): the stage
 * stands up on open, folds back into the page as the list scrolls, and the
 * compact bar takes the title once the big one has gone under it. `?canh=1`
 * swaps the place sketch for an empty-state scene. Never in a production build.
 */
const HANG = Array.from({ length: 18 }, (_, i) => ({ id: `h${i}`, ten: `Dòng thử ${i + 1}`, phu: i % 3 === 0 ? "Một dòng phụ dài hơn để thấy chữ xuống dòng khi cỡ chữ hệ thống lớn." : "Dòng phụ ngắn." }));

export default function ThuManSanKhau() {
  const { colors } = useRudiTheme();
  const { canh } = useLocalSearchParams<{ canh?: string }>();
  if (!CUA_FIXTURE_DEV) return <Redirect href="/welcome" />;
  const laCanh = canh === "1";
  const tieuDe = laCanh ? "Chưa có hội" : "Phố đêm";
  return (
    <RudiScreen
      canh={
        <CanhGap
          coMoTa
          keo
          nhanTren="Màn thử · dữ liệu tổng hợp"
          phuDe="Cuộn xuống: sân khấu gập vào trang, tầng gần nằm xuống trước. Kéo ngang trên tranh để nghiêng."
          san={laCanh ? () => sanKhauTuCanh("chua-co-hoi") : (gon) => sanKhauKyHoa("di-choi-dem", ["Món local", "Đi đêm", "Nhộn nhịp"], { gon })}
          testID="thu-canh-gap"
          tieuDe={tieuDe}
        />
      }
      header={<ThanhCanh tieuDe={tieuDe} />}
      testID="thu-man-san-khau"
    >
      <SectionHeader title="Danh sách thử" />
      {HANG.map((h) => (
        <View key={h.id} style={{ borderBottomColor: colors.line, borderBottomWidth: 1, gap: 2, paddingVertical: 10 }}>
          <Text style={{ ...typography.label, color: colors.ink }}>{h.ten}</Text>
          <Text style={{ ...typography.note, color: colors.inkSoft }}>{h.phu}</Text>
        </View>
      ))}
    </RudiScreen>
  );
}
