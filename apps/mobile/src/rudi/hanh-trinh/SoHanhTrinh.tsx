/** A day's map and editable trip notebook share one revision-bound draft. */
import { useEffect, useMemo, useRef, useState } from "react";
import { AppState, Platform, ScrollView, StyleSheet, Switch, Text, View } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import type { BuoiDi } from "../../screens/len-plan/buoi-di";
import { ApiError, luuHanhTrinh, newAttempt, xemTruocHanhTrinh, type Attempt } from "../../api";
import { typography, useRudiTheme } from "../theme";
import { Chip, RudiButton } from "../ui";
import { ONhapMuc } from "../ui/ONhapMuc";
import { CauTaiCho } from "../ui/CauTaiCho";
import { LenLop } from "../ui/KheLop";
import { MoNgang } from "../ui/MoNgang";
import { Sheet } from "../ui/Sheet";
import { ManHinhHanhTrinh } from "./ManHinhHanhTrinh";
import { useCheDoLichTrinh } from "./che-do";
import { chieuTuChang } from "./chieu";
import type { ChoChieu, ToaDo } from "./mo-hinh";
import { apDungTuyen, changChuaXep, chuNgay, doiViTri, ngayMacDinh, nhapTuKeo, suaChang, trangThaiTuyen, xepVaoNgay, xoaChang, type BanNhap, type ChangDi, type NgayDi, type XemTruoc } from "./ke-hoach";
import { chuKhoangCach, chuThoiGian } from "./tom-tat";

/**
 * The gesture bar's height. On the QA emulator (Android 16, gesture nav) the
 * insets hook reported 0 under this edge-to-edge page, and the home indicator
 * was drawn across «Xem cách đi gọn hơn» (2026-09-29): never less than this.
 */
const CHAN_CU_CHI = Platform.OS === "web" ? 0 : 24;
const PHUONG_TIEN = { motorbike: "XE MÁY", car: "Ô TÔ", walk: "ĐI BỘ" } as const;
/** Local calendar date, the one a person means by «hôm nay». */
function homNay(): string {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

type Props = { outing: BuoiDi; places: ChoChieu[]; actorId?: string; onSaved: (outing: BuoiDi) => void; onReload?: () => Promise<void>; onTimeline: () => void; bottom?: number; fixture?: boolean; controller?: ReturnType<typeof useCheDoLichTrinh>; initialDay?: string; onDay?: (day: string) => void; daToiIds?: readonly string[]; dangTimCho?: boolean };
export function SoHanhTrinh({ outing, places, actorId, onSaved, onReload, onTimeline, bottom = 0, fixture = false, controller, initialDay, onDay, daToiIds = [], dangTimCho = false }: Props) {
  const { colors } = useRudiTheme();
  const insets = useSafeAreaInsets();
  const [draft, setDraft] = useState(() => nhapTuKeo(outing));
  const [day, setDay] = useState(initialDay ?? outing.starts_on);
  const [preview, setPreview] = useState<XemTruoc | null>(null);
  const [suggestion, setSuggestion] = useState(false);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  // A refusal or a failure, in warn ink: in the inkSoft of «Đã giữ trang ngày
  // cho cả hội» a failed save read like a kept one (critique, B3).
  const [loi, setLoi] = useState<string | null>(null);
  const [editing, setEditing] = useState(false);
  const [stopId, setStopId] = useState<string | null>(null);
  const [pin, setPin] = useState<ToaDo | null>(null);
  const [pinName, setPinName] = useState("");
  const [placeQuery, setPlaceQuery] = useState("");
  const [undo, setUndo] = useState<BanNhap | null>(null);
  const request = useRef(0);
  const saving = useRef(false);
  const baseline = useRef(nhapTuKeo(outing));
  const [conflict, setConflict] = useState(false);
  const retry = useRef<{ key: string; attempt: Attempt } | null>(null);
  const localController = useCheDoLichTrinh();
  const che = controller ?? localController;
  useEffect(() => {
    const next = nhapTuKeo(outing);
    if (JSON.stringify(next) === JSON.stringify(baseline.current)) return;
    if (JSON.stringify(draft) === JSON.stringify(baseline.current)) setDraft(next);
    else if (next.expected_revision !== draft.expected_revision) {
      setConflict(true); setMessage("Hội vừa có bản mới. Bản nháp của bạn vẫn đang ở đây.");
    }
    baseline.current = next; setPreview(null); setSuggestion(false); request.current++;
    if (!saving.current) setBusy(false);
  }, [outing]);
  useEffect(() => { if (initialDay) setDay(initialDay); }, [initialDay]);
  const settings = draft.days.find((d) => d.day === day) ?? ngayMacDinh(day);
  const dirty = JSON.stringify(draft) !== JSON.stringify(nhapTuKeo(outing));
  const invalidate = (next: BanNhap) => { if (saving.current) return; request.current++; setDraft(next); setPreview(null); setSuggestion(false); setMessage(null); setLoi(null); setBusy(false); };
  const changeStop = (id: string, change: Partial<ChangDi>) => invalidate(suaChang(draft, id, change));
  const changeDay = (change: Partial<NgayDi>) => invalidate({ ...draft, days: [...draft.days.filter((d) => d.day !== day), { ...settings, ...change }] });
  const chooseDay = (date: string) => {
    if (saving.current) return;
    request.current++; setDay(date); onDay?.(date); setBusy(false); setPreview(null); setSuggestion(false); che.chonHoatDong(null); che.khopHanhTrinh();
  };
  const days = useMemo(() => {
    const result: string[] = []; const cursor = new Date(`${outing.starts_on}T12:00:00Z`); const end = new Date(`${outing.ends_on}T12:00:00Z`);
    for (let i = 0; i < 366 && cursor <= end; i++, cursor.setUTCDate(cursor.getUTCDate() + 1)) result.push(cursor.toISOString().slice(0, 10));
    return result;
  }, [outing.starts_on, outing.ends_on]);
  const inspect = async (recommend: boolean) => {
    if (saving.current) return;
    if (!actorId || fixture) { setMessage("Chưa đăng nhập nên chỉ xem được lịch trình."); return; }
    const sequence = ++request.current; setBusy(true); setMessage(null); setLoi(null); setPreview(null); setSuggestion(false);
    try {
      const result = await xemTruocHanhTrinh(outing.id, draft, day, recommend, actorId, outing.context_id);
      if (sequence !== request.current) return;
      setPreview(result); setSuggestion(false);
      if (recommend && result.suggestion) { che.chonHoatDong(null); che.khopHanhTrinh(); }
      if (recommend && !result.suggestion && result.status === "ready") setMessage("Chưa tìm được phương án tốt hơn với những giờ đã giữ.");
    } catch (error) { if (sequence === request.current) setLoi(error instanceof Error ? error.message : "Chưa lấy được tuyến. Thử lại khi có mạng."); }
    finally { if (sequence === request.current) setBusy(false); }
  };
  // A new transport mode is a question («what if we drive?»): answer it,
  // rather than dropping the map to a pencil draft until someone presses
  // «Tính lại đường» (emulator, 2026-09-29).
  useEffect(() => { void inspect(false); return () => { request.current++; }; }, [day, outing.id, draft.expected_revision, settings.transport_mode]);
  useEffect(() => {
    const subscription = AppState.addEventListener("change", (state) => { if (state === "active" && preview?.status !== "ready" && !dirty) void inspect(false); });
    return () => subscription.remove();
  }, [preview?.status, dirty, day]);
  const fitPoints = useMemo(() => {
    const routed = [preview?.current, preview?.suggestion].flatMap((r) => r?.segments.flatMap((s) => s.geometry.map(([lng, lat]) => ({ lat, lng }))) ?? []);
    if (routed.length) return routed;
    const located = chieuTuChang(draft.stops, places).activities.filter((a) => a.lat !== null && a.lng !== null);
    // An empty day keeps the outing's geographic context instead of jumping
    // to the default city, while displaying no stops from another day.
    if (located.some((a) => draft.stops.find((s) => s.id === a.id)?.day === day)) return [];
    return located.map((a) => ({ lat: a.lat!, lng: a.lng! }));
  }, [preview, draft, places, day]);
  const route = suggestion ? preview?.suggestion : preview?.current;
  const visible = useMemo(() => {
    let stops = draft.stops.filter((s) => s.day === day);
    if (route && suggestion) { const byId = new Map(stops.map((s) => [s.id, s])); stops = route.stops.flatMap((r) => { const s = byId.get(r.id); return s ? [{ ...s, at: r.at ?? s.at }] : []; }); }
    const projected = chieuTuChang(stops, places);
    if (route) projected.routeSegments = route.segments.map((s) => ({ id: `${s.from_stop_id}->${s.to_stop_id}`, fromActivityId: s.from_stop_id, toActivityId: s.to_stop_id, distanceMeters: s.distance_meters, durationSeconds: s.duration_seconds, transportMode: settings.transport_mode, polyline: s.geometry.map(([lng, lat]) => ({ lat, lng })), nguon: "valhalla" }));
    return projected;
  }, [draft, day, places, route, suggestion, settings.transport_mode]);
  const save = async (next: BanNhap, restoring = false) => {
    if (saving.current) return; saving.current = true; request.current++; setBusy(true); setMessage(null); setLoi(null);
    const previous = nhapTuKeo(outing);
    const key = JSON.stringify(next);
    if (retry.current?.key !== key) retry.current = { key, attempt: newAttempt() };
    try {
      const result = fixture ? { ...outing, stops: next.stops, days: next.days, timeline_revision: outing.timeline_revision + 1, itinerary_version: 2 as const } : await luuHanhTrinh(outing.id, next, actorId!, retry.current.attempt, outing.context_id);
      const canUndo = previous.stops.every((s) => result.stops.some((r) => r.id === s.id));
      request.current++; baseline.current = nhapTuKeo(result); setConflict(false);
      onSaved(result); setDraft(nhapTuKeo(result)); setUndo(restoring || !canUndo ? null : { ...previous, expected_revision: result.timeline_revision }); setPreview(null); setSuggestion(false); setEditing(false); retry.current = null; setMessage(restoring ? "Đã hoàn tác lần lưu vừa rồi." : "Đã giữ trang ngày cho cả hội.");
    } catch (error) { if (error instanceof ApiError && error.code === "timeline_conflict") setConflict(true); setLoi(error instanceof ApiError && error.code === "timeline_conflict" ? "Có người vừa sửa lịch trình. Tải bản mới để đối chiếu; bản nháp này chưa được ghi đè." : error instanceof Error ? error.message : "Chưa lưu được. Bạn có thể thử lại."); }
    finally { saving.current = false; setBusy(false); }
  };
  const editStop = draft.stops.find((s) => s.id === stopId);
  const anchorStop = (key: "start_stop_id" | "end_stop_id") => {
    if (!editStop?.day) return;
    const config = draft.days.find((d) => d.day === editStop.day) ?? ngayMacDinh(editStop.day);
    const selected = config[key] === editStop.id;
    const next = selected ? draft : doiViTri(draft, editStop.id, key === "start_stop_id" ? "first" : "last");
    const other = key === "start_stop_id" ? "end_stop_id" : "start_stop_id";
    invalidate({ ...next, days: [...next.days.filter((d) => d.day !== config.day), { ...config, [key]: selected ? null : editStop.id, [other]: config[other] === editStop.id ? null : config[other] }] });
  };
  const matches = places.filter((p) => p.name.toLocaleLowerCase("vi").includes(placeQuery.toLocaleLowerCase("vi"))).slice(0, 12);
  const addPoint = () => {
    if (saving.current || !pin || !pinName.trim() || draft.stops.length >= 50) return;
    const id = `tmp-${Date.now()}`;
    invalidate({ ...draft, days: draft.days.some((d) => d.day === day) ? draft.days : [...draft.days, settings], stops: [...draft.stops, { id, position: draft.stops.length, at: settings.start_at, label: pinName.trim(), place_name: pinName.trim(), place_id: null, day, duration_minutes: null, time_locked: true, meeting_point: { ...pin, label: pinName.trim() } }] });
    setPin(null); setPinName(""); setEditing(true); setStopId(id);
  };
  // How the group gets around decides the line on the map, so it sits in the
  // page head, right under the numbers it changes -- not at the bottom of a
  // scroll the primary button covered (emulator, 2026-09-29).
  // flexGrow/Shrink 0: inside the day page's capped column a horizontal
  // ScrollView was squeezed to the top edge of its chips (emulator, 1.0 and 1.3).
  const phuongTien = <ScrollView horizontal showsHorizontalScrollIndicator={false} style={{ flexGrow: 0, flexShrink: 0 }} contentContainerStyle={styles.row}>
    {(["motorbike", "car", "walk"] as const).map((mode) => <Chip key={mode} label={{ motorbike: "Xe máy", car: "Ô tô", walk: "Đi bộ" }[mode]} selected={settings.transport_mode === mode} onPress={() => changeDay({ transport_mode: mode })} />)}
  </ScrollView>;
  const coChang = visible.activities.length > 0;
  const coMocTrenBanDo = visible.activities.some((a) => a.lat !== null && a.lng !== null);
  // A phone browser has no right click; a long press there raises the same
  // `contextmenu` the map listens to (critique, B3: phones were told to right-click).
  const chuotPhai = Platform.OS === "web" && !(typeof window !== "undefined" && window.matchMedia?.("(hover: none) and (pointer: coarse)").matches);
  const actions = <View style={styles.stack}>
    {preview?.suggestion ? <>
      <View style={styles.row}><Chip label="Hiện tại" selected={!suggestion} onPress={() => setSuggestion(false)} /><Chip label="Gợi ý" selected={suggestion} onPress={() => setSuggestion(true)} /></View>
      {preview.savings ? <Text style={[typography.label, { color: colors.ink }]}>{preview.savings.duration_seconds === 0 ? "Thời gian di chuyển không đổi" : `${chuThoiGian(Math.abs(preview.savings.duration_seconds))} ${preview.savings.duration_seconds > 0 ? "ít di chuyển hơn" : "di chuyển thêm"}`} · {chuKhoangCach(Math.abs(preview.savings.distance_meters))} {preview.savings.distance_meters >= 0 ? "ngắn hơn" : "dài hơn"}</Text> : null}
      {suggestion ? preview.suggestion.stops.filter((s) => s.at !== draft.stops.find((d) => d.id === s.id)?.at).map((s) => <Text key={s.id} style={[typography.note, { color: colors.inkSoft }]}>{draft.stops.find((d) => d.id === s.id)?.label}: {draft.stops.find((d) => d.id === s.id)?.at} → {s.at}</Text>) : null}
    </> : null}
    {/* `empty_day` repeats what the empty page already says (critique, B3). */}
    {preview ? [...preview.issues, ...(route?.issues ?? [])].filter((v, i, all) => !v.code.startsWith("routing_") && v.code !== "empty_day" && all.findIndex((a) => a.code === v.code && a.stop_id === v.stop_id) === i).map((issue, i) => <Text key={i} style={[typography.note, { color: colors.inkSoft }]}>{issue.message}</Text>) : null}
    {route ? <Text style={[typography.caption, { color: colors.inkSoft }]}>Thời gian ước tính · chưa tính giao thông trực tiếp</Text> : null}
    {message ? <Text accessibilityLiveRegion="polite" style={[typography.note, { color: colors.inkSoft }]}>{message}</Text> : null}
    <CauTaiCho cau={loi} />
    {conflict ? <View style={styles.stack}>
      {onReload ? <RudiButton label="Tải bản mới để đối chiếu" variant="outline" disabled={busy} onPress={() => void onReload()} /> : null}
      {outing.timeline_revision !== draft.expected_revision ? <>
        <Text style={[typography.note, { color: colors.inkSoft }]}>Bản của hội: {outing.stops.map((s) => `${s.at} ${s.label}`).join(" → ")}</Text>
        <RudiButton label="Bỏ nháp, dùng bản mới" accessibilityLabel="Bỏ bản nháp, dùng bản của hội" variant="outline" disabled={busy} onPress={() => { invalidate(nhapTuKeo(outing)); setUndo(null); setConflict(false); }} />
      </> : null}
    </View> : null}
    {/* An empty day has no route to recompute. With a draft on a mapped day,
        saving is the pinned primary and the better-route question steps here. */}
    {/* An empty day's one action is to edit it: full width. A day with
        stops keeps the compact row of tools. */}
    {coChang ? <View style={styles.row}>
      <RudiButton compact full={false} label="Sửa trang ngày" variant="outline" onPress={() => { setStopId(che.selectedActivityId); setEditing(true); }} />
      <RudiButton compact full={false} label="Tính lại đường" variant="ghost" disabled={busy} lyDo={busy ? "Đang tính đường" : undefined} onPress={() => void inspect(false)} />
      {dirty && coMocTrenBanDo ? <RudiButton compact full={false} label="Xem cách đi gọn hơn" variant="outline" disabled={busy} lyDo={busy ? "Đang tính đường" : undefined} onPress={() => void inspect(true)} /> : null}
    </View> : <RudiButton compact label="Sửa trang ngày" variant="outline" onPress={() => { setStopId(che.selectedActivityId); setEditing(true); }} />}
    {dirty && !coMocTrenBanDo ? <RudiButton label="Lưu cho cả hội" disabled={busy} onPress={() => void save(draft)} /> : null}
    {undo ? <RudiButton label="Hoàn tác lần lưu vừa rồi" variant="ghost" disabled={busy} onPress={() => void save(undo, true)} /> : null}
  </View>;
  // Stops on none of the outing's days are on no day's map: named on the
  // page, and put on the day being viewed in one tap, as a draft the group
  // only sees once it is saved (QA UI-032).
  const chuaXep = changChuaXep(draft, days);
  const xepChuaXep = () => {
    if (saving.current) return;
    const n = chuaXep.length;
    invalidate(xepVaoNgay(draft, chuaXep.map((s) => s.id), day));
    che.khopHanhTrinh();
    setMessage(`Đã xếp ${n} chặng vào ${days.length > 1 ? `Ngày ${days.indexOf(day) + 1}` : "ngày này"}. Lưu để cả hội cùng thấy.`);
  };
  // The world's toggle, as in Cài đặt: a platform-grey track was the one
  // unthemed control left in the day editor (B3 finish review). `lineStrong`
  // off, so the track keeps 3:1 against the paper.
  // A held stop earlier than the start cannot be reached: the engine says
  // `late_fixed_stop` («Không kịp giờ đã ghim.») only after a preview, so the
  // editor says it under the field where the two times sit (B3 finish review).
  const phut = (t: string) => { const m = /^(\d{1,2}):(\d{2})$/.exec(t.trim()); return m ? Number(m[1]) * 60 + Number(m[2]) : null; };
  const batDau = phut(settings.start_at);
  const somNhat = batDau === null ? undefined : draft.stops.find((s) => s.day === day && s.time_locked && (phut(s.at) ?? Infinity) < batDau);
  const chuaKip = somNhat ? `«${somNhat.label}» giữ giờ ${somNhat.at}, sớm hơn giờ xuất phát nên sẽ không kịp. Đổi giờ xuất phát hoặc bỏ giữ giờ ở chặng đó.` : null;
  const congTac = { trackColor: { false: colors.lineStrong, true: colors.accent }, thumbColor: colors.card, ...(Platform.OS === "web" ? { activeThumbColor: colors.card } : {}) };
  const tuyen = trangThaiTuyen({ fixture, dangTinh: busy, coTuyen: Boolean(route), preview });
  const thuTuNgay = days.indexOf(day);
  const tieuDeTrang = days.length > 1 && thuTuNgay >= 0 ? `Ngày ${thuTuNgay + 1} · ${chuNgay(day)}` : chuNgay(day);
  return <View style={{ flex: 1 }}>
    {/* One day needs no picker: the page head already names it, and on a
        phone the lone «Ngày 1» row cost the map 56dp (review, 2026-09-29). */}
    {days.length > 1 ? <ScrollView horizontal style={{ flexGrow: 0 }} contentContainerStyle={[styles.row, { paddingHorizontal: 16, paddingVertical: 8 }]} showsHorizontalScrollIndicator={false}>{days.map((date, i) => <Chip key={date} label={`Ngày ${i + 1}`} selected={day === date} onPress={() => chooseDay(date)} />)}</ScrollView> : null}
    <ManHinhHanhTrinh nhuongNep={che.cheDo === "hanh-trinh"} goiYGhim={chuotPhai ? "Nhấp chuột phải trên bản đồ để thêm điểm hẹn." : "Giữ trên bản đồ để thêm điểm hẹn."} chuaXep={chuaXep.length ? { ten: chuaXep.map((s) => s.label), onXep: xepChuaXep } : undefined} hanh={visible} fitDem={che.fitDem + 1} cameraKey={`${day}:${route ? "routed" : "draft"}`} fitPoints={fitPoints} toiDem={che.toiDem} selectedActivityId={che.selectedActivityId} selectedSegmentId={che.selectedSegmentId} onChonMoc={che.chonHoatDong} onChonDoan={che.chonDoan} onNen={() => { che.chonHoatDong(null); che.chonDoan(null); }} onKhop={che.khopHanhTrinh} onUserMove={che.userMove} onVeLichTrinh={onTimeline} onGhim={(point) => { if (saving.current) return; if (draft.stops.length >= 50) { setMessage("Lịch trình đã đủ 50 chặng. Bỏ một chặng trước khi thêm điểm hẹn."); return; } setPin(point); setPinName(""); }} chanDuoi={Math.max(bottom, insets.bottom, CHAN_CU_CHI)} tuyen={tuyen} dangTimCho={dangTimCho} chonPhuongTien={phuongTien} phuongTien={PHUONG_TIEN[settings.transport_mode]} tieuDeTrang={tieuDeTrang} daToiIds={daToiIds} dangDi={!fixture && day === homNay()} neo={{ xuatPhat: settings.start_stop_id, ketThuc: settings.end_stop_id, veDiemDau: settings.return_to_start }} actions={actions} primaryAction={suggestion && preview?.suggestion ? <RudiButton disabled={busy || !preview.suggestion.feasible} label="Giữ phương án này" onPress={() => void save(apDungTuyen(draft, day, preview.suggestion!))} /> : dirty ? <RudiButton label="Lưu cho cả hội" loading={busy} disabled={busy} lyDo={busy ? "Đang lưu" : undefined} onPress={() => void save(draft)} /> : <RudiButton label="Xem cách đi gọn hơn" loading={busy} disabled={busy || !visible.activities.length} lyDo={!visible.activities.length ? "Ngày này chưa có chặng" : undefined} onPress={() => void inspect(true)} />} />
    {/* Over the whole screen, header included: drawn here, the scrim stopped
        under the outing's header and left it live (QA UI-041). */}
    <LenLop>
    <Sheet open={editing} onClose={() => setEditing(false)} accessibilityLabel="Sửa trang ngày"><View style={styles.editor}>
      {/* Titled by the button that opened it and the day it edits, so the
          person knows where they are. */}
      <Text style={[typography.h2, { color: colors.ink }]}>Sửa trang ngày</Text>
      <Text style={[typography.caption, { color: colors.inkSoft }]}>{tieuDeTrang}{dirty ? " · bản nháp, cả hội chưa thấy" : ""}</Text>
      <ONhapMuc label="Giờ xuất phát" error={chuaKip} value={settings.start_at} onChangeText={(start_at) => changeDay({ start_at })} />
      <View style={styles.row}><Switch {...congTac} accessibilityLabel="Quay về điểm đầu" value={settings.return_to_start} onValueChange={(return_to_start) => changeDay({ return_to_start })} /><Text style={[typography.body, { color: colors.ink }]}>Quay về điểm đầu</Text></View>
      {/* Each stop at most ~70% of the column, its name cut after the hour,
          the row out to the sheet's scroll box and faded at its cut end: one
          over-wide chip cut flush at the padding read as broken, not as a
          row that scrolls (B3 finish review). */}
      <View>
      <ScrollView horizontal showsHorizontalScrollIndicator={false} style={styles.tran} contentContainerStyle={styles.hangTran}>{draft.stops.map((s) => <View key={s.id} style={styles.oChip}><Chip label={`${s.at} · ${s.label}${s.day === null ? " · chưa xếp ngày" : ""}`} selected={s.id === stopId} onPress={() => setStopId(s.id)} /></View>)}</ScrollView>
      <MoNgang mau={colors.card} />
      </View>
      {editStop ? <View style={styles.stack}>
        <Text style={[typography.h2, { color: colors.ink }]}>{editStop.label}</Text>
        <ONhapMuc label="Tên chặng" value={editStop.label} onChangeText={(label) => changeStop(editStop.id, { label })} />
        <ONhapMuc label="Giờ đến" value={editStop.at} onChangeText={(at) => changeStop(editStop.id, { at })} />
        <View style={styles.row}><Switch {...congTac} accessibilityLabel="Giữ giờ này" value={editStop.time_locked} onValueChange={(time_locked) => changeStop(editStop.id, { time_locked })} /><Text style={[typography.body, { color: colors.ink }]}>Giữ giờ này</Text></View>
        <Text style={[typography.label, { color: colors.ink }]}>Ở lại bao lâu?</Text>
        <View style={styles.row}>{[30, 60, 90, 120].map((duration_minutes) => <Chip key={duration_minutes} label={`${duration_minutes} phút`} selected={editStop.duration_minutes === duration_minutes} onPress={() => changeStop(editStop.id, { duration_minutes })} />)}</View>
        {editStop.duration_minutes === null ? <Text style={[typography.note, { color: colors.inkSoft }]}>Gợi ý 60 phút. Chọn thời lượng để kiểm tra lịch.</Text> : null}
        <ONhapMuc label="Số phút ở lại (0–1440)" keyboardType="number-pad" value={editStop.duration_minutes === null ? "" : String(editStop.duration_minutes)} onChangeText={(v) => { if (/^\d{0,4}$/.test(v) && Number(v) <= 1440) changeStop(editStop.id, { duration_minutes: v === "" ? null : Number(v) }); }} />
        <Text style={[typography.label, { color: colors.ink }]}>Thuộc ngày</Text><ScrollView horizontal contentContainerStyle={styles.row}>{days.map((date, i) => <Chip key={date} label={`Ngày ${i + 1}`} selected={editStop.day === date} onPress={() => changeStop(editStop.id, { day: date })} />)}</ScrollView>
        <View style={styles.row}><Chip label="Điểm xuất phát" selected={draft.days.some((d) => d.start_stop_id === editStop.id)} onPress={() => anchorStop("start_stop_id")} /><Chip label="Điểm kết thúc" selected={draft.days.some((d) => d.end_stop_id === editStop.id)} onPress={() => anchorStop("end_stop_id")} /></View>
        <View style={styles.row}><RudiButton compact full={false} variant="outline" label="Lên trước" onPress={() => invalidate(doiViTri(draft, editStop.id, -1))} /><RudiButton compact full={false} variant="outline" label="Xuống sau" onPress={() => invalidate(doiViTri(draft, editStop.id, 1))} /><RudiButton compact full={false} variant="ghost" tone="warn" label="Bỏ chặng" onPress={() => { invalidate(xoaChang(draft, editStop.id)); setStopId(null); }} /></View>
        <ONhapMuc label="Tìm địa điểm" value={placeQuery} onChangeText={setPlaceQuery} />
        <View style={styles.row}>{matches.map((p) => <Chip key={p.id} label={p.name} selected={editStop.place_id === p.id} onPress={() => changeStop(editStop.id, { place_id: p.id, place_name: p.name, meeting_point: null })} />)}</View>
        {!matches.length ? <Text style={[typography.note, { color: colors.inkSoft }]}>Chưa thấy địa điểm này. Bạn có thể giữ trên bản đồ để chọn điểm hẹn riêng.</Text> : null}
      </View> : <Text style={[typography.note, { color: colors.inkSoft }]}>Chọn chặng để giữ giờ, thêm thời lượng hoặc chia ngày.</Text>}
      <RudiButton label="Thêm chặng" variant="outline" disabled={draft.stops.length >= 50} onPress={() => { const id = `tmp-${Date.now()}`; invalidate({ ...draft, days: draft.days.some((d) => d.day === day) ? draft.days : [...draft.days, settings], stops: [...draft.stops, { id, position: draft.stops.length, at: settings.start_at, label: "Điểm hẹn mới", place_id: null, place_name: null, meeting_point: null, day, duration_minutes: null, time_locked: true }] }); setStopId(id); }} />
      {/* The draft is saved from here too: «Xem trên bản đồ» only closed the
          sheet, and the save sat under the day page's fold, so an edit could
          look kept when it was not (critique, B3). */}
      {dirty ? <RudiButton label="Lưu cho cả hội" disabled={busy} lyDo={busy ? "Đang lưu" : undefined} loading={busy} onPress={() => void save(draft)} /> : null}
      <RudiButton label="Xem trên bản đồ" variant={dirty ? "outline" : "solid"} onPress={() => setEditing(false)} />
    </View></Sheet>
    <Sheet open={pin !== null} onClose={() => setPin(null)} accessibilityLabel="Điểm hẹn của chuyến đi"><View style={styles.editor}><Text style={[typography.h2, { color: colors.ink }]}>Hẹn nhau ở đây</Text><ONhapMuc label="Tên điểm hẹn" value={pinName} onChangeText={setPinName} /><Text style={[typography.note, { color: colors.inkSoft }]}>Điểm bạn chọn sẽ được chia sẻ trong lịch trình của hội.</Text><RudiButton label="Thêm điểm hẹn" disabled={!pinName.trim()} onPress={addPoint} /></View></Sheet>
    </LenLop>
  </View>;
}
const styles = StyleSheet.create({
  stack: { gap: 12 },
  row: { flexDirection: "row", alignItems: "center", gap: 8, flexWrap: "wrap" },
  editor: { padding: 20, gap: 16 },
  // Out to the sheet's edge (the editor pads 20), the first chip still on the column.
  tran: { marginHorizontal: -20, flexGrow: 0 },
  hangTran: { flexDirection: "row", alignItems: "center", gap: 8, paddingHorizontal: 20 },
  oChip: { maxWidth: 240 },
});
