/** One place in full, from the server rather than from the list behind it.
 *
 * `GET /places/{place_id}` (work item rd-be-05) returns everything `GET /places`
 * returns plus the two fields the grid deliberately omits: `description`, the
 * prose about the venue, and `reviews`, what people wrote about it. Until this
 * file existed nothing in the app called it -- the detail screen was drawn
 * entirely out of the card the person had just tapped, so two fields the server
 * was already computing reached no screen at all.
 *
 * That is the whole reason for the route, and it is worth stating why it is a
 * route rather than more columns on the list: prose and reviews are the biggest
 * payload per place and are read for exactly one place at a time. Sending them
 * for all forty is a list response that grows with the corpus for the benefit of
 * a screen that shows one row of it.
 *
 * ## The list place stays the fallback, and that is deliberate
 *
 * `ChiTietDiaDiem` renders from the tapped `Place` immediately and folds this
 * response in when it lands. So a server without the route, an offline phone, or
 * a slow answer costs the description and the reviews and costs nothing else --
 * the screen never blanks, and it never waits before drawing a name, a price
 * band and the match reason it was opened for. The failure is visible (the
 * screen says so in one line) rather than silent, but it is not fatal.
 *
 * No React here on purpose, same rule as `places.ts`: the wire shape and the
 * refusals are checkable without a phone.
 */
import { chiTietLoi } from "../../ui/loi-tren-man";
import { parsePlace, PLACES_BASE_URL, PLACES_WORK_ITEM, type Place } from "./places";

/** One review as the server sends it. Three fields, no id and no date -- the
 *  server does not have either, and inventing "2 ngày trước" under a sentence
 *  is the kind of decoration that turns into a support question. */
export type Review = {
  author: string;
  /** 0-5, one place. Not forced to an integer: 4.5 is a real rating. */
  rating: number;
  body: string;
};

/** One signature dish; every field but the name may be absent. */
export type MonNenThu = { ten: string; moTa: string | null; huongVi: string | null; cachAn: string | null; gia: string | null };
export type NhanMa = { id: string; nhan: string };
export type GioTheoThu = { thu: string; nhan: string; mo: string; dong: string; quaDem: boolean };

/**
 * The page's index of a place (`?include=highlights`): what is worth knowing,
 * grouped the way a reader looks things up. The server drops every empty key
 * and filters what it must never print (evidence, gaps, allergy claims), so
 * an empty value here means «nothing to say» and the screen hides the section
 * rather than writing «Chưa có…». A malformed piece is dropped, never thrown:
 * the facts above it on the page do not depend on this.
 */
export type Highlights = {
  taiSaoDangDen: string | null;
  khongKhi: string | null;
  khungGioDep: string | null;
  mucGia: string | null;
  gioGhiChu: string | null;
  phuHopVoi: string[];
  khauViPhuHop: string[];
  luuY: string[];
  traiNghiem: string[];
  monChinh: string[];
  monPhaiThu: MonNenThu[];
  gia: { minVnd: number; maxVnd: number | null; uoc: boolean } | null;
  gioMoCua: GioTheoThu[];
  canDatTruoc: { ma: string; nhan: string } | null;
  trongNgoai: { ma: string; nhan: string } | null;
  thoiLuong: { min: number | null; max: number | null } | null;
  danhMuc: NhanMa[];
  khiChat: NhanMa[];
};

const chuKhac = (v: unknown): string | null => (typeof v === "string" && v.trim() !== "" ? v.trim() : null);
const dsChu = (v: unknown): string[] => (Array.isArray(v) ? v.map(chuKhac).filter((x): x is string => x !== null) : []);
const dongDem = (v: unknown): number | null => (typeof v === "number" && Number.isInteger(v) && v >= 0 ? v : null);
function nhanMa(v: unknown): { ma: string; nhan: string } | null {
  const o = v as Record<string, unknown> | null;
  const ma = chuKhac(o?.ma);
  const nhan = chuKhac(o?.nhan);
  return ma !== null && nhan !== null ? { ma, nhan } : null;
}
function dsNhan(v: unknown): NhanMa[] {
  if (!Array.isArray(v)) return [];
  return v.flatMap((x) => {
    const o = x as Record<string, unknown> | null;
    const id = chuKhac(o?.id);
    const nhan = chuKhac(o?.nhan);
    return id !== null && nhan !== null ? [{ id, nhan }] : [];
  });
}

export function parseHighlights(raw: unknown): Highlights | null {
  if (raw === null || typeof raw !== "object" || Array.isArray(raw)) return null;
  const h = raw as Record<string, unknown>;
  const monPhaiThu = Array.isArray(h.mon_phai_thu)
    ? h.mon_phai_thu.flatMap((m): MonNenThu[] => {
        const o = m as Record<string, unknown> | null;
        const ten = chuKhac(o?.ten);
        return ten === null ? [] : [{ ten, moTa: chuKhac(o?.mo_ta), huongVi: chuKhac(o?.huong_vi), cachAn: chuKhac(o?.cach_an), gia: chuKhac(o?.gia) }];
      })
    : [];
  const g = h.gia as Record<string, unknown> | null | undefined;
  const giaMin = dongDem(g?.min_vnd);
  const gioMoCua = Array.isArray(h.gio_mo_cua)
    ? h.gio_mo_cua.flatMap((x): GioTheoThu[] => {
        const o = x as Record<string, unknown> | null;
        const thu = chuKhac(o?.thu);
        const nhan = chuKhac(o?.nhan);
        const mo = chuKhac(o?.mo);
        const dong = chuKhac(o?.dong);
        return thu && nhan && mo && dong ? [{ thu, nhan, mo, dong, quaDem: o?.qua_dem === true }] : [];
      })
    : [];
  const tl = h.thoi_luong_phut as Record<string, unknown> | null | undefined;
  const thoiLuong = tl && typeof tl === "object" ? { min: dongDem(tl.min), max: dongDem(tl.max) } : null;
  return {
    taiSaoDangDen: chuKhac(h.tai_sao_dang_den),
    khongKhi: chuKhac(h.khong_khi),
    khungGioDep: chuKhac(h.khung_gio_dep_nhat),
    mucGia: chuKhac(h.muc_gia),
    gioGhiChu: chuKhac(h.gio_ghi_chu),
    phuHopVoi: dsChu(h.phu_hop_voi),
    khauViPhuHop: dsChu(h.khau_vi_phu_hop),
    luuY: dsChu(h.luu_y),
    traiNghiem: dsChu(h.trai_nghiem_phai_thu),
    monChinh: dsChu(h.mon_chinh),
    monPhaiThu,
    gia: giaMin === null ? null : { minVnd: giaMin, maxVnd: dongDem(g?.max_vnd), uoc: g?.uoc === true },
    gioMoCua,
    canDatTruoc: nhanMa(h.can_dat_truoc),
    trongNgoai: nhanMa(h.trong_nha_ngoai_troi),
    thoiLuong: thoiLuong !== null && (thoiLuong.min !== null || thoiLuong.max !== null) ? thoiLuong : null,
    danhMuc: dsNhan(h.danh_muc),
    khiChat: dsNhan(h.khi_chat),
  };
}

/** A `Place`, plus the fields only the detail route carries. */
export type PlaceDetail = Place & {
  /** Prose about the venue, or null when the server has none for it. */
  description: string | null;
  reviews: Review[];
  /**
   * Whether photographs of this venue exist to be shown.
   *
   * True once a licensed photograph has been imported for the place (M12),
   * false for a place nobody has photographed yet. Still read rather than
   * inferred, because the gallery is a second request: the screen has to know
   * whether to make it at all, and an empty strip while that request is in
   * flight is worse than no strip.
   */
  photosAvailable: boolean;
  /**
   * What people do at this venue, in the server's words.
   *
   * Written when the place was imported, out of its own OpenStreetMap tags,
   * rather than asked of a model when the screen opens. So the sentences
   * describe what somebody recorded on the ground, and an empty list is the
   * honest answer for a venue whose tags say nothing about it -- the screen
   * leaves the section out instead of inventing a line for it.
   */
  activities: string[];
  /** The index of what is worth knowing; null when the server sent none. */
  highlights: Highlights | null;
};

/**
 * Everything this screen's extra fetch can be.
 *
 * `khong-co` is separate from the failures: a 404 means the server is up,
 * answering, and does not have this id. On a detail screen opened from a card
 * the server itself sent, that means the catalogue moved under the reader --
 * different words, and a different thing to go and check, from "the route is
 * missing" or "nothing answered".
 */
export type ChiTietState =
  | { kind: "dang-tai" }
  | { kind: "co-du-lieu"; place: PlaceDetail }
  | { kind: "khong-co"; url: string }
  | { kind: "chua-co-endpoint"; url: string; work: string }
  | { kind: "khong-noi-duoc"; url: string; detail: string }
  | { kind: "may-chu-loi"; url: string; status: number; detail: string }
  | { kind: "du-lieu-sai"; url: string; detail: string };

function parseReview(raw: unknown, field: string): Review {
  const r = raw as Record<string, unknown>;
  const rating = r.rating;
  if (typeof rating !== "number" || !Number.isFinite(rating)) {
    throw new Error(`${field}.rating phải là số, nhận được ${JSON.stringify(rating)}`);
  }
  if (rating < 0 || rating > 5) throw new Error(`${field}.rating ngoài 0-5: ${rating}`);
  const author = r.author;
  const body = r.body;
  if (typeof author !== "string" || author.trim() === "") {
    throw new Error(`${field}.author phải là chuỗi không rỗng`);
  }
  if (typeof body !== "string" || body.trim() === "") {
    throw new Error(`${field}.body phải là chuỗi không rỗng`);
  }
  return { author, rating, body };
}

/**
 * Turn the detail body into a `PlaceDetail`, or throw naming the field.
 *
 * `parsePlace` does the shared half, so the detail screen and the grid card can
 * never disagree about what a rating or a price band is. Only the three extra
 * fields are read here.
 */
export function parsePlaceDetail(body: unknown): PlaceDetail {
  const base = parsePlace(body, "place");
  const p = body as Record<string, unknown>;
  const description = p.description;
  if (description !== null && description !== undefined && typeof description !== "string") {
    throw new Error(`place.description phải là chuỗi hoặc null`);
  }
  const rawReviews = p.reviews;
  if (rawReviews !== undefined && !Array.isArray(rawReviews)) {
    throw new Error("place.reviews phải là mảng");
  }
  const rawActivities = p.activities;
  if (rawActivities !== undefined && rawActivities !== null && !Array.isArray(rawActivities)) {
    throw new Error("place.activities phải là mảng");
  }
  return {
    ...base,
    description: typeof description === "string" && description.trim() !== "" ? description : null,
    reviews: (rawReviews ?? []).map((r: unknown, i: number) => parseReview(r, `place.reviews[${i}]`)),
    photosAvailable: p.photos_available === true,
    activities: (rawActivities ?? []).map((a: unknown, i: number) => {
      if (typeof a !== "string" || a.trim() === "") {
        throw new Error(`place.activities[${i}] phải là chuỗi không rỗng`);
      }
      return a;
    }),
    highlights: parseHighlights(p.highlights),
  };
}

export function placeDetailUrl(base: string, placeId: string): string {
  // `encodeURIComponent` and not a template hole: this id comes off a wire
  // object and ends up in a path, and a screen that asks the server about
  // `../../etc` is a screen writing somebody else's URL.
  return `${base.replace(/\/$/, "")}/places/${encodeURIComponent(placeId)}`;
}

/**
 * Ask the server for one place in full.
 *
 * Never throws, for the same reason `fetchPlaces` never does: a rejected promise
 * on a detail screen is a blank card, and every one of these states has words a
 * person can act on.
 */
export async function fetchPlaceDetail(
  placeId: string,
  opts: { base?: string; fetchImpl?: typeof fetch } = {},
): Promise<ChiTietState> {
  const base = opts.base ?? PLACES_BASE_URL;
  const url = placeDetailUrl(base, placeId);
  const doFetch = opts.fetchImpl ?? fetch;

  let res: Response;
  try {
    res = await doFetch(url, { headers: { Accept: "application/json" } });
  } catch (e) {
    return { kind: "khong-noi-duoc", url, detail: chiTietLoi(e) };
  }

  // A 404 here is ambiguous in a way the list's is not -- it is either "no such
  // place" or "no such route" -- and the body is what separates them. The route
  // answers `{"detail": ...}`; a server without the route answers FastAPI's own
  // `{"detail":"Not Found"}`. Rather than pattern-match English, the split is
  // made on the header the API sets on every one of its own refusals.
  if (res.status === 404) {
    let known = false;
    try {
      const body = (await res.json()) as Record<string, unknown>;
      known = typeof body?.code === "string" || body?.detail !== "Not Found";
    } catch {
      /* not JSON at all: treat as a server that does not have the route */
    }
    return known
      ? { kind: "khong-co", url }
      : { kind: "chua-co-endpoint", url, work: PLACES_WORK_ITEM };
  }

  if (!res.ok) {
    let detail = `HTTP ${res.status}`;
    try {
      detail = (await res.text()).slice(0, 200) || detail;
    } catch {
      /* body already consumed or not text; the status alone still says enough */
    }
    return { kind: "may-chu-loi", url, status: res.status, detail };
  }

  try {
    return { kind: "co-du-lieu", place: parsePlaceDetail(await res.json()) };
  } catch (e) {
    return { kind: "du-lieu-sai", url, detail: chiTietLoi(e) };
  }
}

/** One line saying why the extra fields are not on screen, or null when they
 *  are. The detail screen is still fully usable in every one of these states,
 *  so this reads as a note rather than as an error banner. */
export function loiChiTiet(state: ChiTietState): string | null {
  switch (state.kind) {
    case "dang-tai":
    case "co-du-lieu":
      return null;
    case "khong-co":
      return "Chỗ này không còn trong danh mục, nên chưa có giới thiệu và đánh giá.";
    case "chua-co-endpoint":
      return "Chưa có phần giới thiệu cho chỗ này. Phần trên vẫn là dữ liệu thật từ danh sách.";
    case "khong-noi-duoc":
      return "Không kết nối được để lấy giới thiệu và đánh giá. Kiểm tra mạng rồi thử lại.";
    case "may-chu-loi":
      return `Chưa đọc được chi tiết chỗ này (lỗi ${state.status}).`;
    case "du-lieu-sai":
      return `Chi tiết địa điểm sai định dạng: ${state.detail}`;
  }
}
