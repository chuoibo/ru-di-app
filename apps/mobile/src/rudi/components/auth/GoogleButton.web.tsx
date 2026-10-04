import { useEffect, useId, useRef } from "react";
import { View } from "react-native";
import { googleChallenge } from "../../account";
import type { GoogleButtonProps } from "./GoogleButton";
type GIS = { accounts: { id: { initialize(options: { client_id: string; nonce: string; auto_select: boolean; use_fedcm_for_button: boolean; ux_mode: "popup"; callback: (response: { credential: string }) => void }): void; renderButton(node: HTMLElement, options: object): void; cancel(): void } } };
declare global { interface Window { google?: GIS } }
let loaded: Promise<void> | null = null;
function loadGIS() { if (window.google) return Promise.resolve(); if (!loaded) loaded = new Promise<void>((resolve, reject) => { const script = document.createElement("script"); script.src = "https://accounts.google.com/gsi/client"; script.async = true; script.onload = () => resolve(); script.onerror = () => { loaded = null; reject(new Error("google_unavailable")); }; document.head.appendChild(script); }); return loaded; }
export function GoogleButton(props: GoogleButtonProps) {
 const id = "google-" + useId().replace(/:/g, ""); const latest = useRef(props); latest.current = props; const clientId = process.env.EXPO_PUBLIC_GOOGLE_WEB_CLIENT_ID;
 useEffect(() => { if (!clientId) return; let live = true; let timer: ReturnType<typeof setTimeout>; let inFlight = false;
 const mount = async () => { try { await loadGIS(); if (!live) return; const challenge = await googleChallenge(latest.current.purpose, latest.current.actorId); if (!live) return; const node = document.getElementById(id); if (!node || !window.google) return; node.replaceChildren(); window.google.accounts.id.initialize({ client_id: clientId, nonce: challenge.nonce, auto_select: false, use_fedcm_for_button: false, ux_mode: "popup", callback: (response) => { if (!live || inFlight || latest.current.disabled) return; inFlight = true; void latest.current.onProof({ challenge_id: challenge.challenge_id, challenge_secret: challenge.challenge_secret, id_token: response.credential }).catch(() => latest.current.onError("Chưa đăng nhập được với Google.")).finally(() => { inFlight = false; if (live) void mount(); }); } }); window.google.accounts.id.renderButton(node, { type: "standard", theme: "outline", size: "large", text: "signin_with", locale: "vi" }); clearTimeout(timer); timer = setTimeout(() => void mount(), 240000); } catch { if (live) latest.current.onError("Chưa kết nối được với Google. Hãy thử lại."); } };
 void mount(); return () => { live = false; clearTimeout(timer); window.google?.accounts.id.cancel(); };
 }, [clientId, id, props.purpose, props.actorId]);
 return clientId ? <View nativeID={id} pointerEvents={props.disabled ? "none" : "auto"} style={{ minHeight: 44, alignItems: "center" }} /> : null;
}
