import type { ConfigContext, ExpoConfig } from "expo/config";

/** Provider IDs are public build configuration, never application secrets. */
export default ({ config }: ConfigContext): ExpoConfig => {
  const iosClientId = process.env.EXPO_PUBLIC_GOOGLE_IOS_CLIENT_ID?.trim();
  const googlePlugin = "@react-native-google-signin/google-signin";
  const iosUrlScheme = iosClientId && /^[a-zA-Z0-9-]+\.apps\.googleusercontent\.com$/.test(iosClientId)
    ? `com.googleusercontent.apps.${iosClientId.slice(0, -".apps.googleusercontent.com".length)}`
    : undefined;
  return {
    ...config,
    name: config.name ?? "RuDi",
    slug: config.slug ?? "rudi-mobile",
    plugins: (config.plugins ?? []).map((plugin) => plugin === googlePlugin && iosUrlScheme
      ? [googlePlugin, { iosUrlScheme }]
      : plugin),
  };
};
