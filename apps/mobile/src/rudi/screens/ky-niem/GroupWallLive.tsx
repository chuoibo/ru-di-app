/**
 * Tường nhóm on a real session (M6): the group's memories as the server
 * lists them, newest first, paged by its cursor. Hearts and comments are one
 * request each and the counts drawn are the server's; a check-in is a memory
 * without a photo, pinned to a catalogue place; a photo goes through «Thả
 * khoảnh khắc». Only members see any of it, which the subtitle says.
 *
 * UI v2 (đợt 7): the story comes first. Two compact actions under the title,
 * then the posts as pages on the paper -- who, when, the picture at its
 * ratio, the sentence, the two counts, two text actions -- separated by a
 * hairline, not boxed. Check-in opens as a sheet over the wall so the wall
 * does not scroll away under a form.
 */
import { Ionicons } from "@expo/vector-icons";
import { CauTaiCho } from "../../ui/CauTaiCho";
import { KhungAnh } from "../../ui/KhungAnh";
import { useFocusEffect, useRouter } from "expo-router";
import { useCallback, useEffect, useRef, useState } from "react";
import { Keyboard, Pressable, StyleSheet, Text, View } from "react-native";

import { ApiError, attemptFor, thongDiepNguoiDoc, type Attempt } from "../../../api";
import type { Phien } from "../../../phien";
import { danhSachThanhVien } from "../../../screens/vao-cua/cong-api";
import { tenCua, type ThanhVien } from "../../chia-bill/hoa-don";
import { docDanhMucCoLui } from "../../kham-pha/dia-diem";
import { docDiemDenDaChon } from "../../kham-pha/diem-den";
import { tiLeKhung } from "../../ky-niem/ti-le";
import { laPair } from "../../nhan-rieng/nhan-rieng";
import {
  cauKyNiem,
  cauTuongTac,
  checkInKyNiem,
  docBinhLuanCua,
  docTuongNhom,
  doiTim,
  guiBinhLuanCho,
  nguonAnh,
  type BinhLuan,
  type KyNiem,
  nghiengAnh,
} from "../../ky-niem/ky-niem";
import { mucNguoi, typography, useRudiTheme } from "../../theme";
import { Washi } from "../../ui/Washi";
import { Field, Inline, RudiButton, RudiScreen, SearchField, TopBar } from "../../ui";
import { MemoryPhoto } from "../../ui/MemoryPhoto";
import { PhotoViewer, type ViewerPhoto } from "../../ui/PhotoViewer";
import { Canh } from "../../ui/art/Canh";
import { AvatarNguoi } from "../../ui/AvatarNguoi";
import { EmptyState } from "../../ui/EmptyState";
import { ErrorState } from "../../ui/ErrorState";
import { Sheet } from "../../ui/Sheet";
import { SkeletonCard, SkeletonGroup } from "../../ui/Skeleton";
import { gioNgayVN } from "../../ngay-viet";

type Trang =
  | { pha: "dang-doc" }
  | { pha: "xong"; kyNiem: KyNiem[]; conTro: string | null; conNua: boolean }
  | { pha: "hong"; loi: string };

type Cho = { id: string; name: string };

function loiRaChu(error: unknown): string {
  return error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null);
}

function tenHienThi(ten: string | null | undefined): string {
  if (typeof ten === "string" && ten.trim() !== "") return ten;
  return "Thành viên";
}

function gioViet(iso: string): string {
  return gioNgayVN(iso);
}

export function GroupWallLiveScreen({ phien, contextId }: { phien: Phien; contextId: string }) {
  const router = useRouter();
  const { colors, dark, radius } = useRudiTheme();
  const me = phien.person_id;
  const [trang, setTrang] = useState<Trang>({ pha: "dang-doc" });
  const [roster, setRoster] = useState<ThanhVien[]>([]);
  const [thongBao, setThongBao] = useState<string | null>(null);
  // QA UI-095: a heart or a comment that failed is said under that memory,
  // where the press was; it used to land at the top of the wall, off-screen.
  const [loiKyNiem, setLoiKyNiem] = useState<{ id: string; cau: string; thu: () => void } | null>(null);
  // Each memory's photo ratio, learned from the image as it loads (the wire has no size).
  const [tiLe, setTiLe] = useState<Record<string, number>>({});
  const [ban, setBan] = useState(false);
  const [moBinhLuan, setMoBinhLuan] = useState<string | null>(null);
  const [binhLuan, setBinhLuan] = useState<Record<string, BinhLuan[]>>({});
  const [nhap, setNhap] = useState("");
  const nhapTheoBai = useRef<Record<string, string>>({});
  const [loiDocBinhLuan, setLoiDocBinhLuan] = useState<string | null>(null);
  const [viewer, setViewer] = useState<ViewerPhoto | null>(null);
  const [moCheckIn, setMoCheckIn] = useState(false);
  const [danhMuc, setDanhMuc] = useState<Cho[] | null>(null);
  const [dangDocDanhMuc, setDangDocDanhMuc] = useState(false);
  const [loiDanhMuc, setLoiDanhMuc] = useState<string | null>(null);
  const [timCho, setTimCho] = useState("");
  const [choChon, setChoChon] = useState<Cho | null>(null);
  const [cauCheckIn, setCauCheckIn] = useState("");
  const attempts = useRef<Record<string, Attempt>>({});
  const busy = useRef(false);
  const nhom = phien.contexts?.find((n) => n.id === contextId);
  // A pair's wall is the two of them keeping their evenings, not a group's
  // board: it is named for them and says who sees it.
  const laDoi = laPair(nhom);
  const tenNhom = laDoi ? `Bạn và ${nhom?.display_name || "người ấy"}` : nhom?.display_name ?? "Nhóm";

  const docTrangDau = useCallback(async () => {
    const t = await docTuongNhom(contextId, me);
    setTrang({ pha: "xong", kyNiem: t.kyNiem, conTro: t.conTro, conNua: t.conNua });
  }, [contextId, me]);

  useFocusEffect(
    useCallback(() => {
      let song = true;
      docTrangDau().catch((error: unknown) => {
        if (song) setTrang({ pha: "hong", loi: loiRaChu(error) });
      });
      return () => {
        song = false;
      };
    }, [docTrangDau]),
  );

  useEffect(() => {
    let song = true;
    void danhSachThanhVien(contextId, me)
      .then((ds) => {
        if (song) setRoster(ds.map((tv) => ({ id: tv.person_id, name: tenHienThi(tv.display_name) })));
      })
      .catch(() => undefined);
    return () => {
      song = false;
    };
  }, [contextId, me]);

  const chay = async (viec: () => Promise<void>, tai?: { id: string; thu: () => void }) => {
    if (busy.current) return;
    busy.current = true;
    setBan(true);
    setThongBao(null);
    setLoiKyNiem(null);
    try {
      await viec();
    } catch (error) {
      if (tai) setLoiKyNiem({ id: tai.id, cau: loiRaChu(error), thu: tai.thu });
      else setThongBao(loiRaChu(error));
    } finally {
      setBan(false);
      busy.current = false;
    }
  };

  const thayKyNiem = (moi: KyNiem) =>
    setTrang((t) => (t.pha === "xong" ? { ...t, kyNiem: t.kyNiem.map((k) => (k.id === moi.id ? moi : k)) } : t));

  const tim = (k: KyNiem): Promise<void> => chay(async () => thayKyNiem(await doiTim(k, contextId, me)), { id: k.id, thu: () => void tim(k) });

  const moHoacDongBinhLuan = (k: KyNiem): Promise<void> =>
    chay(async () => {
      if (moBinhLuan === k.id) {
        setMoBinhLuan(null);
        return;
      }
      setMoBinhLuan(k.id);
      setNhap(nhapTheoBai.current[k.id] ?? "");
      setLoiDocBinhLuan(null);
      if (binhLuan[k.id] === undefined) {
        try {
          const ds = await docBinhLuanCua(contextId, k.id, me);
          setBinhLuan((b) => ({ ...b, [k.id]: ds }));
        } catch (error) { setLoiDocBinhLuan(loiRaChu(error)); }
      }
    }, { id: k.id, thu: () => void docLaiBinhLuan(k) });

  const docLaiBinhLuan = (k: KyNiem): Promise<void> =>
    chay(async () => {
      setLoiDocBinhLuan(null);
      try {
        const ds = await docBinhLuanCua(contextId, k.id, me);
        setBinhLuan((b) => ({ ...b, [k.id]: ds }));
      } catch (error) { setLoiDocBinhLuan(loiRaChu(error)); }
    }, { id: k.id, thu: () => void docLaiBinhLuan(k) });

  const guiBinhLuan = (k: KyNiem): Promise<void> =>
    chay(async () => {
      if (nhap.trim() === "") return;
      Keyboard.dismiss();
      const bl = await guiBinhLuanCho(contextId, k.id, nhap, me, attempts.current);
      setBinhLuan((b) => ({ ...b, [k.id]: [...(b[k.id] === undefined ? [] : b[k.id]), bl] }));
      thayKyNiem({ ...k, commentCount: k.commentCount + 1 });
      setNhap("");
      delete nhapTheoBai.current[k.id];
    }, { id: k.id, thu: () => void guiBinhLuan(k) });

  const taiThem = () =>
    chay(async () => {
      if (trang.pha !== "xong" || trang.conTro === null) return;
      const t = await docTuongNhom(contextId, me, { before: trang.conTro });
      setTrang({ pha: "xong", kyNiem: [...trang.kyNiem, ...t.kyNiem], conTro: t.conTro, conNua: t.conNua });
    });

  const docDanhMuc = async () => {
    setDangDocDanhMuc(true);
    setLoiDanhMuc(null);
    try {
        // The destination picked on Khám phá; the default is a curated town
        // where nobody in the group is checking in.
        const dm = await docDanhMucCoLui(await docDiemDenDaChon());
        setDanhMuc(dm.places.map((p) => ({ id: p.id, name: p.name })));
    } catch (error) { setLoiDanhMuc(loiRaChu(error)); }
    finally { setDangDocDanhMuc(false); }
  };
  const moCheckInForm = () => {
    setThongBao(null);
    setMoCheckIn(true);
    if (danhMuc === null && !dangDocDanhMuc) void docDanhMuc();
  };

  const dangCheckIn = () =>
    chay(async () => {
      if (choChon === null) return;
      Keyboard.dismiss();
      const k = await checkInKyNiem(contextId, choChon.id, cauCheckIn, me, attemptFor(attempts.current, `check-in:${choChon.id}:${cauCheckIn.trim()}`));
      setTrang((t) => (t.pha === "xong" ? { ...t, kyNiem: [k, ...t.kyNiem] } : t));
      setMoCheckIn(false);
      setChoChon(null);
      setCauCheckIn("");
    });

  // At most a dozen chips: a hundred is a haystack. Diacritic-insensitive, so
  // «di be» finds «Dì Bé» the way the Explore filter does.
  const choHienRa = (danhMuc ?? [])
    .filter((cho) => {
      const q = timCho.trim();
      if (q === "") return true;
      const gap = (x: string) =>
        x.normalize("NFD").replace(/[̀-ͯ]/g, "").replace(/đ/gi, "d").toLowerCase();
      return gap(cho.name).includes(gap(q));
    })
    .slice(0, 12);

  const khayCheckIn = (
    <Sheet accessibilityLabel="Check-in ở đâu?" avoidKeyboard prioritizeFooter showsVerticalScrollIndicator onClose={() => { Keyboard.dismiss(); setMoCheckIn(false); }} open={moCheckIn} footer={
      choChon !== null ? <View style={[styles.editor, { borderTopColor: colors.line }]}>
        <Field accessibilityLabel="Ô câu check-in" onChangeText={setCauCheckIn} placeholder="Một câu về nơi này (tuỳ bạn)" value={cauCheckIn} />
        {thongBao !== null ? <Text accessibilityLiveRegion="polite" style={[typography.caption, { color: colors.warn }]}>{thongBao}</Text> : null}
        <RudiButton disabled={ban} icon="checkmark" label={thongBao !== null ? "Thử đăng lại" : "Đăng check-in"} loading={ban} onPress={() => void dangCheckIn()} />
      </View> : undefined
    }>
      <View style={styles.form}>
        <Text style={[typography.h2, { color: colors.ink }]}>Check-in ở đâu?</Text>
        {dangDocDanhMuc ? <Text style={[typography.caption, { color: colors.inkFaint }]}>Đang đọc danh mục…</Text> : null}
        {loiDanhMuc !== null ? <CauTaiCho cau={loiDanhMuc} co="nho" hanhDong={{ label: "Tải lại danh mục", onPress: () => void docDanhMuc() }} /> : null}
        {/* A destination holds a hundred places since the catalogue became
            real (M9), so this stopped being a chip row and became a search:
            a hundred chips is a haystack, not a choice. The box narrows;
            what stays is the first dozen matches. */}
        {choChon !== null ? <View style={styles.form}>
          <Text style={[typography.label, { color: colors.ink }]}>{choChon.name}</Text>
          <RudiButton compact full={false} label="Chọn chỗ khác" onPress={() => setChoChon(null)} variant="ghost" />
        </View> : danhMuc === null ? null : (
          <>
          <SearchField
            accessibilityLabel="Ô tìm chỗ check-in"
            onChangeText={setTimCho}
            placeholder="Tìm chỗ bạn đang ở"
            value={timCho}
          />
        <View style={styles.form}>
          {choHienRa.map((cho) => (
            <Pressable accessibilityLabel={`Chọn ${cho.name}`} accessibilityRole="button" key={cho.id} onPress={() => { Keyboard.dismiss(); setChoChon(cho); }} style={({ pressed }) => [styles.cho, { borderBottomColor: colors.line }, pressed && styles.pressed]}>
              <Ionicons color={colors.inkSoft} name="location-outline" size={20} />
              <Text style={[typography.body, styles.flex, { color: colors.ink }]}>{cho.name}</Text>
              <Ionicons color={colors.inkSoft} name="chevron-forward" size={18} />
            </Pressable>
          ))}
        </View>
        {danhMuc !== null && choHienRa.length === 0 ? (
          <Text style={[typography.caption, { color: colors.inkFaint }]}>
            Không có chỗ nào khớp «{timCho}». Thử tên ngắn hơn nhé.
          </Text>
        ) : null}
        </>)}
      </View>
    </Sheet>
  );

  const baiBinhLuan = trang.pha === "xong" ? trang.kyNiem.find((k) => k.id === moBinhLuan) : undefined;
  const khayBinhLuan = baiBinhLuan ? <Sheet accessibilityLabel="Bình luận khoảnh khắc" avoidKeyboard prioritizeFooter showsVerticalScrollIndicator open={moBinhLuan !== null} onClose={() => { Keyboard.dismiss(); setMoBinhLuan(null); }} footer={
    <View style={[styles.editor, { borderTopColor: colors.line }]}>
      <Field accessibilityLabel="Ô viết bình luận" onChangeText={(text) => { setNhap(text); nhapTheoBai.current[baiBinhLuan.id] = text; }} placeholder="Viết bình luận…" value={nhap} />
      {loiKyNiem?.id === baiBinhLuan.id ? <CauTaiCho cau={loiKyNiem.cau} co="nho" hanhDong={{ label: "Thử lại", onPress: loiKyNiem.thu }} /> : null}
      {/* ADR-0038: reveal send only when there are words or a pending request. */}
      {nhap.trim() !== "" || ban ? (
        <RudiButton disabled={ban} label="Gửi bình luận" loading={ban} onPress={() => void guiBinhLuan(baiBinhLuan)} />
      ) : null}
    </View>
  }>
    <View style={styles.form}>
      <Text style={[typography.h2, { color: colors.ink }]}>Bình luận</Text>
      <Text numberOfLines={2} style={[typography.caption, { color: colors.inkSoft }]}>{cauKyNiem(baiBinhLuan)}</Text>
      {loiDocBinhLuan !== null ? <CauTaiCho cau={loiDocBinhLuan} co="nho" hanhDong={{ label: "Đọc lại bình luận", onPress: () => void docLaiBinhLuan(baiBinhLuan) }} /> : binhLuan[baiBinhLuan.id] === undefined ? <Text style={[typography.caption, { color: colors.inkSoft }]}>Đang đọc bình luận…</Text> : null}
      {binhLuan[baiBinhLuan.id]?.length === 0 ? <Text style={[typography.body, { color: colors.inkSoft }]}>Chưa có bình luận. Viết câu đầu tiên.</Text> : null}
      {(binhLuan[baiBinhLuan.id] ?? []).map((bl) => <View key={bl.id} style={styles.form}>
        <Text style={[typography.label, { color: colors.ink }]}>{bl.tenTacGia}</Text>
        <Text style={[typography.body, { color: colors.ink }]}>{bl.noiDung}</Text>
      </View>)}
    </View>
  </Sheet> : null;

  return (
    <RudiScreen material="quiet" cot="doc" overlay={<>{khayCheckIn}{khayBinhLuan}</>} testID="group-wall-screen">
      {viewer ? <PhotoViewer photos={[viewer]} initialIndex={0} onClose={() => setViewer(null)} /> : null}
      <TopBar title={laDoi ? "Kỷ niệm của hai bạn" : "Tường nhóm"} />
      <Text style={[typography.h1, { color: colors.ink }]}>{tenNhom}</Text>
      <Text style={[typography.caption, { color: colors.inkSoft }]}>{laDoi ? "Chỉ hai bạn thấy" : "Chỉ thành viên nhóm thấy"}</Text>
      {thongBao !== null && !moCheckIn ? <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.warn }]}>{thongBao}</Text> : null}
      {trang.pha !== "xong" || trang.kyNiem.length > 0 ? <Inline gap={8} wrap>
        <RudiButton compact full={false} icon="camera-outline" label="Thả khoảnh khắc" onPress={() => router.push(`/moments/new?ctx=${contextId}` as never)} />
        <RudiButton compact disabled={ban} full={false} icon="location-outline" label="Check-in" onPress={() => void moCheckInForm()} variant="outline" />
      </Inline> : null}

      {trang.pha === "dang-doc" ? (
        <SkeletonGroup style={styles.khung}>
          <SkeletonCard lines={1} media={220} />
          <SkeletonCard lines={2} />
        </SkeletonGroup>
      ) : null}
      {trang.pha === "hong" ? <ErrorState body={trang.loi} onRetry={() => void chay(docTrangDau)} title="Chưa đọc được tường" /> : null}
      {trang.pha === "xong" && trang.kyNiem.length === 0 ? (
        <EmptyState body="Một tấm ảnh hay một chỗ cả hội đang ngồi: giữ khoảnh khắc đầu tiên ở đây." kind="first-use" layout="inline" illustration={<Canh id="chua-co-anh" width={112} />} title="Chưa có kỷ niệm nào" action={{ label: "Thả khoảnh khắc", onPress: () => router.push(`/moments/new?ctx=${contextId}` as never) }} secondary={{ label: "Check-in", onPress: moCheckInForm }} />
      ) : null}
      {trang.pha === "xong" ? (
        <View>
          {trang.kyNiem.map((k) => {
            const anh = nguonAnh(k.imageUrl, me, contextId);
            const tacGia = tenCua(roster, k.authorId);
            const dangMoBl = moBinhLuan === k.id;
            return (
              <View key={k.id} style={[styles.bai, { borderBottomColor: colors.line }]}>
                <View style={styles.dong}>
                  <AvatarNguoi name={tacGia} personId={k.authorId} size={36} />
                  <View style={styles.flex}>
                    <Text style={[typography.label, { color: k.authorId ? mucNguoi(k.authorId, dark) : colors.ink }]}>{tacGia}</Text>
                    <Text style={[typography.caption, { color: colors.inkFaint }]}>{gioViet(k.createdAt)}</Text>
                  </View>
                </View>
                {anh !== null ? (
                  // A print pinned to the wall: a slight lean of its own and a
                  // strip of washi over the top edge (ADR-0037 D1, plan S6).
                  <View style={styles.khungDan}>
                  <KhungAnh tilt={nghiengAnh(k.id)}>
                    <Pressable accessibilityRole="button" accessibilityLabel={`Mở ảnh: ${cauKyNiem(k)}`} onPress={() => setViewer({ id: k.id, source: anh, caption: k.caption ?? "Khoảnh khắc của nhóm" })}>
                    <MemoryPhoto
                      label={cauKyNiem(k)}
                      // The frame takes the photo's shape once it is known: a
                      // portrait memory was cut to a 4:3 strip (QA 23/09).
                      onLoad={(e) => {
                        const r = tiLeKhung(e.source);
                        setTiLe((cu) => (cu[k.id] === r ? cu : { ...cu, [k.id]: r }));
                      }}
                      source={anh}
                      ratio={tiLe[k.id] ?? 4 / 3}
                    />
                    </Pressable>
                  </KhungAnh>
                  <Washi style={styles.bangDinh} tilt={nghiengAnh(k.id) > 0 ? -2 : 2} />
                  </View>
                ) : null}
                {k.kind === "checkin" ? (
                  <View style={styles.checkin}>
                    <Ionicons color={colors.accent} name="location" size={18} />
                    <View style={styles.flex}>
                      <Text style={[typography.label, { color: colors.ink }]}>{cauKyNiem(k)}</Text>
                      {k.caption !== null && k.caption.trim() !== "" ? <Text style={[typography.body, { color: colors.inkSoft }]}>{k.caption}</Text> : null}
                    </View>
                  </View>
                ) : k.caption !== null && k.caption.trim() !== "" ? (
                  <Text style={[typography.body, { color: colors.ink }]}>{k.caption}</Text>
                ) : null}
                <View style={styles.hanhDong}>
                  <Pressable
                    accessibilityLabel={`${k.toiDaTim ? "Bỏ tim" : "Thả tim"} ${cauKyNiem(k)}`}
                    accessibilityRole="button"
                    aria-pressed={k.toiDaTim}
                    disabled={ban}
                    onPress={() => void tim(k)}
                    style={({ pressed }) => [styles.nutHanhDong, pressed && styles.pressed]}
                  >
                    <Ionicons color={k.toiDaTim ? colors.accent : colors.inkSoft} name={k.toiDaTim ? "heart" : "heart-outline"} size={22} />
                    <Text style={[typography.label, { color: k.toiDaTim ? colors.accent : colors.inkSoft }]}>{k.toiDaTim ? "Đã thả tim" : "Thả tim"}</Text>
                  </Pressable>
                  <Pressable
                    accessibilityLabel={`${dangMoBl ? "Ẩn bình luận" : "Bình luận"} ${cauKyNiem(k)}`}
                    accessibilityRole="button"
                    aria-expanded={dangMoBl}
                    disabled={ban}
                    onPress={() => void moHoacDongBinhLuan(k)}
                    style={({ pressed }) => [styles.nutHanhDong, pressed && styles.pressed]}
                  >
                    <Ionicons color={dangMoBl ? colors.accent : colors.inkSoft} name={dangMoBl ? "chatbubble" : "chatbubble-outline"} size={21} />
                    <Text style={[typography.label, { color: dangMoBl ? colors.accent : colors.inkSoft }]}>{dangMoBl ? "Ẩn bình luận" : "Bình luận"}</Text>
                  </Pressable>
                </View>
                <Text style={[typography.caption, { color: colors.inkSoft }]}>{cauTuongTac(k)}</Text>
                {loiKyNiem?.id === k.id && !dangMoBl ? <CauTaiCho cau={loiKyNiem.cau} co="nho" hanhDong={{ label: "Thử lại", onPress: loiKyNiem.thu }} /> : null}
              </View>
            );
          })}
        </View>
      ) : null}
      {trang.pha === "xong" && trang.conNua ? <RudiButton disabled={ban} label="Tải thêm kỷ niệm cũ hơn" onPress={() => void taiThem()} variant="outline" /> : null}
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  khungDan: { paddingTop: 6 },
  bangDinh: { position: "absolute", top: -4, alignSelf: "center", width: 84, height: 22 },
  flex: { flex: 1 },
  khung: { gap: 16 },
  form: { gap: 10, paddingBottom: 4 },
  bai: { gap: 10, paddingVertical: 16, borderBottomWidth: StyleSheet.hairlineWidth },
  dong: { flexDirection: "row", alignItems: "center", gap: 10 },
  checkin: { flexDirection: "row", alignItems: "flex-start", gap: 8 },
  hanhDong: { flexDirection: "row", flexWrap: "wrap", alignItems: "center", gap: 8 },
  nutHanhDong: { minHeight: 48, maxWidth: "100%", flexDirection: "row", alignItems: "center", gap: 8, paddingRight: 10 },
  cho: { flexDirection: "row", alignItems: "center", gap: 10, paddingVertical: 10, minHeight: 56, borderBottomWidth: StyleSheet.hairlineWidth },
  editor: { gap: 10, paddingTop: 10, borderTopWidth: StyleSheet.hairlineWidth },
  pressed: { opacity: 0.7 },
});
