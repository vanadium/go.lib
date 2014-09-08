package uniqueid

import "testing"

func TestNewID(t *testing.T) {
	g := RandomGenerator{}
	expectedResets := 5
	for i := 0; i < expectedResets*(1<<16); i++ {
		g.NewID()
	}
	if g.resets != expectedResets {
		t.Errorf("wrong number of resets, want %d got %d", expectedResets, g.resets)
	}
}

func BenchmarkNewIDParallel(b *testing.B) {
	g := RandomGenerator{}
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			g.NewID()
		}
	})
}
