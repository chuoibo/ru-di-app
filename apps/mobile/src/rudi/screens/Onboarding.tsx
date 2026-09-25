import { Ionicons } from "@expo/vector-icons";
import * as Haptics from "expo-haptics";
import { useRouter } from "expo-router";
import { useEffect, useState } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";

import { doiTenTrongPhien, suaHoSoToi } from "../../phien";
import { manDau } from "../duong-vao";
import { laTenGiuCho } from "../ten-giu-cho";
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
import { bongGiay, typography, useRudiTheme } from "../theme";
import { Heading, RudiScreen, TopBar } from "../ui";
import { ONhapMuc } from "../ui/ONhapMuc";
import { StampButton } from "../ui/StampButton";
import { GuGlyph } from "../ui/art/Gu";

/** The words are the SERVER's (`so-thich.ts`, held equal to `GET /interests`
 *  by `tests/test_interest_vocabulary_matches_client.py`); the picture for each
 *  is decoration and lives in `art/gu.ts`, keyed by the same id. */
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
 *
 * 2026-09-08 (report 07/09 §9.3): the page had three tiers of words before the
 * first choice (a lead paragraph, a second heading, a budget explanation) and
 * eight identical tiles told apart only by a hairline icon. Now the question
 * is the heading, the eight tastes are objects drawn with one pen
 * (`GuGlyph`), and the budget is one labelled row. The heading and the button
 * names are pinned by the Maestro board and stay as they are.
 */
/** How a pressed sticker leans, the same for the same place on the sheet. */
const NGHIENG_DAN = [-2, 1.5, -1, 2, -1.5, 1];

export function PersonalizationScreen() {
  const router = useRouter();
  const { colors, dark } = useRudiTheme();
  // At a large font scale two columns leave a label the width of one word,
  // and Android breaks «Shopping» in half rather than wrap it (dark/1.3
  // board, 2026-09-08). The grid falls back to one column instead.
  const session = useRudiSession();
  const personId = session.phien?.person_id ?? null;
  // Ask for a name here, once, while the account still carries the server's
  // placeholder. Without it the person who looks this number up to invite them
  // reads «Thành viên mới» and cannot tell they found the right one, and the
  // invitation then refuses any other name (QA 23/09). Skippable like the rest.
  const hoiTen = personId !== null && laTenGiuCho(session.phien?.profile?.display_name);
  const [ten, setTen] = useState("");

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
  /** Save the typed name, if any; a failure is not worth blocking the step. */
  const luuTen = async () => {
    const phien = session.phien;
    if (!hoiTen || phien === null || ten.trim() === "") return;
    try {
      const hoSo = await suaHoSoToi(phien.person_id, { display_name: ten.trim() });
      // The session carries the name the rest of the app greets with; without
      // this it would keep the placeholder until the next sign-in.
      session.datPhien(await doiTenTrongPhien(phien, hoSo.display_name));
    } catch {
      // Editable later from Cá nhân; the step itself stays a door, not a gate.
    }
  };

  const boQua = async () => {
    await luuTen();
    router.replace(personId === null ? "/explore" : manDau(session.phien));
  };

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
      await luuTen();
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
          <Pressable accessibilityRole="button" hitSlop={8} onPress={() => void boQua()} style={({ pressed }) => [styles.boQua, pressed && styles.pressed]}>
            {/* Not a gate. The step is editable forever from Cá nhân, and a
                required question on the first screen of a new account is a
                toll booth, not a personalization. */}
            <Text style={[typography.label, { color: colors.inkSoft }]}>Bỏ qua</Text>
          </Pressable>
        }
      />
      {hoiTen ? (
        <ONhapMuc
          accessibilityLabel="Ô tên của bạn"
          co="lon"
          autoCapitalize="words"
          label="Bạn tên gì?"
          maxLength={60}
          onChangeText={setTen}
          placeholder="Tên bạn bè hay gọi bạn"
          textContentType="name"
          value={ten}
        />
      ) : null}
      <Heading title="Cho Rủ Đi biết gu của bạn" />
      <View style={styles.block}>
        {/* A sheet of stickers (ADR-0037 D1, plan S6/S7): each taste is its
            drawing on a paper sticker; choosing one presses it onto the page --
            a slight lean, a coral rim, a check. Chosen is said three ways,
            never by colour alone. */}
        <View style={styles.bangSticker}>
          {danhSach.map((m, i) => {
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
                  styles.sticker,
                  {
                    backgroundColor: selected ? colors.accentSoft : colors.card,
                    borderColor: selected ? colors.accent : colors.lineStrong,
                    borderWidth: selected ? 2 : 1,
                    borderStyle: selected ? "solid" : "dashed",
                    transform: [{ rotate: selected ? `${NGHIENG_DAN[i % NGHIENG_DAN.length]}deg` : "0deg" }],
                  },
                  selected && bongGiay(1, dark),
                  pressed && styles.pressed,
                ]}
              >
                <GuGlyph id={m.id} size={52} tone={selected ? "accent" : "ink"} />
                <Text numberOfLines={2} style={[typography.label, styles.giua, { color: colors.ink }]}>{m.nhan}</Text>
                <Ionicons color={selected ? colors.accent : colors.lineStrong} name={selected ? "checkmark-circle" : "ellipse-outline"} size={18} style={styles.dauChon} />
              </Pressable>
            );
          })}
        </View>
        <Text accessibilityLiveRegion="polite" style={[duDieuKien ? typography.note : typography.caption, { color: duDieuKien ? colors.inkSoft : colors.ink }]}>
          {demChon}
        </Text>
      </View>
      <View style={styles.blockNganSach}>
        {/* The question at question weight, then one note with what is
            optional and what K means; no paragraph. Tapping the chosen band
            again clears it (`doiKhoang`). */}
        <Text style={[typography.title, { color: colors.ink }]}>Mỗi lần đi chơi, bạn thường tiêu khoảng</Text>
        <Text style={[typography.note, { color: colors.inkSoft }]}>Không bắt buộc · K là nghìn đồng</Text>
        {/* Envelopes, thicker as the amount grows; tap the chosen one again to clear it. */}
        <View style={styles.hangPhongBi}>
          {NGAN_SACH.map((k, i) => {
            const chon = khoang === k.id;
            return (
              <Pressable
                accessibilityLabel={k.nhan}
                accessibilityRole="radio"
                accessibilityState={{ selected: chon }}
                aria-checked={chon}
                key={k.id}
                onPress={() => doiKhoang(k.id)}
                style={({ pressed }) => [styles.phongBi, { backgroundColor: chon ? colors.accentSoft : colors.card, borderColor: chon ? colors.accent : colors.lineStrong, borderWidth: chon ? 2 : 1 }, pressed && styles.pressed]}
              >
                <View style={[styles.napPhongBi, { borderColor: colors.lineStrong }]} />
                <View style={[styles.dayPhongBi, { height: 2 + i * 3, backgroundColor: colors.lineStrong }]} />
                <Text numberOfLines={1} style={[typography.label, { color: colors.ink }]}>{k.nhan}</Text>
              </Pressable>
            );
          })}
        </View>
      </View>
      {loi !== null ? <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.warn }]}>{loi}</Text> : null}
      <StampButton disabled={!duDieuKien || dangLuu} label={nhanNut} loading={dangLuu} onPress={() => void xong()} size="vua" tilt={-1} />
      <Text style={[typography.note, styles.privacyText, { color: colors.inkFaint }]}>
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
  blockNganSach: { gap: 8, marginTop: 10 },
  bangSticker: { flexDirection: "row", flexWrap: "wrap", gap: 10 },
  // Three a row on a phone, more on a wider sheet; never narrower than the drawing.
  sticker: { flexGrow: 1, flexBasis: 96, maxWidth: 160, minHeight: 116, alignItems: "center", justifyContent: "center", gap: 6, padding: 10, borderRadius: 10 },
  dauChon: { position: "absolute", top: 6, right: 6 },
  giua: { textAlign: "center" },
  hangPhongBi: { flexDirection: "row", flexWrap: "wrap", gap: 8 },
  phongBi: { flexGrow: 1, flexBasis: 90, minHeight: 64, borderRadius: 4, alignItems: "center", justifyContent: "flex-end", paddingBottom: 14, paddingTop: 20, overflow: "hidden" },
  napPhongBi: { position: "absolute", top: -14, width: 40, height: 28, borderWidth: 1, transform: [{ rotate: "45deg" }] },
  dayPhongBi: { position: "absolute", bottom: 0, left: 0, right: 0, opacity: 0.5 },
  privacyText: { textAlign: "center", paddingHorizontal: 18 },
});
