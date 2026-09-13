import { useRouter } from "expo-router";
import { useEffect, useRef, useState } from "react";
import { StyleSheet, Text, View } from "react-native";

import { typography, useRudiTheme } from "../../theme";
import { useSoDoi } from "../../to-giay/SoDoi";
import { type ToGiay, phienBan } from "../../to-giay/to-giay";
import { ngayDocDuoc } from "../../to-giay/ngay";
import { Heading, IconButton, ListRow, NhomHang, RudiButton, RudiScreen, TopBar } from "../../ui";
import { Nep } from "../../ui/art/Nep";
import { EmptyState } from "../../ui/EmptyState";
import { Sheet } from "../../ui/Sheet";
import { DeNghiSua } from "./DeNghiSua";
import { DongSo } from "./DongSo";
import { BatMotDoi, LapSo } from "./DongYBac";
import { GiuMotDieu } from "./GiuMotDieu";
import { LoaiSo } from "./LoaiSo";
import { RangBuoc } from "./RangBuoc";
import { NHAN, ToLoiRu } from "./ToLoiRu";

/**
 * The paper surface of the two-person notebook (spec §15.1): ONE open sheet,
 * chosen by `toUuTien` (what I must decide → what I sent and wait on → the
 * plan that stands → the freshest memory), then the closed sheets as plain
 * rows under a group heading. Not three cards: one letter on the table, the
 * rest in the drawer.
 *
 * The empty states are the notebook's own (spec §20.2): Nếp hands over a
 * sheet when the notebook is new, presses one face down while the other
 * person has the turn, folds one up on a week off. No Nếp at all when there
 * has never been an outing -- an empty drawer is not a scene.
 *
 * Every state change here follows the notebook (`useSoDoi`), never precedes
 * it: the fixture provider is synchronous, and Phase 4 replaces it with the
 * server's answer, so no handler assumes the change happened (§3.3 rule 6).
 */
export function KhongGianGiayScreen({ contextId, ruNgay = false }: { contextId: string; ruNgay?: boolean }) {
  const router = useRouter();
  const { colors, space } = useRudiTheme();
  const so = useSoDoi();
  const [mo, setMo] = useState<null | "de-nghi-sua" | "giu" | "lap-so" | "bat-doi" | "rang-buoc" | "dong-so" | "loai-so" | "cai-dat" | "nguoi-kia">(null);
  const daRu = useRef(false);

  // `?ru=1` from «Rủ một người đi chơi»: draft straight away, once, and only
  // when nothing is already on the table (the notebook refuses a second one).
  useEffect(() => {
    if (!ruNgay || daRu.current) return;
    daRu.current = true;
    so.ruDiChoi();
  }, [ruNgay, so]);

  const toMo = so.toMo;
  const dangCoToMo = toMo !== undefined && ["nhap", "da_gui", "da_xem", "de_nghi_sua", "dong_y"].includes(toMo.state);
  const deNghiLapSo = so.deNghiCho.find((d) => d.purpose === "lap_so");
  const deNghiBatDoi = so.deNghiCho.find((d) => d.purpose === "bat_doi");
  const pbMo = toMo ? phienBan(toMo) : undefined;
  const toiGuiToMo = pbMo?.author_type === "human" && pbMo.sent_by === so.toiId;

  const dong = () => setMo(null);

  // The close preview is asked for when the sheet opens, not computed while
  // rendering: on the live build it is a POST that mints the `revision` the
  // close must carry. `null` until it lands, and the sheet says «đang đếm»
  // rather than showing zeros somebody could agree to.
  const [xemTruoc, setXemTruoc] = useState<Awaited<ReturnType<typeof so.xemTruocDongSo>> | null>(null);
  useEffect(() => {
    if (mo !== "dong-so") {
      setXemTruoc(null);
      return;
    }
    let conDung = true;
    void so.xemTruocDongSo().then((ket) => {
      if (conDung) setXemTruoc(ket);
    });
    return () => {
      conDung = false;
    };
  }, [mo, so]);

  let than: React.ReactNode;
  if (so.daDong) {
    than = (
      <EmptyState
        action={{ label: "Lập sổ mới", onPress: () => setMo("lap-so") }}
        body={
          so.toGiay.some((t) => t.state === "da_giu")
            ? "Ký ức đã giữ vẫn đọc được ở dưới."
            : so.toGiay.some((t) => t.state === "chot" || t.state === "da_di")
              ? "Buổi đã chốt vẫn đọc được ở dưới."
              : "Các tờ đã khép vẫn đọc được ở dưới."
        }
        illustration={<Nep gap="manh" pose="gap-lai" />}
        kind="first-use"
        layout="inline"
        testID="giay-so-da-dong"
        title="Sổ này đã đóng"
      />
    );
  } else if (!so.lapSo) {
    than = (
      <EmptyState
        action={{ label: deNghiLapSo ? "Xem lời đề nghị" : "Đề nghị lập sổ", onPress: () => setMo("lap-so") }}
        body="Một chỗ để hai bạn truyền giấy cho nhau mỗi tuần. Cả hai cùng đồng ý thì sổ mở."
        kind="first-use"
        layout="inline"
        testID="giay-chua-lap-so"
        title="Chưa có sổ hai người"
      />
    );
  } else if (toMo) {
    than = (
      <ToLoiRu
        dan={dangCoToMo}
        onBoNhap={() => so.boNhap(toMo.id)}
        onDaDi={() => so.daDi(toMo.id)}
        onDeNghiSua={() => setMo("de-nghi-sua")}
        onDongY={() => so.dongY(toMo.id)}
        onGiu={() => setMo("giu")}
        onGui={() => so.gui(toMo.id)}
        onHuy={() => so.huy(toMo.id)}
        onNghiTuan={() => so.nghiTuan(toMo.id)}
        onRut={() => so.rut(toMo.id)}
        onSuaNhap={() => setMo("de-nghi-sua")}
        testID="to-mo"
        to={toMo}
        toiId={so.toiId}
      />
    );
  } else {
    const coBuoiNao = so.toGiay.some((t) => t.state === "chot" || t.state === "da_di" || t.state === "da_giu");
    than = (
      <EmptyState
        action={{ label: "Rủ đi chơi", onPress: () => void so.ruDiChoi() }}
        body={so.luotCuaToi ? "Tuần này bạn mở lời. Nếp phác sẵn, bạn sửa rồi gửi." : "Tuần này người ấy mở lời. Bạn có thể gửi trước nếu muốn."}
        illustration={coBuoiNao ? <Nep gap="manh" pose={so.luotCuaToi ? "dua-giay" : "up-xuong"} /> : undefined}
        kind="first-use"
        layout="inline"
        testID="giay-trong"
        title="Chưa có tờ nào tuần này"
      />
    );
  }

  return (
    <RudiScreen
      footer={
        // No second lead while a plan stands: with «Đã đi rồi» and «Huỷ buổi
        // này» on the sheet, a coral «Rủ đi chơi» underneath made three things
        // to do on one frame (blind read 12/09).
        !so.daDong && so.lapSo && !dangCoToMo && !(toMo && (toMo.state === "chot" || toMo.state === "da_di")) ? (
          <View style={styles.footer}>
            <RudiButton label="Rủ đi chơi" onPress={() => void so.ruDiChoi()} />
          </View>
        ) : undefined
      }
      footerInset={12}
      // Sheets ride the screen's overlay so the scrim covers the footer too
      // (a sheet drawn among the children left «Rủ đi chơi» lit beneath it).
      overlay={
        <>
      <Sheet accessibilityLabel="Cài đặt sổ" onClose={dong} open={mo === "cai-dat"} testID="cai-dat-so">
        <View style={{ gap: space.sm, paddingBottom: 8 }}>
          <Heading size="h2" title="Sổ hai người" />
          <ListRow icon="people-outline" onPress={() => setMo("loai-so")} subtitle={so.batDoi ? "Một đôi" : "Hai người bạn"} title="Loại sổ" />
          <ListRow icon="hand-left-outline" onPress={() => setMo("rang-buoc")} subtitle="Không ăn được · Đừng" title="Hai ô ràng buộc" />
          <ListRow icon="book-outline" onPress={() => router.push(`/groups/${contextId}/chat` as never)} subtitle="Về cuộc trò chuyện" title="Tin nhắn" />
          {!so.daDong ? <ListRow icon="close-circle-outline" onPress={() => setMo("dong-so")} subtitle="Xem trước rồi mới đóng" title="Đóng sổ" /> : null}
        </View>
      </Sheet>
      {toMo ? (
        <DeNghiSua
          onClose={dong}
          onGui={(content, lyDo) => {
            if (toMo.state === "nhap") so.suaNhap(toMo.id, content, lyDo);
            else so.deNghiSua(toMo.id, content, lyDo);
            dong();
          }}
          open={mo === "de-nghi-sua"}
          to={toMo}
        />
      ) : null}
      {toMo ? (
        <GiuMotDieu
          onClose={dong}
          onGiu={(line) => {
            so.giu(toMo.id, line);
            dong();
          }}
          open={mo === "giu"}
        />
      ) : null}
      <LapSo dangCho={deNghiLapSo !== undefined} deNghiCuaToi={deNghiLapSo?.cuaToi ?? true} nguoiKiaDongY={so.nguoiKia && deNghiLapSo ? () => so.nguoiKia?.dongYDeNghi(deNghiLapSo.id) : null} onClose={dong} onDeNghi={so.deNghiLapSo} onDongY={() => { if (deNghiLapSo) { so.dongYDeNghi(deNghiLapSo.id); dong(); } }} open={mo === "lap-so"} />
      <BatMotDoi dangCho={deNghiBatDoi !== undefined} deNghiCuaToi={deNghiBatDoi?.cuaToi ?? true} nguoiKiaDongY={so.nguoiKia && deNghiBatDoi ? () => so.nguoiKia?.dongYDeNghi(deNghiBatDoi.id) : null} onClose={dong} onDeNghi={so.deNghiBatDoi} onDongY={() => { if (deNghiBatDoi) { so.dongYDeNghi(deNghiBatDoi.id); dong(); } }} open={mo === "bat-doi"} />
      <LoaiSo batDoi={so.batDoi} dangCho={deNghiBatDoi !== undefined} nguoiKiaDongY={so.nguoiKia && deNghiBatDoi ? () => so.nguoiKia?.dongYDeNghi(deNghiBatDoi.id) : null} onChonBan={so.thuHoiBatDoi} onChonDoi={so.deNghiBatDoi} onClose={dong} open={mo === "loai-so"} />
      <RangBuoc nguoiKia={so.rangBuoc.nguoiKia} onClose={dong} onLuu={(rb) => { so.datRangBuoc(rb); dong(); }} open={mo === "rang-buoc"} tenNguoiKia={so.tenNguoiKia} toi={so.rangBuoc.toi} />
      <DongSo onClose={dong} onDong={() => { if (xemTruoc) { so.dongSo(xemTruoc.revision); dong(); } }} open={mo === "dong-so"} xemTruoc={xemTruoc} />
      <Sheet accessibilityLabel="Đóng vai người ấy" onClose={dong} open={mo === "nguoi-kia"} testID="nguoi-kia">
        <View style={{ gap: space.sm, paddingBottom: 8 }}>
          <Heading size="h2" subtitle="Bản trải nghiệm: máy này đóng cả vai người ấy. Mỗi nút là một việc người ấy làm trên máy của họ." title="Đóng vai người ấy" />
          {toMo && toMo.state === "da_gui" ? <RudiButton label="(Bản trải nghiệm) Người kia xem" onPress={() => { so.nguoiKia?.xem(toMo.id); dong(); }} variant="outline" /> : null}
          {toMo ? <RudiButton label="(Bản trải nghiệm) Người kia đồng ý" onPress={() => { so.nguoiKia?.dongY(toMo.id); dong(); }} variant="outline" /> : null}
          {toMo ? (
            <RudiButton
              label="(Bản trải nghiệm) Người kia đề nghị sửa giờ"
              onPress={() => {
                const pb = phienBan(toMo);
                if (!pb) return;
                // The reason must agree with the arrow: 18:30 → 18:00 is earlier,
                // 18:00 → 19:00 is later (blind read 12/09 caught «Sớm hơn» on a
                // change that went later).
                const somHon = pb.content.chang[0]?.gio !== "18:00";
                const chang = pb.content.chang.map((c, i) => (i === 0 ? { ...c, gio: somHon ? "18:00" : "19:00" } : c));
                so.nguoiKia?.deNghiSua(toMo.id, { ...pb.content, chang }, somHon ? "Sớm hơn một chút." : "Muộn hơn một chút.");
                dong();
              }}
              variant="outline"
            />
          ) : null}
          <RudiButton label="Thôi" onPress={dong} variant="ghost" />
        </View>
      </Sheet>
        </>
      }
      header={
        <TopBar
          back
          right={<IconButton accessibilityLabel="Cài đặt sổ" icon="settings-outline" onPress={() => setMo("cai-dat")} quiet />}
          subtitle={so.batDoi ? `Một đôi · ${so.tenNguoiKia}` : `Hai người bạn · ${so.tenNguoiKia}`}
          title="Tờ giấy của hai mình"
        />
      }
      testID="khong-gian-giay"
    >
      <View style={[styles.than, { gap: space.lg }]}>
        {than}
        {so.nguoiKia && toMo && toiGuiToMo && ["da_gui", "da_xem"].includes(toMo.state) ? (
          // One quiet row, not three coral lines: the tester's table must not
          // count among the things the person can do (blind read 12/09).
          <ListRow icon="swap-horizontal-outline" onPress={() => setMo("nguoi-kia")} subtitle="Xem, ừ, hay đề nghị sửa thay người ấy" title="Bản trải nghiệm: đóng vai người ấy" />
        ) : null}
        {so.toKhac.length > 0 ? (
          <View style={{ gap: space.sm }}>
            <Heading size="h2" title="Tờ đã khép" />
            <NhomHang>
              {so.toKhac.map((t) => (
                <ListRow icon="document-text-outline" key={t.id} subtitle={dongTom(t)} title={`${NHAN[t.state]} · ${ngayDocDuoc(phienBan(t)?.content.ngay ?? "")}`} />
              ))}
            </NhomHang>
          </View>
        ) : null}
      </View>

    </RudiScreen>
  );
}

function dongTom(t: ToGiay): string {
  const pb = phienBan(t);
  const chinh = pb?.content.chang[0];
  const giu = t.keeps[0]?.line;
  if (giu) return `Giữ lại: ${giu}`;
  return chinh ? `${chinh.gio} ${chinh.viec}` : "Tờ trống";
}

const styles = StyleSheet.create({
  than: { paddingTop: 8 },
  footer: { paddingHorizontal: 16 },
  dev: { borderWidth: StyleSheet.hairlineWidth, borderStyle: "dashed", borderRadius: 10, padding: 8, gap: 0 },
});
