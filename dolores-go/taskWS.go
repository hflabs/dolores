package main

import (
	"bytes"
	"crypto/tls"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"text/template"
	"time"
)

// Код для вызова задач CDI по API

// Шаблон для запроса "запусти задачу такую-то"
var executeTemplate = `
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns="http://hflabs.ru/cdi/task/15_3">
    <soapenv:Header/>
    <soapenv:Body>
        <executeTaskRequest>
            <name>{{.Name}}</name>
            {{range .TaskParams}}
            <parameters name="{{.ParamName}}">
            <value>{{.ParamValue}}</value>
            </parameters>
            {{end}}
        </executeTaskRequest>
    </soapenv:Body>
</soapenv:Envelope>
`

// Шаблон для запроса проверки статуса работы задачи
var statusTemplate = `
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns="http://hflabs.ru/cdi/task/15_3">
   <soapenv:Header/>
   <soapenv:Body>
      <getTaskStatusRequest>{{.ID}}</getTaskStatusRequest>
   </soapenv:Body>
</soapenv:Envelope>
`

var (
	tmplExecute = template.Must(template.New("execute").Parse(executeTemplate))
	tmplStatus  = template.Must(template.New("status").Parse(statusTemplate))
)

// WSApi interface to work with CDI api
type WSApi interface {
	createRequest(tmpl *template.Template) []byte
	parseResponse(body io.Reader)
}

// Task struct to store task fields
type Task struct {
	cdi        *connectToCdi
	Name       string
	TaskParams []*TaskParam
	ID         string `xml:"Body>executeTaskResponse>id"`
	Status     string `xml:"Body>getTaskStatusResponse>state"`
	Desc       string `xml:"Body>getTaskStatusResponse>description"`
	TimeStamp  time.Time
}

// TaskParam parameters of the task
type TaskParam struct {
	ParamName  string
	ParamValue interface{}
}

// Создаем реквест
func (t *Task) createRequest(tmpl *template.Template) []byte {
	var res bytes.Buffer
	if err := tmpl.Execute(&res, t); err != nil {
		log.Println(err)
	}

	return res.Bytes()
}

// Проверяем ответ
func (t *Task) parseResponse(body io.Reader) {
	err := xml.NewDecoder(body).Decode(t)
	if err != nil {
		log.Println(err)
	}
}

// Метод для выполнения запроса
func (cdi *connectToCdi) doRequest(api WSApi, tmpl *template.Template) {
	httpMethod := "POST"
	payload := api.createRequest(tmpl)

	r, err := http.NewRequest(httpMethod, cdi.url, bytes.NewReader(payload))
	if err != nil {
		log.Println(err)
	}

	r.Header.Set("Content-type", "text/xml")
	r.SetBasicAuth(cdi.username, cdi.password)

	resp, err := cdi.client.Do(r)
	if err != nil {
		log.Println(err)
		return
	}
	if resp.StatusCode != http.StatusOK {
		log.Println(err)
	}
	defer resp.Body.Close()

	api.parseResponse(resp.Body)
}

type cdiChecker interface {
	runTaskAndWait(taskName string, taskParams []*TaskParam) (string, bool)
}

// Создаем новое соединение
type connectToCdi struct {
	client   *http.Client
	username string
	password string
	url      string
}

func newConnectToCdi(username, password, domain, port string) *connectToCdi {
	return &connectToCdi{
		client: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true, //nolint:gosec
				},
			},
		},
		username: username,
		password: password,
		url:      fmt.Sprintf("http://%s:%s/cdi/soap/services/15_3/TaskWS", domain, port),
	}
}

// Для запуска задачи нужно будет выполнить doRequest
func (t *Task) run() {
	t.cdi.doRequest(t, tmplExecute)
	t.TimeStamp = time.Now()
}

// Для проверки статуса — метод checkStatus
func (t *Task) checkStatus() {
	t.cdi.doRequest(t, tmplStatus)
	t.TimeStamp = time.Now()
}

// Запускаем задачи — выполяем функции run() и checkStatus(). Статус выверяем
func (cdi *connectToCdi) runTaskAndWait(taskName string, taskParams []*TaskParam) (string, bool) {
	task := &Task{
		cdi:        cdi,
		Name:       taskName,
		TaskParams: taskParams,
	}
	task.run()
	for {
		time.Sleep(5 * time.Second)
		task.checkStatus()
		switch task.Status {
		case "RUNNING":
			continue
		case "FINISHED":
			return fmt.Sprintf("%s: %s", task.Status, task.Desc), true
		case "SKIPPED":
			return fmt.Sprintf("%s: %s", task.Status, task.Desc), true
		case "ERROR":
			return fmt.Sprintf("%s: %s", task.Status, task.Desc), false
		default:
			return fmt.Sprintf("%s: %s", task.Status, task.Desc), false
		}
	}
}
