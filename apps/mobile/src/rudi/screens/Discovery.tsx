/**
 * Khám phá, Match gu and Chi tiết địa điểm on the fixture catalogue (the dev
 * door, `EXPO_PUBLIC_RUDI_FIXTURE=1`). The live screens in `explore/` read
 * the server; these read `fixtures.ts`. Both draw with the same components
 * (`HangDiaDiem`) so the two builds stop drifting apart visually.
 *
 * Fixture facts (rating, distance, price, «Hợp gu») are the sample world's
 * and sit under a demo badge; on a real session with no catalogue of its own
 * they are called samples in words and the match badge is not drawn.
 */
import { Ionicons } from "@expo/vector-icons";
import * as Haptics from "expo-haptics";
import { useLocalSearchParams, useRouter } from "expo-router";
import { useMemo, useState } from "react";
import { ScrollView, Share, StyleSheet, Text, useWindowDimensions, View } from "react-native";

import { PLACES, type DemoPlace } from "../fixtures";
import { PLACE_CATEGORIES, filterPlaces, type PlaceCategory } from "../places";
import { useRudiSession } from "../session";
import { typography, useRudiTheme } from "../theme";
import {
  AiNote,
  Chip,
  DemoBadge,
  Heading,
  IconButton,
  Inline,
  Photo,
  ResponsiveRow,
  RudiButton,
  RudiScreen,
  SearchField,
  SectionHeader,
  TopBar,
  type IconName,
} from "../ui";
import { Wordmark } from "../ui/Wordmark";
import { EmptyState } from "../ui/EmptyState";
import { PlaceLead, PlaceRow, type DiaDiemHienThi } from "./explore/HangDiaDiem";

const GLYPH: Record<PlaceCategory, IconName> = {
  "Quán ăn": "restaurant-outline",
  Cafe: "cafe-outline",
  "Vui chơi": "game-controller-outline",
  "Đi chơi đêm": "moon-outline",
};

/** The sample place in the row/lead vocabulary. `song`: a real session, so no invented match badge. */
function hienThiMau(place: DemoPlace, song: boolean): DiaDiemHienThi {
  return {
    id: place.id,
    name: place.name,
    sub: place.subtitle,
    facts: [
      { icon: "star", text: `${place.rating} (${place.reviews})` },
      { icon: "navigate-outline", text: place.distance },
      { icon: "wallet-outline", text: place.price },
    ],
    glyph: GLYPH[place.category],
    photo: place.image,
    badge: !song && place.match >= 90 ? "Hợp gu" : null,
  };
}

/**
 * Whose group this screen is talking about.
 *
 * Until M4 ports Explore to `/places`, the catalogue on this screen is the
 * sample one. With a REAL session that must be said out loud and the story
 * must stop naming «Team Đà Lạt» for a group somebody just created: the
 * sentence names their group and calls the suggestions samples. On the
 * fixture build the fixture story stands.
 */
function tenNhomHienTai(session: ReturnType<typeof useRudiSession>): string {
  if (session.cheDo !== "live") return "Team Đà Lạt";
  const phien = session.phien;
  const nhom = phien?.contexts?.find((ung) => ung.id === phien.context_id);
  return nhom?.display_name ?? "nhóm của bạn";
}

export function ExploreScreen() {
  const router = useRouter();
  const { colors } = useRudiTheme();
  const session = useRudiSession();
  const song = session.cheDo === "live";
  const tenNhom = tenNhomHienTai(session);
  const [category, setCategory] = useState<PlaceCategory | null>(null);
  const [query, setQuery] = useState("");
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [matchOnly, setMatchOnly] = useState(false);
  const [nearOnly, setNearOnly] = useState(false);
  const [savedOnly, setSavedOnly] = useState(false);

  const visiblePlaces = useMemo(
    () =>
      filterPlaces(PLACES, {
        query,
        category,
        matchOnly,
        nearOnly,
        savedOnly,
        savedIds: session.savedPlaceIds,
      }),
    [category, matchOnly, nearOnly, query, savedOnly, session.savedPlaceIds],
  );

  const filtering = Boolean(query.trim()) || matchOnly || nearOnly || savedOnly || category !== null;

  const resetFilters = () => {
    setQuery("");
    setCategory(null);
    setMatchOnly(false);
    setNearOnly(false);
    setSavedOnly(false);
  };

  const toggleSaved = (id: string) => {
    void Haptics.selectionAsync();
    session.toggleSaved(id);
  };

  const moDiaDiem = (id: string) => router.push(("/places/" + id) as never);
  const [dan, ...conLai] = visiblePlaces;

  return (
    <RudiScreen bottomInset={112} testID="explore-screen">
      <View style={styles.exploreHeader}>
        <View style={styles.exploreBrand}>
          <Wordmark color={colors.ink} height={20} />
          <Inline gap={5} style={styles.location}>
            <Ionicons color={colors.accent} name="location" size={16} />
            <Text style={[typography.label, { color: colors.ink }]}>
              {song ? "Khu vực chưa chọn" : "Đà Lạt, Lâm Đồng"}
            </Text>
          </Inline>
        </View>
        <Inline gap={8}>
          <DemoBadge />
          <IconButton
            accessibilityLabel="Thông báo"
            icon="notifications-outline"
            onPress={() => session.setInboxOpen(true)}
            quiet
            selected={session.inboxOpen}
          />
        </Inline>
      </View>
      {session.inboxOpen ? (
        <EmptyState
          action={{ label: "Đóng", onPress: () => session.setInboxOpen(false) }}
          body="Chưa có hộp thư máy chủ. Bản trải nghiệm không đẩy thông báo."
          kind="first-use"
          layout="inline"
          title="Thông báo"
        />
      ) : null}
      <View style={styles.searchRow}>
        <View style={styles.flex}>
          <SearchField
            onChangeText={setQuery}
            onSubmitEditing={() => setFiltersOpen(false)}
            value={query}
          />
        </View>
        <IconButton
          accessibilityLabel={filtersOpen ? "Đóng bộ lọc" : "Mở bộ lọc"}
          icon="options-outline"
          onPress={() => setFiltersOpen((value) => !value)}
          selected={filtersOpen}
        />
        {/* The assistant is a button beside the search, not a banner above the places. */}
        <IconButton accessibilityLabel="Match gu cả nhóm bằng AI" icon="sparkles" onPress={() => router.push("/ai-match")} selected tone="ai" />
      </View>
      {filtersOpen ? (
        <Inline gap={7} wrap>
          <Chip icon="sparkles-outline" label="Từ 90% hợp gu" onPress={() => setMatchOnly((value) => !value)} selected={matchOnly} />
          <Chip icon="navigate-outline" label="Trong 2 km" onPress={() => setNearOnly((value) => !value)} selected={nearOnly} />
          <Chip icon="heart-outline" label="Đã lưu" onPress={() => setSavedOnly((value) => !value)} selected={savedOnly} />
        </Inline>
      ) : null}
      <ScrollView contentContainerStyle={styles.hangLoai} horizontal keyboardShouldPersistTaps="handled" showsHorizontalScrollIndicator={false} style={styles.cuonLoai}>
        {PLACE_CATEGORIES.map((label) => {
          const active = category === label;
          return <Chip icon={GLYPH[label]} key={label} label={label} onPress={() => setCategory(active ? null : label)} selected={active} />;
        })}
      </ScrollView>
      <SectionHeader
        action={filtering ? "Xóa lọc" : undefined}
        onAction={filtering ? resetFilters : undefined}
        title={
          filtering
            ? `${visiblePlaces.length} kết quả phù hợp`
            : song
              ? "Mẫu minh hoạ"
              : "Gần bạn, đúng gu"
        }
      />
      {song ? (
        // The rows below carry distances, ratings and prices. For a real
        // session those are sample numbers until M4 reads the catalogue from
        // the server, and a number that looks measured must say it is not.
        <Text style={[typography.caption, { color: colors.inkSoft }]}>
          Gợi ý mẫu cho {tenNhom}: khoảng cách, đánh giá và giá ở đây là số mẫu. Gợi ý thật cho khu vực của nhóm đến ở bản sau.
        </Text>
      ) : null}
      {dan === undefined ? (
        <EmptyState
          action={{ label: "Xóa bộ lọc", onPress: resetFilters }}
          body="Thử từ khóa khác hoặc bỏ bớt bộ lọc nhé."
          kind={query.trim() ? "no-results" : "filtered"}
          layout="inline"
          title="Chưa thấy nơi phù hợp"
        />
      ) : (
        <View style={styles.ketQua}>
          <PlaceLead
            daLuu={session.savedPlaceIds.includes(dan.id)}
            dd={hienThiMau(dan, song)}
            onOpen={() => moDiaDiem(dan.id)}
            onSave={() => toggleSaved(dan.id)}
          />
          {conLai.length > 0 ? (
            <ResponsiveRow gap={0} minItemWidth={300}>
              {conLai.map((place) => (
                <PlaceRow
                  daLuu={session.savedPlaceIds.includes(place.id)}
                  dd={hienThiMau(place, song)}
                  key={place.id}
                  onOpen={() => moDiaDiem(place.id)}
                  onSave={() => toggleSaved(place.id)}
                />
              ))}
            </ResponsiveRow>
          ) : null}
        </View>
      )}
    </RudiScreen>
  );
}

export function AiMatchScreen() {
  const router = useRouter();
  const { colors } = useRudiTheme();
  const session = useRudiSession();
  const [filter, setFilter] = useState("Tất cả");
  const visible = filter === "Tất cả"
    ? PLACES
    : filter === "Dưới 250K"
      ? PLACES.filter((place) => !place.price.includes("320K") && !place.price.includes("260K"))
      : PLACES.filter((place) =>
          filter === "Ăn uống"
            ? place.category === "Quán ăn"
            : filter === "Cafe"
              ? place.category === "Cafe"
              : place.category === "Vui chơi",
        );
  const [dan, ...conLai] = visible;
  /** A suggestion row: what it is, and the two sample tags it was matched on. */
  const goiY = (place: DemoPlace, dau: boolean): DiaDiemHienThi => ({
    id: place.id,
    name: place.name,
    sub: place.subtitle,
    facts: [
      { icon: "pricetags-outline", text: place.tags.slice(0, 2).join(" · ") },
      { icon: "wallet-outline", text: place.price },
    ],
    glyph: GLYPH[place.category],
    photo: place.image,
    badge: dau ? "Gợi ý" : null,
  });

  return (
    <RudiScreen tone="ai" testID="ai-match-screen">
      <TopBar title="Match gu cả nhóm" right={<DemoBadge />} />
      <Heading
        title="Tối nay cả hội đi đâu?"
        subtitle={`Gợi ý từ ${PLACES.length} địa điểm mẫu theo sở thích 8 thành viên. Không phải kết quả LLM.`}
      />
      <ScrollView contentContainerStyle={styles.hangLoai} horizontal keyboardShouldPersistTaps="handled" showsHorizontalScrollIndicator={false} style={styles.cuonLoai}>
        {["Tất cả", "Ăn uống", "Cafe", "Vui chơi", "Dưới 250K"].map((item) => (
          <Chip key={item} label={item} onPress={() => setFilter(item)} selected={filter === item} />
        ))}
      </ScrollView>
      {dan === undefined ? (
        <EmptyState
          action={{ label: "Xem tất cả", onPress: () => setFilter("Tất cả") }}
          body="Không có nơi mẫu nào trong bộ lọc này."
          kind="filtered"
          layout="inline"
          title="Chưa có gợi ý"
        />
      ) : (
        <View style={styles.ketQua}>
          <PlaceLead
            daLuu={session.savedPlaceIds.includes(dan.id)}
            dd={goiY(dan, true)}
            onOpen={() => router.push(("/places/" + dan.id) as never)}
            onSave={() => session.toggleSaved(dan.id)}
          />
          {conLai.length > 0 ? (
            <ResponsiveRow gap={0} minItemWidth={300}>
              {conLai.map((place) => (
                <PlaceRow
                  daLuu={session.savedPlaceIds.includes(place.id)}
                  dd={goiY(place, false)}
                  key={place.id}
                  onOpen={() => router.push(("/places/" + place.id) as never)}
                  onSave={() => session.toggleSaved(place.id)}
                />
              ))}
            </ResponsiveRow>
          ) : null}
        </View>
      )}
      <Text style={[typography.caption, { color: colors.inkFaint }]}>
        Đây là gợi ý có thể chỉnh. Rủ Đi không tự thêm nơi vào kế hoạch của nhóm.
      </Text>
    </RudiScreen>
  );
}

export function PlaceDetailScreen() {
  const router = useRouter();
  const params = useLocalSearchParams<{ id?: string }>();
  const { colors } = useRudiTheme();
  const { width } = useWindowDimensions();
  const session = useRudiSession();
  const [added, setAdded] = useState(false);
  const place = useMemo(
    () => PLACES.find((item) => item.id === params.id) ?? PLACES[0],
    [params.id],
  );
  const saved = session.savedPlaceIds.includes(place.id);
  const rong = width >= 700;

  return (
    <RudiScreen padded={false} testID="place-detail-screen">
      <View style={styles.detailShell}>
        <Photo
          height={rong ? 400 : 300}
          radius={rong ? 24 : 0}
          source={place.image}
          overlay={
            <View style={styles.detailTop}>
              <IconButton accessibilityLabel="Quay lại" icon="chevron-back" onPress={() => router.back()} />
              <Inline gap={8}>
                <IconButton
                  accessibilityLabel="Chia sẻ"
                  icon="share-social-outline"
                  onPress={() =>
                    void Share.share({
                      message: `${place.name}: ${place.subtitle}`,
                    })
                  }
                />
                <IconButton
                  accessibilityLabel={saved ? "Bỏ lưu" : "Lưu địa điểm"}
                  icon={saved ? "heart" : "heart-outline"}
                  onPress={() => session.toggleSaved(place.id)}
                  selected={saved}
                />
              </Inline>
            </View>
          }
        />
        <View style={styles.detailContent}>
          <DemoBadge />
          <Heading title={place.name} subtitle={place.subtitle} />
          {/* The three facts that decide, on the paper, each one text node. */}
          <Inline gap={14} wrap>
            <Inline gap={5}>
              <Ionicons color={colors.accent} name="star" size={15} />
              <Text style={[typography.label, { color: colors.ink }]}>{`${place.rating} (${place.reviews} đánh giá)`}</Text>
            </Inline>
            <Inline gap={5}>
              <Ionicons color={colors.inkFaint} name="navigate-outline" size={15} />
              <Text style={[typography.label, { color: colors.ink }]}>{place.distance}</Text>
            </Inline>
            <Inline gap={5}>
              <Ionicons color={colors.inkFaint} name="wallet-outline" size={15} />
              <Text style={[typography.label, { color: colors.ink }]}>{place.price}</Text>
            </Inline>
          </Inline>
          <Inline gap={8} wrap>
            {place.tags.map((tag) => <Chip key={tag} label={tag} />)}
          </Inline>
          <View style={styles.khoi}>
            <SectionHeader title={`Vì sao hợp ${tenNhomHienTai(session)}?`} />
            <AiNote>View thoáng, món nướng dễ chia sẻ và đủ chỗ cho nhóm 8 người.</AiNote>
          </View>
          <View style={styles.khoi}>
            <SectionHeader title="Không gian" />
            <ResponsiveRow minItemWidth={260}>
              {[PLACES[1].image, PLACES[2].image].map((image, index) => (
                <View key={index} style={styles.galleryCell}>
                  <Photo height={150} radius={14} source={image} />
                </View>
              ))}
            </ResponsiveRow>
          </View>
          <RudiButton
            icon="add-circle-outline"
            label={added ? `Đã thêm vào ${session.tripName}` : `Thêm vào ${session.tripName}`}
            onPress={() => {
              const ok = session.addPlaceToTrip(place.id);
              setAdded(ok);
              router.push(session.tripPath("/itinerary") as never);
            }}
          />
        </View>
      </View>
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  flex: { flex: 1 },
  exploreHeader: { flexDirection: "row", alignItems: "flex-start", justifyContent: "space-between", gap: 10 },
  exploreBrand: { gap: 6, flexShrink: 1 },
  location: { minHeight: 24 },
  searchRow: { flexDirection: "row", alignItems: "flex-end", gap: 8 },
  cuonLoai: { marginHorizontal: -16 },
  hangLoai: { flexDirection: "row", gap: 8, paddingHorizontal: 16 },
  ketQua: { gap: 20 },
  khoi: { gap: 8 },
  detailShell: { width: "100%", maxWidth: 960, alignSelf: "center" },
  detailTop: { position: "absolute", left: 14, right: 14, top: 14, flexDirection: "row", justifyContent: "space-between" },
  detailContent: { paddingHorizontal: 16, paddingTop: 19, gap: 19 },
  galleryCell: { flex: 1 },
});
