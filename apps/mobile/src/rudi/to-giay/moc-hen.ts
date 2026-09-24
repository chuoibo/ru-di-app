/**
 * The moment two people agreed on an evening, told from what the notebook
 * already holds -- never a message dropped into their chat (chat v2 is end to
 * end encrypted; the server writes nothing into a direct conversation).
 *
 * QA 23/09: once a sheet was agreed, the half of the screen under it held one
 * red «Huỷ buổi này» and nothing else, and the conversation said nothing at
 * all -- «the emotional peak of the whole flow (they managed to set a date)
 * had nothing». These are the words the surfaces print instead: how far away
 * the evening is, and which of their evenings it is.
 */
import { nhanNhip, nhipKeo } from "../keo/nhip-keo";
import { ngayDocDuoc } from "./ngay";
import { phienBan, type ToGiay } from "./to-giay";

const DA_HEN: readonly ToGiay["state"][] = ["chot", "da_di", "da_giu"];

/** «Còn 2 ngày», «Ngày mai», «Hôm nay»; null when the day cannot be read or has passed. */
export function demNgay(to: ToGiay, homNay: string): string | null {
  const ngay = phienBan(to)?.content.ngay;
  if (!ngay) return null;
  const nhip = nhipKeo(ngay, ngay, homNay);
  if (nhip.kieu !== "sap-toi" && nhip.kieu !== "hom-nay") return null;
  return nhanNhip(nhip);
}

/**
 * Which of their evenings this is: agreed sheets on an earlier day (or the
 * same day, created before) count before it. «Lần hẹn đầu tiên» for the first.
 */
export function cauLanHen(to: ToGiay, tatCa: readonly ToGiay[]): string | null {
  if (!DA_HEN.includes(to.state)) return null;
  const ngayCua = (t: ToGiay) => phienBan(t)?.content.ngay ?? "";
  const moc = ngayCua(to);
  const truoc = tatCa.filter((t) => t.id !== to.id && DA_HEN.includes(t.state) && ngayCua(t) !== "" && ngayCua(t) < moc).length;
  return truoc === 0 ? "Lần hẹn đầu tiên của hai bạn" : `Lần hẹn thứ ${truoc + 1} của hai bạn`;
}

/**
 * The pinned line in the pair's conversation while a plan stands:
 * «Hai bạn hẹn Thứ Bảy 26/09 · Lẩu Gà Lá É Tao Ngộ · còn 2 ngày».
 */
export function cauHenTrongChat(to: ToGiay, homNay: string, tenCho?: string): string | null {
  if (to.state !== "chot") return null;
  const pb = phienBan(to);
  if (!pb) return null;
  const phan = [`Hai bạn hẹn ${ngayDocDuoc(pb.content.ngay)}`];
  const noi = tenCho ?? pb.content.chang[0]?.viec;
  if (noi) phan.push(noi);
  const dem = demNgay(to, homNay);
  if (dem) phan.push(dem.charAt(0).toLowerCase() + dem.slice(1));
  return phan.join(" · ");
}
