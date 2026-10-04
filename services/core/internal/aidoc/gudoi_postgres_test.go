//go:build postgres

package aidoc

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"mobile/services/core/internal/gudoi"
)

// The couple's taste port against real tables (ADR-0048): only a couple,
// only people whose `chia_gu` covers the chat, asked at every call.

type doiThu struct {
	pool         *pgxpool.Pool
	phong, chuKy string
	an, binh     string
	deNghiDoi    string
	soDeNghi     int
	moi, cu      time.Time
	ctx          context.Context
}

func (d *doiThu) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	if _, err := d.pool.Exec(d.ctx, sql, args...); err != nil {
		t.Fatal(err)
	}
}

func (d *doiThu) id() string {
	d.soDeNghi++
	return fmt.Sprintf("dddddddd-dddd-4ddd-8ddd-%012d", d.soDeNghi)
}

// chiaGu files person's own `chia_gu` the way the notebook does: a proposal
// completed as it is filed and the person's yes on it, at luc.
func (d *doiThu) chiaGu(t *testing.T, person string, luc time.Time) {
	t.Helper()
	p := d.id()
	d.exec(t, `INSERT INTO pair_consent_proposals(id,cycle_id,purpose,proposed_by_id,completed_at,expires_at) VALUES($1,$2,'chia_gu',$3,$4,now() + interval '7 days')`, p, d.chuKy, person, luc)
	d.exec(t, `INSERT INTO pair_consents(id,proposal_id,person_id,granted_at) VALUES($1,$2,$3,$4)`, d.id(), p, person, luc)
}

func (d *doiThu) thuHoi(t *testing.T, person, purpose string) {
	t.Helper()
	d.exec(t, `UPDATE pair_consents k SET revoked_at=now() FROM pair_consent_proposals p WHERE p.id=k.proposal_id AND p.cycle_id=$1 AND p.purpose=$2 AND k.person_id=$3 AND k.revoked_at IS NULL`, d.chuKy, purpose, person)
}

func moDoi(t *testing.T) *doiThu {
	t.Helper()
	pool := kho(t)
	ctx := context.Background()
	for _, table := range []string{"pair_notebooks", "pair_notebook_cycles", "pair_cycle_participants", "pair_consent_proposals", "pair_consents", "person_interests", "pair_shared_constraints"} {
		if _, err := pool.Exec(ctx, fmt.Sprintf("CREATE TABLE %s (LIKE public.%s INCLUDING ALL)", table, table)); err != nil {
			t.Fatal(err)
		}
	}
	d := &doiThu{pool: pool, ctx: ctx, phong: "a1a1a1a1-a1a1-4a1a-8a1a-a1a1a1a1a1a1", chuKy: "c1c1c1c1-c1c1-4c1c-8c1c-c1c1c1c1c1c1",
		an: "a2a2a2a2-a2a2-4a2a-8a2a-a2a2a2a2a2a2", binh: "b2b2b2b2-b2b2-4b2b-8b2b-b2b2b2b2b2b2",
		moi: gudoi.MocChat().Add(time.Hour), cu: gudoi.MocChat().Add(-24 * time.Hour)}
	so := "f1f1f1f1-f1f1-4f1f-8f1f-f1f1f1f1f1f1"
	d.exec(t, `INSERT INTO pair_notebooks(id,context_id) VALUES($1,$2)`, so, d.phong)
	d.exec(t, `INSERT INTO pair_notebook_cycles(id,notebook_id,state,opened_at) VALUES($1,$2,'active',now())`, d.chuKy, so)
	d.exec(t, `INSERT INTO pair_cycle_participants(cycle_id,person_id) VALUES($1,$2),($1,$3)`, d.chuKy, d.an, d.binh)
	d.deNghiDoi = d.id()
	// Expiry follows the database clock (created_at is now()), never the
	// fixed consent instant: cu+7d fell behind now() on the 4th of October.
	d.exec(t, `INSERT INTO pair_consent_proposals(id,cycle_id,purpose,proposed_by_id,completed_at,expires_at) VALUES($1,$2,'bat_doi',$3,$4,now() + interval '7 days')`, d.deNghiDoi, d.chuKy, d.an, d.cu)
	for _, p := range []string{d.an, d.binh} {
		d.exec(t, `INSERT INTO pair_consents(id,proposal_id,person_id,granted_at) VALUES($1,$2,$3,$4)`, d.id(), d.deNghiDoi, p, d.cu)
	}
	for p, tags := range map[string][]string{d.an: {"outdoor", "cafe"}, d.binh: {"cafe", "karaoke"}} {
		for _, tag := range tags {
			d.exec(t, `INSERT INTO person_interests(id,person_id,tag) VALUES($1,$2,$3)`, d.id(), p, tag)
		}
	}
	// A shared constraint the port must never read (and a gate pins it
	// never names the table).
	d.exec(t, `INSERT INTO pair_shared_constraints(cycle_id,owner_id,kind,content,version,updated_at) VALUES($1,$2,'khong_an_duoc','Synthetic constraint',1,now())`, d.chuKy, d.an)
	return d
}

func TestGuDoiChiNguoiDongYMoi(t *testing.T) {
	d := moDoi(t)
	doc := Doc{C: Moi(d.pool, 1)}
	doc0 := func(t *testing.T) []gudoi.Gu {
		t.Helper()
		gs, err := doc.GuDoi(d.ctx, d.phong)
		if err != nil {
			t.Fatal(err)
		}
		return gs
	}
	// Nobody shared: nothing.
	if gs := doc0(t); len(gs) != 0 {
		t.Fatalf("%+v", gs)
	}
	// An old consent (before the cutoff) covers the notebook, not the chat.
	d.chiaGu(t, d.an, d.cu)
	d.chiaGu(t, d.binh, d.cu)
	if gs := doc0(t); len(gs) != 0 {
		t.Fatalf("an old consent reached the chat: %+v", gs)
	}
	// An re-consents (off, then on again under the new wording): only An's.
	d.thuHoi(t, d.an, "chia_gu")
	d.chiaGu(t, d.an, d.moi)
	if gs := doc0(t); !reflect.DeepEqual(gs, []gudoi.Gu{{NguoiID: d.an, The: []string{"cafe", "outdoor"}}}) {
		t.Fatalf("%+v", gs)
	}
	// Both: each person's, in vocabulary order.
	d.thuHoi(t, d.binh, "chia_gu")
	d.chiaGu(t, d.binh, d.moi)
	gs := doc0(t)
	if len(gs) != 2 || !reflect.DeepEqual(gudoi.Chung(gs), []string{"cafe"}) {
		t.Fatalf("%+v", gs)
	}
	// Revocation is seen at the next call.
	d.thuHoi(t, d.an, "chia_gu")
	if gs := doc0(t); len(gs) != 1 || gs[0].NguoiID != d.binh {
		t.Fatalf("a revoked taste was read: %+v", gs)
	}
	// «Một đôi» broken: nothing, whatever `chia_gu` says.
	d.thuHoi(t, d.an, "bat_doi")
	if gs := doc0(t); len(gs) != 0 {
		t.Fatalf("a broken couple's taste was read: %+v", gs)
	}
	// A room with no notebook, and an id that is not a uuid.
	if gs, err := doc.GuDoi(d.ctx, "e1e1e1e1-e1e1-4e1e-8e1e-e1e1e1e1e1e1"); err != nil || len(gs) != 0 {
		t.Fatalf("%+v %v", gs, err)
	}
	if _, err := doc.GuDoi(d.ctx, "khong-phai-uuid"); err != ErrID {
		t.Fatalf("%v", err)
	}
}
