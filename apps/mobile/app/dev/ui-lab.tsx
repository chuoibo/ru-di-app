import { Redirect, useRouter } from "expo-router";
import { useState } from "react";
import type { ImageSource } from "expo-image";
import { Text, View } from "react-native";
import { CUA_FIXTURE_DEV } from "../../src/rudi/cua-fixture";
import { demoAssets } from "../../src/rudi/fixtures";
import { typography, useRudiTheme } from "../../src/rudi/theme";
import { AlbumAnh, type AnhAlbumHienThi } from "../../src/rudi/screens/ky-niem/AlbumAnh";
import { PlaceCompare, PlaceLead, PlaceRow, type DiaDiemHienThi } from "../../src/rudi/screens/explore/HangDiaDiem";
import { HangChang } from "../../src/rudi/screens/keo/HangChang";
import { KhaySticker } from "../../src/rudi/screens/chat/KhaySticker";
import { Sticker } from "../../src/rudi/ui/stickers/Sticker";
import { STICKER_IDS, nhanSticker } from "../../src/rudi/chat/sticker";
import { TIEN_TO_MINH_HOA, anhDanhMuc, type AnhCoGhiCong } from "../../src/rudi/ui/ghi-cong";
import { danhDauLoi, themVaoHang } from "../../src/rudi/chat/hang-cho";
import { HangChoGui } from "../../src/rudi/screens/chat/GroupChatLive";
import { Chip, Heading, Inline, RudiButton, RudiScreen, SearchField, SectionHeader, TopBar } from "../../src/rudi/ui";
import { ThuRenderer } from "../../src/rudi/ui/ThuRenderer";
import { ThuSanKhau } from "../../src/rudi/ui/ThuSanKhau";
import { ThuNepDien } from "../../src/rudi/ui/ThuNepDien";
import { CANH_IDS, moTaCanh } from "../../src/rudi/art/canh";
import { Canh } from "../../src/rudi/ui/art/Canh";
import { KyHoa } from "../../src/rudi/ui/art/KyHoa";
import { Nep } from "../../src/rudi/ui/art/Nep";
import { ThuGapBa } from "../../src/rudi/ui/art/Motif";
import { ToGiay, VetGap } from "../../src/rudi/ui/ToGiay";
import { moTaKyHoa } from "../../src/rudi/art/ky-hoa";
import { EmptyState } from "../../src/rudi/ui/EmptyState";
import { ReorderList } from "../../src/rudi/ui/ReorderList";
import { PhotoViewer } from "../../src/rudi/ui/PhotoViewer";

/**
 * A source that cannot resolve, so the `onError` branch of every image can be
 * seen on the device. Loopback on a closed port: nothing leaves the machine.
 */
// One row per stage, tags as the fixture writes them (with diacritics), plus the unknown category.
const KY_HOA_MAU: readonly { loai: string; tags: readonly string[] }[] = [
  { loai: "quan-an-local", tags: ["View đẹp", "Chill", "Nhóm đông"] },
  { loai: "cafe", tags: ["Nhẹ nhàng", "Cà phê", "Ngoài trời"] },
  { loai: "vui-choi", tags: ["Săn mây", "Ngoài trời"] },
  { loai: "di-choi-dem", tags: ["Món local", "Đi đêm", "Nhộn nhịp"] },
  { loai: "khac", tags: [] },
];

// The two-person notebook's sheet in three states. Exactly ONE is `dan`: the
// coral corner is the one leading mark on a surface (spec §16.4), and a blind
// read of a board with two marks could not say which sheet was the thing to
// do. The reason line sits BELOW the sheet, outside it, so the three rows stay
// equal and the two creases fall at a third and two thirds -- with the reason
// inside, the third row grew and the creases read as table dividers. No
// «khoa» state here: the locked sheet's material is lát 3 and has not been
// designed; a lab must not show a state that does not exist yet.
const TO_GIAY_MAU: readonly { id: string; dan: boolean; hang: readonly [string, string, string]; lyDo: string }[] = [
  { id: "nhap", dan: true, hang: ["18:30  Ăn tối, một quán chưa đi", "20:00  Đi bộ, rồi chè", "Tuần này bạn mở lời"], lyDo: "Vì: ba tuần liền hai bạn ăn ở cùng một khu." },
  { id: "da-gui", dan: false, hang: ["18:30  Ăn tối, một quán chưa đi", "20:00  Đi bộ, rồi chè", "Đã gửi, chờ trả lời"], lyDo: "Người kia chưa xem." },
  { id: "chot", dan: false, hang: ["18:30  Ăn tối, một quán chưa đi", "20:00  Đi bộ, rồi chè", "Đã chốt, thứ Bảy"], lyDo: "Cả hai đã ừ cùng một phiên bản." },
];
// Three acts of the two-person notebook, page beside pocket sheet, at both readings.
const NEP_MANH_MAU = ["dua-giay", "up-xuong", "gap-lai"] as const;

const ANH_HONG = { uri: "http://127.0.0.1:1/khong-co-anh.jpg" };

type CaAnh = "khong" | "co" | "hong" | "lech";
const CA_ANH: { id: CaAnh; nhan: string }[] = [
  { id: "khong", nhan: "Không ảnh" },
  { id: "co", nhan: "Có ảnh" },
  { id: "hong", nhan: "Ảnh hỏng" },
  { id: "lech", nhan: "Cặp lệch" },
];

/** A made-up credit, so the words a frame prints can be seen without claiming a real author. */
const GHI_CONG_MAU = { prefix: TIEN_TO_MINH_HOA, author: "Tác giả tổng hợp", license: "Giấy phép tổng hợp" };

/**
 * The three states of one logical send, built through the SAME pure module the
 * chat screen uses, so what the board photographs is the real machine rather
 * than a hand-drawn copy of it (F32).
 */
const CA_HANG_CHO = (() => {
  const goc = {
    attempt: { key: "lab-1", at: 1 },
    kind: "sticker" as const,
    than: "cho-ti",
    phuDe: null,
    traLoi: null,
    trangThai: "dang-gui" as const,
    loi: null,
    thuLaiDuoc: true,
    luc: "2026-09-09T00:00:00Z",
  };
  const dangGui = themVaoHang([], goc)[0];
  const hong = danhDauLoi(themVaoHang([], goc), "lab-1", "Không nối được máy chủ. Kiểm tra mạng rồi thử lại.", null)[0];
  const vinhVien = danhDauLoi(
    themVaoHang([], { ...goc, than: "di-thoi" }),
    "lab-1",
    "Sticker này bản app chưa có.",
    "sticker_unknown",
  )[0];
  return [
    { nhan: "Đang gửi", tin: dangGui },
    { nhan: "Hỏng, gửi lại được", tin: hong },
    { nhan: "Hỏng vĩnh viễn, không mời thử lại", tin: vinhVien },
  ];
})();

type CaChang = "ghi-cong" | "hong" | "khong";
const CA_CHANG: { id: CaChang; nhan: string }[] = [
  { id: "ghi-cong", nhan: "Chặng có ghi công" },
  { id: "hong", nhan: "Chặng ảnh hỏng" },
  { id: "khong", nhan: "Chặng không ảnh" },
];

/**
 * Two stops of an invented route for the renderer the itinerary and the
 * timeline ship (`HangChang.anh`): the credit must be a line of the stop, and a
 * picture that fails must give way to the category's object in the frame.
 */
function anhChangMau(ca: CaChang, that: ImageSource): AnhCoGhiCong | null {
  if (ca === "khong") return null;
  return anhDanhMuc(ca === "hong" ? ANH_HONG : that, GHI_CONG_MAU);
}

type CaAlbum = "0" | "1" | "2-ngay" | "le" | "ngay-la" | "caption-dai" | "anh-hong";
const CA_ALBUM: { id: CaAlbum; nhan: string }[] = [
  { id: "0", nhan: "0 ảnh" },
  { id: "1", nhan: "1 ảnh" },
  { id: "2-ngay", nhan: "2 ngày, lead lẻ" },
  { id: "le", nhan: "4 ảnh, lẻ" },
  { id: "ngay-la", nhan: "ngày không đọc được" },
  { id: "caption-dai", nhan: "caption dài" },
  { id: "anh-hong", nhan: "ảnh hỏng" },
];
const ANH_LAB = [demoAssets.dalatFriends, demoAssets.cafe, demoAssets.road, demoAssets.friends];

/**
 * Invented album states for the renderer that ships (`AlbumAnh`), so the day
 * labels, the odd wide print and the lead's caption are measured on native
 * without a server. The two-day case is the one the 08/09 review caught: the
 * lead is the only photograph of the first day.
 */
function anhAlbumMau(ca: CaAlbum): AnhAlbumHienThi[] {
  const mk = (i: number, created_at: string, caption?: string): AnhAlbumHienThi => ({
    id: `lab-${i}`,
    source: ANH_LAB[i % ANH_LAB.length],
    caption: caption ?? `Ảnh tổng hợp ${i + 1}`,
    created_at,
  });
  switch (ca) {
    case "0":
      return [];
    case "1":
      return [mk(0, "2026-10-17T10:00:00+07:00")];
    case "2-ngay":
      return [mk(0, "2026-10-17T23:59:59+07:00"), mk(1, "2026-10-18T00:00:00+07:00")];
    case "le":
      return [mk(0, "2026-10-17T09:00:00+07:00"), mk(1, "2026-10-17T10:00:00+07:00"), mk(2, "2026-10-17T11:00:00+07:00"), mk(3, "2026-10-18T08:00:00+07:00")];
    case "ngay-la":
      return [mk(0, "2026-10-17T10:00:00+07:00"), mk(1, "khong-phai-ngay"), mk(2, "2026-10-18T10:00:00+07:00")];
    case "anh-hong":
      return [
        { id: "lab-hong-0", source: ANH_HONG, caption: "Ảnh dẫn không tải được", created_at: "2026-10-17T10:00:00+07:00" },
        { id: "lab-hong-1", source: ANH_HONG, caption: "Ảnh phụ không tải được", created_at: "2026-10-17T11:00:00+07:00" },
        mk(2, "2026-10-18T09:00:00+07:00"),
      ];
    case "caption-dai":
      return [
        mk(0, "2026-10-17T10:00:00+07:00", "Một câu chú thích rất dài để kiểm tra dòng chữ dưới ảnh in khi cỡ chữ hệ thống tăng lên 2.0 và tên tiếng Việt nhiều dấu"),
        mk(1, "2026-10-17T11:00:00+07:00"),
      ];
  }
}

/** Invented places for the renderers Khám phá ships: no picture, a picture, a picture that fails. */
function diaDiemMau(caAnh: CaAnh, tenDai: boolean): DiaDiemHienThi[] {
  // «Cặp lệch»: only the second candidate has a picture, which is the case the
  // finish review asked to see -- the pair must still compare on one axis.
  const anh = (that: ImageSource, thuTu: number): ImageSource | null => {
    if (caAnh === "khong") return null;
    if (caAnh === "hong") return ANH_HONG;
    if (caAnh === "lech") return thuTu === 2 ? that : null;
    return that;
  };
  // A picture never travels without its credit: the frames take both in one object.
  const anhLab = (that: ImageSource, thuTu: number): AnhCoGhiCong | null => {
    const p = anh(that, thuTu);
    return p ? anhDanhMuc(p, GHI_CONG_MAU) : null;
  };
  const ten = (ngan: string, dai: string) => (tenDai ? dai : ngan);
  const facts = (sao: string, xa: string, gia: string): DiaDiemHienThi["facts"] => [
    { icon: "star", text: sao },
    { icon: "navigate-outline", text: xa },
    { icon: "wallet-outline", text: gia },
  ];
  return [
    { id: "lab-a", name: ten("Bánh căn Lệ", "Tiệm Bánh Căn Cô Lệ Đường Nguyễn Văn Trỗi"), sub: "Một dòng mô tả tổng hợp, không phải quán thật", facts: facts("4.6 (188)", "900 m", "40K - 80K/người"), glyph: "location-outline", loai: "quan-an-local", anh: anhLab(demoAssets.cafe, 0), badge: "Hợp gu", lyDo: "Hai gu tổng hợp" },
    { id: "lab-b", name: ten("Lẩu gà lá é", "Lẩu Gà Lá É Gốc Đường Ba Tháng Hai Phường Một"), sub: "Đủ chỗ nhóm tám, tổng hợp", facts: facts("4.7 (214)", "1,6 km", "180K - 260K/người"), glyph: "location-outline", loai: "quan-an-local", anh: anhLab(demoAssets.road, 1), badge: null },
    { id: "lab-c", name: ten("Still Cafe", "Still Cafe Đà Lạt Chi Nhánh Đường Trần Hưng Đạo"), sub: "Cà phê view đồi, tổng hợp", facts: facts("4.7 (512)", "1,8 km", "120K - 200K/người"), glyph: "location-outline", loai: "cafe", anh: anhLab(demoAssets.cafe, 2), badge: "Hợp gu" },
    { id: "lab-d", name: ten("Tiệm trà Sương", "Tiệm Trà Sương Sớm Trên Đồi Thông Phường Mười"), sub: "Trà thảo mộc, tổng hợp", facts: facts("4.5 (143)", "2,1 km", "80K - 140K/người"), glyph: "location-outline", loai: "cafe", anh: anhLab(demoAssets.road, 3), badge: null },
  ];
}

/** Synthetic native probe: gestures, and the album and explore renderers under invented states. Never in a production build. */
export default function UiLab() {
  const { colors, radius } = useRudiTheme();
  const router = useRouter();
  const [items, setItems] = useState([
    { id: "a", label: "Chặng A · 18:00" },
    { id: "b", label: "Chặng B · 08:00" },
    { id: "c", label: "Chặng C · tên dài để kiểm tra dòng chữ khi tăng kích cỡ hệ thống" },
  ]);
  const [dragging, setDragging] = useState(false);
  const [viewer, setViewer] = useState(false);
  const [caAlbum, setCaAlbum] = useState<CaAlbum>("2-ngay");
  const [xemAlbum, setXemAlbum] = useState<number | null>(null);
  const [caAnh, setCaAnh] = useState<CaAnh>("khong");
  const [caChang, setCaChang] = useState<CaChang>("ghi-cong");
  const [khaySticker, setKhaySticker] = useState(false);
  const [stickerChon, setStickerChon] = useState<string>("cho-ti");
  const [tenDai, setTenDai] = useState(false);
  const [daLuu, setDaLuu] = useState<string[]>([]);
  const [lanThu, setLanThu] = useState(0);
  const anhAlbum = anhAlbumMau(caAlbum);
  const [dan, ...conLai] = diaDiemMau(caAnh, tenDai);
  const luu = (id: string) => setDaLuu((ds) => (ds.includes(id) ? ds.filter((x) => x !== id) : [...ds, id]));
  if (!CUA_FIXTURE_DEV) return <Redirect href="/welcome" />;
  // The tray is a sheet over the whole screen, so it goes in the screen's
  // overlay slot; inside the scroll content it would rise below the viewport
  // (measured 08/09: the board saw only its top edge).
  return <RudiScreen overlay={<KhaySticker onChon={(id) => { setStickerChon(id); setKhaySticker(false); }} onClose={() => setKhaySticker(false)} open={khaySticker} />} scrollEnabled={!dragging}>
    <TopBar title="Thử tương tác native" />
    <Heading title="Dữ liệu tổng hợp" subtitle="Chỉ đo gesture và hiển thị. Không phải dữ liệu nhóm hay bằng chứng API live." />
    <SectionHeader action="Chạy lại" onAction={() => setLanThu((n) => n + 1)} title="Sân khấu giấy · renderer Skia hay SVG" />
    <Text style={{ ...typography.note, color: colors.inkFaint }}>
      {"Nếp dựng lên quanh vạch chân (3D) và đường mực tự vẽ. Máy có Skia vẽ bằng Skia; dev client cũ hoặc trình duyệt không WebGL vẽ bằng SVG, cùng chuyển động."}
    </Text>
    <ThuRenderer lan={lanThu} />
    <SectionHeader action="Mở màn thử" onAction={() => router.push("/dev/san-khau")} title="Sân khấu giấy · bật dựng, nghiêng, tab kéo" />
    <ThuSanKhau lan={lanThu} />
    <SectionHeader title="Nếp con rối giấy · chín tiết mục, tám khoảnh khắc" />
    <ThuNepDien />
    <SectionHeader title="Album · renderer live, dữ liệu tổng hợp" />
    <Inline gap={8} wrap>
      {CA_ALBUM.map((ca) => <Chip key={ca.id} label={ca.nhan} onPress={() => setCaAlbum(ca.id)} selected={caAlbum === ca.id} />)}
    </Inline>
    {anhAlbum.length === 0 ? (
      <EmptyState body="Thả khoảnh khắc lên tường nhóm trong những ngày này là ảnh về đây." illustration={<Canh id="chua-co-anh" width={168} />} kind="first-use" layout="inline" title="Chưa có khoảnh khắc" />
    ) : (
      <AlbumAnh onMo={setXemAlbum} photos={anhAlbum} tiLeDan={4 / 3} />
    )}
    {xemAlbum !== null ? <PhotoViewer initialIndex={xemAlbum} onClose={() => setXemAlbum(null)} photos={anhAlbum.map((a) => ({ id: a.id, source: a.source, caption: a.caption }))} title="Album tổng hợp" /> : null}

    <SectionHeader title="Cảnh rỗng · A/B có Nếp và tắt Nếp" />
    <Text style={{ ...typography.note, color: colors.inkFaint }}>
      {"Cùng máy, cùng dữ liệu, chỉ khác `nep`. Trái: mã đang mặc định. Phải: nep={false}."}
    </Text>
    {CANH_IDS.map((id) => (
      <View key={id} style={{ gap: 4 }}>
        <Text style={{ ...typography.caption, color: colors.inkSoft }}>{moTaCanh(id)}</Text>
        <View style={{ alignItems: "flex-end", flexDirection: "row", gap: 16 }}>
          <Canh id={id} testID={`lab-canh-${id}-co`} width={150} />
          <Canh id={id} nep={false} testID={`lab-canh-${id}-khong`} width={150} />
        </View>
      </View>
    ))}

    <SectionHeader title="Ký hoạ · sân khấu theo loại, đạo cụ theo tag, hai khung đọc" />
    <Text style={{ ...typography.note, color: colors.inkFaint }}>
      {"Tờ ký hoạ của nơi chưa có ảnh: trái 3:1 (chữ 1.0), phải 4:1 (chữ lớn). Cùng một hình, khác khung cắt."}
    </Text>
    {KY_HOA_MAU.map(({ loai, tags }) => (
      <View key={loai} style={{ gap: 4 }}>
        <Text style={{ ...typography.caption, color: colors.inkSoft }}>{`${moTaKyHoa(loai, tags)} · gọn: ${moTaKyHoa(loai, tags, { gon: true })}`}</Text>
        <KyHoa gon={false} loai={loai} tags={tags} testID={`lab-ky-hoa-${loai}-day`} />
        <KyHoa gon loai={loai} tags={tags} testID={`lab-ky-hoa-${loai}-gon`} />
      </View>
    ))}

    <SectionHeader title="Tờ giấy · thư gấp ba, mép lineStrong, vết gấp paperShade" />
    <Text style={{ ...typography.note, color: colors.inkFaint }}>
      {"Tờ của sổ hai người: ba hàng, hai vết gấp ngang, mép lineStrong. Góc coral chỉ ở tờ đang là việc cần làm."}
    </Text>
    {TO_GIAY_MAU.map(({ id, dan, hang, lyDo }) => (
      <View key={id} style={{ gap: 6 }}>
        <ToGiay dan={dan} testID={`lab-to-giay-${id}`}>
          <Text style={{ ...typography.body, color: colors.ink }}>{hang[0]}</Text>
          <VetGap />
          <Text style={{ ...typography.body, color: colors.ink }}>{hang[1]}</Text>
          <VetGap />
          <Text style={{ ...typography.body, color: colors.ink }}>{hang[2]}</Text>
        </ToGiay>
        <Text style={{ ...typography.label, color: colors.inkSoft, paddingHorizontal: 4 }}>{lyDo}</Text>
      </View>
    ))}
    <ThuGapBa height={96} testID="lab-thu-gap-ba" width={144} />

    <SectionHeader title="Nếp · trang và mảnh, ba việc của sổ hai người, hai cỡ đọc" />
    <Text style={{ ...typography.note, color: colors.inkFaint }}>
      {"Trái: trang của hội bạn. Giữa và phải: mảnh gấp làm tư, góc coral gấp vào trong, 96 và 48."}
    </Text>
    {NEP_MANH_MAU.map((pose) => (
      <View key={pose} style={{ alignItems: "flex-end", flexDirection: "row", gap: 16 }}>
        <Nep pose={pose} testID={`lab-nep-trang-${pose}-96`} />
        <Nep gap="manh" pose={pose} testID={`lab-nep-manh-${pose}-96`} />
        <Nep gap="manh" pose={pose} size={48} testID={`lab-nep-manh-${pose}-48`} />
      </View>
    ))}

    <SectionHeader title="Khám phá · renderer live, dữ liệu tổng hợp" />
    <Inline gap={8} wrap>
      {CA_ANH.map((ca) => <Chip key={ca.id} label={ca.nhan} onPress={() => setCaAnh(ca.id)} selected={caAnh === ca.id} />)}
      <Chip label="Tên dài" onPress={() => setTenDai((v) => !v)} selected={tenDai} />
    </Inline>
    <View style={{ gap: 12 }}>
      <PlaceLead daLuu={daLuu.includes(dan.id)} dd={dan} onOpen={() => undefined} onSave={() => luu(dan.id)} testID="lab-lead" />
      <PlaceCompare daLuu={(id) => daLuu.includes(id)} items={[conLai[0], conLai[1]]} onOpen={() => undefined} onSave={luu} testID="lab-compare" />
      <PlaceRow daLuu={daLuu.includes(conLai[2].id)} dd={conLai[2]} onOpen={() => undefined} onSave={() => luu(conLai[2].id)} testID="lab-row" />
    </View>

    <SectionHeader title="Chặng · renderer live, dữ liệu tổng hợp" />
    <Inline gap={8} wrap>
      {CA_CHANG.map((ca) => <Chip key={ca.id} label={ca.nhan} onPress={() => setCaChang(ca.id)} selected={caChang === ca.id} />)}
    </Inline>
    <View testID="lab-chang">
      <HangChang
        anh={(() => { const a = anhChangMau(caChang, demoAssets.road); return a ? { anh: a, alt: "Đồi tổng hợp", loai: "vui-choi" } : null; })()}
        gio="06:30"
        phac
        phu="Đồi tổng hợp"
        phuTone="inkSoft"
        tieuDe="Săn mây tổng hợp"
      />
      <HangChang
        anh={(() => { const a = anhChangMau(caChang, demoAssets.cafe); return a ? { anh: a, alt: "Quán tổng hợp", loai: "cafe" } : null; })()}
        cuoi
        gio="09:00"
        phac
        phu="Quán tổng hợp"
        phuTone="inkSoft"
        tieuDe="Cà phê sáng tổng hợp"
      />
    </View>
    <SectionHeader title="Chat · khay sticker và bong bóng, dữ liệu tổng hợp" />
    <Text style={{ ...typography.caption, color: colors.inkSoft }}>
      {"Cùng component chat live dùng: khay (ô 64) và bubble (Sticker 120, không nền không viền). Chọn một hình trong khay để đặt vào bubble; hai bản: của người khác (trái) và của mình (phải)."}
    </Text>
    <View style={{ gap: 4, paddingVertical: 4 }} testID="lab-sticker-bubble">
      <View style={{ alignSelf: "flex-start", paddingVertical: 2 }}>
        <Sticker id={stickerChon} size={120} />
      </View>
      <View style={{ alignSelf: "flex-end", paddingVertical: 2 }}>
        <Sticker id={stickerChon} size={120} />
      </View>
    </View>
    <Inline gap={8} wrap>
      <RudiButton label="Mở khay sticker" onPress={() => setKhaySticker(true)} variant="outline" />
    </Inline>
    <SectionHeader title="Chat · cả tám sticker ở hai cỡ đọc" />
    <Text style={{ ...typography.caption, color: colors.inkSoft }}>
      {"Chấm cả bộ cùng lúc, không phải từng hình: tám hành động khác nhau phải đọc ra khác nhau ở ô khay 64 trước khi đọc nhãn. Hàng trên là bản bubble 120, hàng dưới là bản rút gọn 64 trên nền ô khay."}
    </Text>
    <View style={{ gap: 10 }} testID="lab-tam-sticker">
      {STICKER_IDS.map((id) => (
        <View key={id} style={{ flexDirection: "row", alignItems: "center", gap: 12 }}>
          <Sticker id={id} size={120} />
          <View style={{ backgroundColor: colors.ground, borderColor: colors.line, borderRadius: radius.control, borderWidth: 1, padding: 6 }}>
            <Sticker id={id} size={64} />
          </View>
          {/* `flex: 1`, or the row squeezes this Text to one word and «Chờ tí»
              renders as «Chờ» -- seen only on the device, not in the sheet. */}
          <Text numberOfLines={1} style={{ ...typography.caption, color: colors.inkSoft, flex: 1 }}>{nhanSticker(id)}</Text>
        </View>
      ))}
    </View>
    <SectionHeader title="Chat · sticker đang gửi, hỏng, và gửi lại" />
    <Text style={{ ...typography.caption, color: colors.inkSoft }}>
      {"Ba trạng thái của một lần gửi, đúng hàng mà chat live vẽ (F32). Hình mờ là đang đi; hình rõ kèm câu lỗi là đã hỏng và giữ nguyên lần gửi ấy, nên «Thử lại» gửi lại đúng chìa cũ chứ không tạo tin thứ hai. Lỗi vĩnh viễn (bản app không có hình) không mời thử lại."}
    </Text>
    <View style={{ gap: 12 }} testID="lab-hang-cho">
      {CA_HANG_CHO.map((ca) => (
        <View key={ca.nhan} style={{ gap: 4 }}>
          <Text style={{ ...typography.caption, color: colors.inkFaint }}>{ca.nhan}</Text>
          {/* The chat screen's own row, not a copy of it: a board that
              photographs a hand-drawn twin proves nothing about the screen. */}
          <HangChoGui onBoQua={() => undefined} onThuLai={() => undefined} tin={ca.tin} />
        </View>
      ))}
    </View>
    <SectionHeader title="Ô tìm · placeholder dài ở ba cỡ chữ" />
    <Text style={{ ...typography.caption, color: colors.inkSoft }}>
      {"Placeholder là chữ của nhà vẽ, một dòng, cắt bằng «…» — không phải hint native của Android, vốn xuống dòng rồi bị cắt ở đáy ô khi chữ lớn (F44). Ba ô: câu dài của Khám phá live, câu của «Đi đâu?», và một ô đã gõ."}
    </Text>
    <View style={{ gap: 12 }} testID="lab-o-tim">
      <SearchField accessibilityLabel="Ô tìm địa điểm" onChangeText={() => undefined} placeholder="Tìm quán, món… hoặc hỏi Rủ Đi AI" value="" />
      <SearchField onChangeText={() => undefined} placeholder="Tìm thành phố hoặc tỉnh" value="" />
      <SearchField onChangeText={() => undefined} placeholder="Tìm quán, món… hoặc hỏi Rủ Đi AI" value="Bún bò Huế O Xuân" />
    </View>
    <SectionHeader title="Cử chỉ: kéo thả và bộ ảnh" />
    <Text style={[typography.body, { color: colors.ink }]}>Thứ tự: {items.map((item) => item.id).join(" → ")}</Text>
    <ReorderList items={items} itemKey={(item) => item.id} label={(item) => item.label}
      onChange={setItems} onDragging={setDragging}
      renderItem={(item) => <Text style={[typography.h2, { color: colors.ink, paddingVertical: 24 }]}>{item.label}</Text>} />
    <RudiButton label="Mở bộ ảnh tổng hợp" onPress={() => setViewer(true)} />

    {viewer ? <PhotoViewer initialIndex={0} title="Ảnh minh họa tổng hợp" onClose={() => setViewer(false)} photos={[
      { id: "a", source: demoAssets.cafe, caption: "Ảnh minh họa cafe · không gán cho địa điểm thật" },
      { id: "b", source: demoAssets.friends, caption: "Ảnh minh họa bạn bè · không phải người dùng thật" },
    ]} /> : null}
  </RudiScreen>;
}
