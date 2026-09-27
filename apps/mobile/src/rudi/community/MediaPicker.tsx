import { translatedAsActor, newAttempt } from "../../api";
import { COMMUNITY_ERRORS } from "./api";
import * as ImagePicker from "expo-image-picker";
import { manipulateAsync, SaveFormat } from "expo-image-manipulator";
import { Image } from "expo-image";
import { useState } from "react";
import { ScrollView, Text, View } from "react-native";
import { typography, useRudiTheme } from "../theme";
import { RudiButton } from "../ui";
import { imageSource, uploadMedia, type Media } from "./api";
export function MediaPicker({ person, media, onChange, video = true, onBusy }: {
    person: string;
    media: Media[];
    onChange: (next: Media[]) => void;
    video?: boolean;
    onBusy?: (value: boolean) => void;
}) {
    const { colors } = useRudiTheme();
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const pick = async () => {
        setError(null);
        try {
            const result = await ImagePicker.launchImageLibraryAsync({ mediaTypes: video ? ["images", "videos"] : ["images"], allowsMultipleSelection: false, quality: 0.85, videoMaxDuration: 180 });
            if (result.canceled)
                return;
            const asset = result.assets[0];
            if (!asset)
                return;
            if (asset.type === "video" && (asset.duration ?? 0) > 180000)
                throw new Error("Chọn video dài tối đa 3 phút nhé.");
            if (media.length >= (video ? 10 : 1))
                throw new Error(video ? "Mỗi bài tối đa 10 ảnh." : "Mỗi bình luận có một ảnh.");
            if (media.length && (asset.type === "video" || media.some((m) => m.type.startsWith("video/"))))
                throw new Error("Video có một bài riêng. Bỏ ảnh đã chọn để thêm video nhé.");
            setBusy(true);
            onBusy?.(true);
            let uri = asset.uri;
            let mime = asset.mimeType ?? (asset.type === "video" ? "video/mp4" : "image/jpeg");
            if (asset.type !== "video") {
                const clean = await manipulateAsync(uri, [{ resize: { width: Math.min(asset.width, 2048) } }], { compress: 0.86, format: SaveFormat.JPEG });
                uri = clean.uri;
                mime = "image/jpeg";
            }
            const uploaded = await uploadMedia(person, uri, mime);
            if (uploaded.state === "processing") {
                let ready = false;
                for (let n = 0; n < 60; n++) {
                    await new Promise((resolve) => setTimeout(resolve, 3000));
                    const status = await translatedAsActor<{
                        state: string;
                    }>(COMMUNITY_ERRORS, `/v2/community/media/${uploaded.id}/status`, {
                        actorId: person,
                        method: "GET"
                    });
                    if (status.state === "failed")
                        throw new Error("Video chưa xử lý được. Thử tệp MP4 khác nhé.");
                    if (status.state === "ready") {
                        ready = true;
                        break;
                    }
                }
                if (!ready)
                    throw new Error("Video vẫn đang xử lý. Bạn thử lại sau nhé.");
                uploaded.type = "video/mp4";
            }
            onChange([...media, uploaded]);
        }
        catch (e) {
            setError(e instanceof Error ? e.message : "Chưa chọn được ảnh.");
        }
        finally {
            setBusy(false);
            onBusy?.(false);
        }
    };
    return <View style={{ gap: 12 }}><ScrollView horizontal contentContainerStyle={{ gap: 12 }}>{media.map((m) => <View key={m.id} style={{ width: 120, gap: 4 }}>{m.type.startsWith("image/") ? <Image source={imageSource(person, m.url)} cachePolicy="none" style={{ width: 120, height: 120, borderRadius: 12 }}/> : <Text style={[typography.label, { color: colors.ink, padding: 20 }]}>Video đã sẵn sàng</Text>}<RudiButton compact label="Bỏ tệp" variant="ghost" disabled={busy} onPress={() => onChange(media.filter((item) => item.id !== m.id))}/></View>)}</ScrollView><RudiButton full={false} compact icon="images-outline" label={busy ? "Đang chuẩn bị tệp…" : video ? "Thêm ảnh hoặc video" : "Thêm ảnh"} variant="outline" disabled={busy} onPress={() => void pick()}/>{error ? <Text accessibilityRole="alert" style={[typography.caption, { color: colors.accent }]}>{error}</Text> : null}</View>;
}
