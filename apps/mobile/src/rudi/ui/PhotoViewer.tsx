import { Image, type ImageSource } from "expo-image";
import { useEffect, useState } from "react";
import { FlatList, Modal, Pressable, StyleSheet, Text, View } from "react-native";
import { Gesture, GestureDetector, GestureHandlerRootView } from "react-native-gesture-handler";
import Animated, { runOnJS, useAnimatedStyle, useSharedValue, withTiming } from "react-native-reanimated";
import { SafeAreaView } from "react-native-safe-area-context";

import { typography, useRudiTheme } from "../theme";
import { boundPhotoOffset, viewerIndex } from "../photo-viewer";
import { useMotion } from "./useMotion";

export type ViewerPhoto = { id: string; source: ImageSource; caption: string };

/** Native pager; callers must supply the existing authenticated source adapter. */
export function PhotoViewer({ photos, initialIndex, onClose, title = "Ảnh của nhóm" }: {
  photos: ViewerPhoto[]; initialIndex: number; onClose(): void; title?: string;
}) {
  const { colors } = useRudiTheme();
  const [index, setIndex] = useState(initialIndex);
  const [width, setWidth] = useState(0);
  const [zoomed, setZoomed] = useState(false);
  const currentIndex = viewerIndex(index, photos.length);
  const motion = useMotion();
  return <Modal visible animationType={motion.reduced ? "none" : "fade"} onRequestClose={onClose} presentationStyle="fullScreen">
    <GestureHandlerRootView style={{ flex: 1 }}>
      <SafeAreaView style={[styles.screen, { backgroundColor: colors.cover }]}>
        <View style={styles.header}>
          <View style={styles.flex}>
            <Text style={[typography.label, { color: colors.coverInk }]}>{title}</Text>
            <Text style={[typography.caption, { color: colors.coverInkSoft }]}>{photos.length ? currentIndex + 1 : 0} / {photos.length}</Text>
          </View>
          <Pressable accessibilityRole="button" accessibilityLabel="Đóng ảnh" onPress={onClose} style={styles.close}>
            <Text style={[typography.label, { color: colors.coverInk }]}>Đóng</Text>
          </Pressable>
        </View>
        <View style={styles.flex} onLayout={(event) => {
          const nextWidth = event.nativeEvent.layout.width;
          if (nextWidth !== width) { setZoomed(false); setWidth(nextWidth); }
        }}>
          {width > 0 && photos.length > 0 ? <FlatList key={width} data={photos} horizontal pagingEnabled scrollEnabled={!zoomed}
            initialScrollIndex={currentIndex}
            getItemLayout={(_, i) => ({ length: width, offset: width * i, index: i })}
            keyExtractor={(photo) => photo.id} initialNumToRender={1} maxToRenderPerBatch={2} windowSize={3}
            showsHorizontalScrollIndicator={false}
            onMomentumScrollEnd={(event) => { setIndex(viewerIndex(event.nativeEvent.contentOffset.x / width, photos.length)); setZoomed(false); }}
            renderItem={({ item, index: itemIndex }) => <ZoomPhoto photo={item} width={width} active={itemIndex === currentIndex} onZoom={setZoomed} />}
          /> : null}
        </View>
        <Text style={[typography.caption, styles.hint, { color: colors.coverInkSoft }]}>Vuốt để xem tiếp · chụm hai ngón hoặc chạm đúp để phóng to</Text>
        <Text numberOfLines={4} style={[typography.body, styles.caption, { color: colors.coverInk }]}>{photos[currentIndex]?.caption}</Text>
      </SafeAreaView>
    </GestureHandlerRootView>
  </Modal>;
}

function ZoomPhoto({ photo, width, active, onZoom }: { photo: ViewerPhoto; width: number; active: boolean; onZoom(value: boolean): void }) {
  const motion = useMotion();
  const settle = motion.timing("standard");
  const scale = useSharedValue(1), savedScale = useSharedValue(1);
  const x = useSharedValue(0), y = useSharedValue(0), originX = useSharedValue(0), originY = useSharedValue(0);
  const [height, setHeight] = useState(0);
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
  return <View style={{ width, flex: 1, overflow: "hidden" }} onLayout={(event) => setHeight(event.nativeEvent.layout.height)}>
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
