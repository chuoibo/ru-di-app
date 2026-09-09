/**
 * Cá nhân, Tài chính and Thành tích: the person's own pages.
 *
 * On a real session the profile card is `HoSoSong` (the server's words and
 * counts), the finance page is `GET /people/{id}/finance` printed, and the
 * achievements live in `ky-niem/AchievementsLive.tsx`. The fixture build
 * keeps Team Đà Lạt's story under a demo badge.
 *
 * UI v2 (đợt 7): a footprint, not a trophy page. The hero gradient, the
 * level badge and the three-number card are gone; what is left is the
 * person, one line about them, the next appointment (with a real countdown)
 * and a plain list of doors. Money pages answer «bạn đã chi bao nhiêu» first,
 * separate sums from ledger lines, and draw no chart for decoration.
 */
import { Ionicons } from "@expo/vector-icons";
import { BILL_ITEMS, MEMORY_PHOTOS } from "../fixtures";
import { DongTien } from "../ui/DongTien";
import { useFocusEffect, useRouter } from "expo-router";
import { useCallback, useEffect, useState } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";

import { COLLECTOR_INDEX, DEMO_GROUP, PEOPLE, formatVnd } from "../fixtures";
import { docSoThich, tomTat, type SoThichSong } from "../nguoi/so-thich-song";
import { layTaiChinh, tinhTrangNo, type Finance } from "../../screens/ca-nhan/tai-chinh";
import { nhanKhoangNgay } from "../../screens/len-plan/buoi-di";
import { dauLich, homNay, nhanNhip, nhipKeo } from "../keo/nhip-keo";
import { noiLuu, noiLuuNgan } from "../luu-tru";
import { useRudiSession } from "../session";
import { displayFace, typography, useRudiTheme } from "../theme";
import { DAU_VAN_CAY } from "../dau-van-cay";
import { HoSoSong } from "./profile/HoSoSong";
import {
  DemoBadge,
  Field,
  Heading,
  ListRow,
  RudiButton,
  RudiScreen,
  SectionHeader,
  TopBar,
  type IconName,
} from "../ui";
import { Avatar } from "../ui/Avatar";
import { ErrorState } from "../ui/ErrorState";
import { Money } from "../ui/Money";
import { SkeletonGroup, SkeletonLines, SkeletonRow } from "../ui/Skeleton";
import { Stamp } from "../ui/Stamp";

/** «17/10/2026» (the fixture's own format) as the ISO day `nhip-keo` reads. */
function isoTu(ddmmyyyy: string): string {
  const [d, m, y] = ddmmyyyy.split("/");
  if (!d || !m || !y) return ddmmyyyy;
  return `${y}-${m.padStart(2, "0")}-${d.padStart(2, "0")}`;
}

export function ProfileScreen() {
  const router = useRouter();
  const { colors, radius } = useRudiTheme();
  const session = useRudiSession();
  const [panel, setPanel] = useState<"home" | "account" | "edit" | "saved">("home");
  // Read once so the row below keeps the narrowing inside its own callback.
  const duongTuongToi = session.phien === null ? null : `/people/${session.phien.person_id}`;
  const personId = session.phien?.person_id ?? null;
  // The row's subtitle is what this person told the server (M11), read on
  // focus rather than once: they may have just changed it on the step itself.
  const [soThich, setSoThich] = useState<SoThichSong>({ muc: [], khoang: null });
  useFocusEffect(
    useCallback(() => {
      if (personId === null) return;
      let con = true;
      void docSoThich(personId)
        .then((da) => con && setSoThich(da))
        .catch(() => undefined);
      return () => {
        con = false;
      };
    }, [personId]),
  );

  if (panel === "account") {
    return (
      <RudiScreen bottomInset="tab" testID="profile-screen">
        <TopBar onBack={() => setPanel("home")} title="Tài khoản" />
        <Text style={[typography.body, { color: colors.ink }]}>
          Đang xem với tư cách {session.phien?.profile?.display_name ?? session.displayName}.
        </Text>
        <Text style={[typography.caption, { color: colors.inkFaint }]}>
          {session.phien !== null
            ? "Đăng xuất kết thúc phiên trên máy chủ, xoá lựa chọn trên máy này rồi đưa về màn chào."
            : "Đăng xuất xoá mọi lựa chọn của lần mở app này rồi đưa về welcome. Phiên này không ký máy chủ."}
        </Text>
        {DAU_VAN_CAY ? (
          <Text accessibilityLabel="dau-van-cay" style={[typography.caption, { color: colors.inkFaint }]}>
            Bản dựng {DAU_VAN_CAY}
          </Text>
        ) : null}
        <RudiButton
          label={session.phien !== null ? "Đăng xuất" : "Đăng xuất bản trải nghiệm"}
          onPress={() => {
            session.resetSession();
            router.replace("/welcome");
          }}
        />
      </RudiScreen>
    );
  }
  if (panel === "edit") {
    return (
      <RudiScreen bottomInset="tab" contentStyle={styles.form} testID="profile-screen">
        <TopBar onBack={() => setPanel("home")} title="Chỉnh hồ sơ" />
        <Field label="Tên" onChangeText={session.setDisplayName} value={session.displayName} />
        <Field label="Bio" multiline onChangeText={session.setBio} value={session.bio} />
        <RudiButton label="Xong" onPress={() => setPanel("home")} />
      </RudiScreen>
    );
  }
  if (panel === "saved") {
    return (
      <RudiScreen bottomInset="tab" testID="profile-screen">
        <TopBar onBack={() => setPanel("home")} title="Đã lưu" />
        <Heading
          title={`${session.savedPlaceIds.length} địa điểm`}
          subtitle={`Danh sách ${noiLuu(session.luuTruSong)}. Mở Khám phá để thêm.`}
        />
        <RudiButton label="Mở Khám phá" onPress={() => router.push("/explore")} />
      </RudiScreen>
    );
  }

  const nhip = nhanNhip(nhipKeo(isoTu(session.startDate), isoTu(session.endDate), homNay()));
  const dau = dauLich(isoTu(session.startDate));

  return (
    <RudiScreen bottomInset="tab" testID="profile-screen">
      <View style={styles.profileTop}>
        <View style={styles.flex}>
          <Heading title="Cá nhân" subtitle="Không gian của riêng bạn" />
        </View>
        {/* One door to Settings, and it is the row below. A gear in this corner
            reads as a second one, and on a dev client the launcher's own gear
            sits on top of it, so the corner is the worst place for it. */}
        <DemoBadge />
      </View>
      {session.profileNotice ? (
        <Text accessibilityLiveRegion="polite" style={[typography.caption, { color: colors.accent }]}>{session.profileNotice}</Text>
      ) : null}
      {session.phien !== null ? (
        // A real session: the server's profile and counts. The fixture hero
        // below is Team Đà Lạt's story and must never be shown to a signed-in
        // person as if it were theirs.
        <HoSoSong phien={session.phien} />
      ) : (
        <View style={styles.hero}>
          <View style={styles.heroDau}>
            <Avatar name={session.displayName} ring size={72} />
            <View style={styles.flex}>
              <Text style={[typography.h1, { color: colors.ink }]}>{session.displayName}</Text>
              <Text style={[typography.body, { color: colors.inkSoft }]}>{session.bio}</Text>
            </View>
          </View>
          {/* The counts as one sentence: a footprint, not a scoreboard. */}
          <Text style={[typography.caption, { color: colors.inkSoft }]}>
            1 chuyến đi · {session.savedPlaceIds.length} đã lưu · {session.photoCount} ảnh
          </Text>
          <RudiButton
            compact
            full={false}
            icon="create-outline"
            label="Chỉnh hồ sơ"
            onPress={() => setPanel("edit")}
            variant="outline"
          />
        </View>
      )}
      {session.phien === null ? (
        // The fixture trip. A real session has no outing yet until M4 reads
        // `/outings`; showing Team Đà Lạt's weekend to a signed-in stranger is
        // the exact lie this tab used to tell.
        <View>
          <SectionHeader title="Sắp tới" />
          <Pressable
            accessibilityLabel={`Mở chuyến ${session.tripName}`}
            accessibilityRole="button"
            onPress={() => router.push(session.tripPath("/timeline") as never)}
            style={({ pressed }) => [styles.upcoming, { backgroundColor: colors.accentSoft, borderRadius: radius.base }, pressed && styles.pressed]}
          >
            <View style={styles.dauLich}>
              <Text style={[styles.ngay, { color: colors.ink }]}>{dau?.ngay ?? "?"}</Text>
              <Text numberOfLines={1} style={[typography.caption, { color: colors.inkSoft }]}>{dau?.thang ?? ""}</Text>
            </View>
            <View style={styles.flex}>
              {nhip ? <Stamp label={nhip} tilt={-2} /> : null}
              <Text style={[typography.h2, { color: colors.ink }]}>{session.tripName}</Text>
              <Text style={[typography.caption, { color: colors.inkSoft }]}>
                {DEMO_GROUP.name} · {nhanKhoangNgay(isoTu(session.startDate), isoTu(session.endDate))}
              </Text>
            </View>
            <Ionicons color={colors.inkFaint} name="chevron-forward" size={18} />
          </Pressable>
        </View>
      ) : null}
      <View>
        {session.phien !== null ? (
          <>
            <View style={[styles.hangMenu, { borderBottomColor: colors.line }]}>
              <ListRow
                icon="people-outline"
                onPress={() => router.push("/friends")}
                subtitle="Bạn bè, lời mời đã nhận và đã gửi"
                title="Bạn bè"
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
          </>
        ) : null}
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
          {session.phien !== null ? (
            <ListRow
              icon="ribbon-outline"
              onPress={() => router.push("/achievements")}
              subtitle="Cấp và huy hiệu tính từ sổ của bạn"
              title="Thành tích"
            />
          ) : (
            <ListRow
              icon="ribbon-outline"
              onPress={() => router.push("/achievements")}
              subtitle={nhanHuyHieuDaMo()}
              title="Thành tích"
            />
          )}
        </View>
        <View style={[styles.hangMenu, { borderBottomColor: colors.line }]}>
          <ListRow
            icon="bookmark-outline"
            onPress={() => setPanel("saved")}
            subtitle={`${session.savedPlaceIds.length} địa điểm ${noiLuuNgan(session.luuTruSong)}`}
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
            subtitle="Quyền riêng tư và đăng xuất bản trải nghiệm"
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
  return <TaiChinhNhap />;
}

/**
 * `GET /people/{id}/finance`, printed.
 *
 * No arithmetic: the route's own docstring says `settled + outstanding == spend`
 * is guaranteed by the ledger query that answers it, and deriving even one of
 * the three here would be a second implementation of the same sum.
 */
function TaiChinhLive({ actorId, contextId }: { actorId: string; contextId: string }) {
  const router = useRouter();
  const { colors, radius } = useRudiTheme();
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
  return (
    <RudiScreen tone="split" testID="finance-screen">
      <TopBar title="Tài chính của tôi" />
      {/* The one answer first, as the first line of a ledger: what this person's share of everything has come to. */}
      <View>
        <DongTien dam nhan="Phần chi của bạn" phu={`${du.expense_count} khoản chi trong ${du.group_count} nhóm. Máy chủ tính lại từ sổ mỗi lần hỏi.`} tone="split" vnd={du.spend_vnd} />
        <DongTien nhan="Còn phải trả" phu={`Đã trả ${formatVnd(du.settled_vnd)}`} tone={du.outstanding_vnd > 0 ? "warn" : "ink"} vnd={du.outstanding_vnd} />
        <DongTien nhan="Sẽ nhận" phu="Bạn đã ứng trước" tone="split" vnd={du.receivable_vnd} />
      </View>
      <SectionHeader
        action="Xem quyết toán"
        onAction={() => router.push(("/settlements/" + contextId) as never)}
        title="Chi theo nhóm"
      />
      <View style={styles.ghiChu}>
        <Ionicons color={colors.split} name="calculator-outline" size={20} />
        <Text style={[typography.caption, styles.flex, { color: colors.inkSoft }]}>
          {tinhTrangNo(du).cau} Số này đọc từ sổ cái, không phải số dư ngân hàng.
        </Text>
      </View>
    </RudiScreen>
  );
}

function TaiChinhNhap() {
  const router = useRouter();
  const { colors, radius } = useRudiTheme();
  const session = useRudiSession();
  const picture = session.money;
  const mine = picture.spent[COLLECTOR_INDEX];
  const budget = DEMO_GROUP.budgetPerPersonVnd;
  const owe = picture.transfers
    .filter((row) => row.fromIndex === COLLECTOR_INDEX)
    .reduce((sum, row) => sum + row.amount, 0);
  const receive = picture.collectorReceives;
  const unpaid = picture.transfers.filter((row) => !session.paidFromIndexes.includes(row.fromIndex)).length;
  const transactions: { icon: IconName; title: string; detail: string; amount: number }[] = [
    {
      icon: "restaurant-outline",
      title: "Tiệm Nướng Xóm Lèo",
      detail: "Phần bạn trong bill, cùng số với Quyết toán",
      amount: -picture.shares[COLLECTOR_INDEX],
    },
    {
      icon: "home-outline",
      title: "Homestay + xăng",
      detail: "Phần bạn trong phần còn lại của chuyến",
      amount: -picture.otherShares[COLLECTOR_INDEX],
    },
    {
      icon: "arrow-down-circle-outline",
      title: `${PEOPLE[COLLECTOR_INDEX].name} sẽ thu (bill)`,
      detail: "Nháp, chưa confirm sổ",
      amount: receive,
    },
  ];

  return (
    <RudiScreen tone="split" testID="finance-screen">
      <TopBar title="Tài chính của tôi" right={<DemoBadge />} />
      {/* A ledger, not a dashboard: every sum is a row, the tone lands on the number only. */}
      <View>
        <DongTien dam nhan="Chi của bạn trong chuyến này" phu={`Cả chuyến ${formatVnd(picture.tripTotal)} · một bill Xóm Lèo ${formatVnd(picture.billTotal)}`} tone="split" vnd={mine} />
        <DongTien
          nhan="Ngân sách vui chơi"
          phu={mine <= budget ? `Còn ${formatVnd(budget - mine)} chưa dùng` : `Vượt ${formatVnd(mine - budget)}`}
          vnd={budget}
        />
        <DongTien nhan="Cần trả (bill)" phu="Bạn là người thu" tone={owe > 0 ? "warn" : "ink"} vnd={owe} />
        <DongTien nhan="Sẽ nhận (bill)" phu={`${String(unpaid)} người chưa trả`} tone="split" vnd={receive} />
      </View>
      <Text style={[typography.caption, { color: colors.inkFaint }]}>Không có dữ liệu tháng khác nên không hiện bộ lọc kỳ.</Text>
      <View>
        <SectionHeader
          action="Xem quyết toán"
          onAction={() => router.push(("/settlements/" + DEMO_GROUP.id) as never)}
          title="Chi theo nhóm"
        />
        <View style={[styles.hangSo, { borderBottomColor: colors.line }]}>
          <Text style={[typography.label, styles.flex, { color: colors.ink }]}>Team Đà Lạt</Text>
          <Money size="label" vnd={picture.tripTotal} />
        </View>
      </View>
      <View>
        <SectionHeader title="Giao dịch gần đây" />
        {transactions.map((transaction) => (
          <View key={transaction.title} style={[styles.hangSo, { borderBottomColor: colors.line }]}>
            <View style={[styles.transactionIcon, { backgroundColor: transaction.amount > 0 ? colors.splitSoft : colors.card, borderColor: colors.line }]}>
              <Ionicons color={transaction.amount > 0 ? colors.split : colors.inkSoft} name={transaction.icon} size={20} />
            </View>
            <View style={styles.flex}>
              <Text style={[typography.label, { color: colors.ink }]}>{transaction.title}</Text>
              <Text style={[typography.caption, { color: colors.inkFaint }]}>{transaction.detail}</Text>
            </View>
            <Money sign="always" size="label" tone={transaction.amount > 0 ? "split" : "ink"} vnd={transaction.amount} />
          </View>
        ))}
      </View>
      <View style={styles.ghiChu}>
        <Ionicons color={colors.split} name="calculator-outline" size={20} />
        <Text style={[typography.caption, styles.flex, { color: colors.inkSoft }]}>
          Số trên màn này và Quyết toán cùng một phép tính nháp. Chưa confirm sổ cái. Đây không phải số dư ngân hàng.
        </Text>
      </View>
    </RudiScreen>
  );
}

/**
 * Badges counted from what the fixture actually holds: one outing, seven
 * companions, six dishes on one bill, four photographs, one province. The
 * first cut declared five of six unlocked («Hoàn thành 10 chuyến» beside a
 * profile that says «1 chuyến đi»), which is exactly the invented number the
 * story forbids. Nothing is unlocked yet; every rule shows how far it has come.
 */
const SU_THAT = { chuyen: 1, ban: PEOPLE.length - 1, mon: BILL_ITEMS.length, anh: MEMORY_PHOTOS.length, ngoaiTroi: 1, tinh: 1 } as const;

const HUY_HIEU: readonly { icon: IconName; ten: string; luat: string; can: number; co: number; donVi: string }[] = [
  { icon: "airplane", ten: "Chân đi", luat: "Hoàn thành 10 chuyến", can: 10, co: SU_THAT.chuyen, donVi: "chuyến" },
  { icon: "people", ten: "Kết nối", luat: "Đi cùng 25 người bạn", can: 25, co: SU_THAT.ban, donVi: "người" },
  { icon: "restaurant", ten: "Foodie", luat: "Thử 20 món local", can: 20, co: SU_THAT.mon, donVi: "món" },
  { icon: "camera", ten: "Ký ức", luat: "Đăng 100 khoảnh khắc", can: 100, co: SU_THAT.anh, donVi: "ảnh" },
  { icon: "leaf", ten: "Xanh", luat: "5 chuyến ngoài trời", can: 5, co: SU_THAT.ngoaiTroi, donVi: "chuyến" },
  { icon: "map", ten: "Nhà thám hiểm", luat: "Ghé 15 tỉnh thành", can: 15, co: SU_THAT.tinh, donVi: "tỉnh" },
];

function huyHieuDaMo() {
  return HUY_HIEU.filter((h) => h.co >= h.can);
}

// Built here, not inline in ProfileScreen: the actor-header gate reads every
// template literal with a slash inside a component that calls the API as a
// URL it cannot resolve, and «0/6» is a count, not a route.
function nhanHuyHieuDaMo(): string {
  return `${huyHieuDaMo().length}/${HUY_HIEU.length} huy hiệu đã mở`;
}

export function AchievementsScreen() {
  const { colors } = useRudiTheme();
  const daMo = huyHieuDaMo();
  // The badge nearest its rule, named with what is still missing; no bar, no percent.
  const ganNhat = [...HUY_HIEU].filter((h) => h.co < h.can).sort((a, b) => b.co / b.can - a.co / a.can)[0];

  return (
    <RudiScreen testID="achievements-screen">
      <TopBar title="Thành tích" right={<DemoBadge />} />
      <SectionHeader action={`${daMo.length}/${HUY_HIEU.length} đã mở`} title="Huy hiệu của bạn" />
      <View>
        {HUY_HIEU.map((h) => {
          const mo = h.co >= h.can;
          return (
            <View key={h.ten} style={[styles.hangSo, { borderBottomColor: colors.line }]}>
              <View style={[styles.transactionIcon, { backgroundColor: mo ? colors.accentSoft : colors.card, borderColor: mo ? colors.accentSoft : colors.line }]}>
                <Ionicons color={mo ? colors.accent : colors.inkFaint} name={mo ? h.icon : "lock-closed-outline"} size={20} />
              </View>
              <View style={styles.flex}>
                <Text style={[typography.label, { color: colors.ink }]}>{h.ten}</Text>
                <Text style={[typography.caption, { color: colors.inkSoft }]}>{h.luat}</Text>
              </View>
              {mo ? <Stamp label="Đã mở" /> : <Text style={[typography.caption, styles.tienDo, { color: colors.inkSoft }]}>{`${h.co}/${h.can} ${h.donVi}`}</Text>}
            </View>
          );
        })}
      </View>
      {ganNhat ? (
        <View style={styles.thuThach}>
          <Text style={[typography.h2, { color: colors.ink }]}>Gần nhất</Text>
          <Text style={[typography.body, { color: colors.inkSoft }]}>
            «{ganNhat.ten}»: còn {ganNhat.can - ganNhat.co} {ganNhat.donVi} nữa. Số này đếm từ những gì bạn đã làm trong Rủ Đi.
          </Text>
        </View>
      ) : null}
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  flex: { flex: 1 },
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
