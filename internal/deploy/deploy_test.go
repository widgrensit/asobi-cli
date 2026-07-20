package deploy

import (
	"archive/zip"
	"bytes"
	"testing"

	"github.com/widgrensit/asobi-cli/internal/client"
)

func TestZipScriptsEmitsValidTimestamps(t *testing.T) {
	scripts := []client.Script{
		{Path: "match.lua", Content: "return {}"},
		{Path: "lib/util.lua", Content: "return {}"},
	}

	archive, err := ZipScripts(scripts)
	if err != nil {
		t.Fatalf("ZipScripts: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	if len(zr.File) != len(scripts) {
		t.Fatalf("got %d entries, want %d", len(zr.File), len(scripts))
	}

	for _, f := range zr.File {
		mt := f.Modified
		// A zero mtime encodes to the invalid DOS date 1980-00-00
		// (month 0, day 0); a valid entry has month and day >= 1.
		if mt.Month() < 1 || mt.Day() < 1 {
			t.Errorf("%s: invalid DOS timestamp %s", f.Name, mt.Format("2006-01-02"))
		}
		if f.Method != zip.Deflate {
			t.Errorf("%s: method = %d, want Deflate", f.Name, f.Method)
		}
	}
}

func TestZipScriptsIsReproducible(t *testing.T) {
	scripts := []client.Script{{Path: "match.lua", Content: "return {}"}}

	a, err := ZipScripts(scripts)
	if err != nil {
		t.Fatalf("ZipScripts a: %v", err)
	}
	b, err := ZipScripts(scripts)
	if err != nil {
		t.Fatalf("ZipScripts b: %v", err)
	}
	if !bytes.Equal(a, b) {
		t.Error("archives differ across builds; timestamps should be fixed")
	}
}
