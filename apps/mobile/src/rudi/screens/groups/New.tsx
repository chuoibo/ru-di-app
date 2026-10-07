/**
 * Open a group from inside RuDi.
 *
 * `taoNhom` is the legacy client module, unchanged: the server makes the
 * creator an active admin inside the same transaction, so there is no
 * invite-yourself step and no window with a group nobody administers.
 *
 * One attempt per intent. The `Idempotency-Key` is minted when the name is
 * first submitted and reused for a retry of the SAME name, so a dropped
 * response replays instead of opening a second «Nhóm OTP». A changed name is a
 * different intent and mints a new key -- the server fingerprints the body.
 *
 * After the write the screen re-reads `GET /people/me/contexts` rather than
 * trusting the response: the session needs the membership id and state the
 * list carries, and it is the list `chonNhomMacDinh` reads.
 */
import { useRouter } from "expo-router";
import { useRef, useState } from "react";
import { Pressable, StyleSheet, Text, View, useWindowDimensions } from "react-native";

import { ApiError, newAttempt, thongDiepNguoiDoc, type Attempt } from "../../../api";
import { chonNhom, docNhomCuaToi, ganDanhSachNhom } from "../../../phien";
import { taoNhom } from "../../../screens/vao-cua/cong-api";
import { useRudiSession } from "../../session";
import { typography, useRudiTheme } from "../../theme";
import { Heading, RudiScreen, TopBar } from "../../ui";
import { ChuThichLe } from "../../ui/ChuThichLe";
import { ONhapMuc } from "../../ui/ONhapMuc";
import { AssetIllustration } from "../../asset-pack/react-native/AssetIllustration";
import { Grain } from "../../ui/Grain";
import { StampButton } from "../../ui/StampButton";
import { CuaDangNhap } from "../../ui/CuaDangNhap";

type Trang = { pha: "nhap" } | { pha: "dang-mo" } | { pha: "hong"; loi: string };

export function GroupNewScreen() {
  const router = useRouter();
  const { colors } = useRudiTheme();
  const { width, fontScale } = useWindowDimensions();
  const { phien, phienDaDoc, datPhien } = useRudiSession();
  const [ten, setTen] = useState("");
  const [trang, setTrang] = useState<Trang>({ pha: "nhap" });
  const lock = useRef(false);
  const lanBam = useRef<{ ten: string; attempt: Attempt } | null>(null);

  if (!phienDaDoc) return null;
  if (phien === null) return <CuaDangNhap />;

  const mo = async () => {
    if (lock.current) return;
    const sach = ten.trim();
    if (sach === "") {
      setTrang({ pha: "hong", loi: "Đặt tên cho nhóm." });
      return;
    }
    if (lanBam.current === null || lanBam.current.ten !== sach) {
      lanBam.current = { ten: sach, attempt: newAttempt() };
    }
    lock.current = true;
    setTrang({ pha: "dang-mo" });
    try {
      const lap = await taoNhom(sach, phien.person_id, lanBam.current.attempt);
      const nhom = await docNhomCuaToi(phien.person_id);
      // Into the group just opened, its own room, where the first thing on
      // the page is inviting friends (QA UI-071: it used to land on Khám phá
      // with no group name and no way to invite).
      const moi = await chonNhom(await ganDanhSachNhom(phien, nhom), lap.id);
      datPhien(moi);
      router.replace(`/groups/${lap.id}/chat` as never);
    } catch (error) {
      setTrang({
        pha: "hong",
        loi: error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null),
      });
    } finally { lock.current = false; }
  };

  const dangMo = trang.pha === "dang-mo";

  // A new notebook on the table (ADR-0037 D1, plan S3): its cloth cover, and
  // the group's name written on the label pasted on the front.
  const nhanTrenBia = fontScale < 1.3 && width >= 360;
  const rongBia = Math.max(140, Math.min(nhanTrenBia ? 300 : 180, width - 56));
  const nhapTen = (
    <ONhapMuc
      accessibilityLabel="Tên nhóm mới"
      label="Tên trên bìa sổ"
      editable={!dangMo}
      maxLength={200}
      onChangeText={setTen}
      onSubmitEditing={() => void mo()}
      placeholder="Hội cà phê cuối tuần"
      returnKeyType="done"
      value={ten}
    />
  );
  return (
    <RudiScreen material="quiet" avoidKeyboard contentStyle={styles.screen} testID="group-new-screen">
      <TopBar title="Nhóm mới" />
      <Heading title="Đặt tên cho hội" subtitle="Bạn là quản trị của nhóm này. Mời bạn bè sau." />
      <View style={[styles.bia, { width: rongBia, height: Math.round(rongBia / 0.75) }]}>
        <View pointerEvents="none" accessible={false} style={StyleSheet.absoluteFill}>
          <AssetIllustration name="decor-cover" width={rongBia} height={Math.round(rongBia / 0.75)} />
        </View>
        {nhanTrenBia ? (
          <View style={[styles.nhan, { left: rongBia * 0.12, top: rongBia * 0.57, width: rongBia * 0.7, backgroundColor: colors.card, borderColor: colors.paperShade }]}>
            <Grain material="giayTrang" opacity={0.32} />
            {nhapTen}
          </View>
        ) : null}
      </View>
      {nhanTrenBia ? null : nhapTen}
      {trang.pha === "hong" ? (
        <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.warn }]}>{trang.loi}</Text>
      ) : null}
      <StampButton disabled={dangMo} label="Mở nhóm" loading={dangMo} onPress={() => void mo()} size="vua" tilt={-1} style={styles.mo} />
      {/* A couple is not a group: QA 23/09 found two people inventing «Minh &
          Linh» here because nothing pointed them at the direct conversation,
          where the two-person notebook lives. */}
      <Pressable accessibilityRole="link" onPress={() => router.push("/friends/add")}>
        <ChuThichLe icon="people-outline">
          Muốn rủ riêng một người? Hai bạn có thể cùng đồng ý mở sổ hai người.{" "}
          <Text style={{ color: colors.ink, textDecorationLine: "underline" }}>Thêm bạn</Text>
        </ChuThichLe>
      </Pressable>
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  screen: { gap: 20, maxWidth: 560 },
  bia: { alignSelf: "center" },
  nhan: { position: "absolute", borderWidth: 1, paddingHorizontal: 12, paddingVertical: 14, transform: [{ rotate: "-2deg" }] },
  mo: { alignSelf: "stretch" },
  giua: { textAlign: "center" },
});
