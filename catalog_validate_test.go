package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateLocalBlindCatalogs(t *testing.T) {
	dir := t.TempDir()
	data := []byte("SBI1-catalog-fixture")
	sum := sha256.Sum256(data)
	manifest := blindCatalogManifest{Version: 1, Catalogs: []blindCatalog{{
		ID: "test", Title: "Test", Nside: 4, MagLimit: 10, Brightest: 100,
		File: "test.sbi", SHA256: hex.EncodeToString(sum[:]), Bytes: int64(len(data)), Stars: 1, Quads: 1, Status: "published",
	}}}
	manifestData, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), manifestData, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "test.sbi"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SKYSTACKER_CATALOGS", dir)
	if err := validateLocalBlindCatalogs(); err != nil {
		t.Fatal(err)
	}
}
