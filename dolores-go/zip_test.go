package main

import (
	"io/ioutil"
	"log"
	"os"
	"path"
	"reflect"
	"strings"
	"testing"
)

// Тестируем распаковку диагностики на тестовых данных
func checkDiagPartyExistence() bool {
	files, err := ioutil.ReadDir(dirToSave)
	if err != nil {
		log.Fatal(err)
	}
	for _, f := range files {
		if strings.Contains(f.Name(), "sql.party.xls") {
			err := os.Remove(path.Join(dirToSave, f.Name()))
			if err != nil {
				panic(err)
			}
			return true
		}
	}
	return false
}

func TestParseZipFile(t *testing.T) {
	dirToSave = "test_data"
	tests := []struct {
		name            string
		zippath         string
		want            *applicationVersions
		wantDiagProfile bool
		wantErr         bool
	}{
		{
			name:    "good", // корректные данные, достаем номер последней версии + версию Фактора
			zippath: "test_data/diag_good.zip",
			want: &applicationVersions{
				CoreRevision:     "2c980808",
				CustomerRevision: "01fbd6f4",
				CustomerName:     "demo",
				FactorTagVersion: "20.12",
			},
			wantDiagProfile: true,
			wantErr:         false,
		},
		{
			name:            "without diag", // ни логов с версиями, ни диагностики
			zippath:         "test_data/diag_empty.zip",
			want:            nil,
			wantDiagProfile: false,
			wantErr:         true,
		},
		{
			name:            "without diag", // логи есть, диагностики нет
			zippath:         "test_data/diag_without_diag.zip",
			want:            nil,
			wantDiagProfile: false,
			wantErr:         true,
		},
		{
			name:            "without versions", // диагностика есть, логов с версией нет
			zippath:         "test_data/diag_without_versions.zip",
			want:            nil,
			wantDiagProfile: false,
			wantErr:         true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseZipFile(tt.zippath)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseZipFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseZipFile() = %v, want %v", got, tt.want)
			}
			gotDiag := checkDiagPartyExistence()
			if gotDiag != tt.wantDiagProfile {
				t.Errorf("ParseZipFile() = %v, wantDiag %v", gotDiag, tt.wantDiagProfile)
			}
		})
	}
}
