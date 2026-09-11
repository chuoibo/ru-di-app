/** Web MapLibre GL. Native override: BanDo.native.tsx. */

import { createElement, useEffect, useRef, useState } from "react";
import { StyleSheet, View } from "react-native";
import { GeoJSONSource, Map, Marker, NavigationControl, type MapLayerMouseEvent, type MapMouseEvent } from "maplibre-gl";

import { TAM_DA_LAT } from "./toa-do-mau";
import { DEM_KHOP, hopGioi, KIEU_BAN_DO, tapHop, type BanDoProps, type MocBanDo } from "./kieu-ban-do";

const CSS_HREF = "https://unpkg.com/maplibre-gl@6.9.0/dist/maplibre-gl.css";

function damBaoCss() {
  if (typeof document === "undefined") return;
  if (document.getElementById("maplibre-gl-css")) return;
  const link = document.createElement("link");
  link.id = "maplibre-gl-css";
  link.rel = "stylesheet";
  link.href = CSS_HREF;
  document.head.appendChild(link);
}

function veMoc(moc: MocBanDo, mauMoc: readonly string[], mauInk: string, mauChon: string, mauVien: string): HTMLElement {
  const el = document.createElement("button");
  el.type = "button";
  // Named chips in ManHinhHanhTrinh carry the stop title; pins are numbered so
  // clickLabel is not ambiguous and VoiceOver does not hear each stop twice.
  el.setAttribute("aria-label", `Mốc ${moc.so}`);
  el.setAttribute("role", "button");
  const mau = moc.chon ? mauChon : (mauMoc[(moc.so - 1) % mauMoc.length] ?? mauChon);
  const co = moc.chon ? 36 : 28;
  el.style.cssText = [
    `width:${co}px`,
    `height:${co}px`,
    "border-radius:999px",
    `background:${mau}`,
    `color:${mauInk}`,
    `border:2px solid ${mauVien}`,
    "display:flex",
    "flex-direction:column",
    "align-items:center",
    "justify-content:center",
    "font:700 13px/1 system-ui,sans-serif",
    "padding:0",
    "cursor:pointer",
  ].join(";");
  const so = document.createElement("span");
  so.textContent = String(moc.so);
  el.appendChild(so);
  if (moc.chon && moc.gio) {
    const gio = document.createElement("span");
    gio.textContent = moc.gio;
    gio.style.cssText = "font:600 9px/1 system-ui;margin-top:2px";
    el.style.height = "44px";
    el.style.width = "44px";
    el.appendChild(gio);
  }
  return el;
}

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
  const cbs = useRef({ onUserMove, onChonMoc, onChonDoan, onNen, mauMoc, mauMocInk, mauMocChon, mauVien, mauDuong, mauDuongMo });
  cbs.current = { onUserMove, onChonMoc, onChonDoan, onNen, mauMoc, mauMocInk, mauMocChon, mauVien, mauDuong, mauDuongMo };
  const mapRef = useRef<Map | null>(null);
  const markers = useRef<Marker[]>([]);
  const loaded = useRef(false);
  const [host, setHost] = useState<HTMLDivElement | null>(null);
  const [san, setSan] = useState(false);

  useEffect(() => {
    const el = host;
    if (!el) return;
    damBaoCss();
    const map = new Map({
      container: el,
      style: KIEU_BAN_DO,
      center: [TAM_DA_LAT.lng, TAM_DA_LAT.lat],
      zoom: 12,
      attributionControl: { compact: true },
    });
    map.addControl(new NavigationControl({ showCompass: true, showZoom: false, visualizePitch: true }), "top-right");
    map.on("load", () => {
      loaded.current = true;
      const mau = cbs.current;
      if (!map.getSource("hanh-trinh-duong")) {
        map.addSource("hanh-trinh-duong", { type: "geojson", data: tapHop([]) });
        map.addLayer({
          id: "hanh-trinh-duong",
          type: "line",
          source: "hanh-trinh-duong",
          layout: { "line-cap": "round", "line-join": "round" },
          paint: {
            "line-color": ["case", ["==", ["get", "chon"], 1], mau.mauDuong, mau.mauDuongMo],
            "line-width": ["case", ["==", ["get", "chon"], 1], 5, 3],
            "line-opacity": ["case", ["==", ["get", "chon"], 1], 1, 0.45],
          },
        });
        map.addLayer({
          id: "hanh-trinh-duong-hit",
          type: "line",
          source: "hanh-trinh-duong",
          paint: { "line-color": mau.mauDuong, "line-width": 28, "line-opacity": 0 },
        });
      }
      map.resize();
    });
    map.on("click", "hanh-trinh-duong-hit", (e: MapLayerMouseEvent) => {
      const id = e.features?.[0]?.properties?.id;
      if (typeof id === "string") cbs.current.onChonDoan(id);
    });
    map.on("click", (e: MapMouseEvent) => {
      try {
        if (!map.getLayer("hanh-trinh-duong-hit")) {
          cbs.current.onNen();
          return;
        }
        const feats = map.queryRenderedFeatures(e.point, { layers: ["hanh-trinh-duong-hit"] });
        if (feats.length === 0) cbs.current.onNen();
      } catch {
        cbs.current.onNen();
      }
    });
    map.on("dragstart", () => cbs.current.onUserMove());
    map.on("zoomstart", (e) => {
      if (e.originalEvent) cbs.current.onUserMove();
    });
    map.on("rotatestart", (e) => {
      if (e.originalEvent) cbs.current.onUserMove();
    });
    mapRef.current = map;
    setSan(true);
    const ro = typeof ResizeObserver !== "undefined" ? new ResizeObserver(() => map.resize()) : null;
    ro?.observe(el);
    requestAnimationFrame(() => map.resize());
    return () => {
      ro?.disconnect();
      markers.current.forEach((m) => m.remove());
      markers.current = [];
      map.remove();
      mapRef.current = null;
      loaded.current = false;
      setSan(false);
    };
  }, [host]);

  useEffect(() => {
    const map = mapRef.current;
    if (!map) return;
    const ve = () => {
      const src = map.getSource("hanh-trinh-duong");
      if (src && "setData" in src) (src as GeoJSONSource).setData(tapHop(doan));
    };
    if (loaded.current && map.isStyleLoaded()) ve();
    else map.once("load", ve);
  }, [doan, san]);

  useEffect(() => {
    const map = mapRef.current;
    if (!map) return;
    markers.current.forEach((m) => m.remove());
    markers.current = mocs.map((moc) => {
      const el = veMoc(moc, cbs.current.mauMoc, cbs.current.mauMocInk, cbs.current.mauMocChon, cbs.current.mauVien);
      el.addEventListener("click", (ev) => {
        ev.stopPropagation();
        cbs.current.onChonMoc(moc.id);
      });
      return new Marker({ element: el, anchor: "bottom" }).setLngLat([moc.lng, moc.lat]).addTo(map);
    });
  }, [mocs, san]);

  useEffect(() => {
    const map = mapRef.current;
    if (!map || !san || fitDem === 0) return;
    const hop = hopGioi(mocs);
    if (!hop) {
      map.easeTo({ center: [TAM_DA_LAT.lng, TAM_DA_LAT.lat], zoom: 12, duration: 400 });
      return;
    }
    map.fitBounds(
      [
        [hop[0], hop[1]],
        [hop[2], hop[3]],
      ],
      { padding: DEM_KHOP, duration: 600 },
    );
    // Fit Journey is the only yank; selection uses easeTo, pan stays free.
    // eslint-disable-next-line react-hooks/exhaustive-deps -- mocs is read at the tick of fitDem
  }, [fitDem, san]);

  useEffect(() => {
    const map = mapRef.current;
    if (!map || !toi) return;
    map.easeTo({ center: [toi.lng, toi.lat], zoom: Math.max(map.getZoom(), 14), duration: 500 });
  }, [toi, san]);

  return (
    <View collapsable={false} style={[styles.fill, { backgroundColor: mauNen }]} testID="ban-do-hanh-trinh">
      {createElement("div", {
        id: "ban-do-hanh-trinh",
        ref: (node: HTMLDivElement | null) => {
          setHost(node);
        },
        style: { position: "absolute", top: 0, right: 0, bottom: 0, left: 0, backgroundColor: mauNen },
      })}
    </View>
  );
}

const styles = StyleSheet.create({
  fill: { flex: 1, minHeight: 220, position: "relative" },
});
