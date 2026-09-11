/** Native MapLibre. Web uses BanDo.tsx (maplibre-gl). */

import { useEffect, useMemo, useRef } from "react";
import { StyleSheet, Text, View } from "react-native";
import { Camera, GeoJSONSource, Layer, Map, Marker, type CameraRef } from "@maplibre/maplibre-react-native";

import { TAM_DA_LAT } from "./toa-do-mau";
import { DEM_KHOP, hopGioi, KIEU_BAN_DO, tapHop, type BanDoProps } from "./kieu-ban-do";

export function BanDo({
  mocs,
  doan,
  mauMoc,
  mauMocInk,
  mauMocChon,
  mauDuong,
  mauDuongMo,
  mauVien,
  mauNen,
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
    void cam.current?.easeTo({ center: [toi.lng, toi.lat], zoom: 14, duration: 500 });
  }, [toi]);

  return (
    <Map
      attribution
      attributionPosition={{ bottom: 8, left: 8 }}
      compass
      compassPosition={{ top: 8, right: 8 }}
      mapStyle={KIEU_BAN_DO}
      onPress={(e) => {
        if (vuaMoc.current) {
          vuaMoc.current = false;
          return;
        }
        const feats = "features" in e.nativeEvent ? e.nativeEvent.features : [];
        const id = feats?.[0]?.properties?.id;
        if (typeof id === "string") {
          onChonDoan(id);
          return;
        }
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
      <GeoJSONSource id="hanh-trinh-duong" data={duLieu}>
        <Layer
          id="hanh-trinh-duong-line"
          paint={{
            "line-color": ["case", ["==", ["get", "chon"], 1], mauDuong, mauDuongMo],
            "line-opacity": ["case", ["==", ["get", "chon"], 1], 1, 0.45],
            "line-width": ["case", ["==", ["get", "chon"], 1], 5, 3],
          }}
          type="line"
        />
      </GeoJSONSource>
      {mocs.map((moc) => {
        const chon = moc.chon;
        const nen = chon ? mauMocChon : (mauMoc[(moc.so - 1) % mauMoc.length] ?? mauMocChon);
        const co = chon ? 36 : 28;
        return (
          <Marker
            id={moc.id}
            key={moc.id}
            lngLat={[moc.lng, moc.lat]}
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
