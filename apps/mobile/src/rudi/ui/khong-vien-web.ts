/**
 * B7 (QC 24/09): on the web a focused `TextInput` drew the browser's own
 * outline on top of the field's own focus rule -- two borders, and a yellow
 * frame round the six OTP cells. Every text field here draws its own focus,
 * so the browser's is switched off. Native has no such outline.
 */
import { Platform, type TextStyle } from "react-native";

export const KHONG_VIEN_WEB = (Platform.OS === "web" ? { outlineStyle: "none", outlineWidth: 0 } : {}) as TextStyle;
