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
import { Canh } from "../../ui/art/Canh";
import { KhungAnh } from "../../ui/KhungAnh";
import { Image } from "expo-image";
import { useFocusEffect, useRouter } from "expo-router";
import { useCallback, useEffect, useRef, useState } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";

import { ApiError, attemptFor, thongDiepNguoiDoc, type Attempt } from "../../../api";
import type { Phien } from "../../../phien";
import { danhSachThanhVien } from "../../../screens/vao-cua/cong-api";
import { tenCua, type ThanhVien } from "../../chia-bill/hoa-don";
import { docDanhMuc } from "../../kham-pha/dia-diem";
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
} from "../../ky-niem/ky-niem";
import { typography, useRudiTheme } from "../../theme";
import { Chip, Field, Inline, RudiButton, RudiScreen, SearchField, TopBar } from "../../ui";
import { AvatarNguoi } from "../../ui/AvatarNguoi";
import { EmptyState } from "../../ui/EmptyState";
import { ErrorState } from "../../ui/ErrorState";
import { Sheet } from "../../ui/Sheet";
import { SkeletonCard, SkeletonGroup } from "../../ui/Skeleton";

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
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "";
  return d.toLocaleString("vi-VN", { day: "2-digit", month: "2-digit", hour: "2-digit", minute: "2-digit" });
}

export function GroupWallLiveScreen({ phien, contextId }: { phien: Phien; contextId: string }) {
  const router = useRouter();
  const { colors, radius } = useRudiTheme();
  const me = phien.person_id;
  const [trang, setTrang] = useState<Trang>({ pha: "dang-doc" });
  const [roster, setRoster] = useState<ThanhVien[]>([]);
  const [thongBao, setThongBao] = useState<string | null>(null);
  // Each memory's photo ratio, learned from the image as it loads (the wire has no size).
  const [tiLe, setTiLe] = useState<Record<string, number>>({});
  const [ban, setBan] = useState(false);
  const [moBinhLuan, setMoBinhLuan] = useState<string | null>(null);
  const [binhLuan, setBinhLuan] = useState<Record<string, BinhLuan[]>>({});
  const [nhap, setNhap] = useState("");
  const [moCheckIn, setMoCheckIn] = useState(false);
  const [danhMuc, setDanhMuc] = useState<Cho[] | null>(null);
  const [timCho, setTimCho] = useState("");
  const [choChon, setChoChon] = useState<Cho | null>(null);
  const [cauCheckIn, setCauCheckIn] = useState("");
  const attempts = useRef<Record<string, Attempt>>({});
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

  const chay = async (viec: () => Promise<void>) => {
    setBan(true);
    setThongBao(null);
    try {
      await viec();
    } catch (error) {
      setThongBao(loiRaChu(error));
    } finally {
      setBan(false);
    }
  };

  const thayKyNiem = (moi: KyNiem) =>
    setTrang((t) => (t.pha === "xong" ? { ...t, kyNiem: t.kyNiem.map((k) => (k.id === moi.id ? moi : k)) } : t));

  const tim = (k: KyNiem) => chay(async () => thayKyNiem(await doiTim(k, contextId, me)));

  const moHoacDongBinhLuan = (k: KyNiem) =>
    chay(async () => {
      if (moBinhLuan === k.id) {
        setMoBinhLuan(null);
        return;
      }
      setMoBinhLuan(k.id);
      setNhap("");
      if (binhLuan[k.id] === undefined) {
        const ds = await docBinhLuanCua(contextId, k.id, me);
        setBinhLuan((b) => ({ ...b, [k.id]: ds }));
      }
    });

  const guiBinhLuan = (k: KyNiem) =>
    chay(async () => {
      if (nhap.trim() === "") return;
      const bl = await guiBinhLuanCho(contextId, k.id, nhap, me, attempts.current);
      setBinhLuan((b) => ({ ...b, [k.id]: [...(b[k.id] === undefined ? [] : b[k.id]), bl] }));
      thayKyNiem({ ...k, commentCount: k.commentCount + 1 });
      setNhap("");
    });

  const taiThem = () =>
    chay(async () => {
      if (trang.pha !== "xong" || trang.conTro === null) return;
      const t = await docTuongNhom(contextId, me, { before: trang.conTro });
      setTrang({ pha: "xong", kyNiem: [...trang.kyNiem, ...t.kyNiem], conTro: t.conTro, conNua: t.conNua });
    });

  const moCheckInForm = () =>
    chay(async () => {
      setMoCheckIn(true);
      if (danhMuc === null) {
        const dm = await docDanhMuc();
        setDanhMuc(dm.places.map((p) => ({ id: p.id, name: p.name })));
      }
    });

  const dangCheckIn = () =>
    chay(async () => {
      if (choChon === null) return;
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
    <Sheet accessibilityLabel="Check-in ở đâu?" onClose={() => setMoCheckIn(false)} open={moCheckIn}>
      <View style={styles.form}>
        <Text style={[typography.h2, { color: colors.ink }]}>Check-in ở đâu?</Text>
        {danhMuc === null ? <Text style={[typography.caption, { color: colors.inkFaint }]}>Đang đọc danh mục…</Text> : null}
        {/* A destination holds a hundred places since the catalogue became
            real (M9), so this stopped being a chip row and became a search:
            a hundred chips is a haystack, not a choice. The box narrows;
            what stays is the first dozen matches. */}
        {danhMuc === null ? null : (
          <SearchField
            accessibilityLabel="Ô tìm chỗ check-in"
            onChangeText={setTimCho}
            placeholder="Tìm chỗ bạn đang ở"
            value={timCho}
          />
        )}
        <Inline gap={6} wrap>
          {choHienRa.map((cho) => (
            <Chip accessibilityLabel={`Chọn ${cho.name}`} key={cho.id} label={cho.name} onPress={() => setChoChon(cho)} selected={choChon !== null && choChon.id === cho.id} />
          ))}
        </Inline>
        {danhMuc !== null && choHienRa.length === 0 ? (
          <Text style={[typography.caption, { color: colors.inkFaint }]}>
            Không có chỗ nào khớp «{timCho}». Thử tên ngắn hơn nhé.
          </Text>
        ) : null}
        <Field accessibilityLabel="Ô câu check-in" label="Một câu (không bắt buộc)" onChangeText={setCauCheckIn} placeholder="Ví dụ: Ốc ở đây ngon" value={cauCheckIn} />
        {thongBao !== null && moCheckIn ? <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.warn }]}>{thongBao}</Text> : null}
        <RudiButton disabled={ban || choChon === null} icon="checkmark" label="Đăng check-in" loading={ban} onPress={() => void dangCheckIn()} />
        <RudiButton label="Thôi" onPress={() => setMoCheckIn(false)} variant="ghost" />
      </View>
    </Sheet>
  );

  return (
    <RudiScreen overlay={khayCheckIn} testID="group-wall-screen">
      <TopBar subtitle={laDoi ? "Chỉ hai bạn thấy" : "Chỉ thành viên nhóm thấy"} title={laDoi ? "Kỷ niệm của hai bạn" : "Tường nhóm"} />
      <Text style={[typography.h1, { color: colors.ink }]}>{tenNhom}</Text>
      {thongBao !== null && !moCheckIn ? <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.warn }]}>{thongBao}</Text> : null}
      <Inline gap={8} wrap>
        <RudiButton compact full={false} icon="camera-outline" label="Thả khoảnh khắc" onPress={() => router.push(`/moments/new?ctx=${contextId}` as never)} />
        <RudiButton compact disabled={ban} full={false} icon="location-outline" label="Check-in" onPress={() => void moCheckInForm()} variant="outline" />
      </Inline>

      {trang.pha === "dang-doc" ? (
        <SkeletonGroup style={styles.khung}>
          <SkeletonCard lines={1} media={220} />
          <SkeletonCard lines={2} />
        </SkeletonGroup>
      ) : null}
      {trang.pha === "hong" ? <ErrorState body={trang.loi} onRetry={() => void chay(docTrangDau)} title="Chưa đọc được tường" /> : null}
      {trang.pha === "xong" && trang.kyNiem.length === 0 ? (
        <EmptyState body="Thả khoảnh khắc đầu tiên của nhóm, hoặc check-in ở chỗ đang ngồi." kind="first-use" layout="inline" illustration={<Canh id="chua-co-ky-niem" width={168} />} title="Chưa có kỷ niệm nào" />
      ) : null}
      {trang.pha === "xong" ? (
        <View>
          {trang.kyNiem.map((k) => {
            const anh = nguonAnh(k.imageUrl, me, contextId);
            const dsBl = binhLuan[k.id];
            const tacGia = tenCua(roster, k.authorId);
            const dangMoBl = moBinhLuan === k.id;
            return (
              <View key={k.id} style={[styles.bai, { borderBottomColor: colors.line }]}>
                <View style={styles.dong}>
                  <AvatarNguoi name={tacGia} personId={k.authorId} size={36} />
                  <View style={styles.flex}>
                    <Text style={[typography.label, { color: colors.ink }]}>{tacGia}</Text>
                    <Text style={[typography.caption, { color: colors.inkFaint }]}>{gioViet(k.createdAt)}</Text>
                  </View>
                </View>
                {anh !== null ? (
                  <KhungAnh xuatXu={`${tacGia} · ${gioViet(k.createdAt)}`}>
                    <Image
                      accessibilityLabel={cauKyNiem(k)}
                      contentFit="cover"
                      // The frame takes the photo's shape once it is known: a
                      // portrait memory was cut to a 4:3 strip (QA 23/09).
                      onLoad={(e) => {
                        const r = tiLeKhung(e.source);
                        setTiLe((cu) => (cu[k.id] === r ? cu : { ...cu, [k.id]: r }));
                      }}
                      source={anh}
                      style={[styles.anh, { aspectRatio: tiLe[k.id] ?? 4 / 3, backgroundColor: colors.line }]}
                    />
                  </KhungAnh>
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
                    <Text style={[typography.label, { color: k.toiDaTim ? colors.accent : colors.inkSoft }]}>{k.toiDaTim ? "Đã tim" : "Thích"}</Text>
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
                  <Text style={[typography.caption, styles.flex, { color: colors.inkFaint, textAlign: "right" }]}>{cauTuongTac(k)}</Text>
                </View>
                {dangMoBl ? (
                  <View style={[styles.khungBl, { borderLeftColor: colors.line }]}>
                    {dsBl === undefined ? <Text style={[typography.caption, { color: colors.inkFaint }]}>Đang đọc bình luận…</Text> : null}
                    {dsBl !== undefined && dsBl.length === 0 ? <Text style={[typography.caption, { color: colors.inkFaint }]}>Chưa có bình luận. Viết câu đầu tiên.</Text> : null}
                    {(dsBl === undefined ? [] : dsBl).map((bl) => (
                      <Text key={bl.id} style={[typography.body, { color: colors.ink }]}>
                        <Text style={typography.label}>{bl.tenTacGia}: </Text>
                        {bl.noiDung}
                      </Text>
                    ))}
                    <Field accessibilityLabel="Ô viết bình luận" onChangeText={setNhap} placeholder="Viết bình luận…" value={nhap} />
                    <RudiButton compact disabled={ban || nhap.trim() === ""} full={false} label="Gửi bình luận" loading={ban} onPress={() => void guiBinhLuan(k)} variant="soft" />
                  </View>
                ) : null}
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
  flex: { flex: 1 },
  khung: { gap: 16 },
  form: { gap: 10, paddingBottom: 4 },
  bai: { gap: 10, paddingVertical: 16, borderBottomWidth: StyleSheet.hairlineWidth },
  dong: { flexDirection: "row", alignItems: "center", gap: 10 },
  anh: { width: "100%" },
  checkin: { flexDirection: "row", alignItems: "flex-start", gap: 8 },
  hanhDong: { flexDirection: "row", alignItems: "center", gap: 4 },
  nutHanhDong: { minHeight: 48, flexDirection: "row", alignItems: "center", gap: 6, paddingRight: 12 },
  pressed: { opacity: 0.7 },
  khungBl: { gap: 8, paddingLeft: 12, borderLeftWidth: 2 },
});
