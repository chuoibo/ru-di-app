/**
 * Nếp xin một tấm ảnh, và đọc lại kết quả của chính mình.
 *
 * Đo thật trên đường ống: **99 giây** một tấm ảnh, trong đó 64s là lượt planner
 * của agy và 12s là gen. Không lời gọi nào ở đây chờ tới lúc có ảnh. `xinAnh`
 * trả về một `job_id` gần như tức thì (đo được 26ms ở tầng hàng đợi), rồi màn
 * hỏi lại bằng `docTrangThai`.
 *
 * Quyền sở hữu nằm trong chính `job_id`: máy chủ đúc nó với một tiền tố HMAC
 * của người dùng, nên một id của người khác trả về 404 chứ không phải 403 —
 * người ta không được biết job của người khác có tồn tại hay không.
 *
 * Bảng câu chữ ở đây theo đúng luật của vỏ: hết hạn mức và mô tả không dùng
 * được là những thứ PHẢI nói ra, không phải im lặng. Chỉ `cooldown` và
 * `rate_limited` mới là im lặng cố ý, và media không có hai trạng thái đó.
 */
import { type Attempt, translatedAsActor } from "../../api";

export type LoaiMedia = "anh" | "video";
export type TrangThaiMedia = "dang-cho" | "dang-chay" | "xong" | "hong";

export interface JobMedia {
  job_id: string;
  loai: LoaiMedia;
  trang_thai: TrangThaiMedia;
}

export interface TinhHinhMedia {
  job_id: string;
  trang_thai: TrangThaiMedia;
  so_byte?: number;
  loi?: string;
}

/** Mã máy chủ thành câu người đọc. Thiếu một mã là câu mặc định của `api.ts`. */
export const MEDIA_REFUSALS: Record<string, string> = {
  nep_media_chua_cau_hinh: "Rủ Đi chưa bật phần vẽ ảnh của Nếp.",
  nep_media_thieu_khoa: "Rủ Đi chưa bật phần vẽ ảnh của Nếp.",
  nep_media_khong_goi_duoc: "Nếp chưa nối được tới chỗ vẽ ảnh. Thử lại sau nhé.",
  nep_media_proxy_tu_choi: "Chỗ vẽ ảnh đang trục trặc. Thử lại sau nhé.",
  khong_thay_job: "Không tìm thấy bức này.",
  chua_co_media: "Bức này chưa vẽ xong.",
};

export async function xinAnh(
  actorId: string,
  attempt: Attempt,
  moTa: string,
  tuyChon: { tenAnh?: string; tyLe?: string } = {},
): Promise<JobMedia> {
  return translatedAsActor<JobMedia>(MEDIA_REFUSALS, "/me/nep/media", {
    method: "POST",
    body: { loai: "anh", mo_ta: moTa, ten_anh: tuyChon.tenAnh, ty_le: tuyChon.tyLe },
    actorId,
    attempt,
  });
}

export async function xinVideo(
  actorId: string,
  attempt: Attempt,
  anhJobIds: readonly string[],
  giayMoiAnh?: number,
): Promise<JobMedia> {
  return translatedAsActor<JobMedia>(MEDIA_REFUSALS, "/me/nep/media", {
    method: "POST",
    body: { loai: "video", anh_job_ids: anhJobIds, giay_moi_anh: giayMoiAnh },
    actorId,
    attempt,
  });
}

export async function docTrangThai(actorId: string, jobId: string): Promise<TinhHinhMedia> {
  return translatedAsActor<TinhHinhMedia>(MEDIA_REFUSALS, `/me/nep/media/${jobId}`, {
    method: "GET",
    actorId,
  });
}

/**
 * Đường tới bytes. Đi qua máy chủ của app chứ không trỏ thẳng vào chỗ vẽ: một
 * URL trỏ thẳng là một đường vòng qua chỗ kiểm quyền sở hữu.
 */
export function duongFile(baseUrl: string, jobId: string): string {
  return `${baseUrl.replace(/\/+$/, "")}/me/nep/media/${encodeURIComponent(jobId)}/file`;
}

/** Còn phải hỏi lại nữa không. `xong` và `hong` đều là đã xong việc hỏi. */
export function conPhaiHoi(trang_thai: TrangThaiMedia): boolean {
  return trang_thai === "dang-cho" || trang_thai === "dang-chay";
}

/**
 * Bao lâu thì hỏi lại. Tăng dần: 99 giây là bình thường, nên hỏi mỗi 2 giây
 * suốt hai phút là 60 lần gõ cửa cho một câu trả lời.
 */
export function nhipHoiMs(lanThu: number): number {
  const n = Number.isFinite(lanThu) ? Math.max(0, Math.floor(lanThu)) : 0;
  return Math.min(15_000, 2_000 + n * 1_500);
}
