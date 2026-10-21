package deps

import (
	"testing"
)

func TestModeString(t *testing.T) {
	tests := []struct {
		name string
		mode Mode
		want string
	}{
		{name: "dev", mode: ModeDev, want: "devDependencies"},
		{name: "optional", mode: ModeOptional, want: "optionalDependencies"},
		{name: "prod", mode: ModeProd, want: "dependencies"},
		{name: "peer", mode: ModePeer, want: "peerDependencies"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.mode.String(); got != tt.want {
				t.Errorf("Mode(%d).String() = %q, want %q", tt.mode, got, tt.want)
			}
		})
	}
}

func TestNewMode(t *testing.T) {
	tests := []struct {
		name                      string
		dev, optional, peer, prod bool
		wantDev, wantOpt          bool
		wantPeer, wantProd        bool
	}{
		{name: "nothing set falls back to prod", wantProd: true},
		{name: "dev only", dev: true, wantDev: true},
		{name: "optional only", optional: true, wantOpt: true},
		{name: "peer only", peer: true, wantPeer: true},
		{name: "prod only", prod: true, wantProd: true},
		{name: "dev and prod", dev: true, prod: true, wantDev: true, wantProd: true},
		{name: "all", dev: true, optional: true, peer: true, prod: true, wantDev: true, wantOpt: true, wantPeer: true, wantProd: true},
		{name: "dev and optional", dev: true, optional: true, wantDev: true, wantOpt: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewMode(tt.dev, tt.optional, tt.peer, tt.prod)
			checks := []struct {
				name string
				mode Mode
				want bool
			}{
				{"dev", ModeDev, tt.wantDev},
				{"optional", ModeOptional, tt.wantOpt},
				{"peer", ModePeer, tt.wantPeer},
				{"prod", ModeProd, tt.wantProd},
			}
			for _, c := range checks {
				if (got&c.mode != 0) != c.want {
					t.Errorf("NewMode(%v,%v,%v,%v): %s bit = %v, want %v",
						tt.dev, tt.optional, tt.peer, tt.prod, c.name, got&c.mode != 0, c.want)
				}
			}
		})
	}
}

func TestNewModeDoesNotAddProd(t *testing.T) {
	if got := NewMode(true, false, false, false); got&ModeProd != 0 {
		t.Errorf("NewMode(dev) should not include prod, got %v", got)
	}
	if got := NewMode(false, false, false, false); got != ModeProd {
		t.Errorf("NewMode() with no flags = %v, want %v", got, ModeProd)
	}
}

func TestModeIterator(t *testing.T) {
	t.Run("single mode yields exactly one entry", func(t *testing.T) {
		var gotMode []Mode
		var gotName []string
		for m, s := range ModePeer.Iterator() {
			gotMode = append(gotMode, m)
			gotName = append(gotName, s)
		}
		if len(gotMode) != 1 || gotMode[0] != ModePeer {
			t.Fatalf("expected [ModePeer], got %v", gotMode)
		}
		if gotName[0] != "peerDependencies" {
			t.Errorf("name = %q, want %q", gotName[0], "peerDependencies")
		}
	})

	t.Run("combined mode yields every set bit in a stable order", func(t *testing.T) {
		combined := NewMode(true, true, true, true)
		var gotName []string
		for _, s := range combined.Iterator() {
			gotName = append(gotName, s)
		}
		want := []string{"devDependencies", "optionalDependencies", "dependencies", "peerDependencies"}
		if len(gotName) != len(want) {
			t.Fatalf("got %v, want %v", gotName, want)
		}
		for i := range want {
			if gotName[i] != want[i] {
				t.Errorf("entry %d = %q, want %q", i, gotName[i], want[i])
			}
		}
	})

	t.Run("zero mode yields nothing", func(t *testing.T) {
		count := 0
		for range Mode(0).Iterator() {
			count++
		}
		if count != 0 {
			t.Fatalf("expected no entries for the zero mode, got %d", count)
		}
	})

	t.Run("early termination stops the iteration", func(t *testing.T) {
		count := 0
		for range NewMode(true, true, true, true).Iterator() {
			count++
			break
		}
		if count != 1 {
			t.Fatalf("expected exactly one entry before stopping, got %d", count)
		}
	})
}
