/** MapLibre native implementation. Loaded only when the APK linked the module. */

import { useEffect, useMemo, useRef } from "react";
import { StyleSheet, Text, View } from "react-native";
import { Camera, GeoJSONSource, Layer, Map, Marker, type CameraRef } from "@maplibre/maplibre-react-native";

import { TAM_DA_LAT } from "./toa-do-mau";
import { DEM_KHOP, hopGioi, tapHop, type BanDoProps } from "./kieu-ban-do";

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
  toi,
  onUserMove,
  onChonMoc,
  onChonDoan,
  onNen,
}: BanDoProps) {
  const cam = useRef<CameraRef>(null);
  const vuaMoc = useRef(false);
  const duLieu = useMemo(() => tapHop(doan) as never, [doan]);
  const hopBanDau = hopGioi(mocs);

  useEffect(() => {
    if (fitDem === 0) return;
    const hop = hopGioi(mocs);
    if (!hop) {
      void cam.current?.easeTo({ center: [TAM_DA_LAT.lng, TAM_DA_LAT.lat], zoom: 12, duration: 400 });
      return;
    }
    void cam.current?.fitBounds(hop, { padding: DEM_KHOP, duration: 600 });
    // Only Fit Journey (fitDem) may yank the camera.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [fitDem]);

  useEffect(() => {
    if (!toi) return;
    // The journey panel covers the bottom of the map, so the geometric centre
    // is behind it: pad the camera by the same amount Fit Journey uses, or a
    // chosen stop eases to a spot the person cannot see.
    void cam.current?.easeTo({ center: [toi.lng, toi.lat], duration: 500, padding: DEM_KHOP, zoom: 14 });
  }, [toi]);

  return (
    <Map
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
        if (vuaMoc.current) {
          vuaMoc.current = false;
          return;
        }
        // Segment presses arrive on the source's own onPress; a press that
        // reaches the map carried no feature, so it is a press on the ground.
        onNen();
      }}
      onRegionDidChange={(e) => {
        if (e.nativeEvent.userInteraction) onUserMove();
      }}
      scaleBar={false}
      style={[styles.fill, { backgroundColor: mauNen }]}
    >
      <Camera
        ref={cam}
        initialViewState={
          hopBanDau
            ? { bounds: hopBanDau, padding: DEM_KHOP }
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
          vuaMoc.current = true;
          e.stopPropagation?.();
          onChonDoan(id);
        }}
      >
        <Layer
          id="hanh-trinh-duong-vien"
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
          layout={{ "line-cap": "round", "line-join": "round" }}
          paint={{
            "line-color": ["case", ["==", ["get", "chon"], 1], mauDuong, mauDuongMo],
            "line-width": ["case", ["==", ["get", "chon"], 1], 6, 4],
          }}
          type="line"
        />
      </GeoJSONSource>
      {thuTuVe(mocs).map((moc) => {
        const chon = moc.chon;
        const nen = chon ? mauMocChon : (mauMoc[(moc.so - 1) % mauMoc.length] ?? mauMocChon);
        const co = chon ? 44 : 28;
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
              vuaMoc.current = true;
              onChonMoc(moc.id);
            }}
          >
            <View
              accessibilityLabel={`Mốc ${moc.so}`}
              accessibilityRole="button"
              style={[
                styles.moc,
                {
                  width: co,
                  height: chon ? 44 : co,
                  backgroundColor: nen,
                  borderColor: mauVien,
                },
              ]}
            >
              <Text style={[styles.so, { color: mauMocInk }]}>{moc.so}</Text>
              {chon ? <Text style={[styles.gio, { color: mauMocInk }]}>{moc.gio}</Text> : null}
            </View>
          </Marker>
        );
      })}
    </Map>
  );
}

/** The selected pin draws last so it is never buried by a neighbour. */
function thuTuVe(mocs: BanDoProps["mocs"]): BanDoProps["mocs"] {
  return [...mocs].sort((a, b) => Number(a.chon) - Number(b.chon));
}

const styles = StyleSheet.create({
  fill: { flex: 1, minHeight: 220 },
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
