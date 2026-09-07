/**
 * Khám phá on a real session (M4): the server's catalogue, its categories,
 * saved places that live on the server, and a natural-language search that
 * Rủ Đi AI ranks. Typing filters by name at once; submitting asks the model.
 *
 * ## Places lead, the assistant stands beside the search (UI v2, đợt 4)
 *
 * The first cut opened with a violet card selling the AI and drew four
 * coloured category tiles before any place; the 2026-09-06 review read it as
 * a feature advert over a contact list. Here the page opens with where you
 * are and a search; the assistant is the sparkle button beside it and one
 * sentence under it. Categories are plain chips, only the chosen one tinted.
 * The first place is the lead (a licensed photo when the catalogue has one,
 * the category glyph when it does not); the rest are rows on the paper.
 * Changing a filter crossfades the results over `standard`; typing filters
 * without ceremony.
 */
import { useFocusEffect, useRouter } from "expo-router";
import { useCallback, useMemo, useState } from "react";
import { Pressable, ScrollView, StyleSheet, Text, View } from "react-native";
import Animated, { FadeIn, ReduceMotion } from "react-native-reanimated";
import { Ionicons } from "@expo/vector-icons";

import { ApiError, thongDiepNguoiDoc } from "../../../api";
import type { Phien } from "../../../phien";
import { matchLabel, type Category, type Place } from "../../../screens/kham-pha/places";
import { askSearch, hieuDuocGi, type TimKiemState } from "../../../screens/kham-pha/tim-kiem";
import { SO_THICH } from "../../../screens/vao-cua/so-thich";
import { docDiemDenDaChon } from "../../kham-pha/diem-den";
import {
  TIEN_TO_ANH,
  anhBiaThe,
  bieuTuongLoai,
  boLuuDiaDiem,
  cauChuaCo,
  cauGu,
  cauTimKiem,
  chiTietNgan,
  daoLuu,
  docDaLuu,
  docDanhMucCoLui,
  dongPhu,
  locTheoTen,
  luuDiaDiem,
  type Gu,
} from "../../kham-pha/dia-diem";
import { typography, useRudiTheme } from "../../theme";
import { Chip, IconButton, ResponsiveRow, RudiScreen, SearchField, SectionHeader } from "../../ui";
import { Wordmark } from "../../ui/Wordmark";
import { EmptyState } from "../../ui/EmptyState";
import { ErrorState } from "../../ui/ErrorState";
import { SkeletonCard, SkeletonGroup, SkeletonRow } from "../../ui/Skeleton";
import { useMotion } from "../../ui/useMotion";
import { PlaceLead, PlaceRow, type DiaDiemHienThi } from "./HangDiaDiem";

type Trang =
  | { pha: "dang-doc" }
  | { pha: "xong"; places: Place[]; categories: Category[] }
  | { pha: "hong"; loi: string };

const CAU_MAU = "quán nướng cho 6 người, 200k mỗi người";

function loiRaChu(error: unknown): string {
  return error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null);
}

/** Tapping the selected category clears the filter; tapping another selects it. */
function loaiSauBam(dangChon: boolean, id: string): string | null {
  if (dangChon) return null;
  return id;
}

function tenNhom(phien: Phien): string {
  const nhom = phien.contexts?.find((n) => n.id === phien.context_id);
  if (nhom === undefined) return "nhóm của bạn";
  return nhom.display_name;
}

/** The server's place, in the vocabulary the row and the lead draw. */
export function hienThiDiaDiem(place: Place): DiaDiemHienThi {
  const hop = matchLabel(place.match);
  const bia = anhBiaThe(place);
  return {
    id: place.id,
    name: place.name,
    sub: dongPhu(place),
    facts: chiTietNgan(place).map((m) => ({ icon: m.icon, text: m.chu })),
    glyph: bieuTuongLoai(place.category),
    // The picture comes with its credit or not at all (ADR-0017 §2.5).
    photo: bia === null ? null : bia.nguon,
    // «Quanh đây» travels with the credit: the importer geosearched within
    // 250 m, so the picture is from around here, not of this business.
    attribution: bia === null || place.photoAuthor === null || place.photoLicense === null ? undefined : { author: place.photoAuthor, license: place.photoLicense, prefix: TIEN_TO_ANH },
    badge: hop !== null && hop.real ? hop.text : null,
  };
}

export function ExploreLiveScreen({ phien }: { phien: Phien }) {
  const router = useRouter();
  const { colors } = useRudiTheme();
  const motion = useMotion();
  const [trang, setTrang] = useState<Trang>({ pha: "dang-doc" });
  const [daLuu, setDaLuu] = useState<string[]>([]);
  const [loiLuu, setLoiLuu] = useState<string | null>(null);
  const [loai, setLoai] = useState<string | null>(null);
  const [query, setQuery] = useState("");
  const [timKiem, setTimKiem] = useState<TimKiemState>({ kind: "chua-tim" });
  // Which city the list is of. The server always answers with one and says
  // which, so this starts as null and is filled from the answer -- the screen
  // never guesses a city name it has not been told.
  const [diemDen, setDiemDen] = useState<{ id: string; name: string } | null>(null);
  // Whose taste the badges are relative to. Starts as «chưa biết» because that
  // is true until the server has answered, and it is what the screen says.
  const [gu, setGu] = useState<Gu | null>(null);
  // Only words this build can name. A tag the server knows and this app does
  // not would otherwise print its storage key on screen, which is how «cafe»
  // becomes «mon-local» in front of somebody.
  const chuaCo = cauChuaCo(gu, (id) => SO_THICH.find((m) => m.id === id)?.nhan ?? "");

  const nap = useCallback(async () => {
    try {
      const daChon = await docDiemDenDaChon();
      const [danhMuc, luu] = await Promise.all([
        // A destination this phone remembers may be gone from the catalogue
        // (an import can drop one). That is a 404, and the right answer is the
        // server's default rather than an error screen about a city the person
        // chose last week; the stored choice is cleared so it stops asking.
        docDanhMucCoLui(daChon, phien.person_id),
        docDaLuu(phien.person_id),
      ]);
      setDiemDen(danhMuc.destination);
      setGu(danhMuc.gu);
      setTrang({ pha: "xong", places: danhMuc.places, categories: danhMuc.categories });
      setDaLuu(luu);
    } catch (error) {
      setTrang({ pha: "hong", loi: loiRaChu(error) });
    }
  }, [phien.person_id]);

  useFocusEffect(
    useCallback(() => {
      void nap();
    }, [nap]),
  );

  const doiLuu = async (place: Place) => {
    const truoc = daLuu;
    setDaLuu(daoLuu(daLuu, place.id));
    setLoiLuu(null);
    try {
      if (truoc.includes(place.id)) await boLuuDiaDiem(phien.person_id, place.id);
      else await luuDiaDiem(phien.person_id, place.id);
    } catch (error) {
      setDaLuu(truoc);
      setLoiLuu(loiRaChu(error));
    }
  };

  const hoi = async () => {
    const cau = query.trim();
    if (!cau) return;
    setTimKiem({ kind: "dang-tim", query: cau });
    setTimKiem(await askSearch(cau, { actorId: phien.person_id }));
  };

  const boTim = () => {
    setTimKiem({ kind: "chua-tim" });
    setQuery("");
    setLoai(null);
  };

  const danhSach = useMemo(() => {
    if (trang.pha !== "xong") return [];
    if (timKiem.kind === "co-ket-qua") return timKiem.places;
    const theoLoai = loai === null ? trang.places : trang.places.filter((p) => p.category === loai);
    return locTheoTen(theoLoai, query);
  }, [trang, timKiem, loai, query]);

  const dangLoc = loai !== null || query.trim().length > 0 || timKiem.kind === "co-ket-qua";
  const cauLoi = cauTimKiem(timKiem);
  // A filter change crossfades the results; a keystroke does not (it would
  // flicker on every letter). Reduce Motion cuts straight to the new list.
  const khoaKetQua = `${loai ?? ""}|${timKiem.kind === "co-ket-qua" ? timKiem.query : ""}`;
  const hienRa = FadeIn.duration(motion.ms("standard")).reduceMotion(ReduceMotion.System);
  // The lead is a photograph at reading size. A catalogue that has no picture
  // for its first place (a fresh server, no licensed photos yet) would open
  // on a screenful of empty frame, so without a photo nothing is promoted
  // and every place is a row (report §7.3: a placeholder must be honest,
  // not a stage).
  const daNhat = danhSach[0];
  const coAnhDan = daNhat !== undefined && anhBiaThe(daNhat) !== null;
  const dan = coAnhDan ? daNhat : undefined;
  const conLai = coAnhDan ? danhSach.slice(1) : danhSach;
  const rong = danhSach.length === 0;

  return (
    <RudiScreen bottomInset={112} onRefresh={nap} testID="explore-screen">
      <View style={styles.dau}>
        <Wordmark color={colors.ink} height={20} />
        {/* The destination is a control, not a caption. */}
        <Pressable
          accessibilityLabel="Đổi điểm đến"
          accessibilityRole="button"
          onPress={() => router.push("/destinations")}
          style={({ pressed }) => [styles.viTri, pressed && styles.bam]}
        >
          <Ionicons color={colors.accent} name="location" size={16} />
          <Text style={[typography.label, { color: colors.ink }]}>
            {diemDen !== null ? `${diemDen.name} · đổi nơi khác` : trang.pha === "hong" ? "Chưa đọc được điểm đến · thử lại" : "Đang đọc điểm đến…"}
          </Text>
          <Ionicons color={colors.inkFaint} name="chevron-down" size={14} />
        </Pressable>
      </View>
      <View style={styles.timRow}>
        <View style={styles.flex}>
          <SearchField
            accessibilityLabel="Ô tìm địa điểm"
            onChangeText={(t) => {
              setQuery(t);
              if (timKiem.kind !== "chua-tim") setTimKiem({ kind: "chua-tim" });
            }}
            onSubmitEditing={() => void hoi()}
            placeholder="Tìm quán, hỏi AI..."
            value={query}
          />
        </View>
        {/* The assistant stands beside the search, not above the places: one
            tap drops a sample question in so the person sees what to ask. */}
        <IconButton accessibilityLabel="Hỏi Rủ Đi AI" icon="sparkles" onPress={() => setQuery(CAU_MAU)} selected tone="ai" />
      </View>
      <Text style={[typography.caption, { color: colors.inkFaint }]}>
        Gõ tên để lọc ngay. Hỏi Rủ Đi AI một câu như «{CAU_MAU}» rồi bấm tìm: xếp theo gu {tenNhom(phien)}, ngân sách, số người và khoảng cách.
      </Text>
      {trang.pha === "dang-doc" ? (
        <SkeletonGroup style={styles.khung}>
          <SkeletonCard lines={1} media={200} />
          <SkeletonRow leading={56} />
          <SkeletonRow leading={56} />
          <SkeletonRow leading={56} />
        </SkeletonGroup>
      ) : null}
      {trang.pha === "hong" ? <ErrorState onRetry={() => void nap()} title="Chưa đọc được danh mục" /> : null}
      {trang.pha === "xong" ? (
        <>
          <ScrollView contentContainerStyle={styles.hangLoai} horizontal keyboardShouldPersistTaps="handled" showsHorizontalScrollIndicator={false} style={styles.cuonLoai}>
            {trang.categories.map((c) => {
              const chon = loai === c.id;
              return (
                <Chip
                  icon={bieuTuongLoai(c.id)}
                  key={c.id}
                  label={c.label}
                  onPress={() => setLoai(loaiSauBam(chon, c.id))}
                  selected={chon}
                />
              );
            })}
          </ScrollView>
          {timKiem.kind === "dang-tim" ? (
            <View style={[styles.theAi, { backgroundColor: colors.aiSoft }]}>
              <Text style={[typography.caption, { color: colors.ai }]}>Rủ Đi AI</Text>
              <Text style={[typography.body, { color: colors.ink }]}>Đang đọc câu «{timKiem.query}»...</Text>
            </View>
          ) : null}
          {cauLoi !== null ? (
            <View style={[styles.theAi, { backgroundColor: colors.aiSoft }]}>
              <Text style={[typography.caption, { color: colors.ai }]}>Rủ Đi AI</Text>
              <Text style={[typography.body, { color: colors.ink }]}>{cauLoi}</Text>
            </View>
          ) : null}
          {timKiem.kind === "co-ket-qua" ? (
            <View style={[styles.theAi, { backgroundColor: colors.aiSoft }]}>
              <Text style={[typography.caption, { color: colors.ai }]}>Rủ Đi AI hiểu câu «{timKiem.query}»</Text>
              {hieuDuocGi(timKiem.understood, trang.categories).map((d) => (
                <Text key={d.label} style={[typography.body, { color: colors.ink }]}>
                  {d.label}: {d.value}
                </Text>
              ))}
              {hieuDuocGi(timKiem.understood, trang.categories).length === 0 ? (
                <Text style={[typography.body, { color: colors.ink }]}>Chưa rút được ngân sách, số người hay khu vực; xếp theo gu chung.</Text>
              ) : null}
            </View>
          ) : null}
          {loiLuu !== null ? <Text accessibilityLiveRegion="polite" style={[typography.caption, { color: colors.warn }]}>{loiLuu}</Text> : null}
          <SectionHeader
            action={dangLoc ? "Xóa lọc" : undefined}
            onAction={dangLoc ? boTim : undefined}
            // The city comes from the answer, not from a string typed here:
            // this line used to say «Đà Lạt» over a list of anywhere.
            title={
              dangLoc
                ? `${danhSach.length} kết quả`
                : `${trang.places.length} nơi ở ${diemDen === null ? "đây" : diemDen.name}`
            }
          />
          {/* Whose taste the badges follow (M11). The «chưa biết» sentence is a
              button, because it is the one state the person can fix. */}
          {gu === null || gu.co_so === "chua-biet" ? (
            <Pressable
              accessibilityRole="button"
              onPress={() => router.push("/personalization" as never)}
              style={({ pressed }) => [styles.guRow, styles.guNut, pressed && styles.bam]}
            >
              <Text style={[typography.caption, { color: colors.inkFaint }]}>{cauGu(gu)}</Text>
              {chuaCo !== "" ? (
                <Text style={[typography.caption, { color: colors.inkFaint }]}>{chuaCo}</Text>
              ) : null}
            </Pressable>
          ) : (
            // Once the taste is known there is nothing to press: a disabled
            // Pressable still reads as a control to a screen reader.
            <View style={styles.guRow}>
              <Text style={[typography.caption, { color: colors.inkFaint }]}>{cauGu(gu)}</Text>
              {chuaCo !== "" ? (
                <Text style={[typography.caption, { color: colors.inkFaint }]}>{chuaCo}</Text>
              ) : null}
            </View>
          )}
          {rong ? (
            <EmptyState
              action={{ label: "Xóa lọc", onPress: boTim }}
              body="Thử từ khóa khác, hoặc bỏ bớt bộ lọc để thấy lại cả danh mục."
              kind={query.trim() ? "no-results" : "filtered"}
              layout="inline"
              title="Chưa thấy nơi phù hợp"
            />
          ) : (
            <Animated.View entering={hienRa} key={khoaKetQua} style={styles.ketQua}>
              {dan !== undefined ? (
                <PlaceLead
                  daLuu={daLuu.includes(dan.id)}
                  dd={hienThiDiaDiem(dan)}
                  onOpen={() => router.push(`/places/${dan.id}` as never)}
                  onSave={() => void doiLuu(dan)}
                />
              ) : null}
              {conLai.length > 0 ? (
                <ResponsiveRow gap={0} minItemWidth={300}>
                  {conLai.map((place) => (
                    <PlaceRow
                      daLuu={daLuu.includes(place.id)}
                      dd={hienThiDiaDiem(place)}
                      key={place.id}
                      onOpen={() => router.push(`/places/${place.id}` as never)}
                      onSave={() => void doiLuu(place)}
                    />
                  ))}
                </ResponsiveRow>
              ) : null}
            </Animated.View>
          )}
        </>
      ) : null}
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  flex: { flex: 1 },
  dau: { gap: 6 },
  viTri: { flexDirection: "row", alignItems: "center", gap: 6, minHeight: 48, alignSelf: "flex-start" },
  timRow: { flexDirection: "row", alignItems: "flex-end", gap: 8 },
  khung: { gap: 12 },
  cuonLoai: { marginHorizontal: -16 },
  hangLoai: { flexDirection: "row", gap: 8, paddingHorizontal: 16 },
  theAi: { gap: 4, padding: 14, borderRadius: 14 },
  guRow: { marginTop: -8, gap: 2 },
  // Only while it is a button does the sentence need a 48dp target.
  guNut: { minHeight: 48, justifyContent: "center", marginTop: -14 },
  bam: { opacity: 0.7 },
  ketQua: { gap: 20 },
});
