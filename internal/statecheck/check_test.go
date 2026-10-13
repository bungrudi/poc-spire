package statecheck

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBrokerMarkerAndMetadataBoundToRun(t *testing.T) {
	dir := t.TempDir()
	stream := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	put := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	put("owned-initialized", "run\npod")
	put("meta.properties", "cluster.id=cluster\nnode.id=1\nversion=1\n")
	first := Check("kafka", dir, "run", "pod", stream, "cluster", false)
	if !first.Valid {
		t.Fatal("initialized broker rejected")
	}
	if Check("kafka", dir, "other", "pod", stream, "cluster", false).Valid || Check("kafka", dir, "run", "pod", stream, "foreign", false).Valid {
		t.Fatal("foreign broker incarnation accepted")
	}
	if Check("kafka", dir, "run", "pod", stream, "cluster", true).Valid {
		t.Fatal("lost broker topics accepted")
	}
	for _, name := range []string{"__cluster_metadata-0", "poc.spire.commands." + stream + "-0"} {
		if err := os.Mkdir(filepath.Join(dir, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if !Check("kafka", dir, "run", "pod", stream, "cluster", true).Valid {
		t.Fatal("complete broker rejected")
	}
	put("meta.properties", "cluster.id=cluster\nnode.id=1\nversion=1\ncluster.id=foreign\n")
	if Check("kafka", dir, "run", "pod", stream, "cluster", false).Valid {
		t.Fatal("ambiguous broker metadata accepted")
	}
}
