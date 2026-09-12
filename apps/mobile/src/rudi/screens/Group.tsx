/**
 * The fixture group (dev door): its chat, the AI itinerary draft and a vote.
 * The live counterparts are `chat/GroupChatLive.tsx` and the server's cards;
 * the pieces that matter for the journal's grammar are shared -- `HangChang`
 * for a route, `Avatar` for a person, one small violet line for the AI.
 */
import { Ionicons } from "@expo/vector-icons";
import * as Haptics from "expo-haptics";
import { useRouter } from "expo-router";
import { useState } from "react";
import { Pressable, StyleSheet, Text, TextInput, View, useWindowDimensions } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";

import { chuLon } from "../adaptive";
import { DEMO_GROUP, LOAI_MAU, PEOPLE, PLACES, VOTE_PLACE_IDS } from "../fixtures";
import { banTinhCua, loaiSoCua } from "../so/ban-tinh";
import { CAP_DEMO } from "../to-giay/fixtures-doi";
import { useSoDoi } from "../to-giay/SoDoi";
import { HangToGiay } from "./hai-nguoi/HangToGiay";
import { guTheoLoai } from "../kham-pha/dia-diem";
import { GuGlyph } from "../ui/art/Gu";
import { noiLuu } from "../luu-tru";
import { useRudiSession } from "../session";
import { typography, useRudiTheme } from "../theme";
import {
  Chip,
  DemoBadge,
  Heading,
  IconButton,
  Inline,
  ProgressBar,
  RudiButton,
  RudiScreen,
  TopBar,
} from "../ui";
import { Avatar } from "../ui/Avatar";
import { Money } from "../ui/Money";
import { HangChang } from "./keo/HangChang";

function ChatBubble({
  person,
  time,
  children,
  own = false,
  dauChuoi = true,
  cuoiChuoi = true,
}: {
  person: (typeof PEOPLE)[number];
  time: string;
  children: string;
  own?: boolean;
  /** First message of a run by this person: the name is printed once, here. */
  dauChuoi?: boolean;
  /** Last message of the run: the initial sits at its foot, once. */
  cuoiChuoi?: boolean;
}) {
  const { colors } = useRudiTheme();
  return (
    <View style={[styles.messageRow, own && styles.messageOwn]}>
      {!own ? (cuoiChuoi ? <Avatar name={person.name} size={30} /> : <View style={styles.choChuDau} />) : null}
      <View style={[styles.messageBlock, own && styles.messageBlockOwn]}>
        {!own && dauChuoi ? <Text style={[typography.caption, styles.sender, { color: colors.inkSoft }]}>{person.name}</Text> : null}
        <View
          style={[
            styles.bubble,
            {
              backgroundColor: own ? colors.accent : colors.card,
              borderColor: own ? colors.accent : colors.line,
            },
          ]}
        >
          <Text style={[typography.body, { color: own ? colors.accentInk : colors.ink }]}>{children}</Text>
        </View>
        {cuoiChuoi ? <Text style={[typography.caption, styles.messageTime, { color: colors.inkFaint }]}>{time}</Text> : null}
      </View>
    </View>
  );
}

export function GroupChatScreen({ embeddedInTabs = false, contextId }: { embeddedInTabs?: boolean; contextId?: string } = {}) {
  const { fontScale } = useWindowDimensions();
  // The fixture pair notebook shares this chat body and swaps only the pinned
  // line under the title: the outing pin belongs to the group, the paper line
  // to the two-person notebook (spec «Nếp truyền giấy» §12: the group screen
  // does not change; this is the one branch, keyed by the fixture pair's id).
  const laCapDemo = contextId === CAP_DEMO.id;
  const so = useSoDoi();
  const tuVung = banTinhCua(loaiSoCua({ kind: laCapDemo ? "pair" : "group" }, { bat: so.batDoi })).tuVung;
  const chuLonHon = chuLon(fontScale);
  const router = useRouter();
  const { colors, radius } = useRudiTheme();
  const session = useRudiSession();
  const [draft, setDraft] = useState("");
  const [attachmentOpen, setAttachmentOpen] = useState(false);
  // The AI sheet leads with the decision and keeps its reasoning behind one
  // opening (re-audit 10/09, R5: the sheet was a heading, three lines, two
  // buttons and two footers, and it broke the thread's rhythm).
  const [moViSao, setMoViSao] = useState(false);

  const sendMessage = () => {
    const message = draft.trim();
    if (!message) return;
    session.sendChat(message);
    setDraft("");
    setAttachmentOpen(false);
    void Haptics.notificationAsync(Haptics.NotificationFeedbackType.Success);
  };

  const attach = (label: string) => {
    setDraft((current) => `${current}${current ? " " : ""}${label}`);
    setAttachmentOpen(false);
  };

  const composer = (
    <View style={styles.composerShell}>
      {attachmentOpen ? (
        <Inline gap={7} style={styles.attachmentTray}>
          <Chip icon="images-outline" label="Ảnh" onPress={() => attach("[Ảnh chuyến đi]")} />
          <Chip icon="location-outline" label="Vị trí" onPress={() => attach("[Vị trí]")} />
          <Chip icon="receipt-outline" label="Chi phí" onPress={() => attach("[Khoản chi]")} />
        </Inline>
      ) : null}
      <View style={[styles.composer, { backgroundColor: colors.card, borderColor: colors.line }]}>
        <IconButton
          accessibilityLabel={attachmentOpen ? "Đóng tệp đính kèm" : "Đính kèm"}
          icon={attachmentOpen ? "close" : "add-circle-outline"}
          onPress={() => setAttachmentOpen((value) => !value)}
          quiet
          selected={attachmentOpen}
        />
        <TextInput
          accessibilityLabel="Ô soạn tin"
          cursorColor={colors.accent}
          onChangeText={setDraft}
          onSubmitEditing={sendMessage}
          placeholder={laCapDemo ? `Nhắn ${so.tenNguoiKia}...` : "Nhắn Team Đà Lạt..."}
          placeholderTextColor={colors.inkFaint}
          returnKeyType="send"
          style={[typography.body, styles.oNhap, { color: colors.ink }]}
          value={draft}
        />
        <IconButton
          accessibilityLabel="Gửi tin nhắn"
          dim={draft.trim().length === 0}
          icon="arrow-up"
          onPress={sendMessage}
          solid={draft.trim().length > 0}
        />
      </View>
    </View>
  );

  return (
    <RudiScreen
      avoidKeyboard
      bottomInset={16}
      footer={composer}
      // The tab navigator already keeps its bar out of this screen; the old 92
      // here was a second bar's worth of empty paper under the composer.
      footerInset={12}
      // The name of the room and the outing it is about stay put; the thread
      // scrolls under them, so opening the tab never hides which group this is.
      header={
        laCapDemo ? (
          <>
            <TopBar back={!embeddedInTabs} subtitle={so.batDoi ? "Một đôi" : "Hai người bạn"} title={so.tenNguoiKia} />
            <HangToGiay cauMo={tuVung.cauMo} onPress={() => router.push(`/groups/${contextId}/to-giay` as never)} tieuDe={tuVung.tenKhongGian} toMo={so.toMo} toiId={so.toiId} />
          </>
        ) : (
        <>
          <TopBar
            back={!embeddedInTabs}
            title={DEMO_GROUP.name}
            subtitle={`${PEOPLE.length} thành viên`}
            right={
              <IconButton
                accessibilityLabel="Thông tin nhóm"
                icon="information-circle-outline"
                onPress={() => router.push(("/groups/" + DEMO_GROUP.id + "/wall") as never)}
                quiet
              />
            }
          />
          {/* The outing being talked about: one pinned line, not a photo card. */}
          <Pressable
            accessibilityLabel={`Mở chuyến ${DEMO_GROUP.tripName}`}
            accessibilityRole="button"
            onPress={() => router.push(session.tripPath("/timeline") as never)}
            style={({ pressed }) => [styles.tripPin, { borderColor: colors.line }, pressed && styles.pressed]}
          >
            <View style={[styles.tripPinIcon, { backgroundColor: colors.accentSoft, borderRadius: radius.small }]}>
              <Ionicons color={colors.accent} name="calendar-outline" size={20} />
            </View>
            <View style={styles.tripPinText}>
              <Text numberOfLines={1} style={[typography.label, { color: colors.ink }]}>{DEMO_GROUP.tripName}</Text>
              {/* Keep the dates readable at large text -- and keep the words
                  that say what the row IS. Measured on the emulator at font
                  scale 1.3 (12/09): only the trailing «· 8 người» was clipped,
                  and that count is already in the header above. Dropping the
                  frame as well left a pinned row that reads as a bare date. */}
              <Text numberOfLines={2} style={[typography.caption, { color: colors.inkSoft }]}>Chuyến đi sắp tới · 17 - 19/10/2026</Text>
            </View>
            <Ionicons color={colors.inkFaint} name="chevron-forward" size={18} />
          </Pressable>
        </>
        )
      }
      keepEnd
      testID="group-chat-screen"
    >
      {laCapDemo ? (
        <ThanCapDemo tenNguoiKia={so.tenNguoiKia} tinCuaToi={session.chatMessages} />
      ) : (
      <>
      <View style={styles.dayDivider}>
        <View style={[styles.line, { backgroundColor: colors.line }]} />
        <Text style={[typography.caption, { color: colors.inkFaint }]}>Hôm nay</Text>
        <View style={[styles.line, { backgroundColor: colors.line }]} />
      </View>
      <View style={styles.messages}>
        <ChatBubble person={PEOPLE[1]} time="09:42">Cuối tuần tháng 10 đi Đà Lạt không mọi người?</ChatBubble>
        <ChatBubble person={PEOPLE[2]} time="09:44">Đi chứ! Tớ vote săn mây với BBQ nha 🌤️</ChatBubble>
        <ChatBubble person={PEOPLE[0]} time="09:46" own>Để Rủ Đi gom gu rồi lên lịch trình thử nhé.</ChatBubble>
        {/* The AI's proposal is a sheet of paper in the thread: the heading
            speaks first at the size of a message, not a screen; one line says
            what the plan is; the next decision («Xem lịch trình») comes at once;
            the why sits behind one opening; and the author signs at the foot (a
            label over the heading is a kicker, which the craft floor bans). The
            draft state is said once, by the «AI nháp» badge -- not again in the
            body (Luật Nói Một Lần). No invented figures. */}
        <View style={[styles.aiSheet, { backgroundColor: colors.card, borderColor: colors.line, borderRadius: radius.base }]}>
          <Text style={[typography.title, { color: colors.ink }]}>Rủ Đi đã phác một plan</Text>
          <Text style={[typography.body, { color: colors.ink }]}>3 ngày 2 đêm · đồ ăn local · săn mây</Text>
          {/* Both decisions are named in words: a bar-chart glyph alone was read as
              «statistics» by a reviewer who had not been told what it opens
              (review 11/09, A4). The visible label is the verb the product uses
              («Bình chọn»); the accessible name keeps the fuller «Mở bình chọn»
              the flows tap. Primary takes the row, secondary is content-sized;
              at large text the pair stacks so neither label wraps. The secondary
              keeps a neutral edge (`lineStrong`) and carries the tone only in its
              label: a violet stroke was the crisper shape of the two and won the
              eye over the soft fill once they stacked (finish review 11/09). */}
          <View style={chuLonHon ? styles.hanhDongDoc : styles.hanhDongNgang}>
            <RudiButton
              full={chuLonHon}
              label="Xem lịch trình"
              onPress={() => router.push(session.tripPath("/itinerary") as never)}
              style={chuLonHon ? undefined : styles.flex}
              tone="ai"
              variant="soft"
            />
            <RudiButton
              accessibilityLabel="Mở bình chọn"
              full={chuLonHon}
              label="Bình chọn"
              onPress={() => router.push("/votes/diem-den")}
              style={{ borderColor: colors.lineStrong }}
              tone="ai"
              variant="outline"
            />
          </View>
          {/* One opening for the reasoning, the «Cách tính» shape from Thành tích. */}
          <Pressable
            accessibilityRole="button"
            accessibilityState={{ expanded: moViSao }}
            onPress={() => setMoViSao((v) => !v)}
            style={styles.cuaMo}
          >
            <Text style={[typography.label, styles.flex, { color: colors.inkSoft }]}>Vì sao phác vậy</Text>
            <Ionicons color={colors.inkFaint} name={moViSao ? "chevron-up" : "chevron-down"} size={18} />
          </Pressable>
          {moViSao ? (
            <Text style={[typography.body, { color: colors.inkSoft }]}>
              Ưu tiên đồ ăn local và săn mây theo gu nhóm; các điểm xếp gần nhau để đỡ mệt.
            </Text>
          ) : null}
          <View style={styles.aiSheetHeader}>
            <Ionicons color={colors.ai} name="sparkles" size={15} />
            <Text style={[typography.caption, styles.flex, { color: colors.ai }]}>Rủ Đi AI</Text>
            <DemoBadge label="AI nháp" />
          </View>
        </View>
        <ChatBubble person={PEOPLE[3]} time="09:51">Plan xịn đó, mình bình chọn chỗ BBQ trước đi.</ChatBubble>
        {session.chatMessages.map((message, index) => (
          <ChatBubble
            cuoiChuoi={index === session.chatMessages.length - 1}
            key={`${message}-${index}`}
            person={PEOPLE[0]}
            time="Bây giờ"
            own
          >
            {message}
          </ChatBubble>
        ))}
      </View>
      </>
      )}
    </RudiScreen>
  );
}

export function AiItineraryScreen() {
  const router = useRouter();
  const { colors } = useRudiTheme();
  const session = useRudiSession();
  const [activeDay, setActiveDay] = useState(0);
  const days = session.itinerary;
  const day = days[activeDay] ?? days[0];
  const insets = useSafeAreaInsets();
  // The two decisions stay reachable above the gesture bar however long the
  // day runs; in the scroll they sat under the system's own line.
  const hanhDong = (
    <Inline gap={10}>
      <RudiButton
        full={false}
        icon="create-outline"
        label={session.itineraryEditing ? "Xong chỉnh" : "Chỉnh lịch trình"}
        onPress={() => session.setItineraryEditing(!session.itineraryEditing)}
        style={styles.flex}
        tone="ai"
        variant="outline"
      />
      <RudiButton
        full={false}
        icon="checkmark-circle-outline"
        label="Dùng plan này"
        onPress={() => {
          session.setItineraryEditing(false);
          router.replace(session.tripPath("/timeline") as never);
        }}
        style={styles.flex}
        tone="ai"
      />
    </Inline>
  );

  return (
    <RudiScreen footer={hanhDong} footerInset={Math.max(insets.bottom, 12) + 4} tone="ai" testID="ai-itinerary-screen">
      <TopBar title="Lịch trình AI" right={<DemoBadge compactLabel="Nháp" label="AI nháp" />} />
      <View style={styles.itineraryHead}>
        <Text style={[typography.h1, { color: colors.ink }]}>{session.tripName}</Text>
        <Text style={[typography.body, { color: colors.inkSoft }]}>17 - 19/10/2026 · 3 ngày 2 đêm</Text>
        <View style={styles.aiLine}>
          <Ionicons color={colors.ai} name="sparkles" size={15} />
          <Text style={[typography.caption, styles.flex, { color: colors.ai }]}>
            Rủ Đi AI phác theo quãng đường. Nét chì là nháp: nhóm sửa được trước khi chốt.
          </Text>
        </View>
      </View>
      <Inline gap={8} wrap>
        {days.map((item, index) => (
          <Chip
            key={item.day}
            label={"Ngày " + (index + 1)}
            onPress={() => setActiveDay(index)}
            selected={activeDay === index}
          />
        ))}
      </Inline>
      <Heading
        size="h2"
        title={day.day}
        subtitle={session.itineraryEditing ? "Đang chỉnh: lên, xuống hoặc xoá." : "Xem trước. Bấm Chỉnh lịch trình để sửa."}
      />
      <View>
        {day.items.map((slot, index) => {
          const noi = slot.placeId ? PLACES.find((p) => p.id === slot.placeId) : undefined;
          return (
          <HangChang
            anh={!session.itineraryEditing && noi?.anh ? { anh: noi.anh, alt: noi.name, loai: LOAI_MAU[noi.category] } : null}
            cuoi={index === day.items.length - 1}
            gio={slot.time}
            key={slot.time + slot.title + index}
            phac
            phai={
              session.itineraryEditing ? (
                <Inline gap={2}>
                  <IconButton accessibilityLabel="Lên" icon="chevron-up" onPress={() => session.moveItinerarySlot(activeDay, index, -1)} quiet />
                  <IconButton accessibilityLabel="Xuống" icon="chevron-down" onPress={() => session.moveItinerarySlot(activeDay, index, 1)} quiet />
                  <IconButton accessibilityLabel="Xóa" icon="trash-outline" onPress={() => session.removeItinerarySlot(activeDay, index)} quiet />
                </Inline>
              ) : null
            }
            phu={noi?.name ?? null}
            phuTone="inkSoft"
            tieuDe={slot.title}
          />
          );
        })}
      </View>
      <View style={[styles.budgetLine, { borderTopColor: colors.line }]}>
        <Money vnd={DEMO_GROUP.budgetPerPersonVnd} />
        <Text style={[typography.caption, { color: colors.inkSoft }]}>dự kiến một người · số tham chiếu của chuyến</Text>
      </View>
    </RudiScreen>
  );
}

const VOTE_OPTIONS = VOTE_PLACE_IDS.map((id) => PLACES.find((place) => place.id === id)!);

export function VotingScreen() {
  const { colors, radius } = useRudiTheme();
  const session = useRudiSession();
  const totalVotes = session.voteTallies.reduce((sum, n) => sum + n, 0);

  return (
    <RudiScreen testID="voting-screen">
      <TopBar title="Bình chọn" right={<DemoBadge />} />
      <Heading
        title="BBQ tối thứ Bảy ở đâu?"
        subtitle="Mỗi người chọn một nơi. Kết quả ẩn đến khi xác nhận. Bản trải nghiệm chỉ ghi phiếu của bạn."
      />
      <View>
        {VOTE_OPTIONS.map((place, index) => {
          const active = session.voteChoice === index;
          const votes = session.voteTallies[index];
          const percent = session.voteConfirmed && totalVotes > 0 ? (votes * 100) / totalVotes : 0;
          return (
            <Pressable
              key={place.id}
              accessibilityRole="radio"
              accessibilityState={{ checked: active }}
              aria-checked={active}
              onPress={() => session.setVoteChoice(index)}
              style={({ pressed }) => [
                styles.voteOption,
                { borderBottomColor: colors.line, backgroundColor: active ? colors.accentSoft : "transparent", borderRadius: radius.small },
                pressed && styles.pressed,
              ]}
            >
              {/* Every option is the category drawn with one pen, never a
                  photograph: a ballot is fair only when no option stands out
                  because the catalogue happens to hold a stock picture of its
                  kind, and a 56dp tile has no room for the credit such a
                  picture must be shown under (review 08/09 vòng 2, F21). */}
              <View style={[styles.voteThumb, styles.voteThumbVe, { borderRadius: radius.small, backgroundColor: colors.card, borderColor: colors.line }]}>
                <GuGlyph id={guTheoLoai(LOAI_MAU[place.category])} size={30} tone="accent" />
              </View>
              <View style={styles.voteBody}>
                <Text style={[typography.title, { color: colors.ink }]}>{place.name}</Text>
                <Text style={[typography.caption, { color: colors.inkSoft }]}>{place.distance} · {place.price}</Text>
                {session.voteConfirmed ? (
                  <View style={styles.voteResult}>
                    <ProgressBar value={percent} />
                    <Text style={[typography.caption, { color: colors.inkSoft }]}>{votes} phiếu của bạn trên bản này</Text>
                  </View>
                ) : null}
              </View>
              <Ionicons color={active ? colors.accent : colors.lineStrong} name={active ? "checkmark-circle" : "ellipse-outline"} size={24} />
            </Pressable>
          );
        })}
      </View>
      {session.voteConfirmed && session.voteChoice !== null ? (
        <View style={[styles.voteSummary, { borderColor: colors.line, borderRadius: radius.small }]}>
          <Ionicons color={colors.accent} name="checkmark-done-outline" size={22} />
          <View style={styles.flex}>
            <Text style={[typography.label, { color: colors.ink }]}>Bạn đã chọn {VOTE_OPTIONS[session.voteChoice].name}</Text>
            <Text style={[typography.caption, { color: colors.inkSoft }]}>
              Phiếu {noiLuu(session.luuTruSong)}. Chưa gửi lên máy chủ.
            </Text>
          </View>
        </View>
      ) : null}
      <RudiButton
        disabled={session.voteChoice === null}
        icon="checkmark"
        label={session.voteConfirmed ? "Đã xác nhận" : "Xác nhận lựa chọn của tôi"}
        onPress={() => session.confirmVote()}
      />
    </RudiScreen>
  );
}

/**
 * The fixture pair's thread: two people, a quiet line each, and whatever I
 * type. No AI plan card, no roster of a group that is not here. Role names
 * only («Người ấy», «Bạn»); the pair notebook's own line lives in the header.
 */
function ThanCapDemo({ tenNguoiKia, tinCuaToi }: { tenNguoiKia: string; tinCuaToi: readonly string[] }) {
  const { colors } = useRudiTheme();
  const nguoiKia = { ...PEOPLE[1], id: "nguoi-ay", name: tenNguoiKia, initials: "N" };
  const toi = { ...PEOPLE[0], id: "toi", name: "Bạn", initials: "B" };
  return (
    <>
      <View style={styles.dayDivider}>
        <View style={[styles.line, { backgroundColor: colors.line }]} />
        <Text style={[typography.caption, { color: colors.inkFaint }]}>Hôm nay</Text>
        <View style={[styles.line, { backgroundColor: colors.line }]} />
      </View>
      <View style={styles.messages}>
        <ChatBubble person={nguoiKia} time="18:02">Tuần này tối thứ Bảy rảnh không?</ChatBubble>
        <ChatBubble own person={toi} time="18:05">Rảnh. Để Nếp phác một tờ rồi mình sửa.</ChatBubble>
        {tinCuaToi.map((message, index) => (
          <ChatBubble cuoiChuoi={index === tinCuaToi.length - 1} key={`${message}-${index}`} own person={toi} time="Bây giờ">
            {message}
          </ChatBubble>
        ))}
      </View>
    </>
  );
}

const styles = StyleSheet.create({
  flex: { flex: 1 },
  tripPin: { flexDirection: "row", alignItems: "center", gap: 12, paddingVertical: 10, borderBottomWidth: StyleSheet.hairlineWidth, minHeight: 56 },
  tripPinIcon: { width: 40, height: 40, alignItems: "center", justifyContent: "center" },
  tripPinText: { flex: 1, gap: 2 },
  dayDivider: { flexDirection: "row", alignItems: "center", gap: 10, paddingHorizontal: 20 },
  line: { flex: 1, height: StyleSheet.hairlineWidth },
  messages: { gap: 10 },
  messageRow: { flexDirection: "row", alignItems: "flex-end", gap: 8, maxWidth: "88%" },
  messageOwn: { alignSelf: "flex-end", justifyContent: "flex-end" },
  messageBlock: { alignItems: "flex-start", gap: 3 },
  messageBlockOwn: { alignItems: "flex-end" },
  choChuDau: { width: 30, height: 30 },
  sender: { marginLeft: 7 },
  bubble: { borderWidth: 1, borderRadius: 17, borderBottomLeftRadius: 5, paddingHorizontal: 13, paddingVertical: 10 },
  messageTime: { paddingHorizontal: 7 },
  aiSheet: { gap: 10, padding: 14, borderWidth: 1, alignSelf: "stretch" },
  aiSheetHeader: { flexDirection: "row", alignItems: "center", gap: 6 },
  hanhDongNgang: { flexDirection: "row", alignItems: "center", gap: 9 },
  hanhDongDoc: { gap: 9 },
  cuaMo: { minHeight: 48, flexDirection: "row", alignItems: "center", gap: 6 },
  composerShell: { gap: 8 },
  attachmentTray: { justifyContent: "center" },
  composer: { flexDirection: "row", alignItems: "center", gap: 6, padding: 6, borderWidth: 1, borderRadius: 22 },
  oNhap: { flex: 1, minHeight: 48, paddingHorizontal: 10, paddingVertical: 8 },
  itineraryHead: { gap: 6 },
  aiLine: { flexDirection: "row", alignItems: "center", gap: 6, marginTop: 4 },
  budgetLine: { gap: 2, paddingTop: 12, borderTopWidth: StyleSheet.hairlineWidth },
  voteOption: { flexDirection: "row", alignItems: "center", gap: 12, paddingVertical: 10, paddingHorizontal: 4, borderBottomWidth: StyleSheet.hairlineWidth, minHeight: 72 },
  voteThumb: { width: 56, height: 56 },
  voteThumbVe: { alignItems: "center", justifyContent: "center", borderWidth: StyleSheet.hairlineWidth },
  voteBody: { flex: 1, gap: 3 },
  voteResult: { gap: 4, marginTop: 4 },
  voteSummary: { flexDirection: "row", alignItems: "center", gap: 12, padding: 12, borderWidth: 1, borderStyle: "dashed" },
  pressed: { opacity: 0.78 },
});
