import { useRouter } from "expo-router";
import { useEffect, useRef } from "react";
import { Platform, Text } from "react-native";

import { manDau } from "../src/rudi/duong-vao";
import { useRudiSession } from "../src/rudi/session";
import { luiVeVe } from "../src/rudi/lui-ve";
import { typography, useRudiTheme } from "../src/rudi/theme";
import { IconButton, Inline, RudiScreen } from "../src/rudi/ui";
import { Canh } from "../src/rudi/ui/art/Canh";
import { EmptyState } from "../src/rudi/ui/EmptyState";
import { OpeningApp } from "../src/rudi/ui/OpeningApp";

/**
 * Any path the router does not know. App B (the legacy shell behind `/legacy`
 * and the `?man=` / `#vao=` web doors) is gone. An unknown link must not erase
 * an existing session by sending its owner through the welcome/login door.
 */
export default function UnknownRoute() {
  const router = useRouter();
  const { colors } = useRudiTheme();
  const { phien, phienDaDoc } = useRudiSession();
  const title = useRef<Text>(null);
  useEffect(() => {
    if (Platform.OS !== "web" || !phienDaDoc) return;
    // Wait for the session cover to release inert before handing over focus.
    const frame = requestAnimationFrame(() => {
      const element = title.current as unknown as HTMLElement | null;
      if (element?.isConnected && !document.querySelector('[role="dialog"][aria-modal="true"]')) element.focus({ preventScroll: true });
    });
    return () => cancelAnimationFrame(frame);
  }, [phienDaDoc]);
  if (!phienDaDoc) return <OpeningApp />;
  return <RudiScreen testID="unknown-route-screen">
    <Inline>
      <IconButton accessibilityLabel="Quay lại" icon="chevron-back" onPress={() => luiVeVe(router, manDau(phien))} quiet />
      <Text ref={title} accessibilityRole="header" {...(Platform.OS === "web" ? { tabIndex: -1 } : {})} style={[typography.label, { color: colors.ink, flex: 1 }]}>Không tìm thấy trang</Text>
    </Inline>
    <EmptyState kind="no-results" title="Trang này chưa có trong Rủ Đi"
      body="Có thể đường dẫn đã đổi. Về trang đầu để tiếp tục nhé."
      illustration={<Canh id="tim-khong-ra" width={160} />}
      action={{ label: "Về Rủ Đi", onPress: () => router.replace(manDau(phien) as never) }} />
  </RudiScreen>;
}
