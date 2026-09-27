/**
 * Invite somebody into a group by their telephone number (M2).
 *
 * Three server calls, in the order the server needs them: the number becomes a
 * person id (`POST /identity/person-id`, a keyed digest -- the number is never
 * stored), the id gets the name the inviter knows them by (`PUT /people/{id}`,
 * 200 when they already had one -- an existing name is never overwritten
 * silently: the server keeps theirs; a 403 there means "their own name
 * stands", not "stop", see `moi-bang-so.ts`), and then the membership is created as
 * `invited` (`POST /contexts/{id}/members`).
 *
 * When that person later signs in with the same number, the OTP door derives
 * the same id (ADR-0016), so the invitation is waiting for them on the «Tin
 * nhắn» tab with a «Đồng ý» button. Nobody is put into a group without a press.
 *
 * One `Attempt` per call, kept across retries of the same press, so a dropped
 * response replays instead of inviting twice (the server would answer 409 to a
 * second membership, and that would read like a bug in a working invite).
 */
import { Redirect, useLocalSearchParams, useRouter } from "expo-router";
import { useRef, useState } from "react";
import { StyleSheet, Text, View } from "react-native";

import { ApiError, newAttempt, registerPerson, thongDiepNguoiDoc, type Attempt } from "../../../api";
import { chuanHoaSo } from "../../../screens/vao-cua/danh-tinh";
import { layIdTuSo, moiVaoNhom } from "../../../screens/vao-cua/cong-api";
import { moiBangSo } from "../../moi-bang-so";
import { useRudiSession } from "../../session";
import { typography, useRudiTheme } from "../../theme";
import { Heading, RudiButton, RudiScreen, TopBar } from "../../ui";
import { DauLon } from "../../ui/DauLon";
import { ONhapMuc } from "../../ui/ONhapMuc";
import { PhongBi } from "../../ui/PhongBi";
import { StampButton } from "../../ui/StampButton";

type Trang =
  | { pha: "nhap" }
  | { pha: "dang-moi" }
  | { pha: "xong"; ten: string; tenDaDat: boolean }
  | { pha: "hong"; loi: string };

export function GroupInviteScreen() {
  const router = useRouter();
  const { colors } = useRudiTheme();
  const { id } = useLocalSearchParams<{ id: string }>();
  const { phien, phienDaDoc } = useRudiSession();
  const [phone, setPhone] = useState("");
  const [ten, setTen] = useState("");
  const [trang, setTrang] = useState<Trang>({ pha: "nhap" });
  const lanBam = useRef<{ khoa: string; dat: Attempt; moi: Attempt } | null>(null);

  if (!phienDaDoc) return null;
  if (phien === null) return <Redirect href="/welcome" />;
  if (typeof id !== "string") return <Redirect href="/messages" />;

  const moi = async () => {
    const soSach = phone.trim();
    const tenSach = ten.trim();
    if (chuanHoaSo(soSach) === null) {
      setTrang({ pha: "hong", loi: "Chưa đúng dạng số di động Việt Nam." });
      return;
    }
    if (tenSach === "") {
      setTrang({ pha: "hong", loi: "Đặt tên cho người bạn đang mời, để cả nhóm biết đó là ai." });
      return;
    }
    const khoa = `${soSach}|${tenSach}`;
    if (lanBam.current === null || lanBam.current.khoa !== khoa) {
      lanBam.current = { khoa, dat: newAttempt(), moi: newAttempt() };
    }
    setTrang({ pha: "dang-moi" });
    try {
      const lan = lanBam.current;
      const { tenDaDat } = await moiBangSo(
        {
          layId: layIdTuSo,
          datTen: (personId, tenMoi) => registerPerson({ id: personId, name: tenMoi }, phien.person_id, lan.dat),
          moi: async (personId) => {
            await moiVaoNhom(id, personId, phien.person_id, lan.moi);
          },
        },
        soSach,
        tenSach,
      );
      setTrang({ pha: "xong", ten: tenSach, tenDaDat });
    } catch (error) {
      setTrang({
        pha: "hong",
        loi: error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null),
      });
    }
  };

  if (trang.pha === "xong") {
    return (
      <RudiScreen testID="group-invite-screen">
        <TopBar title="Mời vào nhóm" />
        <Heading
          title={trang.tenDaDat ? `Đã mời ${trang.ten}` : "Đã mời số này"}
          subtitle={
            trang.tenDaDat
              ? "Khi người này đăng nhập bằng số đó, lời mời hiện ở tab Tin nhắn và chính họ bấm «Đồng ý»."
              : `Người này đã dùng Rủ Đi và có tên riêng, nên cả nhóm sẽ thấy tên do chính họ đặt, không phải «${trang.ten}». Lời mời đang chờ ở tab Tin nhắn của họ.`
          }
        />
        {/* The envelope, sealed: the letter went (ADR-0037 D1). */}
        <PhongBi style={styles.phongBi} testID="loi-moi-da-gui">
          <Text style={[typography.title, { color: colors.ink }]}>Gửi {trang.ten}</Text>
          <DauLon co="vua" dong nhan="Đã gửi" tilt={-4} tone="ink" />
        </PhongBi>
        <RudiButton label="Xem thành viên" onPress={() => router.back()} />
        <RudiButton
          label="Mời thêm người"
          onPress={() => {
            setPhone("");
            setTen("");
            setTrang({ pha: "nhap" });
          }}
          variant="outline"
        />
      </RudiScreen>
    );
  }

  const dangMoi = trang.pha === "dang-moi";
  return (
    <RudiScreen contentStyle={styles.screen} testID="group-invite-screen">
      <TopBar title="Mời vào nhóm" />
      <Heading
        title="Mời bằng số điện thoại"
        subtitle="Số điện thoại chỉ dùng để nhận ra đúng người khi họ đăng nhập; Rủ Đi không lưu số."
      />
      {/* The invitation is a letter: its address is who it goes to (plan S3). */}
      <PhongBi style={styles.phongBi} testID="phong-bi-moi">
        <Text style={[typography.stamp, { color: colors.inkSoft }]}>Lời mời vào nhóm</Text>
        <ONhapMuc
          accessibilityLabel="Ô tên người được mời"
          editable={!dangMoi}
          label="Gửi"
          maxLength={200}
          onChangeText={setTen}
          placeholder="Bạn gọi người này là gì"
          value={ten}
        />
        <ONhapMuc
          accessibilityLabel="Ô số điện thoại người được mời"
          autoComplete="tel"
          editable={!dangMoi}
          keyboardType="phone-pad"
          label="Số di động"
          onChangeText={setPhone}
          placeholder="Số di động của bạn ấy"
          textContentType="telephoneNumber"
          value={phone}
        />
      </PhongBi>
      {trang.pha === "hong" ? (
        <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.warn }]}>{trang.loi}</Text>
      ) : null}
      <StampButton disabled={dangMoi} label="Gửi lời mời" loading={dangMoi} onPress={() => void moi()} size="vua" tilt={-1} />
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  screen: { gap: 20, maxWidth: 560 },
  phongBi: { alignSelf: "stretch" },
});
