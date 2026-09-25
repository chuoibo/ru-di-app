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
import { Redirect, useRouter } from "expo-router";
import { useRef, useState } from "react";
import { Pressable, StyleSheet, Text, View, useWindowDimensions } from "react-native";

import { ApiError, newAttempt, thongDiepNguoiDoc, type Attempt } from "../../../api";
import { docNhomCuaToi, ganDanhSachNhom } from "../../../phien";
import { taoNhom } from "../../../screens/vao-cua/cong-api";
import { manDau } from "../../duong-vao";
import { useRudiSession } from "../../session";
import { typography, useRudiTheme } from "../../theme";
import { Heading, RudiScreen, TopBar } from "../../ui";
import { ChuThichLe } from "../../ui/ChuThichLe";
import { ONhapMuc } from "../../ui/ONhapMuc";
import { SoBia } from "../../ui/SoBia";
import { StampButton } from "../../ui/StampButton";

type Trang = { pha: "nhap" } | { pha: "dang-mo" } | { pha: "hong"; loi: string };

export function GroupNewScreen() {
  const router = useRouter();
  const { colors } = useRudiTheme();
  const { width } = useWindowDimensions();
  const { phien, phienDaDoc, datPhien } = useRudiSession();
  const [ten, setTen] = useState("");
  const [trang, setTrang] = useState<Trang>({ pha: "nhap" });
  const lanBam = useRef<{ ten: string; attempt: Attempt } | null>(null);

  if (!phienDaDoc) return null;
  if (phien === null) return <Redirect href="/welcome" />;

  const mo = async () => {
    const sach = ten.trim();
    if (sach === "") {
      setTrang({ pha: "hong", loi: "Đặt tên cho nhóm." });
      return;
    }
    if (lanBam.current === null || lanBam.current.ten !== sach) {
      lanBam.current = { ten: sach, attempt: newAttempt() };
    }
    setTrang({ pha: "dang-mo" });
    try {
      await taoNhom(sach, phien.person_id, lanBam.current.attempt);
      const nhom = await docNhomCuaToi(phien.person_id);
      const moi = await ganDanhSachNhom(phien, nhom);
      datPhien(moi);
      router.replace(manDau(moi) as never);
    } catch (error) {
      setTrang({
        pha: "hong",
        loi: error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null),
      });
    }
  };

  const dangMo = trang.pha === "dang-mo";

  // A new notebook on the table (ADR-0037 D1, plan S3): its cloth cover, and
  // the group's name written on the label pasted on the front.
  const rongBia = Math.min(300, width - 64);
  return (
    <RudiScreen contentStyle={styles.screen} testID="group-new-screen">
      <TopBar title="Nhóm mới" />
      <Heading title="Đặt tên cho hội" subtitle="Bạn là quản trị của nhóm này. Mời bạn bè sau." />
      <SoBia
        cao={Math.round(rongBia * 0.62)}
        nhan={
          <View style={[styles.nhan, { backgroundColor: colors.card, borderColor: colors.coverLineStrong }]}>
            <ONhapMuc
              accessibilityLabel="Ô tên nhóm"
              autoFocus
              editable={!dangMo}
              maxLength={200}
              onChangeText={setTen}
              onSubmitEditing={() => void mo()}
              placeholder="Hội cafe cuối tuần"
              returnKeyType="done"
              style={styles.giua}
              value={ten}
            />
          </View>
        }
        rong={rongBia}
        style={styles.bia}
        ten={[]}
        testID="bia-nhom-moi"
      />
      {trang.pha === "hong" ? (
        <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.warn }]}>{trang.loi}</Text>
      ) : null}
      <StampButton disabled={dangMo} label="Mở nhóm" loading={dangMo} onPress={() => void mo()} size="vua" tilt={-1} />
      {/* A couple is not a group: QA 23/09 found two people inventing «Minh &
          Linh» here because nothing pointed them at the direct conversation,
          where the two-person notebook lives. */}
      <Pressable accessibilityRole="link" onPress={() => router.push("/friends/add")}>
        <ChuThichLe icon="people-outline">
          Chỉ hai người? Không cần nhóm: kết bạn bằng số điện thoại rồi nhắn riêng, sổ hai người nằm ở đó.{" "}
          <Text style={{ color: colors.ink, textDecorationLine: "underline" }}>Thêm bạn</Text>
        </ChuThichLe>
      </Pressable>
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  screen: { gap: 20, maxWidth: 560 },
  bia: { alignSelf: "center" },
  nhan: { borderWidth: 1, borderRadius: 3, paddingHorizontal: 12, paddingVertical: 6 },
  giua: { textAlign: "center" },
});
