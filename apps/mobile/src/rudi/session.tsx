import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from "react";

import { datChuSoHuuBanNhap } from "./chat/ban-nhap-cong-cu";
import { datTokenPhien, tokenPhienHienTai } from "../api";
import { ngheThuHoiPhien } from "../danh-tinh";
import { dangXuat, khoiPhucPhien, quenPhienDaThuHoi, type Phien } from "../phien";
import { xoaPhienAsync } from "./kho";
import { nguonHienTai, type Nguon } from "./nguon";

type RudiSessionApi = {
  /**
   * Where a screen's group data comes from, and the identity to read with.
   * Derived from the session and nothing else (`nguonHienTai`).
   */
  nguon: Nguon;
  /** The bearer session as restored or just minted; `null` while signed out. */
  phien: Phien | null;
  /** Whether SecureStore has answered. Until then `phien === null` means nothing. */
  phienDaDoc: boolean;
  /**
   * A session that just arrived, put into force without a relaunch.
   *
   * `src/phien.ts` owns the disk and the bearer, so signing in already writes
   * both. What it cannot do is tell this provider, and this provider is what
   * `nguon` is derived from -- so before this existed, somebody could redeem a
   * real invitation, land on the group, and read nothing until they killed
   * the app and opened it again.
   *
   * Takes the whole record rather than a flag, so accepting a membership
   * (`membership_state` goes `invited` -> `active`) travels through the same
   * one door as signing in.
   */
  datPhien: (phien: Phien) => void;
  /** Sign out: end the session on the server, forget it here. */
  resetSession: () => void;
  /**
   * Forget the session here only, at once: the server has already revoked it
   * («đăng xuất tất cả»). Sending `DELETE /sessions/current` with a dead
   * token would only earn a 401, and its late answer must not touch a
   * session signed in after it.
   */
  quenTaiCho: () => void;
};

const RudiSessionContext = createContext<RudiSessionApi | null>(null);

export function RudiSessionProvider({ children }: { children: ReactNode }) {
  const [phien, setPhien] = useState<Phien | null>(null);
  const [phienDaDoc, setPhienDaDoc] = useState(false);
  const nguon = useMemo(() => nguonHienTai(phien), [phien]);

  // The session, restored at launch by the module that owns it. `src/phien.ts`
  // (ADR-0014, PR 514) reads SecureStore, drops an expired record rather than
  // sending it, and hands the bearer to `src/api.ts`. This provider only needs
  // to know WHETHER there is one, and who it says we are.
  useEffect(() => {
    let song = true;
    // Builds before 2026-10-03 kept a demo draft (Team Đà Lạt) under
    // `rudi.phien.v1`. Nothing reads it any more; an install that still has
    // one drops it on the first launch of this build.
    void xoaPhienAsync();
    void khoiPhucPhien()
      .then((phien) => {
        if (!song) return;
        if (phien !== null) datTokenPhien(phien.token);
        datChuSoHuuBanNhap(phien?.person_id ?? null);
        setPhien(phien);
        setPhienDaDoc(true);
      })
      .catch(() => {
        // An unreadable store is indistinguishable from a first launch, and
        // both answers are the same: no session, the sign-in door.
        if (!song) return;
        datChuSoHuuBanNhap(null);
        setPhien(null);
        setPhienDaDoc(true);
      });
    return () => {
      song = false;
    };
  }, []);

  // A bearer the server no longer accepts (ADR-0055 revokes every session on
  // a password or email change, a Google link or unlink, a reset, «đăng xuất
  // tất cả»): `src/api.ts` reports it once, already cleared from memory, and
  // the app returns to the sign-in door instead of failing on every screen.
  useEffect(
    () =>
      ngheThuHoiPhien((token) => {
        datChuSoHuuBanNhap(null);
        setPhien((dangCo) => (dangCo !== null && dangCo.token !== token ? dangCo : null));
        void quenPhienDaThuHoi(token).catch(() => undefined);
      }),
    [],
  );

  const api: RudiSessionApi = useMemo(() => ({
    nguon,
    phien,
    phienDaDoc,
    datPhien: (moi: Phien) => {
      // `datTokenPhien` too, not just the state: `phien.ts` already set it on
      // the way in, but a caller that reached here with a record read from
      // somewhere else must not leave the bearer pointing at the old one.
      datTokenPhien(moi.token);
      datChuSoHuuBanNhap(moi.person_id);
      setPhien(moi);
    },
    resetSession: () => {
      datChuSoHuuBanNhap(null);
      // `dangXuat` calls the server FIRST and forgets locally in a `finally`,
      // which is the order that matters: a session only the phone forgets is
      // still a live credential on the server, and a phone somebody else is
      // holding is exactly when that matters. Clearing local state here
      // regardless is right for the same reason -- somebody who pressed
      // sign-out has said what they want.
      const dangCo = phien;
      setPhien(null);
      if (dangCo !== null) void dangXuat(dangCo.person_id);
    },
    quenTaiCho: () => {
      const token = phien?.token ?? tokenPhienHienTai();
      datChuSoHuuBanNhap(null);
      if (tokenPhienHienTai() === token) datTokenPhien(null);
      setPhien(null);
      if (token) void quenPhienDaThuHoi(token).catch(() => undefined);
    },
  }), [nguon, phien, phienDaDoc]);

  return <RudiSessionContext.Provider value={api}>{children}</RudiSessionContext.Provider>;
}

export function useRudiSession(): RudiSessionApi {
  const value = useContext(RudiSessionContext);
  if (!value) {
    throw new Error("useRudiSession must be used inside RudiSessionProvider");
  }
  return value;
}
