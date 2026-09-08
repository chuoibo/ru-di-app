/**
 * One post, with what sits under it (ADR-0022 §2.2): `/posts/{id}`.
 *
 * The post is the gate: a refusal there is the whole screen (404 for a post
 * that is not for this reader, the same 404 as for no post at all). The
 * comments load after and fail on their own. Reactions redraw from the
 * server's recount after every write, never from a local increment; the
 * composer is drawn from `can_comment` -- the server's answer for THIS reader
 * -- and from nothing the client could derive itself.
 */
import { Image } from "expo-image";
import { Canh } from "../../ui/art/Canh";
import { useFocusEffect, useLocalSearchParams, useRouter } from "expo-router";
import { useCallback, useRef, useState } from "react";
import { FlatList, Pressable, StyleSheet, Text, View } from "react-native";
import { Ionicons } from "@expo/vector-icons";

import { attemptFor, type Attempt } from "../../../api";
import { PHAN_UNG } from "../../chat/tin-song";
import { chuDau } from "../../../screens/ca-nhan/ban-be";
import { nguonAnhBai } from "../../nguoi/anh-ca-nhan";
import { cauLucNao, loiRaChu, nhanMuc } from "../../nguoi/ho-so-nguoi";
import { useRudiSession } from "../../session";
import { typography, useRudiTheme } from "../../theme";
import {
  CAU_KHONG_BINH_LUAN,
  apPhanUng,
  boBinhLuan,
  boPhanUngBai,
  cauTuongTacBai,
  coTheBinhLuan,
  coTheXoaBinhLuan,
  daPhanUng,
  demLoai,
  docBaiChiTiet,
  docBinhLuanBai,
  ghepBinhLuan,
  guiBinhLuanBai,
  themPhanUngBai,
  xoaBinhLuanBai,
  type BaiWire,
  type BinhLuanBai,
  type LoaiPhanUng,
} from "../../tuong/bai-chi-tiet";
import { Card, Divider, Field, IconButton, RudiButton, RudiScreen, TopBar } from "../../ui";
import { Sheet } from "../../ui/Sheet";
import { NoiDungBaoCao } from "../nguoi/NoiDungBaoCao";
import { EmptyState } from "../../ui/EmptyState";
import { ErrorState } from "../../ui/ErrorState";
import { SkeletonCard, SkeletonRow } from "../../ui/Skeleton";

type TrangBai = { pha: "dang-doc" } | { pha: "xong"; bai: BaiWire } | { pha: "hong"; loi: string };
type TrangBl =
  | { pha: "dang-doc" }
  | { pha: "xong"; danhSach: BinhLuanBai[]; conNua: boolean; conTro: string | null }
  | { pha: "hong"; loi: string };

export function BaiChiTietScreen() {
  const router = useRouter();
  const { colors, radius, space } = useRudiTheme();
  const { phien, phienDaDoc } = useRudiSession();
  const params = useLocalSearchParams<{ id?: string }>();
  let postId = "";
  if (typeof params.id === "string") postId = params.id;
  const [bai, setBai] = useState<TrangBai>({ pha: "dang-doc" });
  const [bl, setBl] = useState<TrangBl>({ pha: "dang-doc" });
  const [nhap, setNhap] = useState("");
  const [ban, setBan] = useState(false);
  const [loiViet, setLoiViet] = useState<string | null>(null);
  const [anhHong, setAnhHong] = useState(false);
  // ADR-0023 §2.4: báo cáo bài của người khác. Nút nằm dưới cùng, sau bình
  // luận: nó là việc hiếm và không nên tranh chỗ với việc thường.
  const [baoCaoMo, setBaoCaoMo] = useState(false);
  const attempts = useRef<Record<string, Attempt>>({});
  const toi = phien?.person_id ?? "";

  const napBai = useCallback(async () => {
    if (phien === null || postId === "") return;
    setBai({ pha: "dang-doc" });
    try {
      setBai({ pha: "xong", bai: await docBaiChiTiet(postId, phien.person_id) });
    } catch (error) {
      setBai({ pha: "hong", loi: loiRaChu(error) });
    }
  }, [phien, postId]);

  const napBl = useCallback(async () => {
    if (phien === null || postId === "") return;
    setBl({ pha: "dang-doc" });
    try {
      const trang = await docBinhLuanBai(postId, phien.person_id);
      setBl({ pha: "xong", danhSach: trang.comments, conNua: trang.has_more, conTro: trang.next_cursor });
    } catch (error) {
      setBl({ pha: "hong", loi: loiRaChu(error) });
    }
  }, [phien, postId]);

  useFocusEffect(
    useCallback(() => {
      void napBai();
      void napBl();
    }, [napBai, napBl]),
  );

  if (!phienDaDoc) return null;

  const taiThem = async () => {
    if (phien === null || bl.pha !== "xong" || !bl.conNua || bl.conTro === null || ban) return;
    setBan(true);
    try {
      const trang = await docBinhLuanBai(postId, phien.person_id, bl.conTro);
      setBl({ pha: "xong", danhSach: ghepBinhLuan(bl.danhSach, trang.comments), conNua: trang.has_more, conTro: trang.next_cursor });
    } catch (error) {
      setLoiViet(loiRaChu(error));
    } finally {
      setBan(false);
    }
  };

  const phanUng = async (kind: LoaiPhanUng) => {
    if (phien === null || bai.pha !== "xong" || ban) return;
    setBan(true);
    setLoiViet(null);
    const hienTai = bai.bai;
    try {
      const moi = daPhanUng(hienTai, kind)
        ? await boPhanUngBai(postId, kind, phien.person_id, attemptFor(attempts.current, `bo:${kind}:${Date.now()}`))
        : await themPhanUngBai(postId, kind, phien.person_id, attemptFor(attempts.current, `them:${kind}:${Date.now()}`));
      setBai({ pha: "xong", bai: apPhanUng(hienTai, moi) });
    } catch (error) {
      setLoiViet(loiRaChu(error));
    } finally {
      setBan(false);
    }
  };

  const guiBl = async () => {
    if (phien === null || bai.pha !== "xong" || nhap.trim() === "" || ban) return;
    setBan(true);
    setLoiViet(null);
    try {
      const moi = await guiBinhLuanBai(postId, nhap, phien.person_id, attemptFor(attempts.current, `bl:${nhap.trim()}`));
      setNhap("");
      setBl((cu) => (cu.pha === "xong" ? { ...cu, danhSach: ghepBinhLuan(cu.danhSach, [moi]) } : cu));
      setBai({ pha: "xong", bai: { ...bai.bai, comment_count: (bai.bai.comment_count ?? 0) + 1 } });
    } catch (error) {
      setLoiViet(loiRaChu(error));
    } finally {
      setBan(false);
    }
  };

  const xoaBl = async (c: BinhLuanBai) => {
    if (phien === null || bai.pha !== "xong" || ban) return;
    setBan(true);
    setLoiViet(null);
    try {
      await xoaBinhLuanBai(postId, c.id, phien.person_id, attemptFor(attempts.current, `xoa:${c.id}`));
      setBl((cu) => (cu.pha === "xong" ? { ...cu, danhSach: boBinhLuan(cu.danhSach, c.id) } : cu));
      setBai({ pha: "xong", bai: { ...bai.bai, comment_count: Math.max(0, (bai.bai.comment_count ?? 1) - 1) } });
    } catch (error) {
      setLoiViet(loiRaChu(error));
    } finally {
      setBan(false);
    }
  };

  const dau = (
    <View style={{ gap: space.md }}>
      {bai.pha === "dang-doc" ? <SkeletonCard lines={3} media={0} /> : null}
      {bai.pha === "hong" ? <ErrorState body={bai.loi} onRetry={() => void napBai()} title="Chưa mở được bài" /> : null}
      {bai.pha === "xong" ? (
        <Card>
          <Pressable
            accessibilityLabel={`Xem hồ sơ ${bai.bai.author_display_name ?? ""}`.trim()}
            accessibilityRole="button"
            onPress={() => router.push(`/people/${bai.bai.author_id}` as never)}
            style={styles.tacGia}
          >
            <View style={[styles.chuDau, { backgroundColor: colors.accentSoft }]}>
              <Text style={[typography.label, { color: colors.accent }]}>{chuDau(bai.bai.author_display_name ?? "")}</Text>
            </View>
            <View style={styles.tacGiaChu}>
              <Text numberOfLines={1} style={[typography.label, { color: colors.ink }]}>
                {bai.bai.author_display_name && bai.bai.author_display_name !== "" ? bai.bai.author_display_name : "Thành viên"}
              </Text>
              <Text style={[typography.caption, { color: colors.inkFaint }]}>
                {cauLucNao(bai.bai.created_at)} · {nhanMuc(bai.bai.audience)}
              </Text>
            </View>
          </Pressable>
          <Text style={[typography.body, { color: colors.ink }]}>{bai.bai.body}</Text>
          {bai.bai.image_url ? (
            anhHong ? (
              <View accessibilityLabel="Chưa tải được ảnh" style={[styles.anh, styles.anhHong, { backgroundColor: colors.line, borderRadius: radius.small }]}>
                <Text style={[typography.caption, { color: colors.inkFaint }]}>Chưa tải được ảnh</Text>
              </View>
            ) : (
              <Image
                accessibilityLabel="Ảnh bài đăng"
                contentFit="cover"
                onError={() => setAnhHong(true)}
                source={nguonAnhBai(bai.bai.image_url, toi)}
                style={[styles.anh, { borderRadius: radius.small }]}
              />
            )
          ) : null}
          <Text style={[typography.caption, { color: colors.inkSoft }]}>{cauTuongTacBai(bai.bai)}</Text>
          <View style={styles.phanUng}>
            {PHAN_UNG.map((p) => {
              const chon = daPhanUng(bai.bai, p.kind);
              const dem = demLoai(bai.bai, p.kind);
              return (
                <Pressable
                  accessibilityLabel={p.nhan}
                  accessibilityRole="button"
                  accessibilityState={{ selected: chon }}
                  disabled={ban}
                  key={p.kind}
                  onPress={() => void phanUng(p.kind)}
                  style={[
                    styles.nutPhanUng,
                    { borderRadius: 999, borderColor: chon ? colors.accent : colors.lineStrong, backgroundColor: chon ? colors.accentSoft : colors.card },
                  ]}
                >
                  <Text style={[typography.caption, { color: chon ? colors.accent : colors.ink }]}>
                    {p.glyph}
                    {dem > 0 ? ` ${dem}` : ""}
                  </Text>
                </Pressable>
              );
            })}
          </View>
        </Card>
      ) : null}
      {bai.pha === "xong" ? <Text style={[typography.label, { color: colors.ink }]}>Bình luận</Text> : null}
      {bl.pha === "dang-doc" ? <SkeletonRow lines={2} /> : null}
      {bl.pha === "hong" ? <ErrorState body={bl.loi} onRetry={() => void napBl()} title="Chưa đọc được bình luận" /> : null}
      {bl.pha === "xong" && bl.danhSach.length === 0 ? (
        <EmptyState body="Ai đọc được bài này thì đều thấy bình luận ở đây." kind="first-use" layout="inline" illustration={<Canh id="chua-co-tin-nhan" width={150} />} title="Chưa có bình luận" />
      ) : null}
    </View>
  );

  const cuoi = (
    <View style={{ gap: space.sm, paddingBottom: space.lg }}>
      {bl.pha === "xong" && bl.conNua ? (
        <RudiButton disabled={ban} label="Tải thêm bình luận" onPress={() => void taiThem()} variant="outline" />
      ) : null}
      {loiViet ? <Text style={[typography.caption, { color: colors.warn }]}>{loiViet}</Text> : null}
      {bai.pha === "xong" ? (
        coTheBinhLuan(bai.bai) ? (
          <View style={styles.soan}>
            <View style={styles.oSoan}>
              <Field accessibilityLabel="Ô viết bình luận" onChangeText={setNhap} placeholder="Viết bình luận…" value={nhap} />
            </View>
            <IconButton accessibilityLabel="Gửi bình luận" disabled={ban || nhap.trim() === ""} icon="arrow-up" loading={ban} onPress={() => void guiBl()} solid />
          </View>
        ) : (
          <Text style={[typography.caption, { color: colors.inkFaint }]}>{CAU_KHONG_BINH_LUAN}</Text>
        )
      ) : null}
      {bai.pha === "xong" && bai.bai.author_id !== toi ? (
        <RudiButton icon="flag-outline" label="Báo cáo bài này" onPress={() => setBaoCaoMo(true)} variant="ghost" />
      ) : null}
    </View>
  );

  return (
    <RudiScreen scroll={false} testID="bai-chi-tiet-screen">
      <TopBar title="Bài đăng" />
      <FlatList
        ListFooterComponent={cuoi}
        ListHeaderComponent={dau}
        contentContainerStyle={{ gap: space.sm }}
        data={bl.pha === "xong" ? bl.danhSach : []}
        keyExtractor={(c) => c.id}
        keyboardShouldPersistTaps="handled"
        renderItem={({ item: c }) => (
          <Card style={styles.binhLuan}>
            <View style={styles.blDau}>
              <Text numberOfLines={1} style={[typography.label, { color: colors.ink, flex: 1 }]}>
                {c.author_id === toi ? "Bạn" : c.author_display_name}
              </Text>
              <Text style={[typography.caption, { color: colors.inkFaint }]}>{cauLucNao(c.created_at)}</Text>
              {bai.pha === "xong" && coTheXoaBinhLuan(c, bai.bai, toi) ? (
                <Pressable accessibilityLabel={`Xoá bình luận: ${c.body}`} accessibilityRole="button" disabled={ban} hitSlop={8} onPress={() => void xoaBl(c)}>
                  <Ionicons color={colors.inkFaint} name="trash-outline" size={18} />
                </Pressable>
              ) : null}
            </View>
            <Divider />
            <Text style={[typography.body, { color: colors.ink }]}>{c.body}</Text>
          </Card>
        )}
      />
      <Sheet accessibilityLabel="Báo cáo bài đăng" onClose={() => setBaoCaoMo(false)} open={baoCaoMo}>
        {bai.pha === "xong" ? (
          <NoiDungBaoCao
            actorId={toi}
            loai="post"
            onThoi={() => setBaoCaoMo(false)}
            onXong={() => setBaoCaoMo(false)}
            targetId={bai.bai.id}
          />
        ) : null}
      </Sheet>
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  tacGia: { flexDirection: "row", alignItems: "center", gap: 10 },
  tacGiaChu: { flex: 1, gap: 2 },
  chuDau: { width: 36, height: 36, borderRadius: 12, alignItems: "center", justifyContent: "center" },
  anh: { width: "100%", aspectRatio: 4 / 3 },
  anhHong: { alignItems: "center", justifyContent: "center" },
  phanUng: { flexDirection: "row", flexWrap: "wrap", gap: 8 },
  nutPhanUng: { borderWidth: 1, paddingHorizontal: 10, paddingVertical: 6 },
  binhLuan: { gap: 6 },
  blDau: { flexDirection: "row", alignItems: "center", gap: 8 },
  soan: { flexDirection: "row", alignItems: "flex-end", gap: 8 },
  oSoan: { flex: 1 },
});
