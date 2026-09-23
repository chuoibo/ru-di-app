import { BASE_URL, newAttempt, translatedAsActor, type Attempt } from "../../api";

const REFUSALS: Record<string, string> = {
  video_credit_required: "Bạn cần mở thêm một kết hành trình để dựng phim mới.",
  nep_media_chua_cau_hinh: "Máy dựng phim chưa được bật trên máy chủ này.",
  nep_media_thieu_khoa: "Máy dựng phim chưa được bật trên máy chủ này.",
  nep_media_khong_goi_duoc: "Nếp chưa nối được máy dựng phim. Thử lại sau nhé.",
  nep_media_proxy_tu_choi: "Máy dựng phim đang bận. Lượt dựng của bạn được trả lại.",
  invalid_video: "Phim vừa dựng bị lỗi. Lượt dựng của bạn được trả lại.",
  khong_thay_job: "Không tìm thấy phim này.",
  chua_co_media: "Phim vẫn đang được dựng.",
};

export type VideoCreditBalance = { granted: number; used: number; available: number };
export type VideoJob = { job_id: string; status: "reserved" | "queued" | "running" | "ready" | "failed"; kind: "nep_video" };
export type SavedVideoJob = VideoJob & { created_at: string };

export function profileVideoFileURL(jobId: string): string {
  return `${BASE_URL.replace(/\/+$/, "")}/me/profile-videos/${encodeURIComponent(jobId)}/file`;
}

export function videoAttempt(): Attempt {
  return newAttempt();
}

export async function videoCredits(actorId: string): Promise<VideoCreditBalance> {
  return translatedAsActor(REFUSALS, "/me/profile-videos/credits", { method: "GET", actorId });
}

export async function savedProfileVideos(actorId: string): Promise<SavedVideoJob[]> {
  const result = await translatedAsActor<{ jobs: SavedVideoJob[] }>(REFUSALS, "/me/profile-videos", { method: "GET", actorId });
  return result.jobs;
}

export async function createProfileVideo(actorId: string, imageJobIds: string[], attempt: Attempt): Promise<VideoJob> {
  return translatedAsActor(REFUSALS, "/me/profile-videos", {
    method: "POST", actorId, attempt,
    body: { kind: "nep_video", image_job_ids: imageJobIds, seconds_per_image: 3, idempotency_key: attempt.key },
  });
}

export async function profileVideoStatus(actorId: string, jobId: string): Promise<VideoJob> {
  return translatedAsActor(REFUSALS, `/me/profile-videos/${encodeURIComponent(jobId)}`, { method: "GET", actorId });
}
