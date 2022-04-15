package main

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
)

// Парсим архив, который передали Долорес

func parseZipFile(zippath string) (*applicationVersions, error) {
	z, err := zip.OpenReader(zippath)
	if err != nil {
		return nil, fmt.Errorf("could not open zip file: %w", err)
	}
	defer z.Close()

	parseVersions := false
	parsePartyDiag := false
	versions := new(applicationVersions)

	for _, f := range z.File {
		switch f.Name {
		//  Если в папке cdi.logs обнаружили cdi-lifecycle.log — считываем его
		case "cdi.logs/cdi-lifecycle.log":
			handle, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("could not open lifecycle log: %w", err)
			}
			defer func() {
				if err := handle.Close(); err != nil {
					panic(err)
				}
			}()
			versions, err = parseapplicationVersions(handle)
			if err != nil {
				return nil, err
			}
			parseVersions = true
		// Если в корне архива обнаружили sql.party.xls (диагностика) — считываем её
		case "sql.party.xls":
			handle, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("could not open sql.party.xls: %w", err)
			}
			defer func() {
				if err := handle.Close(); err != nil {
					panic(err)
				}
			}()
			// Куда сохранить файл
			pathToSave := filepath.Join(dirToSave, f.Name) //nolint:gosec
			fLocal, err := os.OpenFile(pathToSave, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return nil, fmt.Errorf("could not create sql.party.xls file to copy: %w", err)
			}
			_, err = io.Copy(fLocal, handle) //nolint:gosec
			if err != nil {
				return nil, err
			}
			parsePartyDiag = true
		}
	}

	if !parseVersions && !parsePartyDiag {
		return nil, fmt.Errorf("there is no sql.party.xls file in the zip and versions did not parsed correctly")
	}

	if !parseVersions {
		err := os.Remove(path.Join(dirToSave, "sql.party.xls"))
		if err != nil {
			return nil, fmt.Errorf("there is no lifecycle log and cannot remove sql.party: %v", err)
		}
		return nil, fmt.Errorf("there is no lifecycle log file in the zip")
	}

	if !parsePartyDiag {
		return nil, fmt.Errorf("there is no sql.party.xls file in the zip")
	}

	return versions, nil
}
