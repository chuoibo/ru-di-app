/** Look up an account first; only a confirmed invitation closes the envelope. */
import { Redirect, useLocalSearchParams, useRouter } from "expo-router";
import { useEffect, useRef, useState } from "react";
import { StyleSheet, Text, View } from "react-native";
import { ApiError, newAttempt, type Attempt } from "../../../api";
import { timBanTheoUsername, type NguoiTimDuoc } from "../../../screens/ca-nhan/ban-be";
import { moiVaoNhom } from "../../../screens/vao-cua/cong-api";
import { canonicalUsername, validUsername } from "../../account";
import { AnimatedEnvelope } from "../../asset-pack/react-native/AnimatedEnvelope";
import { docNhom } from "../../chat/nhom-cai-dat";
import { useRudiSession } from "../../session";
import { tenThat } from "../../ten-giu-cho";
import { bongGiay, typography, useRudiTheme } from "../../theme";
import { Heading, RudiButton, RudiScreen, TopBar } from "../../ui";
import { AvatarNguoi } from "../../ui/AvatarNguoi";
import { ONhapMuc } from "../../ui/ONhapMuc";
import { CuaDangNhap } from "../../ui/CuaDangNhap";

export function GroupInviteScreen() {
  const router = useRouter();
  const { colors, dark } = useRudiTheme();
  const { id } = useLocalSearchParams<{ id: string }>();
  const { phien, phienDaDoc } = useRudiSession();
  const [username, setUsername] = useState("");
  const [recipient, setRecipient] = useState<NguoiTimDuoc | null>(null);
  const [groupName, setGroupName] = useState<string | null>(null);
  const [busy, setBusy] = useState<"lookup" | "send" | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [sentId, setSentId] = useState<string | null>(null);
  const lock = useRef(false);
  const attempt = useRef<{ person: string; key: Attempt } | null>(null);
  useEffect(() => {
    if (!phien || typeof id !== "string") return;
    let live = true;
    setGroupName(phien.contexts?.find((group) => group.id === id)?.display_name ?? null);
    void docNhom(id, phien.person_id).then((group) => {
      if (live) setGroupName(group.display_name);
    }).catch((e) => { if (live) setError(e instanceof ApiError ? e.message : "Chưa đọc được tên nhóm. Hãy quay lại nhóm và thử lại."); });
    return () => { live = false; };
  }, [id, phien?.person_id]);
  if (!phienDaDoc) return null;
  if (!phien) return <CuaDangNhap />;
  if (typeof id !== "string") return <Redirect href="/messages" />;
  const recipientName = recipient ? tenThat(recipient.display_name) ?? `@${recipient.username ?? canonicalUsername(username)}` : null;
  const lookup = async () => {
    if (lock.current) return;
    if (!validUsername(username)) { setError("Tên tài khoản gồm 3–32 chữ, số, dấu chấm hoặc gạch dưới."); return; }
    lock.current = true; setBusy("lookup"); setError(null);
    try {
      const person = await timBanTheoUsername(username, phien.person_id);
      if (person.person_id === phien.person_id) { setError("Đó là tài khoản của bạn. Hãy tìm người bạn muốn mời."); return; }
      setRecipient(person);
    } catch (e) { setError(e instanceof ApiError ? e.message : "Chưa tìm được tài khoản. Tên đã nhập vẫn được giữ lại."); }
    finally { lock.current = false; setBusy(null); }
  };
  const invite = async () => {
    if (lock.current || !recipient) return;
    lock.current = true; setBusy("send"); setError(null);
    if (!attempt.current || attempt.current.person !== recipient.person_id) attempt.current = { person: recipient.person_id, key: newAttempt() };
    try {
      const response = await moiVaoNhom(id, recipient.person_id, phien.person_id, attempt.current.key);
      setSentId(response.id);
    } catch (e) { setError(e instanceof ApiError ? e.message : "Chưa gửi được lời mời. Bạn có thể thử lại với người này."); }
    finally { lock.current = false; setBusy(null); }
  };
  return (
    <RudiScreen material="quiet" contentStyle={styles.screen} testID="group-invite-screen">
      <TopBar title="Mời vào nhóm" />
      <Heading title={sentId ? "Lời mời đã gửi" : "Gửi một lời rủ"} subtitle="Người nhận cần đồng ý để trở thành thành viên." />
      <View style={styles.art} pointerEvents="none" accessible={false}><AnimatedEnvelope eventKey={sentId} width={230} /></View>
      <View style={[styles.letter, { backgroundColor: colors.card, borderColor: colors.lineStrong }, bongGiay(1, dark)]} testID="thu-moi-nhom">
        {groupName ? <Text style={[typography.title, { color: colors.ink }]}>{groupName}</Text> : null}
        {recipient ? (
          <View style={styles.recipient}>
            <AvatarNguoi personId={recipient.person_id} name={recipientName ?? "Người nhận"} size={48} />
            <View style={styles.flex}>
              <Text style={[typography.body, { color: colors.ink }]}>{recipientName}</Text>
              <Text style={[typography.caption, { color: colors.inkSoft }]}>{sentId ? "Đang chờ người này đồng ý" : `@${recipient.username ?? canonicalUsername(username)}`}</Text>
            </View>
          </View>
        ) : null}
        {!sentId ? <ONhapMuc label="Tên tài khoản của người nhận" placeholder="@ten_cua_ban" autoCapitalize="none" autoCorrect={false} value={username} editable={busy === null} onChangeText={(value) => { setUsername(value); setRecipient(null); setError(null); }} returnKeyType="search" onSubmitEditing={() => void lookup()} /> : <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.ink }]}>Đã gửi lời mời tới {recipientName}. Lời mời chưa phải là xác nhận tham gia.</Text>}
      </View>
      {error ? <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.warn }]}>{error}</Text> : null}
      {sentId ? <><RudiButton label="Mời thêm người" onPress={() => { setSentId(null); setRecipient(null); setUsername(""); setError(null); attempt.current = null; }} /><RudiButton label="Về nhóm" variant="outline" onPress={() => router.back()} /></> : recipient ? <RudiButton label={`Gửi lời mời tới ${recipientName}`} loading={busy === "send"} disabled={busy !== null} onPress={() => void invite()} /> : <RudiButton label="Tìm người nhận" loading={busy === "lookup"} disabled={busy !== null || !username.trim()} lyDo={!username.trim() ? "Nhập tên tài khoản của bạn ấy trước." : undefined} onPress={() => void lookup()} />}
    </RudiScreen>
  );
}
const styles = StyleSheet.create({ screen: { gap: 18, maxWidth: 560 }, art: { alignSelf: "center" }, letter: { padding: 16, borderWidth: 1, borderRadius: 3, gap: 12 }, recipient: { flexDirection: "row", alignItems: "center", gap: 12 }, flex: { flex: 1, minWidth: 0 } });
