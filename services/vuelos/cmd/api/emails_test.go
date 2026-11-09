package main

import "testing"

func TestParseAirlineEmails(t *testing.T) {
	got, err := parseAirlineEmails(" AR:ops@ar.test , LA:ops@la.test ")
	if err != nil {
		t.Fatal(err)
	}
	if got["AR"] != "ops@ar.test" || got["LA"] != "ops@la.test" || len(got) != 2 {
		t.Errorf("mails: %v", got)
	}
	for _, bad := range []string{"", "AR", "AR:sin-arroba", ":ops@ar.test"} {
		if _, err := parseAirlineEmails(bad); err == nil {
			t.Errorf("%q tendría que dar error", bad)
		}
	}
}
