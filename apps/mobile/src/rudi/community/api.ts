/** Community transport. Visibility is always decided by the authenticated server. */
import { BASE_URL, newAttempt, translatedAsActor } from "../../api";
import { headerNguoiGoi } from "../../danh-tinh";
import type { DiaryDocument, DiaryKind } from "../diary/api";
export type Audience = "friends" | "public" | "only_me" | "group";
export type FeedMode = "for_you" | "following" | "trending" | "saved" | "mine";
export type Media = {
    id: string;
    url: string;
    type: string;
    width: number;
    height: number;
    duration_ms: number;
};
export type Post = {
    diary?: DiaryDocument;
    diary_kind?: DiaryKind;
    id: string;
    author_id: string;
    author: string;
    body: string;
    audience: Audience;
    context_id: string | null;
    created_at: string;
    revision: number;
    status: string;
    reason: string;
    topics: string[];
    mentions: string[];
    media: Media[];
    likes: number;
    comments: number;
    liked: boolean;
    saved: boolean;
    following: boolean;
    can_comment: boolean;
    why: string;
};
export type PostInput = {
    logical_id: string;
    revision?: number;
    body: string;
    audience: Audience;
    context_id?: string | null;
    topics: string[];
    mentions: string[];
    media_ids: string[];
};
export type Comment = {
    id: string;
    author_id: string;
    author: string;
    body: string;
    parent_id: string | null;
    media_id: string | null;
    mentions: string[];
    created_at: string;
    status: string;
    can_delete: boolean;
};
export type Page = {
    posts: Post[];
    next_cursor: string | null;
    personalized: boolean;
};
export type Preferences = {
    asked: boolean;
    personalized: boolean;
};
export const COMMUNITY_ERRORS: Record<string, string> = {
    community_unavailable: "Cộng đồng chưa kết nối được. Bạn thử lại sau một chút nhé.",
    post_not_found: "Bài đã được cất riêng hoặc không còn ở đây.",
    rate_limited: "Bạn thao tác khá nhanh. Đợi một lát rồi thử lại nhé.",
    invalid_body: "Viết một chút về khoảnh khắc của bạn trước khi gửi nhé.",
    revision_conflict: "Bài vừa thay đổi ở nơi khác. Mở lại bài trước khi sửa nhé.",
    source_changed: "Nội dung đã thay đổi. Xem lại phần chia sẻ với Nếp nhé.",
    comments_closed: "Tác giả đã giới hạn bình luận cho bài này.",
    media_not_ready: "Ảnh hoặc video vẫn đang được chuẩn bị. Đợi một chút nhé.",
    media_too_large: "Ảnh tối đa 12 MB, video tối đa 64 MB. Chọn tệp nhỏ hơn nhé.",
    unsupported_media: "Chọn ảnh JPEG, PNG hoặc video MP4, MOV nhé.",
    invalid_video: "Video này chưa đọc được. Thử một tệp MP4 khác nhé.",
    video_too_long: "Chọn video dài tối đa 3 phút nhé.",
    invalid_image: "Ảnh này chưa đọc được. Thử ảnh JPG hoặc PNG khác nhé.",
    feed_expired: "Bảng tin đã có phiên mới. Kéo xuống để làm mới nhé.",
    mention_requires_friend: "Chỉ tag những người đang kết bạn với bạn nhé.",
    nep_unavailable: "Nếp chưa trả lời được. Nội dung của bạn vẫn ở đây.",
    edit_source_diary: "Sửa cuốn sổ gốc rồi chia sẻ lại phiên bản mới nhé.",
    share_source_diary_again: "Mở cuốn sổ gốc để chọn phiên bản muốn chia sẻ nhé.",
    post_unavailable: "Bài hoặc quyền bình luận vừa thay đổi. Mở lại để kiểm tra nhé.",
    cannot_review_own_post: "Nội dung cần được một người kiểm duyệt khác xem xét.",
    moderator_required: "Tài khoản này không có quyền kiểm duyệt.",
};
export const readFeed = (person: string, mode: FeedMode, after?: string | null, topic?: string, author?: string) => translatedAsActor<Page>(COMMUNITY_ERRORS, `/v2/community/feed?mode=${mode}${after ? `&after=${encodeURIComponent(after)}` : ""}${topic ? `&topic=${encodeURIComponent(topic)}` : ""}${author ? `&author=${encodeURIComponent(author)}` : ""}`, {
    actorId: person,
    method: "GET"
});
export const getPost = (person: string, id: string) => translatedAsActor<Post>(COMMUNITY_ERRORS, `/v2/community/posts/${id}`, {
    actorId: person,
    method: "GET"
});
export const createPost = (person: string, input: PostInput) => translatedAsActor<Post>(COMMUNITY_ERRORS, "/v2/community/posts", {
    actorId: person,
    method: "POST",
    body: input,
    attempt: newAttempt()
});
export const likePost = (person: string, id: string, liked: boolean) => translatedAsActor<Post>(COMMUNITY_ERRORS, `/v2/community/posts/${id}/like`, {
    actorId: person,
    method: liked ? "PUT" : "DELETE",
    attempt: newAttempt()
});
export const feedback = (person: string, id: string, kind: "saved" | "hidden", enabled: boolean) => translatedAsActor<void>(COMMUNITY_ERRORS, `/v2/community/posts/${id}/feedback`, {
    actorId: person,
    method: "PUT",
    body: { kind, enabled },
    attempt: newAttempt()
});
export const follow = (person: string, kind: "person" | "topic", target: string, enabled: boolean) => translatedAsActor<void>(COMMUNITY_ERRORS, "/v2/community/follows", {
    actorId: person,
    method: enabled ? "PUT" : "DELETE",
    body: { kind, target },
    attempt: newAttempt()
});
export const readComments = (person: string, id: string, after?: string | null) => translatedAsActor<{
    comments: Comment[];
    pending: Comment[];
    next_cursor: string | null;
}>(COMMUNITY_ERRORS, `/v2/community/posts/${id}/comments?after=${after ? encodeURIComponent(after) : ""}`, {
    actorId: person,
    method: "GET"
});
export const comment = (person: string, id: string, body: string, logical: string, parent: string | null, media: string | null, mentions: string[]) => translatedAsActor<{
    id: string;
    status: string;
}>(COMMUNITY_ERRORS, `/v2/community/posts/${id}/comments`, {
    actorId: person,
    method: "POST",
    body: { body, logical_id: logical, parent_id: parent, media_id: media, mentions },
    attempt: newAttempt()
});
export function imageSource(person: string, path: string) {
    return { uri: BASE_URL + path, headers: headerNguoiGoi(person) };
}
export async function uploadMedia(person: string, uri: string, mime: string) {
    const source = await fetch(uri);
    const blob = await source.blob();
    const headers = headerNguoiGoi(person);
    headers["Content-Type"] = mime;
    // Native file responses can produce an untyped Blob. Set the body's MIME
    // as well as the header; Android's networking stack derives it from Blob.
    // slice creates a view rather than copying a potentially large video.
    let body: Blob | undefined;
    try {
        body = blob.slice(0, blob.size, mime);
        const response = await fetch(
            BASE_URL + "/v2/community/media", { method: "POST", headers, body });
        if (!response.ok) {
            const e = await response.json().catch(() => ({}));
            throw new Error(COMMUNITY_ERRORS[e.code] ?? "Chưa tải được tệp. Thử lại nhé.");
        }
        return await response.json() as Media & {
            state: "ready" | "processing" | "failed";
        };
    } finally {
        // Native slices retain their own reference to the shared Blob storage.
        try {
            (body as (Blob & { close?: () => void }) | undefined)?.close?.();
        } finally {
            (blob as Blob & { close?: () => void }).close?.();
        }
    }
}
export function mergePosts(previous: readonly Post[], incoming: readonly Post[]): Post[] {
    const found = new Set(previous.map((p) => p.id));
    return [...previous, ...incoming.filter((p) => !found.has(p.id) && Boolean(found.add(p.id)))];
}
export function relativeTime(iso: string, now = Date.now()): string {
    const minutes = Math.max(0, Math.floor((now - Date.parse(iso)) / 60000));
    if (!Number.isFinite(minutes) || minutes < 1)
        return "Vừa xong";
    if (minutes < 60)
        return `${minutes} phút`;
    if (minutes < 1440)
        return `${Math.floor(minutes / 60)} giờ`;
    return new Date(iso).toLocaleDateString("vi-VN", { day: "numeric", month: "short" });
}
