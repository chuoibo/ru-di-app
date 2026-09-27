/**
 * The door a real person comes through.
 *
 * Until this screen existed, RuDi -- the 21 screens somebody actually sees --
 * had no way in at all. The only button on the login screen that did anything
 * was «Vào bản trải nghiệm». The working door lived in `src/screens/len-plan/
 * NhanLoiMoi.tsx`, rendered only by `VoTab.tsx`, reachable only through
 * `/legacy`. Two apps in one binary, and the pretty one had no lock.
 *
 * ## What it reuses rather than rebuilds
 *
 * - `doiLoiMoiLayPhien` from `src/phien.ts` -- the exchange, the SecureStore
 *   write, and the `Idempotency-Key` that turns a dropped response into a
 *   replay instead of a spent secret and a locked-out person.
 * - `cauSauKhiNhan` from `NhanLoiMoi.tsx` -- the sentences. Signing in and
 *   joining are two different things, and there is exactly one place in this
 *   repo that knows how to say which one happened. A second copy would drift,
 *   and the drift would show up as two screens telling one person two stories.
 *
 * ## Two ways in, one screen
 *
 * A link (`rudi://moi/<token>`) hands the code through `loi-moi-den.ts`; a
 * person who was sent the code some other way pastes it. Both end at the same
 * request, so there is one place where redemption can be wrong.
 *
 * ## What it must not do
 *
 * Say "thành công". `membership_state` is the difference between somebody who
 * is in and somebody a member still has to accept, and merging those is how a
 * person ends up staring at an empty group wondering what they did wrong.
 */
import { useRouter } from "expo-router";
import { useEffect, useState } from "react";
import { StyleSheet, Text, View } from "react-native";

import { ApiError, thongDiepNguoiDoc } from "../../api";
import { doiLoiMoiLayPhien, vaoNhom, type Phien } from "../../phien";
import { cauSauKhiNhan } from "../loi-moi-den";
import { layLoiMoiDen } from "../loi-moi-den";
import { CUA_FIXTURE_DEV } from "../cua-fixture";
import { useRudiSession } from "../session";
import { typography, useRudiTheme } from "../theme";
import { Heading, RudiButton, RudiScreen, TopBar } from "../ui";
import { ChuThichLe } from "../ui/ChuThichLe";
import { DauLon } from "../ui/DauLon";
import { NepTrongTrang } from "../ui/NepRoi";
import { ONhapMuc } from "../ui/ONhapMuc";
import { PhongBi } from "../ui/PhongBi";
import { StampButton } from "../ui/StampButton";

type Trang =
  | { pha: "cho-ma" }
  | { pha: "dang-doi" }
  | { pha: "xong"; phien: Phien }
  | { pha: "dang-vao" ; phien: Phien }
  | { pha: "hong"; loi: string };

export function LoiMoiScreen() {
  const router = useRouter();
  const { colors } = useRudiTheme();
  // Signing in writes the disk and the bearer; only the provider can put the
  // session into force for the screens already mounted. Without this the
  // person lands on the group and reads fixtures until they restart the app.
  const { datPhien } = useRudiSession();
  const [ma, setMa] = useState("");
  const [trang, setTrang] = useState<Trang>({ pha: "cho-ma" });

  // A link fills the field; it does not redeem on its own. Spending a
  // single-use secret is irreversible, so it stays a thing somebody pressed.
  useEffect(() => {
    const den = layLoiMoiDen();
    if (den !== null) setMa(den);
  }, []);

  const nhan = async () => {
    const sach = ma.trim();
    if (sach === "") {
      setTrang({ pha: "hong", loi: "Dán mã lời mời bạn được gửi." });
      return;
    }
    setTrang({ pha: "dang-doi" });
    try {
      const phien = await doiLoiMoiLayPhien(sach);
      datPhien(phien);
      setTrang({ pha: "xong", phien });
    } catch (error) {
      setTrang({
        pha: "hong",
        loi: error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null),
      });
    }
  };

  const dongY = async (phien: Phien) => {
    setTrang({ pha: "dang-vao", phien });
    try {
      // The screen re-reads the SERVER's answer rather than assuming the press
      // worked. `vaoNhom` writes back whatever state came home, so a row the
      // server declined to move leaves the person where they actually are
      // instead of on a screen that says they are in.
      const daVao = await vaoNhom(phien);
      datPhien(daVao);
      setTrang({ pha: "xong", phien: daVao });
    } catch (error) {
      setTrang({
        pha: "hong",
        loi: error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null),
      });
    }
  };

  if (trang.pha === "xong" || trang.pha === "dang-vao") {
    // `active` is in; anything else is signed in and still waiting. The
    // session is real either way, so the app reloads itself through the entry
    // screen rather than dropping somebody into a group they cannot read.
    const daVao = trang.phien.membership_state === "active";
    const dangVao = trang.pha === "dang-vao";
    return (
      <RudiScreen testID="loi-moi-screen">
        <TopBar title="Lời mời" />
        <Heading
          title={daVao ? "Xong, bạn đã ở trong nhóm" : "Đã đăng nhập"}
          subtitle={cauSauKhiNhan(daVao ? "active" : "invited", "phien")}
        />
        {/* The letter taken out of its envelope, stamped with where it stands. */}
        <PhongBi testID="loi-moi-mo">
          <DauLon co="vua" dong nhan={daVao ? "Đã vào nhóm" : "Chờ bạn gật"} tilt={-4} tone="ink" />
        </PhongBi>
        {daVao ? (
          <StampButton label="Vào nhóm" onPress={() => router.replace("/explore")} size="vua" tilt={-1} />
        ) : (
          <>
            {/* The step that used to be missing, and it is a step rather than
                something done for somebody. A member chose this person by
                name; what is left is this person saying yes, and saying yes is
                a press. Doing it silently on their behalf would put somebody
                in a group without ever asking. */}
            <StampButton disabled={dangVao} label="Đồng ý vào nhóm" loading={dangVao} onPress={() => void dongY(trang.phien)} size="vua" tilt={-1} />
            <RudiButton
              label="Để sau"
              onPress={() => router.replace("/welcome")}
              variant="ghost"
            />
          </>
        )}
      </RudiScreen>
    );
  }

  return (
    <RudiScreen testID="loi-moi-screen">
      <TopBar title="Lời mời" />
      <Heading
        title="Bạn được rủ đi"
        subtitle="Dán mã trong lời mời. Rủ Đi chỉ vào được bằng lời mời của một người đã ở trong nhóm."
      />
      {/* An envelope with a letter in it: the code is what the letter says
          (ADR-0037 D1, plan S3). Nếp brings it over. */}
      <View style={styles.canh}>
        <NepTrongTrang pose="dua-giay" size={96} />
        <PhongBi style={styles.flex} testID="phong-bi-loi-moi">
          <ONhapMuc
            accessibilityLabel="Mã lời mời"
            autoCapitalize="none"
            autoCorrect={false}
            label="Mã lời mời"
            onChangeText={setMa}
            placeholder="Dán mã ở đây"
            value={ma}
          />
        </PhongBi>
      </View>
      <StampButton disabled={trang.pha === "dang-doi"} label="Nhận lời mời" loading={trang.pha === "dang-doi"} onPress={() => void nhan()} size="vua" tilt={-1} />
      {trang.pha === "hong" ? (
        <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.warn }]}>{trang.loi}</Text>
      ) : null}
      <ChuThichLe icon="mail-open-outline">
        Chưa có lời mời? Nhờ một người trong nhóm gửi cho bạn. Đây là chủ ý, không phải thiếu sót: không ai tự tạo tài khoản trước khi có bạn rủ đi.
      </ChuThichLe>
      {/* The fixture door exists only on a QA build (`cua-fixture.ts`); a real
          person reading «bản trải nghiệm» here took it for a demo app. */}
      {CUA_FIXTURE_DEV ? (
        <RudiButton
          label="Xem bản trải nghiệm"
          onPress={() => router.replace("/welcome")}
          variant="ghost"
        />
      ) : null}
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  canh: { flexDirection: "row", alignItems: "flex-end", gap: 8 },
  flex: { flex: 1 },
});
