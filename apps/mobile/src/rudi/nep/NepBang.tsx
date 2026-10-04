import { useEffect, useRef, useState } from "react";
import { Keyboard, Platform, Pressable, ScrollView, StyleSheet, Text, TextInput, View, type TextStyle } from "react-native";

import { useRudiSession } from "../session";
import { typography, useRudiTheme } from "../theme";
import { Nep } from "../ui/art/Nep";
import type { PoseNep } from "../art/nep";
import { RudiButton } from "../ui";
import { anhDanhMuc, MediaSlot } from "../ui/MediaSlot";
import { Sheet } from "../ui/Sheet";
import { cauPhienDiKem, gomPhien, nepDuocHoi } from "./hoi";
import { cauNguCanh } from "./phieu";
import { useNep } from "./NepProvider";
import { useNepAnh } from "./useNepAnh";
import { useNepHoi } from "./useNepHoi";
import { NepPhien } from "./NepPhien";
import { useMotion } from "../ui/useMotion";
import { KHONG_VIEN_WEB } from "../ui/khong-vien-web";
import { NepPhim } from "./NepPhim";
import { PressScale } from "../ui/PressScale";

type LoiNhoVe = { moTa: string; man: string | undefined; actorId: string | null };

/**
 * The panel Nếp talks in.
 *
 * It reuses `ui/Sheet.tsx` rather than a route, because the assistant is not a
 * place in the app: Back, Escape and a drag on the handle all close it, and the
 * screen underneath never navigates.
 *
 * The first block is «what Nếp can see», printed from the context slip the open
 * screen declared. That is not decoration. `CLAUDE.md` forbids the assistant
 * reading chat, taste or history on its own, so the honest way to be
 * context-aware is to show the person the whole of what was shared and nothing
 * more. If the line looks thin, that is the point: it is the real payload.
 *
 * The block's second line counts the turns of this panel session that go with
 * the next question (ADR-0036 §2.5, §2.7): the preview is built from the very
 * list `goiNep` sends, not re-derived from the screen. The session lives in
 * `useNepHoi`'s state and ends when the panel closes. The answer comes back to
 * this person alone and is shown here, never in any room (§2.8).
 */

/** What Nếp holds, by the screen the panel was opened from. */
function tuTheTheoMan(man: string | undefined): PoseNep {
  if (man === undefined) return "gop-y";
  if (man.includes("to-giay")) return "dua-giay";
  if (man.includes("chat") || man === "messages") return "goi-loi";
  if (man === "explore" || man.startsWith("places")) return "cam-ban-do";
  if (man === "plan" || man.startsWith("outings")) return "ghi-lai";
  return "gop-y";
}

export function NepBang({ open, onClose }: { open: boolean; onClose(): void }) {
  const { phieu } = useNep();
  const { colors } = useRudiTheme();
  const { reduced } = useMotion();
  const { nguon } = useRudiSession();
  const actorId = nguon.kieu === "live" ? nguon.actorId : null;
  const buc = useNepAnh(nguon.kieu === "live" ? nguon.actorId : null);
  const [nhap, datNhap] = useState("");
  const [dangNhap, datDangNhap] = useState(false);
  const [loiNhoVe, datLoiNhoVe] = useState<LoiNhoVe | null>(null);
  const oNhap = useRef<TextInput>(null);
  const tieuDeVe = useRef<Text>(null);
  const focusFrame = useRef<number | null>(null);
  const veLock = useRef(false);
  const phien = useNepHoi(nguon.kieu === "live" ? nguon.actorId : null, phieu, open);
  const duocHoi = nepDuocHoi(phieu);
  // Consent belongs to this screen, person and exact description, not the
  // next editor value or a different account that restored in the meantime.
  const xacNhan = open && duocHoi && loiNhoVe?.man === phieu?.man && loiNhoVe?.actorId === actorId ? loiNhoVe : null;
  useEffect(() => { datLoiNhoVe(null); veLock.current = false; }, [open, phieu?.man, actorId]);
  useEffect(() => () => { if (focusFrame.current !== null) cancelAnimationFrame(focusFrame.current); }, [open]);
  useEffect(() => {
    if (!xacNhan || Platform.OS !== "web") return;
    const frame = requestAnimationFrame(() => (tieuDeVe.current as unknown as HTMLElement | null)?.focus({ preventScroll: true }));
    return () => cancelAnimationFrame(frame);
  }, [xacNhan]);
  const huyVe = () => {
    datLoiNhoVe(null);
    // The existing Sheet retains protected focus; return to the editor that
    // still holds the description rather than losing the draft on cancel.
    focusFrame.current = requestAnimationFrame(() => {
      focusFrame.current = null;
      if (Platform.OS === "web") (oNhap.current as unknown as HTMLElement | null)?.focus({ preventScroll: true });
      else oNhap.current?.focus();
    });
  };
  const dongBang = () => { if (xacNhan) huyVe(); else onClose(); };
  const xacNhanVe = () => {
    if (!xacNhan || buc.dangCho || veLock.current) return;
    veLock.current = true;
    datLoiNhoVe(null);
    void buc.nhoVe(xacNhan.moTa, xacNhan.man).finally(() => { veLock.current = false; });
  };
  // Exactly the turns the next send carries, so the count cannot drift from the payload.
  const soLuotDiKem = gomPhien(phien.luot, nhap.trim()).length;
  const guiCau = async () => {
    const cau = nhap.trim();
    if (!cau || phien.dangHoi || !duocHoi) return;
    if (await phien.hoi(cau)) datNhap("");
  };

  const goiY = phieu?.goiY ?? [];
  const hanhDongVe = (
    <View style={[styles.chanVe, { borderColor: colors.line }]}>
      <Text style={[typography.body, { color: colors.inkSoft }]}>Đây là một lượt vẽ mới, thường mất khoảng hai phút. Nếp chỉ nhận mô tả này và tên màn đang mở.</Text>
      <View style={styles.hanhDong}>
      <RudiButton full={false} label="Sửa mô tả" onPress={huyVe} variant="outline" />
      <RudiButton full={false} label="Vẽ đi" onPress={xacNhanVe} tone="ai" />
      </View>
    </View>
  );

  const soan = (
    <View style={[styles.soan, { borderColor: dangNhap ? colors.ai : colors.line }]}>
        {phien.loi ? (
          <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.ink }]} testID="nep-loi-hoi">
            {phien.loi}
          </Text>
        ) : null}
        {buc.loi ? (
          <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.ink }]} testID="nep-loi-ve">
            {buc.loi}
          </Text>
        ) : null}
        {buc.dangCho ? (
          <Text accessibilityLiveRegion="polite" style={[typography.body, { color: colors.inkSoft }]} testID="nep-dang-ve">
            Mình đang vẽ, tầm hai phút. Bạn cứ làm việc khác, xong mình báo.
          </Text>
        ) : null}
        <TextInput
          ref={oNhap}
          accessibilityLabel="Hỏi Nếp"
          multiline
          numberOfLines={3}
          submitBehavior="submit"
          textAlignVertical="top"
          onFocus={() => datDangNhap(true)}
          onBlur={() => datDangNhap(false)}
          onChangeText={datNhap}
          onSubmitEditing={() => void guiCau()}
          placeholder="Hỏi Nếp một câu"
          placeholderTextColor={colors.inkFaint}
          returnKeyType="send"
          style={[typography.body, styles.o, { color: colors.ink }, KHONG_VIEN_WEB,
            Platform.OS === "web" ? { scrollbarWidth: "thin", scrollbarColor: `${colors.lineStrong} ${colors.card}` } as TextStyle : null]}
          testID="nep-o-nhap"
          value={nhap}
        />
        <View style={styles.hanhDong}>
        {/* The kit button, not a hand-rolled one: it resolves its own ink from
            the tone (`colors[`${tone}Ink`]`), and `tests/rudi-khong-hex.test.mjs`
            allows no file but `theme.ts` to spell a colour. */}
        {/* Vẽ là tool vòng 2 trong bảng quyền: nó GHI và nó tốn một lượt quota
            thật, nên hỏi mỗi lần, không nhớ câu trả lời trước. Nó câm ở màn
            tiền y như chữ (ADR-0036 §2.9): `duocHoi` gác cả hai nút, và máy
            chủ gác lại bằng `man`. */}
        <RudiButton
          compact
          disabled={!nhap.trim() || buc.dangCho || !duocHoi}
          full={false}
          label="Vẽ"
          loading={buc.dangCho}
          onPress={() => {
            const moTa = nhap.trim();
            if (!moTa || !duocHoi || buc.dangCho || veLock.current) return;
            Keyboard.dismiss();
            datLoiNhoVe({ moTa, man: phieu?.man, actorId });
          }}
          tone="ai"
          variant="outline"
        />
        <RudiButton
          compact
          disabled={!nhap.trim() || phien.dangHoi || !duocHoi}
          full={false}
          label="Gửi"
          loading={phien.dangHoi}
          onPress={() => void guiCau()}
          tone="ai"
        />
        </View>
      </View>
  );

  return (
    <Sheet avoidKeyboard showsVerticalScrollIndicator footer={xacNhan ? hanhDongVe : soan} accessibilityLabel="Nếp" onBack={dongBang} onClose={() => { datLoiNhoVe(null); onClose(); }} open={open} testID="nep-bang">
      <View style={styles.dau}>
        <View style={styles.dauChu}>
          <Text style={[typography.title, { color: colors.ink }]}>Nếp</Text>
          <Text style={[typography.caption, { color: colors.inkSoft }]}>
            {nguon.kieu === "live" ? "Trợ lý riêng của bạn" : "Đăng nhập để trò chuyện"}
          </Text>
        </View>
      </View>

      {xacNhan ? (
        <View style={styles.xacNhan} testID="nep-xac-nhan-ve" onLayout={() => {
          // Android may move focus to the covered search field when the
          // editor unmounts. Dismiss after the native preview has laid out,
          // not only before its removal; this step has no editable field.
          if (Platform.OS !== "web") Keyboard.dismiss();
        }}>
          <View style={styles.dauVe}>
            <Nep gap="trang" pose="dua-giay" size={72} />
            <Text ref={tieuDeVe} {...(Platform.OS === "web" ? { tabIndex: -1 } : {})} role="heading" style={[typography.h1, styles.dauChu, { color: colors.ink }]}>Nhờ Nếp vẽ?</Text>
          </View>
          <Text style={[typography.body, { color: colors.inkSoft }]}>Mô tả sẽ gửi:</Text>
          <Text style={[typography.body, styles.moTa, { color: colors.ink, borderColor: colors.line }]}>{xacNhan.moTa}</Text>
        </View>
      ) : (
      <>

      {/* Nếp says what it sees, in a speech bubble, holding what fits the
          screen it was opened from (ADR-0037 D1, D5): a map on Khám phá, a
          sheet in the two-person notebook. */}
      <View style={styles.noi}>
        <Nep gap="trang" pose={tuTheTheoMan(phieu?.man)} size={96} />
        <View style={[styles.the, styles.bongNoi, { backgroundColor: colors.aiSoft, borderColor: colors.ai }]}>
          <View style={[styles.duoiNoi, { backgroundColor: colors.aiSoft, borderColor: colors.ai }]} />
          {/* `aiInk` is ink ON the solid ai colour (white in light mode); on `aiSoft`
              it vanished. `ai` reads on `aiSoft` in both themes. */}
          <Text style={[typography.label, { color: colors.ai }]}>Phần sẽ gửi cùng lời nhờ</Text>
          <Text style={[typography.body, { color: colors.ink }]} testID="nep-ngu-canh">
            {cauNguCanh(phieu)}
          </Text>
          <Text style={[typography.caption, { color: colors.inkSoft }]} testID="nep-phien-di-kem">
            {cauPhienDiKem(soLuotDiKem)}
          </Text>
        </View>
      </View>

      {goiY.length > 0 ? (
        <ScrollView contentContainerStyle={styles.goiY} horizontal showsHorizontalScrollIndicator={false}>
          {goiY.map((g) => (
            <PressScale
              accessibilityRole="button"
              accessibilityLabel={g}
              key={g}
              haptic="select"
              onPress={() => datNhap(g)}
              style={[styles.chip, { borderColor: colors.lineStrong, backgroundColor: colors.paper }]}
            >
              <Text style={[typography.caption, { color: colors.ink }]}>{g}</Text>
            </PressScale>
          ))}
        </ScrollView>
      ) : null}

      {/* The session and the answer being written. Never beside money: the
          panel cannot be asked there (`duocHoi`), and an answer still in
          flight when the screen turns into a money screen stops showing. */}
      {duocHoi ? (
        <NepPhien
          cauDangHoi={phien.cauDangHoi}
          chips={phien.chips}
          dangDo={phien.dangDo}
          dangHoi={phien.dangHoi}
          giamChuyenDong={reduced}
          luot={phien.luot}
          onChip={datNhap}
          song={phien.song}
        />
      ) : null}

      {!duocHoi ? (
        <Text style={[typography.body, styles.loi, { color: colors.inkSoft }]} testID="nep-im-man-tien">
          Ở màn tiền Nếp không trả lời. Ra màn khác rồi hỏi nhé.
        </Text>
      ) : null}

      {buc.nguonAnh ? (
        <Pressable accessibilityRole="button" onPress={buc.dep} style={styles.khungAnh}>
          {/* Qua `MediaSlot` chứ không phải một `<Image>` trần: ADR-0017 §2.5 nói
              một tấm ảnh không bao giờ đi mà thiếu xuất xứ, và ảnh này CÓ xuất
              xứ đáng nói. Nó do máy vẽ, và mọi ảnh Gemini sinh ra đều mang dấu
              SynthID không tắt được, nên người xem đáng được biết cả hai điều
              đó ngay cạnh bức tranh chứ không phải trong một trang trợ giúp. */}
          <MediaSlot
            alt="Bức Nếp vừa vẽ"
            contentFit="contain"
            nguon={{
              loai: "danh-muc",
              anh: anhDanhMuc(
                buc.nguonAnh,
                { author: "Nếp", license: "AI vẽ, có dấu SynthID" },
              ),
            }}
            ratio={1}
            testID="nep-anh-ve"
          />
        </Pressable>
      ) : null}

      <NepPhim actorId={nguon.kieu === "live" ? nguon.actorId : null} imageJobIds={buc.anhDaVe} />


      </>
      )}
    </Sheet>
  );
}

const styles = StyleSheet.create({
  dau: { flexDirection: "row", alignItems: "center", gap: 12, marginBottom: 8 },
  noi: { flexDirection: "row", alignItems: "flex-end", gap: 10 },
  bongNoi: { flex: 1 },
  // The bubble's tail, pointing at Nếp: a rotated square half under the bubble.
  duoiNoi: { position: "absolute", left: -7, bottom: 22, width: 12, height: 12, borderLeftWidth: 1, borderBottomWidth: 1, transform: [{ rotate: "45deg" }] },
  dauChu: { flex: 1 },
  the: { borderRadius: 14, borderWidth: StyleSheet.hairlineWidth, padding: 12, gap: 4 },
  goiY: { gap: 8, paddingVertical: 12 },
  chip: { minHeight: 48, justifyContent: "center", borderRadius: 999, borderWidth: StyleSheet.hairlineWidth, paddingHorizontal: 12, paddingVertical: 8 },
  loi: { marginTop: 12 },
  khungAnh: { marginTop: 12, borderRadius: 14, overflow: "hidden" },
  soan: { gap: 8, marginTop: 16, borderTopWidth: StyleSheet.hairlineWidth, paddingTop: 12 },
  o: { minHeight: 72, maxHeight: 144, paddingVertical: 10 },
  hanhDong: { flexDirection: "row", flexWrap: "wrap", justifyContent: "flex-end", gap: 12 },
  chanVe: { borderTopWidth: StyleSheet.hairlineWidth, marginTop: 12, paddingTop: 12, gap: 12 },
  xacNhan: { gap: 16, paddingVertical: 8 },
  dauVe: { flexDirection: "row", alignItems: "center", gap: 16 },
  moTa: { paddingVertical: 12, borderTopWidth: StyleSheet.hairlineWidth, borderBottomWidth: StyleSheet.hairlineWidth },
});
