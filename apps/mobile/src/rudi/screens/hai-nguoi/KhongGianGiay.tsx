import { useRouter } from "expo-router";
import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";
import { Easing, useSharedValue, withDelay, withTiming } from "react-native-reanimated";

import { useRudiSession } from "../../session";
import { typography, useRudiTheme } from "../../theme";
import { useSoDoi } from "../../to-giay/SoDoi";
import { type GuChat, cauGu } from "../../to-giay/gu-doi";
import { docChatCapabilities } from "../../chat/ai-invocations";
import { cauVaiTuan } from "../../to-giay/vai-tuan";
import { type ToGiay, goiYChoLam, nenXinTo, phienBan } from "../../to-giay/to-giay";
import { ngayDocDuoc } from "../../to-giay/ngay";
import { useTenCho } from "../../to-giay/useTenCho";
import { Heading, IconButton, ListRow, NhomHang, RudiButton, RudiScreen, TopBar } from "../../ui";
import { Nep } from "../../ui/art/Nep";
import { DauLon } from "../../ui/DauLon";
import { EmptyState } from "../../ui/EmptyState";
import { ErrorState } from "../../ui/ErrorState";
import { NepDien } from "../../ui/NepDien";
import { NepTrongTrang } from "../../ui/NepRoi";
import { Sheet } from "../../ui/Sheet";
import { SoBia } from "../../ui/SoBia";
import { StampButton } from "../../ui/StampButton";
import { ToGiay as ToGiayView, VetGap } from "../../ui/ToGiay";
import { useMotion } from "../../ui/useMotion";
import { DeNghiSua } from "./DeNghiSua";
import { DongSo } from "./DongSo";
import { XacNhanViec } from "./XacNhanViec";
import { BatMotDoi, LapSo } from "./DongYBac";
import { GiuMotDieu } from "./GiuMotDieu";
import { AiLoTuanNay } from "./AiLoTuanNay";
import { GuHaiBan } from "./GuHaiBan";
import { LoaiSo } from "./LoaiSo";
import { RangBuoc } from "./RangBuoc";
import { NHAN, ToLoiRu } from "./ToLoiRu";
import { homNay, nhipKeo } from "../../keo/nhip-keo";
import { useNepNguCanh } from "../../nep/NepProvider";

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
export function KhongGianGiayScreen({ contextId, ruNgay = false, choGoiY }: { contextId: string; ruNgay?: boolean; choGoiY?: string }) {
  const router = useRouter();
  const { colors, space } = useRudiTheme();
  const motion = useMotion();
  const { phien } = useRudiSession();
  const so = useSoDoi();
  const [mo, setMo] = useState<null | "de-nghi-sua" | "giu" | "lap-so" | "bat-doi" | "rang-buoc" | "dong-so" | "loai-so" | "cai-dat" | "gu" | "vai">(null);
  const daRu = useRef(false);
  // Where my taste switch stands for the chat (ADR-0048 §3.2), asked when the
  // taste sheet opens and again after each change of it; unknown reads as
  // null and offers no «Bật lại cho chat».
  const [guChat, setGuChat] = useState<GuChat | null>(null);
  useEffect(() => {
    if (mo !== "gu" || !so.capId || !so.toiId || so.dangLam !== null) return;
    let bo = false;
    docChatCapabilities(so.capId, so.toiId)
      .then((caps) => { if (!bo) setGuChat(caps.gu_chat ?? null); })
      .catch(() => { if (!bo) setGuChat(null); });
    return () => { bo = true; };
  }, [mo, so.capId, so.toiId, so.dangLam, so.gu?.mine_shared]);

  // `?ru=1` from «Rủ một người đi chơi»: draft straight away, once, and only
  // when nothing is already on the table (the notebook refuses a second one).
  useEffect(() => {
    if (!ruNgay || daRu.current || !so.batDoi || so.daDung) return;
    const lam = nenXinTo(so.daNap, so.toMo, so.lapSo);
    if (lam === "cho") return;
    daRu.current = true;
    if (lam === "xin") so.ruDiChoi();
  }, [ruNgay, so]);

  const toMo = so.toMo;
  // What the two like in common, once both have shared (ADR-0034): the one
  // line of insight the notebook can show without anybody asking for it.
  const cauGuSo = cauGu(so.gu, so.tenNguoiKia);
  // Whose week it is (ADR-0034 §2.4), inferred or chosen; shown only in an
  // open «Một đôi», with a way to change it.
  // A stopped pair has no week to share out (the server refuses the change).
  const cauVai = so.daDung ? null : cauVaiTuan(so.vai, so.toiId, so.tenNguoiKia);
  // «Rủ … tới đây»: once this person's own draft is on the table, open it
  // with the place filled in as the main stop -- once, not on every render.
  const [goiYCho, setGoiYCho] = useState<string | undefined>(choGoiY);
  const [cauGoiY, setCauGoiY] = useState<string | null>(null);
  // Where the place goes when no sheet can take it (QA UI-085, UI-130): a kèo
  // of the two with it as the first stop, or the plan they already agreed.
  const [choVaoKeo, setChoVaoKeo] = useState<{ id: string; lam: "tao-keo" | "them-vao-buoi" } | null>(null);
  const tenChoVaoKeo = useTenCho([choVaoKeo?.id])[choVaoKeo?.id ?? ""];
  const daMoGoiY = useRef(false);
  useEffect(() => {
    if (!goiYCho || daMoGoiY.current || !so.daNap || so.loiDoc !== null) return;
    // Two friends who are not «Một đôi» (with a notebook or not) plan like any
    // group: the place opens a kèo of the two of them, already its first stop.
    // It used to be dropped, with or without a sentence (QA UI-130).
    if (!so.batDoi) {
      daMoGoiY.current = true;
      setChoVaoKeo({ id: goiYCho, lam: "tao-keo" });
      setGoiYCho(undefined);
      return;
    }
    const lam = goiYChoLam(toMo, so.toiId, so.tenNguoiKia, goiYCho);
    if (lam.lam === "cho") return;
    daMoGoiY.current = true;
    if (lam.lam === "mo") setMo("de-nghi-sua");
    else {
      if (lam.lam === "them-vao-buoi") setChoVaoKeo({ id: goiYCho, lam: "them-vao-buoi" });
      setGoiYCho(undefined);
      setCauGoiY(lam.cau);
    }
  }, [goiYCho, toMo, so.toiId, so.tenNguoiKia, so.daNap, so.loiDoc, so.batDoi]);
  const dangCoToMo = toMo !== undefined && ["nhap", "da_gui", "da_xem", "de_nghi_sua", "dong_y"].includes(toMo.state);
  const deNghiLapSo = so.deNghiCho.find((d) => d.purpose === "lap_so");
  const deNghiBatDoi = so.deNghiCho.find((d) => d.purpose === "bat_doi");
  const pbMo = toMo ? phienBan(toMo) : undefined;
  // The notebook's kind, the open sheet's day and how many stops it holds --
  // what the person is reading, without the other person's name.
  useNepNguCanh({
    man: "groups/[id]/to-giay",
    tieuDe: "Tờ giấy của hai mình",
    loaiSo: so.batDoi ? "doi" : "hoi",
    nhip: pbMo?.content.ngay ? nhipKeo(pbMo.content.ngay, pbMo.content.ngay, homNay()) : undefined,
    soLieu: pbMo ? { soChang: pbMo.content.chang.length } : undefined,
    goiY: ["Tuần này đi đâu cho mới?", "Nhắc mình trước buổi hẹn"],
  });
  const toiGuiToMo = pbMo?.author_type === "human" && pbMo.sent_by === so.toiId;

  // Bốn việc không lấy lại được đi qua một tờ xác nhận nói ra hậu quả trước
  // khi làm — luật của chính bản dựng này, và `DongSo` đã có khuôn ấy.
  const [viec, setViec] = useState<null | "bo" | "rut" | "nghi_tuan" | "huy">(null);

  const dong = () => setMo(null);

  // The notebook opening while this screen is up (both agreed just now): the
  // one time the M6 moment plays. A notebook that was already open when the
  // screen came up is not news.
  const lapSoTruoc = useRef<boolean | null>(null);
  const batDoiTruoc = useRef<boolean | null>(null);
  const [vuaMoSo, setVuaMoSo] = useState(false);
  const moBia = useSharedValue(0);
  // Before paint, not after (QA UI-084): an effect left one or two frames of
  // the plain «Tuần này» body between the agreement and the open cover. The
  // sheet that asked for the notebook closes on both phones the moment it
  // opens: it used to stay up on the proposer's and offer to propose again.
  useLayoutEffect(() => {
    if (!so.daNap) return;
    if (lapSoTruoc.current === false && so.lapSo) {
      setVuaMoSo(true);
      setMo((m) => (m === "lap-so" ? null : m));
      moBia.value = withDelay(120, withTiming(1, { duration: motion.ms("shared") * 2, easing: Easing.bezier(0.3, 0, 0.2, 1), reduceMotion: motion.reanimated }));
    }
    lapSoTruoc.current = so.lapSo;
  }, [so.daNap, so.lapSo, moBia, motion]);
  useLayoutEffect(() => {
    if (!so.daNap) return;
    if (batDoiTruoc.current === false && so.batDoi) setMo((m) => (m === "bat-doi" || m === "loai-so" ? null : m));
    batDoiTruoc.current = so.batDoi;
  }, [so.daNap, so.batDoi]);
  // M6, the notebook just opened under this person's eyes (ADR-0037): the
  // cover swings open, Nếp pulls the tab and jumps, the seal lands. Once, at
  // the step that opens the notebook, in whichever body comes next (QA UI-127:
  // a pair not yet «Một đôi» never reached the body that drew it).
  const tenToiBia = phien?.profile?.display_name?.trim() || "Bạn";
  const khoanhKhacM6 = vuaMoSo ? (
    <View style={styles.hangCanh} testID="giay-m6">
      <SoBia mo={moBia} rong={140} ten={[tenToiBia, so.tenNguoiKia || "Người ấy"]} trang={<DauLon co="nho" dong nhan="Sổ đã mở" tone="accent" tre={380} />} />
      <NepDien khoanhKhac="M6" suKien={`so-mo:${so.capId}`} />
    </View>
  ) : null;
  const tenToi = phien?.profile?.display_name?.trim() || "Bạn";

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

  // Sheets that can only be read now: the day, the stops, and the way to the
  // outing they became. Shared by «một hội» and by a stopped pair.
  const toChiDoc = () =>
    so.toGiay.map((t) => {
      const version = phienBan(t);
      return (
        <View key={t.id} style={{ gap: 8 }}>
          <Text style={[typography.h2, { color: colors.ink }]}>{ngayDocDuoc(version?.content.ngay ?? "") || "Tờ chưa có ngày"}</Text>
          <Text style={[typography.caption, { color: colors.inkSoft }]}>Tờ giấy cũ · chỉ đọc</Text>
          {version?.content.chang.map((c, i) => <Text key={i} style={[typography.body, { color: colors.ink }]}>{c.gio} · {c.viec}</Text>)}
          {t.outing_id ? <RudiButton label="Mở cuộc đi này" variant="ghost" onPress={() => router.push(`/outings/${t.outing_id}?ctx=${contextId}` as never)} /> : null}
        </View>
      );
    });

  let than: React.ReactNode;
  // The «hội» body carries «Rủ hội mình đi chơi»; a place brought here rides
  // that one button into the form instead of a second button above it (B11).
  let coNutRuHoi = false;
  const ruHoi = choVaoKeo?.lam === "tao-keo"
    ? `/outings/new?contextId=${contextId}&place=${encodeURIComponent(choVaoKeo.id)}`
    : `/outings/new?contextId=${contextId}`;
  if (so.loiDoc !== null) {
    // QA UI-083: a failed read is not a notebook that was never opened. It
    // drew «Chưa có sổ hai người» and offered «Đề nghị lập sổ» over a notebook
    // both had already opened.
    than = <ErrorState body={so.loiDoc} onRetry={so.lamMoi} title="Chưa đọc được sổ của hai bạn" />;
  } else if (so.daDung) {
    // QA UI-120: blocked, or the other account ended. The server refuses every
    // outward write from now on, so nothing here offers one; one sentence for
    // both causes, and what was already written stays readable underneath.
    than = (
      <View style={{ gap: space.lg }} testID="giay-so-da-dung">
        <EmptyState
          body={so.toGiay.length > 0 ? "Hai bạn không gửi tờ cho nhau được nữa. Những tờ cũ vẫn đọc được ở dưới." : "Hai bạn không gửi tờ cho nhau được nữa."}
          illustration={<Nep gap="manh" pose="gap-lai" />}
          kind="permission"
          layout="inline"
          title="Sổ này đã dừng"
        />
        {toChiDoc()}
      </View>
    );
  } else if (so.daDong) {
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
    // The notebook before both have agreed: a book with their two names on
    // it, shut, and Nếp folding a sheet small beside it (ADR-0037 D1).
    than = (
      <View style={styles.canh} testID="giay-chua-lap-so">
        <View style={styles.hangCanh}>
          <SoBia rong={128} ten={[tenToi, so.tenNguoiKia || "Người ấy"]} />
          <NepTrongTrang pose="gap-lai" size={112} />
        </View>
        {/* Any pair keeps a notebook, «Hội bạn» or «Một đôi» (ADR-0053): the
            line does not assume a couple, and says the kind is chosen later. */}
        <Heading subtitle="Một cuốn sổ chỉ hai bạn đọc, để hẹn nhau từng tuần. Cả hai đồng ý thì sổ mở; là «Hội bạn» hay «Một đôi», hai bạn chọn sau." title="Một chỗ cho chuyện hai mình" />
        {/* Whose proposal is waiting, said on the page (QA UI-086): closing
            the sheet left the proposer reading the receiver's words. */}
        {deNghiLapSo ? (
          <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.ink }]} testID={deNghiLapSo.cuaToi ? "giay-cho-dong-y" : "giay-duoc-de-nghi"}>
            {deNghiLapSo.cuaToi ? `Đã đề nghị lập sổ. Chờ ${so.tenNguoiKia || "người ấy"} đồng ý trên máy của họ.` : `${so.tenNguoiKia || "Người ấy"} đề nghị lập sổ cho hai bạn.`}
          </Text>
        ) : null}
        <StampButton label={deNghiLapSo ? (deNghiLapSo.cuaToi ? "Xem lời đề nghị của bạn" : "Xem lời đề nghị") : "Đề nghị lập sổ"} onPress={() => setMo("lap-so")} size="vua" tilt={-1} />
      </View>
    );
  } else if (!so.batDoi && !dangCoToMo) {
    // A sheet still in play (sent before the pair became a group, or by an
    // older client) keeps its actions below; only settled sheets turn read-only.
    coNutRuHoi = true;
    than = <View style={{ gap: space.md }}>
      {khoanhKhacM6}
      <Heading title="Hai người cũng thành một hội" subtitle={so.toGiay.length > 0 ? "Hẹn nhau như mọi hội bạn. Những tờ giấy cũ vẫn nằm ở đây." : "Hẹn nhau như mọi hội bạn."} />
      {/* QA UI-126: the other person's «Một đôi» proposal is answered from the
          page it leads to, not from behind «Mở sổ cặp đôi». */}
      {deNghiBatDoi && !deNghiBatDoi.cuaToi ? (
        <View style={{ gap: space.sm }} testID="giay-duoc-de-nghi-doi">
          <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.ink }]}>{`${so.tenNguoiKia || "Người ấy"} đề nghị hai bạn là «Một đôi».`}</Text>
          <StampButton label="Xem lời đề nghị" onPress={() => setMo("loai-so")} size="vua" tilt={-1} />
        </View>
      ) : deNghiBatDoi ? (
        <Text style={[typography.body, { color: colors.inkSoft }]} testID="giay-cho-dong-y-doi">{`Đã đề nghị «Một đôi». Chờ ${so.tenNguoiKia || "người ấy"} đồng ý trên máy của họ.`}</Text>
      ) : null}
      <RudiButton label="Rủ hội mình đi chơi" onPress={() => router.push(ruHoi as never)} />
      {/* The screen's own door to «Loại sổ» stays in every state, with the same
          name: the stamp above is a shortcut for a proposal waiting, not a
          replacement (hiding the door broke the way people and Maestro 47
          already knew). Both open the same sheet. */}
      <RudiButton label="Mở sổ cặp đôi" variant="outline" onPress={() => setMo("loai-so")} />
      {toChiDoc()}
    </View>;
  } else if (toMo) {
    than = (
      <ToLoiRu
        dan={dangCoToMo}
        onBoNhap={() => setViec("bo")}
        onDaDi={() => so.daDi(toMo.id)}
        onDeNghiSua={() => setMo("de-nghi-sua")}
        onDongY={() => so.dongY(toMo.id)}
        onGiu={() => setMo("giu")}
        onGui={() => so.gui(toMo.id)}
        onHuy={() => setViec("huy")}
        onNghiTuan={() => setViec("nghi_tuan")}
        onRut={() => setViec("rut")}
        onSuaNhap={() => setMo("de-nghi-sua")}
        // The agreed sheet became an outing in this pair, its stops the
        // outing's timeline: the way there, where it used to be reachable only
        // by guessing the plan list's «Tờ lời rủ dd/mm» (QA 23/09).
        onXemKeo={toMo.outing_id && ["chot", "da_di", "da_giu"].includes(toMo.state) ? () => router.push(`/outings/${toMo.outing_id}?ctx=${contextId}` as never) : undefined}
        tatCa={so.toGiay}
        tenNguoiKia={so.tenNguoiKia}
        testID="to-mo"
        to={toMo}
        toiId={so.toiId}
      />
    );
  } else if (so.xinToBiChan) {
    // B1: this week already holds a sheet this person cannot see -- the other
    // person's draft, private until sent. Say so, wait for it, look again; no
    // «Rủ đi chơi» that would fail the same way (QC 24/09).
    than = (
      <View style={styles.canh} testID="giay-cho-nguoi-kia">
        <NepTrongTrang pose="up-xuong" size={120} style={styles.giuaNgang} />
        <Heading
          subtitle="Tuần này đã có một tờ đang mở ở phía người ấy. Khi tờ được gửi, nó tới đây."
          title={`Tờ tuần này ở phía ${so.tenNguoiKia || "người ấy"}`}
        />
        <RudiButton icon="refresh-outline" label="Làm mới" onPress={so.lamMoi} variant="outline" />
      </View>
    );
  } else {
    // A week with no sheet yet: the sheet itself, tri-folded and still blank,
    // pencil lines where Nếp will draft, and Nếp handing it over -- the first
    // time too (the scene used to wait for a first outing, QA 24/09).
    const cauLuot = so.coLuot
      ? so.luotCuaToi
        ? "Tuần này bạn mở lời. Nếp phác sẵn, bạn sửa rồi gửi."
        : `Tuần này ${so.tenNguoiKia || "người ấy"} mở lời. Bạn có thể gửi trước nếu muốn.`
      : "Ai mở lời trước cũng được: Nếp phác sẵn một tờ, bạn sửa rồi gửi.";
    than = (
      <View style={styles.canh} testID="giay-trong">
        {vuaMoSo ? (
          khoanhKhacM6
        ) : (
          <View style={styles.hangCanh}>
            <View style={styles.toTrong}>
              <ToGiayView dan>
                <Text style={[typography.stamp, styles.lineStamp, { color: colors.inkSoft }]}>Tuần này</Text>
                {[0, 1, 2].map((i) => (
                  <View importantForAccessibility="no-hide-descendants" key={i}>
                    {i > 0 ? <VetGap /> : null}
                    <View style={[styles.dongChi, { borderBottomColor: colors.lineStrong, width: `${[82, 64, 74][i]}%` }]} />
                  </View>
                ))}
              </ToGiayView>
            </View>
            <NepTrongTrang pose={so.coLuot && !so.luotCuaToi ? "up-xuong" : "dua-giay"} size={112} />
          </View>
        )}
        <Heading subtitle={cauLuot} title="Chưa có tờ nào tuần này" />
        <StampButton label="Rủ đi chơi" onPress={() => void so.ruDiChoi()} size="vua" tilt={-1} />
      </View>
    );
  }

  return (
    <RudiScreen
      // A sheet of paper is read, not scanned: one 640 dp column on a tablet,
      // its two buttons with it (QA UI-093: 720 and 912 px wide at C6, C7).
      cot="doc"
      footer={
        // No second lead while a plan stands: with «Đã đi rồi» and «Huỷ buổi
        // này» on the sheet, a coral «Rủ đi chơi» underneath made three things
        // to do on one frame (blind read 12/09).
        // …and no second lead while the EMPTY STATE is already offering the same
        // word: with no sheet at all, «Rủ đi chơi» rendered twice in coral,
        // 1300px apart, and the second one reads as a different action somebody
        // then hunts for the difference between (finish review 14/09).
        so.batDoi && !so.daDong && !so.daDung && so.lapSo && toMo !== undefined && !dangCoToMo && !(toMo.state === "chot" || toMo.state === "da_di") ? (
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
          <Heading size="h2" title="Chuyện của hai mình" />
          {!so.daDung ? <ListRow icon="people-outline" onPress={() => setMo("loai-so")} subtitle={so.batDoi ? "Một đôi" : "Hội bạn"} title="Loại sổ" /> : null}
          {!so.daDung ? <ListRow icon="hand-left-outline" onPress={() => setMo("rang-buoc")} subtitle="Không ăn được · Đừng" title="Những điều cần tránh" /> : null}
          {so.gu ? <ListRow icon="heart-outline" onPress={() => setMo("gu")} subtitle={cauGuSo?.chung ?? (so.gu.mine_shared ? "Bạn đang chia gu" : "Mỗi người tự bật")} title="Gu của hai bạn" /> : null}
          <ListRow icon="book-outline" onPress={() => router.push(`/groups/${contextId}/chat` as never)} subtitle="Về cuộc trò chuyện" title="Tin nhắn" />
          <ListRow icon="images-outline" onPress={() => router.push(`/groups/${contextId}/wall` as never)} subtitle="Ảnh và những buổi hai bạn đã giữ" title="Kỷ niệm của hai bạn" />
          {!so.daDong ? <ListRow icon="close-circle-outline" onPress={() => setMo("dong-so")} subtitle="Xem trước rồi mới đóng" title="Đóng sổ" /> : null}
        </View>
      </Sheet>
      {toMo ? (
        <DeNghiSua
          choGoiY={daMoGoiY.current ? goiYCho : undefined}
          onClose={() => {
            setGoiYCho(undefined);
            dong();
          }}
          onGui={(content, lyDo) => {
            if (toMo.state === "nhap") so.suaNhap(toMo.id, content, lyDo);
            else so.deNghiSua(toMo.id, content, lyDo);
            setGoiYCho(undefined);
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
      {/* A sheet closes on the notebook's answer, not on the press: a refused
          or dropped write used to close it exactly like a saved one (QA 23/09). */}
      <LapSo dangCho={deNghiLapSo !== undefined} deNghiCuaToi={deNghiLapSo?.cuaToi ?? true} onClose={dong} onDeNghi={so.deNghiLapSo} onDongY={() => { if (deNghiLapSo) void so.dongYDeNghi(deNghiLapSo.id).then((ok) => ok && dong()); }} open={mo === "lap-so"} tenNguoiKia={so.tenNguoiKia} />
      <BatMotDoi dangCho={deNghiBatDoi !== undefined} deNghiCuaToi={deNghiBatDoi?.cuaToi ?? true} onClose={dong} onDeNghi={so.deNghiBatDoi} onDongY={() => { if (deNghiBatDoi) void so.dongYDeNghi(deNghiBatDoi.id).then((ok) => ok && dong()); }} open={mo === "bat-doi"} tenNguoiKia={so.tenNguoiKia} />
      {/* Choosing «Một đôi» opens the rung's own sheet (what it allows, what it
          does not pull along) instead of filing the proposal on one tap. */}
      <LoaiSo batDoi={so.batDoi} dangCho={deNghiBatDoi !== undefined} deNghiCuaToi={deNghiBatDoi?.cuaToi ?? true} onChonBan={so.thuHoiBatDoi} onChonDoi={() => { if (!so.batDoi && deNghiBatDoi === undefined) setMo("bat-doi"); }} onClose={dong} onDongY={deNghiBatDoi && !deNghiBatDoi.cuaToi ? () => void so.dongYDeNghi(deNghiBatDoi.id).then((ok) => ok && dong()) : undefined} open={mo === "loai-so"} tenNguoiKia={so.tenNguoiKia} />
      <RangBuoc dangLuu={so.dangLam?.includes("rang-buoc") ?? false} loi={mo === "rang-buoc" ? so.loiLenh : null} nguoiKia={so.rangBuoc.nguoiKia} onClose={dong} onLuu={(rb) => void so.datRangBuoc(rb).then((ok) => ok && dong())} open={mo === "rang-buoc"} tenNguoiKia={so.tenNguoiKia} toi={so.rangBuoc.toi} />
      {/* Chỉ tồn tại khi có cả việc lẫn tờ. Bản trước mount vô điều kiện và
          rơi về chuỗi rỗng khi thiếu một trong hai — không tới được hôm nay,
          nhưng hình dạng hỏng của nó là một tờ xác nhận huỷ MỞ RA với hậu quả
          trống và một nút không làm gì. */}
      {viec !== null && toMo ? (
        <XacNhanViec
          hauQua={HAU_QUA[viec](so.tenNguoiKia, phienBan(toMo)?.content.ngay ?? "")}
          nhanLam={NHAN_LAM[viec]}
          nguyHiem={viec === "bo" || viec === "huy"}
          onClose={() => setViec(null)}
          onXacNhan={() => {
            if (viec === "bo") so.boNhap(toMo.id);
            else if (viec === "rut") so.rut(toMo.id);
            else if (viec === "nghi_tuan") so.nghiTuan(toMo.id);
            else so.huy(toMo.id);
            setViec(null);
          }}
          open
          testID="xac-nhan-viec"
          tieuDe={TIEU_DE[viec]}
        />
      ) : null}
      <AiLoTuanNay dangLam={so.dangLam?.startsWith("vai:") ?? false} onChon={(lo) => so.chonLo(lo)} onClose={dong} open={mo === "vai"} tenNguoiKia={so.tenNguoiKia} toiId={so.toiId} vai={so.vai} />
      <GuHaiBan dangLam={so.dangLam?.includes("chia_gu") ?? false} gu={so.gu} onBat={so.chiaGu} onClose={dong} onSuaGuCuaToi={() => { dong(); router.push("/personalization" as never); }} onTat={so.thoiChiaGu} open={mo === "gu"} tenNguoiKia={so.tenNguoiKia} guChat={guChat} onBatLai={so.batLaiChiaGu} loi={so.loiGu} />
      <DongSo onClose={dong} onDong={() => { if (xemTruoc) { so.dongSo(xemTruoc.revision); dong(); } }} open={mo === "dong-so"} xemTruoc={xemTruoc} />
        </>
      }
      header={
        <TopBar
          back
          right={
            <View style={styles.dauPhai}>
              <IconButton accessibilityLabel="Cài đặt sổ" icon="settings-outline" onPress={() => setMo("cai-dat")} quiet />
            </View>
          }
          subtitle={so.loiDoc !== null ? so.tenNguoiKia : so.batDoi ? `Một đôi · ${so.tenNguoiKia}` : `Hội bạn · ${so.tenNguoiKia}`}
          title="Tờ giấy của hai mình"
        />
      }
      testID="khong-gian-giay"
    >
      <View style={[styles.than, { gap: space.lg }]}>
        {/* The waiting state below already says what `paper_wrong_state` meant;
            the server's sentence above it said the same thing worse (QC 24/09). */}
        {so.loiLenh && so.loiGu === null && !so.xinToBiChan && !so.daDung ? (
          <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.warn }]} testID="loi-lenh-so">
            {so.loiLenh}
          </Text>
        ) : null}
        {cauVai ? (
          <Pressable accessibilityHint="Đổi ai lo tuần này" accessibilityRole="button" onPress={() => setMo("vai")} style={styles.vai} testID="giay-ai-lo">
            <Text style={[typography.label, { color: colors.ink }]}>{cauVai.nhan}</Text>
            <Text style={[typography.caption, { color: colors.inkSoft }]}>{`${cauVai.vi} · Đổi`}</Text>
          </Pressable>
        ) : null}
        {cauGoiY ? (
          <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.inkSoft }]} testID="giay-goi-y-cho">
            {cauGoiY}
          </Text>
        ) : null}
        {choVaoKeo?.lam === "them-vao-buoi" ? (
          <RudiButton accessibilityLabel={tenChoVaoKeo ? `Thêm ${tenChoVaoKeo} vào buổi đã hẹn` : "Thêm chỗ này vào buổi đã hẹn"} icon="add-circle-outline" label="Thêm vào buổi đã hẹn" onPress={() => router.push(`/outings/chon?place=${encodeURIComponent(choVaoKeo.id)}` as never)} variant="outline" />
        ) : null}
        {choVaoKeo?.lam === "tao-keo" ? (
          // The place named, where it goes, and the one way there (QA UI-130).
          <View style={{ gap: space.sm }} testID="giay-goi-y-keo">
            <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.ink }]}>
              {coNutRuHoi
                ? `${tenChoVaoKeo ?? "Chỗ bạn chọn"} đi vào kèo của hai bạn, làm chặng đầu: chạm «Rủ hội mình đi chơi».`
                : `${tenChoVaoKeo ?? "Chỗ bạn chọn"} đi vào một kèo của hai bạn, làm chặng đầu.`}
            </Text>
            {coNutRuHoi ? null : (
              <RudiButton label={tenChoVaoKeo ? `Tạo kèo ở ${tenChoVaoKeo}` : "Tạo kèo với chỗ này"} onPress={() => router.push(ruHoi as never)} />
            )}
          </View>
        ) : null}
        {than}
        {cauGuSo?.chung ? (
          <Pressable accessibilityRole="button" onPress={() => setMo("gu")} testID="giay-gu-chung">
            <Text style={[typography.caption, { color: colors.inkSoft }]}>{cauGuSo.chung}</Text>
          </Pressable>
        ) : null}
        {/* After the evening: a photo of it goes into the pair's own memories
            (ADR-0021 §2.5), beside the one line the sheet keeps. Before 24/09
            a memory could only go to a group's wall. */}
        {toMo && (toMo.state === "da_di" || toMo.state === "da_giu") ? (
          <RudiButton
            icon="camera-outline"
            label="Giữ một tấm ảnh của buổi này"
            onPress={() => router.push(`/moments/new?ctx=${contextId}` as never)}
            variant="outline"
          />
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

/**
 * Mỗi việc một câu nói nó làm gì, cho ai — không phải «bạn có chắc không?».
 *
 * Tên người kia có mặt ở đây vì hậu quả rơi lên họ: «Ca cũng thấy buổi biến
 * mất» là thứ làm người đang bấm dừng lại, còn «không lấy lại được» thì không.
 */
const TIEU_DE: Record<"bo" | "rut" | "nghi_tuan" | "huy", string> = {
  bo: "Bỏ bản phác này?",
  rut: "Rút lại tờ đã gửi?",
  nghi_tuan: "Tuần này nghỉ?",
  huy: "Huỷ buổi đã chốt?",
};

const NHAN_LAM: Record<"bo" | "rut" | "nghi_tuan" | "huy", string> = {
  bo: "Bỏ bản phác",
  rut: "Rút lại",
  nghi_tuan: "Nghỉ tuần này",
  huy: "Huỷ buổi này",
};

const HAU_QUA: Record<"bo" | "rut" | "nghi_tuan" | "huy", (ten: string, ngay: string) => string> = {
  bo: () => "Những gì bạn vừa viết mất đi. Người ấy chưa từng thấy tờ này, nên không ai được báo.",
  rut: (ten) => `Tờ biến khỏi màn của ${ten || "người ấy"}. Muốn đổi ý thì gửi một tờ mới.`,
  nghi_tuan: (ten) => `Tuần này hai bạn không hẹn gì. ${ten || "Người ấy"} cũng thấy tuần này khép lại.`,
  huy: (ten, ngay) =>
    `${ngay ? `${ngayDocDuoc(ngay)} không còn. ` : ""}${ten || "Người ấy"} đã đồng ý buổi này và cũng thấy nó biến mất. Không lấy lại được.`,
};

function dongTom(t: ToGiay): string {
  const pb = phienBan(t);
  const chinh = pb?.content.chang[0];
  const giu = t.keeps[0]?.line;
  if (giu) return `Giữ lại: ${giu}`;
  return chinh ? `${chinh.gio} ${chinh.viec}` : "Tờ trống";
}

const styles = StyleSheet.create({
  dauPhai: { alignItems: "center", flexDirection: "row", gap: 4 },
  than: { paddingTop: 8 },
  canh: { gap: 16 },
  hangCanh: { flexDirection: "row", alignItems: "flex-end", justifyContent: "center", gap: 12 },
  giuaNgang: { alignSelf: "center" },
  toTrong: { flex: 1, maxWidth: 240, transform: [{ rotate: "-2deg" }] },
  lineStamp: { lineHeight: 18, marginBottom: 6 },
  // A pencil line waiting for Nếp's draft: a rule in lineStrong, dashed.
  dongChi: { height: 14, borderBottomWidth: 1, borderStyle: "dashed" },
  vai: { gap: 2, paddingVertical: 4 },
  footer: { paddingHorizontal: 16 },
  dev: { borderWidth: StyleSheet.hairlineWidth, borderStyle: "dashed", borderRadius: 10, padding: 8, gap: 0 },
});
