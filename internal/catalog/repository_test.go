package catalog

import "testing"

type stubRepo struct{}

func (stubRepo) Addresses() []Address            { return nil }
func (stubRepo) Places(Address, Section) []Place { return nil }
func (stubRepo) Menu(string) (Place, bool)       { return Place{}, false }
func (stubRepo) Usual(Address) (Usual, bool)     { return Usual{}, false }
func (stubRepo) InstamartItems(Address) []Item   { return nil }

func TestStubSatisfiesRepository(t *testing.T) {
	var _ Repository = stubRepo{}
}
