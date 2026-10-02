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
import { loiChuDe } from "./chu-de";
import { CauTaiCho } from "../ui/CauTaiCho";
import { MediaPicker } from "./MediaPicker";
import { MentionPicker } from "./MentionPicker";
import { coTuongNhom } from "../so/ban-tinh";
import { KHONG_VIEN_WEB } from "../ui/khong-vien-web";
import { toggleState } from "../../ui/a11y";
/** Who reads a story: the title and the line under it, one place for both forms. */
const NGUOI_DOC: Record<Audience, [string, string]> = {
  friends: ["Riêng tư · Bạn bè", "Chỉ bạn và những người đang kết bạn với bạn."],
  public: ["Công khai · Cộng đồng", "Sau khi được duyệt, bài xuất hiện trên tường và cộng đồng."],
  only_me: ["Chỉ mình tôi", "Giữ riêng câu chuyện này cho bạn."],
  group: ["Một nhóm", "Chỉ thành viên hiện tại của nhóm được xem."],
};

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
    const [bodyFocused, setBodyFocused] = useState(false);
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
    // The topic box checks itself as the server will (QA UI-133): the sentence
    // sits under the box while typing, and «Gửi» waits with the reason.
    const loiChuDeGo = loiChuDe(topics);
    if (!phien)
        return <RudiScreen><TopBar title="Kể một khoảnh khắc"/><Text style={[typography.body, { color: colors.ink }]}>Đăng nhập để viết câu chuyện của bạn.</Text></RudiScreen>;
    const choice = (value: Audience, title: string, detail: string) => <Pressable key={value} {...toggleState("radio", audience === value, params.edit ? undefined : () => change(() => setAudience(value)))} aria-disabled={Boolean(params.edit)} disabled={Boolean(params.edit)} onPress={() => change(() => setAudience(value))} style={{ paddingVertical: 16, borderBottomWidth: 1, borderColor: colors.line, gap: 4 }}><Text style={[typography.title, { color: audience === value ? colors.accent : colors.ink }]}>{title}{audience === value ? " · Đã chọn" : ""}</Text><Text style={[typography.caption, { color: colors.inkSoft }]}>{detail}</Text></Pressable>;
    return <RudiScreen avoidKeyboard cot="form" footerInset={insets.bottom + 12} testID="community-composer" footer={<View style={{ gap: 8 }}>
      {/* A refusal from the server is said right above the button that sent it, never at the end of the scroll (QA UI-133). */}
      <CauTaiCho cau={error} testID="cong-dong-loi-gui" />
      <RudiButton label={busy ? "Đang gửi…" : params.edit ? "Gửi bản sửa" : audience === "public" ? "Gửi lên cộng đồng" : "Đăng lên tường"} disabled={busy || uploading || !body.trim() || (audience === "group" && !group) || loiChuDeGo !== null} lyDo={!body.trim() ? "Viết vài dòng về khoảnh khắc trước khi gửi." : loiChuDeGo !== null ? "Sửa phần chủ đề trước khi gửi." : audience === "group" && !group ? "Chọn một nhóm trước khi gửi." : undefined} onPress={() => void send()}/>
    </View>}>
    <TopBar title={params.edit ? "Viết tiếp câu chuyện" : "Kể một khoảnh khắc"}/>
    <Text style={[typography.h1, { color: colors.ink }]}>Hôm nay có gì đáng nhớ?</Text><Text style={[typography.body, { color: colors.inkSoft }]}>Một cuộc đi thật, một điều bạn muốn kể.</Text>
    <TextInput testID="community-body" accessibilityLabel="Nội dung bài đăng" placeholder="Quán nhỏ ở góc phố, chuyến đi còn vương nắng…" placeholderTextColor={colors.inkFaint} multiline maxLength={5000} value={body} editable={!busy} onChangeText={(value) => change(() => setBody(value))} onFocus={() => setBodyFocused(true)} onBlur={() => setBodyFocused(false)} style={[typography.body, KHONG_VIEN_WEB, { borderBottomWidth: 1, borderBottomColor: bodyFocused ? colors.accent : colors.line, color: colors.ink, minHeight: 180, textAlignVertical: "top", paddingVertical: 20, lineHeight: 28 }]}/>
    <MediaPicker person={phien.person_id} media={media} onChange={(v) => change(() => setMedia(v))} onBusy={setUploading}/>
    <Field error={loiChuDeGo} helper={loiChuDeGo === null ? "Tối đa 5 chủ đề, ngăn cách bằng dấu phẩy." : undefined} label="Chủ đề" placeholder="Ví dụ: cà phê, đi bộ, Đà Lạt" value={topics} onChangeText={(v) => change(() => setTopics(v))}/>
    <MentionPicker person={phien.person_id} selected={mentions} onChange={(v) => change(() => setMentions(v))}/>
    {/* An edit keeps who reads the story: only that choice is shown, locked,
        with where the change is made (QA UI-091 on Cộng đồng, TC-N14-SUA-KHOA);
        no «Lựa chọn khác» opening more choices that cannot be taken. */}
    <View accessibilityRole="radiogroup">{params.edit ? choice(audience, ...NGUOI_DOC[audience]) : <>{choice("friends", ...NGUOI_DOC.friends)}{choice("public", ...NGUOI_DOC.public)}{advanced ? <>{choice("only_me", ...NGUOI_DOC.only_me)}{choice("group", ...NGUOI_DOC.group)}</> : null}</>}</View>
    {params.edit ? <Text style={[typography.caption, { color: colors.inkSoft }]} testID="cong-dong-nguoi-doc-khoa">Bản sửa giữ người đọc như lúc đăng. Muốn đổi người đọc, mở «Thêm lựa chọn cho bài» (⋯) ở trang bài.</Text> : <RudiButton compact full={false} label={advanced ? "Thu gọn lựa chọn" : "Lựa chọn khác"} variant="ghost" onPress={() => setAdvanced(!advanced)}/>}
    {audience === "group" && !params.edit ? groups.map((g) => <RudiButton key={g.id} label={g.display_name} variant={group === g.id ? "solid" : "outline"} onPress={() => change(() => setGroup(g.id))}/>) : null}
    {audience === "public" ? <Text style={[typography.caption, { color: colors.inkSoft }]}>Cộng đồng dành cho đi chơi, ăn uống, du lịch và trải nghiệm. Nội dung cần được duyệt trước khi công khai; bạn có thể sửa hoặc yêu cầu xem xét.</Text> : null}
  </RudiScreen>;
}
