/**
 * «Ai có gì»: the assignment read the other way round.
 *
 * The line list says, per dish, who shares it. Beside it on a wide window the
 * reader wants the inverse -- per person, which dishes -- and the dishes still
 * waiting for somebody. Counts and names only: the money is the server's to
 * split at the next step (`chiaTrenMayChu`), and a client-side estimate would
 * be a number this screen cannot stand behind (three money laws).
 */
import { isOn, type Assignment } from "../../assignment";

export interface MonCuaNguoi {
  id: string;
  ten: string;
  /** Dish names, in bill order. */
  mon: string[];
}

export interface AiCoGi {
  nguoi: MonCuaNguoi[];
  /** Dish names nobody has yet, in bill order. */
  chuaChon: string[];
}

export function aiCoGi(
  lines: readonly { id: string; name: string }[],
  nguoi: readonly { id: string; name: string }[],
  assignment: Assignment,
): AiCoGi {
  const chuaChon: string[] = [];
  const theoNguoi = nguoi.map((n) => ({ id: n.id, ten: n.name, mon: [] as string[] }));
  lines.forEach((line, i) => {
    const ten = line.name.trim() === "" ? `Món ${i + 1}` : line.name;
    let coAi = false;
    for (const n of theoNguoi) {
      if (isOn(assignment, line.id, n.id)) {
        n.mon.push(ten);
        coAi = true;
      }
    }
    if (!coAi) chuaChon.push(ten);
  });
  return { nguoi: theoNguoi, chuaChon };
}

/** One line under a name: «3 món · Bún bò, Chả giò, Trà đá» or the honest «Chưa có món nào». */
export function cauMonCuaNguoi(n: MonCuaNguoi): string {
  if (n.mon.length === 0) return "Chưa có món nào";
  return `${n.mon.length} món · ${n.mon.join(", ")}`;
}
