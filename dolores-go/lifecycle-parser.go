package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"regexp"
	"strings"
)

// Код для обработки lifecycle-лога. Мы достанем оттуда:
// - имя заказчика (название проекта, который надо выкачать из git)
// - версию, на которой воспроизвели проблему (последняя запись в логе будет актуальным на момент сбора диагностики состоянием системы)
// - версию Фактора (смежная система, сборку которой скачаем из Team City)

var (
	// У нас 2 шаблона записи строки в этот лог, парсим оба
	lifecycleStartLineTemplate  = regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2},\d{3} INFO  start - CDI application \[([\w ]+) ([0-9\.]+)-SNAPSHOT (?:.*?)\(([0-9a-z]+), core ([0-9a-z]+)\)\] \[(?:.*?)\] started in \d+ s\.$`)
	lifecycleStartLineTemplate2 = regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2},\d{3} INFO  LifecycleEventListener - CDI application \[([\w ]+) ([0-9\.]+)-SNAPSHOT (?:.*?)\(([0-9a-z]+), core ([0-9a-z]+)\)\] \[[0-9\.]+\] started in \d+ s\.`)
)

// Устанавливаем алиасы на то, что прочитали в логе. Может быть такая ситуация: в логе пишется длинное имя, а в системе контроля версий репозиторий называется по-другому, кратко
var clientsAliases = map[string]string{
	"demo":      "demo",
	"cdi test":  "test",
	"bank":      "bank",
	"long name": "name",
}

// Структура переменной applicationVersions: что мы достаем из лога и сохраняем в версию
type applicationVersions struct {
	CoreRevision     string
	CustomerRevision string
	CustomerName     string
	FactorTagVersion string
}

func parseapplicationVersions(in io.Reader) (*applicationVersions, error) {
	scanner := bufio.NewScanner(in)
	res := new(applicationVersions)
	found := false
	for scanner.Scan() {
		// Ищем соответствие с нашим шаблоном
		if !lifecycleStartLineTemplate.MatchString(scanner.Text()) && !lifecycleStartLineTemplate2.MatchString(scanner.Text()) {
			continue
		}
		matches := lifecycleStartLineTemplate.FindAllStringSubmatch(scanner.Text(), -1)
		matches2 := lifecycleStartLineTemplate2.FindAllStringSubmatch(scanner.Text(), -1)
		// Делаем выбор нужной строки
		if len(matches) < len(matches2) {
			matches = matches2
		}
		if len(matches[0]) != 5 {
			continue
		}
		found = true
		// Из полученного паттерна достаем информацию по ревизии кода, версии Фактора и имени заказчика
		res.CoreRevision = matches[0][4]
		res.CustomerRevision = matches[0][3]
		res.FactorTagVersion = matches[0][2]
		res.CustomerName = clientsAliases[strings.ToLower(matches[0][1])]
	}
	if !found {
		return nil, fmt.Errorf("application version not found in lifecycle log")
	}
	log.Printf("PARSED VERSIONS: %+v\n", res)

	return res, nil
}
