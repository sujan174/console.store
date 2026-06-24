package main

import (
	"console.store/internal/auth"
	"console.store/internal/broker"
)

// Compile-time assertions that the adapters satisfy the broker/auth seams.
var _ broker.TokenStore = brokerStore{}
var _ auth.AccountStore = authStore{}
