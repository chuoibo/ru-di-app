/**
 * The story viewer (L4, ADR-0022 §2.3): one author's live stories, full
 * screen over the indigo cover, five seconds each.
 *
 * Opens on the first story this reader has not seen. Each story is reported
 * to the server the moment it is shown (`POST /stories/{id}/seen`, once per
 * story per opening) so the rail on the way back already reads «đã xem».
 * Tapping the right half moves on, the left half moves back. The clock moves
 * between stories only: on the author's last story it fills the bar and
 * stops, and the reader closes the viewer by choice («Đóng story», Back, or
 * a tap on the right half). A viewer that shut itself the moment the clock
 * ran out took the last photo away mid-look. Under Reduce Motion the clock
 * does not run at all: the reader moves by tapping.
 *
 * `RudiScreen` lays paper over every surface and expects a `CoverBand` to
 * paint the indigo; a full-cover screen paints it itself through
 * `contentStyle`, or every `coverInk` word is cream on cream.
 *
 * The photo is fetched through the one personal-photo gate with this reader's
 * bearer (`nguonAnhBai`); the server, not this screen, decides whether the
 * bytes come back.
 */
import { Ionicons } from "@expo/vector-icons";
import { Image } from "expo-image";
import { useLocalSearchParams, useRouter } from "expo-router";
import { useCallback, useEffect, useRef, useState } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";

import { ApiError, thongDiepNguoiDoc } from "../../../api";
import { nguonAnhBai } from "../../nguoi/anh-ca-nhan";
import { useRudiSession } from "../../session";
import {
  THOI_LUONG_MS,
  cauTuoi,
  chiSoBatDau,
  conHan,
  danhDauDaXem,
  docStories,
  laCuaToi,
  nhomCua,
  xoaStory,
  type NhomStoryWire,
} from "../../story/story";
import { typography, useRudiTheme } from "../../theme";
import { EmptyState } from "../../ui/EmptyState";
import { ErrorState } from "../../ui/ErrorState";
import { RudiScreen } from "../../ui";
import { useMotion } from "../../ui/useMotion";

type Trang =
  | { pha: "dang-doc" }
  | { pha: "xong"; nhom: NhomStoryWire }
  | { pha: "trong" }
  | { pha: "hong"; loi: string };

const NHIP_MS = 100;

export function XemStoryScreen() {
  const router = useRouter();
  const { personId } = useLocalSearchParams<{ personId: string }>();
  // Statement form on purpose (see the id-default gate): the route param is a
  // selection the rail made, never a label, and an array param is not one.
  let authorId = "";
  if (typeof personId === "string") authorId = personId;
  const { colors } = useRudiTheme();
  const { reduced } = useMotion();
  const { phien, phienDaDoc } = useRudiSession();
  const toi = phien?.person_id ?? "";
  const [trang, setTrang] = useState<Trang>({ pha: "dang-doc" });
  const [chiSo, setChiSo] = useState(0);
  const [batDau, setBatDau] = useState(() => Date.now());
  const [phan, setPhan] = useState(0);
  const [hoiXoa, setHoiXoa] = useState(false);
  const [dangXoa, setDangXoa] = useState(false);
  // The clock has run out on the last story: the bar stays full, nothing moves.
  const [het, setHet] = useState(false);
  const daBao = useRef<Set<string>>(new Set());

  const nap = useCallback(async () => {
    if (toi === "") return;
    try {
      const dai = await docStories(toi);
      const nhom = nhomCua(dai, authorId);
      if (nhom === null || nhom.stories.length === 0) {
        setTrang({ pha: "trong" });
        return;
      }
      setChiSo(chiSoBatDau(nhom));
      setBatDau(Date.now());
      setHet(false);
      setTrang({ pha: "xong", nhom });
    } catch (error) {
      setTrang({ pha: "hong", loi: error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null) });
    }
  }, [toi, authorId]);

  useEffect(() => {
    void nap();
  }, [nap]);

  const nhom = trang.pha === "xong" ? trang.nhom : null;
  const story = nhom === null ? null : (nhom.stories[chiSo] ?? null);
  const cuaToi = nhom !== null && laCuaToi(nhom, toi);

  // Report «seen» once per story per opening. Fire and forget: a failed report
  // costs nothing but the ring colour on the way back.
  useEffect(() => {
    if (story === null || toi === "" || daBao.current.has(story.id)) return;
    daBao.current.add(story.id);
    void danhDauDaXem(story.id, toi).catch(() => undefined);
  }, [story, toi]);

  const tiep = useCallback(
    (tuNguoi: boolean) => {
      if (nhom === null) return;
      if (chiSo + 1 >= nhom.stories.length) {
        // Last story: the clock stops here; only the reader's own tap closes.
        if (tuNguoi) router.back();
        else {
          setPhan(1);
          setHet(true);
        }
        return;
      }
      setChiSo(chiSo + 1);
      setBatDau(Date.now());
      setPhan(0);
      setHet(false);
    },
    [nhom, chiSo, router],
  );

  const lui = () => {
    if (chiSo === 0) return;
    setChiSo(chiSo - 1);
    setBatDau(Date.now());
    setPhan(0);
    setHet(false);
  };

  // The clock. Not under Reduce Motion, not while the delete question is up.
  useEffect(() => {
    if (story === null || reduced || hoiXoa || het) return;
    const id = setInterval(() => {
      const now = Date.now();
      if (!conHan(story.expires_at, now)) {
        tiep(false);
        return;
      }
      const p = (now - batDau) / THOI_LUONG_MS;
      if (p >= 1) tiep(false);
      else setPhan(p);
    }, NHIP_MS);
    return () => clearInterval(id);
  }, [story, batDau, reduced, hoiXoa, het, tiep]);

  const xoa = async () => {
    if (story === null || dangXoa) return;
    setDangXoa(true);
    try {
      await xoaStory(story.id, toi);
      router.back();
    } catch (error) {
      setTrang({ pha: "hong", loi: error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null) });
    } finally {
      setDangXoa(false);
    }
  };

  if (!phienDaDoc) return null;

  return (
    <RudiScreen
      contentStyle={{ backgroundColor: colors.cover }}
      padded={false}
      scroll={false}
      surface="cover"
      testID="xem-story-screen"
    >
      {trang.pha === "dang-doc" ? (
        <View style={styles.giua}>
          <Text style={[typography.caption, { color: colors.coverInkSoft }]}>Đang mở story…</Text>
        </View>
      ) : null}
      {trang.pha === "trong" ? (
        <View style={[styles.giua, { backgroundColor: colors.ground }]}>
          <EmptyState
            action={{ label: "Quay lại", onPress: () => router.back() }}
            body="Có thể story đã qua 24 giờ, hoặc người đăng đã gỡ."
            kind="no-results"
            layout="inline"
            title="Không còn story nào"
          />
        </View>
      ) : null}
      {trang.pha === "hong" ? (
        <View style={[styles.giua, { backgroundColor: colors.ground }]}>
          <ErrorState body={trang.loi} onRetry={() => void nap()} title="Chưa mở được story" />
        </View>
      ) : null}
      {nhom !== null && story !== null ? (
        <View style={styles.than}>
          <View style={styles.dau}>
            <View style={styles.tienDo}>
              {nhom.stories.map((s, i) => {
                // Filled share of this segment as flex weights, not a percent
                // string: ADR-0009 keeps every «%» out of the source a reader
                // could see, and a bar needs none.
                const day = i < chiSo ? 1 : i === chiSo ? phan : 0;
                return (
                  <View key={s.id} style={[styles.doan, { backgroundColor: colors.coverLine }]}>
                    <View style={[styles.doanDay, { backgroundColor: colors.coverInk, flex: day }]} />
                    <View style={{ flex: 1 - day }} />
                  </View>
                );
              })}
            </View>
            <View style={styles.hangTen}>
              <View style={styles.ten}>
                <Text style={[typography.label, { color: colors.coverInk }]}>{cuaToi ? "Story của bạn" : nhom.author.display_name}</Text>
                <Text style={[typography.caption, { color: colors.coverInkSoft }]}>{cauTuoi(story.created_at, Date.now())}</Text>
              </View>
              {cuaToi ? (
                <Pressable accessibilityLabel="Xoá story" accessibilityRole="button" hitSlop={8} onPress={() => setHoiXoa(true)} style={styles.nutTron}>
                  <Ionicons color={colors.coverInk} name="trash-outline" size={22} />
                </Pressable>
              ) : null}
              <Pressable accessibilityLabel="Đóng story" accessibilityRole="button" hitSlop={8} onPress={() => router.back()} style={styles.nutTron}>
                <Ionicons color={colors.coverInk} name="close" size={26} />
              </Pressable>
            </View>
          </View>
          <View style={styles.khungAnh}>
            <Image
              accessibilityLabel="Ảnh story"
              contentFit="contain"
              source={nguonAnhBai(story.image_url, toi)}
              style={styles.anh}
            />
            <Pressable accessibilityLabel="Story trước" onPress={lui} style={[styles.vungCham, styles.vungTrai]} />
            <Pressable accessibilityLabel="Story tiếp theo" onPress={() => tiep(true)} style={[styles.vungCham, styles.vungPhai]} />
          </View>
          {story.caption ? (
            <Text style={[typography.body, styles.chuThich, { color: colors.coverInk }]}>{story.caption}</Text>
          ) : null}
          {hoiXoa ? (
            <View style={[styles.hopXoa, { backgroundColor: colors.card, borderColor: colors.line }]}>
              <Text style={[typography.body, { color: colors.ink }]}>Xoá story này? Bạn bè sẽ không thấy nó nữa.</Text>
              <View style={styles.hangNut}>
                <Pressable accessibilityLabel="Giữ lại" accessibilityRole="button" disabled={dangXoa} onPress={() => setHoiXoa(false)} style={styles.nutChu}>
                  <Text style={[typography.label, { color: colors.ink }]}>Giữ lại</Text>
                </Pressable>
                <Pressable accessibilityLabel="Xoá" accessibilityRole="button" disabled={dangXoa} onPress={() => void xoa()} style={styles.nutChu}>
                  <Text style={[typography.label, { color: colors.warn }]}>{dangXoa ? "Đang xoá…" : "Xoá"}</Text>
                </Pressable>
              </View>
            </View>
          ) : null}
        </View>
      ) : null}
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  giua: { flex: 1, alignItems: "center", justifyContent: "center", padding: 24 },
  than: { flex: 1, paddingTop: 48, paddingBottom: 24 },
  dau: { paddingHorizontal: 12, gap: 8 },
  tienDo: { flexDirection: "row", gap: 4 },
  doan: { flex: 1, height: 3, borderRadius: 2, overflow: "hidden", flexDirection: "row" },
  doanDay: { height: 3 },
  hangTen: { flexDirection: "row", alignItems: "center", gap: 8 },
  ten: { flex: 1, gap: 2 },
  nutTron: { width: 44, height: 44, alignItems: "center", justifyContent: "center" },
  khungAnh: { flex: 1, marginTop: 8 },
  anh: { width: "100%", height: "100%" },
  vungCham: { position: "absolute", top: 0, bottom: 0 },
  vungTrai: { left: 0, width: "35%" },
  vungPhai: { right: 0, width: "65%" },
  chuThich: { paddingHorizontal: 20, paddingTop: 12, textAlign: "center" },
  hopXoa: { margin: 16, padding: 16, borderRadius: 16, borderWidth: 1, gap: 12 },
  hangNut: { flexDirection: "row", justifyContent: "flex-end", gap: 8 },
  nutChu: { minHeight: 44, minWidth: 88, alignItems: "center", justifyContent: "center", paddingHorizontal: 12 },
});
