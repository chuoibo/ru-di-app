/**
 * Somebody else's profile and wall (M8): `/people/{id}`.
 *
 * Two independent reads. The profile is the gate -- a refusal there is the
 * whole screen, because there is nothing honest to draw without it. The wall
 * is loaded after and fails on its own, so a wall that does not answer does
 * not hide a person who did.
 *
 * The header tile is the same warm initial used on every friend surface, not
 * `Avatar`: that primitive takes a fixture person and a fixture colour, and
 * this screen never touches the fixture.
 */
import { useFocusEffect, useLocalSearchParams, useRouter } from "expo-router";
import { useCallback, useRef, useState } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";

import { Image } from "expo-image";

import { attemptFor, type Attempt } from "../../../api";
import { docHoSoToi, ganDanhSachNhom } from "../../../phien";
import { chuDau } from "../../../screens/ca-nhan/ban-be";
import { nguonAnhBai } from "../../nguoi/anh-ca-nhan";
import { CHINH_SACH, datChinhSachBinhLuan, laChinhSach, type ChinhSachBinhLuan } from "../../nguoi/chinh-sach-tuong";
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
import { ghepVaoDanhSach, moNhanRieng } from "../../nhan-rieng/nhan-rieng";
import { useRudiSession } from "../../session";
import { typography, useRudiTheme } from "../../theme";
import { cauTuongTacBai } from "../../tuong/bai-chi-tiet";
import { Card, Chip, Divider, Heading, RudiButton, RudiScreen, TopBar } from "../../ui";

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

  if (!phienDaDoc) return null;

  return (
    <RudiScreen testID="ho-so-nguoi-screen">
      <TopBar title="Hồ sơ" />
      {hoSo.pha === "dang-doc" ? (
        <Card>
          <Text style={[typography.body, { color: colors.inkFaint }]}>Đang đọc từ máy chủ…</Text>
        </Card>
      ) : null}
      {hoSo.pha === "hong" ? (
        <Card>
          <Text style={[typography.body, { color: colors.warn }]}>{hoSo.loi}</Text>
          <View style={styles.khoangTren}>
            <RudiButton label="Thử lại" onPress={() => void napHoSo()} variant="outline" />
          </View>
        </Card>
      ) : null}
      {hoSo.pha === "xong" ? (
        <>
          <Card>
            <View style={styles.dau}>
              <View style={[styles.chuDau, { backgroundColor: colors.accentSoft }]}>
                <Text style={[typography.h2, { color: colors.accent }]}>
                  {chuDau(hoSo.hoSo.display_name)}
                </Text>
              </View>
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
            {hoSo.hoSo.relation === "self" ? (
              <View style={styles.khoangTren}>
                <RudiButton
                  icon="create-outline"
                  label="Đăng bài mới"
                  onPress={() => router.push("/posts/new")}
                />
              </View>
            ) : null}
            {hoSo.hoSo.relation === "self" ? (
              <View style={[styles.khoangTren, styles.chinhSach]}>
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
              <View style={[styles.khoangTren, styles.khoiChat]}>
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
              <View style={[styles.khoangTren, styles.khoiChat]}>
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
          </Card>
          <Heading title={hoSo.hoSo.relation === "self" ? "Tường của bạn" : "Tường cá nhân"} />
          {tuong.pha === "dang-doc" ? (
            <Card>
              <Text style={[typography.caption, { color: colors.inkFaint }]}>Đang đọc bài…</Text>
            </Card>
          ) : null}
          {tuong.pha === "hong" ? (
            <Card>
              <Text style={[typography.body, { color: colors.warn }]}>{tuong.loi}</Text>
              <View style={styles.khoangTren}>
                <RudiButton label="Đọc lại tường" onPress={() => void napTuong()} variant="outline" />
              </View>
            </Card>
          ) : null}
          {tuong.pha === "xong" && tuong.bai.length === 0 ? (
            <Card>
              <Text style={[typography.body, { color: colors.inkSoft }]}>
                {cauTuongRong(hoSo.hoSo.relation)}
              </Text>
            </Card>
          ) : null}
          {tuong.pha === "xong" && tuong.bai.length > 0 ? (
            <Card style={styles.danhSach}>
              {tuong.bai.map((bai, i) => (
                <View key={bai.id}>
                  {i > 0 ? <Divider /> : null}
                  <Pressable
                    accessibilityLabel={`Mở bài: ${bai.body}`}
                    accessibilityRole="button"
                    onPress={() => router.push(`/posts/${bai.id}` as never)}
                    style={styles.bai}
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
                </View>
              ))}
            </Card>
          ) : null}
        </>
      ) : null}
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  dau: { flexDirection: "row", alignItems: "center", gap: 14 },
  dauChu: { flex: 1, gap: 2 },
  chuDau: { width: 60, height: 60, borderRadius: 20, alignItems: "center", justifyContent: "center" },
  chips: { flexDirection: "row", flexWrap: "wrap", gap: 8 },
  danhSach: { paddingVertical: 6 },
  bai: { gap: 6, paddingVertical: 10 },
  khoangTren: { marginTop: 4 },
  khoiChat: { gap: 6 },
  chinhSach: { gap: 8 },
  anhBai: { width: "100%", aspectRatio: 4 / 3 },
});
