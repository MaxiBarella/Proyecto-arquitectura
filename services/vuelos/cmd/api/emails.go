package main

import (
	"errors"
	"fmt"
	"strings"
)

// parseAirlineEmails lee el mail de cada aerolínea con el formato AR:mail1,LA:mail2 (27.3).
func parseAirlineEmails(raw string) (map[string]string, error) {
	emails := map[string]string{}
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		airline, email, ok := strings.Cut(entry, ":")
		airline, email = strings.TrimSpace(airline), strings.TrimSpace(email)
		if !ok || airline == "" || !strings.Contains(email, "@") {
			return nil, fmt.Errorf("AEROLINEAS_EMAILS: %q no tiene la forma aerolínea:mail", entry)
		}
		emails[airline] = email
	}
	if len(emails) == 0 {
		return nil, errors.New("falta la variable AEROLINEAS_EMAILS, con la forma AR:mail,LA:mail")
	}
	return emails, nil
}
