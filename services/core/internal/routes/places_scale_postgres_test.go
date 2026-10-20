//go:build postgres

package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/aiharness/llm"
	"mobile/services/core/internal/aiharness/motluot"
	"mobile/services/core/internal/db"
	"mobile/services/core/internal/httpapi/endpoint"
	"mobile/services/core/internal/ingest"
	"mobile/services/core/internal/limit"
	"mobile/services/core/internal/testdb"
)

const scaleReader = "7d8f1c2a-3b4c-4d5e-8f90-a1b2c3d4e5f6"

func scaleGet(t *testing.T, h http.Handler, target string, reader bool) (int, map[string]any) {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, target, nil)
	if reader {
		r.Header.Set("X-Actor-ID", scaleReader)
		r.Header.Set("X-Actor-Roles", "member")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var out map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func scaleIDs(t *testing.T, body map[string]any) []string {
	t.Helper()
	var out []string
	for _, p := range body["places"].([]any) {
		out = append(out, p.(map[string]any)["id"].(string))
	}
	return out
}

func scaleReaderRow(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	for _, stmt := range []string{
		`INSERT INTO people(id,display_name,budget_band) VALUES ('` + scaleReader + `','Người đọc thử','vua-phai')`,
		`INSERT INTO person_interests(id,person_id,tag) VALUES (gen_random_uuid(),'` + scaleReader + `','cafe')`,
	} {
		if _, err := pool.Exec(context.Background(), stmt); err != nil {
			t.Fatal(err)
		}
	}
}

// Without limit/offset the answer is the whole list and carries no paging
// keys (the bytes Python sends). With either, pages cut from one ordered list
// tile it exactly, `total` counts the filtered list and the chain ends on a
// null next_offset.
func TestPlacesPagesTileTheCatalogueAndLegacyCallersKeepTheWholeList(t *testing.T) {
	pool := catalogueSchema(t)
	h := handlerOn(t, pool, nil)
	code, whole := scaleGet(t, h, "/places?destination=d-da-lat", false)
	if code != 200 {
		t.Fatal(code)
	}
	if _, has := whole["total"]; has {
		t.Fatal("a caller that did not ask for paging got paging keys")
	}
	want := scaleIDs(t, whole)
	if len(want) < 1000 {
		t.Fatalf("expected a big catalogue, got %d", len(want))
	}
	var got []string
	offset, pages := 0, 0
	for {
		code, page := scaleGet(t, h, "/places?destination=d-da-lat&limit=100&offset="+scaleItoa(offset), false)
		if code != 200 {
			t.Fatal(code, page)
		}
		if int(page["total"].(float64)) != len(want) {
			t.Fatalf("total %v, want %d", page["total"], len(want))
		}
		ids := scaleIDs(t, page)
		if len(ids) > 100 {
			t.Fatalf("page of %d", len(ids))
		}
		got = append(got, ids...)
		pages++
		next := page["next_offset"]
		if next == nil {
			break
		}
		offset = int(next.(float64))
		if pages > len(want) {
			t.Fatal("next_offset never ends")
		}
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("pages do not tile the whole list (%d vs %d)", len(got), len(want))
	}
	// limit over the ceiling is clamped; a bad one is a 422.
	_, big := scaleGet(t, h, "/places?destination=d-da-lat&limit=100000", false)
	if len(scaleIDs(t, big)) != 100 {
		t.Fatalf("limit not clamped: %d", len(scaleIDs(t, big)))
	}
	if code, _ := scaleGet(t, h, "/places?destination=d-da-lat&limit=0", false); code != 422 {
		t.Fatalf("limit=0 -> %d", code)
	}
	// The filter runs before the cut: total is the filtered count.
	_, filtered := scaleGet(t, h, "/places?destination=d-da-lat&category=cafe&limit=10", false)
	_, all := scaleGet(t, h, "/places?destination=d-da-lat&category=cafe", false)
	n := len(scaleIDs(t, all))
	if int(filtered["total"].(float64)) != n || n == 0 || n >= len(want) {
		t.Fatalf("filtered total %v, want %d of %d", filtered["total"], n, len(want))
	}
}

func scaleItoa(n int) string { return strconv.Itoa(n) }

// The list never waits for the model, even for a reader whose taste is known.
// The reasons are a separate request: one model call for the asked places, a
// grounded sentence served, an invented figure dropped, and no database
// connection held while the model thinks.
func TestPlacesListNeverCallsTheModelAndReasonsReleaseTheConnection(t *testing.T) {
	pool := catalogueSchema(t)
	scaleReaderRow(t, pool)

	quiet := newModelGhi("[]")
	code, list := scaleGet(t, handlerOn(t, pool, quiet), "/places?destination=d-hoi-an", true)
	if code != 200 || len(quiet.stub.YeuCau()) != 0 {
		t.Fatalf("code %d, %d model calls for a list", code, len(quiet.stub.YeuCau()))
	}
	for _, p := range list["places"].([]any) {
		if m, _ := p.(map[string]any)["match"].(map[string]any); m != nil && m["source"] != "none" {
			t.Fatalf("the list claims a model sentence: %v", m)
		}
	}

	answer := `[{"id": "ha-ca-phe-hoai-niem", "verdict": "hop", "reason": "Giá 30-60k, yên tĩnh."},` +
		` {"id": "p5k-0003", "verdict": "tam", "reason": "Mở từ năm 1998."}]`
	slow := &modelGhi{stub: llm.NewStub(llm.Buoc{Text: answer, Cho: 600 * time.Millisecond})}
	env := endpoint.Env{Mode: endpoint.ModeDev, NewUnit: func() *db.Unit { return db.NewUnit(pool) }, Now: time.Now,
		Limits: limit.NewSet(limit.Monotonic), AI: motluot.Moi(slow.stub, 1)}
	h := coreWithEnv(t, env, func(next http.Handler) http.Handler { return next })

	var wg sync.WaitGroup
	var body map[string]any
	var status int
	wg.Add(1)
	go func() {
		defer wg.Done()
		status, body = scaleGet(t, h, "/places?reasons_for=ha-ca-phe-hoai-niem,p5k-0003", true)
	}()
	time.Sleep(300 * time.Millisecond)
	if held := pool.Stat().AcquiredConns(); held != 0 {
		t.Fatalf("%d connections held while the request waited for the model", held)
	}
	wg.Wait()
	if status != 200 {
		t.Fatal(status, body)
	}
	if len(slow.stub.YeuCau()) != 1 {
		t.Fatalf("%d model calls, want 1", len(slow.stub.YeuCau()))
	}
	reasons := body["reasons"].(map[string]any)
	good, _ := reasons["ha-ca-phe-hoai-niem"].(map[string]any)
	if good["reason"] != "Giá 30-60k, yên tĩnh." || good["verdict"] != "hop" || good["source"] != "ai" {
		t.Fatalf("a grounded reason was lost: %v", reasons)
	}
	if _, has := reasons["p5k-0003"]; has {
		t.Fatalf("an invented figure reached the app: %v", reasons)
	}
	// Nobody to write for: no taste, no call.
	nobody := newModelGhi("[]")
	_, anon := scaleGet(t, handlerOn(t, pool, nobody), "/places?reasons_for=ha-ca-phe-hoai-niem", false)
	if len(anon["reasons"].(map[string]any)) != 0 || len(nobody.stub.YeuCau()) != 0 {
		t.Fatalf("an anonymous reader got reasons or spent a call: %v", anon)
	}
}

func TestDestinationsCoverIsOptInAndPrefersTheBestEvidencedPlace(t *testing.T) {
	pool := catalogueSchema(t)
	ctx := context.Background()
	for _, stmt := range []string{
		`UPDATE places SET evidence_posts = 3 WHERE id = 'p5k-0001'`,
		`UPDATE places SET evidence_posts = 90 WHERE id = 'p5k-0004'`,
		`INSERT INTO place_photos(id,place_id,storage_key,content_type,byte_size,width,height,author,license,source_url,sort_order) VALUES
		 (gen_random_uuid(),'p5k-0001','k1','image/jpeg',100,10,10,'A','CC','https://x/1',0),
		 ('a1b2c3d4-e5f6-4a7b-8c9d-abcdefabcdef','p5k-0004','k4','image/jpeg',100,10,10,'Tác giả','CC BY','https://x/4',0),
		 (gen_random_uuid(),'p5k-0004','k4b','image/jpeg',100,10,10,NULL,NULL,'https://x/4b',1)`,
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	// p5k-0001 -> index 1 -> d-tphcm? ids are 1+i%3 over the array: i=1 -> d-tphcm, i=4 -> d-tphcm.
	h := handlerOn(t, pool, nil)
	_, plain := scaleGet(t, h, "/destinations", false)
	for _, d := range plain["destinations"].([]any) {
		if _, has := d.(map[string]any)["cover_photo_url"]; has {
			t.Fatal("a caller that did not ask got cover fields")
		}
	}
	_, withCover := scaleGet(t, h, "/destinations?include=cover", false)
	byID := map[string]map[string]any{}
	for _, d := range withCover["destinations"].([]any) {
		m := d.(map[string]any)
		byID[m["id"].(string)] = m
	}
	hcm := byID["d-tphcm"]
	if hcm["cover_photo_url"] != "/places/p5k-0004/photos/a1b2c3d4-e5f6-4a7b-8c9d-abcdefabcdef" ||
		hcm["cover_photo_author"] != "Tác giả" || hcm["cover_photo_license"] != "CC BY" {
		t.Fatalf("cover of d-tphcm: %v", hcm)
	}
	for _, id := range []string{"d-da-lat", "d-hoi-an"} {
		for _, k := range []string{"cover_photo_url", "cover_photo_author", "cover_photo_license"} {
			if v, has := byID[id][k]; !has || v != nil {
				t.Fatalf("%s.%s must be null, got %v (present=%v)", id, k, v, has)
			}
		}
	}
}

func TestPlaceDetailHighlightsAreOptInLiveOnlyAndNeverCarryEvidence(t *testing.T) {
	ctx := context.Background()
	if err := ingest.Migrate(ctx, testdb.Pool(t)); err != nil {
		t.Fatal(err)
	}
	pool := catalogueSchema(t)
	for _, table := range []string{"place_facts", "place_danh_muc", "place_lam_giau"} {
		if _, err := pool.Exec(ctx, `CREATE TABLE `+table+` (LIKE public.`+table+` INCLUDING ALL)`); err != nil {
			t.Fatal(err)
		}
	}
	reviews := `{"khong_khi":"Yên tĩnh, nhiều cây.","luu_y":["Đông cuối tuần"],"bang_chung":["LOI-NGUYEN-VAN-CUA-KHACH"],
	             "thieu_gi":["GHI-CHU-THIEU"],"mon_phai_thu":[{"ten":"Cà phê muối","gia":"35k"}]}`
	for _, stmt := range []string{
		`INSERT INTO places(id,destination_id,name,category,kinds,address,lat,lng,geo_precision,traits,reviews,source,source_ref)
		 VALUES ('vnl-live','d-hoi-an','Quán Có Dữ Liệu','cafe','[]','Hội An',15.88,108.33,'rooftop','[]','` + reviews + `','vnlocal','plc_live'),
		        ('vnl-old','d-hoi-an','Quán Hết Hạn','cafe','[]','Hội An',15.88,108.33,'rooftop','[]','{}','vnlocal','plc_old'),
		        ('vnl-bare','d-hoi-an','Quán Trống','cafe','[]','Hội An',15.88,108.33,'rooftop','[]','{}','vnlocal','plc_bare')`,
		`INSERT INTO place_facts(place_id,source_ref,schema_version,trang_thai,con_hoat_dong,gio_mo_cua,gio_ghi_chu,menu,hoat_dong,can_dat_truoc,
		   trong_nha_ngoai_troi,gia_min_vnd,gia_max_vnd,gia_uoc,checked_at,het_han_at,synced_at) VALUES
		 ('vnl-live','plc_live','web-facts@2','co_du_lieu','con_hoat_dong','[{"thu":"mon","mo":"08:00","dong":"22:00","qua_dem":false}]','Mở cả tuần','[]','[]',
		   'nen','ngoai_troi',30000,60000,true, now(), now() + interval '30 days', now()),
		 ('vnl-old','plc_old','web-facts@2','co_du_lieu','con_hoat_dong','[{"thu":"mon","mo":"08:00","dong":"22:00","qua_dem":false}]','Đã cũ','[]','[]',
		   'nen','ngoai_troi',30000,60000,false, now() - interval '60 days', now() - interval '1 day', now())`,
		`INSERT INTO place_danh_muc(place_id,source_ref,danh_muc,schema_version,checked_at,synced_at)
		 VALUES ('vnl-live','plc_live','{cafe}','danh-muc@1',now(),now())`,
		`INSERT INTO place_lam_giau(place_id,source_ref,di_ung,an_kieng,khi_chat,mon_chinh,chen_lenh,tin_cay,schema_version,checked_at,synced_at)
		 VALUES ('vnl-live','plc_live','{sua}','{chay}','{yen_tinh}','{cà phê muối}',false,'cao','lam-giau@1',now(),now())`,
	} {
		if _, err := pool.Exec(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	h := handlerOn(t, pool, nil)
	_, plain := scaleGet(t, h, "/places/vnl-live", false)
	if _, has := plain["highlights"]; has {
		t.Fatal("highlights sent to a caller that did not ask")
	}
	code, rich := scaleGet(t, h, "/places/vnl-live?include=highlights", false)
	if code != 200 {
		t.Fatal(code, rich)
	}
	hl, _ := rich["highlights"].(map[string]any)
	if hl == nil {
		t.Fatalf("no highlights: %v", rich)
	}
	raw, _ := json.Marshal(hl)
	for _, leak := range []string{"LOI-NGUYEN-VAN-CUA-KHACH", "GHI-CHU-THIEU", "bang_chung", "thieu_gi", "di_ung", "an_kieng", "sua", "chay"} {
		if leak != "sua" && leak != "chay" && strings.Contains(string(raw), leak) {
			t.Fatalf("leak %q in %s", leak, raw)
		}
	}
	if strings.Contains(string(raw), `"sua"`) || strings.Contains(string(raw), `"chay"`) {
		t.Fatalf("allergen reading in %s", raw)
	}
	for _, key := range []string{"khong_khi", "luu_y", "mon_phai_thu", "gia", "gio_mo_cua", "can_dat_truoc", "trong_nha_ngoai_troi", "danh_muc", "khi_chat", "mon_chinh"} {
		if _, ok := hl[key]; !ok {
			t.Errorf("missing %s in %s", key, raw)
		}
	}
	if gia, _ := hl["gia"].(map[string]any); gia["uoc"] != true {
		t.Errorf("an estimated price must say so: %v", hl["gia"])
	}
	// The expired fact reads as unknown, and a place with nothing has no key.
	_, old := scaleGet(t, h, "/places/vnl-old?include=highlights", false)
	if _, has := old["highlights"]; has {
		t.Fatalf("an expired fact was shown: %v", old["highlights"])
	}
	_, bare := scaleGet(t, h, "/places/vnl-bare?include=highlights", false)
	if _, has := bare["highlights"]; has {
		t.Fatalf("an empty highlights object: %v", bare["highlights"])
	}
}
