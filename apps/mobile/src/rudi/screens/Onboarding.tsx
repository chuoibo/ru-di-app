import { Ionicons } from "@expo/vector-icons";
import * as Haptics from "expo-haptics";
import { useLocalSearchParams, useRouter } from "expo-router";
import { useEffect, useState } from "react";
import { Pressable, StyleSheet, Text, useWindowDimensions, View } from "react-native";

import { doiTenTrongPhien, suaHoSoToi } from "../../phien";
import { duongTiep, manDau } from "../duong-vao";
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
import { RudiScreen, TopBar } from "../ui";
import { Grain } from "../ui/Grain";
import { PaperObject } from "../asset-pack/react-native/PaperObject";
import { usePaperPalette } from "../asset-pack/react-native/usePaperPalette";
import { ONhapMuc } from "../ui/ONhapMuc";
import { StampButton } from "../ui/StampButton";
import { GuGlyph } from "../ui/art/Gu";
import { toggleState } from "../../ui/a11y";
import { laVaoQuaLoiMoi } from "../duong-vao";

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
 * review §8.3). Here the choices sit directly on the page in short paper rows,
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

/** The supplied cut-paper note grows with live labels, including large text. */
function GuPaper({ selected }: { selected: boolean }) {
  const { colors, dark } = useRudiTheme();
  const palette = usePaperPalette(true);
  const [box, setBox] = useState({ w: 0, h: 0 });
  return <View pointerEvents="none" accessible={false} style={StyleSheet.absoluteFill} onLayout={(event) => {
    const w = Math.round(event.nativeEvent.layout.width), h = Math.round(event.nativeEvent.layout.height);
    setBox((old) => old.w === w && old.h === h ? old : { w, h });
  }}>
    {box.w > 0 ? <View style={{ position: "absolute", left: -box.w * 0.06, top: -box.h * 0.05 }}>
      <PaperObject name="note" layer="paper" width={box.w * 1.12} height={box.h * 1.1} palette={{ ...palette, paper: selected ? colors.accentSoft : dark ? colors.paper : colors.ground, shade: selected ? colors.accent : colors.lineStrong }} />
    </View> : null}
    <Grain material="giayTrang" opacity={0.32} style={styles.stickerGrain} />
  </View>;
}

export function PersonalizationScreen() {
  const router = useRouter();
  // A new person who arrived by a link goes on to it after this step.
  const thamSo = useLocalSearchParams<{ tiep?: string }>();
  const tiep = duongTiep(thamSo.tiep);
  // Arrived through a group invitation (QA UI-073): the account carries the
  // name the inviter typed. It is shown, filled in, and the person decides.
  // QA UI-073: arrived through a group's invitation, read from the session
  // (not from the URL, which anyone can edit).
  const quaLoiMoi = laVaoQuaLoiMoi(useRudiSession().phien);
  const { colors, dark } = useRudiTheme();
  const { width, fontScale } = useWindowDimensions();
  const haiCot = width < 360 || Math.round(fontScale * 100) >= 130;
  const nganSachMotCot = Math.round(fontScale * 100) >= 130;
  const session = useRudiSession();
  const personId = session.phien?.person_id ?? null;
  // Ask for a name here, once, while the account still carries the server's
  // placeholder. Without it the person who looks this number up to invite them
  // reads «Thành viên mới» and cannot tell they found the right one, and the
  // invitation then refuses any other name (QA 23/09). Skippable like the rest.
  const tenNhomGoi = quaLoiMoi ? (session.phien?.profile?.display_name?.trim() ?? "") : "";
  const hoiTen = personId !== null && (laTenGiuCho(session.phien?.profile?.display_name) || (quaLoiMoi && tenNhomGoi !== ""));
  const [ten, setTen] = useState(laTenGiuCho(tenNhomGoi) ? "" : tenNhomGoi);

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

  /** Save the typed name, if any; a failure is not worth blocking the step. */
  const luuTen = async () => {
    const phien = session.phien;
    // An invited person who keeps the name the group gave writes nothing.
    if (!hoiTen || phien === null || ten.trim() === "" || ten.trim() === tenNhomGoi) return;
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
    router.replace((tiep ?? manDau(session.phien)) as never);
  };

  const xong = async () => {
    setLoi(null);
    if (personId === null) {
      // The route is behind `CanPhien`; without a session there is nothing to
      // attach the answers to.
      router.replace("/login");
      return;
    }
    setDangLuu(true);
    try {
      await luuTen();
      await luuSoThich(personId, { muc, khoang });
      router.replace((tiep ?? manDau(session.phien)) as never);
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
    <RudiScreen material="quiet" avoidKeyboard contentStyle={styles.personalization} testID="personalization-screen">
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
          helper={quaLoiMoi && tenNhomGoi !== "" && !laTenGiuCho(tenNhomGoi) ? `Nhóm mời bạn đang gọi bạn là «${tenNhomGoi}». Sửa nếu bạn muốn được gọi khác.` : undefined}
          label="Bạn tên gì?"
          maxLength={60}
          onChangeText={setTen}
          placeholder="Tên bạn bè hay gọi bạn"
          textContentType="name"
          value={ten}
        />
      ) : null}
      <View style={styles.heading}>
        <Text style={[typography.h1, { color: colors.ink }]}>Điều gì khiến bạn muốn ra ngoài?</Text>
        <Text style={[typography.body, styles.headingBody, { color: colors.inkSoft }]}>Chọn vài điều mình thích. Gu đổi theo ngày, bạn luôn sửa lại được.</Text>
      </View>
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
                {...toggleState("checkbox", selected, () => doiMucChon(m.id))}
                onPress={() => doiMucChon(m.id)}
                style={({ pressed }) => [
                  styles.sticker,
                  haiCot && styles.haiCot,
                  {
                    transform: [{ rotate: selected ? `${NGHIENG_DAN[i % NGHIENG_DAN.length]}deg` : "0deg" }],
                  },
                  bongGiay(1, dark),
                  pressed && styles.pressed,
                ]}
              >
                <GuPaper selected={selected} />
                <GuGlyph id={m.id} size={42} tone={selected ? "accent" : "ink"} />
                <Text style={[typography.label, styles.giua, { color: colors.ink }]}>{m.nhan}</Text>
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
                {...toggleState("radio", chon, () => doiKhoang(k.id))}
                key={k.id}
                onPress={() => doiKhoang(k.id)}
                style={({ pressed }) => [styles.phongBi, nganSachMotCot && styles.motCot, { backgroundColor: chon ? colors.accentSoft : colors.paper, borderColor: chon ? colors.accent : colors.lineStrong, borderWidth: chon ? 2 : 1 }, pressed && styles.pressed]}
              >
                <View style={[styles.napPhongBi, { borderColor: colors.lineStrong }]} />
                <View style={[styles.dayPhongBi, { height: 2 + i * 3, backgroundColor: colors.lineStrong }]} />
                <Text style={[typography.label, styles.giua, { color: colors.ink }]}>{k.nhan}</Text>
                <Text style={[typography.caption, styles.giua, { color: colors.inkSoft }]}>{k.phu}</Text>
                {chon ? <Ionicons name="checkmark-circle" size={18} color={colors.accent} style={styles.dauChon} /> : null}
              </Pressable>
            );
          })}
        </View>
      </View>
      {loi !== null ? <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.warn }]}>{loi}</Text> : null}
      <StampButton
        disabled={!duDieuKien || dangLuu}
        label={nhanNut}
        loading={dangLuu}
        lyDo={duDieuKien ? undefined : `Chọn thêm ${TOI_THIEU - muc.length} mục nữa.`}
        onPress={() => void xong()}
        size="vua"
        style={styles.save}
        tilt={-1}
      />
      <Text style={[typography.note, styles.privacyText, { color: colors.inkFaint }]}>
        {cauLuuTru(personId !== null)}
      </Text>
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  pressed: { opacity: 0.7 },
  personalization: { maxWidth: 640, gap: 12 },
  boQua: { minHeight: 48, justifyContent: "center", paddingHorizontal: 6 },
  heading: { gap: 6 },
  headingBody: { fontSize: 15, lineHeight: 22 },
  block: { gap: 8 },
  blockNganSach: { gap: 6, marginTop: 2 },
  bangSticker: { flexDirection: "row", flexWrap: "wrap", justifyContent: "center", gap: 8 },
  // Three short chips; narrow windows and large text keep two readable columns.
  sticker: { flexBasis: "31%", maxWidth: "31%", minHeight: 86, minWidth: 48, alignItems: "center", justifyContent: "center", gap: 3, paddingHorizontal: 6, paddingVertical: 8, borderRadius: 6 },
  haiCot: { flexBasis: "48%", maxWidth: "48%" },
  stickerGrain: { borderRadius: 5 },
  motCot: { flexBasis: "100%", maxWidth: "100%" },
  dauChon: { position: "absolute", top: 4, right: 4 },
  giua: { textAlign: "center" },
  hangPhongBi: { flexDirection: "row", flexWrap: "wrap", gap: 8 },
  // Four envelopes since ADR-0038 §2.5: two to a row, never a lone fourth.
  phongBi: { flexGrow: 1, flexBasis: 140, minHeight: 60, borderRadius: 4, alignItems: "center", justifyContent: "flex-end", paddingHorizontal: 10, paddingBottom: 8, paddingTop: 14, overflow: "hidden" },
  napPhongBi: { position: "absolute", top: -14, width: 40, height: 28, borderWidth: 1, transform: [{ rotate: "45deg" }] },
  dayPhongBi: { position: "absolute", bottom: 0, left: 0, right: 0, opacity: 0.5 },
  privacyText: { textAlign: "center", paddingHorizontal: 18 },
  save: { alignSelf: "stretch" },
});
