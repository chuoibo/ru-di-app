/**
 * `ui-lab`'s paper board (ADR-0037, plan S0.5): every shared primitive of the
 * paper stage on invented data, so each can be seen, read with a screen
 * reader and filmed in both schemes before a screen uses it.
 */
import { useState } from "react";
import { Text, View } from "react-native";

import { mucNguoi, typography, useRudiTheme } from "../theme";
import { Chip, Inline, RudiButton } from "../ui";
import { Avatar, HinhNhan } from "./Avatar";
import { BanXoay } from "./BanXoay";
import { CauRu } from "./CauRu";
import { ChonNgayLich } from "./ChonNgayLich";
import { ChuThichLe } from "./ChuThichLe";
import { CuongPhieu } from "./CuongPhieu";
import { DauLon } from "./DauLon";
import { DongHoaDon, HoaDonGiay, TieuDeHoaDon, VachCat } from "./HoaDonGiay";
import { LatTrang } from "./LatTrang";
import { NapGiay } from "./NapGiay";
import { ONhapMuc } from "./ONhapMuc";
import { PhongBi } from "./PhongBi";
import { SoMoHaiTrang } from "./SoMoHaiTrang";
import { StampButton } from "./StampButton";
import { Tem } from "./Tem";
import { TheVe } from "./TheVe";

// Invented people: ids chosen only to show eight different inks.
const NGUOI = ["an", "binh", "chi", "dung", "giang", "hoa", "khanh", "linh"].map((ten, i) => ({ id: `lab-nguoi-${i}-${ten}`, ten: ten[0].toUpperCase() + ten.slice(1) }));

function Nhan({ children }: { children: string }) {
  const { colors } = useRudiTheme();
  return <Text style={{ ...typography.caption, color: colors.inkSoft }}>{children}</Text>;
}

export function ThuBoGiay() {
  const { colors, dark } = useRudiTheme();
  const [ten, setTen] = useState("");
  const [ghiChu, setGhiChu] = useState("");
  const [tu, setTu] = useState("25/09/2026");
  const [den, setDen] = useState("");
  const [gio, setGio] = useState<number | null>(18 * 60 + 30);
  const [so, setSo] = useState(4);
  const [dauLan, setDauLan] = useState(0);
  const [trang, setTrang] = useState(0);
  return (
    <View style={{ gap: 18 }}>
      <Nhan>{"Ô viết trên dòng kẻ (ONhapMuc): không hộp, gạch mực 1dp, 2dp cam khi viết, 2dp cảnh báo khi sai."}</Nhan>
      <ONhapMuc accessibilityLabel="Ô tên thử" label="Tên kèo" onChangeText={setTen} placeholder="Ví dụ: Ăn tối thứ Bảy" value={ten} />
      <ONhapMuc co="lon" label="Tên lớn" onChangeText={setTen} placeholder="Hội Đạp Xe" value={ten} />
      <ONhapMuc error="Tên món còn trống" label="Có lỗi" onChangeText={() => undefined} value="" />
      <ONhapMuc helper="Dòng kẻ dưới mỗi dòng chữ" label="Ghi chú nhiều dòng" multiline numberOfLines={3} onChangeText={setGhiChu} value={ghiChu} />

      <Nhan>{"Câu rủ có ô trống (CauRu) với lá lịch, số người, ô tên:"}</Nhan>
      <CauRu
        mau="Rủ {nhom} đi {ten} từ {tu} tới {den}, {so} người."
        moTa="Rủ Hội Đạp Xe đi, tên kèo, từ ngày, tới ngày, số người"
        o={{
          nhom: <Text style={{ ...typography.h2, color: colors.accent }}>Hội Đạp Xe</Text>,
          ten: <ONhapMuc accessibilityLabel="Ô tên kèo thử" co="lon" khungStyle={{ minWidth: 150 }} onChangeText={setTen} placeholder="ăn tối" value={ten} />,
          tu: <ChonNgayLich giaTri={tu} nhan="Ngày đi" onChange={setTu} testID="lab-ngay-di" />,
          den: <ChonNgayLich giaTri={den} nhan="Ngày về" onChange={setDen} />,
          so: (
            <Inline gap={4}>
              <Chip label="−" onPress={() => setSo((n) => Math.max(1, n - 1))} />
              <Text style={{ ...typography.h2, color: colors.ink, minWidth: 28, textAlign: "center" }}>{so}</Text>
              <Chip label="+" onPress={() => setSo((n) => n + 1)} />
            </Inline>
          ),
        }}
        testID="lab-cau-ru"
      />
      <ChonNgayLich giaTri={tu} kieu="dong" nhan="Ngày đi (gõ tay)" oLabel="Ô ngày đi thử" onChange={setTu} />
      <BanXoay nhan="Giờ chặng" oLabel="Ô giờ chặng thử" onChange={setGio} phut={gio} testID="lab-ban-xoay" />

      <Nhan>{"Hoá đơn nhiệt, cuống phiếu mực người, vé, phong bì, tem:"}</Nhan>
      <HoaDonGiay rangTren testID="lab-hoa-don">
        <TieuDeHoaDon phu="25/09/2026 · 19:40" ten="Quán Bún Chả" />
        <VachCat />
        <DongHoaDon phai="120.000" phu="2 phần × 60.000" trai="Bún chả" />
        <DongHoaDon phai="45.000" trai="Nem cua bể" />
        <DongHoaDon phai="30.000" trai="Trà đá" />
        <VachCat />
        <DongHoaDon dam phai="195.000" trai="Tổng" />
      </HoaDonGiay>
      <View style={{ gap: 10 }}>
        {NGUOI.slice(0, 3).map((p, i) => (
          <CuongPhieu key={p.id} mau={mucNguoi(p.id, dark)} noi={i === 0}>
            <Inline gap={10}>
              <Avatar name={p.ten} personId={p.id} size={32} />
              <Text style={{ ...typography.title, color: colors.ink, flex: 1 }}>{i === 0 ? "Phần của bạn" : p.ten}</Text>
              <Text style={{ ...typography.title, color: colors.split, fontVariant: ["tabular-nums"] }}>65.000</Text>
            </Inline>
          </CuongPhieu>
        ))}
      </View>
      <TheVe
        cuong={
          <>
            <Text style={{ ...typography.stamp, color: colors.inkSoft }}>Th 9</Text>
            <Text style={{ ...typography.h1, color: colors.ink }}>27</Text>
          </>
        }
        testID="lab-the-ve"
      >
        <Text style={{ ...typography.stamp, color: colors.accent }}>Kèo · Hội Đạp Xe</Text>
        <Text style={{ ...typography.h2, color: colors.ink }}>Ăn tối thứ Bảy</Text>
        <Text style={{ ...typography.note, color: colors.inkSoft }}>18:30 · 4 người</Text>
      </TheVe>
      <PhongBi testID="lab-phong-bi">
        <Text style={{ ...typography.h2, color: colors.ink }}>Mời vào hội</Text>
        <ONhapMuc accessibilityLabel="Ô số điện thoại thử" keyboardType="phone-pad" label="Số điện thoại" onChangeText={() => undefined} placeholder="09xx xxx xxx" value="" />
        <ChuThichLe icon="lock-closed-outline">{"Số điện thoại chỉ dùng để gửi lời mời này."}</ChuThichLe>
      </PhongBi>
      <Inline gap={12} wrap>
        <Tem accessibilityLabel="Tem đã có">
          <Text style={{ ...typography.caption, color: colors.ink, textAlign: "center" }}>Lần đầu chia bill</Text>
        </Tem>
        <Tem accessibilityLabel="Tem chưa có" khoa>
          <Text style={{ ...typography.caption, color: colors.inkFaint, textAlign: "center" }}>Chưa có</Text>
        </Tem>
      </Inline>

      <Nhan>{"Dấu: nút dấu cam (hỏi) và teal (quyết định tiền); dấu lớn dập xuống:"}</Nhan>
      <Inline gap={12} wrap>
        <StampButton label="Tạo kèo" onPress={() => undefined} size="vua" />
        <StampButton label="Ghi vào sổ" onPress={() => undefined} size="vua" tone="split" />
      </Inline>
      <DauLon dong={dauLan > 0} key={dauLan} nhan="Đã ghi sổ" testID="lab-dau-lon" />
      <RudiButton label="Dập lại dấu" onPress={() => setDauLan((n) => n + 1)} variant="outline" />

      <Nhan>{"Nắp lật cho chữ phụ, chú thích lề cho chữ phải thấy:"}</Nhan>
      <NapGiay tieuDe="Cách chia">
        <Text style={{ ...typography.body, color: colors.ink }}>{"Mỗi món chia đều cho người ăn món đó; phần lẻ làm tròn về người trả."}</Text>
      </NapGiay>
      <ChuThichLe icon="lock-open-outline">{"Chưa mã hoá đầu cuối."}</ChuThichLe>

      <Nhan>{"Lật trang giữa các bước (trang mới đứng yên, tờ cũ lật đi):"}</Nhan>
      <LatTrang khoa={`trang-${trang}`} thuTu={trang}>
        <View style={{ minHeight: 120, gap: 8, padding: 12, borderWidth: 1, borderColor: colors.lineStrong, borderRadius: 10, backgroundColor: colors.card }}>
          <Text style={{ ...typography.h2, color: colors.ink }}>{`Bước ${trang + 1}/3`}</Text>
          <Text style={{ ...typography.body, color: colors.inkSoft }}>{["Chọn ảnh hoá đơn", "Xem lại các món", "Gán món cho người"][trang]}</Text>
        </View>
      </LatTrang>
      <View style={{ flexDirection: "row", gap: 8 }}>
        <View style={{ flex: 1 }}>
          <RudiButton disabled={trang === 0} label="Lùi" onPress={() => setTrang((n) => Math.max(0, n - 1))} variant="outline" />
        </View>
        <View style={{ flex: 1 }}>
          <RudiButton disabled={trang === 2} label="Tiếp" onPress={() => setTrang((n) => Math.min(2, n + 1))} />
        </View>
      </View>

      <Nhan>{"Mực người: tám người, tám mực; hình nhân giấy:"}</Nhan>
      <Inline gap={8} wrap>
        {NGUOI.map((p) => (
          <Avatar key={p.id} name={p.ten} personId={p.id} size={40} />
        ))}
      </Inline>
      <Inline gap={4} wrap>
        {NGUOI.slice(0, 5).map((p) => (
          <HinhNhan key={p.id} name={p.ten} personId={p.id} size={38} />
        ))}
      </Inline>

      <SoMoHaiTrang
        phai={<Text style={{ ...typography.body, color: colors.ink }}>{"Trang phải (chỉ trải hai trang từ 840dp)."}</Text>}
        trai={<Text style={{ ...typography.body, color: colors.ink }}>{"Trang trái."}</Text>}
      />
    </View>
  );
}
