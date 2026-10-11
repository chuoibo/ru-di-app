/** Real seal/provider/draft components; layout events are synthetic, not pixel evidence. */
import assert from "node:assert/strict";
import { after, test } from "node:test";
import { React, act, withHost, closeHost } from "./n-behavior-host.mjs";
after(closeHost);
const phrase="Gửi đề xuất";
function snapshot(scale=1) {
  const bases=[-0.65,-0.3,-0.15,0.4,0.8,1.6,11,12,13,14,15,17,18,19,21,22,23,24,27,28,34];
  return {status:"available",fontScale:scale,fontMetrics:{density:2,spToDp:bases.map(base=>({base,dp:base*scale}))}};
}
const flatten=(h,n)=>h.rn.StyleSheet.flatten(n.props.style);
function visible(h,label=phrase) {
  const nodes=h.all("RNText").filter(n=>n.children.length===1&&n.children[0]===label&&n.props.importantForAccessibility!=="no-hide-descendants");
  assert.equal(nodes.length,1,`one visible native label: ${label}`);return nodes[0];
}
function probe(h,label=phrase) {
  const nodes=h.all("RNText").filter(n=>n.children.length===1&&n.children[0]===label&&n.props.importantForAccessibility==="no-hide-descendants"&&typeof n.props.onTextLayout==="function");
  assert.ok(nodes.length>=1&&nodes.length<=2,`actual wide/fitted measurements: ${label}`);return nodes[0];
}
function column(h,label=phrase) {
  const text=probe(h,label);let parent=text.parent;
  while(parent&&(parent.type!=="RNView"||!parent.props.onLayout))parent=parent.parent;
  assert.ok(parent,"measurement has a finite native column");return parent;
}
function firstColumn(h,component) {return h.root.findByType(component).findAllByType("RNView").find(n=>n.props.onLayout&&flatten(h,n).minWidth===0);}
async function layout(node,width,height=0){await act(async()=>node.props.onLayout({nativeEvent:{layout:{width,height}}}));}
async function measure(node,widths){await act(async()=>(typeof node==="function"?node:node.props.onTextLayout)({nativeEvent:{lines:widths.map(width=>({width}))}}));}
// Fit callbacks are explicit synthetic native inputs, not pixel rendering evidence.
async function measureAndFit(h,node,widths){
  const label=node.children[0];
  await measure(node,widths);
  assert.equal(flatten(h,visible(h,label)).width,undefined,"wide line advance cannot publish a fitted width before its callback");
  let attempts=0;
  for(;;){
    const nodes=h.all("RNText").filter(n=>n.children.length===1&&n.children[0]===label&&n.props.importantForAccessibility==="no-hide-descendants"&&typeof n.props.onTextLayout==="function");
    const fitted=nodes[1];if(!fitted)break;
    assert.ok(attempts<12,"fit requests terminate within the native callback budget");
    assert.equal(fitted.props.numberOfLines,undefined,"a probe cannot truncate to manufacture fit");
    await measure(fitted,widths);attempts++;
  }
}
async function fixture(h,kind="button",initial=snapshot(1),extra={}) {
  const {NativeFontMetricsProvider,TextInput}=await h.load("apps/mobile/src/rudi/ui/NativeFontMetrics.tsx");
  const {StampButton}=await h.load("apps/mobile/src/rudi/ui/StampButton.tsx");
  const {Stamp}=await h.load("apps/mobile/src/rudi/ui/Stamp.tsx");
  const Component=kind==="button"?StampButton:Stamp;
  const {PressScale}=await h.load("apps/mobile/src/rudi/ui/PressScale.tsx");
  function Draft({metrics,props}){
    const [reason,setReason]=React.useState("");
    return React.createElement(NativeFontMetricsProvider,{snapshot:metrics},React.createElement(h.rn.View,{testID:"draft-parent"},
      React.createElement(TextInput,{testID:"draft-input",value:reason,onChangeText:setReason,style:{fontSize:17,lineHeight:22,letterSpacing:0.4}}),
      React.createElement(Component,{measureLabel:true,label:kind==="button"?phrase:"ĐÃ VỀ",size:"vua",testID:"seal",onPress:()=>{},...props})));
  }
  const render=(metrics=initial,props=extra)=>h.render(React.createElement(Draft,{metrics,props}));
  await render();return {Component,PressScale,render};
}
test("W01 actual button rounds measured intrinsic width up to a pixel, caps to its column, and never measures its padded visible width",async()=>{
  await withHost(async h=>{
    const f=await fixture(h);await layout(firstColumn(h,f.Component),200);
    const hidden=probe(h);assert.equal(flatten(h,hidden).width,152);assert.equal(hidden.props.accessible,false);
    assert.equal(hidden.props.importantForAccessibility,"no-hide-descendants");assert.equal(hidden.props.pointerEvents,"none");assert.equal(flatten(h,hidden).opacity,0);
    await measureAndFit(h,hidden,[96.446]);assert.equal(flatten(h,visible(h)).width,96.5,"ceil to half-dp at density2, with no arbitrary extra pixel");
    const leaf=visible(h);await measure(hidden,[96.446]);assert.equal(visible(h)===leaf,true,"same measurement cannot churn the visible label");
    assert.equal(visible(h).props.onTextLayout,undefined,"measurement has no feedback from visible explicit width");
    await layout(column(h),201);await layout(column(h),200);
    await measureAndFit(h,probe(h),[1000]);assert.equal(flatten(h,visible(h)).width,152,"the finite column caps any oversized result");
    await layout(column(h),201);await layout(column(h),200);
    await measureAndFit(h,probe(h),[80,70]);assert.equal(flatten(h,visible(h)).width,152,"multiline intrinsic measurement uses the finite cap");
  });
});
test("W02 hot200→100 clears old width, ignores its delayed event, and keeps actual parent/button/body/rim/input state",async()=>{
  await withHost(async h=>{
    const f=await fixture(h,"button",snapshot(2));await layout(firstColumn(h,f.Component),300);
    const oldNode=probe(h),oldProbe=oldNode.props.onTextLayout;await measureAndFit(h,oldNode,[193.8]);assert.equal(flatten(h,visible(h)).width,194);
    const input=h.byId("draft-input")[0];await act(async()=>input.props.onChangeText("Nháp giữ nguyên"));
    const body=h.root.findByType(f.Component).findAllByType("RNView").find(n=>n.props.onLayout&&flatten(h,n).minHeight===52);
    await layout(body,242,52);
    const before={owner:h.root.findByType(f.Component),press:h.root.findByType(f.PressScale),button:h.byId("seal")[0],body,parent:h.byId("draft-parent")[0],input,rim:h.all("Svg")[0]};
    await f.render(snapshot(1));assert.equal(flatten(h,visible(h)).width,undefined,"converted metrics invalidate old200 width before new measurement");
    await measure(oldProbe,[193.8]);assert.equal(flatten(h,visible(h)).width,undefined,"late200 callback cannot restore stale width");
    await measureAndFit(h,probe(h),[96.446]);assert.equal(flatten(h,visible(h)).width,96.5);
    await measure(oldProbe,[193.8]);assert.equal(flatten(h,visible(h)).width,96.5,"late200 callback cannot erase a fresh100 measurement");
    const now={owner:h.root.findByType(f.Component),press:h.root.findByType(f.PressScale),button:h.byId("seal")[0],body:h.root.findByType(f.Component).findAllByType("RNView").find(n=>n.props.onLayout&&flatten(h,n).minHeight===52),parent:h.byId("draft-parent")[0],input:h.byId("draft-input")[0],rim:h.all("Svg")[0]};
    for(const key of Object.keys(before))assert.equal(now[key]===before[key],true,`${key} remains mounted`);
    assert.equal(input.props.value,"Nháp giữ nguyên");assert.equal(now.rim.props.width,242,"existing rim box state stays intact until real next layout");
  });
});
test("W03 line-height/tracking/label and finite-column changes invalidate measurements independently",async()=>{
  await withHost(async h=>{
    const f=await fixture(h);await layout(firstColumn(h,f.Component),200);await measureAndFit(h,probe(h),[96.446]);
    let metrics=snapshot(1);
    for(const base of [22,0.4]) {
      metrics=structuredClone(metrics);metrics.fontMetrics.spToDp.find(row=>row.base===base).dp+=0.7;
      const old=probe(h).props.onTextLayout;await f.render(metrics);assert.equal(flatten(h,visible(h)).width,undefined,`base${base} has its own measurement signature`);
      await measure(old,[96.446]);assert.equal(flatten(h,visible(h)).width,undefined);await measureAndFit(h,probe(h),[97.1]);
    }
    const old=probe(h).props.onTextLayout;await layout(column(h),180);assert.equal(flatten(h,visible(h)).width,undefined);
    await measure(old,[97.1]);assert.equal(flatten(h,visible(h)).width,undefined);await measureAndFit(h,probe(h),[97.1]);
    await f.render(metrics,{label:"Gửi lại đề xuất"});assert.equal(flatten(h,visible(h,"Gửi lại đề xuất")).width,undefined);
  });
});
test("W04 actual status Stamp opts in without replacing its visible Text, seal, or adjacent draft on hot metrics",async()=>{
  await withHost(async h=>{
    const f=await fixture(h,"status",snapshot(2));await layout(firstColumn(h,f.Component),280);
    await measureAndFit(h,probe(h,"ĐÃ VỀ"),[61.8]);assert.equal(flatten(h,visible(h,"ĐÃ VỀ")).width,62);
    const leaf=visible(h,"ĐÃ VỀ"),owner=h.root.findByType(f.Component),seal=h.byId("seal")[0],input=h.byId("draft-input")[0];
    await f.render(snapshot(1));assert.equal(flatten(h,visible(h,"ĐÃ VỀ")).width,undefined);
    await measureAndFit(h,probe(h,"ĐÃ VỀ"),[30.884]);assert.equal(flatten(h,visible(h,"ĐÃ VỀ")).width,31);
    assert.equal(visible(h,"ĐÃ VỀ")===leaf,true,"status label needs explicit geometry, no remount");
    assert.equal(h.root.findByType(f.Component)===owner,true);assert.equal(h.byId("seal")[0]===seal,true);assert.equal(h.byId("draft-input")[0]===input,true);
    await f.render(snapshot(1),{labelStyle:{fontFamily:"SyntheticDifferentFace"}});assert.equal(flatten(h,visible(h,"ĐÃ VỀ")).width,undefined,"font family change invalidates intrinsic measurement");
  });
});
test("W05 default seals and wrapLabel keep their original visible geometry; actual spinner width reduces opt-in cap",async()=>{
  await withHost(async h=>{
    const f=await fixture(h,"button",snapshot(1),{measureLabel:false});assert.equal(h.all("RNText").filter(n=>n.props.onTextLayout).length,0);assert.equal(flatten(h,visible(h)).width,undefined);
    await f.render(snapshot(1),{measureLabel:true,wrapLabel:true});assert.equal(firstColumn(h,f.Component),undefined,"wrapping seals retain their original wrapper geometry");
    assert.equal(h.all("RNText").filter(n=>n.props.onTextLayout).length,0);assert.equal(flatten(h,visible(h)).width,undefined);
    await f.render(snapshot(1),{measureLabel:true,loading:true});await layout(firstColumn(h,f.Component),200);
    await layout(h.all("RNActivityIndicator")[0],20,20);assert.equal(flatten(h,probe(h)).width,122,"48 padding + actual20 spinner +10 gap are reserved");
    await measureAndFit(h,probe(h),[1000]);assert.equal(flatten(h,visible(h)).width,122);
  });
});
const actor="aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",other="bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",context="cccccccc-cccc-4ccc-8ccc-cccccccccccc",batch="synthetic-batch";
function wire(){return {batch_owner_id:actor,batch_status:"published",expenses:[{expense_id:"synthetic-expense",description:"Khoản thử",paid_by_id:other,recorded_by_id:actor,total_amount_vnd:62000,allocations:[{person_id:actor,amount_vnd:31000},{person_id:other,amount_vnd:31000}]}],amendments:[]};}
test("W06 real board wires both opt-ins; after native hot event fresh width preserves every draft input and performs no new reads/writes",async()=>{
  await withHost(async h=>{
    h.state.nativeSnapshot=snapshot(2);const calls=[];h.state.api=async(path,options)=>{
      calls.push({path,options});assert.equal(options.method,"GET","this geometry witness never writes");
      if(path.endsWith("/amendments"))return wire();
      if(path.endsWith("/obligations"))return {disputed_count:0,obligations:[{obligation_id:"synthetic-obligation",sender_id:actor,recipient_id:other,amount_vnd:31000,obligation_status:"confirmed",disputed:false}]};
      if(path.endsWith("/batches"))return {batches:[{batch_id:batch,status:"published",created_at:"2026-01-01T00:00:00Z",published_at:"2026-01-01T00:00:00Z",obligation_count:1,confirmed_count:1,disputed_count:0,total_vnd:31000}]};
      if(path.endsWith("/members"))return {members:[{person_id:actor,display_name:"Người thử A",membership_state:"active"},{person_id:other,display_name:"Người thử B",membership_state:"active"}]};throw new Error(path);
    };
    const {DotThuLiveScreen}=await h.load("apps/mobile/src/rudi/screens/dot-thu/DotThuLive.tsx");
    const {StampButton}=await h.load("apps/mobile/src/rudi/ui/StampButton.tsx");
    const {Stamp}=await h.load("apps/mobile/src/rudi/ui/Stamp.tsx");
    await h.render(React.createElement(DotThuLiveScreen,{phien:{person_id:actor,context_id:context,membership_state:"active",token:"synthetic-host-only"},batchId:batch}));await h.flush();
    const statusOwner=h.root.findByType(Stamp);let statusRow=statusOwner.parent;
    while(statusRow&&(statusRow.type!=="RNView"||!statusRow.props.onLayout))statusRow=statusRow.parent;
    assert.ok(statusRow,"actual status row supplies its finite column");await layout(statusRow,280);
    assert.equal(flatten(h,firstColumn(h,Stamp)).width,undefined,"status measurement wrapper stays content-sized for adjacent annotations");
    await measureAndFit(h,probe(h,"đã về"),[61.8]);
    await h.press("Sửa sau khi phát");await act(async()=>h.all("RNPressable").find(n=>h.label(n).startsWith("Đề xuất sửa «")).props.onPress({}));
    const input=h.all("RNTextInput").find(n=>n.props.accessibilityLabel==="Lý do sửa");await act(async()=>input.props.onChangeText("Nháp giữ nguyên"));
    const owner=h.root.findAllByType(StampButton).find(n=>n.props.label===phrase);await layout(firstColumn(h,StampButton),220);await measureAndFit(h,probe(h),[169.8]);
    const draft=h.byId("soan-dieu-chinh")[0],inputs=h.all("RNTextInput"),button=h.button(phrase),readCount=calls.length;
    await h.font(snapshot(1));assert.equal(flatten(h,visible(h)).width,undefined);assert.equal(flatten(h,visible(h,"đã về")).width,undefined);
    await measureAndFit(h,probe(h),[96.446]);await measureAndFit(h,probe(h,"đã về"),[30.884]);
    assert.equal(flatten(h,visible(h)).width,96.5);assert.equal(flatten(h,visible(h,"đã về")).width,31);
    assert.equal(h.root.findAllByType(StampButton).find(n=>n.props.label===phrase)===owner,true);assert.equal(h.button(phrase)===button,true);assert.equal(h.byId("soan-dieu-chinh")[0]===draft,true);
    assert.equal(h.all("RNTextInput").every((node,i)=>node===inputs[i]),true);assert.equal(input.props.value,"Nháp giữ nguyên");assert.equal(calls.length,readCount);assert.equal(h.nativeFontObservers,1);
  });
});
