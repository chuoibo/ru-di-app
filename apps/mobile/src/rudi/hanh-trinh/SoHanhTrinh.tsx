/** A day's map and editable trip notebook share one revision-bound draft. */
import { useEffect, useMemo, useRef, useState } from "react";
import { AppState, Platform, ScrollView, StyleSheet, Switch, Text, View } from "react-native";
import { useSafeAreaInsets } from "react-native-safe-area-context";
import type { BuoiDi } from "../../screens/len-plan/buoi-di";
import { ApiError, luuHanhTrinh, newAttempt, xemTruocHanhTrinh, type Attempt } from "../../api";
import { typography, useRudiTheme } from "../theme";
import { Chip, Field, RudiButton } from "../ui";
import { Sheet } from "../ui/Sheet";
import { ManHinhHanhTrinh } from "./ManHinhHanhTrinh";
import { useCheDoLichTrinh } from "./che-do";
import { chieuTuChang } from "./chieu";
import type { ChoChieu, ToaDo } from "./mo-hinh";
import { apDungTuyen, doiViTri, ngayMacDinh, nhapTuKeo, suaChang, xoaChang, type BanNhap, type ChangDi, type NgayDi, type XemTruoc } from "./ke-hoach";
import { chuKhoangCach, chuThoiGian } from "./tom-tat";

type Props = { outing: BuoiDi; places: ChoChieu[]; actorId?: string; onSaved: (outing: BuoiDi) => void; onReload?: () => Promise<void>; onTimeline: () => void; bottom?: number; fixture?: boolean; controller?: ReturnType<typeof useCheDoLichTrinh>; initialDay?: string; onDay?: (day: string) => void };
export function SoHanhTrinh({ outing, places, actorId, onSaved, onReload, onTimeline, bottom = 0, fixture = false, controller, initialDay, onDay }: Props) {
  const { colors } = useRudiTheme();
  const insets = useSafeAreaInsets();
  const [draft, setDraft] = useState(() => nhapTuKeo(outing));
  const [day, setDay] = useState(initialDay ?? outing.starts_on);
  const [preview, setPreview] = useState<XemTruoc | null>(null);
  const [suggestion, setSuggestion] = useState(false);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
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
  const invalidate = (next: BanNhap) => { if (saving.current) return; request.current++; setDraft(next); setPreview(null); setSuggestion(false); setMessage(null); setBusy(false); };
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
    if (!actorId || fixture) { setMessage("Bản dùng thử chỉ xem lịch trình. Đề xuất cần kết nối dịch vụ đường bộ."); return; }
    const sequence = ++request.current; setBusy(true); setMessage(null); setPreview(null); setSuggestion(false);
    try {
      const result = await xemTruocHanhTrinh(outing.id, draft, day, recommend, actorId, outing.context_id);
      if (sequence !== request.current) return;
      setPreview(result); setSuggestion(false);
      if (recommend && result.suggestion) { che.chonHoatDong(null); che.khopHanhTrinh(); }
      if (recommend && !result.suggestion && result.status === "ready") setMessage("Chưa tìm được phương án tốt hơn với những giờ đã giữ.");
    } catch (error) { if (sequence === request.current) setMessage(error instanceof Error ? error.message : "Chưa lấy được tuyến. Thử lại khi có mạng."); }
    finally { if (sequence === request.current) setBusy(false); }
  };
  useEffect(() => { void inspect(false); return () => { request.current++; }; }, [day, outing.id, draft.expected_revision]);
  useEffect(() => {
    const subscription = AppState.addEventListener("change", (state) => { if (state === "active" && preview?.status !== "ready" && !dirty) void inspect(false); });
    return () => subscription.remove();
  }, [preview?.status, dirty, day]);
  const fitPoints = useMemo(() => [preview?.current, preview?.suggestion].flatMap((r) => r?.segments.flatMap((s) => s.geometry.map(([lng, lat]) => ({ lat, lng }))) ?? []), [preview]);
  const route = suggestion ? preview?.suggestion : preview?.current;
  const visible = useMemo(() => {
    let stops = draft.stops.filter((s) => s.day === day);
    if (route && suggestion) { const byId = new Map(stops.map((s) => [s.id, s])); stops = route.stops.flatMap((r) => { const s = byId.get(r.id); return s ? [{ ...s, at: r.at ?? s.at }] : []; }); }
    const projected = chieuTuChang(stops, places);
    if (route) projected.routeSegments = route.segments.map((s) => ({ id: `${s.from_stop_id}->${s.to_stop_id}`, fromActivityId: s.from_stop_id, toActivityId: s.to_stop_id, distanceMeters: s.distance_meters, durationSeconds: s.duration_seconds, transportMode: settings.transport_mode, polyline: s.geometry.map(([lng, lat]) => ({ lat, lng })), nguon: "valhalla" }));
    return projected;
  }, [draft, day, places, route, suggestion, settings.transport_mode]);
  const save = async (next: BanNhap, restoring = false) => {
    if (saving.current) return; saving.current = true; request.current++; setBusy(true); setMessage(null);
    const previous = nhapTuKeo(outing);
    const key = JSON.stringify(next);
    if (retry.current?.key !== key) retry.current = { key, attempt: newAttempt() };
    try {
      const result = fixture ? { ...outing, stops: next.stops, days: next.days, timeline_revision: outing.timeline_revision + 1, itinerary_version: 2 as const } : await luuHanhTrinh(outing.id, next, actorId!, retry.current.attempt, outing.context_id);
      const canUndo = previous.stops.every((s) => result.stops.some((r) => r.id === s.id));
      request.current++; baseline.current = nhapTuKeo(result); setConflict(false);
      onSaved(result); setDraft(nhapTuKeo(result)); setUndo(restoring || !canUndo ? null : { ...previous, expected_revision: result.timeline_revision }); setPreview(null); setSuggestion(false); setEditing(false); retry.current = null; setMessage(restoring ? "Đã hoàn tác lần lưu vừa rồi." : "Đã giữ trang ngày cho cả hội.");
    } catch (error) { if (error instanceof ApiError && error.code === "timeline_conflict") setConflict(true); setMessage(error instanceof ApiError && error.code === "timeline_conflict" ? "Có người vừa sửa lịch trình. Tải bản mới để đối chiếu; bản nháp này chưa được ghi đè." : error instanceof Error ? error.message : "Chưa lưu được. Bạn có thể thử lại."); }
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
  const actions = <View style={styles.stack}>
    <ScrollView horizontal showsHorizontalScrollIndicator={false} contentContainerStyle={styles.row}>
      {(["motorbike", "car", "walk"] as const).map((mode) => <Chip key={mode} label={{ motorbike: "Xe máy", car: "Ô tô", walk: "Đi bộ" }[mode]} selected={settings.transport_mode === mode} onPress={() => changeDay({ transport_mode: mode })} />)}
    </ScrollView>
    {preview?.suggestion ? <>
      <View style={styles.row}><Chip label="Hiện tại" selected={!suggestion} onPress={() => setSuggestion(false)} /><Chip label="Gợi ý" selected={suggestion} onPress={() => setSuggestion(true)} /></View>
      {preview.savings ? <Text style={[typography.label, { color: colors.ink }]}>{preview.savings.duration_seconds === 0 ? "Thời gian di chuyển không đổi" : `${chuThoiGian(Math.abs(preview.savings.duration_seconds))} ${preview.savings.duration_seconds > 0 ? "ít di chuyển hơn" : "di chuyển thêm"}`} · {chuKhoangCach(Math.abs(preview.savings.distance_meters))} {preview.savings.distance_meters >= 0 ? "ngắn hơn" : "dài hơn"}</Text> : null}
      {suggestion ? preview.suggestion.stops.filter((s) => s.at !== draft.stops.find((d) => d.id === s.id)?.at).map((s) => <Text key={s.id} style={[typography.note, { color: colors.inkSoft }]}>{draft.stops.find((d) => d.id === s.id)?.label}: {draft.stops.find((d) => d.id === s.id)?.at} → {s.at}</Text>) : null}
    </> : null}
    {preview ? [...preview.issues, ...(route?.issues ?? [])].filter((v, i, all) => all.findIndex((a) => a.code === v.code && a.stop_id === v.stop_id) === i).map((issue, i) => <Text key={i} style={[typography.note, { color: colors.inkSoft }]}>{issue.message}</Text>) : null}
    {route ? <Text style={[typography.caption, { color: colors.inkSoft }]}>Thời gian ước tính · chưa tính giao thông trực tiếp</Text> : null}
    {message ? <Text accessibilityLiveRegion="polite" style={[typography.note, { color: colors.inkSoft }]}>{message}</Text> : null}
    {conflict ? <View style={styles.stack}>
      {onReload ? <RudiButton label="Tải bản mới để đối chiếu" variant="outline" disabled={busy} onPress={() => void onReload()} /> : null}
      {outing.timeline_revision !== draft.expected_revision ? <>
        <Text style={[typography.note, { color: colors.inkSoft }]}>Bản của hội: {outing.stops.map((s) => `${s.at} ${s.label}`).join(" → ")}</Text>
        <RudiButton label="Bỏ nháp, dùng bản mới" accessibilityLabel="Bỏ bản nháp, dùng bản của hội" variant="outline" disabled={busy} onPress={() => { invalidate(nhapTuKeo(outing)); setUndo(null); setConflict(false); }} />
      </> : null}
    </View> : null}
    <View style={styles.row}><RudiButton compact full={false} label="Sửa trang ngày" variant="outline" onPress={() => { setStopId(che.selectedActivityId); setEditing(true); }} /><RudiButton compact full={false} label="Tính lại đường" variant="ghost" disabled={busy} onPress={() => void inspect(false)} /></View>
    {dirty ? <RudiButton label="Lưu những thay đổi" variant="outline" disabled={busy} onPress={() => void save(draft)} /> : null}
    {undo ? <RudiButton label="Hoàn tác lần lưu vừa rồi" variant="ghost" disabled={busy} onPress={() => void save(undo, true)} /> : null}
    <Text style={[typography.caption, { color: colors.inkSoft }]}>{Platform.OS === "web" ? "Nhấp chuột phải trên bản đồ để thêm điểm hẹn." : "Giữ trên bản đồ để thêm điểm hẹn."}</Text>
  </View>;
  return <View style={{ flex: 1 }}>
    <ScrollView horizontal style={{ flexGrow: 0 }} contentContainerStyle={[styles.row, { paddingHorizontal: 16, paddingVertical: 8 }]} showsHorizontalScrollIndicator={false}>{days.map((date, i) => <Chip key={date} label={`Ngày ${i + 1}`} selected={day === date} onPress={() => chooseDay(date)} />)}</ScrollView>
    <ManHinhHanhTrinh hanh={visible} fitDem={che.fitDem + 1} cameraKey={`${day}:${preview ? "routed" : "draft"}`} fitPoints={fitPoints} toiDem={che.toiDem} selectedActivityId={che.selectedActivityId} selectedSegmentId={che.selectedSegmentId} onChonMoc={che.chonHoatDong} onChonDoan={che.chonDoan} onNen={() => { che.chonHoatDong(null); che.chonDoan(null); }} onKhop={che.khopHanhTrinh} onUserMove={che.userMove} onVeLichTrinh={onTimeline} onGhim={(point) => { if (saving.current) return; if (draft.stops.length >= 50) { setMessage("Lịch trình đã đủ 50 chặng. Bỏ một chặng trước khi thêm điểm hẹn."); return; } setPin(point); setPinName(""); }} chanDuoi={Math.max(bottom, insets.bottom)} actions={actions} primaryAction={suggestion && preview?.suggestion ? <RudiButton disabled={busy || !preview.suggestion.feasible} label="Giữ phương án này" onPress={() => void save(apDungTuyen(draft, day, preview.suggestion!))} /> : <RudiButton label="Xem cách đi gọn hơn" loading={busy} disabled={busy || !visible.activities.length} onPress={() => void inspect(true)} />} />
    <Sheet open={editing} onClose={() => setEditing(false)} accessibilityLabel="Sửa trang ngày"><View style={styles.editor}>
      <Text style={[typography.h2, { color: colors.ink }]}>Những hẹn quan trọng</Text>
      <Field label="Giờ xuất phát" value={settings.start_at} onChangeText={(start_at) => changeDay({ start_at })} />
      <View style={styles.row}><Switch accessibilityLabel="Quay về điểm đầu" value={settings.return_to_start} onValueChange={(return_to_start) => changeDay({ return_to_start })} /><Text style={[typography.body, { color: colors.ink }]}>Quay về điểm đầu</Text></View>
      <ScrollView horizontal contentContainerStyle={styles.row}>{draft.stops.map((s) => <Chip key={s.id} label={`${s.at} · ${s.label}${s.day === null ? " · chưa chia ngày" : ""}`} selected={s.id === stopId} onPress={() => setStopId(s.id)} />)}</ScrollView>
      {editStop ? <View style={styles.stack}>
        <Text style={[typography.h2, { color: colors.ink }]}>{editStop.label}</Text>
        <Field label="Tên chặng" value={editStop.label} onChangeText={(label) => changeStop(editStop.id, { label })} />
        <Field label="Giờ đến" value={editStop.at} onChangeText={(at) => changeStop(editStop.id, { at })} />
        <View style={styles.row}><Switch accessibilityLabel="Giữ giờ này" value={editStop.time_locked} onValueChange={(time_locked) => changeStop(editStop.id, { time_locked })} /><Text style={[typography.body, { color: colors.ink }]}>Giữ giờ này</Text></View>
        <Text style={[typography.label, { color: colors.ink }]}>Ở lại bao lâu?</Text>
        <View style={styles.row}>{[30, 60, 90, 120].map((duration_minutes) => <Chip key={duration_minutes} label={`${duration_minutes} phút`} selected={editStop.duration_minutes === duration_minutes} onPress={() => changeStop(editStop.id, { duration_minutes })} />)}</View>
        {editStop.duration_minutes === null ? <Text style={[typography.note, { color: colors.inkSoft }]}>Gợi ý 60 phút. Chọn thời lượng để kiểm tra lịch.</Text> : null}
        <Field label="Số phút ở lại (0–1440)" keyboardType="number-pad" value={editStop.duration_minutes === null ? "" : String(editStop.duration_minutes)} onChangeText={(v) => { if (/^\d{0,4}$/.test(v) && Number(v) <= 1440) changeStop(editStop.id, { duration_minutes: v === "" ? null : Number(v) }); }} />
        <Text style={[typography.label, { color: colors.ink }]}>Thuộc ngày</Text><ScrollView horizontal contentContainerStyle={styles.row}>{days.map((date, i) => <Chip key={date} label={`Ngày ${i + 1}`} selected={editStop.day === date} onPress={() => changeStop(editStop.id, { day: date })} />)}</ScrollView>
        <View style={styles.row}><Chip label="Điểm xuất phát" selected={draft.days.some((d) => d.start_stop_id === editStop.id)} onPress={() => anchorStop("start_stop_id")} /><Chip label="Điểm kết thúc" selected={draft.days.some((d) => d.end_stop_id === editStop.id)} onPress={() => anchorStop("end_stop_id")} /></View>
        <View style={styles.row}><RudiButton compact full={false} variant="outline" label="Lên trước" onPress={() => invalidate(doiViTri(draft, editStop.id, -1))} /><RudiButton compact full={false} variant="outline" label="Xuống sau" onPress={() => invalidate(doiViTri(draft, editStop.id, 1))} /><RudiButton compact full={false} variant="ghost" label="Bỏ chặng" onPress={() => { invalidate(xoaChang(draft, editStop.id)); setStopId(null); }} /></View>
        <Field label="Tìm địa điểm" value={placeQuery} onChangeText={setPlaceQuery} />
        <View style={styles.row}>{matches.map((p) => <Chip key={p.id} label={p.name} selected={editStop.place_id === p.id} onPress={() => changeStop(editStop.id, { place_id: p.id, place_name: p.name, meeting_point: null })} />)}</View>
        {!matches.length ? <Text style={[typography.note, { color: colors.inkSoft }]}>Chưa thấy địa điểm này. Bạn có thể giữ trên bản đồ để chọn điểm hẹn riêng.</Text> : null}
      </View> : <Text style={[typography.note, { color: colors.inkSoft }]}>Chọn chặng để giữ giờ, thêm thời lượng hoặc chia ngày.</Text>}
      <RudiButton label="Thêm chặng" variant="outline" disabled={draft.stops.length >= 50} onPress={() => { const id = `tmp-${Date.now()}`; invalidate({ ...draft, days: draft.days.some((d) => d.day === day) ? draft.days : [...draft.days, settings], stops: [...draft.stops, { id, position: draft.stops.length, at: settings.start_at, label: "Điểm hẹn mới", place_id: null, place_name: null, meeting_point: null, day, duration_minutes: null, time_locked: true }] }); setStopId(id); }} />
      <RudiButton label="Xem trên bản đồ" onPress={() => setEditing(false)} />
    </View></Sheet>
    <Sheet open={pin !== null} onClose={() => setPin(null)} accessibilityLabel="Điểm hẹn của chuyến đi"><View style={styles.editor}><Text style={[typography.h2, { color: colors.ink }]}>Hẹn nhau ở đây</Text><Field label="Tên điểm hẹn" value={pinName} onChangeText={setPinName} /><Text style={[typography.note, { color: colors.inkSoft }]}>Điểm bạn chọn sẽ được chia sẻ trong lịch trình của hội.</Text><RudiButton label="Thêm điểm hẹn" disabled={!pinName.trim()} onPress={addPoint} /></View></Sheet>
  </View>;
}
const styles = StyleSheet.create({ stack: { gap: 12 }, row: { flexDirection: "row", alignItems: "center", gap: 8, flexWrap: "wrap" }, editor: { padding: 20, gap: 16 } });
