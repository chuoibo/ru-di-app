import { Image, type ImageSource } from "expo-image";
import { useEffect, useRef, useState } from "react";
import { FlatList, Modal, Platform, Pressable, StyleSheet, Text, View } from "react-native";
import { Gesture, GestureDetector, GestureHandlerRootView } from "react-native-gesture-handler";
import Animated, { runOnJS, useAnimatedStyle, useSharedValue, withTiming } from "react-native-reanimated";
import { SafeAreaView } from "react-native-safe-area-context";

import { typography, useRudiTheme } from "../theme";
import { boundPhotoOffset, viewerIndex } from "../photo-viewer";
import { useMotion } from "./useMotion";

/**
 * One picture in the full-screen pager: the group's own, or an authored asset.
 * A catalogue photograph cannot get here without its credit (F31).
 */
export type ViewerPhoto = { id: string; source: ImageSource; caption: string };

/** Native pager; callers must supply the existing authenticated source adapter. */
export function PhotoViewer({ photos, initialIndex, onClose, title = "Ảnh của nhóm" }: {
  photos: ViewerPhoto[]; initialIndex: number; onClose(): void; title?: string;
}) {
  const { colors } = useRudiTheme();
  const [index, setIndex] = useState(initialIndex);
  const [width, setWidth] = useState(0);
  // The pager's own height, given to every page: on the web a page sized by
  // `flex` inside a horizontal list measured 0 tall, so the photo and the
  // gesture area under it were 0px (QA UI-094).
  const [height, setHeight] = useState(0);
  const [zoomed, setZoomed] = useState(false);
  const currentIndex = viewerIndex(index, photos.length);
  const motion = useMotion();
  // Closing fades as opening does (QA UI-098): the Modal is told to hide and
  // runs its own fade, and only then does the caller unmount it.
  const [mo, setMo] = useState(true);
  const dong = () => {
    if (!mo) return;
    setMo(false);
    setTimeout(onClose, motion.reduced ? 0 : motion.ms("standard"));
  };
  // Web: the pager turns in JS, one page per swipe. The browser's own paging
  // snapped to the next page on release and then let the fling carry on to
  // the one after (b9-sau3: 0 → 390 → 780, «1 / 5» → «3 / 5»), and no
  // `scroll-snap-stop` reached the wrapper react-native-web puts around each
  // page. Native keeps FlatList paging, which stops on one page.
  const listRef = useRef<FlatList<ViewerPhoto>>(null);
  const viTri = useRef(currentIndex);
  viTri.current = currentIndex;
  const den = (i: number, animated = !motion.reduced) => {
    const n = viewerIndex(i, photos.length);
    listRef.current?.scrollToOffset({ offset: n * width, animated });
    setIndex(n);
  };
  // The page a swipe starts from, fixed for the whole swipe: the counter
  // follows the scroll while the finger moves, and reading it mid-swipe added
  // a page to the offset and another on release (0 → 642 → 390 → 780).
  const tuTrang = useRef(currentIndex);
  const dangKeo = useRef(false);
  // A wheel or a trackpad still scrolls the box freely (touch is the app's,
  // and the box stays a scroll box even while a photo is zoomed: the swipe is
  // off then, the photo's own pan moves it); when it rests, the page settles
  // on the nearest photo.
  const henKhop = useRef<ReturnType<typeof setTimeout> | null>(null);
  const khiCuon = (x: number) => {
    setIndex(viewerIndex(x / width, photos.length));
    if (!WEB) return;
    if (henKhop.current !== null) clearTimeout(henKhop.current);
    henKhop.current = setTimeout(() => { if (!dangKeo.current) den(Math.round(x / width)); }, 140);
  };
  useEffect(() => () => { if (henKhop.current !== null) clearTimeout(henKhop.current); }, []);
  const latTrangWeb = Gesture.Pan().enabled(WEB && !zoomed).runOnJS(true).maxPointers(1)
    .activeOffsetX([-12, 12]).failOffsetY([-24, 24])
    .onStart(() => { tuTrang.current = viTri.current; dangKeo.current = true; })
    .onFinalize(() => { dangKeo.current = false; })
    .onUpdate((e) => listRef.current?.scrollToOffset({ offset: tuTrang.current * width - e.translationX, animated: false }))
    .onEnd((e) => {
      const buoc = e.translationX < -48 || e.velocityX < -400 ? 1 : e.translationX > 48 || e.velocityX > 400 ? -1 : 0;
      den(tuTrang.current + buoc);
    });
  // The arrow keys turn the page on the web: a keyboard has no swipe.
  useEffect(() => {
    if (!WEB || typeof window === "undefined") return;
    const phim = (event: KeyboardEvent) => {
      if (event.key === "ArrowRight") den(viTri.current + 1);
      else if (event.key === "ArrowLeft") den(viTri.current - 1);
    };
    window.addEventListener("keydown", phim);
    return () => window.removeEventListener("keydown", phim);
    // `den` reads the page through `viTri`; the listener is rebuilt with the width.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [width, photos.length]);
  return <Modal visible={mo} animationType={motion.reduced ? "none" : "fade"} onRequestClose={dong} presentationStyle="fullScreen">
    <GestureHandlerRootView style={{ flex: 1 }}>
      <SafeAreaView style={[styles.screen, { backgroundColor: colors.cover }]}>
        <View style={styles.header}>
          <View style={styles.flex}>
            <Text style={[typography.label, { color: colors.coverInk }]}>{title}</Text>
            <Text style={[typography.caption, { color: colors.coverInkSoft }]}>{photos.length ? currentIndex + 1 : 0} / {photos.length}</Text>
          </View>
          <Pressable accessibilityRole="button" accessibilityLabel="Đóng ảnh" onPress={dong} style={styles.close}>
            <Text style={[typography.label, { color: colors.coverInk }]}>Đóng</Text>
          </Pressable>
        </View>
        <GestureDetector gesture={latTrangWeb} touchAction="none">
        <View style={styles.flex} onLayout={(event) => {
          const nextWidth = event.nativeEvent.layout.width;
          setHeight(event.nativeEvent.layout.height);
          if (nextWidth !== width) { setZoomed(false); setWidth(nextWidth); }
        }}>
          {width > 0 && height > 0 && photos.length > 0 ? <FlatList key={width} ref={listRef} data={photos} horizontal pagingEnabled={!WEB} scrollEnabled={WEB || !zoomed}
            // One swipe, one photo, and the counter follows while it moves:
            // the web never sent `onMomentumScrollEnd`, so «1 / 3» stayed put.
            disableIntervalMomentum
            onScroll={(event) => khiCuon(event.nativeEvent.contentOffset.x)}
            scrollEventThrottle={32}
            initialScrollIndex={currentIndex}
            getItemLayout={(_, i) => ({ length: width, offset: width * i, index: i })}
            keyExtractor={(photo) => photo.id} initialNumToRender={1} maxToRenderPerBatch={2} windowSize={3}
            showsHorizontalScrollIndicator={false}
            onMomentumScrollEnd={(event) => { setIndex(viewerIndex(event.nativeEvent.contentOffset.x / width, photos.length)); setZoomed(false); }}
            renderItem={({ item, index: itemIndex }) => <ZoomPhoto photo={item} width={width} height={height} active={itemIndex === currentIndex} onZoom={setZoomed} />}
          /> : null}
        </View>
        </GestureDetector>
        <Text style={[typography.caption, styles.hint, { color: colors.coverInkSoft }]}>Vuốt để xem tiếp · chụm hai ngón hoặc chạm đúp để phóng to</Text>
        <Text numberOfLines={4} style={[typography.body, styles.caption, { color: colors.coverInk }]}>{photos[currentIndex]?.caption}</Text>
      </SafeAreaView>
    </GestureHandlerRootView>
  </Modal>;
}

const WEB = Platform.OS === "web";

function ZoomPhoto({ photo, width, height, active, onZoom }: { photo: ViewerPhoto; width: number; height: number; active: boolean; onZoom(value: boolean): void }) {
  const motion = useMotion();
  const settle = motion.timing("standard");
  const scale = useSharedValue(1), savedScale = useSharedValue(1);
  const x = useSharedValue(0), y = useSharedValue(0), originX = useSharedValue(0), originY = useSharedValue(0);
  const [canPan, setCanPan] = useState(false);
  const zoomChanged = (value: boolean) => { setCanPan(value); onZoom(value); };
  useEffect(() => {
    if (!active) { scale.value = 1; savedScale.value = 1; x.value = 0; y.value = 0; setCanPan(false); }
  }, [active, scale, savedScale, x, y]);
  const pinch = Gesture.Pinch().onStart(() => { savedScale.value = scale.value; })
    .onUpdate((event) => {
      scale.value = Math.max(1, Math.min(4, savedScale.value * event.scale));
      x.value = boundPhotoOffset(x.value, width, scale.value);
      y.value = boundPhotoOffset(y.value, height, scale.value);
    })
    .onEnd(() => {
      savedScale.value = scale.value;
      if (scale.value === 1) { x.value = 0; y.value = 0; }
      runOnJS(zoomChanged)(scale.value > 1);
    });
  const pan = Gesture.Pan().enabled(canPan).minPointers(1).onStart(() => { originX.value = x.value; originY.value = y.value; })
    .onUpdate((event) => {
      if (scale.value <= 1) return;
      x.value = boundPhotoOffset(originX.value + event.translationX, width, scale.value);
      y.value = boundPhotoOffset(originY.value + event.translationY, height, scale.value);
    });
  const doubleTap = Gesture.Tap().numberOfTaps(2).onEnd(() => {
    const next = scale.value > 1 ? 1 : 2;
    scale.value = withTiming(next, settle);
    savedScale.value = next;
    x.value = 0; y.value = 0;
    runOnJS(zoomChanged)(next > 1);
  });
  const style = useAnimatedStyle(() => ({ transform: [{ translateX: x.value }, { translateY: y.value }, { scale: scale.value }] }));
  return <View style={{ width, height, overflow: "hidden" }}>
    {/* Every touch on the photo is the app's (`touchAction` none on the web):
        two fingers zoom the photo, never the page; the swipe is the pager's. */}
    <GestureDetector gesture={Gesture.Simultaneous(pinch, pan, doubleTap)}>
      <Animated.View style={[StyleSheet.absoluteFill, style]}>
        <Image source={photo.source} accessibilityLabel={photo.caption || "Ảnh của nhóm"} contentFit="contain" cachePolicy="none" style={StyleSheet.absoluteFill} />
      </Animated.View>
    </GestureDetector>
  </View>;
}

const styles = StyleSheet.create({
  screen: { flex: 1 }, flex: { flex: 1 },
  header: { flexDirection: "row", alignItems: "center", paddingHorizontal: 16, gap: 16 },
  close: { minWidth: 56, minHeight: 48, alignItems: "center", justifyContent: "center" },
  hint: { textAlign: "center", padding: 12 },
  caption: { paddingHorizontal: 20, paddingBottom: 24 },
});
