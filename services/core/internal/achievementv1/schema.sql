CREATE TABLE achievement_runs (
 id uuid PRIMARY KEY,
 person_id uuid NOT NULL REFERENCES people(id) ON DELETE CASCADE,
 route_id text NOT NULL CHECK(route_id IN ('dau_chan','ky_niem','dong_hanh','nga_re')),
 ending_id text NOT NULL,
 selected_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 finished_at timestamptz,
 closed_at timestamptz
);
CREATE UNIQUE INDEX achievement_one_active_run ON achievement_runs(person_id) WHERE closed_at IS NULL;
CREATE INDEX achievement_runs_person_history ON achievement_runs(person_id,selected_at DESC);

CREATE TABLE achievement_earned (
 person_id uuid NOT NULL REFERENCES people(id) ON DELETE CASCADE,
 badge_id text NOT NULL CHECK(badge_id IN (
  'first_checkin','first_photo','first_story','first_together',
  'many_turns','open_map','photos_remain','storyteller','again_together',
  'full_house','map_becomes_page','shared_memory','whole_journey')),
 earned_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(person_id,badge_id)
);
CREATE TABLE achievement_display (
 person_id uuid NOT NULL REFERENCES people(id) ON DELETE CASCADE,
 badge_id text NOT NULL,
 position smallint NOT NULL CHECK(position BETWEEN 0 AND 2),
 PRIMARY KEY(person_id,badge_id),
 UNIQUE(person_id,position),
 FOREIGN KEY(person_id,badge_id) REFERENCES achievement_earned(person_id,badge_id) ON DELETE CASCADE
);
CREATE TABLE achievement_mp4_credits (
 person_id uuid NOT NULL REFERENCES people(id) ON DELETE CASCADE,
 source text NOT NULL CHECK(source IN ('route:dau_chan','route:ky_niem','route:dong_hanh','route:nga_re','true_end')),
 granted_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(person_id,source)
);
