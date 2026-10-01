import { Ionicons } from "@expo/vector-icons";
import { useEffect, useState } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";
import Svg, { Path } from "react-native-svg";

import type { Phien } from "../../../phien";
import {
  BADGE_TITLES, chonKet, docHanhTrinh, goiYNep, nhanKet, trungBayHuyHieu,
  type JourneySnapshot, type RouteChoice, type RouteID,
} from "../../ky-niem/achievement-routes";
import { choicesForRoute, toggleDisplayedBadge } from "../../ky-niem/journey-view";
import { huyHieuMoi } from "../../ky-niem/ky-niem";
import { docGiaoDienAsync, ghiGiaoDienAsync } from "../../kho";
import { NepDien } from "../../ui/NepDien";
import { displayFace, typography, useRudiTheme } from "../../theme";
import { RudiButton, RudiScreen, SectionHeader, TopBar } from "../../ui";
import { BadgeArt } from "../../ui/BadgeArt";
import { ErrorState } from "../../ui/ErrorState";
import { SkeletonGroup, SkeletonRow } from "../../ui/Skeleton";
import { ToGiay } from "../../ui/ToGiay";
import { Washi } from "../../ui/Washi";
import { TABLIST, tabState, toggleState } from "../../../ui/a11y";

type Page = { phase: "loading" } | { phase: "ready"; book: JourneySnapshot } | { phase: "error"; message: string };

function messageOf(error: unknown): string {
  if (error instanceof Error && error.message) return error.message;
  return "Chưa đọc được sổ hành trình. Thử lại nhé.";
}

function progressText(choice: RouteChoice): string {
  return choice.requirements.map((r) => `${r.label}: ${Math.min(r.have, r.need)}/${r.need}`).join(" · ");
}

/** Where this phone remembers which journey badges it has already shown earned. */
const KHOA_DA_THAY = "rudi.huy-hieu-hanh-trinh-da-thay";

export function AchievementsLiveScreen({ phien }: { phien: Phien }) {
  const { colors, radius } = useRudiTheme();
  const [page, setPage] = useState<Page>({ phase: "loading" });
  const [routeId, setRouteId] = useState<RouteID>("dau_chan");
  const [busy, setBusy] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [previewOpen, setPreviewOpen] = useState(false);
  const [suggested, setSuggested] = useState<string[]>([]);
  const [suggestionSource, setSuggestionSource] = useState<"ai" | "go" | null>(null);
  const [suggestionLine, setSuggestionLine] = useState<string | null>(null);
  const [mapWidth, setMapWidth] = useState(320);
  // M8: a badge earned since the last look gets its moment, once.
  const [moi, setMoi] = useState<string | null>(null);
  useEffect(() => {
    if (page.phase !== "ready") return;
    const earned = page.book.earned_badges.map((badge) => badge.id);
    let live = true;
    void docGiaoDienAsync(`${KHOA_DA_THAY}:${phien.person_id}`).then((stored) => {
      if (!live) return;
      setMoi(huyHieuMoi(earned, stored));
      void ghiGiaoDienAsync(`${KHOA_DA_THAY}:${phien.person_id}`, JSON.stringify(earned));
    });
    return () => { live = false; };
  }, [page, phien.person_id]);

  const reload = async () => {
    const book = await docHanhTrinh(phien.person_id);
    setPage({ phase: "ready", book });
    if (book.active_run) setRouteId(book.active_run.route_id);
  };
  useEffect(() => {
    let live = true;
    void docHanhTrinh(phien.person_id).then((book) => {
      if (!live) return;
      setPage({ phase: "ready", book });
      if (book.active_run) setRouteId(book.active_run.route_id);
    }).catch((error: unknown) => {
      if (live) setPage({ phase: "error", message: messageOf(error) });
    });
    return () => { live = false; };
  }, [phien.person_id]);

  const perform = async (key: string, work: () => Promise<unknown>) => {
    if (busy !== null) return;
    setBusy(key);
    setActionError(null);
    try { await work(); await reload(); }
    catch (error) { setActionError(messageOf(error)); }
    finally { setBusy(null); }
  };

  if (page.phase === "loading") {
    return <RudiScreen testID="achievements-screen"><TopBar title="Hành trình" /><SkeletonGroup><SkeletonRow /><SkeletonRow /><SkeletonRow /></SkeletonGroup></RudiScreen>;
  }
  if (page.phase === "error") {
    return <RudiScreen testID="achievements-screen"><TopBar title="Hành trình" /><ErrorState title="Chưa mở được sổ hành trình" body={page.message} onRetry={() => { setPage({ phase: "loading" }); void reload().catch((error: unknown) => setPage({ phase: "error", message: messageOf(error) })); }} /></RudiScreen>;
  }

  const book = page.book;
  const route = book.routes.find((item) => item.id === routeId) ?? book.routes[0];
  const choices = choicesForRoute(book.candidates, routeId, book.active_run?.ending_id ?? null);
  const earnedIds = book.earned_badges.map((badge) => badge.id);
  const displayedIds = book.earned_badges.filter((badge) => badge.displayed).map((badge) => badge.id);
  const openingBadges = book.earned_badges.filter((badge) => badge.id.startsWith("first_"));
  const endingBadges = book.earned_badges.filter((badge) => !badge.id.startsWith("first_"));
  const nextChapter = book.chapters[0];
  const chapterTarget = nextChapter ? book.candidates.find((choice) => choice.id === nextChapter.target_ending_id) : undefined;

  return (
    <RudiScreen contentStyle={styles.page} onRefresh={async () => { try { await reload(); } catch (error) { setActionError(messageOf(error)); } }} testID="achievements-screen">
      <TopBar title="Hành trình" />
      <View style={[styles.cover, { backgroundColor: colors.cover, borderRadius: radius.base }]}>
        <View style={[styles.coverRule, { backgroundColor: colors.coverLineStrong }]} />
        <Text style={[styles.coverTitle, { color: colors.coverInk }]}>Cuốn sổ có nhiều ngã rẽ</Text>
        <Text style={[typography.body, { color: colors.coverInkSoft }]}>Bạn chọn cách kể chuyến đi của mình. Mỗi huy hiệu mở thêm một trang, và những trang đã mở sẽ ở lại.</Text>
        <View style={styles.coverBottom}>
          <Ionicons color={colors.coverInkSoft} name="book-outline" size={19} />
          <Text style={[typography.note, { color: colors.coverInkSoft }]}>{endingBadges.length} kết đã mở · {book.mp4_credits.available} lượt dựng MP4 còn dùng được</Text>
        </View>
      </View>

      {moi ? (
        // The badge earned since the last look, as its stamp; Nếp lifts it (M8).
        <View style={[styles.fresh, { backgroundColor: colors.accentSoft, borderRadius: radius.base }]}>
          <BadgeArt badgeId={moi} label={BADGE_TITLES[moi] ?? "Huy hiệu hành trình"} size={72} state="unlocked" />
          <View style={styles.flex}>
            <Text style={[typography.caption, { color: colors.accent }]}>MỚI MỞ</Text>
            <Text style={[typography.h2, { color: colors.ink }]}>{BADGE_TITLES[moi] ?? "Huy hiệu hành trình"}</Text>
          </View>
          <NepDien khoanhKhac="M8" suKien={`huy-hieu:${moi}`} />
        </View>
      ) : null}

      <View style={styles.sectionTop}>
        <Text style={[typography.h2, { color: colors.ink }]}>Chọn lối đi</Text>
        <Text style={[typography.note, { color: colors.inkSoft }]}>Có thể đổi hướng bất cứ lúc nào. Dấu mốc cũ vẫn được giữ.</Text>
      </View>
      <View onLayout={(event) => setMapWidth(Math.round(event.nativeEvent.layout.width))} style={styles.routeMap} {...TABLIST} accessibilityLabel="Bản đồ bốn tuyến hành trình. Ba tuyến đầu gặp nhau ở Ngã rẽ.">
        <Svg pointerEvents="none" width={mapWidth} height={184} style={StyleSheet.absoluteFill}>
          {[mapWidth / 6, mapWidth / 2, mapWidth * 5 / 6].map((x, index) => <Path key={index} d={`M ${x} 62 Q ${x} 114 ${mapWidth / 2} 145`} fill="none" stroke={colors.lineStrong} strokeWidth={1.5} strokeDasharray={index === 1 ? undefined : "4 5"} />)}
        </Svg>
        <View style={styles.mapTop}>{book.routes.slice(0, 3).map((item) => {
          const selected = item.id === routeId;
          const count = book.candidates.filter((choice) => choice.route_id === item.id && choice.earned).length;
          return <Pressable key={item.id} {...tabState(selected)} accessibilityLabel={`${item.title}, ${count} kết đã đạt`} onPress={() => setRouteId(item.id)} style={[styles.mapNode, { borderColor: selected ? colors.accent : colors.lineStrong, backgroundColor: selected ? colors.accentSoft : colors.paper, borderRadius: radius.small }]}>
            <Text style={[typography.label, { color: selected ? colors.accent : colors.ink, textAlign: "center" }]}>{item.title}</Text>
            <Text style={[typography.note, { color: colors.inkSoft }]}>{count} kết</Text>
          </Pressable>;
        })}</View>
        {book.routes[3] ? <Pressable {...tabState(routeId === "nga_re")} onPress={() => setRouteId("nga_re")} style={[styles.mapNode, styles.mapConfluence, { borderColor: routeId === "nga_re" ? colors.accent : colors.lineStrong, backgroundColor: routeId === "nga_re" ? colors.accentSoft : colors.paper, borderRadius: radius.small }]}>
          <Text style={[typography.label, { color: routeId === "nga_re" ? colors.accent : colors.ink }]}>Ngã rẽ</Text>
          <Text style={[typography.note, { color: colors.inkSoft }]}>Các tuyến gặp nhau</Text>
        </Pressable> : null}
      </View>
      {nextChapter && chapterTarget ? <ToGiay dan style={styles.chapter}>
        <View style={styles.chapterHeading}>
          <Text style={[typography.caption, { color: colors.inkSoft }]}>TRANG VỪA MỞ TỪ LỐI BẠN CHỌN</Text>
          <Ionicons color={colors.accent} name="git-branch-outline" size={20} />
        </View>
        <Text style={[typography.h2, { color: colors.ink }]}>{nextChapter.title}</Text>
        <Text style={[typography.body, { color: colors.inkSoft }]}>{nextChapter.line}</Text>
        <View style={[styles.chapterTarget, { borderTopColor: colors.lineStrong }]}>
          <BadgeArt badgeId={chapterTarget.id} label={chapterTarget.title} state={chapterTarget.earned ? "unlocked" : chapterTarget.eligible ? "progress" : "locked"} size={58} />
          <View style={styles.flex}>
            <Text style={[typography.label, { color: colors.ink }]}>{chapterTarget.title}</Text>
            <Text style={[typography.note, { color: colors.inkSoft }]}>{chapterTarget.earned ? "Kết đã ghi vào sổ" : chapterTarget.eligible ? "Đủ dấu mốc để nhận kết" : progressText(chapterTarget)}</Text>
          </View>
          <Pressable accessibilityRole="button" accessibilityLabel={`Mở hướng ${chapterTarget.title}`} onPress={() => setRouteId(chapterTarget.route_id)} style={[styles.chapterArrow, { backgroundColor: colors.accentSoft, borderRadius: radius.small }]}>
            <Ionicons color={colors.accent} name="arrow-forward" size={20} />
          </Pressable>
        </View>
      </ToGiay> : null}
      <View style={styles.routeIntro}>
        <View style={[styles.routeMark, { backgroundColor: colors.accent }]} />
        <View style={styles.flex}>
          <Text style={[typography.h2, { color: colors.ink }]}>{route.title}</Text>
          <Text style={[typography.body, { color: colors.inkSoft }]}>{route.blurb}</Text>
        </View>
      </View>

      <View style={styles.branchList}>
        {choices.length === 0 ? <ToGiay><Text style={[typography.title, { color: colors.ink }]}>Ngã rẽ đang chờ</Text><Text style={[typography.body, { color: colors.inkSoft }]}>Đạt một kết Dấu chân, Kỷ niệm hoặc Đồng hành để những câu chuyện bắt đầu gặp nhau ở đây.</Text></ToGiay> : null}
        {choices.map((choice, index) => {
          const active = book.active_run?.ending_id === choice.id;
          const actionLabel = choice.earned ? "Đã ghi vào sổ" : active ? choice.eligible ? "Nhận kết này" : "Đang theo hướng này" : "Chọn hướng này";
          return <ToGiay key={choice.id} dan={active || (index === 0 && !book.active_run)} style={styles.branch}>
            <View style={styles.branchHead}>
              <BadgeArt badgeId={choice.id} label={choice.title} size={72} state={choice.earned ? "unlocked" : active ? "progress" : "locked"} />
              <View style={styles.flex}>
                <Text style={[typography.h2, { color: colors.ink }]}>{choice.title}</Text>
                <Text style={[typography.note, { color: colors.inkSoft }]}>{choice.description}</Text>
              </View>
            </View>
            <View style={[styles.routeLine, { borderColor: colors.lineStrong }]}>
              <Text style={[typography.note, { color: colors.inkSoft }]}>{progressText(choice) || "Các tuyến chính đã gặp nhau."}</Text>
            </View>
            <View style={styles.rewardRow}><Ionicons color={colors.ai} name="sparkles-outline" size={18} /><Text style={[typography.label, { color: colors.ink }]}>Mẫu sáng tạo sắp dùng được: {choice.reward}</Text></View>
            {suggested.includes(choice.id) ? <Text style={[typography.caption, { color: colors.ai }]}>{suggestionSource === "ai" ? "Nếp gợi ý hướng này" : "Sổ gợi ý hướng này"}</Text> : null}
            <RudiButton label={actionLabel} compact full={false} variant="solid" disabled={choice.earned || (active && !choice.eligible) || busy !== null} lyDo={choice.earned ? "Kết này đã ghi vào sổ." : active && !choice.eligible ? `Còn thiếu dấu mốc: ${progressText(choice)}.` : undefined} loading={busy === choice.id} onPress={() => void perform(choice.id, active && book.active_run ? () => nhanKet(phien.person_id, book.active_run!.id) : () => chonKet(phien.person_id, choice.route_id, choice.id))} />
          </ToGiay>;
        })}
      </View>

      <View style={styles.suggestion}>
        <SectionHeader title="Nếp nhìn đường đi" />
        <Text style={[typography.body, { color: colors.inkSoft }]}>Nếp có thể gợi ý 2–3 ngã rẽ theo lối bạn chọn và những dấu mốc đã có.</Text>
        <Pressable accessibilityRole="button" aria-expanded={previewOpen} onPress={() => setPreviewOpen((open) => !open)} style={styles.previewToggle}>
          <Text style={[typography.label, { color: colors.ai }]}>{previewOpen ? "Đóng bản xem trước" : "Xem Nếp sẽ nhận gì"}</Text>
          <Ionicons color={colors.ai} name={previewOpen ? "chevron-up" : "chevron-down"} size={17} />
        </Pressable>
        {previewOpen ? <ToGiay>
          <Text style={[typography.label, { color: colors.ink }]}>Chỉ gửi số đếm và mã ngã rẽ</Text>
          <Text style={[typography.body, { color: colors.inkSoft }]}>{book.suggestion_preview.checkins} lần check-in · {book.suggestion_preview.distinct_destinations} điểm đến · {book.suggestion_preview.photo_days} ngày có ảnh · {book.suggestion_preview.story_days} ngày có bài kể · {book.suggestion_preview.shared_outings} chuyến có bạn cùng đi.</Text>
          <Text style={[typography.note, { color: colors.inkSoft }]}>Còn gửi mã nhánh hiện tại, các ngã rẽ có thể chọn và tối đa tám lựa chọn gần đây. Không gửi ảnh, nội dung bài, danh tính bạn đồng hành, chat hoặc vị trí cụ thể. Mỗi lần hỏi cần bạn đồng ý lại.</Text>
          <RudiButton label="Đồng ý hỏi Nếp lần này" tone="ai" loading={busy === "suggest"} disabled={busy !== null} onPress={() => void perform("suggest", async () => {
            const result = await goiYNep(phien.person_id, true);
            setSuggested(result.candidate_ids);
            setSuggestionSource(result.source);
            setSuggestionLine(result.line);
            setPreviewOpen(false);
          })} />
        </ToGiay> : null}
        {suggestionLine ? <Text style={[typography.body, { color: colors.ink }]}>{suggestionLine}</Text> : null}
        {suggested.length > 0 ? <View style={styles.suggestedList}>{suggested.map((id) => {
          const choice = book.candidates.find((candidate) => candidate.id === id);
          if (!choice) return null;
          return <Pressable key={id} accessibilityRole="button" accessibilityLabel={`Xem ngã rẽ ${choice.title}`} onPress={() => setRouteId(choice.route_id)} style={[styles.suggestedLink, { borderColor: colors.lineStrong, borderRadius: radius.small }]}>
            <Text style={[typography.label, { color: colors.ink }]}>{choice.title}</Text>
            <Ionicons color={colors.ai} name="arrow-forward" size={18} />
          </Pressable>;
        })}</View> : null}
        {suggestionSource === "go" ? <Text style={[typography.note, { color: colors.inkSoft }]}>Nếp đang vắng. Sổ đã gợi ý từ tiến độ thật của bạn.</Text> : null}
      </View>

      <SectionHeader title="Dấu ấn đã giữ" />
      <Text style={[typography.note, { color: colors.inkSoft }]}>Chọn tối đa ba huy hiệu để hiện trên hồ sơ. Tiến độ và số lượt MP4 chỉ mình bạn thấy.</Text>
      {earnedIds.length === 0 ? <Text style={[typography.body, { color: colors.inkSoft }]}>Bắt đầu bằng một lần check-in tự khai, một ảnh kỷ niệm hoặc một lời kể.</Text> : null}
      <View style={styles.badgeList}>
        {[...openingBadges, ...endingBadges].map((badge) => {
          const displayed = displayedIds.includes(badge.id);
          const title = BADGE_TITLES[badge.id] ?? "Huy hiệu hành trình";
          return <Pressable key={badge.id} {...toggleState("checkbox", displayed)} aria-disabled={busy !== null && busy !== badge.id} accessibilityLabel={`${title}, ${displayed ? "đang trưng bày" : "chưa trưng bày"}`} onPress={() => void perform(badge.id, () => trungBayHuyHieu(phien.person_id, toggleDisplayedBadge(displayedIds, badge.id, earnedIds)))} style={[styles.badgeRow, { borderBottomColor: colors.line }]}>
            <BadgeArt badgeId={badge.id} label={title} state="unlocked" size={54} />
            <View style={styles.flex}><Text style={[typography.label, { color: colors.ink }]}>{title}</Text><Text style={[typography.note, { color: colors.inkSoft }]}>{displayed ? "Trên hồ sơ" : "Chạm để trưng bày"}</Text></View>
            <Ionicons color={displayed ? colors.accent : colors.inkFaint} name={displayed ? "checkmark-circle" : "ellipse-outline"} size={23} />
          </Pressable>;
        })}
      </View>
      {actionError ? <Text accessibilityRole="alert" style={[typography.body, { color: colors.warn }]}>{actionError}</Text> : null}
      <Washi tone="ai" tilt={-1}><Text style={[typography.stamp, { color: colors.aiInk }]}>Mỗi chuyến đi là một câu chuyện khác</Text></Washi>
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  page: { gap: 20 }, flex: { flex: 1 },
  cover: { overflow: "hidden", padding: 24, gap: 12, minHeight: 192 },
  coverRule: { position: "absolute", left: 13, top: 0, bottom: 0, width: 1 },
  coverTitle: { fontFamily: displayFace.extraBold, fontSize: 30, lineHeight: 35, letterSpacing: -0.7 },
  coverBottom: { flexDirection: "row", alignItems: "center", gap: 9, marginTop: 9 },
  sectionTop: { gap: 3 },
  fresh: { flexDirection: "row", alignItems: "center", gap: 14, padding: 16 },
  routeMap: { height: 184, alignSelf: "center", width: "100%", maxWidth: 560, justifyContent: "space-between" },
  mapTop: { flexDirection: "row", justifyContent: "space-between" },
  mapNode: { minHeight: 64, width: "30%", borderWidth: 1, alignItems: "center", justifyContent: "center", paddingHorizontal: 6, paddingVertical: 6 },
  mapConfluence: { alignSelf: "center", width: "48%" },
  routeIntro: { flexDirection: "row", alignItems: "flex-start", gap: 13 },
  chapter: { gap: 9 }, chapterHeading: { flexDirection: "row", alignItems: "center", justifyContent: "space-between" },
  chapterTarget: { flexDirection: "row", alignItems: "center", gap: 10, borderTopWidth: StyleSheet.hairlineWidth, paddingTop: 10 },
  chapterArrow: { width: 42, height: 42, alignItems: "center", justifyContent: "center" },
  routeMark: { width: 6, minHeight: 49, borderRadius: 3 },
  branchList: { gap: 14 }, branch: { gap: 14 },
  branchHead: { flexDirection: "row", alignItems: "center", gap: 12 },
  routeLine: { borderTopWidth: StyleSheet.hairlineWidth, paddingTop: 10 },
  rewardRow: { flexDirection: "row", alignItems: "center", gap: 7 },
  suggestion: { gap: 10 }, previewToggle: { alignSelf: "flex-start", flexDirection: "row", alignItems: "center", minHeight: 48, gap: 7 },
  suggestedList: { gap: 8 }, suggestedLink: { minHeight: 48, borderWidth: 1, paddingHorizontal: 14, flexDirection: "row", justifyContent: "space-between", alignItems: "center" },
  badgeList: { gap: 0 }, badgeRow: { flexDirection: "row", alignItems: "center", gap: 12, minHeight: 72, borderBottomWidth: StyleSheet.hairlineWidth, paddingVertical: 8 },
});
