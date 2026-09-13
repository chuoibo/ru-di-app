/** MapLibre native implementation. Loaded only when the APK linked the module. */

import { useEffect, useMemo, useRef, useState } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";
import { Camera, GeoJSONSource, Layer, Map, Marker, type CameraRef, type MapRef } from "@maplibre/maplibre-react-native";

import Svg, { Path } from "react-native-svg";
import { TAM_DA_LAT } from "./toa-do-mau";
import { DEM_KHOP, hopGioi, hopHanhTrinh, muiTenDoan, tapHop, type BanDoProps } from "./kieu-ban-do";


export function BanDo({
  mocs,
  doan,
  mauMoc,
  mauMocInk,
  mauMocChon,
  mauDuong,
  mauDuongMo,
  mauVien,
  mauVienDuong,
  mauNen,
  kieu,
  fitDem,
  cameraKey,
  fitPoints,
  padding = DEM_KHOP,
  duration = 0,
  onGhim,
  toi,
  onUserMove,
  onChonMoc,
  onChonDoan,
  onNen,
}: BanDoProps) {
  const cam = useRef<CameraRef>(null);
  const map = useRef<MapRef>(null);
  const [groups, setGroups] = useState<string[][]>([]);
  const [choosing, setChoosing] = useState<string[]>([]);
  const [viewport, setViewport] = useState({ width: 0, height: 0 });
  const projection = useRef(0);
  const updateGroups = async () => {
    const sequence = ++projection.current;
    try {
      const points = await Promise.all(mocs.map(async (m) => ({ id: m.id, pixel: await map.current?.project([m.lng, m.lat]) })));
      if (sequence !== projection.current) return;
      const pending = points.filter((p) => p.pixel);
      const next: string[][] = [];
      while (pending.length) {
        const first = pending.shift()!;
        const close = pending.filter((p) => Math.hypot(p.pixel![0] - first.pixel![0], p.pixel![1] - first.pixel![1]) < 52);
        next.push([first.id, ...close.map((p) => p.id)]);
        for (const p of close) pending.splice(pending.indexOf(p), 1);
      }
      setGroups(next);
    } catch { /* The map may have unmounted during a camera transition. */ }
  };
  useEffect(() => { setChoosing([]); void updateGroups(); return () => { projection.current++; }; }, [cameraKey, mocs.map((m) => `${m.id}:${m.lat}:${m.lng}`).join("|")]);
  const vuaMoc = useRef(0);
  const duLieu = useMemo(() => tapHop(doan) as never, [doan]);
  const hopBanDau = (fitPoints?.length ? hopGioi(fitPoints) : hopHanhTrinh(mocs, doan));

  useEffect(() => {
    if (fitDem === 0 || !viewport.width || !viewport.height) return;
    const hop = (fitPoints?.length ? hopGioi(fitPoints) : hopHanhTrinh(mocs, doan));
    // Fit after the map's measured frame reaches native, including a tablet
    // layout change. A hidden view reports zero and keeps its previous frame.
    const frame = requestAnimationFrame(() => {
      if (!hop) void cam.current?.easeTo({ center: [TAM_DA_LAT.lng, TAM_DA_LAT.lat], zoom: 12, duration });
      else void cam.current?.fitBounds(hop, { padding, duration });
    });
    return () => cancelAnimationFrame(frame);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [fitDem, cameraKey, padding, viewport.width, viewport.height]);

  useEffect(() => {
    if (!toi) return;
    // The journey panel covers the bottom of the map, so the geometric centre
    // is behind it: pad the camera by the same amount Fit Journey uses, or a
    // chosen stop eases to a spot the person cannot see.
    void cam.current?.easeTo({ center: [toi.lng, toi.lat], duration, padding, zoom: 14 });
  }, [toi?.dem, padding]);

  return (
    <View style={styles.fill} onLayout={(e) => {
      const { width, height } = e.nativeEvent.layout;
      if (width > 0 && height > 0) setViewport((previous) => previous.width === width && previous.height === height ? previous : { width, height });
    }}>
    <Map
      ref={map}
      attribution
      // Top-right: the OSM/OpenFreeMap credit must stay reachable, and the
      // bottom of the map belongs to the journey panel.
      attributionPosition={{ top: 8, right: 8 }}
      compass
      // Facing north -- which is every state until someone rotates with two
      // fingers -- the compass ornament drew as a blank white rectangle over
      // the tiles (emulator, 2026-09-12). Hide it until it means something.
      compassHiddenFacingNorth
      compassPosition={{ top: 8, right: 8 }}
      logo={false}
      mapStyle={kieu}
      onPress={() => {
        if (Date.now() - vuaMoc.current < 80) {
          vuaMoc.current = 0;
          return;
        }
        // Segment presses arrive on the source's own onPress; a press that
        // reaches the map carried no feature, so it is a press on the ground.
        setChoosing([]); onNen();
      }}
      onLongPress={(event) => {
        const [lng, lat] = event.nativeEvent.lngLat;
        onGhim?.({ lat, lng });
      }}
      onRegionDidChange={(e) => {
        if (e.nativeEvent.userInteraction) onUserMove();
        void updateGroups();
      }}
      onDidFinishLoadingMap={() => void updateGroups()}
      scaleBar={false}
      style={[styles.fill, { backgroundColor: mauNen }]}
    >
      <Camera
        ref={cam}
        initialViewState={
          hopBanDau
            ? { bounds: hopBanDau, padding }
            : { center: [TAM_DA_LAT.lng, TAM_DA_LAT.lat], zoom: 12 }
        }
      />
      <GeoJSONSource
        data={duLieu}
        // Without this the press never carries `features` and every tap on the
        // route read as a tap on the ground: the segment card was unreachable
        // on the shipped binary (emulator, 2026-09-12). The hitbox is the
        // finger-sized target the thin line cannot offer by itself.
        hitbox={{ top: 22, right: 22, bottom: 22, left: 22 }}
        id="hanh-trinh-duong"
        onPress={(e) => {
          const id = e.nativeEvent.features?.[0]?.properties?.id;
          if (typeof id !== "string") return;
          // A source press bubbles up to the map unless it is stopped, and the
          // map's own handler clears the selection -- so without this the leg
          // was selected and deselected in the same tap. The flag is the belt
          // to stopPropagation's braces: RN has swallowed one or the other
          // depending on the platform's event path.
          vuaMoc.current = Date.now();
          e.stopPropagation?.();
          onChonDoan(id);
        }}
      >
        <Layer
          id="hanh-trinh-duong-vien"
          filter={["!=", ["get", "uocLuong"], 1]}
          layout={{ "line-cap": "round", "line-join": "round" }}
          paint={{
            "line-color": mauVienDuong,
            "line-opacity": 0.9,
            "line-width": ["case", ["==", ["get", "chon"], 1], 11, 8],
          }}
          type="line"
        />
        <Layer
          id="hanh-trinh-duong-line"
          filter={["!=", ["get", "uocLuong"], 1]}
          layout={{ "line-cap": "round", "line-join": "round" }}
          paint={{
            "line-color": ["case", ["==", ["get", "chon"], 1], mauDuong, mauDuongMo],
            "line-width": ["case", ["==", ["get", "chon"], 1], 6, 4],
            "line-offset": ["*", ["%", ["get", "thuTu"], 2], 3],
          }}
          type="line"
        />
        <Layer
          id="hanh-trinh-net-noi"
          filter={["==", ["get", "uocLuong"], 1]}
          paint={{ "line-color": mauDuongMo, "line-width": 2, "line-dasharray": [2, 3] }}
          type="line"
        />
      </GeoJSONSource>
      {muiTenDoan(doan).map((arrow) => <Marker id={`direction-${arrow.id}`} key={`direction-${arrow.id}`} lngLat={[arrow.lng, arrow.lat]} anchor="center" onPress={() => onChonDoan(arrow.id)}>
        <View pointerEvents="none" style={{ transform: [{ rotate: `${arrow.heading}deg` }] }}><Svg width={18} height={20} viewBox="0 0 18 20"><Path d="M3 14 L9 5 L15 14" fill="none" stroke={mauVienDuong} strokeWidth={6} strokeLinecap="round" strokeLinejoin="round" /><Path d="M3 14 L9 5 L15 14" fill="none" stroke={mauDuong} strokeWidth={3} strokeLinecap="round" strokeLinejoin="round" /></Svg></View>
      </Marker>)}
      {thuTuVe(mocs).filter((m) => !groups.some((g) => g.length > 1 && g.includes(m.id) && g[0] !== m.id)).map((moc) => {
        const group = groups.find((g) => g[0] === moc.id) ?? [moc.id];
        const chon = moc.chon;
        const nen = chon ? mauMocChon : (mauMoc[(moc.so - 1) % mauMoc.length] ?? mauMocChon);
        const co = chon ? 52 : 48;
        return (
          <Marker
            anchor="center"
            id={moc.id}
            key={moc.id}
            lngLat={[moc.lng, moc.lat]}
            // Two stops can share a pixel; the chosen one must be the one on
            // top. Child order alone does not decide that for native markers.
            style={{ zIndex: chon ? 2 : 1 }}
            onPress={() => {
              vuaMoc.current = Date.now();
              if (group.length > 1) setChoosing(group);
              else { setChoosing([]); onChonMoc(moc.id); }
            }}
          >
            <View
              accessibilityLabel={group.length > 1 ? `${group.length} điểm hẹn gần nhau. Chạm để chọn.` : `Mốc ${moc.so}, ${moc.gio}, ${moc.tieuDe}`}
              accessibilityRole="button"
              style={[
                styles.moc,
                {
                  minWidth: co,
                  height: co,
                  backgroundColor: nen,
                  borderColor: mauVien,
                },
              ]}
            >
              <Text maxFontSizeMultiplier={1.3} style={[styles.so, { color: mauMocInk }]}>{group.map((id) => mocs.find((m) => m.id === id)?.so).join(" · ")}</Text>
            </View>
          </Marker>
        );
      })}
    </Map>
    {choosing.length > 1 ? <View accessibilityViewIsModal style={[styles.chooser, { backgroundColor: mauNen, borderColor: mauVien }]}>
      {choosing.map((id) => mocs.find((m) => m.id === id)).filter((m) => m !== undefined).map((m) => <Pressable key={m.id} accessibilityRole="button" onPress={() => { setChoosing([]); onChonMoc(m.id); }} style={styles.choice}><Text style={{ color: mauDuong }}>{m.so} · {m.gio} · {m.tieuDe}</Text></Pressable>)}
      <Pressable accessibilityRole="button" onPress={() => setChoosing([])} style={styles.choice}><Text style={{ color: mauDuong }}>Đóng</Text></Pressable>
    </View> : null}
    </View>
  );
}

/** The selected pin draws last so it is never buried by a neighbour. */
function thuTuVe(mocs: BanDoProps["mocs"]): BanDoProps["mocs"] {
  return [...mocs].sort((a, b) => Number(a.chon) - Number(b.chon));
}

const styles = StyleSheet.create({
  fill: { flex: 1, minHeight: 0 },
  chooser: { position: "absolute", top: 60, left: 16, right: 16, borderWidth: 1, borderRadius: 12, padding: 8 },
  choice: { minHeight: 48, justifyContent: "center", padding: 8 },
  moc: {
    borderRadius: 999,
    borderWidth: 2,
    alignItems: "center",
    justifyContent: "center",
    paddingHorizontal: 4,
  },
  so: { fontSize: 13, fontWeight: "700", lineHeight: 14 },
  gio: { fontSize: 9, fontWeight: "600", lineHeight: 11 },
});
