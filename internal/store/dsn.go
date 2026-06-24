package store

import "net/url"

// brokerDSN rewrites an owner DSN to connect as the console_broker role.
// Migrate creates console_broker with a known dev password.
func brokerDSN(ownerDSN string) (string, error) {
	u, err := url.Parse(ownerDSN)
	if err != nil {
		return "", err
	}
	u.User = url.UserPassword("console_broker", "console_broker_dev")
	return u.String(), nil
}
