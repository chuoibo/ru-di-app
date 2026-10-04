/** Invite an existing account without minting identities or overwriting its name. */
import { Redirect, useLocalSearchParams, useRouter } from "expo-router";
import { useRef, useState } from "react";
import { Text } from "react-native";
import { ApiError, newAttempt, type Attempt } from "../../../api";
import { timBanTheoUsername } from "../../../screens/ca-nhan/ban-be";
import { moiVaoNhom } from "../../../screens/vao-cua/cong-api";
import { useRudiSession } from "../../session";
import { typography, useRudiTheme } from "../../theme";
import { Heading, RudiButton, RudiScreen, TopBar } from "../../ui";
import { ONhapMuc } from "../../ui/ONhapMuc";
import { CuaDangNhap } from "../../ui/CuaDangNhap";
export function GroupInviteScreen() {
 const router = useRouter(); const { colors } = useRudiTheme(); const { id } = useLocalSearchParams<{ id: string }>(); const { phien, phienDaDoc } = useRudiSession();
 const [username, setUsername] = useState(""); const [busy, setBusy] = useState(false); const [error, setError] = useState<string | null>(null); const [sent, setSent] = useState<string | null>(null); const lock = useRef(false); const attempt = useRef<{ person: string; key: Attempt } | null>(null);
 if (!phienDaDoc) return null; if (!phien) return <CuaDangNhap />; if (typeof id !== "string") return <Redirect href="/messages" />;
 const invite = async () => { if (lock.current) return; lock.current = true; setBusy(true); setError(null); try { const person = await timBanTheoUsername(username, phien.person_id); if (!attempt.current || attempt.current.person !== person.person_id) attempt.current = { person: person.person_id, key: newAttempt() }; await moiVaoNhom(id, person.person_id, phien.person_id, attempt.current.key); setSent(person.display_name); } catch (e) { setError(e instanceof ApiError ? e.message : "Chưa gửi được lời mời."); } finally { setBusy(false); lock.current = false; } };
 return <RudiScreen testID="group-invite-screen"><TopBar title="Mời vào nhóm" /><Heading title="Mời bằng username" subtitle="Bạn ấy cần có tài khoản và cho phép tìm kiếm. Lời mời sẽ chờ họ đồng ý." />
 {sent ? <><Text style={[typography.body, { color: colors.ink }]}>Đã gửi lời mời tới {sent}.</Text><RudiButton label="Mời thêm người" onPress={() => { setSent(null); setUsername(""); }} /><RudiButton label="Về nhóm" variant="outline" onPress={() => router.back()} /></> : <><ONhapMuc label="Tên tài khoản" placeholder="@ten_cua_ban" autoCapitalize="none" autoCorrect={false} value={username} onChangeText={setUsername} editable={!busy} /><RudiButton label="Gửi lời mời" loading={busy} disabled={busy || !username} onPress={() => void invite()} /></>}
 {error && <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.warn }]}>{error}</Text>}</RudiScreen>;
}
