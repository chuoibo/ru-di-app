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
 *
 * 2026-09-08 (report 07/09 §9.4, §4.3): the paragraph under the search that
 * explained filtering, the assistant, taste, budget, headcount and distance in
 * one breath is gone; the placeholder and the sparkle button carry it. The
 * results change rhythm: one lead (only with a photo), then **two candidates
 * side by side** to compare, then the rest as rows. Frames without a photo
 * show the category drawn by the art layer.
 */
import { useFocusEffect, useRouter } from "expo-router";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Pressable, ScrollView, StyleSheet, Text, View, type TextInput } from "react-native";
import Animated, { FadeIn } from "react-native-reanimated";
import { Ionicons } from "@expo/vector-icons";

import { ApiError, thongDiepNguoiDoc } from "../../../api";
import type { Phien } from "../../../phien";
import { matchLabel, type Category, type Place } from "../../../screens/kham-pha/places";
import { askSearch, hieuDuocGi, type TimKiemState } from "../../../screens/kham-pha/tim-kiem";
import { SO_THICH } from "../../../screens/vao-cua/so-thich";
import { docDiemDenDaChon } from "../../kham-pha/diem-den";
import {
  canDocLaiDanhMuc,
  anhBiaThe,
  cauXemThem,
  HANG_MOI_LUOT,
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
  guTheoLoai,
  locTheoTen,
  luuDiaDiem,
  nenLocTheoTen,
  type Gu,
} from "../../kham-pha/dia-diem";
import { typography, useRudiTheme } from "../../theme";
import { Chip, IconButton, ResponsiveRow, RudiButton, RudiScreen, SearchField, SectionHeader } from "../../ui";
import type { DungDau } from "../../ui/DauKhamPha";
import { SanThanhPho } from "../../ui/SanThanhPho";
import { Canh } from "../../ui/art/Canh";
import { GuGlyph } from "../../ui/art/Gu";
import { EmptyState } from "../../ui/EmptyState";
import { ErrorState } from "../../ui/ErrorState";
import { SkeletonCard, SkeletonGroup, SkeletonRow } from "../../ui/Skeleton";
import { useMotion } from "../../ui/useMotion";
import { CauTaiCho } from "../../ui/CauTaiCho";
import { PlaceCompare, PlaceLead, PlaceRow, taiSoSanh, type DiaDiemHienThi } from "./HangDiaDiem";

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
    // Category only: the live catalogue carries no per-place tags, so nothing
    // honest can tell two eateries' slots apart here (the fixture's `gu` comes
    // from its tags). Three bowls in a row are the truth of this data.
    loai: place.category,
    // …and so the lead's sketch draws the stage only: no tag, no prop.
    tags: [],
    // The picture comes with its credit or not at all (ADR-0017 §2.5), in one
    // value whose address cannot be taken out on its own, so no adapter can
    // hand the frame the picture alone. «Quanh đây» travels with the credit:
    // the importer geosearched within 250 m, so the picture is from around
    // here, not of this business.
    anh: bia,
    badge: hop !== null && hop.real ? hop.text : null,
    // One grounded reason under the lead: the model's own sentence when the
    // match is real, nothing otherwise. Never the tagline dressed as a reason.
    lyDo: hop !== null && hop.real && place.match?.reason ? place.match.reason : undefined,
  };
}

export function ExploreLiveScreen({ phien, dau }: { phien: Phien; dau?: DungDau }) {
  const router = useRouter();
  const { colors } = useRudiTheme();
  // Large text: the search box takes the whole line and the assistant button
  // drops under it. The placeholder here is thirty characters; it draws itself
  // on one line now (F44), and the full width is what keeps most of it legible.
  const motion = useMotion();
  const [trang, setTrang] = useState<Trang>({ pha: "dang-doc" });
  const [daLuu, setDaLuu] = useState<string[]>([]);
  const [loiLuu, setLoiLuu] = useState<string | null>(null);
  const [loai, setLoai] = useState<string | null>(null);
  const [query, setQuery] = useState("");
  const [timKiem, setTimKiem] = useState<TimKiemState>({ kind: "chua-tim" });
  // The field holds a question for Rủ Đi AI that is not sent yet (the sample
  // ✦ dropped in, being edited): a question is not a name, so it does not
  // filter the list by name. It did, and «0 kết quả» came before any answer
  // (QA UI-024).
  const [choGui, setChoGui] = useState(false);
  const oTim = useRef<TextInput | null>(null);
  // A reload that fails while a list is on screen keeps the list and says so
  // in one line; only a first read that fails is the error screen (QA UI-030).
  const [loiNapLai, setLoiNapLai] = useState<string | null>(null);
  // Which city the list is of. The server always answers with one and says
  // which, so this starts as null and is filled from the answer -- the screen
  // never guesses a city name it has not been told.
  const [diemDen, setDiemDen] = useState<{ id: string; name: string } | null>(null);
  // The destination the list on screen was read for, readable inside `nap`
  // without making the read depend on it (it would re-run on its own answer).
  const dangHien = useRef<string | null>(null);
  // When, and for which destination, the catalogue was last read.
  const lanDoc = useRef<{ diemDen: string | null; luc: number } | null>(null);
  const [soHang, setSoHang] = useState(HANG_MOI_LUOT);
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
      if (!canDocLaiDanhMuc(lanDoc.current, daChon, Date.now())) {
        // Only the bookmarks can have changed on the way back (a save on the
        // detail screen), and they are a small read.
        setDaLuu(await docDaLuu(phien.person_id));
        return;
      }
      // A different city was chosen: the old list must not stand under the
      // new name while a large catalogue loads (seconds, on real data).
      if (daChon !== null && dangHien.current !== null && daChon !== dangHien.current) {
        setTrang({ pha: "dang-doc" });
      }
      const [danhMuc, luu] = await Promise.all([
        // A destination this phone remembers may be gone from the catalogue
        // (an import can drop one). That is a 404, and the right answer is the
        // server's default rather than an error screen about a city the person
        // chose last week; the stored choice is cleared so it stops asking.
        docDanhMucCoLui(daChon, phien.person_id),
        docDaLuu(phien.person_id),
      ]);
      // The id that was asked for, not the one the server answered with: after
      // a fallback the two differ for good, and comparing against the answer
      // would blank the list to a skeleton on every return to this tab.
      dangHien.current = daChon ?? danhMuc.destination?.id ?? null;
      lanDoc.current = { diemDen: dangHien.current, luc: Date.now() };
      setDiemDen(danhMuc.destination);
      setGu(danhMuc.gu);
      setTrang({ pha: "xong", places: danhMuc.places, categories: danhMuc.categories });
      setDaLuu(luu);
      setLoiNapLai(null);
    } catch (error) {
      const loi = loiRaChu(error);
      setTrang((t) => (t.pha === "xong" ? t : { pha: "hong", loi }));
      setLoiNapLai(loi);
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
    setChoGui(false);
    setTimKiem({ kind: "dang-tim", query: cau });
    setTimKiem(await askSearch(cau, { actorId: phien.person_id, destination: diemDen?.id ?? null }));
  };

  // ✦ asks: with a sentence in the field it sends it; on an empty field it
  // lays a sample question down to edit and puts the cursor there. It used to
  // only fill the field, and the field then filtered by name (QA UI-024).
  const bamHoi = () => {
    if (query.trim()) {
      void hoi();
      return;
    }
    setQuery(CAU_MAU);
    setChoGui(true);
    setTimKiem({ kind: "chua-tim" });
    oTim.current?.focus();
  };

  const boTim = () => {
    setTimKiem({ kind: "chua-tim" });
    setQuery("");
    setChoGui(false);
    setLoai(null);
  };

  // The field filters by name only while it holds a name: not while a question
  // waits to be sent, and not once one has been asked (its words are not a
  // place's name, whatever the answer was).
  const locTen = nenLocTheoTen(query, timKiem.kind, choGui);
  const danhSach = useMemo(() => {
    if (trang.pha !== "xong") return [];
    if (timKiem.kind === "co-ket-qua") return timKiem.places;
    const theoLoai = loai === null ? trang.places : trang.places.filter((p) => p.category === loai);
    return locTen ? locTheoTen(theoLoai, query) : theoLoai;
  }, [trang, timKiem, loai, query, locTen]);

  const dangLoc = loai !== null || locTen || timKiem.kind === "co-ket-qua";
  // A search session (a name, a question, an answer) keeps the top of the
  // screen for the results: the city's stage folds away.
  const gapSan = dangLoc || query !== "";
  const cauLoi = cauTimKiem(timKiem);
  // A filter change crossfades the results; a keystroke does not (it would
  // flicker on every letter). Reduce Motion cuts straight to the new list.
  const khoaKetQua = `${loai ?? ""}|${timKiem.kind === "co-ket-qua" ? timKiem.query : ""}`;
  // A new list starts from its first step again.
  useEffect(() => {
    setSoHang(HANG_MOI_LUOT);
  }, [khoaKetQua, query, diemDen?.id]);
  const hienRa = FadeIn.duration(motion.ms("standard")).reduceMotion(motion.reanimated);
  // The lead is a photograph at reading size. A catalogue that has no picture
  // for its first place (a fresh server, no licensed photos yet) would open
  // on a screenful of empty frame, so without a photo nothing is promoted
  // and every place is a row (report §7.3: a placeholder must be honest,
  // not a stage).
  const daNhat = danhSach[0];
  const coAnhDan = daNhat !== undefined && anhBiaThe(daNhat) !== null;
  const dan = coAnhDan ? daNhat : undefined;
  // Two candidates to compare before the list: the same axis for both, so the
  // choice starts as a comparison (`taiSoSanh`).
  const { soSanh, hang: conLai } = taiSoSanh(coAnhDan ? danhSach.slice(1) : danhSach);
  const rong = danhSach.length === 0;
  // An empty list for a city with nothing in it is not an empty search: it
  // must not advise dropping a filter that is not there (QA UI-028).
  const thanhPhoRong = rong && !dangLoc && trang.pha === "xong" && trang.places.length === 0;
  // A name search that found nothing but reads like a question is handed to
  // Rủ Đi AI rather than left at «0 kết quả».
  const giongCauHoi = locTen && query.trim().split(/\s+/).length >= 3;


  return (
    <RudiScreen bottomInset="tab" header={dau?.()} onRefresh={nap} testID="explore-screen">
      <View style={styles.dau}>
        {/* The destination is a control, not a caption: the city in ink, the
            way to change it in the invitation's coral (owner's mockup). */}
        <Pressable
          accessibilityLabel="Đổi điểm đến"
          accessibilityRole="button"
          onPress={() => router.push("/destinations")}
          style={({ pressed }) => [styles.viTri, pressed && styles.bam]}
        >
          <Ionicons color={colors.accent} name="location" size={16} />
          {diemDen !== null ? (
            <Text style={[typography.label, { color: colors.ink }]}>
              {diemDen.name}
              <Text style={{ color: colors.accent }}> · đổi nơi khác</Text>
            </Text>
          ) : (
            <Text style={[typography.label, { color: colors.ink }]}>
              {trang.pha === "hong" ? "Chưa đọc được điểm đến · thử lại" : "Đang đọc điểm đến…"}
            </Text>
          )}
          <Ionicons color={diemDen !== null ? colors.accent : colors.inkFaint} name="chevron-down" size={14} />
        </Pressable>
      </View>
      {/* The city itself, as a pop-up stage (ADR-0037 D1, plan S4): drawn from
          what the server says about the place. It stands up once per city and
          folds away while a search or filter is under way, so the results
          keep the top of the screen. */}
      {diemDen !== null ? (
        <SanThanhPho gap={gapSan} id={diemDen.id} ten={diemDen.name} />
      ) : null}
      {/* One row at every font size: the field's own hint ellipsizes, so the
          assistant no longer drops to a line of its own at 1.3 (QA 23/09). */}
      <View style={styles.timRow}>
        <View style={styles.flex}>
          <SearchField
            accessibilityLabel="Ô tìm địa điểm"
            oRef={oTim}
            onChangeText={(t) => {
              setQuery(t);
              if (t.trim() === "") setChoGui(false);
              if (timKiem.kind !== "chua-tim") setTimKiem({ kind: "chua-tim" });
            }}
            onSubmitEditing={() => void hoi()}
            placeholder="Một món thèm, một nơi muốn ghé…"
            value={query}
          />
        </View>
        {/* The assistant stands beside the search, not above the places: one
            tap drops a sample question in so the person sees what to ask. */}
        <IconButton accessibilityLabel="Hỏi Rủ Đi AI" icon="sparkles" onPress={bamHoi} selected tone="ai" />
      </View>
      {choGui ? (
        <Text style={[typography.caption, { color: colors.ai }]}>Sửa câu cho đúng ý bạn, rồi chạm ✦ hoặc Enter để hỏi Rủ Đi AI.</Text>
      ) : null}
      <CauTaiCho cau={loiNapLai !== null && trang.pha === "xong" ? `Chưa cập nhật được danh mục: ${loiNapLai}` : null} />
      {trang.pha === "dang-doc" ? (
        <SkeletonGroup style={styles.khung}>
          <SkeletonCard lines={1} media={200} />
          <SkeletonRow leading={56} />
          <SkeletonRow leading={56} />
          <SkeletonRow leading={56} />
        </SkeletonGroup>
      ) : null}
      {trang.pha === "hong" ? <ErrorState body={trang.loi} onRetry={() => void nap()} title="Những chỗ hay chưa hiện lên" /> : null}
      {trang.pha === "xong" ? (
        <>
          <ScrollView contentContainerStyle={styles.hangLoai} horizontal keyboardShouldPersistTaps="handled" showsHorizontalScrollIndicator={false} style={styles.cuonLoai}>
            {trang.categories.map((c) => {
              const chon = loai === c.id;
              return (
                <Chip
                  key={c.id}
                  label={c.label}
                  leading={<GuGlyph id={guTheoLoai(c.id)} size={22} tone={chon ? "accent" : "ink"} />}
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
            action={dangLoc || query !== "" ? "Xóa lọc" : undefined}
            onAction={dangLoc || query !== "" ? boTim : undefined}
            // The city comes from the answer, not from a string typed here:
            // this line used to say «Đà Lạt» over a list of anywhere. Not
            // the mockup's near-you-and-to-taste heading: the catalogue comes in
            // the server's order (ORDER BY places.id), not by distance or
            // taste, so the heading claims neither; the count moves under it.
            title={dangLoc ? `${danhSach.length.toLocaleString("vi-VN")} kết quả` : `Chỗ hay ở ${diemDen === null ? "đây" : diemDen.name}`}
          />
          {!dangLoc ? (
            <Text style={[typography.caption, styles.soNoi, { color: colors.inkFaint }]}>{`${trang.places.length.toLocaleString("vi-VN")} nơi`}</Text>
          ) : null}
          {/* Whose taste the badges follow (M11). The «chưa biết» sentence is a
              button, because it is the one state the person can fix. */}
          {/* Whose taste a list follows is nothing to say over a city with no places. */}
          {thanhPhoRong ? null : gu === null || gu.co_so === "chua-biet" ? (
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
          {thanhPhoRong ? (
            <EmptyState
              action={{ label: "Đổi điểm đến", onPress: () => router.push("/destinations") }}
              body="Danh mục của Rủ Đi chưa có chỗ nào ở đây. Chọn một điểm đến khác để xem chỗ hay."
              illustration={<Canh id="tim-khong-ra" width={168} />}
              kind="no-results"
              layout="inline"
              title={`${diemDen?.name ?? "Nơi này"} chưa có địa điểm nào`}
            />
          ) : rong && giongCauHoi ? (
            <EmptyState
              action={{ label: "Hỏi Rủ Đi AI", onPress: () => void hoi() }}
              body="Câu này giống một câu hỏi hơn một cái tên. Để Rủ Đi AI tìm theo ý câu này."
              illustration={<Canh id="tim-khong-ra" width={168} />}
              kind="no-results"
              layout="inline"
              secondary={{ label: "Xóa lọc", onPress: boTim }}
              title={`Không có tên nào khớp «${query.trim()}»`}
            />
          ) : rong ? (
            <EmptyState
              action={{ label: "Xóa lọc", onPress: boTim }}
              body="Thử từ khóa khác, hoặc bỏ bớt bộ lọc để thấy lại cả danh mục."
              illustration={<Canh id="tim-khong-ra" width={168} />}
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
              {soSanh !== null ? (
                <PlaceCompare
                  daLuu={(id) => daLuu.includes(id)}
                  items={[hienThiDiaDiem(soSanh[0]), hienThiDiaDiem(soSanh[1])]}
                  onOpen={(id) => router.push(`/places/${id}` as never)}
                  onSave={(id) => {
                    const place = soSanh.find((p) => p.id === id);
                    if (place !== undefined) void doiLuu(place);
                  }}
                  testID="explore-compare"
                />
              ) : null}
              {conLai.length > 0 ? (
                <ResponsiveRow gap={0} minItemWidth={300}>
                  {conLai.slice(0, soHang).map((place) => (
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
              {conLai.length > soHang ? (
                <RudiButton
                  label={cauXemThem(conLai.length - soHang)}
                  onPress={() => setSoHang((n) => n + HANG_MOI_LUOT)}
                  variant="outline"
                />
              ) : null}
            </Animated.View>
          )}
        </>
      ) : null}
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  // Close under its heading, as one block (the column's gap is 18).
  soNoi: { marginTop: -14 },
  flex: { flex: 1 },
  // Above the city stage, whose sky rises under this line (SanThanhPho).
  dau: { gap: 6, zIndex: 1 },
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
