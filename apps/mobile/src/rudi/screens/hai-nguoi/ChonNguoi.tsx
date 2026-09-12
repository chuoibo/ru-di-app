import { useRouter } from "expo-router";
import { View } from "react-native";

import { CAP_DEMO, NGUOI_KIA_DEMO } from "../../to-giay/fixtures-doi";
import { Heading, ListRow, NhomHang, RudiScreen, TopBar } from "../../ui";

/**
 * «Rủ một người đi chơi» (the one new entry in «Tạo mới», spec §20.1): pick
 * the person, land in the pair's paper surface with a sheet already drafted.
 * The experience build has one person to pick, the fixture pair; the live
 * build (Phase 4) lists friends and opens the direct conversation first.
 */
export function ChonNguoiScreen() {
  const router = useRouter();
  return (
    <RudiScreen header={<TopBar back title="Rủ một người đi chơi" subtitle="Nếp phác sẵn, bạn gửi" />} testID="chon-nguoi">
      <View style={{ gap: 10, paddingTop: 8 }}>
        <Heading size="h2" subtitle="Tờ giấy đi vào sổ hai người của hai bạn, không vào hội." title="Rủ ai?" />
        <NhomHang>
          <ListRow icon="person-outline" onPress={() => router.replace(`/groups/${CAP_DEMO.id}/to-giay?ru=1` as never)} subtitle="Sổ hai người đang mở" title={NGUOI_KIA_DEMO.ten} />
        </NhomHang>
      </View>
    </RudiScreen>
  );
}
