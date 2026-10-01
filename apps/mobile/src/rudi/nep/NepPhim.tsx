import { useEffect, useRef, useState } from "react";
import { Alert, AppState, Platform, Pressable, StyleSheet, Text, View } from "react-native";
import { useVideoPlayer, VideoView } from "expo-video";

import { ApiError } from "../../api";
import { actorHeaders } from "../../danh-tinh";
import { typography, useRudiTheme } from "../theme";
import {
  createProfileVideo, profileVideoFileURL, profileVideoStatus, videoAttempt, videoCredits,
  savedProfileVideos, type SavedVideoJob, type VideoCreditBalance, type VideoJob,
} from "./profile-video";
import { giuState } from "../../ui/a11y";

function VideoFrame({ uri, actorId }: { uri: string; actorId: string }) {
  const headers = actorHeaders(actorId);
  const player = useVideoPlayer({ uri, headers }, (ready) => {
    ready.loop = true;
  });
  return <VideoView accessibilityLabel="Phim Nếp vừa dựng" contentFit="contain" nativeControls player={player} style={styles.video} />;
}

export function NepPhim({ actorId, imageJobIds }: { actorId: string | null; imageJobIds: string[] }) {
  const { colors } = useRudiTheme();
  const [credits, setCredits] = useState<VideoCreditBalance | null>(null);
  const [job, setJob] = useState<VideoJob | null>(null);
  const [saved, setSaved] = useState<SavedVideoJob[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [webURI, setWebURI] = useState<string | null>(null);
  const [sending, setSending] = useState(false);
  const attempt = useRef<ReturnType<typeof videoAttempt> | null>(null);
  const sequence = useRef(0);

  useEffect(() => {
    if (!actorId) return;
    let live = true;
    void videoCredits(actorId).then((value) => { if (live) setCredits(value); }).catch(() => {
      if (live) setCredits(null);
    });
    return () => { live = false; };
  }, [actorId, job?.status]);

  useEffect(() => {
    if (!actorId) return;
    let live = true;
    void savedProfileVideos(actorId).then((jobs) => {
      if (!live) return;
      setSaved(jobs);
      setJob((current) => current ?? jobs[0] ?? null);
    }).catch(() => {});
    return () => { live = false; };
  }, [actorId]);

  useEffect(() => {
    if (!actorId || !job || (job.status !== "reserved" && job.status !== "queued" && job.status !== "running")) return;
    const current = ++sequence.current;
    let timer: ReturnType<typeof setTimeout>;
    const poll = async () => {
      if (current !== sequence.current) return;
      if (AppState.currentState === "active") {
        try {
          const next = await profileVideoStatus(actorId, job.job_id);
          if (current !== sequence.current) return;
          setJob(next);
          setSaved((before) => before.map((item) => item.job_id === next.job_id ? { ...item, status: next.status } : item));
          if (next.status === "failed") {
            setError("Lượt dựng chưa thành công. Nếp đã trả lại lượt để bạn thử tiếp.");
            return;
          }
          if (next.status === "ready") return;
        } catch (cause) {
          if (current !== sequence.current) return;
          setError(cause instanceof ApiError ? cause.message : "Nếp chưa xem được tiến độ phim.");
        }
      }
      timer = setTimeout(poll, 3500);
    };
    timer = setTimeout(poll, 3500);
    return () => { sequence.current += 1; clearTimeout(timer); };
  }, [actorId, job?.job_id, job?.status]);

  useEffect(() => {
    if (Platform.OS !== "web" || !actorId || job?.status !== "ready") return;
    let active = true;
    let objectURL: string | null = null;
    void fetch(profileVideoFileURL(job.job_id), { headers: actorHeaders(actorId) })
      .then(async (response) => {
        if (!response.ok) throw new Error("video unavailable");
        return response.blob();
      })
      .then((blob) => {
        if (!active) return;
        objectURL = URL.createObjectURL(blob);
        setWebURI(objectURL);
      })
      .catch(() => { if (active) setError("Chưa tải được phim. Thử mở lại Nếp nhé."); });
    return () => {
      active = false;
      if (objectURL) URL.revokeObjectURL(objectURL);
      setWebURI(null);
    };
  }, [actorId, job?.job_id, job?.status]);

  if (imageJobIds.length === 0 && !job && saved.length === 0) return null;

  const start = async () => {
    if (!actorId || sending) return;
    const selected = [...imageJobIds];
    if (!attempt.current) attempt.current = videoAttempt();
    setSending(true);
    setError(null);
    try {
      const result = await createProfileVideo(actorId, selected, attempt.current);
      setJob(result);
      setSaved((before) => [{ ...result, created_at: new Date().toISOString() }, ...before.filter((item) => item.job_id !== result.job_id)]);
      attempt.current = null;
    } catch (cause) {
      setError(cause instanceof ApiError ? cause.message : "Nếp chưa dựng được phim lúc này.");
    } finally {
      setSending(false);
    }
  };

  const confirmStart = () => {
    if (Platform.OS === "web") {
      if (globalThis.confirm("Dùng một lượt thưởng để dựng MP4 từ các bức Nếp vừa vẽ?")) void start();
      return;
    }
    Alert.alert("Dựng phim cùng Nếp?", "Dùng một lượt thưởng để dựng MP4 từ các bức Nếp vừa vẽ.", [
      { text: "Để sau", style: "cancel" },
      { text: "Dựng phim", onPress: () => void start() },
    ]);
  };

  const readyURI = job?.status === "ready" ? (Platform.OS === "web" ? webURI : profileVideoFileURL(job.job_id)) : null;
  return (
    <View style={[styles.ticket, { backgroundColor: colors.paper, borderColor: colors.ai }]} testID="nep-phim">
      <View style={styles.topline}>
        <Text style={[typography.label, { color: colors.aiInk }]}>XƯỞNG PHIM CỦA NẾP</Text>
        <Text style={[typography.caption, { color: colors.inkSoft }]}>{credits ? `${credits.available} lượt còn` : "Phần thưởng hành trình"}</Text>
      </View>
      <Text style={[typography.body, { color: colors.ink }]}>
        {job?.status === "ready" ? "Một đoạn phim thật, ghép từ những bức bạn chọn." :
          job?.status === "reserved" || job?.status === "queued" || job?.status === "running" ? "Nếp đang nối các khung hình thành phim MP4…" :
            `${imageJobIds.length} bức Nếp vừa vẽ · 1 lượt dựng cho 1 phim MP4`}
      </Text>
      {readyURI && actorId ? <VideoFrame key={readyURI} uri={readyURI} actorId={actorId} /> : null}
      {saved.length > 1 ? <View style={styles.library}>
        <Text style={[typography.label, { color: colors.inkSoft }]}>Các phim đã dựng</Text>
        <View style={styles.libraryRow}>{saved.map((item, index) => <Pressable key={item.job_id} accessibilityRole="button" {...giuState(job?.job_id === item.job_id)} onPress={() => { setWebURI(null); setJob(item); }} style={[styles.filmTab, { borderColor: job?.job_id === item.job_id ? colors.ai : colors.lineStrong }]}>
          <Text style={[typography.caption, { color: colors.ink }]}>{`Phim ${saved.length - index}`}</Text>
        </Pressable>)}</View>
      </View> : null}
      {error ? <Text style={[typography.caption, { color: colors.ink }]}>{error}</Text> : null}
      {!job || job.status === "failed" || job.status === "ready" ? (
        <Pressable
          accessibilityRole="button"
          disabled={!actorId || imageJobIds.length === 0 || sending || credits?.available === 0}
          onPress={confirmStart}
          style={[styles.action, { backgroundColor: colors.ai, opacity: !actorId || imageJobIds.length === 0 || sending || credits?.available === 0 ? 0.45 : 1 }]}
          testID="nep-dung-phim"
        >
          <Text style={[typography.label, { color: colors.aiInk }]}>{sending ? "Đang gửi tới xưởng…" : job?.status === "ready" ? "Dựng phim khác" : "Dựng thành phim"}</Text>
        </Pressable>
      ) : null}
    </View>
  );
}

const styles = StyleSheet.create({
  ticket: { borderWidth: 1, borderRadius: 16, padding: 14, gap: 10, marginTop: 12 },
  topline: { flexDirection: "row", justifyContent: "space-between", alignItems: "center", gap: 8 },
  action: { alignSelf: "flex-start", borderRadius: 999, paddingHorizontal: 18, paddingVertical: 10 },
  video: { width: "100%", aspectRatio: 9 / 16, borderRadius: 12, overflow: "hidden" },
  library: { gap: 7 },
  libraryRow: { flexDirection: "row", flexWrap: "wrap", gap: 7 },
  filmTab: { borderWidth: 1, borderRadius: 999, paddingHorizontal: 12, paddingVertical: 7 },
});
