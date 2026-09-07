/**
 * The fixture group's memories (dev door): its wall, the trip album and the
 * share screen. The live counterparts are in `ky-niem/`; the grammar is the
 * same -- a post is a page on the paper, an album opens on its first
 * photograph, tagging people means their names. Nothing here is called a
 * video: the fixture has photographs, and says so.
 */
import { Ionicons } from "@expo/vector-icons";
import { useRouter } from "expo-router";
import { useState } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";

import { COLLECTOR_INDEX, DEMO_GROUP, MEMORY_PHOTOS, PEOPLE, demoAssets } from "../fixtures";
import { useRudiSession } from "../session";
import { useAdaptiveLayout } from "../ui/useAdaptiveLayout";
import { lopPhu, mucTrenAnh, typography, useRudiTheme } from "../theme";
import {
  Chip,
  DemoBadge,
  Field,
  IconButton,
  Inline,
  Photo,
  PhotoShade,
  ResponsiveRow,
  RudiButton,
  RudiScreen,
  SectionHeader,
  Segmented,
  TopBar,
} from "../ui";
import { Avatar, AvatarStack } from "../ui/Avatar";
import { KhungAnh } from "../ui/KhungAnh";
import { RosterPicker } from "../ui/RosterPicker";

function FeedPost({
  personIndex,
  imageIndex,
  caption,
  time,
}: {
  personIndex: number;
  imageIndex: number;
  caption: string;
  time: string;
}) {
  const { colors } = useRudiTheme();
  const [liked, setLiked] = useState(false);
  const person = PEOPLE[personIndex];

  return (
    <View style={[styles.post, { borderBottomColor: colors.line }]}>
      <View style={styles.postHeader}>
        <Avatar name={person.name} size={36} />
        <View style={styles.flex}>
          <Text style={[typography.label, { color: colors.ink }]}>{person.name}</Text>
          <Text style={[typography.caption, { color: colors.inkFaint }]}>Đà Lạt · {time}</Text>
        </View>
        <IconButton accessibilityLabel="Tùy chọn bài viết" icon="ellipsis-horizontal" quiet />
      </View>
      <Text style={[typography.body, { color: colors.ink }]}>{caption}</Text>
      <KhungAnh xuatXu={`${person.name} · Đà Lạt · ${time}`}>
        <Photo height={240} radius={4} source={MEMORY_PHOTOS[imageIndex]} />
      </KhungAnh>
      <View style={styles.postActions}>
        <Pressable
          accessibilityRole="button"
          onPress={() => setLiked((value) => !value)}
          style={({ pressed }) => [styles.postAction, pressed && styles.pressed]}
        >
          <Ionicons color={liked ? colors.accent : colors.inkSoft} name={liked ? "heart" : "heart-outline"} size={22} />
          <Text style={[typography.label, { color: liked ? colors.accent : colors.inkSoft }]}>Thích</Text>
        </Pressable>
        <Pressable accessibilityRole="button" style={({ pressed }) => [styles.postAction, pressed && styles.pressed]}>
          <Ionicons color={colors.inkSoft} name="chatbubble-outline" size={20} />
          <Text style={[typography.label, { color: colors.inkSoft }]}>Bình luận</Text>
        </Pressable>
        <Text style={[typography.caption, styles.flex, { color: colors.inkFaint, textAlign: "right" }]}>
          {PEOPLE[COLLECTOR_INDEX].name} và {PEOPLE.length - 1} người khác · 4 bình luận
        </Text>
      </View>
    </View>
  );
}

export function GroupWallScreen() {
  const router = useRouter();
  const { colors, radius } = useRudiTheme();
  const session = useRudiSession();
  const [tab, setTab] = useState(0);

  const onTab = (index: number) => {
    if (index === 1) {
      router.push(session.tripPath("/album") as never);
      return;
    }
    if (index === 2) {
      router.push(session.tripPath("/itinerary") as never);
      return;
    }
    setTab(index);
  };

  return (
    <RudiScreen testID="group-wall-screen">
      <TopBar
        title={DEMO_GROUP.name}
        subtitle="Không gian kỷ niệm"
        right={<IconButton accessibilityLabel="Tùy chọn nhóm" icon="ellipsis-horizontal" quiet />}
      />
      {/* A low cover: the group's picture, its name, who is in it. The counts
          are one line under it, not a card of three numbers. */}
      <Photo
        height={180}
        radius={20}
        source={demoAssets.friends}
        overlay={
          <PhotoShade>
            <View style={styles.wallHero}>
              <Text style={styles.wallTitle}>Team Đà Lạt</Text>
              <View style={styles.wallMeta}>
                <AvatarStack max={5} people={PEOPLE.map((p) => ({ name: p.name }))} />
                <Text style={styles.wallMetaText}>{PEOPLE.length} thành viên · 1 chuyến đi</Text>
              </View>
            </View>
          </PhotoShade>
        }
      />
      <View style={styles.dongDem}>
        <DemoBadge />
        <Text style={[typography.caption, { color: colors.inkSoft }]}>
          {session.photoCount} ảnh · {session.checkInCount} check-in
        </Text>
      </View>
      <Segmented items={["Tường", "Album", "Kế hoạch", "Thành viên"]} onSelect={onTab} selected={tab} />
      {tab === 3 ? (
        <View>
          {PEOPLE.map((person) => (
            <View key={person.id} style={[styles.hangNguoi, { borderBottomColor: colors.line }]}>
              <Avatar name={person.name} size={40} />
              <Text style={[typography.label, { color: colors.ink }]}>{person.name}</Text>
            </View>
          ))}
        </View>
      ) : (
        <>
          <Pressable
            accessibilityLabel="Chia sẻ khoảnh khắc với cả nhóm"
            accessibilityRole="button"
            onPress={() => router.push("/moments/new")}
            style={({ pressed }) => [styles.sharePrompt, pressed && styles.pressed]}
          >
            <Avatar name={session.displayName} size={40} />
            <View style={[styles.promptField, { backgroundColor: colors.card, borderColor: colors.lineStrong, borderRadius: radius.control }]}>
              <Text style={[typography.body, { color: colors.inkFaint }]}>Chia sẻ khoảnh khắc với cả nhóm...</Text>
            </View>
            <Ionicons color={colors.accent} name="images-outline" size={22} />
          </Pressable>
          <SectionHeader
            action="Mở album"
            onAction={() => router.push(session.tripPath("/album") as never)}
            title="Chuyện của hội mình"
          />
          <View>
            <FeedPost
              caption="Sáng Đà Lạt lạnh nhưng cả hội vẫn dậy đúng giờ săn mây. Xứng đáng ghê! ☁️"
              imageIndex={2}
              personIndex={2}
              time="2 giờ"
            />
            <FeedPost
              caption="Một chiếc ảnh đủ 8 người sau bao lần hẹn mãi mới đủ mặt 🌿"
              imageIndex={0}
              personIndex={1}
              time="Hôm qua"
            />
          </View>
        </>
      )}
    </RudiScreen>
  );
}

export function TripAlbumScreen() {
  const router = useRouter();
  const { colors, radius } = useRudiTheme();
  const { sizeClass } = useAdaptiveLayout();
  // A phone reads the lead photograph at 4:3; a tablet would get a wall of
  // pixels at that ratio, so the lead widens to a band and the story starts sooner.
  const tiLeDan = sizeClass === "compact" ? 4 / 3 : 21 / 9;
  const [newestFirst, setNewestFirst] = useState(false);
  const [selecting, setSelecting] = useState(false);
  const [selectedPhotos, setSelectedPhotos] = useState<number[]>([]);
  const visiblePhotos = MEMORY_PHOTOS.map((photo, originalIndex) => ({ photo, originalIndex }));
  if (newestFirst) visiblePhotos.reverse();
  const [dan, ...conLai] = visiblePhotos;

  const togglePhoto = (index: number) => {
    setSelectedPhotos((items) =>
      items.includes(index) ? items.filter((item) => item !== index) : [...items, index],
    );
  };

  const toggleSelectionMode = () => {
    setSelecting((value) => {
      if (value) setSelectedPhotos([]);
      return !value;
    });
  };

  const oAnh = (photo: (typeof visiblePhotos)[number], lead: boolean) => {
    const selected = selectedPhotos.includes(photo.originalIndex);
    return (
      <Pressable
        key={photo.originalIndex}
        accessibilityLabel={
          selecting
            ? `${selected ? "Bỏ chọn" : "Chọn"} ảnh ${photo.originalIndex + 1}`
            : `Mở ảnh ${photo.originalIndex + 1}`
        }
        accessibilityRole={selecting ? "checkbox" : "button"}
        aria-checked={selecting ? selected : undefined}
        onPress={() => {
          if (!selecting) setSelecting(true);
          togglePhoto(photo.originalIndex);
        }}
        style={({ pressed }) => [styles.gridPhoto, pressed && styles.pressed]}
      >
        {lead ? (
          <KhungAnh xuatXu={`${DEMO_GROUP.name} · Đà Lạt · 17 - 19/10/2026`}>
            <Photo radius={4} ratio={tiLeDan} source={photo.photo} />
          </KhungAnh>
        ) : (
          <Photo radius={radius.small} ratio={1} source={photo.photo} />
        )}
        {selecting ? (
          <View style={[styles.selectionBadge, selected && { backgroundColor: colors.accent, borderColor: colors.accent }]}>
            <Ionicons color={mucTrenAnh} name={selected ? "checkmark" : "ellipse-outline"} size={17} />
          </View>
        ) : null}
      </Pressable>
    );
  };

  return (
    <RudiScreen testID="trip-album-screen">
      <TopBar
        title="Album Đà Lạt"
        subtitle="17 - 19/10/2026"
        right={<IconButton accessibilityLabel="Thêm ảnh" icon="add" onPress={() => router.push("/moments/new")} quiet />}
      />
      {/* The first photograph leads; the title and the date sit right under it. */}
      {dan ? oAnh(dan, true) : null}
      <View style={styles.albumDau}>
        <Text style={[typography.h2, { color: colors.ink }]}>Những ngày mình đi cùng nhau</Text>
        <Text style={[typography.caption, { color: colors.inkSoft }]}>Đà Lạt cuối tuần · 17 - 19/10/2026 · Team Đà Lạt · {MEMORY_PHOTOS.length} ảnh</Text>
      </View>
      <View style={styles.albumToolbar}>
        <Text style={[typography.label, styles.flex, { color: colors.ink }]}>
          {selecting ? `${selectedPhotos.length} ảnh đã chọn` : "Khoảnh khắc"}
        </Text>
        <Inline gap={7}>
          <Chip
            icon="calendar-outline"
            label={newestFirst ? "Mới trước" : "Theo ngày"}
            onPress={() => setNewestFirst((value) => !value)}
            selected={newestFirst}
          />
          <IconButton
            accessibilityLabel={selecting ? "Đóng chọn ảnh" : "Chọn ảnh"}
            icon={selecting ? "close-circle" : "checkmark-circle-outline"}
            onPress={toggleSelectionMode}
            selected={selecting}
          />
        </Inline>
      </View>
      <ResponsiveRow gap={6} maxColumns={6} minItemWidth={104}>
        {conLai.map((photo) => oAnh(photo, false))}
      </ResponsiveRow>
      <RudiButton
        disabled={selecting && selectedPhotos.length === 0}
        icon={selecting ? "share-social-outline" : "cloud-upload-outline"}
        label={selecting ? `Chia sẻ ${selectedPhotos.length} ảnh` : "Thêm khoảnh khắc"}
        onPress={() => router.push("/moments/new")}
      />
    </RudiScreen>
  );
}

export function ShareMomentScreen() {
  const router = useRouter();
  const { colors, radius } = useRudiTheme();
  const [caption, setCaption] = useState("Đà Lạt có lạnh, nhưng hội mình thì không 🌲✨");
  const [visibility, setVisibility] = useState(0);
  const [selectedPeople, setSelectedPeople] = useState<string[]>(PEOPLE.slice(0, 4).map((p) => p.id));

  const toggle = (id: string) => {
    setSelectedPeople((items) => (items.includes(id) ? items.filter((item) => item !== id) : [...items, id]));
  };

  return (
    <RudiScreen contentStyle={styles.form} testID="share-moment-screen">
      <TopBar title="Chia sẻ khoảnh khắc" right={<DemoBadge />} />
      {/* The print: the picture, the sentence, and where it was taken. */}
      <KhungAnh chuThich={caption.trim() === "" ? "Câu của bạn hiện ở đây" : caption.trim()} xuatXu={`${PEOPLE[COLLECTOR_INDEX].name} · Đà Lạt, Lâm Đồng · hôm nay`}>
        <Photo height={240} radius={4} source={demoAssets.dalatFriends} />
      </KhungAnh>
      <Field
        label="Viết vài dòng"
        multiline
        onChangeText={setCaption}
        placeholder="Kể hội bạn nghe về khoảnh khắc này..."
        value={caption}
      />
      <View style={styles.section}>
        <Text style={[typography.label, { color: colors.ink }]}>Gắn thẻ bạn bè</Text>
        {/* Names, chosen or not, all readable: a faded head is not a choice. */}
        <RosterPicker onToggle={toggle} people={PEOPLE} selected={selectedPeople} tone="accent" />
      </View>
      <View style={[styles.locationRow, { borderTopColor: colors.line, borderBottomColor: colors.line }]}>
        <Ionicons color={colors.accent} name="location" size={20} />
        <View style={styles.flex}>
          <Text style={[typography.label, { color: colors.ink }]}>Đà Lạt, Lâm Đồng</Text>
          <Text style={[typography.caption, { color: colors.inkFaint }]}>Vị trí demo</Text>
        </View>
      </View>
      <View style={styles.section}>
        <Text style={[typography.label, { color: colors.ink }]}>Chia sẻ với</Text>
        <Segmented items={["Chỉ nhóm", "Bạn bè"]} onSelect={setVisibility} selected={visibility} />
      </View>
      <RudiButton
        icon="paper-plane-outline"
        label="Đăng vào tường nhóm"
        onPress={() => router.replace(("/groups/" + DEMO_GROUP.id + "/wall") as never)}
      />
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  flex: { flex: 1 },
  form: { maxWidth: 640 },
  post: { gap: 10, paddingVertical: 16, borderBottomWidth: StyleSheet.hairlineWidth },
  postHeader: { flexDirection: "row", alignItems: "center", gap: 10 },
  postActions: { flexDirection: "row", alignItems: "center", gap: 4 },
  postAction: { minHeight: 48, flexDirection: "row", alignItems: "center", gap: 6, paddingRight: 12 },
  pressed: { opacity: 0.7 },
  wallHero: { gap: 6 },
  wallTitle: { color: mucTrenAnh, fontSize: 26, lineHeight: 31, fontWeight: "800", letterSpacing: -0.7 },
  wallMeta: { flexDirection: "row", alignItems: "center", gap: 9 },
  wallMetaText: { color: lopPhu.trang(0.88), fontSize: 13, lineHeight: 18, fontWeight: "600" },
  dongDem: { flexDirection: "row", alignItems: "center", gap: 10 },
  hangNguoi: { flexDirection: "row", alignItems: "center", gap: 12, minHeight: 56, paddingVertical: 8, borderBottomWidth: StyleSheet.hairlineWidth },
  sharePrompt: { flexDirection: "row", alignItems: "center", gap: 10, minHeight: 56 },
  promptField: { flex: 1, minHeight: 48, borderWidth: 1, paddingHorizontal: 14, justifyContent: "center" },
  albumDau: { gap: 4 },
  albumToolbar: { flexDirection: "row", alignItems: "center", justifyContent: "space-between", gap: 10 },
  gridPhoto: { position: "relative" },
  selectionBadge: { position: "absolute", top: 8, right: 8, width: 30, height: 30, borderRadius: 15, alignItems: "center", justifyContent: "center", backgroundColor: lopPhu.xam(0.5), borderWidth: 2, borderColor: mucTrenAnh },
  section: { gap: 10 },
  locationRow: { flexDirection: "row", alignItems: "center", gap: 12, paddingVertical: 12, borderTopWidth: StyleSheet.hairlineWidth, borderBottomWidth: StyleSheet.hairlineWidth },
});
