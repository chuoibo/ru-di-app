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
import { HoaDonGiay, VachCat } from "../ui/HoaDonGiay";
import { Money } from "../ui/Money";
import { useFocusEffect, useRouter } from "expo-router";
import { useCallback, useEffect, useState } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";

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
import { HoSoSong, type ProfileEditorControls } from "./profile/HoSoSong";
import { useAdaptiveLayout } from "../ui/useAdaptiveLayout";
import { HanhTrinhTeaser } from "./profile/HanhTrinhTeaser";
import { Heading, ListRow, NhomHang, RudiButton, RudiScreen, SectionHeader, TopBar } from "../ui";
import { useLuiLop } from "../ui/useLuiLop";
import { useTenCho } from "../to-giay/useTenCho";
import { ErrorState } from "../ui/ErrorState";
import { SkeletonGroup, SkeletonLines, SkeletonRow } from "../ui/Skeleton";
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
  const [page, setPage] = useState<"profile" | "kept">("profile");
  const [editor, setEditor] = useState<ProfileEditorControls | null>(null);
  const editingProfile = editor !== null;
  const dangLuuHoSo = editor?.busy ?? false;
  const insets = useSafeAreaInsets();
  const { rail } = useAdaptiveLayout();
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
      <RudiScreen material="quiet" bottomInset="tab" cot="doc" testID="profile-screen">
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
      <RudiScreen material="quiet" bottomInset="tab" cot="doc" testID="profile-screen">
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
    <RudiScreen material="quiet" bottomInset={editingProfile ? 16 : "tab"} cot="doc" avoidKeyboard={editingProfile} cuonVeDau={editingProfile ? "edit" : "passport"} testID="profile-screen"
      header={editor ? <TopBar title="Chỉnh hồ sơ" onBack={editor.back} right={<RudiButton compact full={false} label="Huỷ" disabled={dangLuuHoSo} onPress={editor.cancel} variant="ghost" />} /> : undefined}
      footer={editor ? <RudiButton label="Lưu hồ sơ" disabled={dangLuuHoSo} loading={dangLuuHoSo} onPress={editor.save} /> : undefined}
      footerInset={rail ? insets.bottom + 12 : 24}>
      {!editingProfile ? <View style={styles.profileTop}>
        <View style={styles.flex}>
          <Heading title="Cá nhân" />
        </View>
        {/* One door to Settings, and it is the row below. A gear in this corner
            reads as a second one, and on a dev client the launcher's own gear
            sits on top of it, so the corner is the worst place for it. */}
      </View> : null}
      <HoSoSong phien={phien} onEditorChange={setEditor} />
      {!editingProfile ? <>
      <View accessibilityRole="tablist" style={[styles.profileTabs, { borderBottomColor: colors.line }]}>
        {([['profile', 'Hồ sơ'], ['kept', 'Trang đã giữ']] as const).map(([value, label]) => <Pressable key={value} accessibilityRole="tab" accessibilityState={{ selected: page === value }} onPress={() => setPage(value)} style={[styles.profileTab, { borderBottomColor: page === value ? colors.accent : 'transparent' }]}><Text style={[typography.label, styles.profileTabLabel, { color: page === value ? colors.ink : colors.inkSoft }]}>{label}</Text></Pressable>)}
      </View>
      {page === "kept" ? <>
      <DiaryWall person={phien.person_id} owner={phien.person_id} />
      <HanhTrinhTeaser personId={personId} />
      </> : <>
      <View>
            <Text accessibilityRole="header" style={[typography.title, styles.menuHeading, { color: colors.ink }]}>Kết nối &amp; gu</Text>
            <View style={[styles.hangMenu, { borderBottomColor: colors.line }]}>
              <ListRow
                icon="people-outline"
                onPress={() => router.push((loiMoiCho > 0 ? "/friends?muc=da-nhan" : "/friends") as never)}
                subtitle={loiMoiCho > 0 ? `${loiMoiCho} lời mời đang chờ` : undefined}
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
                icon="sparkles-outline"
                onPress={() => router.push("/personalization")}
                subtitle={tomTat(soThich)}
                title="Sở thích"
              />
            </View>
        <Text accessibilityRole="header" style={[typography.title, styles.menuHeading, { color: colors.ink }]}>Sổ của bạn</Text>
        <View style={[styles.hangMenu, { borderBottomColor: colors.line }]}>
          <ListRow icon="newspaper-outline" onPress={() => router.push(duongTuongToi)} title="Tường của tôi" />
        </View>
        <View style={[styles.hangMenu, { borderBottomColor: colors.line }]}>
          <ListRow
            icon="wallet-outline"
            onPress={() => router.push("/finance")}
            title="Tài chính của tôi"
            tone="split"
          />
        </View>
        <View style={[styles.hangMenu, { borderBottomColor: colors.line }]}>
          <ListRow
            icon="ribbon-outline"
            onPress={() => router.push("/achievements")}
            title="Hành trình"
          />
        </View>
        <View style={[styles.hangMenu, { borderBottomColor: colors.line }]}>
          <ListRow
            icon="bookmark-outline"
            onPress={() => setPanel("saved")}
            subtitle={soLuuHien === null ? undefined : `${soLuuHien} địa điểm`}
            title="Đã lưu"
          />
        </View>
        <Text accessibilityRole="header" style={[typography.title, styles.menuHeading, { color: colors.ink }]}>Quản lý</Text>
        <View style={[styles.hangMenu, { borderBottomColor: colors.line }]}>
          <ListRow
            icon="settings-outline"
            onPress={() => router.push("/settings" as never)}
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
            title="Tài khoản"
          />
        </View>
      </View>
      </>}
      </> : null}
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
    return <TaiChinhLive key={session.nguon.actorId} actorId={session.nguon.actorId} contextId={session.nguon.contextId} />;
  }
  // Signed in, in no group yet: the finance route is per PERSON, so it reads
  // the real (empty) ledger.
  if (session.phien !== null) {
    return <TaiChinhLive key={session.phien.person_id} actorId={session.phien.person_id} contextId={null} />;
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
  const { colors, dark } = useRudiTheme();
  const [du, setDu] = useState<Finance | null>(null);
  const [loi, setLoi] = useState<string | null>(null);
  const [lan, setLan] = useState(0);

  useFocusEffect(useCallback(() => {
    let song = true;
    setDu(null);
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
  }, [actorId, lan]));

  if (loi !== null) {
    return (
      <RudiScreen cot="form" material="quiet" tone="split" testID="finance-screen">
        <TopBar title="Tài chính của tôi" />
        <ErrorState body={loi} onRetry={() => setLan((n) => n + 1)} title="Chưa đọc được sổ" />
      </RudiScreen>
    );
  }
  if (du === null) {
    return (
      <RudiScreen cot="form" material="quiet" tone="split" testID="finance-screen">
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
    <RudiScreen cot="form" material="quiet" tone="split" testID="finance-screen">
      <TopBar title="Tài chính của tôi" />
      {/* Separate ledger figures, with no total inferred on the client. */}
      <HoaDonGiay rangTren testID="trang-so-tai-chinh">
        {([
          { nhan: "Phần chi của bạn", vnd: du.spend_vnd, tone: "split" },
          { nhan: "Còn phải trả", phu: `Đã trả ${formatVnd(du.settled_vnd)}đ`, vnd: du.outstanding_vnd, tone: du.outstanding_vnd > 0 ? "warn" : "ink" },
          { nhan: "Sẽ nhận", phu: "Bạn đã ứng trước", vnd: du.receivable_vnd, tone: "split" },
        ] as const).map((dong) => (
          <View key={dong.nhan} style={[styles.dongSoTaiChinh, { borderBottomColor: colors.line }]}>
            <View>
              <Text style={[typography.label, { color: colors.ink }]}>{dong.nhan}</Text>
              {"phu" in dong ? <Text style={[typography.caption, { color: colors.inkSoft }]}>{dong.phu}</Text> : null}
            </View>
            <Money numberOfLines={0} size="label" style={styles.tienTaiChinhDayDu} tone={dong.tone} vnd={dong.vnd} />
          </View>
        ))}
        <Text style={[typography.caption, { color: colors.inkSoft }]}>{`${du.expense_count} khoản chi trong ${du.group_count} nhóm. Tính lại từ sổ mỗi lần mở.`}</Text>
      </HoaDonGiay>
      {/* Every figure is said once, on the page above; under it only where
          they come from and the way to the group's settlement. A section
          heading here promised rows the finance read does not carry («Chi theo
          nhóm», QA UI-060), then repeated «Sẽ nhận» in a sentence («Ai nợ ai»,
          B4 finish review). */}
      <ChuThichLe icon="calculator-outline">Các số trên đọc từ sổ cái, không phải số dư ngân hàng.</ChuThichLe>
      {contextId !== null ? (
        <RudiButton compact icon="wallet-outline" label="Xem quyết toán" onPress={() => router.push(("/settlements/" + contextId) as never)} tone="split" variant="ghost" />
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
                <Text style={[typography.label, styles.soMoc, { color: m.direction === "in" ? colors.split : colors.ink }]}>{tienCoDau(m)}</Text>
              </View>
            </View>
          ))}
        </View>
      )}
      {gioiHan !== null ? <Text style={[typography.caption, { color: colors.inkSoft }]}>{gioiHan}</Text> : null}
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  dongSoTaiChinh: { alignItems: "stretch", gap: 4, minWidth: 0, paddingVertical: 10, borderBottomWidth: StyleSheet.hairlineWidth },
  tienTaiChinhDayDu: { width: "100%", minWidth: 0 },
  mocTien: { flexDirection: "row", gap: 12, minHeight: 64 },
  cotMoc: { width: 14, alignItems: "center", paddingTop: 6 },
  chamMoc: { width: 12, height: 12, borderRadius: 6, borderWidth: 2 },
  dayMoc: { width: 2, flex: 1, marginTop: 4 },
  thanMoc: { flex: 1, minWidth: 0, gap: 2, paddingBottom: 16 },
  ngayMoc: { lineHeight: 18 },
  soMoc: { maxWidth: "100%", fontVariant: ["tabular-nums"], paddingTop: 2 },
  flex: { flex: 1 },
  dem: { minWidth: 24, height: 24, borderRadius: 12, alignItems: "center", justifyContent: "center", paddingHorizontal: 7 },
  form: { maxWidth: 560 },
  khung: { gap: 14 },
  profileTop: { flexDirection: "row", alignItems: "flex-start", justifyContent: "space-between", gap: 10 },
  profileTabs: { flexDirection: "row", gap: 16, borderBottomWidth: StyleSheet.hairlineWidth },
  profileTab: { flex: 1, minHeight: 48, paddingVertical: 12, alignItems: "center", justifyContent: "center", borderBottomWidth: 2 },
  profileTabLabel: { width: "100%", minWidth: 0, textAlign: "center" },
  menuHeading: { paddingTop: 24, paddingBottom: 8 },
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
