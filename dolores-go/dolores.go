package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/docker/docker/client"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

const (
	// default время ожидания, пока сервер поднимется
	waitInPendingSeconds = 600
	// как в коде называется dockerfile
	dockerfile = "Dockerfile"
)

// IP виртуалки, на которой будет работать бот
var serverIP = "127.0.0.1"

// Cоздаем бота в телеге через botFather, получаем токен и потом с ним работаем
var botToken = "1319000055:AAFHVUNFLS"

var (
	// Массив taskToRun см в helpers.go
	taskChain                                   []taskToRun
	ports, filesToIncludeToContext, volumeBinds []string
	schemaName, dirToSave, cdiPort              string
)

func initVars(taskChain *[]taskToRun,
	ports, filesToIncludeToContext, volumeBinds *[]string,
	schemaName, dirToSave, cdiPort *string,
) {
	*cdiPort = "8080"
	*dirToSave = "diag"
	*schemaName = "cdi_temp_user_1"
	*ports = []string{
		"8080:8080",
		"18080:18080",
		"9990:9990",
		"19990:19990",
		"5005:5005",
	}
	// Эти файлы используем в коде для настроек
	*filesToIncludeToContext = []string{
		dockerfile,
		"settings_hflabs.xml",
		"diag/sql.party.xls",
	}
	// В директорию /opt/diag хост-машины докера будем монтировать /root/cdi-misc/dolores-bot/diag
	*volumeBinds = []string{
		"/root/cdi-misc/dolores-bot/diag:/opt/diag",
	}
	// Какие задачи запускать внутри приложения. У нас есть API, по которому можно дергать задачи из админки
	// Тут указываем, какие задачи будем использовать:
	// taskName — название задачи (по нему дергаем)
	// taskParams — параметры задачи
	// message — сообщение, которое напишет Долорес, если сможет успешно выполнить задачу
	*taskChain = []taskToRun{
		{
			// Задача, которая загрузит данные из эксельки в БД
			taskName: "importDataSetTask",
			taskParams: []*TaskParam{
				{ParamName: "dataSetFile", ParamValue: "/opt/diag/sql.party.xls"},
				{ParamName: "schemaName", ParamValue: *schemaName},
			},
			message: "Успешно загрузила диагностику",
		},
		{
			// Задача, которая перестраивает индексы lucene, по факту очищает кеши и приводит систему в консистентное состояние
			taskName:   "enginesFullRebuild",
			taskParams: nil,
			message:    "Успешно перестроила индексы",
		},
	}
}

func main() {
	flag.Parse()
	initVars(&taskChain,
		&ports, &filesToIncludeToContext, &volumeBinds,
		&schemaName, &dirToSave, &cdiPort)

	for _, task := range taskChain[0].taskParams {
		fmt.Printf("%+v\n", task)
	}
	log.SetOutput(os.Stdout)

	// Данные для входа в админку приложения
	cdiClient := newConnectToCdi(
		"admin_login",
		"admin_pass",
		"localhost",
		"8080",
	)

	dockerClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.Fatal(err)
	}

	botClient, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Panic(err)
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := botClient.GetUpdatesChan(u)
	if err != nil {
		panic(err)
	}

	as := newActiveSession(botClient, cdiClient, dockerClient)

	for update := range updates {
		go func(update tgbotapi.Update) {
			as.handleMessage(update)
		}(update)
	}
}
