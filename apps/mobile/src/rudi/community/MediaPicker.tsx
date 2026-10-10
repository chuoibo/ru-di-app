import { translatedAsActor } from "../../api";
import { COMMUNITY_ERRORS } from "./api";
import * as ImagePicker from "expo-image-picker";
import { manipulateAsync, SaveFormat } from "expo-image-manipulator";
import { Image } from "expo-image";
import { useEffect, useRef, useState, type ReactNode } from "react";
import { ScrollView, Text, View } from "react-native";
import { typography, useRudiTheme } from "../theme";
import { RudiButton } from "../ui";
import { imageSource, uploadMedia, type Media } from "./api";

export type SelectedFile = { asset: ImagePicker.ImagePickerAsset; uploaded?: Media & { state?: string } };

export function MediaPicker({ person, media, onChange, video = true, onBusy, initialFile, onFileChange, compact = false, renderToolbar }: {
    person: string;
    media: Media[];
    onChange: (next: Media[]) => void;
    video?: boolean;
    onBusy?: (value: boolean) => void;
    initialFile?: SelectedFile | null;
    onFileChange?: (file: SelectedFile | null) => void;
    /** An attachment tool in a composer, rather than a standalone form field. */
    compact?: boolean;
    /** Keep a composer's tools together while previews/errors retain full width. */
    renderToolbar?: (pick: () => void, busy: boolean) => ReactNode;
}) {
    const { colors } = useRudiTheme();
    const [busy, setBusy] = useState(false);
    const [phase, setPhase] = useState<"picking" | "uploading" | "processing" | null>(null);
    const [error, setError] = useState<string | null>(null);
    const [selected, setSelected] = useState<SelectedFile | null>(initialFile ?? null);
    useEffect(() => { onFileChange?.(selected); }, [selected, onFileChange]);
    const working = useRef(false);
    const mounted = useRef(true);
    const generation = useRef(0);
    useEffect(() => { mounted.current = true; return () => { mounted.current = false; generation.current++; }; }, []);
    const prepare = async (retry?: SelectedFile) => {
        if (working.current) return;
        working.current = true;
        const operation = ++generation.current;
        const current = () => mounted.current && generation.current === operation;
        setBusy(true); setPhase(retry?.uploaded ? "processing" : "picking"); onBusy?.(true); setError(null);
        try {
            let file = retry;
            if (!file) {
                const result = await ImagePicker.launchImageLibraryAsync({ mediaTypes: video ? ["images", "videos"] : ["images"], allowsMultipleSelection: false, quality: 0.85, videoMaxDuration: 180 });
                if (!current() || result.canceled || !result.assets[0]) return;
                file = { asset: result.assets[0] };
            }
            const asset = file.asset;
            if (asset.type === "video" && (asset.duration ?? 0) > 180000) throw new Error("Chọn video dài tối đa 3 phút nhé.");
            if (media.length >= (video ? 10 : 1)) throw new Error(video ? "Mỗi bài tối đa 10 ảnh." : "Mỗi bình luận có một ảnh.");
            if (media.length && (asset.type === "video" || media.some((m) => m.type.startsWith("video/")))) throw new Error("Video có một bài riêng. Bỏ ảnh đã chọn để thêm video nhé.");
            setSelected(file);
            let uploaded = file.uploaded;
            if (!uploaded) {
                setPhase("uploading");
                let uri = asset.uri;
                let mime = asset.mimeType ?? (asset.type === "video" ? "video/mp4" : "image/jpeg");
                if (asset.type !== "video") {
                    const clean = await manipulateAsync(uri, [{ resize: { width: Math.min(asset.width, 2048) } }], { compress: 0.86, format: SaveFormat.JPEG });
                    uri = clean.uri; mime = "image/jpeg";
                }
                if (!current()) return;
                uploaded = await uploadMedia(person, uri, mime);
                if (!current()) return;
                file = { asset, uploaded }; setSelected(file);
            }
            if (uploaded.state === "processing") {
                setPhase("processing");
                let ready = false;
                for (let n = 0; n < 60; n++) {
                    await new Promise((resolve) => setTimeout(resolve, 3000));
                    if (!current()) return;
                    const status = await translatedAsActor<{ state: string }>(COMMUNITY_ERRORS, `/v2/community/media/${uploaded.id}/status`, { actorId: person, method: "GET" });
                    if (!current()) return;
                    if (status.state === "failed") {
                        setSelected({ asset });
                        throw new Error("Video chưa xử lý được. Thử tải lại hoặc chọn tệp MP4 khác nhé.");
                    }
                    if (status.state === "ready") { ready = true; break; }
                }
                if (!ready) throw new Error("Video vẫn đang xử lý. Thử lại để kiểm tra tệp này nhé.");
                uploaded = { ...uploaded, type: "video/mp4", state: "ready" };
            }
            if (current()) { onChange([...media, uploaded]); setSelected(null); }
        } catch (e) {
            if (current()) setError(e instanceof Error ? e.message : "Chưa chuẩn bị được tệp. Tệp bạn chọn vẫn ở đây.");
        } finally {
            if (current()) { working.current = false; setBusy(false); setPhase(null); onBusy?.(false); }
        }
    };
    return <View style={{ gap: 12 }}>
        {media.length ? <ScrollView horizontal contentContainerStyle={{ gap: 12 }}>{media.map((m) => <View key={m.id} style={{ width: 120, gap: 4 }}>
            {m.type.startsWith("image/") ? <Image accessibilityLabel="Ảnh đã đính kèm" source={imageSource(person, m.url)} cachePolicy="none" style={{ width: 120, height: 120, borderRadius: 8 }}/> : <Text style={[typography.label, { color: colors.ink, paddingVertical: 20 }]}>Video đã sẵn sàng</Text>}
            <RudiButton compact label="Bỏ tệp" accessibilityLabel={`Bỏ tệp đính kèm ${media.indexOf(m) + 1}`} variant="ghost" disabled={busy} onPress={() => onChange(media.filter((item) => item.id !== m.id))}/>
        </View>)}</ScrollView> : null}
        {renderToolbar ? renderToolbar(() => void prepare(), busy) : <RudiButton full={false} compact icon="images-outline" label={busy ? "Đang chuẩn bị tệp…" : video ? "Thêm ảnh hoặc video" : "Thêm ảnh"} variant={compact ? "ghost" : "outline"} disabled={busy} loading={busy} onPress={() => void prepare()}/>}
        {error ? <Text accessibilityRole="alert" style={[typography.caption, { color: colors.accent }]}>{error}</Text> : null}
        {selected && !busy ? <View style={{ flexDirection: "row", flexWrap: "wrap", gap: 8 }}><RudiButton full={false} label="Thử lại tệp đã chọn" variant="outline" onPress={() => void prepare(selected)}/><RudiButton full={false} label="Bỏ tệp đang chờ" variant="ghost" onPress={() => { setSelected(null); setError(null); }}/></View> : null}
        {phase === "processing" ? <RudiButton full={false} label="Dừng chờ tệp" variant="ghost" onPress={() => { generation.current++; working.current = false; setBusy(false); setPhase(null); onBusy?.(false); setError("Đã dừng chờ. Thử lại để kiểm tra tệp đang xử lý."); }}/> : null}
        {busy ? <Text style={[typography.caption, { color: colors.inkSoft }]}>Lời viết vẫn ở đây trong lúc tệp được chuẩn bị.</Text> : null}
    </View>;
}
