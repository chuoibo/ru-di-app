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
import { Pressable, ScrollView, StyleSheet, View, useWindowDimensions, type TextInput } from "react-native";
import Animated, { FadeIn } from "react-native-reanimated";
import { Ionicons } from "@expo/vector-icons";

import { ApiError, thongDiepNguoiDoc } from "../../../api";
import type { Phien } from "../../../phien";
import { matchLabel, type Category, type Place } from "../../../screens/kham-pha/places";
import { askSearch, hieuDuocGi, type TimKiemState } from "../../../screens/kham-pha/tim-kiem";
import { SO_THICH } from "../../../screens/vao-cua/so-thich";
import { docBiaDiemDen, docDiemDenDaChon } from "../../kham-pha/diem-den";
import { canhDiemDen, phanCanhDanhSach, vungCua, type TenCanh } from "../../kham-pha/canh-ve";
import { KHUNG_THANH_PHO, hinhThanhPho } from "../../art/thanh-pho";
import { VeLop } from "../../ui/art/VeLop";
import { MediaSlot } from "../../ui/MediaSlot";
import type { AnhCoGhiCong } from "../../ui/ghi-cong";
import {
  canDocLaiDanhMuc,
  anhBiaThe,
  cauXemThem,
  docDanhMuc,
  docLyDo,
  voiLyDo,
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
import { CommunityFontMetrics } from "../../community/Comments";
import { Text, useNativeStandaloneTextMetrics } from "../../ui/NativeFontMetrics";
import { Chip, IconButton, ResponsiveRow, RudiButton, RudiScreen, SearchField } from "../../ui";
import type { DungDau } from "../../ui/DauKhamPha";
import { AssetIllustration } from "../../asset-pack/react-native/AssetIllustration";
import { Canh } from "../../ui/art/Canh";
import { GuGlyph } from "../../ui/art/Gu";
import { EmptyState } from "../../ui/EmptyState";
import { ErrorState } from "../../ui/ErrorState";
import { SkeletonCard, SkeletonGroup, SkeletonRow } from "../../ui/Skeleton";
import { useMotion } from "../../ui/useMotion";
import { CauTaiCho } from "../../ui/CauTaiCho";
import { PlaceCompare, PlaceLead, PlaceRow, taiSoSanh, type DiaDiemHienThi } from "./HangDiaDiem";

/**
 * The city stage under the destination line: the destination's real cover
 * with its credit when the catalogue has one (finish review 21/10: the drawn
 * lake stood in front of a real photograph), else its own painted scene where
 * one paints that region, else its line sketch. Never Đà Lạt's lake for Hà Nội.
 */
function SanThanhPho({ anh, id, ten, rong, cao }: { anh: AnhCoGhiCong | null; id: string; ten: string; rong: number; cao: number }) {
  const canh = canhDiemDen(id);
  const ve = canh !== null ? (
    <AssetIllustration name={canh} width={rong} height={cao} />
  ) : (
    <View style={[styles.sanPhac, { width: rong, height: cao }]}>
      <VeLop height={cao} khungH={KHUNG_THANH_PHO.h} khungW={KHUNG_THANH_PHO.w} lop={hinhThanhPho(id)} width={Math.min(rong, Math.round((cao * KHUNG_THANH_PHO.w) / KHUNG_THANH_PHO.h))} />
    </View>
  );
  return (
    <View style={[styles.buuThiep, { width: rong }]} testID="explore-city-stage">
      {anh === null ? ve : <MediaSlot alt={`Ảnh ${ten}`} fallback={ve} height={cao} nguon={{ loai: "danh-muc", anh }} radius={8} width={rong} />}
    </View>
  );
}

type Trang =
  | { pha: "dang-doc" }
  | { pha: "xong"; places: Place[]; categories: Category[]; tiep: number | null; total: number | null }
  | { pha: "hong"; loi: string };

/** A filtered read of the same destination (a category, or a name), paged like the base list. */
type Loc = { khoa: string; places: Place[]; tiep: number | null; total: number | null };

/**
 * Stale-while-revalidate, per person and destination: switching back to a
 * destination shows what was read last at once, and the read behind it
 * replaces it. Lives for the session, not on disk.
 */
type BoNho = { places: Place[]; categories: Category[]; tiep: number | null; total: number | null; destination: { id: string; name: string }; gu: Gu };
const boNhoDanhMuc = new Map<string, BoNho>();

/** Places of a later page after the ones already held, without repeats. */
function noiTiep(co: readonly Place[], them: readonly Place[]): Place[] {
  const da = new Set(co.map((p) => p.id));
  return [...co, ...them.filter((p) => !da.has(p.id))];
}

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
export function hienThiDiaDiem(place: Place, canh?: TenCanh | null): DiaDiemHienThi {
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
    // The list picks the painted stand-in (region-true, neighbours varied); alone, the card's own default.
    ...(canh === undefined ? {} : { canh }),
  };
}

export function ExploreLiveScreen(props: { phien: Phien; dau?: DungDau }) {
  return <CommunityFontMetrics><ExploreLiveContent {...props} /></CommunityFontMetrics>;
}

function ExploreLiveContent({ phien, dau }: { phien: Phien; dau?: DungDau }) {
  const router = useRouter();
  const { colors } = useRudiTheme();
  const bodyMetrics = useNativeStandaloneTextMetrics(typography.body);
  // Unusable native metrics keep a generous layout without inventing font scale.
  const largeText = bodyMetrics === null || (bodyMetrics.fontSize ?? typography.body.fontSize) >= typography.body.fontSize * 1.25;
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
  // The destination's real cover (the picker's own read), keyed by the city it is of.
  const [biaDiemDen, setBiaDiemDen] = useState<{ id: string; anh: AnhCoGhiCong | null } | null>(null);
  const idDiemDen = diemDen?.id ?? null;
  useEffect(() => {
    if (idDiemDen === null) return;
    let con = true;
    docBiaDiemDen(idDiemDen)
      .then((anh) => { if (con) setBiaDiemDen({ id: idDiemDen, anh }); })
      .catch(() => { if (con) setBiaDiemDen({ id: idDiemDen, anh: null }); });
    return () => { con = false; };
  }, [idDiemDen]);
  // The destination the list on screen was read for, readable inside `nap`
  // without making the read depend on it (it would re-run on its own answer).
  const dangHien = useRef<string | null>(null);
  // When, and for which destination, the catalogue was last read.
  const lanDoc = useRef<{ diemDen: string | null; luc: number } | null>(null);
  const [loc, setLoc] = useState<Loc | null>(null);
  const [dangTaiThem, setDangTaiThem] = useState(false);
  const dangTai = useRef(false);
  // The model's reasons for the first cards arrive after the list, apart from it.
  const [lyDo, setLyDo] = useState<Record<string, { reason: string; verdict: "hop" | "tam" | "khong-hop" }>>({});
  const daHoiLyDo = useRef(new Set<string>());
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
      const khoaDem = `${phien.person_id}|${daChon ?? ""}`;
      const cu = boNhoDanhMuc.get(khoaDem);
      if (cu !== undefined && (daChon === null || daChon !== dangHien.current)) {
        // What was read last for this destination shows now; the read below replaces it.
        if (daChon !== null) dangHien.current = daChon;
        else dangHien.current = cu.destination.id;
        setDiemDen(cu.destination);
        setGu(cu.gu);
        setLoc(null);
        setTrang({ pha: "xong", places: cu.places, categories: cu.categories, tiep: cu.tiep, total: cu.total });
      } else if (daChon !== null && dangHien.current !== null && daChon !== dangHien.current) {
        // A destination never read: nothing of the old city may stand under the new name.
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
      setLoc(null);
      setTrang({ pha: "xong", places: danhMuc.places, categories: danhMuc.categories, tiep: danhMuc.nextOffset, total: danhMuc.total });
      const moi: BoNho = { places: danhMuc.places, categories: danhMuc.categories, tiep: danhMuc.nextOffset, total: danhMuc.total, destination: danhMuc.destination, gu: danhMuc.gu };
      boNhoDanhMuc.set(khoaDem, moi);
      boNhoDanhMuc.set(`${phien.person_id}|${danhMuc.destination.id}`, moi);
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

  const luuLock = useRef(new Set<string>());
  const [dangLuu, setDangLuu] = useState<string[]>([]);
  const doiLuu = async (place: Place) => {
    if (luuLock.current.has(place.id)) return;
    luuLock.current.add(place.id);
    setDangLuu([...luuLock.current]);
    const truoc = daLuu.includes(place.id);
    setDaLuu((current) => daoLuu(current, place.id));
    setLoiLuu(null);
    try {
      if (truoc) await boLuuDiaDiem(phien.person_id, place.id);
      else await luuDiaDiem(phien.person_id, place.id);
    } catch (error) {
      setDaLuu((current) => current.includes(place.id) === truoc ? current : daoLuu(current, place.id));
      setLoiLuu(loiRaChu(error));
    } finally {
      luuLock.current.delete(place.id);
      setDangLuu([...luuLock.current]);
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
  const locDangDung = loai !== null || locTen;
  const khoaLoc = `${loai ?? ""}|${locTen ? query.trim() : ""}`;
  const locSan = loc !== null && loc.khoa === khoaLoc ? loc : null;
  const danhSachGoc = useMemo(() => {
    if (trang.pha !== "xong") return [];
    if (timKiem.kind === "co-ket-qua") return timKiem.places;
    const theoLoai = loai === null ? trang.places : trang.places.filter((p) => p.category === loai);
    const cucBo = locTen ? locTheoTen(theoLoai, query) : theoLoai;
    // The loaded pages answer at once; the server's filtered read adds what the
    // pages not yet loaded would have matched.
    return locSan !== null ? noiTiep(cucBo, locSan.places) : cucBo;
  }, [trang, timKiem, loai, query, locTen, locSan]);
  const danhSach = useMemo(
    () => (timKiem.kind === "co-ket-qua" ? danhSachGoc : danhSachGoc.map((p) => voiLyDo(p, lyDo[p.id]))),
    [danhSachGoc, lyDo, timKiem.kind],
  );
  // Painted stand-ins for the cards without a photograph: by category, only
  // scenes of this destination's own region, and no two neighbours alike.
  const canhCho = useMemo(
    () => phanCanhDanhSach(danhSach.map((p) => ({ id: p.id, loai: p.category, coAnh: anhBiaThe(p) !== null })), vungCua({ diemDen: diemDen?.id })),
    [danhSach, diemDen?.id],
  );
  const tiepHienTai = trang.pha !== "xong" || timKiem.kind === "co-ket-qua" ? null : locDangDung ? (locSan?.tiep ?? null) : trang.tiep;
  const tongHienTai = trang.pha !== "xong" ? null : locDangDung ? (locSan?.total ?? null) : trang.total;
  const khoaTrang = useRef("");
  khoaTrang.current = `${diemDen?.id ?? ""}|${khoaLoc}`;

  // A category or a name is also read from the server, so what the loaded pages
  // do not hold can still be found. Names wait for a pause in typing.
  useEffect(() => {
    if (!locDangDung || trang.pha !== "xong") {
      setLoc(null);
      return;
    }
    let huy = false;
    const hen = setTimeout(() => {
      docDanhMuc({ destination: dangHien.current, category: loai, q: locTen ? query.trim() : undefined, personId: phien.person_id })
        .then((dm) => {
          if (!huy) setLoc({ khoa: khoaLoc, places: dm.places, tiep: dm.nextOffset, total: dm.total });
        })
        .catch(() => undefined);
    }, locTen ? 350 : 0);
    return () => {
      huy = true;
      clearTimeout(hen);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [khoaLoc, locDangDung, trang.pha, diemDen?.id, phien.person_id]);

  // The next page, from where the list on screen ends.
  const taiThem = () => {
    if (tiepHienTai === null || dangTai.current) return;
    const khoa = khoaTrang.current;
    const theoLoc = locDangDung;
    dangTai.current = true;
    setDangTaiThem(true);
    docDanhMuc({ destination: dangHien.current, category: theoLoc ? loai : null, q: theoLoc && locTen ? query.trim() : undefined, personId: phien.person_id, offset: tiepHienTai })
      .then((dm) => {
        if (khoaTrang.current !== khoa) return;
        if (theoLoc) {
          setLoc((l) => (l !== null && l.khoa === khoaLoc ? { ...l, places: noiTiep(l.places, dm.places), tiep: dm.nextOffset, total: dm.total ?? l.total } : l));
        } else {
          setTrang((t) => (t.pha === "xong" ? { ...t, places: noiTiep(t.places, dm.places), tiep: dm.nextOffset, total: dm.total ?? t.total } : t));
        }
      })
      .catch((error) => setLoiNapLai(loiRaChu(error)))
      .finally(() => {
        dangTai.current = false;
        setDangTaiThem(false);
      });
  };

  // Reasons for the first dozen cards on screen, once each, never blocking the list.
  const idDauTien = danhSachGoc.slice(0, 12).map((p) => p.id).join(",");
  useEffect(() => {
    if (timKiem.kind === "co-ket-qua" || idDauTien === "") return;
    const can = idDauTien.split(",").filter((id) => !daHoiLyDo.current.has(id));
    if (can.length === 0) return;
    can.forEach((id) => daHoiLyDo.current.add(id));
    let huy = false;
    void docLyDo(can, { personId: phien.person_id }).then((ly) => {
      if (!huy && Object.keys(ly).length > 0) setLyDo((cu) => ({ ...cu, ...ly }));
      // A read that returned nothing may be asked again if the card shows once more.
      else if (!huy) can.forEach((id) => daHoiLyDo.current.delete(id));
    });
    return () => {
      huy = true;
      can.forEach((id) => { if (!(id in lyDoRef.current)) daHoiLyDo.current.delete(id); });
    };
  }, [idDauTien, timKiem.kind, phien.person_id]);
  const lyDoRef = useRef(lyDo);
  lyDoRef.current = lyDo;

  const dangLoc = loai !== null || locTen || timKiem.kind === "co-ket-qua";
  // A search session (a name, a question, an answer) keeps the top of the
  // screen for the results: the city's stage folds away.
  const { width: screenWidth, height: screenHeight } = useWindowDimensions();
  // Composition follows the viewport, so hot text scaling never moves the
  // same place name between the header and the metadata below its picture.
  const compactLead = screenWidth < 600 && screenHeight < 700;
  const cauLoi = cauTimKiem(timKiem);
  // A filter change crossfades the results; a keystroke does not (it would
  // flicker on every letter). Reduce Motion cuts straight to the new list.
  const khoaKetQua = `${loai ?? ""}|${timKiem.kind === "co-ket-qua" ? timKiem.query : ""}`;
  const hienRa = FadeIn.duration(motion.ms("standard")).reduceMotion(motion.reanimated);
  // Promote the first real result with either credited media or labelled category artwork.
  const daNhat = danhSach[0];
  const dan = daNhat;
  // Two candidates to compare before the list: the same axis for both, so the
  // choice starts as a comparison (`taiSoSanh`).
  const { soSanh, hang: conLai } = taiSoSanh(danhSach.slice(1));
  const rong = danhSach.length === 0;
  // An empty list for a city with nothing in it is not an empty search: it
  // must not advise dropping a filter that is not there (QA UI-028).
  const thanhPhoRong = rong && !dangLoc && trang.pha === "xong" && trang.places.length === 0;
  // A name search that found nothing but reads like a question is handed to
  // Rủ Đi AI rather than left at «0 kết quả».
  const giongCauHoi = locTen && query.trim().split(/\s+/).length >= 3;


  return (
    <RudiScreen bottomInset="tab" header={dau?.()} onGanCuoi={taiThem} contentStyle={compactLead ? { gap: 12 } : undefined} onRefresh={nap} testID="explore-screen">
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
            <Text style={[typography.label, styles.tenViTri, { color: colors.ink }]}>
              {diemDen.name}
              <Text style={{ color: colors.accent }}> · đổi nơi khác</Text>
            </Text>
          ) : (
            <Text style={[typography.label, styles.tenViTri, { color: colors.ink }]}>
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
      {diemDen !== null && !dangLoc && query.trim() === "" ? (
        <SanThanhPho anh={biaDiemDen?.id === diemDen.id ? biaDiemDen.anh : null} id={diemDen.id} ten={diemDen.name} rong={Math.max(120, Math.min(480, screenWidth - 32))} cao={compactLead ? 80 : 150} />
      ) : null}
      {/* One row at every font size: the field's own hint ellipsizes, so the
          assistant no longer drops to a line of its own at 1.3 (QA 23/09). */}
      <View style={styles.timRow}>
        <View style={styles.flex}>
          <SearchField
            nativeMetrics
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
        <IconButton accessibilityLabel="Hỏi Rủ Đi AI" icon="sparkles" onPress={bamHoi} selected tone="ai" style={styles.hoiAi} />
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
          <View style={styles.dauKetQua}>
            <View style={styles.tenKetQua}>
              {/* The destination and count come from the response; neither
                  the heading nor the artwork implies a distance ranking. */}
              <Text style={[typography.h2, styles.tenKhu, { color: colors.ink }]}>
                {dangLoc ? `${Math.max(tongHienTai ?? 0, danhSach.length).toLocaleString("vi-VN")} kết quả` : `Chỗ hay ở ${diemDen === null ? "đây" : diemDen.name}`}
              </Text>
              {dangLoc || query !== "" ? (
                <Pressable accessibilityRole="button" onPress={boTim} style={({ pressed }) => [styles.xoaLoc, pressed && styles.bam]}>
                  <Text style={[typography.label, { color: colors.accent }]}>Xóa lọc</Text>
                </Pressable>
              ) : (
                <Text style={[typography.caption, { color: colors.inkFaint }]}>{`${Math.max(trang.total ?? 0, trang.places.length).toLocaleString("vi-VN")} nơi`}</Text>
              )}
            </View>
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
          </View>
          {thanhPhoRong ? (
            <EmptyState
              nativeMetrics
              action={{ label: "Đổi điểm đến", onPress: () => router.push("/destinations") }}
              body="Danh mục của Rủ Đi chưa có chỗ nào ở đây. Chọn một điểm đến khác để xem chỗ hay."
              illustration={<Canh id="tim-khong-ra" width={168} />}
              kind="no-results"
              layout="inline"
              title={`${diemDen?.name ?? "Nơi này"} chưa có địa điểm nào`}
            />
          ) : rong && giongCauHoi ? (
            <EmptyState
              nativeMetrics
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
              nativeMetrics
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
                  nativeMetrics
                  compact={compactLead}
                  daLuu={daLuu.includes(dan.id)}
                  dd={hienThiDiaDiem(dan, canhCho.get(dan.id))}
                  onOpen={() => router.push(`/places/${dan.id}` as never)}
                  onSave={() => void doiLuu(dan)}
                  saving={dangLuu.includes(dan.id)}
                />
              ) : null}
              {soSanh !== null ? (
                <PlaceCompare
                  nativeMetrics
                  largeText={largeText}
                  daLuu={(id) => daLuu.includes(id)}
                  items={[hienThiDiaDiem(soSanh[0], canhCho.get(soSanh[0].id)), hienThiDiaDiem(soSanh[1], canhCho.get(soSanh[1].id))]}
                  onOpen={(id) => router.push(`/places/${id}` as never)}
                  onSave={(id) => {
                    const place = soSanh.find((p) => p.id === id);
                    if (place !== undefined) void doiLuu(place);
                  }}
                  saving={(id) => dangLuu.includes(id)}
                  testID="explore-compare"
                />
              ) : null}
              {conLai.length > 0 ? (
                <ResponsiveRow gap={0} minItemWidth={300}>
                  {conLai.map((place) => (
                    <PlaceRow
                      nativeMetrics
                      largeText={largeText}
                      daLuu={daLuu.includes(place.id)}
                      dd={hienThiDiaDiem(place, canhCho.get(place.id))}
                      key={place.id}
                      onOpen={() => router.push(`/places/${place.id}` as never)}
                      onSave={() => void doiLuu(place)}
                      saving={dangLuu.includes(place.id)}
                    />
                  ))}
                </ResponsiveRow>
              ) : null}
              {tiepHienTai !== null ? (
                <RudiButton
                  label={dangTaiThem ? "Đang tải thêm…" : cauXemThem(Math.max(1, (tongHienTai ?? danhSach.length + 1) - danhSach.length))}
                  loading={dangTaiThem}
                  onPress={taiThem}
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
  dauKetQua: { gap: 4 },
  tenKetQua: { flexDirection: "row", alignItems: "center", gap: 12 },
  tenKhu: { flex: 1, minWidth: 0 },
  xoaLoc: { minWidth: 48, minHeight: 48, justifyContent: "center" },
  flex: { flex: 1 },
  // Above the city stage, whose sky rises under this line (SanThanhPho).
  dau: { gap: 6, zIndex: 1 },
  buuThiep: { alignItems: "center", alignSelf: "center", marginTop: -12, overflow: "hidden", borderRadius: 8 },
  sanPhac: { alignItems: "center", justifyContent: "center" },
  viTri: { flexDirection: "row", alignItems: "center", gap: 6, minHeight: 48, maxWidth: "100%", alignSelf: "flex-start" },
  tenViTri: { flexShrink: 1, minWidth: 0 },
  timRow: { flexDirection: "row", alignItems: "stretch", gap: 8 },
  hoiAi: { height: "auto", minHeight: 48, alignSelf: "stretch" },
  khung: { gap: 12 },
  // Keep a paper gutter between scrolling labels and Nếp's bookmark rail.
  cuonLoai: { marginLeft: -16 },
  hangLoai: { flexDirection: "row", gap: 8, paddingHorizontal: 16 },
  theAi: { gap: 4, padding: 14, borderRadius: 14 },
  guRow: { gap: 2 },
  // Only while it is a button does the sentence need a 48dp target.
  guNut: { minHeight: 48, justifyContent: "center" },
  bam: { opacity: 0.7 },
  ketQua: { gap: 20 },
});
