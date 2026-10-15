/** Synthetic behavioral witnesses on actual TSX. No native layout/paint or server atomicity claim. */
import assert from "node:assert/strict";
import { after, test } from "node:test";
import { React, act, withHost, closeHost, deferred } from "./n-behavior-host.mjs";
after(closeHost);

const actor = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const other = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";
const context = "cccccccc-cccc-4ccc-8ccc-cccccccccccc";
const batch = "synthetic-batch";
const amendment = "synthetic-amendment";
const roster = [{ id: actor, name: "Người thử A" }, { id: other, name: "Người thử B" }];
const oldLinks = [{ senderId: actor, senderName: "Người thử A", amountVnd: 31000, url: "https://synthetic.invalid/old", opened: false, obligations: [{ obligationId: "old-obligation", amountVnd: 31000 }] }];
const freshBoard = [{ id: "fresh-obligation", senderId: actor, recipientId: other, amountVnd: 47000, trangThai: "outstanding", tranhCai: false }];
const oldBoard = [{ ...freshBoard[0], id: "old-obligation", amountVnd: 31000 }];
const reviewLinks = [{ senderId: actor, url: "https://synthetic.invalid/new/dieu-chinh" }];
function wire(status = "proposed", pending = [actor]) {
  return {
    batch_owner_id: actor, batch_status: "published",
    expenses: [{ expense_id: "synthetic-expense", description: "Khoản thử", paid_by_id: other, recorded_by_id: actor, total_amount_vnd: 62000, allocations: [{ person_id: actor, amount_vnd: 31000 }, { person_id: other, amount_vnd: 31000 }] }],
    amendments: status === "none" ? [] : [{ id: amendment, status, expense_id: "synthetic-expense", proposed_by_id: other, reason: "Lý do thử", created_at: "2026-01-01T00:00:00Z", expires_at: "2026-01-02T00:00:00Z", resolved_at: status === "applied" ? "2026-01-01T01:00:00Z" : null,
      lines: [{ sender_id: actor, recipient_id: other, old_amount_vnd: 31000, new_amount_vnd: 47000 }],
      parties: [{ person_id: actor, decision: pending.includes(actor) ? null : "accept", via: "app" }], pending_person_ids: pending }],
  };
}
async function amendmentHost(h, overrides = {}) {
  const { DieuChinhDot } = await h.load("apps/mobile/src/rudi/screens/dot-thu/DieuChinhDot.tsx");
  // The legacy prop is supplied only so the identical harness can execute the
  // pre-change original and witness its stale-render board use. R3 ignores it.
  const props = { actorId: actor, contextId: context, batchId: batch, roster, links: oldLinks, bang: oldBoard, docLaiBang: async () => freshBoard, ...overrides };
  await h.render(React.createElement(DieuChinhDot, props));
  await h.flush();
  return { props, DieuChinhDot };
}
function finance(person_id = actor, spend_vnd = 91037) {
  return { person_id, display_name: null, spend_vnd, settled_vnd: 12013, outstanding_vnd: 79024, receivable_vnd: 67009, expense_count: 3, group_count: 2, movements: [] };
}
function response(value) { return { ok: true, status: 200, json: async () => value }; }

test("N01 status is read before fresh board; stored envelope uses that board amount and obligation ID", async () => {
  await withHost(async h => {
    let reads = 0;
    h.storage.set(`rudi.dot-thu.dieu-chinh.${amendment}`, JSON.stringify(reviewLinks));
    h.state.api = async (path, options) => {
      if (path.endsWith("/decision")) { h.events.push(["decision", options.body]); return { status: "applied" }; }
      h.events.push(["status", ++reads]);
      return reads === 1 ? wire() : wire("applied", []);
    };
    let boards = 0;
    await amendmentHost(h, { docLaiBang: async () => { h.events.push(["board", ++boards]); return freshBoard; } });
    h.events.length = 0;
    await h.press("Đồng ý với số mới");
    await h.flush();
    const steps = h.events.filter(e => ["decision", "status", "board", "storage-write"].includes(e[0])).map(e => e[0]);
    assert.deepEqual(steps, ["decision", "status", "board", "storage-write", "board"]);
    const stored = JSON.parse(h.storage.get(`rudi.dot-thu.link.${batch}`));
    assert.deepEqual(stored, [{ ...oldLinks[0], url: "https://synthetic.invalid/new", amountVnd: 47000, obligations: [{ obligationId: "fresh-obligation", amountVnd: 47000 }] }]);
    assert.ok(h.consumed.has("apps/mobile/src/rudi/dot-thu/dieu-chinh.ts"), "actual production mapper and link helper executed");
    assert.ok(h.consumed.has("apps/mobile/src/rudi/dot-thu/kho-link.ts"), "actual storage adapter executed");
    assert.ok(h.consumed.has("apps/mobile/src/rudi/ui/CauTaiCho.tsx"), "actual error renderer executed");
  });
});

test("N02 applied status plus unavailable fresh board never rewrites local links and still renders error after reply control disappears", async () => {
  await withHost(async h => {
    let reads = 0;
    h.storage.set(`rudi.dot-thu.dieu-chinh.${amendment}`, JSON.stringify(reviewLinks));
    h.state.api = async path => path.endsWith("/decision") ? { status: "applied" } : ++reads === 1 ? wire() : wire("applied", []);
    await amendmentHost(h, { docLaiBang: async () => null });
    await h.press("Đồng ý với số mới");
    await h.flush();
    assert.equal(h.events.filter(e => e[0] === "storage-write").length, 0);
    assert.equal(h.all("RNPressable").filter(node => node.props.accessibilityLabel === "Đồng ý với số mới").length, 0);
    const alerts = h.root.findAll(node => typeof node.type === "string" && node.props.accessibilityRole === "alert");
    assert.equal(alerts.length, 1, "reread error remains visible after actor has no reply action");
    assert.ok(alerts[0].findAllByType("RNText").some(node => node.children.join("").length > 0));
    assert.equal(h.all("RNPressable").filter(node => h.label(node) === "Sửa sau khi phát").length, 0, "error cannot be folded away");
  });
});

test("N03 rejected/error status paths do not rewrite local envelopes", async () => {
  for (const failure of ["not-applied", "status-error", "board-error"]) {
    await withHost(async h => {
      let reads = 0, boards = 0;
      h.storage.set(`rudi.dot-thu.dieu-chinh.${amendment}`, JSON.stringify(reviewLinks));
      h.state.api = async path => {
        if (path.endsWith("/decision")) return { status: "applied" };
        if (++reads === 1) return wire();
        if (failure === "status-error") throw new Error("synthetic reread error");
        return wire(failure === "not-applied" ? "rejected" : "applied", []);
      };
      await amendmentHost(h, { docLaiBang: async () => { boards++; if (failure === "board-error") throw new Error("synthetic board error"); return freshBoard; } });
      await h.press("Đồng ý với số mới");
      await h.flush();
      assert.equal(h.events.filter(e => e[0] === "storage-write").length, 0, failure);
      assert.equal(boards, failure === "status-error" ? 0 : 1, failure);
    });
  }
});

test("N04 idle disclosure collapses help, preserves draft when opened, active amendment never collapses", async () => {
  await withHost(async h => {
    h.state.api = async () => wire("none");
    await amendmentHost(h);
    assert.equal(h.byId("soan-dieu-chinh").length, 0);
    assert.equal(h.all("RNPressable").filter(n => h.label(n).startsWith("Đề xuất sửa «")).length, 0);
    await h.press("Sửa sau khi phát");
    const propose = h.all("RNPressable").find(n => h.label(n).startsWith("Đề xuất sửa «"));
    assert.ok(propose);
    await act(async () => { propose.props.onPress({}); });
    assert.equal(h.byId("soan-dieu-chinh").length, 1);
    assert.equal(h.all("RNPressable").filter(n => h.label(n) === "Gấp phần sửa sau khi phát").length, 0);
    const reason = h.all("RNTextInput").find(n => n.props.accessibilityLabel === "Lý do sửa");
    assert.ok(reason);
    await act(async () => { reason.props.onChangeText("Lý do nháp thử"); });
    assert.equal(h.all("RNTextInput").find(n => n.props.accessibilityLabel === "Lý do sửa").props.value, "Lý do nháp thử");
  });
  await withHost(async h => {
    h.state.api = async () => wire();
    await amendmentHost(h);
    assert.equal(h.byId("dieu-chinh-dang-mo").length, 1);
    assert.equal(h.all("RNPressable").filter(n => h.label(n) === "Sửa sau khi phát").length, 0);
    assert.ok(h.button("Đồng ý với số mới"));
  });
});

test("N05 finance focus refresh clears prior ledger and ignores late response from previous focus", async () => {
  await withHost(async h => {
    const reads = [];
    h.state.finance = (path, options) => { const item = deferred(); reads.push({ ...item, path, options }); return item.promise; };
    await h.render(await h.financeElement());
    assert.equal(reads.length, 1);
    assert.ok(reads[0].path.endsWith(`/people/${actor}/finance`));
    await act(async () => { reads[0].resolve(response(finance())); });
    assert.ok(h.texts().includes("91.037đ"));
    await h.focus(false);
    await h.focus(true);
    assert.equal(reads.length, 2, "focus must issue a fresh read");
    assert.equal(h.byId("trang-so-tai-chinh").length, 0, "stale ledger cleared while refetch pending");
    await h.focus(false);
    await h.focus(true);
    assert.equal(reads.length, 3);
    await act(async () => { reads[2].resolve(response(finance(actor, 73021))); });
    assert.ok(h.texts().includes("73.021đ"));
    await act(async () => { reads[1].resolve(response(finance(actor, 999991))); });
    assert.ok(h.texts().includes("73.021đ"));
    assert.equal(h.texts().includes("999.991đ"), false, "cleaned-up focus cannot replace current ledger");
    assert.equal(reads[2].options.headers["X-Actor-ID"], actor);
  });
});

test("N06 finance actor switch ignores late previous actor read and retry works after current failure", async () => {
  await withHost(async h => {
    const reads = [];
    h.state.finance = (path, options) => { const item = deferred(); reads.push({ ...item, path, options }); return item.promise; };
    await h.render(await h.financeElement());
    await h.actor(other);
    assert.equal(reads.length, 2);
    assert.ok(reads[1].path.endsWith(`/people/${other}/finance`));
    await act(async () => { reads[1].resolve(response(finance(other, 73021))); });
    await act(async () => { reads[0].resolve(response(finance(actor, 999991))); });
    assert.ok(h.texts().includes("73.021đ"));
    assert.equal(h.texts().includes("999.991đ"), false);
    await h.focus(false);
    await h.focus(true);
    assert.equal(reads.length, 3, "refocus after actor switch must read current actor again");
    await act(async () => { reads[2].reject(new Error("synthetic disconnected")); });
    assert.equal(h.byId("trang-so-tai-chinh").length, 0);
    await h.press("Thử lại");
    assert.equal(reads.length, 4);
    await act(async () => { reads[3].resolve(response(finance(other, 87031))); });
    assert.ok(h.texts().includes("87.031đ"));
  });
});

test("N07 FinanceScreen does not emit prior actor's loaded ledger during actor switch, in both session branches", async () => {
  for (const context_id of [null, context]) {
    await withHost(async h => {
      const reads = [];
      h.state.restoredSession.context_id = context_id;
      h.state.restoredSession.membership_state = context_id === null ? null : "active";
      h.state.finance = (path, options) => { const item = deferred(); reads.push({ ...item, path, options }); return item.promise; };
      await h.render(await h.financeElement());
      await act(async () => { reads[0].resolve(response(finance())); });
      assert.ok(h.texts().includes("91.037đ"));
      h.nativeRenders.length = 0;
      await h.actor(other);
      assert.equal(reads.length, 2);
      assert.equal(h.nativeRenders.some(render => render.label === "91.037đ"), false, "no host render may receive old ledger amount after the actor changed");
      assert.equal(h.byId("trang-so-tai-chinh").length, 0);
      await act(async () => { reads[1].resolve(response(finance(other, 73021))); });
      assert.ok(h.texts().includes("73.021đ"));
    });
  }
});

test("N08 pending post-submit reread cannot collapse while draft has already closed", async () => {
  await withHost(async h => {
    const reread = deferred();
    let reads = 0;
    h.state.api = async (_path, options) => {
      if (options.method === "GET") return ++reads === 1 ? wire("none") : reread.promise;
      return { amendment_id: amendment, status: "applied", review_links: [] };
    };
    await amendmentHost(h);
    await h.press("Sửa sau khi phát");
    const propose = h.all("RNPressable").find(n => h.label(n).startsWith("Đề xuất sửa «"));
    await act(async () => { propose.props.onPress({}); });
    await act(async () => { h.all("RNTextInput").find(n => n.props.accessibilityLabel === "Lý do sửa").props.onChangeText("Lý do đang gửi"); });
    await h.press("Gửi đề xuất");
    assert.equal(reads, 2);
    assert.equal(h.byId("soan-dieu-chinh").length, 0, "submit closed draft before reread resolves");
    assert.equal(h.all("RNPressable").filter(n => ["Sửa sau khi phát", "Gấp phần sửa sau khi phát"].includes(h.label(n))).length, 0, "busy state must not expose idle fold control");
    await act(async () => { reread.resolve(wire("applied", [])); });
    assert.ok(h.button("Gấp phần sửa sau khi phát"));
  });
});

test("N09 actual board parent passes the fresh reread to amendment reconciliation", async () => {
  await withHost(async h => {
    let statusReads = 0, boardReads = 0;
    h.storage.set(`rudi.dot-thu.link.${batch}`, JSON.stringify(oldLinks));
    h.storage.set(`rudi.dot-thu.dieu-chinh.${amendment}`, JSON.stringify(reviewLinks));
    h.state.api = async (path, options) => {
      if (path.endsWith("/decision")) return { status: "applied" };
      if (path.endsWith("/amendments")) { h.events.push(["status", ++statusReads]); return statusReads === 1 ? wire() : wire("applied", []); }
      if (path.endsWith("/obligations")) {
        h.events.push(["board", ++boardReads]);
        const amount = boardReads === 1 ? 31000 : 47000;
        return { disputed_count: 0, obligations: [{ obligation_id: boardReads === 1 ? "old-obligation" : "fresh-obligation", sender_id: actor, recipient_id: other, amount_vnd: amount, obligation_status: "outstanding", disputed: false }] };
      }
      if (path.endsWith("/batches")) return { batches: [{ batch_id: batch, status: "published", created_at: "2026-01-01T00:00:00Z", published_at: "2026-01-01T00:00:00Z", obligation_count: 1, confirmed_count: 0, disputed_count: 0, total_vnd: 31000 }] };
      if (path.endsWith("/members")) return { members: roster.map(r => ({ person_id: r.id, display_name: r.name, membership_state: "active" })) };
      throw new Error(`unexpected parent integration API: ${options.method} ${path}`);
    };
    const { DotThuLiveScreen } = await h.load("apps/mobile/src/rudi/screens/dot-thu/DotThuLive.tsx");
    await h.render(React.createElement(DotThuLiveScreen, { phien: { person_id: actor, context_id: context, membership_state: "active", token: "synthetic-host-only" }, batchId: batch }));
    await h.flush();
    assert.ok(h.button("Đồng ý với số mới"));
    h.events.length = 0;
    await h.press("Đồng ý với số mới");
    await h.flush();
    const stored = JSON.parse(h.storage.get(`rudi.dot-thu.link.${batch}`));
    assert.equal(stored[0].amountVnd, 47000);
    assert.deepEqual(stored[0].obligations, [{ obligationId: "fresh-obligation", amountVnd: 47000 }]);
    assert.deepEqual(h.events.filter(e => ["status", "board", "storage-write"].includes(e[0])).map(e => e[0]), ["status", "board", "storage-write", "board"]);
    assert.ok(h.texts().includes("47.000đ"));
  });
});

function nativeSnapshot(scale) {
  // Independent synthetic mappings, not Android measurements. The complete
  // existing module vocabulary permits the real wrapper's manual path.
  const bases = [-0.65, -0.3, -0.15, 0.4, 0.8, 1.6, 11, 12, 13, 14, 15, 17, 18, 19, 21, 22, 23, 24, 27, 28, 34];
  return { status: "available", fontScale: scale, fontMetrics: { density: 2, spToDp: bases.map(base => ({ base, dp: base * scale + (base > 0 ? 0.25 : -0.025) })) } };
}

test("N10 actual finance font observer updates stable metrics without resetting loaded ledger or refetching", async () => {
  await withHost(async h => {
    h.state.nativeSnapshot = nativeSnapshot(1);
    let reads = 0;
    h.state.finance = async () => { reads++; return response(finance()); };
    await h.render(await h.financeElement());
    assert.equal(h.nativeFontObservers, 1, "actual finance root must mount one native font observer");
    const money = h.all("RNText").find(n => n.props.accessibilityLabel === "91.037đ");
    assert.ok(money);
    assert.equal(money.props.allowFontScaling, false, "real metrics provider is active for the real Money wrapper");
    assert.equal(h.rn.StyleSheet.flatten(money.props.style).fontSize, 14.25);
    await h.font(nativeSnapshot(2));
    assert.equal(reads, 1, "font event cannot restart financial read");
    assert.equal(h.all("RNText").find(n => n.props.accessibilityLabel === "91.037đ") === money, true, "font event preserves the mounted Money host");
    assert.equal(h.rn.StyleSheet.flatten(money.props.style).fontSize, 28.25, "the native event reaches the real wrapper while the ledger host stays mounted");
    assert.ok(h.texts().includes("91.037đ"));
    assert.equal(h.nativeFontObservers, 1, "font event cannot duplicate subscriptions");
  });
});

test("N11 actual board font observer preserves open amendment draft and its typed reason on hot change", async () => {
  await withHost(async h => {
    h.state.nativeSnapshot = nativeSnapshot(1);
    let statusReads = 0, boardReads = 0;
    h.state.api = async path => {
      if (path.endsWith("/amendments")) { statusReads++; return wire("none"); }
      if (path.endsWith("/obligations")) { boardReads++; return { disputed_count: 0, obligations: [] }; }
      if (path.endsWith("/batches")) return { batches: [{ batch_id: batch, status: "published", created_at: "2026-01-01T00:00:00Z", published_at: "2026-01-01T00:00:00Z", obligation_count: 0, confirmed_count: 0, disputed_count: 0, total_vnd: 0 }] };
      if (path.endsWith("/members")) return { members: roster.map(r => ({ person_id: r.id, display_name: r.name, membership_state: "active" })) };
      throw new Error(`unexpected font/draft fixture API: ${path}`);
    };
    const { DotThuLiveScreen } = await h.load("apps/mobile/src/rudi/screens/dot-thu/DotThuLive.tsx");
    await h.render(React.createElement(DotThuLiveScreen, { phien: { person_id: actor, context_id: context, membership_state: "active", token: "synthetic-host-only" }, batchId: batch }));
    await h.flush();
    assert.equal(h.nativeFontObservers, 1);
    await h.press("Sửa sau khi phát");
    const propose = h.all("RNPressable").find(n => h.label(n).startsWith("Đề xuất sửa «"));
    await act(async () => { propose.props.onPress({}); });
    const reason = h.all("RNTextInput").find(n => n.props.accessibilityLabel === "Lý do sửa");
    await act(async () => { reason.props.onChangeText("Nháp qua đổi cỡ chữ"); });
    const before = { statusReads, boardReads };
    await h.font(nativeSnapshot(2));
    const current = h.all("RNTextInput").find(n => n.props.accessibilityLabel === "Lý do sửa");
    assert.ok(current, "font event cannot discard the open draft");
    assert.equal(current === reason, true, "hot configuration preserves actual ruled-input host");
    assert.equal(current.props.value, "Nháp qua đổi cỡ chữ");
    assert.equal(h.byId("soan-dieu-chinh").length, 1);
    assert.deepEqual({ statusReads, boardReads }, before, "font event cannot restart board/amendment reads");
    assert.equal(h.nativeFontObservers, 1);
  });
});
