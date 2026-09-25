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
  docTuongCua,
  dongPhuBai,
  loiRaChu,
  type Bai,
  type HoSoNguoi,
} from "../../nguoi/ho-so-nguoi";
import { attemptFor, type Attempt } from "../../../api";
import { docHoSoToi, ganDanhSachNhom } from "../../../phien";
import { guiLoiMoi } from "../../../screens/ca-nhan/ban-be";
import { nguonAnhBai } from "../../nguoi/anh-ca-nhan";
import { CHINH_SACH, datChinhSachBinhLuan, laChinhSach, type ChinhSachBinhLuan } from "../../nguoi/chinh-sach-tuong";
import { ghepVaoDanhSach, moNhanRieng } from "../../nhan-rieng/nhan-rieng";
import { useRudiSession } from "../../session";
import { typography, useRudiTheme } from "../../theme";
import { cauTuongTacBai } from "../../tuong/bai-chi-tiet";
import { HanhDongHoSoSheet } from "./HanhDongHoSo";
import { Chip, Heading, RudiButton, RudiScreen, TopBar } from "../../ui";
import { AvatarNguoi } from "../../ui/AvatarNguoi";
import { EmptyState } from "../../ui/EmptyState";
import { ErrorState } from "../../ui/ErrorState";
import { SkeletonGroup, SkeletonLines, SkeletonRow } from "../../ui/Skeleton";

type TrangHoSo =
  | { pha: "dang-doc" }
  | { pha: "xong"; hoSo: HoSoNguoi }
  | { pha: "hong"; loi: string };

type TrangTuong =
  | { pha: "dang-doc" }
  | { pha: "xong"; bai: Bai[] }
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

  const [ketBan, setKetBan] = useState<"chua" | "dang-gui" | "da-gui">("chua");
  const [loiKetBan, setLoiKetBan] = useState<string | null>(null);
  const guiKetBan = async () => {
    if (phien === null || personId === "" || ketBan !== "chua") return;
    setKetBan("dang-gui");
    setLoiKetBan(null);
    try {
      await guiLoiMoi(personId, phien.person_id, attemptFor(attempts.current, `ket-ban:${personId}`));
      setKetBan("da-gui");
    } catch (error) {
      setKetBan("chua");
      setLoiKetBan(loiRaChu(error));
    }
  };

  const nhanTin = async (den: "chat" | "to-giay" = "chat") => {
    if (phien === null || personId === "" || dangMoChat) return;
    setDangMoChat(true);
    setLoiChat(null);
    try {
      const cap = await moNhanRieng(personId, phien.person_id, attemptFor(attempts.current, `dm:${personId}`));
      datPhien(await ganDanhSachNhom(phien, ghepVaoDanhSach(phien.contexts, cap)));
      router.push(`/groups/${cap.id}/${den}` as never);
    } catch (error) {
      setLoiChat(loiRaChu(error));
    } finally {
      setDangMoChat(false);
    }
  };

  const napHoSo = useCallback(async () => {
    if (phien === null || personId === "") return;
    setHoSo({ pha: "dang-doc" });
    try {
      setHoSo({ pha: "xong", hoSo: await docHoSoNguoi(personId, phien.person_id) });
    } catch (error) {
      setHoSo({ pha: "hong", loi: loiRaChu(error) });
    }
  }, [personId, phien]);

  const napTuong = useCallback(async () => {
    if (phien === null || personId === "") return;
    setTuong({ pha: "dang-doc" });
    try {
      setTuong({ pha: "xong", bai: await docTuongCua(personId, phien.person_id) });
    } catch (error) {
      setTuong({ pha: "hong", loi: loiRaChu(error) });
    }
  }, [personId, phien]);

  useFocusEffect(
    useCallback(() => {
      void napHoSo();
      void napTuong();
      void napChinhSach();
    }, [napHoSo, napTuong, napChinhSach]),
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
          <View style={styles.hoSo}>
            <View style={styles.dau}>
              <AvatarNguoi name={hoSo.hoSo.display_name} personId={personId} size={60} />
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
              <Chip icon={hoSo.hoSo.relation === "couple" ? "heart" : undefined} label={cauQuanHe(hoSo.hoSo.relation)} selected={hoSo.hoSo.relation === "couple"} />
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
            {hoSo.hoSo.relation === "friend" || hoSo.hoSo.relation === "couple" ? (
              <View style={styles.khoiChat}>
                <RudiButton
                  icon="chatbubble-outline"
                  label="Nhắn tin"
                  loading={dangMoChat}
                  onPress={() => void nhanTin()}
                />
                {/* ADR-0034: for the two of a couple, the notebook is one tap
                    from the other's profile, not three screens away. */}
                {hoSo.hoSo.relation === "couple" ? (
                  <RudiButton
                    disabled={dangMoChat}
                    icon="mail-outline"
                    label="Tờ giấy của hai mình"
                    onPress={() => void nhanTin("to-giay")}
                    variant="outline"
                  />
                ) : null}
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
                {/* B3 (QC 24/09): the sentence said «make friends» with nothing
                    to press; adding somebody from the same group meant knowing
                    their number. Not offered to a person just blocked here. */}
                {!daChan ? (
                  ketBan === "da-gui" ? (
                    <Text accessibilityLiveRegion="polite" style={[typography.caption, { color: colors.inkSoft }]} testID="ho-so-da-gui-ket-ban">
                      Đã gửi lời mời kết bạn. Khi {hoSo.hoSo.display_name} đồng ý, hai bạn nhắn riêng được.
                    </Text>
                  ) : (
                    <RudiButton icon="person-add-outline" label="Kết bạn" loading={ketBan === "dang-gui"} onPress={() => void guiKetBan()} variant="outline" />
                  )
                ) : null}
                {loiKetBan ? <Text accessibilityLiveRegion="polite" style={[typography.caption, { color: colors.warn }]}>{loiKetBan}</Text> : null}
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
          <Heading title={hoSo.hoSo.relation === "self" ? "Tường của bạn" : "Tường cá nhân"} />
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
            <View>
              {tuong.bai.map((bai) => (
                <Pressable
                  accessibilityLabel={`Mở bài: ${bai.body}`}
                  accessibilityRole="button"
                  key={bai.id}
                  onPress={() => router.push(`/posts/${bai.id}` as never)}
                  style={({ pressed }) => [styles.bai, { borderBottomColor: colors.line }, pressed && styles.bam]}
                >
                  <Text style={[typography.body, { color: colors.ink }]}>{bai.body}</Text>
                  {bai.image_url ? (
                    <Image
                      accessibilityLabel="Ảnh bài đăng"
                      contentFit="cover"
                      source={nguonAnhBai(bai.image_url, phien?.person_id ?? "")}
                      style={[styles.anhBai, { borderRadius: 12 }]}
                    />
                  ) : null}
                  <Text style={[typography.caption, { color: colors.inkFaint }]}>
                    {dongPhuBai(bai)} · {cauTuongTacBai(bai)}
                  </Text>
                </Pressable>
              ))}
            </View>
          ) : null}
        </>
      ) : null}
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  hoSo: { gap: 10 },
  dau: { flexDirection: "row", alignItems: "center", gap: 14 },
  dauChu: { flex: 1, gap: 2 },
  chips: { flexDirection: "row", flexWrap: "wrap", gap: 8 },
  bai: { gap: 6, paddingVertical: 12, borderBottomWidth: StyleSheet.hairlineWidth },
  khoiChat: { gap: 6, marginTop: 4 },
  chinhSach: { gap: 8, marginTop: 4 },
  anhBai: { width: "100%", aspectRatio: 4 / 3 },
  bam: { opacity: 0.7 },
});
