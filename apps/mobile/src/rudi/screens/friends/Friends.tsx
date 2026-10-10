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
import { useFocusEffect, useLocalSearchParams, useRouter } from "expo-router";
import { PaperObject } from "../../asset-pack/react-native/PaperObject";
import { useCallback, useEffect, useRef, useState, type ReactElement } from "react";
import { View } from "react-native";

import { ApiError, attemptFor, thongDiepNguoiDoc, type Attempt } from "../../../api";
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
import { IconButton, RudiButton, RudiScreen, Segmented, TopBar } from "../../ui";
import { CauTaiCho } from "../../ui/CauTaiCho";
import { EmptyState } from "../../ui/EmptyState";
import { ErrorState } from "../../ui/ErrorState";
import { HangNguoi, HangNguoiCho } from "./HangNguoi";
import { LuoiNguoi } from "../../ui/LuoiNguoi";
import { CuaDangNhap } from "../../ui/CuaDangNhap";
import { ngayVN } from "../../ngay-viet";

type Du = { ban: Ban[]; daNhan: LoiMoi[]; daGui: LoiMoi[] };
type Trang = { pha: "dang-doc" } | { pha: "xong"; du: Du } | { pha: "hong"; loi: string };

const MUC = ["Đã là bạn", "Đã nhận", "Đã gửi"];

function loiRaChu(error: unknown): string {
  return error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null);
}

function ngayKetBan(iso: string): string {
  return `Bạn từ ${ngayVN(iso)}`;
}

/** Rows on the paper, a hairline between neighbours; two columns where they fit (QA UI-081). */
function DanhSach({ hang }: { hang: ReactElement[] }) {
  return <LuoiNguoi hang={hang.map((h, i) => ({ key: String(h.key ?? i), node: h }))} />;
}

export function FriendsScreen() {
  const router = useRouter();
  const { phien, phienDaDoc, datPhien } = useRudiSession();
  // Cá nhân links here with `?muc=da-nhan` when requests are waiting, so the
  // person lands on them instead of on an empty «Đã là bạn» tab (QA 23/09).
  const { muc: mucMo } = useLocalSearchParams<{ muc?: string }>();
  const [muc, setMuc] = useState(mucMo === "da-nhan" ? 1 : mucMo === "da-gui" ? 2 : 0);
  useEffect(() => { setMuc(mucMo === "da-nhan" ? 1 : mucMo === "da-gui" ? 2 : 0); }, [mucMo]);
  const [trang, setTrang] = useState<Trang>({ pha: "dang-doc" });
  const [dangTraLoi, setDangTraLoi] = useState<string | null>(null);
  // ADR-0021 §2.5: «Nhắn tin» on a friend's row opens (or finds) the pair.
  const [dangNhan, setDangNhan] = useState<string | null>(null);
  const actionLock = useRef(false);
  const [readError, setReadError] = useState<string | null>(null);
  const attempts = useRef<Record<string, Attempt>>({});
  // QA UI-077: a refused answer or «Nhắn tin» is said under that row; the
  // list that was read stays, and the row can be pressed again.
  const [loiHang, setLoiHang] = useState<{ id: string; cau: string } | null>(null);

  const nap = useCallback(async () => {
    if (phien === null) return;
    try {
      const toi = phien.person_id;
      const [ban, daNhan, daGui] = await Promise.all([
        docDanhSachBan(toi, toi),
        docLoiMoi(toi, toi, "incoming"),
        docLoiMoi(toi, toi, "outgoing"),
      ]);
      setReadError(null);
      setTrang({
        pha: "xong",
        du: {
          ban,
          daNhan: daNhan.filter((lm) => lm.state === "pending"),
          daGui: daGui.filter((lm) => lm.state === "pending"),
        },
      });
    } catch (error) {
      setReadError(loiRaChu(error));
      setTrang((held) => held.pha === "xong" ? held : { pha: "hong", loi: loiRaChu(error) });
    }
  }, [phien]);

  useFocusEffect(
    useCallback(() => {
      void nap();
    }, [nap]),
  );

  if (!phienDaDoc) return null;
  if (phien === null) return <CuaDangNhap />;

  const nhanTin = async (b: Ban) => {
    if (actionLock.current) return;
    actionLock.current = true;
    setDangNhan(b.person_id);
    setLoiHang(null);
    try {
      const cap = await moNhanRieng(b.person_id, phien.person_id, attemptFor(attempts.current, `dm:${b.person_id}`));
      datPhien(await ganDanhSachNhom(phien, ghepVaoDanhSach(phien.contexts, cap)));
      router.push(`/groups/${cap.id}/chat` as never);
    } catch (error) {
      setLoiHang({ id: b.person_id, cau: loiRaChu(error) });
    } finally {
      actionLock.current = false;
      setDangNhan(null);
    }
  };

  const traLoi = async (lm: LoiMoi, quyetDinh: TraLoi) => {
    if (actionLock.current) return;
    actionLock.current = true;
    setDangTraLoi(lm.id);
    setLoiHang(null);
    try {
      await traLoiLoiMoi(lm.id, quyetDinh, phien.person_id, attemptFor(attempts.current, `answer:${lm.id}:${quyetDinh}`));
      await nap();
    } catch (error) {
      setLoiHang({ id: lm.id, cau: loiRaChu(error) });
    } finally {
      actionLock.current = false;
      setDangTraLoi(null);
    }
  };

  return (
    <RudiScreen
      material="quiet"
      header={<TopBar title="Bạn bè" right={<IconButton quiet icon="person-add-outline" accessibilityLabel="Thêm bạn bằng tên tài khoản" onPress={() => router.push("/friends/add")} />} />}
      bottomInset={16}
      // The reading column on a tablet keeps «Nhắn tin» beside the name (QA UI-081).
      cot="doc"
      testID="friends-screen"
    >
      <Segmented items={MUC} onSelect={setMuc} selected={muc} />
      {readError && trang.pha === "xong" ? <CauTaiCho cau={`Chưa cập nhật được danh sách. ${readError}`} hanhDong={{ label: "Đọc lại", onPress: () => void nap() }} /> : null}
      {trang.pha === "dang-doc" ? <HangNguoiCho /> : null}
      {trang.pha === "hong" ? <ErrorState body={trang.loi} onRetry={() => void nap()} title="Chưa đọc được danh sách bạn" /> : null}
      {trang.pha === "xong" && muc === 0 ? (
        trang.du.ban.length === 0 ? (
          <EmptyState body="Thêm bạn bằng tên tài khoản. Người ấy đồng ý thì hai bên là bạn." kind="first-use" layout="inline" illustration={<PaperObject name="photo-stack" width={112} />} title="Chưa có bạn nào" />
        ) : (
          <DanhSach
            hang={trang.du.ban.map((b) => (
              <View key={b.person_id}>
              <HangNguoi
                inlineAction
                duoi={
                  <IconButton
                    accessibilityLabel={`Nhắn tin cho ${b.display_name}`}
                    disabled={dangNhan !== null}
                    icon="chatbubble-outline"
                    quiet
                    loading={dangNhan === b.person_id}
                    onPress={() => void nhanTin(b)}
                  />
                }
                onPress={() => router.push(`/people/${b.person_id}`)}
                personId={b.person_id}
                phu={ngayKetBan(b.friends_since)}
                ten={b.display_name}
              />
              {loiHang?.id === b.person_id ? <CauTaiCho cau={loiHang.cau} co="nho" hanhDong={{ label: "Thử lại", onPress: () => void nhanTin(b) }} /> : null}
              </View>
            ))}
          />
        )
      ) : null}
      {trang.pha === "xong" && muc === 1 ? (
        trang.du.daNhan.length === 0 ? (
          <EmptyState body="Khi ai đó gửi lời mời kết bạn, nó hiện ở đây." kind="first-use" layout="inline" illustration={<PaperObject name="envelope" width={112} />} title="Không có lời mời nào đang chờ" />
        ) : (
          <DanhSach
            hang={trang.du.daNhan.map((lm) => (
              <View key={lm.id}>
              <HangNguoi
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
              {loiHang?.id === lm.id ? <CauTaiCho cau={loiHang.cau} co="nho" /> : null}
              </View>
            ))}
          />
        )
      ) : null}
      {trang.pha === "xong" && muc === 2 ? (
        trang.du.daGui.length === 0 ? (
          <EmptyState body="Lời mời bạn gửi và đang chờ trả lời hiện ở đây." kind="first-use" layout="inline" illustration={<PaperObject name="envelope" width={112} />} title="Bạn chưa gửi lời mời nào" />
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
