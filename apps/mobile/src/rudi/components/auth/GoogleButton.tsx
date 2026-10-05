import { useRef, useState } from "react";
import { ActivityIndicator, Image, Platform, Pressable, StyleSheet, Text, View } from "react-native";
import { googleChallenge, googleErrorMessage, googleLabel, type GoogleProof } from "../../account";
import { typography, useRudiTheme } from "../../theme";
export type GoogleButtonProps = {
  purpose: "login" | "link" | "reauth";
  actorId?: string;
  disabled?: boolean;
  onProof: (proof: GoogleProof) => Promise<void>;
  onError: (message: string) => void;
};
export function GoogleButton(props: GoogleButtonProps) {
  const [busy, setBusy] = useState(false);
  const lock = useRef(false);
  const { colors } = useRudiTheme();
  const clientId = process.env.EXPO_PUBLIC_GOOGLE_WEB_CLIENT_ID;
  if (Platform.OS !== "android" || !clientId) return null;
  const signin = async () => {
    if (lock.current || props.disabled) return;
    lock.current = true;
    setBusy(true);
    try {
      const challenge = await googleChallenge(props.purpose, props.actorId);
      const { requireNativeModule } = await import("expo-modules-core");
      const module = requireNativeModule<{
        signIn(clientId: string, nonce: string): Promise<string | null>;
      }>("RudiGoogle");
      const token = await module.signIn(clientId, challenge.nonce);
      if (token) await props.onProof({
        challenge_id: challenge.challenge_id,
        challenge_secret: challenge.challenge_secret,
        id_token: token,
      });
    } catch (error) {
      props.onError(googleErrorMessage(error));
    } finally {
      setBusy(false);
      lock.current = false;
    }
  };
  // Google's pre-approved image includes the required logo, font and padding.
  // Its caption is the sign-in one, so outside sign-in the purpose is printed above it.
  return <View style={styles.container}>
    {props.purpose === "login" ? null : <Text style={[typography.label, { color: colors.ink }]}>{googleLabel[props.purpose]}</Text>}
    <Pressable
      accessibilityRole="button"
      accessibilityLabel={googleLabel[props.purpose]}
      accessibilityState={{ disabled: busy || props.disabled, busy }}
      disabled={busy || props.disabled}
      onPress={() => void signin()}
      style={({ pressed }) => [styles.button, { opacity: busy || props.disabled ? 0.5 : pressed ? 0.8 : 1 }]}
    >
      <Image source={require("../../../../assets/google-signin-light.png")} resizeMode="contain" style={styles.image} />
    </Pressable>
    {busy ? <ActivityIndicator accessibilityLabel="Đang kết nối Google" style={styles.progress} /> : null}
  </View>;
}

const styles = StyleSheet.create({
  container: { minHeight: 48, alignItems: "center", justifyContent: "center", gap: 6 },
  button: { minHeight: 48, minWidth: 184, justifyContent: "center" },
  image: { width: 184, height: 40 },
  progress: { position: "absolute", right: 8 },
});
