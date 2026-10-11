/** Actual production Text/StampButton/draft; native glyph paint and IME are outside this host. */
import assert from "node:assert/strict";
import { after, test } from "node:test";
import { React, act, withHost, closeHost } from "./n-behavior-host.mjs";
after(closeHost);
const actor = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa";
const other = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb";
const context = "cccccccc-cccc-4ccc-8ccc-cccccccccccc";
const batch = "synthetic-batch";
const phrase = "Gửi đề xuất";
function snapshot(scale) {
  // Independent synthetic conversions; no Android measurement is represented.
  const bases = [-0.65,-0.3,-0.15,0.4,0.8,1.6,11,12,13,14,15,17,18,19,21,22,23,24,27,28,34];
  return { status:"available", fontScale:scale, fontMetrics:{density:2, spToDp:bases.map(base=>({base,dp:base*scale+(base>0?0.25:-0.025)}))} };
}
function change(base, fontBase, dp) {
  const value=structuredClone(base);
  value.fontMetrics.spToDp.find(row=>row.base===fontBase).dp=dp;
  return value;
}
const unavailable={status:"unavailable",reason:"module-unavailable"};
function nativeLabel(h, label=phrase) {
  const matches=h.all("RNText").filter(n=>n.props.importantForAccessibility!=="no-hide-descendants"&&n.children.length===1&&n.children[0]===label);
  assert.equal(matches.length,1,`one actual native label: ${label}`);
  return matches[0];
}
async function stampFixture(h) {
  const {NativeFontMetricsProvider,Text,TextInput}=await h.load("apps/mobile/src/rudi/ui/NativeFontMetrics.tsx");
  const {StampButton}=await h.load("apps/mobile/src/rudi/ui/StampButton.tsx");
  const {PressScale}=await h.load("apps/mobile/src/rudi/ui/PressScale.tsx");
  function StatefulDraft({metrics,loading=false,disabled=false}) {
    const [reason,setReason]=React.useState("");
    const [sent,setSent]=React.useState(0);
    return React.createElement(NativeFontMetricsProvider,{snapshot:metrics},
      React.createElement(h.rn.View,{testID:"draft-parent"},
        React.createElement(TextInput,{testID:"draft-input",value:reason,onChangeText:setReason,style:{fontSize:17,lineHeight:22,letterSpacing:0.4}}),
        React.createElement(Text,{testID:"send-count"},String(sent)),
        React.createElement(StampButton,{label:phrase,size:"vua",tone:"split",testID:"actual-stamp",loading,disabled,onPress:()=>setSent(n=>n+1)})));
  }
  return {StampButton,PressScale,render:metrics=>h.render(React.createElement(StatefulDraft,{metrics}))};
}
function sealNodes(h,fixture) {
  return {
    stamp:h.root.findByType(fixture.StampButton),press:h.root.findByType(fixture.PressScale),
    button:h.byId("actual-stamp")[0],body:h.all("RNView").find(n=>typeof n.props.onLayout==="function"),
    input:h.byId("draft-input")[0],parent:h.byId("draft-parent")[0],rim:h.all("Svg")[0],label:nativeLabel(h),
  };
}
function stableSeal(h,fixture,before) {
  const now=sealNodes(h,fixture);
  for(const key of ["stamp","press","button","body","input","parent","rim"])assert.equal(now[key]===before[key],true,`${key} must stay mounted`);
  assert.equal(now.input.props.value,"Nháp giữ nguyên");
  assert.equal(now.rim.props.width,145,"measured StampButton box state is retained");
  return now;
}
test("B01 hot 200→100→200 replaces only actual StampButton native label and preserves parent/input/rim/action state",async()=>{
  await withHost(async h=>{
    const fixture=await stampFixture(h);await fixture.render(snapshot(2));
    const input=h.byId("draft-input")[0];
    await act(async()=>{input.props.onChangeText("Nháp giữ nguyên");h.all("RNView").find(n=>n.props.onLayout).props.onLayout({nativeEvent:{layout:{width:145,height:52}}});});
    const before=sealNodes(h,fixture);
    assert.equal(before.label.props.allowFontScaling,false);
    assert.equal(h.rn.StyleSheet.flatten(before.label.props.style).fontSize,34.25);
    assert.equal(Object.hasOwn(before.label.props,"invalidateOnMetricsChange"),false,"opt-in never leaks into native props");
    await fixture.render(snapshot(1));
    const after=stableSeal(h,fixture,before);
    assert.equal(after.label===before.label,false,"200→100 must invalidate only the native label leaf");
    assert.equal(after.label.children[0],phrase);
    assert.equal(h.rn.StyleSheet.flatten(after.label.props.style).fontSize,17.25);
    assert.equal(after.button.props.accessibilityLabel,phrase);
    assert.equal(after.button.props.accessibilityState.disabled,false);
    await fixture.render(snapshot(2));
    const back=stableSeal(h,fixture,before);
    assert.equal(back.label===after.label,false,"100→200 must also invalidate the native label leaf");
    await h.press(phrase);
    assert.equal(h.byId("send-count")[0].children[0],"1","same parent receives the enabled production action exactly once");
    assert.equal(h.byId("draft-input")[0].props.value,"Nháp giữ nguyên");
  });
});
test("B02 snapshot identity/fontScale/density/unrelated rows cannot churn a label whose effective metrics are unchanged",async()=>{
  await withHost(async h=>{
    const fixture=await stampFixture(h);const base=snapshot(1);await fixture.render(base);
    const before=nativeLabel(h);
    const metadataOnly=structuredClone(base);metadataOnly.fontScale=2;metadataOnly.fontMetrics.density=3;
    metadataOnly.fontMetrics.spToDp.find(row=>row.base===34).dp=98.5;
    for(const same of [structuredClone(base),metadataOnly,structuredClone(base)]){
      await fixture.render(same);assert.equal(nativeLabel(h)===before,true,"effective label metrics, not snapshot identity/scale, control invalidation");
    }
  });
});
test("B03 native label invalidates on line-height or tracking conversion changes even when fontSize/fontScale are identical",async()=>{
  await withHost(async h=>{
    const fixture=await stampFixture(h);const base=snapshot(1);await fixture.render(base);
    const initial=nativeLabel(h);
    const line=change(base,22,29.75);await fixture.render(line);
    const afterLine=nativeLabel(h);assert.equal(afterLine===initial,false,"line-height-only conversion change must replace the label leaf");
    assert.equal(h.rn.StyleSheet.flatten(afterLine.props.style).fontSize,17.25);
    assert.equal(h.rn.StyleSheet.flatten(afterLine.props.style).lineHeight,29.75);
    const tracking=change(line,0.4,0.91);await fixture.render(tracking);
    assert.equal(nativeLabel(h)===afterLine,false,"tracking-only conversion change must replace the label leaf");
    assert.equal(h.rn.StyleSheet.flatten(nativeLabel(h).props.style).letterSpacing,0.91);
  });
});
test("B04 default/selectable/nested Text and all TextInputs remain mounted; unavailable fallback changes only the opted-in label",async()=>{
  await withHost(async h=>{
    const {NativeFontMetricsProvider,Text,TextInput}=await h.load("apps/mobile/src/rudi/ui/NativeFontMetrics.tsx");
    const style={fontSize:17,lineHeight:22,letterSpacing:0.4};
    const render=metrics=>h.render(React.createElement(NativeFontMetricsProvider,{snapshot:metrics},React.createElement(h.rn.View,null,
      React.createElement(Text,{testID:"ordinary",style},"Thường"),
      React.createElement(Text,{testID:"selectable",selectable:true,invalidateOnMetricsChange:true,style},"Chọn được"),
      React.createElement(Text,{testID:"paragraph",invalidateOnMetricsChange:true,style},React.createElement(Text,{testID:"nested",invalidateOnMetricsChange:true},"Đoạn con")),
      React.createElement(Text,{testID:"selectable-paragraph",selectable:true,style},React.createElement(Text,{testID:"selectable-nested",invalidateOnMetricsChange:true},"Con chọn được")),
      React.createElement(TextInput,{testID:"input",style,value:"125000",selection:{start:2,end:2}}),
      React.createElement(Text,{testID:"opted-leaf",invalidateOnMetricsChange:true,style},phrase))));
    await render(snapshot(2));const ids=["ordinary","selectable","paragraph","nested","selectable-paragraph","selectable-nested","input"];
    const before=Object.fromEntries(ids.map(id=>[id,h.byId(id)[0]]));let label=h.byId("opted-leaf")[0];
    for(const next of [snapshot(1),unavailable,snapshot(2)]){
      await render(next);
      for(const id of ids)assert.equal(h.byId(id)[0]===before[id],true,`${id} must remain mounted`);
      assert.equal(h.byId("opted-leaf")[0]===label,false,"manual/native mode or effective metrics change invalidates opted-in leaf");label=h.byId("opted-leaf")[0];
      assert.equal(h.byId("input")[0].props.value,"125000");assert.deepEqual(h.byId("input")[0].props.selection,{start:2,end:2});
      assert.equal(h.byId("selectable")[0].props.selectable,true);
    }
    await render(structuredClone(snapshot(2)));assert.equal(h.byId("opted-leaf")[0]===label,true);
  });
});
function wire(){return {batch_owner_id:actor,batch_status:"published",expenses:[{expense_id:"synthetic-expense",description:"Khoản thử",paid_by_id:other,recorded_by_id:actor,total_amount_vnd:62000,allocations:[{person_id:actor,amount_vnd:31000},{person_id:other,amount_vnd:31000}]}],amendments:[]};}
test("B05 actual full board hot decrease preserves draft/input/StampButton; enabled submit uses typed reason and amounts once",async()=>{
  await withHost(async h=>{
    h.state.nativeSnapshot=snapshot(2);const calls=[];
    h.state.api=async(path,options)=>{
      calls.push({path,options});
      if(options.method!=="GET")throw new Error("synthetic proposal failure");
      if(path.endsWith("/amendments"))return wire();
      if(path.endsWith("/obligations"))return {disputed_count:0,obligations:[]};
      if(path.endsWith("/batches"))return {batches:[{batch_id:batch,status:"published",created_at:"2026-01-01T00:00:00Z",published_at:"2026-01-01T00:00:00Z",obligation_count:0,confirmed_count:0,disputed_count:0,total_vnd:0}]};
      if(path.endsWith("/members"))return {members:[{person_id:actor,display_name:"Người thử A",membership_state:"active"},{person_id:other,display_name:"Người thử B",membership_state:"active"}]};
      throw new Error(`unexpected synthetic read: ${path}`);
    };
    const {DotThuLiveScreen}=await h.load("apps/mobile/src/rudi/screens/dot-thu/DotThuLive.tsx");
    const {StampButton}=await h.load("apps/mobile/src/rudi/ui/StampButton.tsx");
    await h.render(React.createElement(DotThuLiveScreen,{phien:{person_id:actor,context_id:context,membership_state:"active",token:"synthetic-host-only"},batchId:batch}));await h.flush();
    await h.press("Sửa sau khi phát");
    await act(async()=>{h.all("RNPressable").find(n=>h.label(n).startsWith("Đề xuất sửa «")).props.onPress({});});
    const reason=h.all("RNTextInput").find(n=>n.props.accessibilityLabel==="Lý do sửa");
    assert.equal(reason.props.placeholder,"Ví dụ: chia lại món lẩu","shorter empty reason hint is rendered by the actual input");
    await act(async()=>{reason.props.onChangeText("Nháp giữ nguyên");});
    await act(async()=>{h.all("RNTextInput").find(n=>n.props.accessibilityLabel.startsWith("Phần của Người thử A")).props.onChangeText("45000");});
    const stamp=h.root.findAllByType(StampButton).find(n=>n.props.label===phrase);
    const button=h.button(phrase),draft=h.byId("soan-dieu-chinh")[0],inputs=h.all("RNTextInput"),label=nativeLabel(h),readCount=calls.length;
    assert.equal(button.props.disabled,false);
    assert.equal(label.props.allowFontScaling,false,"native snapshot passes actual module validation");
    assert.equal(h.nativeFontObservers,1);
    await h.font(snapshot(1));
    assert.equal(h.root.findAllByType(StampButton).find(n=>n.props.label===phrase)===stamp,true);
    assert.equal(h.button(phrase)===button,true);assert.equal(h.byId("soan-dieu-chinh")[0]===draft,true);
    assert.equal(h.all("RNTextInput").length,inputs.length);assert.equal(h.all("RNTextInput").every((input,index)=>input===inputs[index]),true,"all draft input hosts stay mounted");assert.equal(reason.props.value,"Nháp giữ nguyên");
    assert.equal(nativeLabel(h)===label,false,"real provider event must replace only submit label native leaf");
    assert.equal(calls.length,readCount,"font event cannot refetch board or amendments");assert.equal(h.nativeFontObservers,1);
    await h.press(phrase);await h.flush();
    const writes=calls.filter(call=>call.options.method!=="GET");assert.equal(writes.length,1);
    assert.equal(writes[0].options.body.reason,"Nháp giữ nguyên");
    assert.equal(Object.values(writes[0].options.body.allocations).reduce((sum,vnd)=>sum+vnd,0),76000);
    assert.deepEqual(writes[0].options.body.allocations,{[actor]:45000,[other]:31000});
    assert.equal(h.byId("soan-dieu-chinh")[0]===draft,true,"failed submit preserves actual draft");
    assert.equal(reason.props.value,"Nháp giữ nguyên");
    assert.equal(h.root.findAll(n=>typeof n.type==="string"&&n.props.accessibilityRole==="alert").length,1,"real error renderer remains visible");
  });
});
