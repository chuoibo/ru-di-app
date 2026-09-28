/**
 * The avatar stream: one socket per signed-in app, open while it is in the
 * foreground, so a friend's new picture reaches every screen without a reload.
 *
 * The socket only carries hints. The server says "ready" after every
 * (re)connect and whenever its own listener restarted; the client answers by
 * asking `GET /people/avatars` again for everyone it has drawn. So anything
 * missed while the socket was down -- background, network loss, a server
 * restart -- is repaired by the resync, never assumed delivered.
 *
 * Timers and the socket are injected so `tests/luong-anh-dai-dien.test.mjs`
 * can drive reconnects and pauses under bare node.
 */
import { lamMoiTatCa, nhanSuKien } from "./anh-dai-dien";

export interface SocketToiThieu {
  readyState: number;
  onopen: (() => void) | null;
  onmessage: ((e: { data: unknown }) => void) | null;
  onclose: (() => void) | null;
  onerror: (() => void) | null;
  send(data: string): void;
  close(): void;
}

export interface LuaChonLuong {
  /** `ws(s)://host/people/avatars/stream`. */
  url: string;
  actorId: string;
  /** Read at every connect: a refreshed session must not reuse a stale bearer. */
  token: () => string | null;
  taoSocket: (url: string) => SocketToiThieu;
  hen?: (f: () => void, ms: number) => unknown;
  huyHen?: (h: unknown) => void;
  ngauNhien?: () => number;
}

export interface Luong {
  /** Back in the foreground: resync now and reconnect. */
  tiepTuc(): void;
  /** Into the background: close and stay closed. */
  tamDung(): void;
  /** Signed out or unmounted: close for good. */
  dung(): void;
}

/** Reconnect delay after `lanHong` consecutive failures: 0.5 s doubling to 30 s, plus up to 250 ms jitter. */
export function choTruocKhiNoiLai(lanHong: number, ngauNhien: () => number = Math.random): number {
  return Math.min(30_000, 500 * 2 ** Math.min(Math.max(lanHong - 1, 0), 6)) + Math.floor(ngauNhien() * 250);
}

export function moLuongAnhDaiDien(o: LuaChonLuong): Luong {
  const hen = o.hen ?? ((f: () => void, ms: number) => setTimeout(f, ms));
  const huyHen = o.huyHen ?? ((h: unknown) => clearTimeout(h as ReturnType<typeof setTimeout>));
  let socket: SocketToiThieu | null = null;
  let henNoiLai: unknown = null;
  let lanHong = 0;
  let dangChay = true;
  let daDung = false;

  const noi = () => {
    if (!dangChay || daDung || socket !== null) return;
    const token = o.token();
    if (!token) return;
    const s = o.taoSocket(o.url);
    socket = s;
    s.onopen = () => {
      if (socket !== s) return;
      s.send(JSON.stringify({ type: "authenticate", token }));
    };
    s.onmessage = (e) => {
      if (socket !== s) return;
      let data: unknown;
      try {
        data = JSON.parse(String(e.data));
      } catch {
        s.close();
        return;
      }
      if ((data as { type?: unknown })?.type === "ready") lanHong = 0;
      nhanSuKien(data, o.actorId);
    };
    s.onerror = () => s.close();
    s.onclose = () => {
      if (socket !== s) return;
      socket = null;
      if (!dangChay || daDung) return;
      lanHong += 1;
      henNoiLai = hen(() => {
        henNoiLai = null;
        noi();
      }, choTruocKhiNoiLai(lanHong, o.ngauNhien));
    };
  };

  const dong = () => {
    if (henNoiLai !== null) {
      huyHen(henNoiLai);
      henNoiLai = null;
    }
    const s = socket;
    socket = null;
    s?.close();
  };

  noi();
  return {
    tiepTuc() {
      if (daDung) return;
      dangChay = true;
      lamMoiTatCa(o.actorId);
      lanHong = 0;
      noi();
    },
    tamDung() {
      dangChay = false;
      dong();
    },
    dung() {
      daDung = true;
      dong();
    },
  };
}
