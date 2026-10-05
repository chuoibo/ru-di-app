import { useEffect, useId, useRef, useState } from "react";
import { Text, View } from "react-native";
import { ApiError } from "../../../api";
import { googleChallenge, googleErrorMessage, googleLabel } from "../../account";
import { typography, useRudiTheme } from "../../theme";
import { RudiButton } from "../../ui";
import type { GoogleButtonProps } from "./GoogleButton";
type GIS = { accounts: { id: { initialize(options: { client_id: string; nonce: string; auto_select: boolean; use_fedcm_for_button: boolean; ux_mode: "popup"; callback: (response: { credential: string }) => void }): void; renderButton(node: HTMLElement, options: object): void; cancel(): void } } };
declare global { interface Window { google?: GIS } }
let loaded: Promise<void> | null = null;
function loadGIS() { if (window.google) return Promise.resolve(); if (!loaded) loaded = new Promise<void>((resolve, reject) => { const script = document.createElement("script"); script.src = "https://accounts.google.com/gsi/client"; script.async = true; script.onload = () => resolve(); script.onerror = () => { loaded = null; reject(new Error("google_unavailable")); }; document.head.appendChild(script); }); return loaded; }
/** A challenge lives five minutes: renew it before then; after a failure, try again quietly. */
const RENEW_MS = 240000; const RETRY_MS = 60000;
/**
 * GIS needs the nonce before it draws, so the challenge is fetched on mount, not on press.
 * The caller re-keys this on a new session token or a fresh reauth: a challenge is bound to
 * the session that asked for it. Nothing here turns red before the person has pressed
 * anything: sign-in hides the button and retries; link and reauth say why, with «Thử lại».
 */
export function GoogleButton(props: GoogleButtonProps) {
 const id = "google-" + useId().replace(/:/g, ""); const latest = useRef(props); latest.current = props; const clientId = process.env.EXPO_PUBLIC_GOOGLE_WEB_CLIENT_ID;
 const { colors } = useRudiTheme(); const [ready, setReady] = useState(false); const [problem, setProblem] = useState<string | null>(null); const [attempt, setAttempt] = useState(0);
 useEffect(() => { if (!clientId) return; let live = true; let timer: ReturnType<typeof setTimeout> | undefined; let inFlight = false;
 const arm = (ms: number) => { clearTimeout(timer); timer = setTimeout(() => void mount(), ms); };
 const mount = async () => { const purpose = latest.current.purpose; try { await loadGIS(); if (!live) return; const challenge = await googleChallenge(purpose, latest.current.actorId); if (!live) return; const node = document.getElementById(id); if (!node || !window.google) { arm(RETRY_MS); return; } node.replaceChildren(); window.google.accounts.id.initialize({ client_id: clientId, nonce: challenge.nonce, auto_select: false, use_fedcm_for_button: false, ux_mode: "popup", callback: (response) => { if (!live || inFlight || latest.current.disabled) return; inFlight = true; void latest.current.onProof({ challenge_id: challenge.challenge_id, challenge_secret: challenge.challenge_secret, id_token: response.credential }).catch(() => latest.current.onError("Chưa đăng nhập được với Google.")).finally(() => { inFlight = false; if (live) void mount(); }); } }); window.google.accounts.id.renderButton(node, { type: "standard", theme: "outline", size: "large", text: purpose === "login" ? "signin_with" : "continue_with", locale: "vi" }); setReady(true); setProblem(null); arm(RENEW_MS); }
 catch (error) { if (!live) return; document.getElementById(id)?.replaceChildren(); setReady(false);
 // A stale reauth is not fixed by retrying; the screen re-keys this after «Xác thực lại».
 if (error instanceof ApiError && error.code === "reauthentication_required") { setProblem(purpose === "link" ? "Xác thực lại trước khi liên kết Google." : googleErrorMessage(error)); return; }
 setProblem(googleErrorMessage(error)); arm(RETRY_MS); } };
 setReady(false); setProblem(null); void mount(); return () => { live = false; clearTimeout(timer); window.google?.accounts.id.cancel(); };
 }, [clientId, id, props.purpose, props.actorId, attempt]);
 if (!clientId) return null;
 // Sign-in shows nothing until Google can be used; the password door is right above it.
 const said = props.purpose === "login" ? null : problem;
 return <View style={{ gap: 8, alignItems: "center" }}>
 {props.purpose !== "login" && (ready || said !== null) ? <Text style={[typography.label, { color: colors.ink }]}>{googleLabel[props.purpose]}</Text> : null}
 <View nativeID={id} accessibilityLabel={googleLabel[props.purpose]} pointerEvents={props.disabled ? "none" : "auto"} style={{ minHeight: ready ? 44 : 0, alignItems: "center" }} />
 {said !== null && <><Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.inkSoft, textAlign: "center" }]}>{said}</Text><RudiButton label="Thử lại" variant="outline" compact full={false} disabled={props.disabled} lyDo={props.disabled ? "Chờ thao tác đang chạy xong." : undefined} onPress={() => setAttempt(n => n + 1)} /></>}
 </View>;
}
