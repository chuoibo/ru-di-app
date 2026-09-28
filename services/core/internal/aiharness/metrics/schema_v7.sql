-- aiharness version 7: a couple's shared taste (ADR-0048). The tool
-- gu_hai_ban joins the registry, so the one array column may name it. The
-- row records THAT the tool ran, never what it returned: no taste, no tag,
-- no name reaches this table. Still a closed list; version 2's column CHECK
-- is replaced by its generated name, and nothing else of the table changes.
ALTER TABLE ai_turn_metrics
  DROP CONSTRAINT IF EXISTS ai_turn_metrics_cong_cu_check,
  ADD CONSTRAINT ai_turn_metrics_cong_cu_check
    CHECK (cong_cu <@ ARRAY['search_places','get_place','list_destinations','nearest_area','group_snapshot','list_group_outings','gu_hai_ban','search_app_manual','explain_screen','propose_places','propose_itinerary','draft_poll','suggest_screen','my_upcoming_outings','recall_memory','remember_fact','forget_fact','what_you_remember','set_reminder']::text[]);
