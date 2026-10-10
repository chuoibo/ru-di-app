import { Ionicons } from "@expo/vector-icons";
import { useFocusEffect, useRouter } from "expo-router";
import { useCallback, useRef, useState } from "react";
import { StyleSheet, Text, View } from "react-native";

import { ApiError, attemptFor, thongDiepNguoiDoc, type Attempt } from "../../../api";
import { ganDanhSachNhom, type Phien } from "../../../phien";
import { docDanhSachBan, type Ban } from "../../../screens/ca-nhan/ban-be";
import { ghepVaoDanhSach, moNhanRieng } from "../../nhan-rieng/nhan-rieng";
import { useRudiSession } from "../../session";
import { CuaDangNhap } from "../../ui/CuaDangNhap";
import { mucNguoi, typography, useRudiTheme } from "../../theme";
import { Heading, IconButton, RudiScreen, TopBar } from "../../ui";
import { HinhNhan } from "../../ui/Avatar";
import { EmptyState } from "../../ui/EmptyState";
import { PressScale } from "../../ui/PressScale";
import { ErrorState } from "../../ui/ErrorState";
import { SkeletonGroup, SkeletonRow } from "../../ui/Skeleton";
import { AssetIllustration } from "../../asset-pack/react-native/AssetIllustration";

/**
 * «Hẹn người thương» (the one new entry in «Tạo mới», spec §20.1): pick
 * the person, land in the pair's paper surface with a sheet already drafted.
 *
 * A real session lists the person's actual friends and opens (or finds) the
 * direct conversation with the one picked, through the same
 * `POST /people/{id}/dm` the friend list's «Nhắn tin» uses. Before 23/09 this
 * screen offered the fixture pair to everybody, so a signed-in person with no
 * friends at all saw «Người ấy — Mở sổ lời hẹn», and pressing on
 * through it did nothing (QA 23/09). Signed out, it is the sign-in door.
 */
export function ChonNguoiScreen() {
  const { phien } = useRudiSession();
  if (phien === null) return <CuaDangNhap tiep="/hai-nguoi/chon-nguoi" />;
  return <ChonNguoiSong phien={phien} />;
}

type Trang = { pha: "dang-doc" } | { pha: "xong"; ban: Ban[] } | { pha: "hong"; loi: string };

function ChonNguoiSong({ phien }: { phien: Phien }) {
  const router = useRouter();
  const { colors, dark } = useRudiTheme();
  const { datPhien } = useRudiSession();
  const [trang, setTrang] = useState<Trang>({ pha: "dang-doc" });
  const [dangMo, setDangMo] = useState<string | null>(null);
  const [loiMo, setLoiMo] = useState<string | null>(null);
  const lock = useRef(false);
  const attempts = useRef<Record<string, Attempt>>({});

  const nap = useCallback(async () => {
    try {
      setTrang({ pha: "xong", ban: await docDanhSachBan(phien.person_id, phien.person_id) });
    } catch (error) {
      setTrang({ pha: "hong", loi: error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null) });
    }
  }, [phien.person_id]);

  useFocusEffect(
    useCallback(() => {
      void nap();
    }, [nap]),
  );

  const ruNguoi = async (ban: Ban) => {
    if (lock.current) return;
    lock.current = true;
    setDangMo(ban.person_id);
    setLoiMo(null);
    try {
      const cap = await moNhanRieng(ban.person_id, phien.person_id, attemptFor(attempts.current, `dm:${ban.person_id}`));
      // The paper route names the other person from `phien.contexts`, so the
      // pair has to be in the session before the push, not after.
      datPhien(await ganDanhSachNhom(phien, ghepVaoDanhSach(phien.contexts, cap)));
      router.replace(`/groups/${cap.id}/to-giay?ru=1` as never);
    } catch (error) {
      setLoiMo(error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null));
    } finally {
      lock.current = false;
      setDangMo(null);
    }
  };

  return (
    <RudiScreen material="journal" cot="form" header={<TopBar back title="Rủ một người" right={<IconButton quiet icon="person-add-outline" accessibilityLabel="Thêm bạn bằng tên tài khoản" onPress={() => router.push("/friends/add")} />} />} testID="chon-nguoi">
      <View style={styles.noiDung}>
        <Heading size="h2" subtitle="Chọn từ những người đã kết bạn." title="Bạn muốn rủ ai?" />
        {trang.pha === "dang-doc" ? (
          <SkeletonGroup>
            <SkeletonRow leading={40} />
            <SkeletonRow leading={40} />
          </SkeletonGroup>
        ) : null}
        {trang.pha === "hong" ? <ErrorState body={trang.loi} onRetry={() => void nap()} title="Chưa đọc được danh sách bạn" /> : null}
        {trang.pha === "xong" && trang.ban.length === 0 ? (
          <EmptyState
            action={{ label: "Thêm bạn bằng tên tài khoản", onPress: () => router.push("/friends/add") }}
            body="Kết bạn trước để tìm thấy nhau ở đây. Sổ lời hẹn chỉ mở khi cả hai đồng ý."
            illustration={<AssetIllustration name="spot-keeps" width={220} height={170} />}
            kind="first-use"
            layout="inline"
            title="Chưa có bạn nào để rủ"
          />
        ) : null}
        {trang.pha === "xong" && trang.ban.length > 0 ? (
          // Friends as people, not rows: each a paper standee in their own ink
          // (ADR-0037 D6) on a card you press to open your notebook together.
          <View style={styles.luoi}>
            {trang.ban.map((ban, index) => (
              <PressScale
                accessibilityHint="Mở chỗ hẹn của hai bạn"
                accessibilityLabel={ban.display_name}
                accessibilityRole="button"
                accessibilityState={{ busy: dangMo === ban.person_id, disabled: dangMo !== null }}
                disabled={dangMo !== null}
                haptic="select"
                key={ban.person_id}
                onPress={() => void ruNguoi(ban)}
                style={[styles.the, { borderTopColor: colors.line, borderTopWidth: index === 0 ? 0 : StyleSheet.hairlineWidth, opacity: dangMo !== null && dangMo !== ban.person_id ? 0.55 : 1 }]}
              >
                <HinhNhan name={ban.display_name} personId={ban.person_id} size={46} />
                <View style={styles.nguoi}>
                  <Text style={[typography.label, { color: mucNguoi(ban.person_id, dark) }]}>{ban.display_name}</Text>
                  {dangMo === ban.person_id ? <Text accessibilityLiveRegion="polite" style={[typography.note, { color: colors.inkSoft }]}>Đang mở sổ của hai bạn…</Text> : null}
                </View>
                <Ionicons name="chevron-forward" size={20} color={colors.inkSoft} />
              </PressScale>
            ))}
          </View>
        ) : null}
        {loiMo !== null ? (
          <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.warn }]}>{loiMo}</Text>
        ) : null}
      </View>
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  noiDung: { gap: 24, paddingTop: 8 },
  luoi: { width: "100%" },
  the: { flexDirection: "row", minHeight: 80, alignItems: "center", gap: 12, paddingVertical: 16 },
  nguoi: { flex: 1, minWidth: 0, gap: 4 },
});
