/**
 * Which painted scene may stand in for a place or a destination that has no
 * photograph.
 *
 * The scenes carry no label since 2026-10-20, so a painting is read as the
 * place itself. Four of the six paint somewhere in particular: the lake, the
 * pines and the church tower of Đà Lạt (`scene-hills`, and the same view behind
 * the cup of `scene-cafe`), the yellow lantern street of Hội An by day and by
 * night (`scene-street`, `scene-lanterns`). A Hà Nội eatery under Hội An's
 * lanterns is a picture of another city wearing this one's name (finish review
 * 21/10). So a landmark scene is drawn only inside its own province, the beach
 * only where there is a coast, and the one interior (`scene-atelier`) anywhere.
 * When nothing honest is left, the caller draws the category's object instead.
 *
 * Pure, so a node test can hold the rule over every scene and every region.
 */

/** The painted scenes of the asset pack (`AssetIllustration` names). */
export type TenCanh = "scene-hills" | "scene-street" | "scene-lanterns" | "scene-coast" | "scene-atelier" | "scene-cafe";

/** A region a painted scene depicts. */
export type Vung = "lam-dong" | "hoi-an" | "bien";

/** What each scene shows; null is an interior that is true of anywhere. */
export const VUNG_CUA_CANH: Readonly<Record<TenCanh, Vung | null>> = {
  "scene-hills": "lam-dong",
  "scene-cafe": "lam-dong",
  "scene-street": "hoi-an",
  "scene-lanterns": "hoi-an",
  "scene-coast": "bien",
  "scene-atelier": null,
};

/**
 * Where each region is: catalogue destination ids, and the words an address
 * carries there (lower-case, no diacritics). Lâm Đồng is the 2025 province
 * (d-tinh-68); Đà Nẵng (d-tinh-48) holds Hội An since the same merger. The
 * coast lists only destinations whose own description names a sea, bay or
 * island, so the beach is never guessed.
 */
const VUNG: Readonly<Record<Vung, { diemDen: readonly string[]; tu: readonly string[] }>> = {
  "lam-dong": { diemDen: ["d-da-lat", "d-tinh-68"], tu: ["lam dong", "da lat"] },
  "hoi-an": { diemDen: ["d-hoi-an", "d-tinh-48"], tu: ["hoi an", "quang nam"] },
  bien: {
    diemDen: [
      "d-da-nang", "d-nha-trang", "d-phu-quoc", "d-vung-tau", "d-quy-nhon", "d-mui-ne", "d-hoi-an",
      "d-tinh-22", "d-tinh-31", "d-tinh-38", "d-tinh-40", "d-tinh-42", "d-tinh-44", "d-tinh-48",
      "d-tinh-51", "d-tinh-52", "d-tinh-56", "d-tinh-91",
    ],
    tu: ["nha trang", "vung tau", "phu quoc", "quy nhon", "mui ne", "phan thiet", "ha long", "sam son", "cua lo"],
  },
};

/** The scenes a category may use, its own first; anything else falls to the landscape pair. */
const CANH_THEO_LOAI: Readonly<Record<string, readonly TenCanh[]>> = {
  cafe: ["scene-cafe", "scene-atelier"],
  "quan-an-local": ["scene-street", "scene-atelier"],
  "di-choi-dem": ["scene-lanterns", "scene-street"],
  "vui-choi": ["scene-atelier"],
};
const CANH_KHAC: readonly TenCanh[] = ["scene-hills", "scene-coast"];

function gapChu(s: string): string {
  return s.normalize("NFD").replace(/[̀-ͯ]/g, "").replace(/đ/g, "d").replace(/Đ/g, "D").toLowerCase();
}

/** The regions a place or destination is known to be in: from the destination id, else from the address words. */
export function vungCua({ diemDen, diaChi }: { diemDen?: string | null; diaChi?: string | null }): ReadonlySet<Vung> {
  const ra = new Set<Vung>();
  const chu = diaChi ? gapChu(diaChi) : "";
  for (const [vung, noi] of Object.entries(VUNG) as [Vung, (typeof VUNG)[Vung]][]) {
    if ((diemDen && noi.diemDen.includes(diemDen)) || (chu !== "" && noi.tu.some((t) => chu.includes(t)))) ra.add(vung);
  }
  return ra;
}

/** Whether a scene may be drawn for something in these regions. */
export function canhHopVung(canh: TenCanh, vung: ReadonlySet<Vung>): boolean {
  const cua = VUNG_CUA_CANH[canh];
  return cua === null || vung.has(cua);
}

/** The honest scenes for a category in these regions, in order of preference. */
export function canhDuocDung(loai: string | undefined, vung: ReadonlySet<Vung>): TenCanh[] {
  return (CANH_THEO_LOAI[loai ?? ""] ?? CANH_KHAC).filter((c) => canhHopVung(c, vung));
}

/** A stable small number from an id, so a place keeps its scene between renders. */
function bam(id: string): number {
  let h = 0;
  for (let i = 0; i < id.length; i += 1) h = (h * 31 + id.charCodeAt(i)) >>> 0;
  return h;
}

/**
 * One place's scene, or null when no painted scene is honest for it.
 *
 * Category first; among the category's honest scenes the id picks where to
 * start, and `truoc` (the scene of the card just above) is skipped when there
 * is another to take, so two neighbours do not wear the same painting.
 */
export function canhChoDiaDiem(
  { id, loai, vung }: { id?: string; loai?: string; vung: ReadonlySet<Vung> },
  truoc: TenCanh | null = null,
): TenCanh | null {
  const ds = canhDuocDung(loai, vung);
  if (ds.length === 0) return null;
  const dau = id === undefined ? 0 : bam(id) % ds.length;
  for (let i = 0; i < ds.length; i += 1) {
    const c = ds[(dau + i) % ds.length];
    if (c !== truoc) return c;
  }
  return ds[dau];
}

/**
 * Scenes for a list, top to bottom. Only a card that draws its scene (no
 * photograph) counts as the neighbour the next one must differ from.
 */
export function phanCanhDanhSach(
  items: readonly { id: string; loai?: string; coAnh: boolean }[],
  vung: ReadonlySet<Vung>,
): ReadonlyMap<string, TenCanh | null> {
  const ra = new Map<string, TenCanh | null>();
  let truoc: TenCanh | null = null;
  for (const it of items) {
    const canh = canhChoDiaDiem({ id: it.id, loai: it.loai, vung }, truoc);
    ra.set(it.id, canh);
    if (!it.coAnh) truoc = canh;
  }
  return ra;
}

/** A destination's own painted scene, only where the painting is of that region; else null (draw its line sketch). */
export function canhDiemDen(id: string): TenCanh | null {
  const vung = vungCua({ diemDen: id });
  if (vung.has("lam-dong")) return "scene-hills";
  if (id === "d-hoi-an") return "scene-lanterns";
  if (vung.has("bien")) return "scene-coast";
  return null;
}
