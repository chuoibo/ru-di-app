import { Ionicons } from "@expo/vector-icons";
import * as Haptics from "expo-haptics";
import { useRouter } from "expo-router";
import { useEffect, useState } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";

import { manDau } from "../duong-vao";
import {
  LOI_SO_THICH,
  cauLuuTru,
  docSoThich,
  docTuVung,
  luuSoThich,
  maLoi,
} from "../nguoi/so-thich-song";
import { NGAN_SACH, SO_THICH, doiMuc } from "../../screens/vao-cua/so-thich";
import { useRudiSession } from "../session";
import { typography, useRudiTheme } from "../theme";
import { Chip, Heading, Inline, ResponsiveRow, RudiButton, RudiScreen, TopBar } from "../ui";

/** One icon per taste word the SERVER knows. The words themselves come from
 *  `so-thich.ts`, which `tests/test_interest_vocabulary_matches_client.py`
 *  keeps equal to the server's list; the icon is decoration and lives here. */
const BIEU_TUONG: Record<string, keyof typeof Ionicons.glyphMap> = {
  "an-uong": "restaurant-outline",
  cafe: "cafe-outline",
  nightlife: "wine-outline",
  "mon-local": "fast-food-outline",
  outdoor: "trail-sign-outline",
  shopping: "bag-handle-outline",
  karaoke: "mic-outline",
  game: "game-controller-outline",
};

const TOI_THIEU = 3;

/**
 * The one question the product asks about the person, on paper.
 *
 * The first cut wrapped eight tiles in a card and gave each an icon box, a
 * full progress bar for a single step, and a button called «Tạo không gian
 * của tôi» -- a registration form wearing the journal's colours (2026-09-06
 * review §8.3). Here the choices sit directly on the page in two airy columns,
 * the count says out loud why the button is still waiting («Đã chọn 2/3»),
 * and the button names the thing it does: save, or simply continue when there
 * is no account to save into. The words and their ids are the server's
 * vocabulary and are not edited here.
 */
export function PersonalizationScreen() {
  const router = useRouter();
  const { colors } = useRudiTheme();
  const session = useRudiSession();
  const personId = session.phien?.person_id ?? null;

  const [muc, setMuc] = useState<string[]>([]);
  const [khoang, setKhoang] = useState<string | null>(null);
  const [dangLuu, setDangLuu] = useState(false);
  const [loi, setLoi] = useState<string | null>(null);
  // Which words the server will accept. Null until it answers, and null forever
  // if it cannot be reached -- the screen renders the local list either way.
  const [tuVung, setTuVung] = useState<string[] | null>(null);

  useEffect(() => {
    let con = true;
    void docTuVung().then((ds) => con && setTuVung(ds));
    return () => {
      con = false;
    };
  }, []);

  const danhSach = tuVung === null ? SO_THICH : SO_THICH.filter((m) => tuVung.includes(m.id));

  // What this person already told the server, so re-opening the step shows
  // their answers rather than an empty screen that looks like a reset.
  useEffect(() => {
    if (personId === null) return;
    let con = true;
    void docSoThich(personId)
      .then((da) => {
        if (!con) return;
        setMuc(da.muc);
        setKhoang(da.khoang);
      })
      .catch(() => undefined);
    return () => {
      con = false;
    };
  }, [personId]);

  /** Tapping the chosen band again clears it: «bỏ qua» has to stay reachable
   *  after somebody has answered, or the first answer is permanent. */
  const doiKhoang = (id: string) => {
    void Haptics.selectionAsync();
    if (khoang === id) {
      setKhoang(null);
      return;
    }
    setKhoang(id);
  };

  const doiMucChon = (id: string) => {
    void Haptics.selectionAsync();
    setMuc(doiMuc(muc, id));
  };

  // Skipping from the fixture door (no session) goes into the fixture app, not
  // back to the cover: `manDau(null)` is Welcome, which the Maestro board
  // caught as a loop on 2026-09-06.
  const boQua = () => router.replace(personId === null ? "/explore" : manDau(session.phien));

  const xong = async () => {
    setLoi(null);
    if (personId === null) {
      // The dev fixture door reaches this screen with no session. Nothing to
      // attach the answers to, and writing them to a file a later sign-in
      // adopts would make one phone's guesses look like somebody's taste.
      router.replace("/explore");
      return;
    }
    setDangLuu(true);
    try {
      await luuSoThich(personId, { muc, khoang });
      router.replace(manDau(session.phien));
    } catch (error) {
      const ma = maLoi(error);
      setLoi((ma !== null ? LOI_SO_THICH[ma] : null) ?? "Chưa lưu được. Thử lại giúp mình nhé.");
    } finally {
      setDangLuu(false);
    }
  };

  const duDieuKien = muc.length >= TOI_THIEU;
  const demChon =
    muc.length === 0
      ? `Chọn ít nhất ${TOI_THIEU} để tiếp tục.`
      : duDieuKien
        ? `Đã chọn ${muc.length}.`
        : `Đã chọn ${muc.length}/${TOI_THIEU}.`;
  const nhanNut = dangLuu ? "Đang lưu…" : personId === null ? "Tiếp tục" : "Lưu sở thích";

  return (
    <RudiScreen contentStyle={styles.personalization} testID="personalization-screen">
      <TopBar
        right={
          <Pressable accessibilityRole="button" hitSlop={8} onPress={boQua} style={({ pressed }) => [styles.boQua, pressed && styles.pressed]}>
            {/* Not a gate. The step is editable forever from Cá nhân, and a
                required question on the first screen of a new account is a
                toll booth, not a personalization. */}
            <Text style={[typography.label, { color: colors.inkSoft }]}>Bỏ qua</Text>
          </Pressable>
        }
      />
      <Heading
        title="Cho Rủ Đi biết gu của bạn"
        subtitle="Rủ Đi xếp gợi ý theo đúng những gì bạn chọn. Sửa lại bất cứ lúc nào ở Cá nhân."
      />
      <View style={styles.block}>
        <Text style={[typography.h2, { color: colors.ink }]}>Đi chơi, bạn thường mê gì?</Text>
        <ResponsiveRow minItemWidth={140} gap={10}>
          {danhSach.map((m) => {
            const selected = muc.includes(m.id);
            return (
              <Pressable
                key={m.id}
                accessibilityLabel={m.nhan}
                accessibilityRole="checkbox"
                accessibilityState={{ checked: selected }}
                aria-checked={selected}
                onPress={() => doiMucChon(m.id)}
                style={({ pressed }) => [
                  styles.tile,
                  {
                    backgroundColor: selected ? colors.accentSoft : colors.card,
                    borderColor: selected ? colors.accent : colors.lineStrong,
                  },
                  pressed && styles.pressed,
                ]}
              >
                <Ionicons color={selected ? colors.accent : colors.inkSoft} name={BIEU_TUONG[m.id] ?? "sparkles-outline"} size={22} />
                <Text style={[typography.label, styles.tileLabel, { color: colors.ink }]}>{m.nhan}</Text>
                {/* Chosen is said twice: the fill and a check, never colour alone. */}
                <Ionicons
                  color={selected ? colors.accent : colors.lineStrong}
                  name={selected ? "checkmark-circle" : "ellipse-outline"}
                  size={20}
                />
              </Pressable>
            );
          })}
        </ResponsiveRow>
        <Text accessibilityLiveRegion="polite" style={[typography.caption, { color: duDieuKien ? colors.inkSoft : colors.ink }]}>
          {demChon}
        </Text>
      </View>
      <View style={styles.block}>
        <Text style={[typography.h2, { color: colors.ink }]}>Mỗi lần đi chơi bạn tiêu khoảng</Text>
        <Text style={[typography.body, { color: colors.inkSoft }]}>
          Bỏ qua cũng được. Rủ Đi để trống chỗ này chứ không đoán thay bạn.
        </Text>
        <Inline gap={8} wrap>
          {NGAN_SACH.map((k) => (
            <Chip key={k.id} label={k.nhan} onPress={() => doiKhoang(k.id)} selected={khoang === k.id} />
          ))}
        </Inline>
      </View>
      {loi !== null ? <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.warn }]}>{loi}</Text> : null}
      <RudiButton disabled={!duDieuKien || dangLuu} label={nhanNut} loading={dangLuu} onPress={() => void xong()} />
      <Text style={[typography.caption, styles.privacyText, { color: colors.inkFaint }]}>
        {cauLuuTru(personId !== null)}
      </Text>
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  pressed: { opacity: 0.7 },
  personalization: { maxWidth: 640 },
  boQua: { minHeight: 48, justifyContent: "center", paddingHorizontal: 6 },
  block: { gap: 12 },
  tile: { minHeight: 56, flexDirection: "row", alignItems: "center", gap: 10, paddingHorizontal: 12, paddingVertical: 10, borderRadius: 14, borderWidth: 1 },
  tileLabel: { flex: 1, flexShrink: 1 },
  privacyText: { textAlign: "center", paddingHorizontal: 18 },
});
