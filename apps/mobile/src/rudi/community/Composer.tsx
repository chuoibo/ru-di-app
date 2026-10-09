import { translatedAsActor, newAttempt } from "../../api";
import { COMMUNITY_ERRORS } from "./api";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import { useLocalSearchParams, useRouter } from "expo-router";
import { useEffect, useRef, useState } from "react";
import { Pressable, StyleSheet, Text, TextInput, View, useWindowDimensions } from "react-native";
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
import { SkeletonLines } from "../ui/Skeleton";
import { AvatarNguoi } from "../ui/AvatarNguoi";
import { RudiIcon } from "../ui/RudiIcon";
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
    const { height } = useWindowDimensions();
    const params = useLocalSearchParams<{
        edit?: string;
        wall?: string;
    }>();
    const [body, setBody] = useState("");
    const [audience, setAudience] = useState<Audience>(params.wall ? "friends" : "public");
    const [topics, setTopics] = useState("");
    const [mentions, setMentions] = useState<string[]>([]);
    const [media, setMedia] = useState<Media[]>([]);
    const [groupError, setGroupError] = useState<string | null>(null);
    const [groupLoading, setGroupLoading] = useState(false);
    const [groupRetry, setGroupRetry] = useState(0);
    const [groups, setGroups] = useState<NhomTomTat[]>([]);
    const [group, setGroup] = useState<string | null>(null);
    const [audienceOpen, setAudienceOpen] = useState(false);
    const [topicsOpen, setTopicsOpen] = useState(false);
    const [busy, setBusy] = useState(false);
    const [uploading, setUploading] = useState(false);
    const [error, setError] = useState<string | null>(null);
    const [revision, setRevision] = useState(0);
    const [editReady, setEditReady] = useState(!params.edit);
    const [loading, setLoading] = useState(Boolean(params.edit));
    const [retry, setRetry] = useState(0);
    const sending = useRef(false);
    const attempt = useRef(newAttempt().key);
    useEffect(() => {
        if (!phien) return;
        let current = true;

        if (params.edit) {
            setLoading(true); setEditReady(false); setError(null);
            void getPost(phien.person_id, params.edit).then((p) => {
                if (!current) return;
                setBody(p.body); setAudience(p.audience); setTopics(p.topics.join(", ")); setMentions(p.mentions); setMedia(p.media); setRevision(p.revision); setGroup(p.context_id); setEditReady(true);
            }).catch((e) => { if (current) setError(e instanceof Error ? e.message : "Chưa mở được bản viết."); }).finally(() => { if (current) setLoading(false); });
        } else setLoading(false);
        return () => { current = false; };
    }, [phien, params.edit, retry]);
    useEffect(() => {
        if (!phien) return;
        let current = true; setGroupLoading(true); setGroupError(null);
        void docNhomCuaToi(phien.person_id).then((rows) => { if (current) setGroups(rows.filter((g) => g.my_state === "active" && coTuongNhom(g))); }).catch((e) => { if (current) setGroupError(e instanceof Error ? e.message : "Chưa đọc được nhóm của bạn."); }).finally(() => { if (current) setGroupLoading(false); });
        return () => { current = false; };
    }, [phien, groupRetry]);
    const send = async () => {
        if (!phien || sending.current || loading || uploading || !body.trim() || loiChuDe(topics) || (audience === "group" && !group) || !editReady)
            return;
        sending.current = true;
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
            sending.current = false;
            setBusy(false);
        }
    };
    const change = (update: () => void) => { attempt.current = newAttempt().key; update(); };
    // The topic box checks itself as the server will (QA UI-133): the sentence
    // sits under the box while typing, and «Gửi» waits with the reason.
    const loiChuDeGo = loiChuDe(topics);
    if (!phien)
        return <RudiScreen><TopBar title="Kể một khoảnh khắc"/><Text style={[typography.body, { color: colors.ink }]}>Đăng nhập để viết câu chuyện của bạn.</Text></RudiScreen>;
    const chooseAudience = (value: Audience) => change(() => { setAudience(value); setAudienceOpen(false); });
    const choice = (value: Audience, title: string, detail: string) => <Pressable key={value} {...toggleState("radio", audience === value, () => chooseAudience(value))} disabled={busy} onPress={() => chooseAudience(value)} style={[styles.audienceChoice, { borderColor: colors.line }]}><View style={styles.audienceChoiceTitle}><Text style={[typography.label, styles.flex, { color: audience === value ? colors.accent : colors.ink }]}>{title}</Text>{audience === value ? <RudiIcon name="checkmark" color={colors.accent} size={20} /> : null}</View><Text style={[typography.note, { color: colors.inkSoft }]}>{detail}</Text></Pressable>;
    if (loading || !editReady) return <RudiScreen material="quiet" cot="form"><TopBar title="Viết tiếp câu chuyện"/>{loading ? <SkeletonLines lines={5}/> : <CauTaiCho cau={error ?? "Chưa mở được bản viết."} hanhDong={{ label: "Thử lại", onPress: () => setRetry((n) => n + 1) }}/>}</RudiScreen>;
    const audienceLabel = { friends: "Bạn bè", public: "Cộng đồng", only_me: "Chỉ mình tôi", group: groups.find((g) => g.id === group)?.display_name ?? "Một nhóm" }[audience];
    const cannotSend = busy || uploading || !body.trim() || (audience === "group" && !group) || loiChuDeGo !== null;
    return <RudiScreen material="quiet" avoidKeyboard cot="form" bottomInset={16} footerInset={insets.bottom + 12} testID="community-composer" header={<TopBar title={params.edit ? "Sửa bài viết" : "Bài viết mới"}/>} footer={<View style={styles.footer}>
      {/* A refusal from the server is said right above the button that sent it, never at the end of the scroll (QA UI-133). */}
      <CauTaiCho cau={error} testID="cong-dong-loi-gui" />
      <Text style={[typography.note, { color: colors.inkSoft }]}>{audience === "public" ? "Bài được duyệt trước khi xuất hiện trên cộng đồng." : NGUOI_DOC[audience][1]}</Text>
      <RudiButton label={busy ? "Đang gửi…" : params.edit ? "Gửi bản sửa" : audience === "public" ? "Gửi lên cộng đồng" : "Đăng lên tường"} loading={busy} disabled={cannotSend} style={cannotSend ? { borderStyle: "solid", borderWidth: 1, backgroundColor: colors.line } : undefined} lyDo={uploading ? "Chờ tệp chuẩn bị xong trước khi gửi." : !body.trim() ? "Viết nội dung để gửi." : loiChuDeGo !== null ? "Sửa phần chủ đề trước khi gửi." : audience === "group" && !group ? "Chọn một nhóm trước khi gửi." : undefined} onPress={() => void send()}/>
    </View>}>
    <View style={styles.author}>
      <AvatarNguoi name={phien.profile?.display_name ?? "Bạn"} personId={phien.person_id} size={44} />
      <View style={styles.authorText}>
        <Text style={[typography.title, { color: colors.ink }]}>{phien.profile?.display_name ?? "Bạn"}</Text>
        <Pressable accessibilityRole="button" accessibilityLabel={`Người đọc: ${audienceLabel}`} accessibilityState={{ expanded: audienceOpen, disabled: Boolean(params.edit) }} disabled={Boolean(params.edit) || busy} onPress={() => setAudienceOpen(!audienceOpen)} style={styles.audienceTrigger}>
          <RudiIcon name={audience === "public" ? "globe-outline" : audience === "only_me" ? "lock-closed-outline" : "people-outline"} color={colors.inkSoft} size={16} />
          <Text style={[typography.label, styles.flex, { color: colors.inkSoft }]}>{audienceLabel}</Text>
          <RudiIcon name={params.edit ? "lock-closed-outline" : "chevron-down"} color={colors.inkSoft} size={16} />
        </Pressable>
      </View>
    </View>
    {/* An edit keeps who reads the story: only that choice is shown, locked,
        with where the change is made (QA UI-091 on Cộng đồng, TC-N14-SUA-KHOA);
        no «Lựa chọn khác» opening more choices that cannot be taken. */}
    {audienceOpen && !params.edit ? <View accessibilityRole="radiogroup" style={[styles.audienceOptions, { backgroundColor: colors.card, borderColor: colors.line }]}>{(Object.keys(NGUOI_DOC) as Audience[]).map((value) => choice(value, ...NGUOI_DOC[value]))}</View> : null}
    {params.edit ? <Text style={[typography.note, { color: colors.inkSoft }]} testID="cong-dong-nguoi-doc-khoa">Bản sửa giữ người đọc như lúc đăng. Muốn đổi người đọc, mở «Thêm lựa chọn cho bài» (⋯) ở trang bài.</Text> : null}
    {audience === "group" && !params.edit ? <><CauTaiCho cau={groupError} hanhDong={{ label: "Thử lại nhóm của bạn", onPress: () => setGroupRetry((n) => n + 1) }}/>{groupLoading ? <SkeletonLines lines={2}/> : !groupError && !groups.length ? <Text style={[typography.body, { color: colors.inkSoft }]}>Bạn chưa ở nhóm đang hoạt động có tường chung. Chọn một mức người đọc khác để đăng.</Text> : null}{groups.map((g) => <RudiButton key={g.id} label={g.display_name} variant={group === g.id ? "solid" : "outline"} onPress={() => change(() => setGroup(g.id))}/>)}</> : null}
    <View style={styles.writing}>
      <TextInput testID="community-body" accessibilityLabel="Nội dung bài đăng" placeholder="Bạn muốn kể điều gì?" placeholderTextColor={colors.inkSoft} multiline maxLength={5000} value={body} editable={!busy} onChangeText={(value) => change(() => setBody(value))} style={[typography.body, KHONG_VIEN_WEB, styles.body, { color: colors.ink, minHeight: Math.min(240, Math.max(112, height * 0.22)) }]} />
      <Text style={[typography.note, styles.counter, { color: colors.inkSoft }]}>{body.length}/5000</Text>
    </View>
    <View style={[styles.tools, { borderColor: colors.line }]}>
      <MediaPicker compact person={phien.person_id} media={media} onChange={(v) => change(() => setMedia(v))} onBusy={setUploading}/>
      <View style={styles.secondaryTools}>
        <RudiButton compact full={false} icon="pricetag-outline" label={topics ? "Sửa chủ đề" : "Chủ đề"} expanded={topicsOpen} variant="ghost" onPress={() => setTopicsOpen(!topicsOpen)} />
        <MentionPicker person={phien.person_id} selected={mentions} onChange={(v) => change(() => setMentions(v))}/>
      </View>
      {topicsOpen || topics || loiChuDeGo ? <Field error={loiChuDeGo} helper={loiChuDeGo === null ? "Tối đa 5 chủ đề, ngăn cách bằng dấu phẩy." : undefined} label="Chủ đề" placeholder="Cà phê, đi bộ, Đà Lạt…" value={topics} onChangeText={(v) => change(() => setTopics(v))}/> : null}
    </View>
  </RudiScreen>;
}

const styles = StyleSheet.create({
  flex: { flex: 1, minWidth: 0 },
  footer: { gap: 10 },
  author: { flexDirection: "row", alignItems: "flex-start", gap: 12 },
  authorText: { flex: 1, minWidth: 0 },
  audienceTrigger: { flexDirection: "row", alignItems: "center", gap: 6, minHeight: 48, alignSelf: "flex-start", maxWidth: "100%" },
  audienceOptions: { borderWidth: 1, borderRadius: 12, paddingHorizontal: 14 },
  audienceChoice: { minHeight: 64, paddingVertical: 12, borderBottomWidth: StyleSheet.hairlineWidth, gap: 4 },
  audienceChoiceTitle: { flexDirection: "row", alignItems: "center", gap: 8 },
  writing: { gap: 8 },
  body: { textAlignVertical: "top", paddingHorizontal: 0, paddingVertical: 4, maxHeight: 420, lineHeight: 26 },
  counter: { textAlign: "right", fontVariant: ["tabular-nums"] },
  tools: { borderTopWidth: StyleSheet.hairlineWidth, paddingTop: 8, gap: 4 },
  secondaryTools: { flexDirection: "row", flexWrap: "wrap", alignItems: "flex-start", columnGap: 12, rowGap: 4 },
});
