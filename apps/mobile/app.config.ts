import type { ConfigContext, ExpoConfig } from "expo/config";
export default ({ config }: ConfigContext): ExpoConfig => ({ ...config, name: config.name ?? "RuDi", slug: config.slug ?? "rudi-mobile", plugins: [...(config.plugins ?? []).filter(p => (Array.isArray(p) ? p[0] : p) !== "@react-native-google-signin/google-signin"), "expo-video"] });
