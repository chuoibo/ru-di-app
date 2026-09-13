/**
 * The fixture trip (dev door): create an outing, the trip's timeline tab, and
 * the manual check-in. The live counterparts are in `keo/`; both draw stops
 * with `HangChang` and dates with `nhip-keo.ts`, so «còn N ngày» here is
 * counted from the sample trip's dates rather than typed.
 */
import { Ionicons } from "@expo/vector-icons";
import { LinearGradient } from "expo-linear-gradient";
import { useRouter } from "expo-router";
import { useMemo, useState } from "react";
import { Pressable, ScrollView, StyleSheet, Switch, Text, useWindowDimensions, View } from "react-native";

import { tabBarHeight } from "../adaptive";
import { DEMO_GROUP, LOAI_MAU, PEOPLE, PLACES, demoAssets, formatVnd } from "../fixtures";
import { homNay, nhanNhip, nhipKeo } from "../keo/nhip-keo";
import { noiLuu, noiLuuNgan } from "../luu-tru";
import { nhanKhoangNgay } from "../../screens/len-plan/buoi-di";
import { useRudiSession } from "../session";
import { displayFace, lopPhu, mucTrenAnh, typography, useRudiTheme } from "../theme";
import {
  Chip,
  DemoBadge,
  Field,
  Heading,
  IconButton,
  Inline,
  ListRow,
  Photo,
  RudiButton,
  RudiScreen,
  TopBar,
} from "../ui";
import { Avatar, AvatarStack } from "../ui/Avatar";
import { Money } from "../ui/Money";
import { RosterPicker } from "../ui/RosterPicker";
import { Sheet } from "../ui/Sheet";
import { Stamp } from "../ui/Stamp";
import { HangChang } from "./keo/HangChang";
import { chieuTuNgay, ganMappedVaoCho, idSlot } from "../hanh-trinh/chieu";
import { useCheDoLichTrinh } from "../hanh-trinh/che-do";
import { SoHanhTrinh } from "../hanh-trinh/SoHanhTrinh";
import type { BuoiDi } from "../../screens/len-plan/buoi-di";
import { ngayMacDinh } from "../hanh-trinh/ke-hoach";
import { ThanhCheDo } from "../hanh-trinh/ThanhCheDo";

import { choTuId } from "../hanh-trinh/toa-do-mau";

/** «17/10/2026» (the fixture's own format) as the ISO day `nhip-keo` reads. */
function isoTu(ddmmyyyy: string): string {
  const [d, m, y] = ddmmyyyy.split("/");
  if (!d || !m || !y) return ddmmyyyy;
  return `${y}-${m.padStart(2, "0")}-${d.padStart(2, "0")}`;
}

export function CreateOutingScreen() {
  const router = useRouter();
  const { colors } = useRudiTheme();
  const session = useRudiSession();
  const selected = session.selectedMemberIds;

  return (
    <RudiScreen contentStyle={styles.form} testID="create-outing-screen">
      <TopBar title="Tạo cuộc hẹn" right={<DemoBadge />} />
      <Heading
        title="Hội mình đi đâu?"
        subtitle="Một lời rủ: tên, nơi đến, ngày và ngân sách. Chốt chỗ và lịch trình làm cùng nhau sau."
      />
      <View style={styles.khoi}>
        <Field
          icon="flag-outline"
          label="Tên cuộc hẹn"
          onChangeText={session.setTripName}
          placeholder="Ví dụ: Đà Lạt cuối tuần"
          value={session.tripName}
        />
        <Field
          icon="location-outline"
          label="Điểm đến"
          onChangeText={session.setDestination}
          placeholder="Đà Lạt, Lâm Đồng"
          value={session.destination}
        />
      </View>
      <View style={styles.khoi}>
        <Inline gap={10}>
          <View style={styles.flex}>
            <Field icon="calendar-outline" label="Ngày đi" value={session.startDate} />
          </View>
          <View style={styles.flex}>
            <Field icon="calendar-outline" label="Ngày về" value={session.endDate} />
          </View>
        </Inline>
        <Field
          icon="wallet-outline"
          keyboardType="number-pad"
          label="Ngân sách mỗi người"
          value={formatVnd(DEMO_GROUP.budgetPerPersonVnd)}
        />
      </View>
      <View style={styles.khoi}>
        <View style={styles.sectionTitleRow}>
          <View style={styles.flex}>
            <Text style={[typography.h2, { color: colors.ink }]}>Rủ hội bạn</Text>
            <Text style={[typography.caption, { color: colors.inkSoft }]}>{selected.length}/{PEOPLE.length} người được chọn</Text>
          </View>
          <Pressable accessibilityRole="button" hitSlop={8} onPress={() => session.selectAllMembers()} style={({ pressed }) => [styles.chonTatCa, pressed && styles.pressed]}>
            <Text style={[typography.label, { color: colors.accent }]}>Chọn tất cả</Text>
          </Pressable>
        </View>
        {/* Names, not eight tiny coloured heads: the same picker the bill uses, in the invitation's own tone. */}
        <RosterPicker onToggle={session.toggleMember} people={PEOPLE} selected={selected} tone="accent" />
      </View>
      <View style={[styles.aiRow, { borderTopColor: colors.line, borderBottomColor: colors.line }]}>
        <Ionicons color={colors.ai} name="sparkles" size={22} />
        <View style={styles.flex}>
          <Text style={[typography.label, { color: colors.ink }]}>Nhờ Rủ Đi gợi ý lịch trình</Text>
          <Text style={[typography.caption, { color: colors.inkSoft }]}>Dựa trên gu của {selected.length} thành viên; bạn sửa được trước khi chốt.</Text>
        </View>
        <Switch
          accessibilityLabel="Nhờ Rủ Đi gợi ý lịch trình"
          onValueChange={session.setAiSuggest}
          thumbColor={colors.card}
          trackColor={{ false: colors.lineStrong, true: colors.ai }}
          value={session.aiSuggest}
        />
      </View>
      <RudiButton
        disabled={!session.tripName || selected.length === 0}
        icon="arrow-forward"
        label="Tạo cuộc hẹn"
        onPress={() => router.replace(session.tripPath("/timeline") as never)}
      />
    </RudiScreen>
  );
}

export function TripTimelineScreen() {
  const router = useRouter();
  const { colors } = useRudiTheme();
  const session = useRudiSession();
  const { fontScale } = useWindowDimensions();
  const [day, setDay] = useState(0);
  const [menuOpen, setMenuOpen] = useState(false);
  const che = useCheDoLichTrinh();
  const hanhTrinh = che.cheDo === "hanh-trinh";
  const days = session.itinerary;
  const current = days[day] ?? days[0];
  const nhan = nhanNhip(nhipKeo(isoTu(session.startDate), isoTu(session.endDate), homNay()));
  const diCung = PEOPLE.filter((person) => session.selectedMemberIds.includes(person.id));
  const cho = useMemo(() => choTuId(PLACES), []);
  const hanh = useMemo(
    () => chieuTuNgay({ day: current.day, items: current.items }, cho),
    [current, cho],
  );

  const doiNgay = (index: number) => {
    setDay(index);
    const next = days[index];
    if (!next) return;
    const ids = next.items.map((slot, i) => idSlot(slot, i));
    if (che.selectedActivityId && !ids.includes(che.selectedActivityId)) che.chonHoatDong(null);
  };

  const [savedJourney, setSavedJourney] = useState<BuoiDi | null>(null);
  const journeyOuting = useMemo<BuoiDi>(() => savedJourney ?? {
    id: "fixture-journey", context_id: DEMO_GROUP.id, created_by_id: "fixture", title: session.tripName,
    starts_on: isoTu(session.startDate), ends_on: isoTu(session.endDate), headcount: 4, budget_per_person_vnd: 0, created_at: "", timeline_revision: 0,
    days: days.map((_, i) => { const date = new Date(`${isoTu(session.startDate)}T12:00:00Z`); date.setUTCDate(date.getUTCDate() + i); return ngayMacDinh(date.toISOString().slice(0, 10)); }),
    stops: days.flatMap((d, di) => d.items.map((item, i) => { const date = new Date(`${isoTu(session.startDate)}T12:00:00Z`); date.setUTCDate(date.getUTCDate() + di); return { id: `fixture-${di}-${i}`, position: i, at: item.time, label: item.title, place_name: null, place_id: item.placeId ?? null, day: date.toISOString().slice(0, 10), duration_minutes: null, time_locked: true, meeting_point: null }; })),
  }, [savedJourney, days, session.tripName, session.startDate, session.endDate]);
  const saveJourney = (next: BuoiDi) => {
    setSavedJourney(next);
    days.forEach((d, i) => { const date = new Date(`${isoTu(session.startDate)}T12:00:00Z`); date.setUTCDate(date.getUTCDate() + i); const iso = date.toISOString().slice(0, 10); session.datHangNgay(i, next.stops.filter((stop) => stop.day === iso).map((stop) => ({ time: stop.at, title: stop.label, icon: "location-outline", color: "accent", ...(stop.place_id ? { placeId: stop.place_id } : {}) }))); });
  };
  const journeyDate = journeyOuting.days?.[day]?.day ?? journeyOuting.starts_on;
  const journeySelection = journeyOuting.stops.filter((s) => s.day === journeyDate).map((s, i) => ({ id: s.id, timelineId: idSlot({ time: s.at, title: s.label, placeId: s.place_id ?? undefined }, i) }));
  const journeyController = { ...che,
    selectedActivityId: journeySelection.find((s) => s.timelineId === che.selectedActivityId)?.id ?? null,
    chonHoatDong: (id: string | null) => che.chonHoatDong(journeySelection.find((s) => s.id === id)?.timelineId ?? null),
  };

  const header = (
    <View style={styles.dauMan}>
      <TopBar
        back={false}
        title={DEMO_GROUP.name}
        subtitle={nhanKhoangNgay(isoTu(session.startDate), isoTu(session.endDate))}
        right={
          <IconButton
            accessibilityLabel="Tùy chọn"
            icon="ellipsis-horizontal"
            onPress={() => setMenuOpen(true)}
            quiet
          />
        }
      />
      {hanhTrinh ? null : <Inline gap={8} wrap>
        {days.map((item, index) => (
          <Chip key={item.day} label={"Ngày " + (index + 1)} onPress={() => doiNgay(index)} selected={day === index} />
        ))}
      </Inline>}
      <ThanhCheDo cheDo={che.cheDo} onDoi={che.doiCheDo} />
      {hanhTrinh ? null : (
        <View style={styles.sectionTitleRow}>
          <View style={styles.flex}>
            <Text style={[typography.h2, { color: colors.ink }]}>{current.day}</Text>
            <Text style={[typography.caption, { color: colors.inkSoft }]}>{current.items.length} hoạt động</Text>
          </View>
          <IconButton
            accessibilityLabel="Mở lịch trình AI"
            icon="sparkles"
            onPress={() => router.push(session.tripPath("/itinerary") as never)}
            selected
            tone="ai"
          />
        </View>
      )}
    </View>
  );

  return (
    <RudiScreen
      bottomInset="tab"
      contentStyle={styles.mapInner}
      header={header}
      overlay={
        <Sheet accessibilityLabel="Tùy chọn chuyến đi" onClose={() => setMenuOpen(false)} open={menuOpen}>
          <View style={styles.khay}>
            <Text style={[typography.h2, { color: colors.ink }]}>{session.tripName}</Text>
            <RudiButton
              label="Mở lịch trình AI"
              onPress={() => {
                setMenuOpen(false);
                router.push(session.tripPath("/itinerary") as never);
              }}
              variant="ghost"
            />
            <RudiButton
              label="Check-in nhóm"
              onPress={() => {
                setMenuOpen(false);
                router.push("/check-ins/new");
              }}
              variant="ghost"
            />
            <RudiButton
              label="Tường nhóm"
              onPress={() => {
                setMenuOpen(false);
                router.push(("/groups/" + DEMO_GROUP.id + "/wall") as never);
              }}
              variant="ghost"
            />
          </View>
        </Sheet>
      }
      padded={false}
      scroll={false}
      testID="trip-timeline-screen"
    >
      <View style={{ flex: 1, display: hanhTrinh ? "flex" : "none" }}>
        <SoHanhTrinh fixture controller={journeyController} initialDay={journeyDate} onDay={(date) => { const index = journeyOuting.days?.findIndex((d) => d.day === date) ?? 0; if (index >= 0) setDay(index); }} outing={journeyOuting} places={cho} onSaved={saveJourney} onTimeline={() => che.doiCheDo("lich-trinh")} bottom={tabBarHeight(fontScale)} />
      </View>
      {hanhTrinh ? null : (
        <ScrollView keyboardShouldPersistTaps="handled" contentContainerStyle={{ padding: 16, paddingBottom: tabBarHeight(fontScale) + 48, gap: 20 }}>
          <Photo
            height={200}
            radius={20}
            source={demoAssets.road}
            overlay={
              <>
                <LinearGradient
                  colors={[lopPhu.toi(0.02), lopPhu.toi(0.82)]}
                  style={StyleSheet.absoluteFill}
                />
                <View style={styles.tripHeroBadge}><DemoBadge /></View>
                <View style={styles.tripHeroCopy}>
                  <Text style={styles.tripTitle}>{session.tripName}</Text>
                  <Text style={styles.tripMeta}>{session.destination} · 3 ngày 2 đêm · {diCung.length} người</Text>
                </View>
              </>
            }
          />
          <View style={styles.tomTat}>
            <View style={styles.oTomTat}>
              <Money vnd={DEMO_GROUP.budgetPerPersonVnd} />
              <Text style={[typography.caption, { color: colors.inkSoft }]}>dự kiến một người</Text>
            </View>
            <View style={styles.oTomTat}>
              <AvatarStack max={4} people={diCung.map((p) => ({ name: p.name }))} />
              <Text style={[typography.caption, { color: colors.inkSoft }]}>{diCung.length} tham gia</Text>
            </View>
            {nhan ? <Stamp label={nhan} tilt={-2} /> : null}
          </View>
          <View>
            {current.items.map((slot, index) => {
              const noi = slot.placeId ? PLACES.find((p) => p.id === slot.placeId) : undefined;
              const id = idSlot(slot, index);
              return (
                <HangChang
                  chon={che.selectedActivityId === id}
                  cuoi={index === current.items.length - 1}
                  gio={slot.time}
                  key={slot.time + slot.title + index}
                  onPress={() => {
                    if (che.selectedActivityId === id && slot.placeId) {
                      router.push(("/places/" + slot.placeId) as never);
                      return;
                    }
                    che.chonHoatDong(id);
                  }}
                  anh={noi?.anh ? { anh: noi.anh, alt: noi.name, loai: LOAI_MAU[noi.category] } : null}
                  phu={noi ? noi.name : slot.placeId ? "Địa điểm · bấm để mở" : "Cả nhóm"}
                  phuTone={slot.placeId ? "accent" : "inkFaint"}
                  tieuDe={slot.title}
                />
              );
            })}
          </View>
          <ListRow icon="location" onPress={() => router.push("/check-ins/new")} subtitle="Check-in để giữ lại khoảnh khắc cùng nhóm." title="Đến nơi rồi?" />
        </ScrollView>
      )}
    </RudiScreen>
  );
}

export function CheckInScreen() {
  const router = useRouter();
  const { colors } = useRudiTheme();
  const session = useRudiSession();
  const arrived = session.checkedInIds.length;
  // The person just checked in under this finger: their seal lands once.
  const [vuaToi, setVuaToi] = useState<string | null>(null);
  const missing = PEOPLE.filter((person) => !session.checkedInIds.includes(person.id));
  const daToi = PEOPLE.filter((person) => session.checkedInIds.includes(person.id));

  return (
    <RudiScreen testID="check-in-screen">
      <TopBar title="Check-in nhóm" right={<DemoBadge />} />
      <Heading title={`${arrived}/${PEOPLE.length} thành viên đã tới`} subtitle="Quảng trường Lâm Viên · Đà Lạt" />
      <AvatarStack max={8} people={daToi.map((p) => ({ name: p.name }))} tone="split" />
      {/* No map. This build reads no GPS; drawing a map box would promise one. */}
      <View style={[styles.viTri, { borderTopColor: colors.line, borderBottomColor: colors.line }]}>
        <View style={styles.flex}>
          <Text style={[typography.label, { color: colors.ink }]}>
            {session.locationSharing ? "Đang chia sẻ vị trí đến 11:30" : "Chưa chia sẻ vị trí"}
          </Text>
          <Text style={[typography.caption, { color: colors.inkSoft }]}>
            Bản trải nghiệm không đọc GPS: bạn tự đánh dấu, trạng thái {noiLuu(session.luuTruSong)}.
          </Text>
        </View>
        <RudiButton
          compact
          full={false}
          label={session.locationSharing ? "Dừng chia sẻ" : "Chia vị trí"}
          onPress={() => session.setLocationSharing(!session.locationSharing)}
          variant="outline"
        />
      </View>
      <View style={styles.khoi}>
        <Text style={[typography.h2, { color: colors.ink }]}>Ai đã tới</Text>
        {PEOPLE.map((person) => {
          const here = session.checkedInIds.includes(person.id);
          return (
            <Pressable
              key={person.id}
              accessibilityLabel={person.name}
              accessibilityRole="checkbox"
              accessibilityState={{ checked: here }}
              aria-checked={here}
              onPress={() => {
                // Landing only for a check-in, never for an undo.
                if (here) setVuaToi(null);
                else setVuaToi(person.id);
                session.toggleCheckIn(person.id);
              }}
              style={({ pressed }) => [styles.checkRow, { borderBottomColor: colors.line }, pressed && styles.pressed]}
            >
              <Avatar name={person.name} ring={here} size={40} tone="split" />
              <View style={styles.flex}>
                <Text style={[typography.label, { color: colors.ink }]}>{person.name}</Text>
                {/* The stamp is the state; a caption saying it again was the
                    same word twice on one row (report 07/09 §4.6). */}
                {here ? null : <Text style={[typography.note, { color: colors.inkSoft }]}>Chưa tới</Text>}
              </View>
              {here ? (
                <Stamp dong={vuaToi === person.id} label="Đã tới" tilt={-2} tone="split" />
              ) : (
                <Ionicons color={colors.lineStrong} name="ellipse-outline" size={22} />
              )}
            </Pressable>
          );
        })}
      </View>
      {missing.length ? (
        <Text style={[typography.caption, { color: colors.inkSoft }]}>
          {missing.map((person) => person.name).join(", ")} chưa check-in.
        </Text>
      ) : (
        <Text style={[typography.caption, { color: colors.split }]}>Đủ 8 người.</Text>
      )}
      <View>
        <Text style={[typography.caption, { color: colors.inkFaint }]}>Điểm đến tiếp theo</Text>
        <Text style={[typography.title, { color: colors.ink }]}>Still Cafe · 10:00</Text>
      </View>
      <Inline gap={10}>
        <RudiButton
          full={false}
          icon="notifications-outline"
          label="Nhắc thành viên"
          onPress={() => session.remindPending()}
          style={styles.flex}
          variant="outline"
        />
        <RudiButton
          full={false}
          icon="location"
          label="Tôi đã tới"
          onPress={() => {
            session.checkInSelf();
            router.replace(("/groups/" + DEMO_GROUP.id + "/wall") as never);
          }}
          style={styles.flex}
        />
      </Inline>
      {session.remindedPending ? (
        <Text style={[typography.caption, styles.demoNote, { color: colors.accent }]}>
          Đã ghi nhắc {missing.length} người {noiLuuNgan(session.luuTruSong)}. Chưa gửi push.
        </Text>
      ) : null}
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  flex: { flex: 1 },
  // The map is the page here: it runs to the bottom edge and the journey
  // panel keeps its own clearance over the tab bar.
  mapInner: { flex: 1, paddingBottom: 0 },
  dauMan: { gap: 10, paddingBottom: 8 },
  form: { maxWidth: 640 },
  khoi: { gap: 10 },
  khay: { gap: 6 },
  sectionTitleRow: { flexDirection: "row", alignItems: "center", justifyContent: "space-between", gap: 12 },
  chonTatCa: { minHeight: 48, justifyContent: "center", paddingHorizontal: 6 },
  aiRow: { flexDirection: "row", alignItems: "center", gap: 12, paddingVertical: 14, borderTopWidth: StyleSheet.hairlineWidth, borderBottomWidth: StyleSheet.hairlineWidth },
  pressed: { opacity: 0.75 },
  tripHeroBadge: { position: "absolute", right: 12, top: 12 },
  tripHeroCopy: { position: "absolute", left: 18, right: 18, bottom: 16, gap: 4 },
  tripTitle: { color: mucTrenAnh, fontFamily: displayFace.extraBold, fontSize: 28, lineHeight: 33, letterSpacing: -0.7 },
  tripMeta: { color: mucTrenAnh, fontSize: 13, lineHeight: 18, fontWeight: "600" },
  tomTat: { flexDirection: "row", flexWrap: "wrap", alignItems: "center", gap: 20 },
  oTomTat: { gap: 4 },
  viTri: { flexDirection: "row", alignItems: "center", gap: 12, paddingVertical: 12, borderTopWidth: StyleSheet.hairlineWidth, borderBottomWidth: StyleSheet.hairlineWidth },
  checkRow: { minHeight: 56, flexDirection: "row", alignItems: "center", gap: 12, paddingVertical: 6, borderBottomWidth: StyleSheet.hairlineWidth },
  demoNote: { textAlign: "center", paddingHorizontal: 20 },
});
