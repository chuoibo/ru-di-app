import test from 'node:test';
import assert from 'node:assert/strict';
import { datChuSoHuuBanNhap, docBanNhapCongCu, ghiBanNhapCongCu, loiBinhChon } from '../dist-test/rudi/chat/ban-nhap-cong-cu.js';
import { chatRoute } from '../dist-test/rudi/chat/chat-route.js';

test('tool drafts survive remount reads without crossing room or identity boundaries', () => {
 datChuSoHuuBanNhap('alice');
 const draft={question:'Cuối tuần đi đâu?',choices:['Bờ hồ','Công viên','Quán nhỏ'],prompt:'Đi dạo chiều thứ bảy'};
 ghiBanNhapCongCu('alice','group-one',draft);
 draft.choices[0]='Mutated caller';
 const restored=docBanNhapCongCu('alice','group-one');
 assert.deepEqual(restored.choices,['Bờ hồ','Công viên','Quán nhỏ']);
 assert.equal(restored.prompt,'Đi dạo chiều thứ bảy');
 restored.choices[1]='Mutated read';
 assert.equal(docBanNhapCongCu('alice','group-one').choices[1],'Công viên');
 assert.equal(docBanNhapCongCu('alice','group-two').question,'');
 assert.equal(docBanNhapCongCu('bob','group-one').question,'');
 datChuSoHuuBanNhap('alice');
 assert.equal(docBanNhapCongCu('alice','group-one').question,'Cuối tuần đi đâu?');
 datChuSoHuuBanNhap('bob');
 ghiBanNhapCongCu('alice','group-one',draft);
 datChuSoHuuBanNhap('alice');
 assert.equal(docBanNhapCongCu('alice','group-one').question,'');
});

test('discarding one tool retains the other, logout destroys both', () => {
 datChuSoHuuBanNhap('owner');
 ghiBanNhapCongCu('owner','room',{question:'A?',choices:['A','B'],prompt:'AI draft'});
 ghiBanNhapCongCu('owner','room',{...docBanNhapCongCu('owner','room'),question:'',choices:['','']});
 assert.equal(docBanNhapCongCu('owner','room').prompt,'AI draft');
 datChuSoHuuBanNhap(null);
 datChuSoHuuBanNhap('owner');
 assert.equal(docBanNhapCongCu('owner','room').prompt,'');
});

test('poll corrections remove validation while parser-ambiguous values remain refused', () => {
 assert.match(loiBinhChon('', ['', '']), /hai lựa chọn/);
 assert.equal(loiBinhChon('Đi đâu?', ['Bờ hồ', 'Quán nhỏ']),null);
 assert.match(loiBinhChon('Đi đâu?', ['Bờ hồ','bờ hồ']),/khác nhau/);
 assert.match(loiBinhChon('Đi | đâu?', ['A','B']),/không chứa/);
 assert.match(loiBinhChon('Đi? ở đâu?', ['A','B']),/cuối câu/);
});

test('cold signed-out real chat never becomes a demo, fixtures are exact allowlist', () => {
 const fixtures=['demo-hoi','demo-doi'];
 const real='e7f91f65-614c-4b11-9e42-e6d46867c04c';
 assert.equal(chatRoute(real,false,fixtures),'login');
 assert.equal(chatRoute(real,true,fixtures),'live');
 assert.equal(chatRoute('demo-hoi',false,fixtures),'fixture');
 assert.equal(chatRoute('demo-doi',false,fixtures),'fixture');
 assert.equal(chatRoute('demo-hoi-unknown',false,fixtures),'login');
 assert.equal(chatRoute(undefined,false,fixtures),'messages');
});
