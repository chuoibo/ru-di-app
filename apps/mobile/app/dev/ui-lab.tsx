import { Redirect } from "expo-router";
import { useState } from "react";
import { Text } from "react-native";
import { CUA_FIXTURE_DEV } from "../../src/rudi/cua-fixture";
import { demoAssets } from "../../src/rudi/fixtures";
import { typography, useRudiTheme } from "../../src/rudi/theme";
import { Heading, RudiButton, RudiScreen, TopBar } from "../../src/rudi/ui";
import { ReorderList } from "../../src/rudi/ui/ReorderList";
import { PhotoViewer } from "../../src/rudi/ui/PhotoViewer";

/** Synthetic native gesture probe; never available in a production build. */
export default function UiLab() {
  const { colors } = useRudiTheme();
  const [items, setItems] = useState([
    { id: "a", label: "Chặng A · 18:00" },
    { id: "b", label: "Chặng B · 08:00" },
    { id: "c", label: "Chặng C · tên dài để kiểm tra dòng chữ khi tăng kích cỡ hệ thống" },
  ]);
  const [dragging, setDragging] = useState(false);
  const [viewer, setViewer] = useState(false);
  if (!CUA_FIXTURE_DEV) return <Redirect href="/welcome" />;
  return <RudiScreen scrollEnabled={!dragging}>
    <TopBar title="Thử tương tác native" />
    <Heading title="Dữ liệu tổng hợp" subtitle="Chỉ đo gesture và hiển thị. Không phải dữ liệu nhóm hay bằng chứng API live." />
    <Text style={[typography.body, { color: colors.ink }]}>Thứ tự: {items.map((item) => item.id).join(" → ")}</Text>
    <ReorderList items={items} itemKey={(item) => item.id} label={(item) => item.label}
      onChange={setItems} onDragging={setDragging}
      renderItem={(item) => <Text style={[typography.h2, { color: colors.ink, paddingVertical: 24 }]}>{item.label}</Text>} />
    <RudiButton label="Mở bộ ảnh tổng hợp" onPress={() => setViewer(true)} />
    {viewer ? <PhotoViewer initialIndex={0} title="Ảnh minh họa tổng hợp" onClose={() => setViewer(false)} photos={[
      { id: "a", source: demoAssets.cafe, caption: "Ảnh minh họa cafe · không gán cho địa điểm thật" },
      { id: "b", source: demoAssets.friends, caption: "Ảnh minh họa bạn bè · không phải người dùng thật" },
    ]} /> : null}
  </RudiScreen>;
}
