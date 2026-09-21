package main

import "testing"

func TestLaboratoryRefusesProductionConfiguration(t *testing.T) {
	base := map[string]string{"RUDI_CHAT_LAB": "1", "RUDI_CHAT_LAB_DATABASE_URL": "postgresql://localhost/chat_test"}
	for _, tc := range []struct{ key, value string }{{"RUDI_CHAT_LAB", ""}, {"RUDI_CHAT_LAB_LISTEN", "0.0.0.0:8198"}, {"RUDI_CHAT_LAB_LISTEN", ":8198"}, {"RUDI_CHAT_LAB_LISTEN", "127.0.0.1:0"}, {"RUDI_CHAT_LAB_DATABASE_URL", "postgresql://localhost/mobile"}} {
		t.Run(tc.key+tc.value, func(t *testing.T) {
			env := func(k string) string {
				if k == tc.key {
					return tc.value
				}
				return base[k]
			}
			if _, _, err := settings(env); err == nil {
				t.Fatal("unsafe laboratory settings accepted")
			}
		})
	}
	if _, _, err := settings(func(k string) string { return base[k] }); err != nil {
		t.Fatal(err)
	}
}
