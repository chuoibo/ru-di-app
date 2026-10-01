/**
 * Sharing a link from any screen, with an answer the screen can say out loud.
 *
 * react-native-web's `Share.share` resolves `undefined` (no `action`) or
 * rejects, so every caller that read `ketQua.action` threw on the web and the
 * batch screen told the organiser «Kiểm tra mạng» at the top of a long page,
 * while the link -- which only this phone holds -- went nowhere (QA UI-049,
 * P1). The community «Chia sẻ» swallowed the same failure (UI-136).
 *
 * One function, four outcomes the screen words in place, next to the button
 * the finger just left:
 *
 * - `da-mo-khay`: the system share sheet opened (native, or Web Share). Whether
 *   the person then sent anything is not something any platform reports.
 * - `da-chep`: no share sheet, so the text went to the clipboard; the screen
 *   says «Đã chép» and where to paste it.
 * - `huy`: the person closed the sheet. Not an error; nothing to say.
 * - `khong-duoc`: neither worked. The screen shows the link itself, selectable,
 *   so it can still be copied by hand.
 */
import { Platform, Share } from "react-native";

export type KetQuaChiaSe = "da-mo-khay" | "da-chep" | "huy" | "khong-duoc";

export type NoiDungChiaSe = {
  /** The message as it should arrive; may carry its own link. */
  text?: string;
  /** A link given apart, so the receiving app treats it as one (Web Share, iOS). */
  url?: string;
  /** The sheet's title where the platform shows one. */
  title?: string;
};

type WebShareNavigator = {
  share?: (data: { title?: string; text?: string; url?: string }) => Promise<void>;
  clipboard?: { writeText?: (text: string) => Promise<void> };
};

function navWeb(): WebShareNavigator | undefined {
  return typeof navigator === "undefined" ? undefined : (navigator as unknown as WebShareNavigator);
}

/** Copy text on the web: the async clipboard first, then the old selection trick. */
export async function chepChu(chu: string): Promise<boolean> {
  const nav = navWeb();
  try {
    if (nav?.clipboard?.writeText) {
      await nav.clipboard.writeText(chu);
      return true;
    }
  } catch {
    // Permission refused or no secure context: try the selection below.
  }
  if (typeof document === "undefined") return false;
  try {
    const vung = document.createElement("textarea");
    vung.value = chu;
    vung.setAttribute("readonly", "");
    vung.style.position = "fixed";
    vung.style.opacity = "0";
    document.body.appendChild(vung);
    vung.select();
    const ok = document.execCommand("copy");
    document.body.removeChild(vung);
    return ok;
  } catch {
    return false;
  }
}

/** Text and link as one message, for the places that take only one string. */
function motTinNhan({ text, url }: NoiDungChiaSe): string {
  return [text, url].filter(Boolean).join("\n");
}

export async function chiaSe(noiDung: NoiDungChiaSe): Promise<KetQuaChiaSe> {
  if (Platform.OS !== "web") {
    try {
      // iOS takes the link as its own item; Android reads only the message.
      const ketQua = await Share.share(
        Platform.OS === "ios" && noiDung.url
          ? { message: noiDung.text, title: noiDung.title, url: noiDung.url }
          : { message: motTinNhan(noiDung), title: noiDung.title },
      );
      return ketQua?.action === Share.dismissedAction ? "huy" : "da-mo-khay";
    } catch {
      return "khong-duoc";
    }
  }
  const nav = navWeb();
  if (typeof nav?.share === "function") {
    try {
      await nav.share({ title: noiDung.title, text: noiDung.text, url: noiDung.url });
      return "da-mo-khay";
    } catch (error) {
      // The person closed the sheet: say nothing. Anything else (no user
      // gesture, a refused type) falls through to the clipboard.
      if (error instanceof Error && error.name === "AbortError") return "huy";
    }
  }
  return (await chepChu(motTinNhan(noiDung))) ? "da-chep" : "khong-duoc";
}
