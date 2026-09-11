/**
 * One kèo on a real session: its stops in the group's chosen order with who has
 * arrived at each, a sheet to add a stop (optionally on a catalogue place),
 * and «Đánh dấu tôi đã tới» per stop. A stop with a place opens that place;
 * a stop without one opens the picker to attach one, so every row goes
 * somewhere.
 *
 * ## The plan is a route, not a list (UI v2, đợt 4)
 *
 * The hour sits on a left axis, one continuous ink line runs through the
 * stops, the stop itself is on the right (`HangChang`). Ink is what the
 * group holds; while an unsaved reorder draft exists the line turns to pencil
 * dashes and says so in words. Editors (a new stop, attaching a place) open
 * as a bottom sheet over the route instead of unfolding a form in the middle
 * of it, so the evening stays readable while you edit it. The budget figures
 * are `Money` at their own size and wrap when the window is narrow; nothing
 * shrinks a sum to fit a row.
 */
import { Redirect, useFocusEffect, useLocalSearchParams, useRouter } from "expo-router";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { ScrollView, StyleSheet, Text, useWindowDimensions, View } from "react-native";

import { ApiError, newAttempt, thongDiepNguoiDoc, type Attempt } from "../../../api";
import type { Phien } from "../../../phien";
import type { Place } from "../../../screens/kham-pha/places";
import {
  nhanKhoangNgay,
  sapXepChang,
  nhomCheckInTheoChang,
  tongDuKien,
  type BuoiDi,
  type ChangDung,
  type CheckIn,
} from "../../../screens/len-plan/buoi-di";
import { tabBarHeight } from "../../adaptive";
import { docDanhMuc } from "../../kham-pha/dia-diem";
import {
  cauDaToi,
  changGuiTu,
  cauSoChang,
  danhDauToi,
  docDaToi,
  docKeoCuaNhom,
  ganDiaDiem,
  gioTiepTheo,
  kiemTraChangMoi,
  luuLichTrinh,
  reconcileOrder,
  themChang,
} from "../../keo/keo";
import { homNay, nhanNhip, nhipKeo } from "../../keo/nhip-keo";
import { typography, useRudiTheme } from "../../theme";
import { Chip, Field, IconButton, RudiButton, RudiScreen, SectionHeader, TopBar } from "../../ui";
import { ErrorState } from "../../ui/ErrorState";
import { Money } from "../../ui/Money";
import { ReorderList } from "../../ui/ReorderList";
import { Sheet } from "../../ui/Sheet";
import { SkeletonGroup, SkeletonLines, SkeletonRow } from "../../ui/Skeleton";
import { HangChang } from "./HangChang";
import { chieuTuChang, ganMappedTheoId } from "../../hanh-trinh/chieu";
import { useCheDoLichTrinh } from "../../hanh-trinh/che-do";
import { ManHinhHanhTrinh } from "../../hanh-trinh/ManHinhHanhTrinh";
import { ThanhCheDo } from "../../hanh-trinh/ThanhCheDo";
import { toiUuGanNhat } from "../../hanh-trinh/toi-uu";

type Trang =
  | { pha: "dang-doc" }
  | { pha: "xong"; keo: BuoiDi; daToi: CheckIn[] }
  | { pha: "hong"; loi: string };

/** A route param is a string or nothing. */
function thamSoChuoi(v: unknown): string {
  if (typeof v === "string") return v;
  return "";
}

/** The picked place's key, or nothing when none is picked. */
function idNeuCo(place: Place | null): string | null {
  if (place === null) return null;
  return place.id;
}

function tenNeuCo(place: Place | null): string | null {
  if (place === null) return null;
  return place.name;
}

function loiRaChu(error: unknown): string {
  return error instanceof ApiError ? error.message : thongDiepNguoiDoc(0, null);
}

/** The second line of a stop: where, or what is still missing, in words. */
function dongDiaDiem(stop: ChangDung): { chu: string; tone: "accent" | "inkFaint" } {
  if (stop.place_id === null) return { chu: "Chọn địa điểm", tone: "inkFaint" };
  if (stop.place_name === null || stop.place_name === stop.label) return { chu: "Mở địa điểm", tone: "accent" };
  return { chu: stop.place_name, tone: "accent" };
}

export function OutingLiveScreen({ phien }: { phien: Phien }) {
  const router = useRouter();
  const params = useLocalSearchParams<{ id?: string }>();
  const { colors } = useRudiTheme();
  const { fontScale } = useWindowDimensions();
  const outingId = thamSoChuoi(params.id);
  const [trang, setTrang] = useState<Trang>({ pha: "dang-doc" });
  const [thongBao, setThongBao] = useState<string | null>(null);
  const [dangGhi, setDangGhi] = useState(false);
  const [moThem, setMoThem] = useState(false);
  const [gio, setGio] = useState(gioTiepTheo());
  const [nhan, setNhan] = useState("");
  const [danhMuc, setDanhMuc] = useState<Place[]>([]);
  const [choDiaDiem, setChoDiaDiem] = useState<Place | null>(null);
  const [ganChoChang, setGanChoChang] = useState<ChangDung | null>(null);
  const [draft, setDraft] = useState<{ stops: ChangDung[]; revision: number } | null>(null);
  const [dragging, setDragging] = useState(false);
  const [conflict, setConflict] = useState(false);
  const writing = useRef(false);
  const retry = useRef<{ key: string; attempt: Attempt } | null>(null);
  const contextId = phien.context_id;
  const che = useCheDoLichTrinh();
  const hanhTrinh = che.cheDo === "hanh-trinh";

  const nap = useCallback(async () => {
    if (contextId === null || !outingId) return;
    try {
      const keo = (await docKeoCuaNhom(contextId, phien.person_id)).find((k) => k.id === outingId);
      if (keo === undefined) {
        setTrang({ pha: "hong", loi: "Kèo này không còn trong nhóm." });
        return;
      }
      const daToi = await docDaToi(keo, phien.person_id);
      setTrang({ pha: "xong", keo, daToi });
    } catch (error) {
      setTrang({ pha: "hong", loi: loiRaChu(error) });
    }
  }, [contextId, outingId, phien.person_id]);

  useFocusEffect(
    useCallback(() => {
      void nap();
    }, [nap]),
  );

  const napDanhMuc = useCallback(async () => {
    if (danhMuc.length > 0) return;
    try {
      setDanhMuc((await docDanhMuc()).places);
    } catch (error) {
      setThongBao(loiRaChu(error));
    }
  }, [danhMuc.length]);

  const theoChang = useMemo<Record<string, CheckIn[]>>(
    () => (trang.pha === "xong" ? nhomCheckInTheoChang(trang.daToi) : {}),
    [trang],
  );

  useEffect(() => {
    if (hanhTrinh) void napDanhMuc();
  }, [hanhTrinh, napDanhMuc]);

  const cho = useMemo(
    () => danhMuc.map((p) => ({ id: p.id, name: p.name, lat: p.lat, lng: p.lng, address: p.address })),
    [danhMuc],
  );
  const stopsHien = trang.pha === "xong" ? (draft?.stops ?? trang.keo.stops) : [];
  const hanh = useMemo(() => chieuTuChang(stopsHien, cho), [stopsHien, cho]);

  if (contextId === null) return <Redirect href="/(tabs)/plan" />;

  const ghiLichTrinh = async (keo: BuoiDi, stops: ReturnType<typeof themChang>) => {
    if (writing.current) return false;
    writing.current = true;
    const key = JSON.stringify([keo.id, keo.timeline_revision, stops]);
    if (retry.current?.key !== key) retry.current = { key, attempt: newAttempt() };
    setDangGhi(true);
    setThongBao(null);
    try {
      const moi = await luuLichTrinh(keo, stops, phien.person_id, retry.current.attempt);
      setTrang({ pha: "xong", keo: moi, daToi: trang.pha === "xong" ? trang.daToi : [] });
      retry.current = null;
      setConflict(false);
      return true;
    } catch (error) {
      if (error instanceof ApiError && error.code === "timeline_conflict") setConflict(true);
      setThongBao(loiRaChu(error));
      return false;
    } finally {
      writing.current = false;
      setDangGhi(false);
    }
  };

  const themChangMoi = async (keo: BuoiDi) => {
    const kq = kiemTraChangMoi(gio, nhan);
    if (!kq.ok) {
      setThongBao(kq.loi);
      return;
    }
    const ok = await ghiLichTrinh(
      keo,
      themChang(keo.stops, {
        at: gio.trim(),
        label: nhan.trim(),
        place_name: tenNeuCo(choDiaDiem),
        place_id: idNeuCo(choDiaDiem),
      }),
    );
    if (ok) {
      setMoThem(false);
      setNhan("");
      setChoDiaDiem(null);
      setGio(gioTiepTheo());
    }
  };

  const ganChang = async (keo: BuoiDi, stop: ChangDung, place: Place) => {
    const ok = await ghiLichTrinh(keo, ganDiaDiem(keo.stops, stop.id, place));
    if (ok) setGanChoChang(null);
  };

  const daToiChang = async (keo: BuoiDi, stop: ChangDung) => {
    setDangGhi(true);
    setThongBao(null);
    try {
      await danhDauToi(stop.id, keo.context_id, phien.person_id, newAttempt());
      setTrang({ pha: "xong", keo, daToi: await docDaToi(keo, phien.person_id) });
    } catch (error) {
      setThongBao(loiRaChu(error));
    } finally {
      setDangGhi(false);
    }
  };

  const moHang = (stop: ChangDung) => {
    if (stop.place_id !== null) {
      router.push(`/places/${stop.place_id}` as never);
      return;
    }
    setGanChoChang(stop);
    void napDanhMuc();
  };

  const moThemChang = () => {
    setMoThem((v) => !v);
    void napDanhMuc();
  };

  // Editors live over the route, outside the scroll box (`RudiScreen overlay`).
  const overlay =
    trang.pha === "xong" ? (
      <>
        <Sheet accessibilityLabel="Chặng mới" onClose={() => setMoThem(false)} open={moThem}>
          <View style={styles.khay}>
            <Text style={[typography.h2, { color: colors.ink }]}>Chặng mới</Text>
            <View style={styles.hang}>
              <View style={styles.oGio}>
                <Field accessibilityLabel="Ô giờ chặng" icon="time-outline" keyboardType="numbers-and-punctuation" label="Giờ" onChangeText={setGio} value={gio} />
              </View>
              <View style={styles.flex}>
                <Field accessibilityLabel="Ô tên chặng" icon="flag-outline" label="Chặng" onChangeText={setNhan} placeholder="Ví dụ: Ăn tối" value={nhan} />
              </View>
            </View>
            <Text style={[typography.caption, { color: colors.inkSoft }]}>Địa điểm trong danh mục (tuỳ chọn)</Text>
            {/* One scrolling row: at font 1.3 the catalogue wrapped into eight rows and pushed the submit off-screen. */}
            <ScrollView contentContainerStyle={styles.hangChip} horizontal keyboardShouldPersistTaps="handled" showsHorizontalScrollIndicator={false}>
              {danhMuc.map((p) => (
                <Chip
                  key={p.id}
                  label={p.name}
                  onPress={() => setChoDiaDiem(choDiaDiem !== null && choDiaDiem.id === p.id ? null : p)}
                  selected={choDiaDiem !== null && choDiaDiem.id === p.id}
                />
              ))}
            </ScrollView>
            {thongBao !== null && moThem ? <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.warn }]}>{thongBao}</Text> : null}
            <RudiButton disabled={dangGhi} label="Thêm chặng" loading={dangGhi} onPress={() => void themChangMoi(trang.keo)} />
          </View>
        </Sheet>
        <Sheet accessibilityLabel="Gắn địa điểm" onClose={() => setGanChoChang(null)} open={ganChoChang !== null}>
          <View style={styles.khay}>
            <Text style={[typography.h2, { color: colors.ink }]}>Gắn địa điểm cho «{ganChoChang?.label ?? ""}»</Text>
            <ScrollView contentContainerStyle={styles.hangChip} horizontal keyboardShouldPersistTaps="handled" showsHorizontalScrollIndicator={false}>
              {danhMuc.map((p) => (
                <Chip key={p.id} label={p.name} onPress={() => { if (ganChoChang !== null) void ganChang(trang.keo, ganChoChang, p); }} />
              ))}
            </ScrollView>
            <RudiButton label="Để sau" onPress={() => setGanChoChang(null)} variant="ghost" />
          </View>
        </Sheet>
      </>
    ) : null;

  const toiUu = async () => {
    if (trang.pha !== "xong" || draft !== null) return;
    const mapped = hanh.activities.filter((a) => a.lat !== null && a.lng !== null);
    if (mapped.length < 2) return;
    const ids = toiUuGanNhat(mapped.map((a) => ({ id: a.id, lat: a.lat as number, lng: a.lng as number })));
    // `at` belongs to the slot, not to the stop: reordering by geography must
    // not make the evening run backwards. See ganMappedTheoId.
    await ghiLichTrinh(trang.keo, ganMappedTheoId(trang.keo.stops, ids, "at").map(changGuiTu));
  };

  return (
    <RudiScreen
      contentStyle={hanhTrinh ? styles.mapInner : undefined}
      header={
        <View style={styles.dauMan}>
          <TopBar
            right={
              trang.pha === "xong" ? (
                <IconButton
                  accessibilityLabel="Thành viên nhóm"
                  icon="people-outline"
                  onPress={() => router.push(`/groups/${trang.keo.context_id}/members` as never)}
                  quiet
                />
              ) : undefined
            }
            title="Kèo"
          />
          {trang.pha === "xong" ? (
            <Text style={[typography.caption, { color: colors.inkSoft }]} numberOfLines={1}>
              {trang.keo.title}
            </Text>
          ) : null}
          {trang.pha === "xong" ? <ThanhCheDo cheDo={che.cheDo} onDoi={che.doiCheDo} /> : null}
        </View>
      }
      overlay={overlay}
      padded={!hanhTrinh}
      scroll={!hanhTrinh && !dragging}
      testID="outing-screen"
    >
      {trang.pha === "dang-doc" ? (
        <SkeletonGroup style={styles.khung}>
          <SkeletonLines lastWidth="40%" lineHeight={22} lines={2} />
          <SkeletonRow leading={0} />
          <SkeletonRow leading={0} />
        </SkeletonGroup>
      ) : null}
      {trang.pha === "hong" ? (
        <ErrorState body={trang.loi} onRetry={() => void nap()} secondary={{ label: "Về Lên plan", onPress: () => router.back() }} title="Chưa mở được kèo" />
      ) : null}
      {trang.pha === "xong" && hanhTrinh ? (
        <ManHinhHanhTrinh
          dangToiUu={dangGhi}
          fitDem={che.fitDem}
          hanh={hanh}
          onChonDoan={che.chonDoan}
          onChonMoc={che.chonHoatDong}
          onKhop={che.khopHanhTrinh}
          onNen={() => {
            che.chonHoatDong(null);
            che.chonDoan(null);
          }}
          onToiUu={draft ? undefined : () => void toiUu()}
          onUserMove={che.userMove}
          onVeLichTrinh={() => che.doiCheDo("lich-trinh")}
          chanDuoi={tabBarHeight(fontScale)}
          selectedActivityId={che.selectedActivityId}
          selectedSegmentId={che.selectedSegmentId}
          toiDem={che.toiDem}
        />
      ) : null}
      {trang.pha === "xong" && !hanhTrinh ? (
        <>
          <View style={styles.dau}>
            <Text style={[typography.h1, { color: colors.ink }]}>{trang.keo.title}</Text>
            <Text style={[typography.body, { color: colors.inkSoft }]}>
              {nhanKhoangNgay(trang.keo.starts_on, trang.keo.ends_on)} · {trang.keo.headcount} người
              {nhanNhip(nhipKeo(trang.keo.starts_on, trang.keo.ends_on, homNay())) ? ` · ${nhanNhip(nhipKeo(trang.keo.starts_on, trang.keo.ends_on, homNay()))}` : ""}
            </Text>
            {/* Two sums, side by side while the window allows, one under the other
                when it does not. Neither is ever shrunk to fit. */}
            <View style={styles.tien}>
              <View style={styles.oTien}>
                <Money vnd={trang.keo.budget_per_person_vnd} />
                <Text style={[typography.caption, { color: colors.inkSoft }]}>một người</Text>
              </View>
              <View style={styles.oTien}>
                <Money vnd={tongDuKien(trang.keo.budget_per_person_vnd, trang.keo.headcount)} />
                <Text style={[typography.caption, { color: colors.inkSoft }]}>cả kèo, {trang.keo.headcount} người</Text>
              </View>
            </View>
          </View>
          <SectionHeader
            action={draft ? undefined : moThem ? "Đóng" : "Thêm chặng"}
            onAction={moThemChang}
            title={cauSoChang(trang.keo.stops.length)}
          />
          {thongBao !== null && !moThem ? <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.warn }]}>{thongBao}</Text> : null}
          {conflict ? <RudiButton label="Tải bản mới để đối chiếu" variant="outline" onPress={() => void nap()} /> : null}
          {trang.keo.stops.length > 0 ? (
            <View style={styles.danhSach}>
              <Text style={[typography.caption, { color: colors.inkFaint }]}>Giữ tay nắm bên phải rồi kéo để đổi thứ tự. Giờ hẹn không đổi khi bạn đổi thứ tự.</Text>
              <ReorderList items={draft?.stops ?? trang.keo.stops} itemKey={(stop) => stop.id} label={(stop) => stop.label}
                disabled={dangGhi || moThem || ganChoChang !== null} onDragging={setDragging}
                onChange={(stops) => setDraft({ stops, revision: draft?.revision ?? trang.keo.timeline_revision })}
                renderItem={(stop, i) => {
                  const daToi = theoChang[stop.id] ?? [];
                  const toiRoi = daToi.some((c) => c.person_id === phien.person_id);
                  const dong = dongDiaDiem(stop);
                  const soChang = (draft?.stops ?? trang.keo.stops).length;
                  return (
                    <HangChang
                      accessibilityLabel={`Chặng ${stop.label}`}
                      chon={che.selectedActivityId === stop.id}
                      cuoi={i === soChang - 1}
                      daToi={toiRoi}
                      ghiChu={cauDaToi(daToi, phien.person_id)}
                      gio={stop.at}
                      onPress={() => {
                        if (che.selectedActivityId === stop.id) {
                          moHang(stop);
                          return;
                        }
                        che.chonHoatDong(stop.id);
                        if (stop.place_id === null) moHang(stop);
                      }}
                      phac={draft !== null}
                      phai={
                        toiRoi ? (
                          // Arrived is a fact, not a control that went grey: a static badge.
                          <Chip icon="checkmark" label="Đã tới" selected tone="split" />
                        ) : (
                          <RudiButton compact disabled={dangGhi} full={false} label="Tôi đã tới" onPress={() => void daToiChang(trang.keo, stop)} variant="outline" />
                        )
                      }
                      phu={dong.chu}
                      phuTone={dong.tone}
                      tieuDe={stop.label}
                    />
                  );
                }} />
              {draft ? <View style={styles.form}>
                <Text style={[typography.caption, { color: colors.inkSoft }]}>Thứ tự nháp · chưa lưu lên nhóm</Text>
                {draft.revision !== trang.keo.timeline_revision ? <>
                  <Text style={[typography.body, { color: colors.inkSoft }]}>Bản mới: {trang.keo.stops.map((stop) => stop.label).join(" → ")}</Text>
                  <RudiButton label="Ghép thứ tự nháp vào bản mới" variant="outline" onPress={() => {
                    setDraft({ stops: reconcileOrder(draft.stops, trang.keo.stops), revision: trang.keo.timeline_revision });
                    retry.current = null;
                    setConflict(false);
                    setThongBao("Đã ghép thứ tự. Giữ giờ và nội dung mới của nhóm; chặng mới nằm cuối. Kiểm tra rồi lưu.");
                  }} />
                </> : null}
                <RudiButton label="Lưu thứ tự" loading={dangGhi} disabled={conflict || draft.revision !== trang.keo.timeline_revision}
                  onPress={() => void ghiLichTrinh({ ...trang.keo, timeline_revision: draft.revision }, draft.stops.map(changGuiTu)).then((ok) => { if (ok) setDraft(null); })} />
                <RudiButton label="Bỏ thứ tự nháp" variant="ghost" disabled={dangGhi}
                  onPress={() => { setDraft(null); setConflict(false); setThongBao(null); }} />
              </View> : <RudiButton label="Xếp theo giờ hẹn" variant="ghost" disabled={dangGhi}
                onPress={() => setDraft({ stops: sapXepChang(trang.keo.stops), revision: trang.keo.timeline_revision })} />}
            </View>
          ) : (
            <Text style={[typography.body, { color: colors.inkSoft }]}>
              Bấm «Thêm chặng», hoặc mở một địa điểm ở Khám phá rồi «Thêm vào kèo».
            </Text>
          )}
        </>
      ) : null}
    </RudiScreen>
  );
}

const styles = StyleSheet.create({
  hangChip: { flexDirection: "row", gap: 6, paddingRight: 8 },
  flex: { flex: 1 },
  // The map is the page here: it runs to the bottom edge and the journey
  // panel keeps its own clearance over the tab bar.
  mapInner: { flex: 1, paddingBottom: 0 },
  dauMan: { gap: 8, paddingBottom: 8 },
  khung: { gap: 14 },
  dau: { gap: 8 },
  tien: { flexDirection: "row", flexWrap: "wrap", gap: 24, marginTop: 4 },
  oTien: { gap: 2, minWidth: 140 },
  khay: { gap: 12, paddingBottom: 4 },
  form: { gap: 12 },
  hang: { flexDirection: "row", gap: 10 },
  oGio: { width: 118 },
  danhSach: { paddingVertical: 4, gap: 10 },
});
