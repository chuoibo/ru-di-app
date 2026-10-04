import { DiaryWall } from "../diary/Wall";
/**
 * Cá nhân, Tài chính and Hành trình: the person's own pages.
 *
 * On a real session the profile card is `HoSoSong` (the server's words and
 * counts), the finance page is `GET /people/{id}/finance` printed, and the
 * achievements live in `ky-niem/AchievementsLive.tsx`. Signed out, each is
 * the sign-in door.
 *
 * UI v2 (đợt 7): a footprint, not a trophy page. The hero gradient, the
 * level badge and the three-number card are gone; what is left is the
 * person, one line about them, the next appointment (with a real countdown)
 * and a plain list of doors. Money pages answer «bạn đã chi bao nhiêu» first,
 * separate sums from ledger lines, and draw no chart for decoration.
 */
import { DongTien } from "../ui/DongTien";
import { useFocusEffect, useRouter } from "expo-router";
import { useCallback, useEffect, useState } from "react";
import { StyleSheet, Text, View } from "react-native";

import { formatVnd } from "../../../../../packages/shared/money.mjs";
import { docSoThich, tomTat, type SoThichSong } from "../nguoi/so-thich-song";
import { ghiChuGioiHan, layTaiChinh, moTaGiaoDich, ngayNgan, tienCoDau, type Finance } from "../../screens/ca-nhan/tai-chinh";
import { docLoiMoi } from "../../screens/ca-nhan/ban-be";
import { docDaLuu } from "../kham-pha/dia-diem";
import { useRudiSession } from "../session";
import type { Phien } from "../../phien";
import { CuaDangNhap } from "../ui/CuaDangNhap";
import { displayFace, mucNguoi, typography, useRudiTheme } from "../theme";
import { DAU_VAN_CAY } from "../dau-van-cay";
import { HoSoSong } from "./profile/HoSoSong";
import { HanhTrinhTeaser } from "./profile/HanhTrinhTeaser";
import { Heading, ListRow, NhomHang, RudiButton, RudiScreen, SectionHeader, TopBar } from "../ui";
import { useLuiLop } from "../ui/useLuiLop";
import { useTenCho } from "../to-giay/useTenCho";
import { ErrorState } from "../ui/ErrorState";
import { SkeletonGroup, SkeletonLines, SkeletonRow } from "../ui/Skeleton";
import { TrangSo } from "../ui/TrangSo";
import { ChuThichLe } from "../ui/ChuThichLe";
import { useNepNguCanh } from "../nep/NepProvider";

export function ProfileScreen() {
  const { phien, phienDaDoc } = useRudiSession();
  if (!phienDaDoc) return null;
  if (phien === null) return <CuaDangNhap tiep="/profile" />;
  return <ProfileSong phien={phien} />;
}

function ProfileSong({ phien }: { phien: Phien }) {
  const router = useRouter();
  const { colors } = useRudiTheme();
  const session = useRudiSession();
  const [panel, setPanel] = useState<"home" | "account" | "saved">("home");
  useNepNguCanh({ man: "profile", tieuDe: "Trang cá nhân", goiY: ["Gu của mình đang thế nào?", "Mình đã đi những đâu?"] });
  // Read once so the row below keeps the narrowing inside its own callback.
  const duongTuongToi = `/people/${phien.person_id}`;
  const personId = phien.person_id;
  // The row's subtitle is what this person told the server (M11), read on
  // focus rather than once: they may have just changed it on the step itself.
  const [soThich, setSoThich] = useState<SoThichSong>({ muc: [], khoang: null });
  // Counts what the SERVER saved and who is waiting on it: a friend request
  // nobody can see is a request that never arrives.
  const [soDaLuu, setSoDaLuu] = useState<string[] | null>(null);
  const [loiMoiCho, setLoiMoiCho] = useState(0);
  useFocusEffect(
    useCallback(() => {
      let con = true;
      void docSoThich(personId)
        .then((da) => con && setSoThich(da))
        .catch(() => undefined);
      void docDaLuu(personId)
        .then((ids) => con && setSoDaLuu(ids))
        .catch(() => undefined);
      void docLoiMoi(personId, personId, "incoming")
        .then((ds) => con && setLoiMoiCho(ds.filter((loi) => loi.state === "pending").length))
        .catch(() => undefined);
      return () => {
        con = false;
      };
    }, [personId]),
  );
  const soLuuHien = soDaLuu?.length ?? null;
  // QA UI-109: «Đã lưu» lists every saved place by name, each the way to it.
  const idsDaLuu = soDaLuu ?? [];
  const tenDaLuu = useTenCho(panel === "saved" ? idsDaLuu : []);
  // QA UI-108: Back on a panel closes it and stays on Cá nhân.
  useLuiLop(panel !== "home", () => setPanel("home"));

  if (panel === "account") {
    return (
      <RudiScreen bottomInset="tab" cot="doc" testID="profile-screen">
        <TopBar onBack={() => setPanel("home")} title="Tài khoản" />
        <Text style={[typography.body, { color: colors.ink }]}>
          Đang xem với tư cách {phien.profile?.display_name ?? "bạn"}.
        </Text>
        <Text style={[typography.caption, { color: colors.inkFaint }]}>
          Đăng xuất kết thúc phiên đăng nhập, xoá lựa chọn trên máy này rồi đưa về màn chào.
        </Text>
        {DAU_VAN_CAY ? (
          <Text accessibilityLabel="dau-van-cay" style={[typography.caption, { color: colors.inkFaint }]}>
            Bản dựng {DAU_VAN_CAY}
          </Text>
        ) : null}
        <RudiButton
          label="Đăng xuất"
          onPress={() => {
            session.resetSession();
            router.replace("/welcome");
          }}
        />
      </RudiScreen>
    );
  }
  if (panel === "saved") {
    return (
      <RudiScreen bottomInset="tab" cot="doc" testID="profile-screen">
        <TopBar onBack={() => setPanel("home")} title="Đã lưu" />
        <Heading
          title={soLuuHien === null ? "Đã lưu" : `${soLuuHien} địa điểm`}
          subtitle="Danh sách lưu trong tài khoản của bạn. Mở Khám phá để thêm."
        />
        {idsDaLuu.length > 0 ? (
          <NhomHang>
            {idsDaLuu.map((id) => (
              <ListRow icon="bookmark-outline" key={id} onPress={() => router.push(`/places/${id}` as never)} subtitle="Mở trang của chỗ này" title={tenDaLuu[id] ?? "Một chỗ đã lưu"} />
            ))}
          </NhomHang>
        ) : null}
        <RudiButton label="Mở Khám phá" onPress={() => router.push("/explore")} variant={idsDaLuu.length > 0 ? "outline" : "solid"} />
      </RudiScreen>
    );
  }

  return (
    <RudiScreen bottomInset="tab" cot="doc" testID="profile-screen">
      <View style={styles.profileTop}>
        <View style={styles.flex}>
          <Heading title="Cá nhân" subtitle="Không gian của riêng bạn" />
        </View>
        {/* One door to Settings, and it is the row below. A gear in this corner
            reads as a second one, and on a dev client the launcher's own gear
            sits on top of it, so the corner is the worst place for it. */}
      </View>
      <HoSoSong phien={phien} />
      <DiaryWall person={phien.person_id} owner={phien.person_id} />
      <HanhTrinhTeaser personId={personId} />
      <View>
            <View style={[styles.hangMenu, { borderBottomColor: colors.line }]}>
              <ListRow
                icon="people-outline"
                onPress={() => router.push((loiMoiCho > 0 ? "/friends?muc=da-nhan" : "/friends") as never)}
                subtitle={loiMoiCho > 0 ? `${loiMoiCho} lời mời kết bạn đang chờ bạn` : "Bạn bè, lời mời đã nhận và đã gửi"}
                title="Bạn bè"
                trailing={
                  loiMoiCho > 0 ? (
                    <View accessibilityLabel={`${loiMoiCho} lời mời đang chờ`} style={[styles.dem, { backgroundColor: colors.accent }]}>
                      <Text style={[typography.caption, { color: colors.accentInk }]}>{loiMoiCho}</Text>
                    </View>
                  ) : undefined
                }
              />
            </View>
            <View style={[styles.hangMenu, { borderBottomColor: colors.line }]}>
              <ListRow
                icon="newspaper-outline"
                onPress={() => duongTuongToi && router.push(duongTuongToi)}
                subtitle="Bài bạn đã đăng và ai đọc được"
                title="Tường của tôi"
              />
            </View>
            <View style={[styles.hangMenu, { borderBottomColor: colors.line }]}>
              <ListRow
                icon="sparkles-outline"
                onPress={() => router.push("/personalization")}
                subtitle={tomTat(soThich)}
                title="Sở thích"
              />
            </View>
        <View style={[styles.hangMenu, { borderBottomColor: colors.line }]}>
          <ListRow
            icon="wallet-outline"
            onPress={() => router.push("/finance")}
            subtitle="Chi tiêu, công nợ và lịch sử"
            title="Tài chính của tôi"
            tone="split"
          />
        </View>
        <View style={[styles.hangMenu, { borderBottomColor: colors.line }]}>
          <ListRow
            icon="ribbon-outline"
            onPress={() => router.push("/achievements")}
            subtitle="Sổ huy hiệu và các ngã rẽ của bạn"
            title="Hành trình"
          />
        </View>
        <View style={[styles.hangMenu, { borderBottomColor: colors.line }]}>
          <ListRow
            icon="bookmark-outline"
            onPress={() => setPanel("saved")}
            subtitle={soLuuHien === null ? "Địa điểm bạn đã lưu" : `${soLuuHien} địa điểm trong tài khoản`}
            title="Đã lưu"
          />
        </View>
        <View style={[styles.hangMenu, { borderBottomColor: colors.line }]}>
          <ListRow
            icon="settings-outline"
            onPress={() => router.push("/settings" as never)}
            subtitle="Hồ sơ, phiên, quyền riêng tư, giao diện"
            title="Cài đặt"
          />
        </View>
        {/* «Tài khoản» and «Đăng xuất» stay here, on this screen, with these
            words: `_dang-xuat-neu-co.yaml` walks through them on every OTP
            table, and moving them into Settings would break the evidence of
            every other slice. */}
        <View style={[styles.hangMenu, { borderBottomColor: colors.line }]}>
          <ListRow
            icon="shield-checkmark-outline"
            onPress={() => setPanel("account")}
            subtitle="Quyền riêng tư và đăng xuất"
            title="Tài khoản"
          />
        </View>
      </View>
    </RudiScreen>
  );
}

/**
 * The finance screen, on the same switch as the settlement screen.
 *
 * These two are the pair the whole PR was about: they must never print two
 * different totals for one trip. Wiring only ONE of them to the server would
 * recreate exactly that defect, pointed the other way -- measured on the
 * emulator, where the settlement screen showed the seeded group's 6.785.000đ
 * while this one still showed the fixture's 3.840.000đ in the same launch.
 */
export function FinanceScreen() {
  const session = useRudiSession();
  if (session.nguon.kieu === "live") {
    return <TaiChinhLive actorId={session.nguon.actorId} contextId={session.nguon.contextId} />;
  }
  // Signed in, in no group yet: the finance route is per PERSON, so it reads
  // the real (empty) ledger.
  if (session.phien !== null) {
    return <TaiChinhLive actorId={session.phien.person_id} contextId={null} />;
  }
  if (!session.phienDaDoc) return null;
  return <CuaDangNhap tiep="/finance" />;
}

/**
 * `GET /people/{id}/finance`, printed.
 *
 * No arithmetic: the route's own docstring says `settled + outstanding == spend`
 * is guaranteed by the ledger query that answers it, and deriving even one of
 * the three here would be a second implementation of the same sum.
 */
function TaiChinhLive({ actorId, contextId }: { actorId: string; contextId: string | null }) {
  const router = useRouter();
  const { colors, dark, radius } = useRudiTheme();
  const [du, setDu] = useState<Finance | null>(null);
  const [loi, setLoi] = useState<string | null>(null);
  const [lan, setLan] = useState(0);

  useEffect(() => {
    let song = true;
    setLoi(null);
    void layTaiChinh(actorId)
      .then((ketQua) => {
        if (song) setDu(ketQua);
      })
      .catch((error: unknown) => {
        if (!song) return;
        setLoi(error instanceof Error ? error.message : "Không đọc được tài chính.");
      });
    return () => {
      song = false;
    };
  }, [actorId, lan]);

  if (loi !== null) {
    return (
      <RudiScreen tone="split" testID="finance-screen">
        <TopBar title="Tài chính của tôi" />
        <ErrorState body={loi} onRetry={() => setLan((n) => n + 1)} title="Chưa đọc được sổ" />
      </RudiScreen>
    );
  }
  if (du === null) {
    return (
      <RudiScreen tone="split" testID="finance-screen">
        <TopBar title="Tài chính của tôi" />
        <SkeletonGroup style={styles.khung}>
          <SkeletonLines lastWidth="50%" lineHeight={28} lines={2} />
          <SkeletonRow leading={0} />
        </SkeletonGroup>
      </RudiScreen>
    );
  }
  const gioiHan = ghiChuGioiHan(du.movements);
  return (
    <RudiScreen tone="split" testID="finance-screen">
      <TopBar title="Tài chính của tôi" />
      {/* The one answer first, as the first line of a ledger: what this
          person's share of everything has come to, written on the ledger page. */}
      <TrangSo ke={false} testID="trang-so-tai-chinh">
        <DongTien dam nhan="Phần chi của bạn" phu={`${du.expense_count} khoản chi trong ${du.group_count} nhóm. Tính lại từ sổ mỗi lần mở.`} tone="split" vnd={du.spend_vnd} />
        <DongTien nhan="Còn phải trả" phu={`Đã trả ${formatVnd(du.settled_vnd)}đ`} tone={du.outstanding_vnd > 0 ? "warn" : "ink"} vnd={du.outstanding_vnd} />
        <DongTien cuoi nhan="Sẽ nhận" phu="Bạn đã ứng trước" tone="split" vnd={du.receivable_vnd} />
      </TrangSo>
      {/* Every figure is said once, on the page above; under it only where
          they come from and the way to the group's settlement. A section
          heading here promised rows the finance read does not carry («Chi theo
          nhóm», QA UI-060), then repeated «Sẽ nhận» in a sentence («Ai nợ ai»,
          B4 finish review). */}
      <ChuThichLe icon="calculator-outline">Các số trên đọc từ sổ cái, không phải số dư ngân hàng.</ChuThichLe>
      {contextId !== null ? (
        <RudiButton compact full={false} icon="wallet-outline" label="Xem quyết toán" onPress={() => router.push(("/settlements/" + contextId) as never)} tone="split" variant="ghost" />
      ) : null}
      {/* What has actually arrived, newest first: the movements the server
          always sent and the screen never showed. Each one a line on the
          timeline, dotted in the other person's ink. */}
      <SectionHeader title="Tiền đã về" />
      {du.movements.length === 0 ? (
        <Text style={[typography.body, { color: colors.inkSoft }]}>Chưa có khoản chuyển nào được xác nhận là đã về.</Text>
      ) : (
        <View testID="dong-thoi-gian">
          {du.movements.map((m, i) => (
            <View key={m.obligation_id} style={styles.mocTien}>
              <View style={styles.cotMoc}>
                <View style={[styles.chamMoc, { backgroundColor: mucNguoi(m.counterparty_id, dark), borderColor: colors.card }]} />
                {i < du.movements.length - 1 ? <View style={[styles.dayMoc, { backgroundColor: colors.line }]} /> : null}
              </View>
              <View style={[styles.flex, styles.thanMoc]}>
                <Text style={[typography.stamp, styles.ngayMoc, { color: colors.inkSoft }]}>{ngayNgan(m.occurred_at)}</Text>
                <Text style={[typography.body, { color: colors.ink }]}>{moTaGiaoDich(m)}</Text>
                {m.context_name || m.occasion ? (
                  <Text style={[typography.caption, { color: colors.inkSoft }]}>{[m.occasion, m.context_name].filter(Boolean).join(" · ")}</Text>
                ) : null}
              </View>
              <Text style={[typography.label, styles.soMoc, { color: m.direction === "in" ? colors.split : colors.ink }]}>{tienCoDau(m)}</Text>
            </View>
          ))}
        </View>
      )}
      {gioiHan !== null ? <Text style={[typography.caption, { color: colors.inkSoft }]}>{gioiHan}</Text> : null}
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  mocTien: { flexDirection: "row", gap: 12, minHeight: 64 },
  cotMoc: { width: 14, alignItems: "center", paddingTop: 6 },
  chamMoc: { width: 12, height: 12, borderRadius: 6, borderWidth: 2 },
  dayMoc: { width: 2, flex: 1, marginTop: 4 },
  thanMoc: { gap: 2, paddingBottom: 16 },
  ngayMoc: { lineHeight: 18 },
  soMoc: { fontVariant: ["tabular-nums"], paddingTop: 16 },
  flex: { flex: 1 },
  dem: { minWidth: 24, height: 24, borderRadius: 12, alignItems: "center", justifyContent: "center", paddingHorizontal: 7 },
  form: { maxWidth: 560 },
  khung: { gap: 14 },
  profileTop: { flexDirection: "row", alignItems: "flex-start", justifyContent: "space-between", gap: 10 },
  hero: { gap: 10 },
  heroDau: { flexDirection: "row", alignItems: "center", gap: 14 },
  upcoming: { flexDirection: "row", alignItems: "center", gap: 14, padding: 16, marginTop: 10 },
  dauLich: { minWidth: 64, alignItems: "center" },
  ngay: { fontFamily: displayFace.extraBold, fontSize: 32, lineHeight: 36, letterSpacing: -1, fontVariant: ["tabular-nums"] },
  pressed: { opacity: 0.75 },
  hangMenu: { borderBottomWidth: StyleSheet.hairlineWidth },
  hangSo: { flexDirection: "row", alignItems: "center", gap: 12, minHeight: 60, paddingVertical: 10, borderBottomWidth: StyleSheet.hairlineWidth },
  transactionIcon: { width: 40, height: 40, borderRadius: 20, borderWidth: 1, alignItems: "center", justifyContent: "center" },
  ghiChu: { flexDirection: "row", alignItems: "flex-start", gap: 9 },
  tienDo: { fontVariant: ["tabular-nums"] },
  thuThach: { gap: 8, paddingTop: 4 },
});
