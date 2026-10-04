/**
 * Where the first kept page starts (QA UI-153): the shelf «Những ngày muốn
 * giữ», empty, said «open an outing that has passed» and offered no way to
 * one. A notebook is kept from an outing's own screen («Giữ lại cuộc đi»), so
 * the one action is the group's most recent outing that has ended; with none
 * yet, the group's outings, where one gets made.
 */
import { chiaKeo, type KeoCoNgay } from "../keo/nhip-keo";

export type LoiVaoTrangDau =
  | { kieu: "keo"; id: string; ten: string }
  | { kieu: "plan" };

export function loiVaoTrangDau<T extends KeoCoNgay & { id: string; title: string }>(keo: readonly T[], today: string): LoiVaoTrangDau {
  const [ganNhat] = chiaKeo(keo, today).daQua;
  return ganNhat ? { kieu: "keo", id: ganNhat.id, ten: ganNhat.title.trim() || "cuộc đi vừa qua" } : { kieu: "plan" };
}
