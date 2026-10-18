package money

import "poc-spire/internal/identity"

func permitted(method, path, id string) bool {
	return method == "POST" && path == "/transfer" && id == identity.Trusted.String() ||
		method == "GET" && path == "/balances" && id == identity.Observer.String()
}
