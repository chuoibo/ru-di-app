/**
 * Bạn bè (M2): the person's own friend graph, three views of one table.
 *
 * `friend_requests` rows are the truth for all three segments -- accepted
 * edges are friends, pending edges I received are «Đã nhận», pending edges I
 * sent are «Đã gửi». The server resolves «the other person» per reader, so no
 * screen branches on direction (and none can get it backwards).
 *
 * Reuses the legacy client module (`ban-be.ts`) as-is: the routes are the ones
 * App B called, with the bearer now doing the identifying.
 */
import { Redirect, useFocusEffect, useRouter } from "expo-router";
import { Canh } from "../../ui/art/Canh";
import { useCallback, useRef, useState, type ReactNode } from "react";
import { StyleSheet, Text, View } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";

import { ApiError, attemptFor, newAttempt, thongDiepNguoiDoc, type Attempt } from "../../../api";
import { ganDanhSachNhom } from "../../../phien";
import {
  docDanhSachBan,
  docLoiMoi,
  traLoiLoiMoi,
  type Ban,
  type LoiMoi,
  type TraLoi,
} from "../../../screens/ca-nhan/ban-be";
import { ghepVaoDanhSach, moNhanRieng } from "../../nhan-rieng/nhan-rieng";
import { useRudiSession } from "../../session";
import { Divider, RudiButton, RudiScreen, Segmented, TopBar } from "../../ui";
import { EmptyState } from "../../ui/EmptyState";
import { ErrorState } from "../../ui/ErrorState";
import { HangNguoi, HangNguoiCho } from "./HangNguoi";

type Du = { ban: Ban[]; daNhan: LoiMoi[]; daGui: LoiMoi[] };
type Trang = { pha: "dang-doc" } | { pha: "xong"; du: Du } | { pha: "hong"; loi: string };

const MUC = ["Đã là bạn", "Đã nhận", "Đã gửi"];

function loiRaChu(error: unknown): string {
  return error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null);
}

function ngayKetBan(iso: string): string {
  return `Bạn từ ${new Date(iso).toLocaleDateString("vi-VN")}`;
}

/** Rows on the paper, a hairline between neighbours. */
function DanhSach({ hang }: { hang: ReactNode[] }) {
  return (
    <View style={styles.danhSach}>
      {hang.map((h, i) => (
        <View key={i}>
          {i > 0 ? (
            <View style={styles.vach}>
              <Divider />
            </View>
          ) : null}
          {h}
        </View>
      ))}
    </View>
  );
}

export function FriendsScreen() {
  const router = useRouter();
  // The pinned footer must clear the gesture bar: the screen shell only pads
  // top/left/right, so the bottom inset is this screen's to add.
  const { bottom: menDuoi } = useSafeAreaInsets();
  const { phien, phienDaDoc, datPhien } = useRudiSession();
  const [muc, setMuc] = useState(0);
  const [trang, setTrang] = useState<Trang>({ pha: "dang-doc" });
  const [dangTraLoi, setDangTraLoi] = useState<string | null>(null);
  // ADR-0021 §2.5: «Nhắn tin» on a friend's row opens (or finds) the pair.
  const [dangNhan, setDangNhan] = useState<string | null>(null);
  const attempts = useRef<Record<string, Attempt>>({});

  const nap = useCallback(async () => {
    if (phien === null) return;
    try {
      const toi = phien.person_id;
      const [ban, daNhan, daGui] = await Promise.all([
        docDanhSachBan(toi, toi),
        docLoiMoi(toi, toi, "incoming"),
        docLoiMoi(toi, toi, "outgoing"),
      ]);
      setTrang({
        pha: "xong",
        du: {
          ban,
          daNhan: daNhan.filter((lm) => lm.state === "pending"),
          daGui: daGui.filter((lm) => lm.state === "pending"),
        },
      });
    } catch (error) {
      setTrang({ pha: "hong", loi: loiRaChu(error) });
    }
  }, [phien]);

  useFocusEffect(
    useCallback(() => {
      void nap();
    }, [nap]),
  );

  if (!phienDaDoc) return null;
  if (phien === null) return <Redirect href="/welcome" />;

  const nhanTin = async (b: Ban) => {
    if (dangNhan !== null) return;
    setDangNhan(b.person_id);
    try {
      const cap = await moNhanRieng(b.person_id, phien.person_id, attemptFor(attempts.current, `dm:${b.person_id}`));
      datPhien(await ganDanhSachNhom(phien, ghepVaoDanhSach(phien.contexts, cap)));
      router.push(`/groups/${cap.id}/chat` as never);
    } catch (error) {
      setTrang({ pha: "hong", loi: loiRaChu(error) });
    } finally {
      setDangNhan(null);
    }
  };

  const traLoi = async (lm: LoiMoi, quyetDinh: TraLoi) => {
    setDangTraLoi(lm.id);
    try {
      await traLoiLoiMoi(lm.id, quyetDinh, phien.person_id, newAttempt());
      await nap();
    } catch (error) {
      setTrang({ pha: "hong", loi: loiRaChu(error) });
    } finally {
      setDangTraLoi(null);
    }
  };

  return (
    <RudiScreen
      footer={
        <RudiButton
          icon="person-add-outline"
          label="Thêm bạn bằng số điện thoại"
          onPress={() => router.push("/friends/add")}
        />
      }
      footerInset={14 + menDuoi}
      testID="friends-screen"
    >
      <TopBar title="Bạn bè" />
      <Segmented items={MUC} onSelect={setMuc} selected={muc} />
      {trang.pha === "dang-doc" ? <HangNguoiCho /> : null}
      {trang.pha === "hong" ? <ErrorState body={trang.loi} onRetry={() => void nap()} title="Chưa đọc được danh sách bạn" /> : null}
      {trang.pha === "xong" && muc === 0 ? (
        trang.du.ban.length === 0 ? (
          <EmptyState body="Thêm bạn bằng số điện thoại. Người ấy đồng ý thì hai bên là bạn." kind="first-use" layout="inline" illustration={<Canh id="chua-co-ban" width={168} />} title="Chưa có bạn nào" />
        ) : (
          <DanhSach
            hang={trang.du.ban.map((b) => (
              <HangNguoi
                duoi={
                  <RudiButton
                    accessibilityLabel={`Nhắn tin cho ${b.display_name}`}
                    compact
                    disabled={dangNhan !== null}
                    full={false}
                    icon="chatbubble-outline"
                    label="Nhắn tin"
                    loading={dangNhan === b.person_id}
                    onPress={() => void nhanTin(b)}
                    variant="soft"
                  />
                }
                key={b.person_id}
                onPress={() => router.push(`/people/${b.person_id}`)}
                phu={ngayKetBan(b.friends_since)}
                ten={b.display_name}
              />
            ))}
          />
        )
      ) : null}
      {trang.pha === "xong" && muc === 1 ? (
        trang.du.daNhan.length === 0 ? (
          <EmptyState body="Khi ai đó gửi lời mời kết bạn, nó hiện ở đây." kind="first-use" layout="inline" illustration={<Canh id="chua-co-loi-moi" width={150} />} title="Không có lời mời nào đang chờ" />
        ) : (
          <DanhSach
            hang={trang.du.daNhan.map((lm) => (
              <HangNguoi
                key={lm.id}
                duoi={
                  <>
                    <RudiButton
                      compact
                      disabled={dangTraLoi !== null}
                      full={false}
                      label="Đồng ý"
                      loading={dangTraLoi === lm.id}
                      onPress={() => void traLoi(lm, "accept")}
                    />
                    <RudiButton
                      compact
                      disabled={dangTraLoi !== null}
                      full={false}
                      label="Từ chối"
                      onPress={() => void traLoi(lm, "decline")}
                      variant="ghost"
                    />
                  </>
                }
                phu="Muốn kết bạn với bạn"
                ten={lm.other_display_name}
              />
            ))}
          />
        )
      ) : null}
      {trang.pha === "xong" && muc === 2 ? (
        trang.du.daGui.length === 0 ? (
          <EmptyState body="Lời mời bạn gửi và đang chờ trả lời hiện ở đây." kind="first-use" layout="inline" illustration={<Canh id="chua-co-loi-moi" width={150} />} title="Bạn chưa gửi lời mời nào" />
        ) : (
          <DanhSach
            hang={trang.du.daGui.map((lm) => (
              <HangNguoi key={lm.id} phu="Đang chờ người ấy trả lời" ten={lm.other_display_name} />
            ))}
          />
        )
      ) : null}
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  danhSach: { paddingVertical: 2 },
  // Hairline starts at the text column (avatar 40 + gap 12).
  vach: { marginLeft: 52 },
});
