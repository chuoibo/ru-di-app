import { Stack, useRouter } from "expo-router";
import { StatusBar } from "expo-status-bar";
import { useEffect, useRef } from "react";
import { Linking, LogBox, Platform } from "react-native";
import { SafeAreaProvider } from "react-native-safe-area-context";
import { GestureHandlerRootView } from "react-native-gesture-handler";

import { diemVaoTuUrl, manDau } from "../src/rudi/duong-vao";
import { datLoiMoiDen } from "../src/rudi/loi-moi-den";
import { useRudiFonts } from "../src/rudi/fonts";
import { stackAnimation } from "../src/rudi/motion";
import { RudiSessionProvider, useRudiSession } from "../src/rudi/session";
import { LuongAnhDaiDien } from "../src/rudi/nguoi/LuongAnhDaiDien";
import { NepNoi } from "../src/rudi/nep/NepNoi";
import { NepProvider } from "../src/rudi/nep/NepProvider";
import { SoDoiProvider } from "../src/rudi/to-giay/SoDoi";
import { useRudiTheme } from "../src/rudi/theme";
import { useMotion } from "../src/rudi/ui/useMotion";
import { GiaoDienProvider } from "../src/rudi/ui/GiaoDienProvider";
import "../src/rudi/tep-anh-native";

// Module level, before the first frame: `index.ts` never runs under
// `expo-router/entry`, so the call that used to live in the legacy App.tsx never
// ran on this shell. Measured on Expo Go 57 / Android 15: the LogBox strip sat
// over the tab bar and swallowed the "Tạo mới" button -- the door to the hero
// flow -- with no error and no navigation. Only the *notification* is silenced:
// uncaught errors still open LogBox full-screen, and warnings still reach the
// console and logcat. No-op in release builds and on web.
LogBox.ignoreAllLogs();

/*
 * Direction contract v3 «Sân khấu giấy» (ADR-0037, Lead 2026-09-24; plan copy in
 * docs/architecture/04-ui-v3-san-khau-giay.md). Builds on v2 (seed c8e88116,
 * «the travel journal»); what v2 promised still holds unless named here.
 * THESIS  -- The group's travel journal is a pop-up book. Every screen is a
 *            stage of cut paper that stands up out of the page's fold; every
 *            job is a paper object you handle (receipt, invitation, folded
 *            letter, ticket, stamp, passport), not a form you fill in. It still
 *            refuses sunset photo + white cards + coral pill, and it refuses the
 *            stacked-input screen.
 * OWN-WORLD -- Indigo cloth cover, bright paper pages; the three meaning tones
 *            are three paper stocks (coral = the ask, teal = money, violet = AI
 *            tracing paper). Paper height 0-3 is the only depth: printed,
 *            pasted, standing, lifted; one light from the top left (a desk lamp
 *            at night). People are paper standees in their own ink. Nếp is a
 *            paper puppet with brads at its joints.
 * STORY   -- Open the cover -> Rủ Đi thôi! -> a city stage pops up -> send an
 *            invitation card -> the ticket joins the plan -> the table pops up,
 *            dishes go to seats -> the receipt tears into everyone's stub, Nếp
 *            stamps «ĐÃ GHI SỔ» -> the night becomes a print on the wall.
 * FIRST VIEWPORT -- Welcome is the closed cover: indigo full-bleed, wordmark
 *            very large in the upper third, one diagonal orange washi strip
 *            carrying "AI đi chơi, chia bill thông minh", the CTA
 *            "Rủ Đi thôi!" as a large stamp; pressing it turns the cover on
 *            its spine onto the bright Login page.
 * FORM    -- expo-router stack + 4 tabs + create sheet; 48dp targets, 13sp
 *            floor, tabular money; four motion steps plus composite stage
 *            budgets (pop-up <= 420 ms, a Nếp performance <= 1400 ms, never
 *            holding input), input-linked motion only while the finger moves,
 *            Reduce Motion to the final frame. Skia draws stages, the puppet
 *            and materials; SVG draws rows and is the fallback. Every word is
 *            React Native text. Signatures: the pop-up, the page turn, the
 *            stamp, the folding letter, the tearing receipt, Nếp performing
 *            eight moments and never touching a number.
 * FINISH: unreviewed and undocumented is unfinished; this campaign ends with
 *         captures on the LIVE seeded world, a blind read, the finish review,
 *         DESIGN.md v3, and every shipping raster carrying its provenance
 */
/** Decides the first screen of a cold start, and routes warm links.
 *
 * The URL decision lives in `src/rudi/duong-vao.ts` so it can be tested
 * without a device; see the docstring there for the deep link this used to
 * swallow. The session decision (`manDau`) lives beside it for the same
 * reason: a pathless launch means «welcome» only for somebody signed out. */
function LegacyFragmentAdapter() {
  const router = useRouter();
  const { phien, phienDaDoc } = useRudiSession();
  // Once. The effect below re-runs when the session changes (sign-in, sign-out)
  // and must not replay the cold-start redirect on top of wherever the person
  // navigated to since.
  const daQuyetDinh = useRef(false);

  useEffect(() => {
    if (Platform.OS === "web") return;
    // Not before the disk has answered: deciding on `phien === null` while
    // SecureStore is still reading would send every signed-in person to the
    // welcome screen on every launch.
    if (!phienDaDoc || daQuyetDinh.current) return;
    daQuyetDinh.current = true;
    let live = true;
    void Linking.getInitialURL().then((url) => {
      // Re-checked after the await, not before it. The guard that was only
      // checked before is the whole of the defect.
      if (!live) return;
      const diem = diemVaoTuUrl(url);
      if (diem.kieu === "loi-moi") {
        // The code goes through a module, never through a route param: a
        // single-use bearer secret should not land in navigation state. See
        // `src/rudi/loi-moi-den.ts`.
        datLoiMoiDen(diem.ma);
        router.replace("/moi" as never);
        return;
      }
      if (diem.kieu !== "doi-huong") return;
      // «welcome» from the URL alone becomes «back where you were» when a
      // session survived the restart. Same function `app/index.tsx` uses.
      const toi = diem.toi === "/welcome" ? manDau(phien) : diem.toi;
      router.replace(toi as never);
    });
    return () => {
      live = false;
    };
  }, [router, phien, phienDaDoc]);

  useEffect(() => {
    if (Platform.OS === "web") return;
    // Warm links: the app is already open when a friend's invite arrives, or
    // when the dev client (whose launcher swallows cold `rudi://` links) hands
    // one over after the bundle is up. Same decision function, same routes.
    // A separate effect with its own lifetime: the cold-start one above ends
    // after a single decision, and this listener must outlive it.
    const sub = Linking.addEventListener("url", ({ url }) => {
      const diem = diemVaoTuUrl(url);
      if (diem.kieu === "loi-moi") {
        datLoiMoiDen(diem.ma);
        router.replace("/moi" as never);
        return;
      }
      if (diem.kieu === "doi-huong") router.replace(diem.toi as never);
    });
    return () => sub.remove();
  }, [router]);

  return null;
}

export default function RootLayout() {
  // The light/dark choice wraps everything, including the part of this file
  // that reads the theme: `RootInner` is a separate component so its
  // `useRudiTheme()` runs INSIDE the provider. Read it in `RootLayout` and
  // the status bar would keep the system's answer forever.
  return (
    <GiaoDienProvider>
      <RootInner />
    </GiaoDienProvider>
  );
}

function RootInner() {
  const { dark, colors } = useRudiTheme();
  // Hold the first frame until the display face is in: a heading that flips
  // from Roboto to Bricolage a beat after launch is the cheapest tell that a
  // page was assembled rather than built.
  const [fontsLoaded, fontsError] = useRudiFonts();
  // Reduce Motion reaches the navigator from here and nowhere else. Android's
  // animation scales at 0 leave a react-native-screens push sliding (Codex
  // re-audit 10/09, R1), so the stack is told to cut, and `useMotion` re-renders
  // this component when the setting changes mid-session.
  const motion = useMotion();
  if (!fontsLoaded && !fontsError) return null;
  const chuyen = (wanted: "slide_from_right" | "slide_from_bottom" | "fade") => stackAnimation(wanted, motion.reduced);

  // Design contract: warm editorial surfaces, one semantic leading tone per
  // screen, native 44pt targets, real text, restrained motion, and no visual
  // treatment that could blur the boundary between demo and live money data.
  return (
    <GestureHandlerRootView style={{ flex: 1 }}>
    <SafeAreaProvider>
      <RudiSessionProvider>
      {/* Friends' new avatars reach every screen while the app is open. */}
      <LuongAnhDaiDien />
      {/* The two-person notebook of the experience build: in memory, wire-shaped,
          swapped for the ADR-0027 routes in Phase 4. Inside the session so it can
          later read the bearer; outside the Stack so every route sees one notebook. */}
      <SoDoiProvider>
      {/* Nếp (ADR-0032): one assistant for every route. Inside the session so it
          can read `phien`/`cheDo`, outside the Stack so a push does not remount it
          and lose where the person parked it. */}
      <NepProvider>
        <StatusBar style={dark ? "light" : "dark"} />
        <LegacyFragmentAdapter />
        <Stack
          screenOptions={{
            animation: chuyen("slide_from_right"),
            contentStyle: { backgroundColor: colors.ground },
            headerShown: false,
          }}
        >
          <Stack.Screen name="(tabs)" options={{ animation: chuyen("fade") }} />
          <Stack.Screen
            name="create"
            // The route only fades and paints nothing: the screen underneath stays
            // visible under the scrim, and the kit Sheet inside springs the panel.
            options={{ animation: chuyen("fade"), contentStyle: { backgroundColor: "transparent" }, presentation: "transparentModal" }}
          />
          <Stack.Screen
            name="check-ins/new"
            options={{ animation: chuyen("slide_from_bottom"), presentation: "modal" }}
          />
          <Stack.Screen
            name="moments/new"
            options={{ animation: chuyen("slide_from_bottom"), presentation: "modal" }}
          />
          <Stack.Screen
            name="stories/new"
            options={{ animation: chuyen("slide_from_bottom"), presentation: "modal" }}
          />
          {/* The viewer is its own root so the hardware Back closes it and
              `check_screens_reachable` finds it; full screen over the cover. */}
          <Stack.Screen
            name="stories/[personId]"
            options={{ animation: chuyen("fade"), presentation: "fullScreenModal" }}
          />
        </Stack>
        {/* Last child: the dock paints over whatever route is open. */}
        <NepNoi />
      </NepProvider>
      </SoDoiProvider>
      </RudiSessionProvider>
    </SafeAreaProvider>
    </GestureHandlerRootView>
  );
}
