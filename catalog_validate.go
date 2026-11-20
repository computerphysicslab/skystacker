package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func validateLocalBlindCatalogs() error {
	manifest, _, err := loadBlindCatalogManifest()
	if err != nil {
		return err
	}
	ok, missing := 0, 0
	for _, entry := range manifest.Catalogs {
		if entry.Status == "buildable" {
			fmt.Printf("%s: SKIP (buildable, not published)\n", entry.ID)
			continue
		}
		catalog, catDir, err := findBlindCatalog(entry.ID)
		if err != nil {
			fmt.Printf("%s: ERROR %v\n", entry.ID, err)
			missing++
			continue
		}
		found := ""
		for _, path := range []string{filepath.Join(catDir, catalog.File), blindCatalogCachePath(catalog)} {
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			if err := verifyBlindCatalog(data, catalog); err != nil {
				fmt.Printf("%s: ERROR %v\n", entry.ID, err)
				missing++
				found = path
				break
			}
			fmt.Printf("%s: OK (%d bytes, sha256 verified)\n", entry.ID, len(data))
			ok++
			found = path
			break
		}
		if found == "" {
			fmt.Printf("%s: MISSING (run --download-blind-catalog %s)\n", entry.ID, entry.ID)
			missing++
		}
	}
	fmt.Printf("Blind catalog validation: %d OK, %d missing/invalid\n", ok, missing)
	if missing > 0 {
		return fmt.Errorf("%d blind catalogs missing or invalid", missing)
	}
	return nil
}
