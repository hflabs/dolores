package main

import (
	"os"
	"reflect"
	"testing"
)

// Любой код надо тестировать! Даже код бота =)
// Автотесты на работу lifecycle-parser.go

// Считываем тестовый файлик
func readLifecycleLogAndParse(filepath string) (*applicationVersions, error) {
	handle, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer handle.Close()
	return parseapplicationVersions(handle)
}

// Парсим!
func TestReadLifecycleLogAndParse(t *testing.T) {
	tests := []struct {
		name     string
		filepath string
		want     *applicationVersions
		wantErr  bool
	}{
		{
			name: "versions exist",
			// В этом файлике игнорируем последние строки про «Application migration» и берем данные из последнего start — Demo 21.19
			filepath: "test_data/cdi-lifecycle.log",
			want: &applicationVersions{
				CoreRevision:     "2c980808", // тут было другое значение, но по логике это же?
				CustomerRevision: "01fbd6f4",
				CustomerName:     "demo",
				FactorTagVersion: "21.19",
			},
			wantErr: false,
		},
		{
			name: "two words",
			// В этом файлике видим имя заказчика из двух слов и подменяем его на название проекта в гите
			filepath: "test_data/cdi-lifecycle-2.log",
			want: &applicationVersions{
				CoreRevision:     "badfd026",
				CustomerRevision: "eb02e922",
				CustomerName:     "name",
				FactorTagVersion: "20.12",
			},
			wantErr: false,
		},
		{
			name:     "versions not exist",
			filepath: "test_data/cdi-lifecycle-not-found.log",
			want:     nil,
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readLifecycleLogAndParse(tt.filepath)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReadLifecycleLogAndParse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ReadLifecycleLogAndParse() = %v, want %v", got, tt.want)
			}
		})
	}
}
