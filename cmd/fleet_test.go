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

	t.Run("known fleet wins over current directory without manifest", func(t *testing.T) {
		t.Chdir(t.TempDir())
		target, ok := resolveFleet(conf, "github.com", "dauph-in")
		if !ok {
			t.Fatal("expected manifest to be found")
		}
		if target.Dir != fleetDir {
			t.Errorf("dir = %q, want %q", target.Dir, fleetDir)
		}
	})

	t.Run("unknown owner falls back to current directory", func(t *testing.T) {
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

	t.Run("current directory with matching manifest wins", func(t *testing.T) {
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

func TestDiscoverFleets(t *testing.T) {
	fleetDir := t.TempDir()
	if err := manifest.Save(&manifest.FleetManifest{Version: "1", Owner: "dauph-in"}, manifest.Path(fleetDir)); err != nil {
		t.Fatal(err)
	}
	otherDir := t.TempDir()
	if err := manifest.Save(&manifest.FleetManifest{Version: "1", Owner: "other"}, manifest.Path(otherDir)); err != nil {
		t.Fatal(err)
	}
	conf := &Conf{Fleet: FleetConf{KnownFleets: map[string]string{"dauph-in": fleetDir, "other": otherDir}}}

	t.Run("current directory manifest wins by default", func(t *testing.T) {
		t.Chdir(fleetDir)
		targets := discoverFleets(conf, false)
		if len(targets) != 1 || targets[0].Dir != fleetDir {
			t.Errorf("targets = %+v, want single %q", targets, fleetDir)
		}
	})

	t.Run("all bypasses current directory and returns known fleets", func(t *testing.T) {
		t.Chdir(fleetDir)
		targets := discoverFleets(conf, true)
		if len(targets) != 2 {
			t.Fatalf("len(targets) = %d, want 2", len(targets))
		}
		if targets[0].Owner != "dauph-in" || targets[1].Owner != "other" {
			t.Errorf("targets = %+v, want dauph-in then other", targets)
		}
	})

	t.Run("all bypasses fleet.path", func(t *testing.T) {
		t.Chdir(t.TempDir())
		c := &Conf{Fleet: FleetConf{Path: fleetDir, KnownFleets: conf.Fleet.KnownFleets}}
		targets := discoverFleets(c, true)
		if len(targets) != 2 {
			t.Errorf("len(targets) = %d, want 2", len(targets))
		}
	})
}
