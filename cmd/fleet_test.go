package cmd

import (
	"testing"

	"github.com/depado/minifleet/internal/manifest"
)

func TestResolveFleet(t *testing.T) {
	fleetDir := t.TempDir()
	mf := &manifest.FleetManifest{Version: "1", Owner: "dauph-in"}
	if err := manifest.Save(mf, manifest.Path(fleetDir)); err != nil {
		t.Fatal(err)
	}
	conf := &Conf{Fleet: FleetConf{KnownFleets: map[string]string{"dauph-in": fleetDir}}}

	t.Run("known fleet wins over CWD without manifest", func(t *testing.T) {
		t.Chdir(t.TempDir())
		target, ok := resolveFleet(conf, "github.com", "dauph-in")
		if !ok {
			t.Fatal("expected manifest to be found")
		}
		if target.Dir != fleetDir {
			t.Errorf("dir = %q, want %q", target.Dir, fleetDir)
		}
	})

	t.Run("unknown owner falls back to CWD", func(t *testing.T) {
		cwd := t.TempDir()
		t.Chdir(cwd)
		target, ok := resolveFleet(conf, "github.com", "someone-else")
		if ok {
			t.Fatal("expected no manifest")
		}
		if target.Dir != cwd {
			t.Errorf("dir = %q, want %q", target.Dir, cwd)
		}
	})

	t.Run("CWD with matching manifest wins", func(t *testing.T) {
		t.Chdir(fleetDir)
		target, ok := resolveFleet(&Conf{}, "github.com", "dauph-in")
		if !ok {
			t.Fatal("expected manifest to be found")
		}
		if target.Dir != fleetDir {
			t.Errorf("dir = %q, want %q", target.Dir, fleetDir)
		}
	})
}
