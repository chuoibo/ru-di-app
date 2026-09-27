import { translatedAsActor, newAttempt } from "../../api";
import { COMMUNITY_ERRORS } from "./api";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { useLocalSearchParams, useRouter } from "expo-router";
import { useEffect, useRef, useState } from "react";
import { Pressable, Text, TextInput, View } from "react-native";
import { docNhomCuaToi, type NhomTomTat } from "../../phien";
import { useRudiSession } from "../session";
import { typography, useRudiTheme } from "../theme";
import { Field, RudiButton, RudiScreen, TopBar } from "../ui";
import { createPost, getPost, type Audience, type Media } from "./api";
import { MediaPicker } from "./MediaPicker";
import { MentionPicker } from "./MentionPicker";
import { coTuongNhom } from "../so/ban-tinh";
export function Composer() {
    const router = useRouter();
    const insets = useSafeAreaInsets();
    const { phien } = useRudiSession();
    const { colors } = useRudiTheme();
    const params = useLocalSearchParams<{
        edit?: string;
        wall?: string;
    }>();
    const [body, setBody] = useState("");
    const [audience, setAudience] = useState<Audience>(params.wall ? "friends" : "public");
    const [topics, setTopics] = useState("");
    const [mentions, setMentions] = useState<string[]>([]);
    const [media, setMedia] = useState<Media[]>([]);
    const [groups, setGroups] = useState<NhomTomTat[]>([]);
    const [group, setGroup] = useState<string | null>(null);
    const [advanced, setAdvanced] = useState(false);
    const [busy, setBusy] = useState(false);
    const [uploading, setUploading] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [revision, setRevision] = useState(0);
    const attempt = useRef(newAttempt().key);
    useEffect(() => { if (!phien)
        return; void docNhomCuaToi(phien.person_id).then((rows) => setGroups(rows.filter((g) => g.my_state === "active" && coTuongNhom(g)))).catch(() => { }); if (params.edit)
        void getPost(phien.person_id, params.edit).then((p) => { setBody(p.body); setAudience(p.audience); setTopics(p.topics.join(", ")); setMentions(p.mentions); setMedia(p.media); setRevision(p.revision); setGroup(p.context_id); }).catch((e) => setError(String(e.message))); }, [phien, params.edit]);
    const send = async () => {
        if (!phien || busy || uploading)
            return;
        setBusy(true);
        setError(null);
        const input = { logical_id: attempt.current, body, audience, topics: topics.split(",").map((s) => s.trim()).filter(Boolean), mentions, media_ids: media.map((m) => m.id), context_id: audience === "group" ? group : null, revision };
        try {
            const post = params.edit ? await translatedAsActor<{
                id: string;
            }>(COMMUNITY_ERRORS, `/v2/community/posts/${params.edit}`, {
                actorId: phien.person_id,
                method: "PUT",
                body: input,
                attempt: newAttempt()
            }) : await createPost(phien.person_id, input);
            router.replace(`/community/posts/${post.id}` as never);
        }
        catch (e) {
            setError(e instanceof Error ? e.message : "Chưa gửi được bài. Bản viết vẫn ở đây.");
        }
        finally {
            setBusy(false);
        }
    };
    const change = (update: () => void) => { attempt.current = newAttempt().key; update(); };
    if (!phien)
        return <RudiScreen><TopBar title="Kể một khoảnh khắc"/><Text style={[typography.body, { color: colors.ink }]}>Đăng nhập để viết câu chuyện của bạn.</Text></RudiScreen>;
    const choice = (value: Audience, title: string, detail: string) => <Pressable key={value} accessibilityRole="radio" accessibilityState={{ selected: audience === value }} disabled={Boolean(params.edit)} onPress={() => change(() => setAudience(value))} style={{ paddingVertical: 16, borderBottomWidth: 1, borderColor: colors.line, gap: 4 }}><Text style={[typography.title, { color: audience === value ? colors.accent : colors.ink }]}>{title}{audience === value ? " · Đã chọn" : ""}</Text><Text style={[typography.caption, { color: colors.inkSoft }]}>{detail}</Text></Pressable>;
    return <RudiScreen avoidKeyboard footerInset={insets.bottom + 12} testID="community-composer" footer={<RudiButton label={busy ? "Đang gửi…" : params.edit ? "Gửi bản sửa" : audience === "public" ? "Gửi lên cộng đồng" : "Đăng lên tường"} disabled={busy || uploading || !body.trim() || (audience === "group" && !group)} onPress={() => void send()}/>}>
    <TopBar title={params.edit ? "Viết tiếp câu chuyện" : "Kể một khoảnh khắc"}/>
    <Text style={[typography.h1, { color: colors.ink }]}>Hôm nay có gì đáng nhớ?</Text><Text style={[typography.body, { color: colors.inkSoft }]}>Một cuộc đi thật, một điều bạn muốn kể.</Text>
    <TextInput testID="community-body" accessibilityLabel="Nội dung bài đăng" placeholder="Quán nhỏ ở góc phố, chuyến đi còn vương nắng…" placeholderTextColor={colors.inkFaint} multiline maxLength={5000} value={body} editable={!busy} onChangeText={(value) => change(() => setBody(value))} style={[typography.body, { color: colors.ink, minHeight: 180, textAlignVertical: "top", paddingVertical: 20, lineHeight: 28 }]}/>
    <MediaPicker person={phien.person_id} media={media} onChange={(v) => change(() => setMedia(v))} onBusy={setUploading}/>
    <Field label="Chủ đề" placeholder="Ví dụ: cà phê, đi bộ, Đà Lạt" value={topics} onChangeText={(v) => change(() => setTopics(v))}/>
    <Text style={[typography.caption, { color: colors.inkFaint }]}>Tối đa 5 chủ đề, ngăn cách bằng dấu phẩy.</Text>
    <MentionPicker person={phien.person_id} selected={mentions} onChange={(v) => change(() => setMentions(v))}/>
    <View accessibilityRole="radiogroup">{choice("friends", "Riêng tư · Bạn bè", "Chỉ bạn và những người đang kết bạn với bạn.")}{choice("public", "Công khai · Cộng đồng", "Sau khi được duyệt, bài xuất hiện trên tường và cộng đồng.")}{advanced ? <>{choice("only_me", "Chỉ mình tôi", "Giữ riêng câu chuyện này cho bạn.")}{choice("group", "Một nhóm", "Chỉ thành viên hiện tại của nhóm được xem.")}</> : null}</View>
    <RudiButton compact full={false} label={advanced ? "Thu gọn lựa chọn" : "Lựa chọn khác"} variant="ghost" onPress={() => setAdvanced(!advanced)}/>
    {audience === "group" ? groups.map((g) => <RudiButton key={g.id} label={g.display_name} variant={group === g.id ? "solid" : "outline"} onPress={() => change(() => setGroup(g.id))}/>) : null}
    {audience === "public" ? <Text style={[typography.caption, { color: colors.inkSoft }]}>Cộng đồng dành cho đi chơi, ăn uống, du lịch và trải nghiệm. Nội dung cần được duyệt trước khi công khai; bạn có thể sửa hoặc yêu cầu xem xét.</Text> : null}
    {error ? <Text accessibilityRole="alert" style={[typography.body, { color: colors.accent }]}>{error}</Text> : null}
  </RudiScreen>;
}
