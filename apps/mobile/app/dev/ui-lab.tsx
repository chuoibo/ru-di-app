import { Redirect } from "expo-router";
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
import { TIEN_TO_MINH_HOA, type AnhCoGhiCong } from "../../src/rudi/ui/ghi-cong";
import { Chip, Heading, Inline, RudiButton, RudiScreen, SectionHeader, TopBar } from "../../src/rudi/ui";
import { CANH_IDS, moTaCanh } from "../../src/rudi/art/canh";
import { Canh } from "../../src/rudi/ui/art/Canh";
import { EmptyState } from "../../src/rudi/ui/EmptyState";
import { ReorderList } from "../../src/rudi/ui/ReorderList";
import { PhotoViewer } from "../../src/rudi/ui/PhotoViewer";

/**
 * A source that cannot resolve, so the `onError` branch of every image can be
 * seen on the device. Loopback on a closed port: nothing leaves the machine.
 */
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
  return { source: ca === "hong" ? ANH_HONG : that, nguon: GHI_CONG_MAU };
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
    return p ? { source: p, nguon: GHI_CONG_MAU } : null;
  };
  const ten = (ngan: string, dai: string) => (tenDai ? dai : ngan);
  const facts = (sao: string, xa: string, gia: string): DiaDiemHienThi["facts"] => [
    { icon: "star", text: sao },
    { icon: "navigate-outline", text: xa },
    { icon: "wallet-outline", text: gia },
  ];
  return [
    { id: "lab-a", name: ten("Bánh căn Lệ", "Tiệm Bánh Căn Cô Lệ Đường Nguyễn Văn Trỗi"), sub: "Một dòng mô tả tổng hợp, không phải quán thật", facts: facts("4.6 (188)", "900 m", "40K - 80K/người"), glyph: "location-outline", loai: "quan-an-local", anh: anhLab(demoAssets.cafe, 0), badge: "Hợp gu", lyDo: "Hợp gu nhờ hai gu tổng hợp" },
    { id: "lab-b", name: ten("Lẩu gà lá é", "Lẩu Gà Lá É Gốc Đường Ba Tháng Hai Phường Một"), sub: "Đủ chỗ nhóm tám, tổng hợp", facts: facts("4.7 (214)", "1,6 km", "180K - 260K/người"), glyph: "location-outline", loai: "quan-an-local", anh: anhLab(demoAssets.road, 1), badge: null },
    { id: "lab-c", name: ten("Still Cafe", "Still Cafe Đà Lạt Chi Nhánh Đường Trần Hưng Đạo"), sub: "Cà phê view đồi, tổng hợp", facts: facts("4.7 (512)", "1,8 km", "120K - 200K/người"), glyph: "location-outline", loai: "cafe", anh: anhLab(demoAssets.cafe, 2), badge: "Hợp gu" },
    { id: "lab-d", name: ten("Tiệm trà Sương", "Tiệm Trà Sương Sớm Trên Đồi Thông Phường Mười"), sub: "Trà thảo mộc, tổng hợp", facts: facts("4.5 (143)", "2,1 km", "80K - 140K/người"), glyph: "location-outline", loai: "cafe", anh: anhLab(demoAssets.road, 3), badge: null },
  ];
}

/** Synthetic native probe: gestures, and the album and explore renderers under invented states. Never in a production build. */
export default function UiLab() {
  const { colors } = useRudiTheme();
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
