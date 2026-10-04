/** Server-owned journey paths. The client never awards badges or credits. */
import { newAttempt, translatedAsActor } from "../../api";

const LOI = {
  achievement_unavailable: "Chưa mở được sổ hành trình. Thử lại sau nhé.",
  ending_not_available: "Ngã rẽ này chưa hiện trên bản đồ của bạn.",
  ending_not_earned: "Bạn còn thiếu vài dấu mốc trước khi nhận kết này.",
  run_replaced: "Bạn đã chọn một ngã rẽ khác. Mở lại sổ hành trình nhé.",
  badge_not_earned: "Chỉ có thể trưng bày huy hiệu bạn đã đạt.",
  consent_required: "Bạn cần đồng ý trước khi hỏi Nếp.",
} as const;

export type RouteID = "dau_chan" | "ky_niem" | "dong_hanh" | "nga_re";

export const BADGE_TITLES: Record<string, string> = {
  first_checkin: "Dấu chân đầu tiên", first_photo: "Khung ảnh đầu", first_story: "Lời kể đầu tiên",
  first_together: "Có người đi cùng", many_turns: "Một ngày nhiều ngã", open_map: "Bản đồ mở",
  photos_remain: "Những tấm ảnh còn đây", storyteller: "Chuyện mình kể", again_together: "Hẹn rồi lại hẹn",
  full_house: "Đủ mặt hôm nay", map_becomes_page: "Bản đồ thành trang", shared_memory: "Kỷ niệm chung",
  whole_journey: "Hành trình của mình",
};

export type Requirement = { label: string; have: number; need: number };
export type RouteChoice = {
  id: string;
  route_id: RouteID;
  title: string;
  description: string;
  reward: string;
  eligible: boolean;
  earned: boolean;
  requirements: Requirement[];
};
export type JourneyRoute = { id: RouteID; title: string; blurb: string };
export type JourneyChapter = { id: string; title: string; line: string; route_id: RouteID; target_ending_id: string };
/**
 * `seen`: the person's own book has presented the badge (QA UI-160). Only the
 * owner's read carries it; another person's view of displayed badges does not.
 */
export type EarnedBadge = { id: string; earned_at: string; displayed: boolean; seen?: boolean };
export type JourneyRun = { id: string; route_id: RouteID; ending_id: string; selected_at: string; finished_at?: string };
export type JourneySnapshot = {
  routes: JourneyRoute[];
  active_run: JourneyRun | null;
  earned_badges: EarnedBadge[];
  candidates: RouteChoice[];
  chapters: JourneyChapter[];
  mp4_credits: { granted: number; used: number; available: number };
  suggestion_preview: {
    checkins: number;
    distinct_destinations: number;
    photo_days: number;
    story_days: number;
    shared_outings: number;
  };
};

export async function docHanhTrinh(actorId: string): Promise<JourneySnapshot> {
  return translatedAsActor<JourneySnapshot>(LOI, "/me/achievement-routes", { method: "GET", actorId });
}

export async function chonKet(actorId: string, routeId: RouteID, endingId: string): Promise<JourneyRun> {
  return translatedAsActor<JourneyRun>(LOI, "/me/achievement-runs", {
    method: "POST", actorId, attempt: newAttempt(), body: { route_id: routeId, ending_id: endingId },
  });
}

export async function nhanKet(actorId: string, runId: string): Promise<{ badge_id: string; awarded: boolean; reward: string }> {
  return translatedAsActor(LOI, `/me/achievement-runs/${encodeURIComponent(runId)}/finish`, {
    method: "POST", actorId, attempt: newAttempt(), body: {},
  });
}

export async function trungBayHuyHieu(actorId: string, badgeIds: string[]): Promise<{ badge_ids: string[] }> {
  return translatedAsActor(LOI, "/me/achievement-display", {
    method: "PATCH", actorId, attempt: newAttempt(), body: { badge_ids: badgeIds },
  });
}

/** The consent preview must be shown in UI. False never reaches the network. */
export async function goiYNep(actorId: string, consent: boolean): Promise<{ candidate_ids: string[]; source: "ai" | "go"; line: string }> {
  if (!consent) throw new Error("Bạn chưa đồng ý chia sẻ tóm tắt tiến độ với Nếp.");
  return translatedAsActor(LOI, "/me/achievement-suggestions", {
    method: "POST", actorId, attempt: newAttempt(), body: { consent: true },
  });
}

/** Other profiles receive only the owner's selected earned badges. */
/** The book presented these badges as just opened; no phone presents them again (QA UI-160). */
export async function danhDauDaThay(actorId: string, badgeIds: string[]): Promise<{ badge_ids: string[] }> {
  return translatedAsActor(LOI, "/me/achievement-seen", { method: "POST", actorId, body: { badge_ids: badgeIds } });
}

export async function docHuyHieuTrungBay(actorId: string, personId: string): Promise<{ person_id: string; badges: EarnedBadge[] }> {
  return translatedAsActor(LOI, `/people/${encodeURIComponent(personId)}/achievements`, { method: "GET", actorId });
}
