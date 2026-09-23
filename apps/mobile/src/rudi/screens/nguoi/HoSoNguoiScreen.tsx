/**
 * Somebody else's profile and wall (M8): `/people/{id}`.
 *
 * Two independent reads. The profile is the gate -- a refusal there is the
 * whole screen, because there is nothing honest to draw without it. The wall
 * is loaded after and fails on its own, so a wall that does not answer does
 * not hide a person who did.
 *
 * UI v2 (đợt 7): initial, name, when they joined, the relation as a word;
 * posts are rows on the paper with a hairline between them.
 */
import { useFocusEffect, useLocalSearchParams, useRouter } from "expo-router";
import { Canh } from "../../ui/art/Canh";
import { useCallback, useRef, useState } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";

import { Image } from "expo-image";

import {
  cauNgayVao,
  cauQuanHe,
  cauTuongRong,
  docHoSoNguoi,
  dongPhuBai,
  loiRaChu,
  type HoSoNguoi,
} from "../../nguoi/ho-so-nguoi";
import { ApiError, attemptFor, type Attempt } from "../../../api";
import { docHoSoToi, ganDanhSachNhom } from "../../../phien";
import { nguonAnhBai } from "../../nguoi/anh-ca-nhan";
import { BADGE_TITLES, docHuyHieuTrungBay, type EarnedBadge } from "../../ky-niem/achievement-routes";
import { CHINH_SACH, datChinhSachBinhLuan, laChinhSach, type ChinhSachBinhLuan } from "../../nguoi/chinh-sach-tuong";
import { ghepVaoDanhSach, moNhanRieng } from "../../nhan-rieng/nhan-rieng";
import { useRudiSession } from "../../session";
import { typography, useRudiTheme } from "../../theme";
import { docDoiTuong, docTrangTuong, ghepTrangTuong, type BaiTuong } from "../../tuong/social-v2";
import { HanhDongHoSoSheet } from "./HanhDongHoSo";
import { Chip, Heading, RudiButton, RudiScreen, TopBar } from "../../ui";
import { Avatar } from "../../ui/Avatar";
import { BadgeArt } from "../../ui/BadgeArt";
import { EmptyState } from "../../ui/EmptyState";
import { ErrorState } from "../../ui/ErrorState";
import { SkeletonGroup, SkeletonLines, SkeletonRow } from "../../ui/Skeleton";

type TrangHoSo =
  | { pha: "dang-doc" }
  | { pha: "xong"; hoSo: HoSoNguoi }
  | { pha: "hong"; loi: string };

type TrangTuong =
  | { pha: "dang-doc" }
  | { pha: "xong"; bai: BaiTuong[]; conTro: string | null; conNua: boolean }
  | { pha: "hong"; loi: string };

export function HoSoNguoiScreen() {
  const router = useRouter();
  const { colors } = useRudiTheme();
  const { phien, phienDaDoc, datPhien } = useRudiSession();
  const params = useLocalSearchParams<{ id?: string }>();
  // Written as a statement, not `x ? x : ""`: the id-default scanner reads that
  // shape as a display fallback wherever it appears, and it is right to.
  let personId = "";
  if (typeof params.id === "string") personId = params.id;
  const [hoSo, setHoSo] = useState<TrangHoSo>({ pha: "dang-doc" });
  const [huyHieu, setHuyHieu] = useState<EarnedBadge[]>([]);
  const [tuong, setTuong] = useState<TrangTuong>({ pha: "dang-doc" });
  // ADR-0023 §2.3: blocking and reporting live behind «Thêm hành động». The
  // flag is local because the server never says «you blocked them» on a
  // profile read -- the list of people one blocks is its own screen.
  const [moHanhDong, setMoHanhDong] = useState(false);
  const [daChan, setDaChan] = useState(false);
  // ADR-0021 §2.5: «Nhắn tin» opens (or finds) the pair with this friend. One
  // attempt per person, held in a ref, so a retry is the same write.
  const [dangMoChat, setDangMoChat] = useState(false);
  const [loiChat, setLoiChat] = useState<string | null>(null);
  const attempts = useRef<Record<string, Attempt>>({});
  const doiTuongCursor = useRef<string | null>(null);
  const [dangTaiThem, setDangTaiThem] = useState(false);
  // ADR-0022 §2.2: on one's own wall, who may comment. Read from `/people/me`
  // (the public profile never carries it) and written with one PATCH.
  const [chinhSach, setChinhSach] = useState<ChinhSachBinhLuan | null>(null);
  const [dangDoiChinhSach, setDangDoiChinhSach] = useState(false);
  const [loiChinhSach, setLoiChinhSach] = useState<string | null>(null);

  const napChinhSach = useCallback(async () => {
    if (phien === null || personId !== phien.person_id) return;
    try {
      const toi = await docHoSoToi(phien.person_id);
      setChinhSach(laChinhSach(toi.wall_comment_policy) ? toi.wall_comment_policy : "readers");
    } catch {
      // The wall still draws; the chips wait for the next focus.
    }
  }, [personId, phien]);

  const doiChinhSach = async (moi: ChinhSachBinhLuan) => {
    if (phien === null || dangDoiChinhSach || moi === chinhSach) return;
    setDangDoiChinhSach(true);
    setLoiChinhSach(null);
    try {
      const sau = await datChinhSachBinhLuan(moi, phien.person_id, attemptFor(attempts.current, `chinh-sach:${moi}`));
      setChinhSach(sau.wall_comment_policy);
    } catch (error) {
      setLoiChinhSach(loiRaChu(error));
    } finally {
      setDangDoiChinhSach(false);
    }
  };

  const nhanTin = async () => {
    if (phien === null || personId === "" || dangMoChat) return;
    setDangMoChat(true);
    setLoiChat(null);
    try {
      const cap = await moNhanRieng(personId, phien.person_id, attemptFor(attempts.current, `dm:${personId}`));
      datPhien(await ganDanhSachNhom(phien, ghepVaoDanhSach(phien.contexts, cap)));
      router.push(`/groups/${cap.id}/chat` as never);
    } catch (error) {
      setLoiChat(loiRaChu(error));
    } finally {
      setDangMoChat(false);
    }
  };

  const napHoSo = useCallback(async (quiet = false) => {
    if (phien === null || personId === "") return;
    if (!quiet) setHoSo({ pha: "dang-doc" });
    try {
      setHoSo({ pha: "xong", hoSo: await docHoSoNguoi(personId, phien.person_id) });
    } catch (error) {
      if (!quiet || (error instanceof ApiError && (error.status === 403 || error.status === 404))) {
        setHoSo({ pha: "hong", loi: loiRaChu(error) });
      }
    }
  }, [personId, phien]);

  const napHuyHieu = useCallback(async (quiet = false) => {
    if (phien === null || personId === "") return;
    if (!quiet) setHuyHieu([]);
    try {
      const result = await docHuyHieuTrungBay(phien.person_id, personId);
      setHuyHieu(result.badges);
    } catch {
      if (!quiet) setHuyHieu([]);
    }
  }, [personId, phien]);

  const napTuong = useCallback(async (quiet = false) => {
    if (phien === null || personId === "") return;
    if (!quiet) setTuong({ pha: "dang-doc" });
    try {
      const page = await docTrangTuong(personId, phien.person_id);
      setTuong({ pha: "xong", bai: page.posts, conTro: page.next_cursor, conNua: page.has_more });
    } catch (error) {
      if (!quiet || (error instanceof ApiError && (error.status === 403 || error.status === 404))) {
        setTuong({ pha: "hong", loi: loiRaChu(error) });
      }
    }
  }, [personId, phien]);

  const taiThem = async () => {
    if (phien === null || tuong.pha !== "xong" || !tuong.conNua || tuong.conTro === null || dangTaiThem) return;
    setDangTaiThem(true);
    try {
      const page = await docTrangTuong(personId, phien.person_id, tuong.conTro);
      setTuong({ pha: "xong", bai: ghepTrangTuong(tuong.bai, page.posts), conTro: page.next_cursor, conNua: page.has_more });
    } catch (error) {
      setTuong({ pha: "hong", loi: loiRaChu(error) });
    } finally {
      setDangTaiThem(false);
    }
  };

  useFocusEffect(
    useCallback(() => {
      doiTuongCursor.current = null;
      void napHoSo();
      void napHuyHieu();
      void napTuong();
      void napChinhSach();
      let active = true;
      const watch = async () => {
        while (active && phien !== null && personId !== "") {
          try {
            const change = await docDoiTuong(personId, phien.person_id, doiTuongCursor.current);
            if (!active) return;
            doiTuongCursor.current = change.next_cursor;
            await Promise.all([napTuong(true), napHoSo(true), napHuyHieu(true)]);
          } catch {
            if (!active) return;
            await new Promise((resolve) => setTimeout(resolve, 3000));
          }
        }
      };
      void watch();
      return () => { active = false; };
    }, [napHoSo, napHuyHieu, napTuong, napChinhSach, personId, phien]),
  );

  // ADR-0023 §2.3: khay này phải nằm NGOÀI hộp cuộn của màn. `Sheet` phủ
  // đúng CHA của nó (`StyleSheet.absoluteFill`), nên đặt trong nội dung
  // cuộn thì nó phủ khung nội dung chứ không phủ màn: bảng 2026-09-07 chụp
  // được cảnh chữ của tường vẽ đè lên nút của khay, và cú chạm rơi vào
  // chữ. `RudiScreen` có sẵn `overlay` cho đúng việc này.
  const khayHanhDong =
    hoSo.pha === "xong" && hoSo.hoSo.relation !== "self" && personId !== "" ? (
      <HanhDongHoSoSheet
        actorId={phien?.person_id ?? ""}
        daChan={daChan}
        displayName={hoSo.hoSo.display_name}
        onClose={() => setMoHanhDong(false)}
        onDoiChan={(chan) => {
          setDaChan(chan);
          // Chặn làm mất tình bạn, nên «Nhắn tin» và tường đều sai ngay lúc
          // nó xong. Hỏi lại máy chủ chứ không để một quan hệ cũ trên màn.
          void napHoSo();
          void napTuong();
        }}
        open={moHanhDong}
        personId={personId}
      />
    ) : null;

  if (!phienDaDoc) return null;

  return (
    <RudiScreen overlay={khayHanhDong} testID="ho-so-nguoi-screen">
      <TopBar title="Hồ sơ" />
      {hoSo.pha === "dang-doc" ? (
        <SkeletonGroup>
          <SkeletonRow leading={60} />
        </SkeletonGroup>
      ) : null}
      {hoSo.pha === "hong" ? <ErrorState body={hoSo.loi} onRetry={() => void napHoSo()} title="Chưa mở được hồ sơ" /> : null}
      {hoSo.pha === "xong" ? (
        <>
          <View style={[styles.hoSo, { backgroundColor: colors.accentSoft, borderColor: colors.lineStrong }]}>
            <Text style={[typography.h1, { color: colors.ink }]}>
              {hoSo.hoSo.relation === "self" ? "Những ngày mình đã đi" : `Những ngày của ${hoSo.hoSo.display_name}`}
            </Text>
            <View style={styles.dau}>
              <Avatar name={hoSo.hoSo.display_name} size={60} />
              <View style={styles.dauChu}>
                <Text numberOfLines={2} style={[typography.h2, { color: colors.ink }]}>
                  {hoSo.hoSo.display_name}
                </Text>
                <Text style={[typography.caption, { color: colors.inkFaint }]}>
                  {cauNgayVao(hoSo.hoSo.created_at)}
                </Text>
              </View>
            </View>
            <View style={styles.chips}>
              <Chip label={cauQuanHe(hoSo.hoSo.relation)} />
              {hoSo.hoSo.city ? <Chip icon="location-outline" label={hoSo.hoSo.city} /> : null}
            </View>
            {hoSo.hoSo.bio ? (
              <Text style={[typography.body, { color: colors.inkSoft }]}>{hoSo.hoSo.bio}</Text>
            ) : (
              <Text style={[typography.caption, { color: colors.inkFaint }]}>
                {hoSo.hoSo.relation === "self"
                  ? "Bạn chưa viết giới thiệu. Sửa ở Cá nhân."
                  : "Người này chưa viết giới thiệu."}
              </Text>
            )}
            {huyHieu.length > 0 ? (
              <View style={[styles.huyHieu, { borderTopColor: colors.lineStrong }]}>
                <Text style={[typography.label, { color: colors.inkSoft }]}>Dấu ấn mang theo</Text>
                <View style={styles.huyHieuHang}>
                  {huyHieu.map((badge) => (
                    <View key={badge.id} style={styles.huyHieuMot}>
                      <BadgeArt badgeId={badge.id} label={BADGE_TITLES[badge.id] ?? "Huy hiệu hành trình"} size={54} state="unlocked" />
                      <Text numberOfLines={2} style={[typography.caption, { color: colors.ink, textAlign: "center" }]}>{BADGE_TITLES[badge.id] ?? "Huy hiệu hành trình"}</Text>
                    </View>
                  ))}
                </View>
              </View>
            ) : null}
            {hoSo.hoSo.relation === "self" ? <RudiButton compact full={false} icon="map-outline" label="Xem hành trình" onPress={() => router.push("/achievements")} variant="ghost" /> : null}
            {hoSo.hoSo.relation === "self" ? (
              <RudiButton
                compact
                full={false}
                icon="create-outline"
                label="Đăng bài mới"
                onPress={() => router.push("/posts/new")}
              />
            ) : null}
            {hoSo.hoSo.relation === "self" ? (
              <View style={styles.chinhSach}>
                <Text style={[typography.label, { color: colors.ink }]}>Ai được bình luận tường tôi</Text>
                <View accessibilityRole="radiogroup" style={styles.chips}>
                  {CHINH_SACH.map((c) => (
                    <Chip
                      accessibilityLabel={`Bình luận: ${c.nhan}`}
                      key={c.id}
                      label={c.nhan}
                      onPress={() => void doiChinhSach(c.id)}
                      selected={chinhSach === c.id}
                    />
                  ))}
                </View>
                <Text style={[typography.caption, { color: colors.inkFaint }]}>
                  {CHINH_SACH.find((c) => c.id === chinhSach)?.giaiThich ?? "Đang đọc cài đặt của tường…"}
                </Text>
                {loiChinhSach ? <Text style={[typography.caption, { color: colors.warn }]}>{loiChinhSach}</Text> : null}
              </View>
            ) : null}
            {hoSo.hoSo.relation === "friend" ? (
              <View style={styles.khoiChat}>
                <RudiButton
                  icon="chatbubble-outline"
                  label="Nhắn tin"
                  loading={dangMoChat}
                  onPress={() => void nhanTin()}
                />
                {loiChat ? <Text style={[typography.caption, { color: colors.warn }]}>{loiChat}</Text> : null}
              </View>
            ) : null}
            {hoSo.hoSo.relation === "groupmate" ? (
              <View style={styles.khoiChat}>
                <RudiButton
                  disabled
                  icon="chatbubble-outline"
                  label="Nhắn tin"
                  onPress={() => undefined}
                  variant="ghost"
                />
                <Text style={[typography.caption, { color: colors.inkFaint }]}>Kết bạn để nhắn riêng.</Text>
              </View>
            ) : null}
            {hoSo.hoSo.relation !== "self" ? (
              <View style={styles.khoiChat}>
                {daChan ? <Chip label="Đã chặn" selected /> : null}
                <RudiButton
                  icon="ellipsis-horizontal"
                  label="Thêm hành động"
                  onPress={() => setMoHanhDong(true)}
                  variant="ghost"
                />
              </View>
            ) : null}
          </View>
          <Heading title={hoSo.hoSo.relation === "self" ? "Trang viết của bạn" : "Trang viết được chia sẻ"} />
          {tuong.pha === "dang-doc" ? (
            <SkeletonGroup>
              <SkeletonLines lines={2} />
            </SkeletonGroup>
          ) : null}
          {tuong.pha === "hong" ? <ErrorState body={tuong.loi} onRetry={() => void napTuong()} title="Chưa đọc được tường" /> : null}
          {tuong.pha === "xong" && tuong.bai.length === 0 ? (
            <EmptyState illustration={<Canh id="chua-co-ky-niem" width={150} />} kind="first-use" layout="inline" title={cauTuongRong(hoSo.hoSo.relation)} />
          ) : null}
          {tuong.pha === "xong" && tuong.bai.length > 0 ? (
            <View style={styles.dongChay}>
              {tuong.bai.map((bai) => (
                <View
                  key={bai.id}
                  style={[styles.bai, { backgroundColor: colors.card, borderColor: colors.line, shadowColor: colors.ink }]}
                >
                  <Pressable accessibilityLabel={`Mở bài: ${bai.body}`} accessibilityRole="button" onPress={() => router.push(`/posts/${bai.id}` as never)} style={({ pressed }) => [styles.baiNoiDung, pressed && styles.bam]}>
                    <View style={styles.baiDau}>
                      <View style={[styles.dauMoc, { backgroundColor: colors.accentSoft }]}>
                        <Text style={[typography.stamp, { color: colors.accent }]}>{bai.is_repost ? "CHIA SẺ" : "NHẬT KÝ"}</Text>
                      </View>
                      <Text style={[typography.caption, { color: colors.inkFaint }]}>{dongPhuBai(bai)}</Text>
                    </View>
                    <Text style={[typography.body, { color: colors.ink }]}>{bai.body}</Text>
                  </Pressable>
                  {bai.image_url ? (
                    <Pressable
                      accessibilityLabel="Mở ảnh và bình luận"
                      accessibilityRole="button"
                      onPress={() => router.push(`/posts/${bai.id}?photo=1` as never)}
                    >
                      <Image
                        accessibilityLabel="Ảnh bài đăng"
                        contentFit="cover"
                        source={nguonAnhBai(bai.image_url, phien?.person_id ?? "")}
                        style={[styles.anhBai, { borderRadius: 12 }]}
                      />
                    </Pressable>
                  ) : null}
                  <Pressable accessibilityLabel={`Mở ${bai.comment_count} bình luận của bài`} accessibilityRole="button" onPress={() => router.push(`/posts/${bai.id}` as never)} style={({ pressed }) => [styles.baiNoiDung, pressed && styles.bam]}>
                    {bai.is_repost ? (
                      <View style={[styles.trichDan, { borderColor: colors.lineStrong }]}>
                        <Text style={[typography.label, { color: colors.ink }]}>{bai.origin?.author_display_name ?? "Bài gốc không còn xem được"}</Text>
                        {bai.origin ? <Text numberOfLines={3} style={[typography.body, { color: colors.inkSoft }]}>{bai.origin.body}</Text> : null}
                      </View>
                    ) : null}
                    <Text style={[typography.caption, { color: colors.inkFaint }]}>
                      {bai.like_count} thích · {bai.comment_count} bình luận
                    </Text>
                  </Pressable>
                </View>
              ))}
              {tuong.conNua ? <RudiButton label="Xem những trang trước" loading={dangTaiThem} onPress={() => void taiThem()} variant="outline" /> : null}
            </View>
          ) : null}
        </>
      ) : null}
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  hoSo: { gap: 14, borderWidth: 1, borderRadius: 24, padding: 20, marginBottom: 12 },
  dau: { flexDirection: "row", alignItems: "center", gap: 14 },
  dauChu: { flex: 1, gap: 2 },
  chips: { flexDirection: "row", flexWrap: "wrap", gap: 8 },
  dongChay: { gap: 16, paddingBottom: 24 },
  bai: { gap: 12, padding: 16, borderWidth: 1, borderRadius: 18, shadowOpacity: 0.06, shadowRadius: 12, shadowOffset: { width: 0, height: 6 } },
  baiNoiDung: { gap: 12 },
  baiDau: { flexDirection: "row", justifyContent: "space-between", alignItems: "center", gap: 8 },
  dauMoc: { borderRadius: 5, paddingHorizontal: 8, paddingVertical: 5 },
  trichDan: { borderWidth: 1, borderRadius: 10, padding: 12, gap: 4 },
  huyHieu: { gap: 10, borderTopWidth: StyleSheet.hairlineWidth, paddingTop: 14 },
  huyHieuHang: { flexDirection: "row", flexWrap: "wrap", gap: 8 },
  huyHieuMot: { width: 72, alignItems: "center", gap: 5 },
  khoiChat: { gap: 6, marginTop: 4 },
  chinhSach: { gap: 8, marginTop: 4 },
  anhBai: { width: "100%", aspectRatio: 4 / 3 },
  bam: { opacity: 0.7 },
});
