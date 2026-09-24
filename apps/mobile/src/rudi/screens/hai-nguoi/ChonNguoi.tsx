import { useFocusEffect, useRouter } from "expo-router";
import { useCallback, useRef, useState } from "react";
import { Text, View } from "react-native";

import { ApiError, attemptFor, thongDiepNguoiDoc, type Attempt } from "../../../api";
import { ganDanhSachNhom, type Phien } from "../../../phien";
import { docDanhSachBan, type Ban } from "../../../screens/ca-nhan/ban-be";
import { ghepVaoDanhSach, moNhanRieng } from "../../nhan-rieng/nhan-rieng";
import { useRudiSession } from "../../session";
import { typography, useRudiTheme } from "../../theme";
import { CAP_DEMO, NGUOI_KIA_DEMO } from "../../to-giay/fixtures-doi";
import { Heading, ListRow, NhomHang, RudiButton, RudiScreen, TopBar } from "../../ui";
import { EmptyState } from "../../ui/EmptyState";
import { ErrorState } from "../../ui/ErrorState";
import { SkeletonGroup, SkeletonRow } from "../../ui/Skeleton";

/**
 * «Rủ một người đi chơi» (the one new entry in «Tạo mới», spec §20.1): pick
 * the person, land in the pair's paper surface with a sheet already drafted.
 *
 * A real session lists the person's actual friends and opens (or finds) the
 * direct conversation with the one picked, through the same
 * `POST /people/{id}/dm` the friend list's «Nhắn tin» uses. Before 23/09 this
 * screen offered the fixture pair to everybody, so a signed-in person with no
 * friends at all saw «Người ấy — Sổ hai người đang mở», and pressing on
 * through it did nothing (QA 23/09). The fixture row stays for the experience
 * build only.
 */
export function ChonNguoiScreen() {
  const { phien } = useRudiSession();
  if (phien !== null) return <ChonNguoiSong phien={phien} />;
  return <ChonNguoiTraiNghiem />;
}

type Trang = { pha: "dang-doc" } | { pha: "xong"; ban: Ban[] } | { pha: "hong"; loi: string };

function ChonNguoiSong({ phien }: { phien: Phien }) {
  const router = useRouter();
  const { colors } = useRudiTheme();
  const { datPhien } = useRudiSession();
  const [trang, setTrang] = useState<Trang>({ pha: "dang-doc" });
  const [dangMo, setDangMo] = useState<string | null>(null);
  const [loiMo, setLoiMo] = useState<string | null>(null);
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
    if (dangMo !== null) return;
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
      setDangMo(null);
    }
  };

  return (
    <RudiScreen header={<TopBar back title="Rủ một người đi chơi" subtitle="Nếp phác sẵn, bạn gửi" />} testID="chon-nguoi">
      <View style={{ gap: 10, paddingTop: 8 }}>
        <Heading size="h2" subtitle="Tờ giấy đi vào sổ hai người của hai bạn, không vào hội." title="Rủ ai?" />
        {trang.pha === "dang-doc" ? (
          <SkeletonGroup>
            <SkeletonRow leading={40} />
            <SkeletonRow leading={40} />
          </SkeletonGroup>
        ) : null}
        {trang.pha === "hong" ? <ErrorState body={trang.loi} onRetry={() => void nap()} title="Chưa đọc được danh sách bạn" /> : null}
        {trang.pha === "xong" && trang.ban.length === 0 ? (
          <EmptyState
            action={{ label: "Thêm bạn bằng số điện thoại", onPress: () => router.push("/friends/add") }}
            body="Sổ hai người mở giữa hai người đã là bạn. Kết bạn trước, rồi quay lại rủ."
            kind="first-use"
            layout="inline"
            title="Chưa có bạn nào để rủ"
          />
        ) : null}
        {trang.pha === "xong" && trang.ban.length > 0 ? (
          <NhomHang>
            {trang.ban.map((ban) => (
              <ListRow
                key={ban.person_id}
                icon="person-outline"
                onPress={dangMo === null ? () => void ruNguoi(ban) : undefined}
                subtitle={dangMo === ban.person_id ? "Đang mở sổ của hai bạn…" : "Mở tờ giấy của hai bạn"}
                title={ban.display_name}
              />
            ))}
          </NhomHang>
        ) : null}
        {loiMo !== null ? (
          <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.warn }]}>{loiMo}</Text>
        ) : null}
        {trang.pha === "xong" && trang.ban.length > 0 ? (
          <RudiButton icon="person-add-outline" label="Thêm bạn khác" onPress={() => router.push("/friends/add")} variant="ghost" />
        ) : null}
      </View>
    </RudiScreen>
  );
}

/** The experience build: one person to pick, the fixture pair. */
function ChonNguoiTraiNghiem() {
  const router = useRouter();
  return (
    <RudiScreen header={<TopBar back title="Rủ một người đi chơi" subtitle="Nếp phác sẵn, bạn gửi" />} testID="chon-nguoi">
      <View style={{ gap: 10, paddingTop: 8 }}>
        <Heading size="h2" subtitle="Tờ giấy đi vào sổ hai người của hai bạn, không vào hội." title="Rủ ai?" />
        <NhomHang>
          <ListRow icon="person-outline" onPress={() => router.replace(`/groups/${CAP_DEMO.id}/to-giay?ru=1` as never)} subtitle="Sổ hai người đang mở" title={NGUOI_KIA_DEMO.ten} />
        </NhomHang>
      </View>
    </RudiScreen>
  );
}
