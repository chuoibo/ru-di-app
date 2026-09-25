/**
 * The city stages of «Khám phá» (ADR-0037 D1, plan S4): one sketched scene per
 * destination the server lists, built from what the server's own blurb says
 * about the place -- pine hills and a coffee for Đà Lạt, the lantern street
 * for Hội An, the limestone bay for Hạ Long -- and a postcard for a city this
 * table does not know yet.
 *
 * Drawn in the sketch frame (`KHUNG`), in the three stroke weights the place
 * sketches use (far 1.7, mid 2.4, near 3.0), so each scene lifts into the same
 * three pop-up layers as `sanKhauKyHoa`. One coral fill per scene -- the sun,
 * or Hội An's one lit lantern -- is the scene's light (ADR-0037 D2). Absolute
 * M/L/C/Z only, numbers built at run time.
 */
import { NET_KY_HOA } from "./ky-hoa";
import { type Diem, type LopVe, bau, daGiac, duong, khungBo, netGay, tron } from "./net";
import { type SanKhau, type TangSanKhau, nguonSangCua, tachDoSau } from "./san-khau";

export const KHUNG_THANH_PHO = { w: 288, h: 120 } as const;
/** The table line every layer stands on. */
export const SAN_THANH_PHO = 108;

const W = KHUNG_THANH_PHO.w;
const SAN = SAN_THANH_PHO;
const XA = NET_KY_HOA.xa;
const VUA = NET_KY_HOA.vua;
const GAN = NET_KY_HOA.gan;

const net = (d: string, w: number, mau: LopVe["mau"] = "muc"): LopVe => ({ d, mau, net: w });
const to = (d: string, mau: LopVe["mau"] = "bong"): LopVe => ({ d, mau });

// --- far: land, water, light ----------------------------------------------

function matTroi(x: number, y: number, r = 9): LopVe[] {
  return [to(tron(x, y, r), "gap")];
}

/** A mountain range from the floor through its peaks, shaded, outlined far. */
function nui(dinh: readonly Diem[]): LopVe[] {
  const dau: Diem = [dinh[0][0], SAN];
  const cuoi: Diem = [dinh[dinh.length - 1][0], SAN];
  return [to(daGiac([dau, ...dinh, cuoi])), net(netGay(dinh), XA)];
}

/** One rolling hill between two floor points, `cao` above the floor. */
function doi(x0: number, x1: number, cao: number, mau: LopVe["mau"] = "bong", w = XA): LopVe[] {
  const g = (x0 + x1) / 2;
  const dinh = SAN - cao;
  const vien = duong("M", x0, SAN, "C", x0 + (g - x0) * 0.5, dinh, g - (g - x0) * 0.4, dinh, g, dinh, "C", g + (x1 - g) * 0.4, dinh, x1 - (x1 - g) * 0.5, dinh, x1, SAN);
  return [to(`${vien} Z`, mau), net(vien, w)];
}

/** A limestone karst: a tall rounded tower rising from the water. */
function karst(x: number, rong: number, cao: number, w = XA): LopVe[] {
  const d = SAN - cao;
  const vien = duong("M", x, SAN, "C", x - 2, SAN - cao * 0.5, x + rong * 0.1, d, x + rong * 0.45, d, "C", x + rong * 0.85, d, x + rong + 2, SAN - cao * 0.6, x + rong, SAN);
  return [to(`${vien} Z`), net(vien, w)];
}

/** Water: the horizon, then two rows of short strokes. */
function nuoc(y: number, x0 = 6, x1 = W - 6): LopVe[] {
  const lop: LopVe[] = [net(netGay([[x0, y], [x1, y]]), XA)];
  for (let hang = 0; hang < 2; hang += 1) {
    const yy = y + 6 + hang * 6;
    for (let x = x0 + 10 + hang * 14; x + 12 <= x1; x += 34) lop.push(net(netGay([[x, yy], [x + 12, yy]]), XA, "bong"));
  }
  return lop;
}

/** Mist or cloud: a long soft wave. */
function may(x: number, y: number, rong: number): LopVe[] {
  const b = rong / 4;
  return [net(duong("M", x, y, "C", x + b, y - 4, x + b * 2, y + 4, x + b * 2, y, "C", x + b * 2, y - 4, x + b * 3, y + 4, x + rong, y), XA, "bong")];
}

// --- mid: landmarks --------------------------------------------------------

function thong(x: number, cao: number, w = VUA): LopVe[] {
  const day = SAN;
  const dinh = day - cao;
  const rong = cao * 0.42;
  const than: Diem[] = [
    [x, dinh],
    [x + rong * 0.35, dinh + cao * 0.35],
    [x + rong * 0.2, dinh + cao * 0.35],
    [x + rong * 0.5, dinh + cao * 0.75],
    [x - rong * 0.5, dinh + cao * 0.75],
    [x - rong * 0.2, dinh + cao * 0.35],
    [x - rong * 0.35, dinh + cao * 0.35],
  ];
  return [to(daGiac(than), "giay"), net(`${netGay([...than, than[0]])}`, w), net(netGay([[x, dinh + cao * 0.75], [x, day]]), w)];
}

/** A row of shophouses with pitched roofs and a window each. */
function nhaPho(x0: number, cao: readonly number[], rong = 22, w = VUA): LopVe[] {
  const lop: LopVe[] = [];
  cao.forEach((h, i) => {
    const x = x0 + i * rong;
    const tuong: Diem[] = [[x, SAN], [x, SAN - h], [x + rong / 2, SAN - h - 8], [x + rong, SAN - h], [x + rong, SAN]];
    lop.push(to(daGiac(tuong), "giay"), net(netGay(tuong), w));
    lop.push(net(khungBo(x + rong * 0.3, SAN - h + 6, rong * 0.4, 8, 1), XA));
  });
  return lop;
}

/** Towers of a skyline; the tallest wears a notch (a helipad, a crown). */
function toaNha(x0: number, cot: readonly (readonly [number, number])[], w = VUA): LopVe[] {
  const lop: LopVe[] = [];
  let x = x0;
  for (const [rong, cao] of cot) {
    const khung: Diem[] = [[x, SAN], [x, SAN - cao], [x + rong, SAN - cao], [x + rong, SAN]];
    lop.push(to(daGiac(khung), "giay"), net(netGay(khung), w));
    for (let y = SAN - cao + 8; y < SAN - 6; y += 9) lop.push(net(netGay([[x + 4, y], [x + rong - 4, y]]), XA, "bong"));
    x += rong + 3;
  }
  return lop;
}

function denLong(x: number, y: number, mau: LopVe["mau"] = "mo"): LopVe[] {
  return [net(netGay([[x, y - 12], [x, y - 7]]), XA), to(bau(x, y, 5, 6.5), mau), net(bau(x, y, 5, 6.5), VUA)];
}

function thuyen(x: number, y: number, rong: number, buom = true, w = VUA): LopVe[] {
  const than: Diem[] = [[x, y], [x + rong, y], [x + rong * 0.82, y + 7], [x + rong * 0.18, y + 7]];
  const lop: LopVe[] = [to(daGiac(than), "giay"), net(netGay([...than, than[0]]), w)];
  if (buom) {
    const cot = x + rong * 0.45;
    const buomVai: Diem[] = [[cot, y - 2], [cot, y - rong * 0.7], [cot + rong * 0.35, y - 4]];
    lop.push(to(daGiac(buomVai), "mo"), net(netGay([...buomVai, buomVai[0]]), w));
  }
  return lop;
}

/** The dragon bridge: a long wave of arches over the river. */
function cauRong(x0: number, x1: number, y: number): LopVe[] {
  const buoc = (x1 - x0) / 4;
  const phan: (string | number)[] = ["M", x0, y];
  for (let i = 0; i < 4; i += 1) {
    const a = x0 + i * buoc;
    phan.push("C", a + buoc * 0.25, y - 14, a + buoc * 0.75, y - 14, a + buoc, y);
  }
  const lop: LopVe[] = [net(duong(...phan), VUA), net(netGay([[x0, y + 4], [x1, y + 4]]), VUA)];
  for (let i = 0; i <= 4; i += 1) lop.push(net(netGay([[x0 + i * buoc, y + 4], [x0 + i * buoc, SAN]]), XA));
  return lop;
}

/** A Cham tower: three stepped tiers and a tip. */
function thapCham(x: number, cao: number, w = VUA): LopVe[] {
  const b = cao / 4;
  const vien: Diem[] = [
    [x - 12, SAN], [x - 12, SAN - b * 2], [x - 9, SAN - b * 2], [x - 9, SAN - b * 3], [x - 5, SAN - b * 3], [x, SAN - cao],
    [x + 5, SAN - b * 3], [x + 9, SAN - b * 3], [x + 9, SAN - b * 2], [x + 12, SAN - b * 2], [x + 12, SAN],
  ];
  return [to(daGiac(vien), "giay"), net(netGay(vien), w), net(khungBo(x - 4, SAN - b * 1.4, 8, b * 1.4, 3), XA)];
}

/** A citadel gate: a long wall, an arch, and a two-tier roof. */
function congThanh(x: number, rong: number, cao: number, w = VUA): LopVe[] {
  const t = SAN - cao;
  const tuong: Diem[] = [[x, SAN], [x, t], [x + rong, t], [x + rong, SAN]];
  const mai1: Diem[] = [[x + rong * 0.18, t], [x + rong * 0.28, t - 10], [x + rong * 0.72, t - 10], [x + rong * 0.82, t]];
  const mai2: Diem[] = [[x + rong * 0.32, t - 10], [x + rong * 0.4, t - 18], [x + rong * 0.6, t - 18], [x + rong * 0.68, t - 10]];
  const g = x + rong / 2;
  const vom = duong("M", g - 9, SAN, "L", g - 9, SAN - cao * 0.45, "C", g - 9, SAN - cao * 0.72, g + 9, SAN - cao * 0.72, g + 9, SAN - cao * 0.45, "L", g + 9, SAN);
  return [
    to(daGiac(tuong), "giay"), net(netGay(tuong), w),
    to(daGiac(mai1), "bong"), net(netGay(mai1), w),
    to(daGiac(mai2), "bong"), net(netGay(mai2), w),
    net(vom, w),
  ];
}

/** A statue with open arms on a hilltop (Vũng Tàu). */
function tuong(x: number, y: number): LopVe[] {
  return [
    net(netGay([[x, y], [x, y + 16]]), VUA),
    net(netGay([[x - 9, y + 4], [x + 9, y + 4]]), VUA),
    to(tron(x, y - 3, 2.6), "giay"), net(tron(x, y - 3, 2.6), XA),
    net(netGay([[x - 5, y + 18], [x + 5, y + 18]]), VUA),
  ];
}

/** Terraces on a slope: stacked curved rules. */
function ruongBacThang(x0: number, x1: number, y0: number, soBac: number): LopVe[] {
  const lop: LopVe[] = [];
  for (let i = 0; i < soBac; i += 1) {
    const y = y0 + i * 7;
    const lui = i * 6;
    lop.push(net(duong("M", x0 + lui, y, "C", x0 + lui + (x1 - x0) * 0.3, y - 5, x1 - lui - (x1 - x0) * 0.3, y + 5, x1 - lui, y), XA));
  }
  return lop;
}

// --- near: the ground things ---------------------------------------------

function cayDua(x: number, cao: number): LopVe[] {
  const dinh: Diem = [x + 8, SAN - cao];
  const lop: LopVe[] = [net(duong("M", x, SAN, "C", x + 1, SAN - cao * 0.4, x + 6, SAN - cao * 0.8, dinh[0], dinh[1]), GAN)];
  for (const [dx, dy] of [[-14, 6], [14, 4], [-10, -4], [12, -6], [0, -9]] as const) {
    lop.push(net(duong("M", dinh[0], dinh[1], "C", dinh[0] + dx * 0.4, dinh[1] + dy * 0.2 - 4, dinh[0] + dx * 0.8, dinh[1] + dy * 0.6, dinh[0] + dx, dinh[1] + dy), VUA));
  }
  return lop;
}

function lyCaPhe(x: number): LopVe[] {
  const y = SAN - 16;
  const ly: Diem[] = [[x, y], [x + 16, y], [x + 14, SAN], [x + 2, SAN]];
  return [
    to(daGiac(ly), "giay"), net(netGay([...ly, ly[0]]), GAN),
    net(duong("M", x + 16, y + 4, "C", x + 22, y + 4, x + 22, y + 11, x + 15, y + 11), GAN),
    net(duong("M", x + 5, y - 4, "C", x + 3, y - 8, x + 8, y - 10, x + 6, y - 14), XA),
    net(duong("M", x + 11, y - 4, "C", x + 9, y - 8, x + 14, y - 10, x + 12, y - 14), XA),
  ];
}

function doiCat(x0: number, x1: number, cao: number): LopVe[] {
  const g = x0 + (x1 - x0) * 0.62;
  const vien = duong("M", x0, SAN, "C", x0 + (g - x0) * 0.5, SAN - cao * 0.2, g - 12, SAN - cao, g, SAN - cao, "C", g + 10, SAN - cao, x1 - (x1 - g) * 0.4, SAN - cao * 0.3, x1, SAN);
  return [to(`${vien} Z`, "giay"), net(vien, GAN), net(duong("M", g, SAN - cao, "C", g - 4, SAN - cao * 0.5, g + 6, SAN - cao * 0.3, g + 2, SAN), XA, "bong")];
}

function mat(): LopVe[] {
  return [net(netGay([[4, SAN], [W - 4, SAN]]), GAN)];
}

// --- the cities ----------------------------------------------------------

interface ThanhPho {
  ten: string;
  /** What the sketch shows, in the words of the server's blurb. */
  canh: string;
  hinh: () => LopVe[];
}

const THANH_PHO: Readonly<Record<string, ThanhPho>> = {
  "d-da-lat": {
    ten: "Đà Lạt",
    canh: "đồi thông trong sương và một ly cà phê",
    hinh: () => [...matTroi(236, 30), ...doi(0, 170, 44), ...doi(110, 288, 56), ...may(24, 40, 90), ...may(150, 30, 80), ...thong(40, 52), ...thong(64, 38), ...thong(200, 58), ...thong(226, 42), ...lyCaPhe(128), ...mat()],
  },
  "d-tphcm": {
    ten: "TP. Hồ Chí Minh",
    canh: "những toà nhà cao, sân thượng và cà phê vợt",
    hinh: () => [...matTroi(40, 30), ...toaNha(70, [[26, 50], [22, 70], [30, 92], [24, 60], [20, 44]]), ...toaNha(214, [[22, 38], [28, 54]]), ...lyCaPhe(22), ...mat()],
  },
  "d-ha-noi": {
    ten: "Hà Nội",
    canh: "phố cổ mái ngói bên hồ",
    hinh: () => [...matTroi(250, 28), ...nhaPho(18, [38, 50, 32, 44, 36]), ...cauRong(150, 270, 90), ...nuoc(96, 140, 282), ...mat()],
  },
  "d-da-nang": {
    ten: "Đà Nẵng",
    canh: "cầu Rồng vắt qua sông, biển xa",
    hinh: () => [...matTroi(230, 32), ...nuoc(70), ...doi(0, 110, 30), ...cauRong(40, 270, 84), ...thuyen(200, 96, 34, false), ...mat()],
  },
  "d-hoi-an": {
    ten: "Hội An",
    canh: "phố đèn lồng bên sông Hoài",
    hinh: () => [...nhaPho(20, [46, 40, 52, 44, 48, 40]), ...denLong(44, 36, "gap"), ...denLong(88, 34), ...denLong(132, 30), ...denLong(176, 38), ...nuoc(98, 150, 282), ...thuyen(210, 94, 40), ...mat()],
  },
  "d-nha-trang": {
    ten: "Nha Trang",
    canh: "biển dài và hàng dừa",
    hinh: () => [...matTroi(150, 30), ...doi(180, 288, 36), ...nuoc(74, 6, 200), ...thuyen(70, 84, 30), ...cayDua(28, 62), ...cayDua(236, 56), ...mat()],
  },
  "d-hue": {
    ten: "Huế",
    canh: "cổng kinh thành bên sông Hương",
    hinh: () => [...matTroi(240, 30), ...doi(180, 288, 30), ...congThanh(36, 128, 50), ...nuoc(98, 176, 282), ...thuyen(206, 92, 42), ...mat()],
  },
  "d-sa-pa": {
    ten: "Sa Pa",
    canh: "núi và ruộng bậc thang trong sương",
    hinh: () => [...matTroi(246, 26), ...nui([[0, 70], [60, 30], [120, 58], [180, 20], [240, 50], [288, 36]]), ...may(30, 50, 110), ...ruongBacThang(20, 268, 74, 4), ...mat()],
  },
  "d-phu-quoc": {
    ten: "Phú Quốc",
    canh: "đảo, biển và thuyền đánh cá",
    hinh: () => [...matTroi(210, 30), ...doi(120, 250, 26), ...nuoc(82), ...thuyen(40, 90, 36), ...thuyen(150, 94, 28, false), ...cayDua(250, 64), ...mat()],
  },
  "d-vung-tau": {
    ten: "Vũng Tàu",
    canh: "tượng trên núi nhìn ra biển gần",
    hinh: () => [...matTroi(236, 32), ...doi(10, 170, 62), ...tuong(90, 28), ...nuoc(88, 170, 282), ...thuyen(210, 94, 34), ...mat()],
  },
  "d-can-tho": {
    ten: "Cần Thơ",
    canh: "chợ nổi trên sông",
    hinh: () => [...matTroi(48, 30), ...doi(150, 288, 22), ...nuoc(78), ...thuyen(30, 88, 44), ...thuyen(100, 92, 40, false), ...thuyen(172, 86, 46), ...mat()],
  },
  "d-ha-long": {
    ten: "Hạ Long",
    canh: "vịnh đá vôi và một chiếc du thuyền",
    hinh: () => [...matTroi(150, 26), ...nuoc(96, 6, 282), ...karst(12, 40, 62), ...karst(62, 26, 44), ...karst(196, 36, 70), ...karst(240, 30, 48), ...thuyen(104, 90, 56), ...mat()],
  },
  "d-quy-nhon": {
    ten: "Quy Nhơn",
    canh: "tháp Chăm bên biển vắng",
    hinh: () => [...matTroi(232, 30), ...doi(0, 120, 34), ...thapCham(60, 58), ...nuoc(84, 130, 282), ...thuyen(196, 92, 32, false), ...mat()],
  },
  "d-ninh-binh": {
    ten: "Ninh Bình",
    canh: "núi đá, ruộng lúa và thuyền trên sông",
    hinh: () => [...matTroi(144, 24), ...ruongBacThang(40, 250, 84, 2), ...karst(8, 46, 70), ...karst(58, 30, 50), ...karst(190, 40, 66), ...karst(236, 44, 54), ...thuyen(116, 96, 40, false), ...mat()],
  },
  "d-mui-ne": {
    ten: "Mũi Né",
    canh: "đồi cát, biển gió và hàng dừa",
    hinh: () => [...matTroi(70, 30), ...nuoc(72, 6, 160), ...doiCat(100, 288, 48), ...cayDua(30, 58), ...mat()],
  },
};

/** A city this table does not have yet: a postcard of hills and a road. */
const BUU_THIEP: ThanhPho = {
  ten: "",
  canh: "đồi, con đường và mặt trời",
  hinh: () => [
    ...matTroi(220, 32),
    ...doi(0, 160, 46),
    ...doi(120, 288, 34),
    net(duong("M", 120, SAN, "C", 128, 96, 150, 90, 170, 74), VUA),
    net(duong("M", 150, SAN, "C", 152, 98, 166, 92, 178, 74), VUA),
    ...mat(),
  ],
};

/** The destination ids this table draws; any other id gets the postcard. */
export const THANH_PHO_IDS: readonly string[] = Object.keys(THANH_PHO);

/** The scene of a destination, as layers in the city frame. */
export function hinhThanhPho(id: string | null | undefined): LopVe[] {
  const tp = THANH_PHO[id ?? ""] ?? BUU_THIEP;
  return tp.hinh();
}

/** One sentence for a screen reader: «Ký hoạ Đà Lạt: …». */
export function moTaThanhPho(id: string | null | undefined, tenMayChu?: string): string {
  const tp = THANH_PHO[id ?? ""];
  if (tp) return `Ký hoạ ${tp.ten}: ${tp.canh}`;
  return `Ký hoạ ${tenMayChu?.trim() || "một thành phố"}: ${BUU_THIEP.canh}`;
}

/** The city as a pop-up stage: far land and light, then landmarks, then the ground things. */
export function sanKhauThanhPho(id: string | null | undefined, tenMayChu?: string): SanKhau {
  const lop = hinhThanhPho(id);
  const [xa, vua, gan] = tachDoSau(lop, NET_KY_HOA);
  const tang: TangSanKhau[] = [];
  if (xa.length > 0) tang.push({ id: "xa", sau: 0, nep: SAN, dung: true, cao: 2, lop: xa });
  if (vua.length > 0) tang.push({ id: "vua", sau: 1, nep: SAN, dung: true, cao: 2, lop: vua });
  if (gan.length > 0) tang.push({ id: "gan", sau: 2, nep: SAN, dung: true, cao: 2, lop: gan });
  return {
    id: `thanh-pho:${THANH_PHO[id ?? ""]?.ten ?? "buu-thiep"}`,
    khung: { ...KHUNG_THANH_PHO },
    nen: [],
    tang,
    nguonSang: nguonSangCua(lop),
    moTa: moTaThanhPho(id, tenMayChu),
  };
}
