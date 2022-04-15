package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/client"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// enum for handling session status
type sessionStatus int

const (
	ACTIVE sessionStatus = iota
	DISACTIVE
	PENDING
)

type telegramUser struct {
	username string
	id       int64
}

func newTelegramUser(username string, id int64) *telegramUser {
	return &telegramUser{
		username: username,
		id:       id,
	}
}

type botSender interface {
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
	GetFileDirectURL(fileID string) (string, error)
}

// struct to handle active user session
type activeSession struct {
	// sync mutex
	mu sync.Mutex
	// username and ID from telegram
	user *telegramUser
	// time when the session started
	time string
	// name of the image and container to run {customerName}-{customerVersion}-{chatID}
	customer string
	// path to diagnostic zip
	diagZipPath string
	// version and revisions of cdi and factor to start
	versions *applicationVersions
	// current status
	status sessionStatus
	// telegram bot connection
	bot botSender
	// cdi connection
	cdi cdiChecker
	// docker connection
	docker DockerRunner
	// sessions queue
	q                    *sessionsQueue
	waitInPendingSeconds time.Duration
}

func newActiveSession(bot *tgbotapi.BotAPI, cdi *connectToCdi, docker *client.Client) *activeSession {
	return &activeSession{
		status:               DISACTIVE,
		bot:                  bot,
		cdi:                  cdi,
		docker:               NewDockerClient(docker),
		q:                    newSessionsQueue(),
		waitInPendingSeconds: waitInPendingSeconds,
	}
}

func (as *activeSession) cleanUp() {
	as.mu.Lock()
	defer as.mu.Unlock()
	log.Println("cleanup after superhuman request")
	as.user = nil
	as.time = ""
	as.customer = ""
	as.diagZipPath = ""
	as.versions = nil
	as.status = DISACTIVE
	as.q = newSessionsQueue()
}

// only change session status to active
// need when user start an interaction
// and send a zip file
func (as *activeSession) activate() {
	as.mu.Lock()
	defer as.mu.Unlock()
	as.status = ACTIVE
}

// Можно встать в очередь и Долорес напишет, когда она освободится.
// Но если её игнорировать waitInPendingSeconds секунд, то скажет "Сорри, я ушла" и выкинет эту сессию из головы
func (as *activeSession) waitInPending() {
	time.Sleep(as.waitInPendingSeconds * time.Second)
	if as.status == PENDING {
		_, err := as.bot.Send(newMessage(as.user.id, "Не дождалась, очередь пошла дальше"))
		if err != nil {
			log.Println("ERROR: ", err)
		}
		as.deactivate()
	}
}

// deactivate status and clean all session info
func (as *activeSession) deactivate() {
	as.mu.Lock()
	defer as.mu.Unlock()
	if as.user != nil {
		log.Printf("try to deactivate session for %s\n", as.user.username)
	} else {
		log.Println("try to deactivate session for nil user")
	}

	as.user = as.q.getNext()
	as.time = ""
	as.customer = ""
	as.diagZipPath = ""
	as.versions = nil
	err := as.docker.KillRunningContainers(as.getCustomer())
	if err != nil {
		fmt.Printf("fail to cleanup: %v\n", err)
	}
	if as.user != nil {
		_, err := as.bot.Send(newMessage(as.user.id, "Стенд освободился, можно начинать развертывание. Жду 10 минут и передаю очередь дальше."))
		if err != nil {
			log.Println("ERROR: ", err)
		}
		as.status = PENDING
		go as.waitInPending()
		return
	}
	as.status = DISACTIVE
}

// set of base getters and setters

func (as *activeSession) isActive(userID int64) bool {
	if as.user == nil {
		return false
	}
	if as.status != DISACTIVE && as.user.id != userID {
		return true
	}
	return false
}

func (as *activeSession) getUser() string {
	return as.user.username
}

func (as *activeSession) getCustomer() string {
	return as.customer
}

func (as *activeSession) setCustomer(name, version string, id int64) {
	as.mu.Lock()
	defer as.mu.Unlock()
	name = strings.Replace(name, " ", "-", -1)
	as.customer = fmt.Sprintf("%v-%v-%v", name, version, id)
}

func (as *activeSession) setVersions(versions *applicationVersions) {
	as.mu.Lock()
	defer as.mu.Unlock()
	as.versions = versions
}

func (as *activeSession) getVersions() *applicationVersions {
	return as.versions
}

func (as *activeSession) getVersionsString() string {
	return fmt.Sprintf(
		"%s-%s (%s, core %s)",
		as.versions.CustomerName,
		as.versions.FactorTagVersion,
		as.versions.CustomerRevision,
		as.versions.CoreRevision,
	)
}

func (as *activeSession) setDiagZipPath(path string) {
	as.mu.Lock()
	defer as.mu.Unlock()
	as.diagZipPath = path
}

func (as *activeSession) getTimeFrom() string {
	return as.time
}

func (as *activeSession) setActiveUser(update tgbotapi.Update) {
	as.mu.Lock()
	defer as.mu.Unlock()
	if update.Message.From != nil {
		as.user = newTelegramUser(update.Message.From.String(), update.Message.Chat.ID)
	}
	as.time = time.Now().Format("2006-01-02 15:04:05")
}

// messages for Dolores
var doloresMessages = struct {
	tryToStop                  string
	addedToQueue               string
	alreadyInQueue             string
	notInQueue                 string
	exitQueue                  string
	somethingWrong             string
	sessionSuccessfullyDeleted string
	busy                       string
	busyWrong                  string
	hello                      string
	tooBigFile                 string
	cannotGetDownloadLink      string
	cannotDownload             string
	onlyZip                    string
	fileDownloaded             string
	cannotParseVersion         string
	startDeploy                string
	imageFail                  string
	imageSuccess               string
	containerFail              string
	containerSuccess           string
	containerCheckError        string
	containerIsDead            string
	containerExists            string
	cdiAlive                   string
	cdiStartingWait            string
	cdiTimeout                 string
	taskFailed                 string
	allDone                    string
	notifyForDelete            string
}{
	tryToStop:                  "Пытаюсь остановить работающий контейнер...",
	addedToQueue:               "Добавила тебя в очередь на место %v",
	alreadyInQueue:             "А ты уже в очереди на месте %v",
	notInQueue:                 "А тебя нет в очереди",
	exitQueue:                  "Убрала тебя из очереди, заходи еще",
	somethingWrong:             "Что-то пошло не так. Зови создателя.",
	sessionSuccessfullyDeleted: "Контейнер удален, сессия закончена. Приходи еще, мясной мешочек, и расскажи другим.",
	busy:                       "Сейчас я уже помогаю человеку %s с развертыванием %s с %s. Можешь пока занять очередь, тогда я напишу тебе, как стенд освободится.",
	busyWrong:                  "Сейчас уже есть запущенный контейнер, который никому не принадлежит. У меня не получилось его убить, позови создателя.",
	hello:                      "Привет, человек, сейчас я свободна. Пришли мне файл с диагностикой, я постараюсь помочь",
	tooBigFile:                 "Файл слишком большой :( Нужно до 20мб.",
	cannotGetDownloadLink:      "Не смогла получить ссылку на файл, что-то не так",
	cannotDownload:             "Не получилось скачать файл. Где-то ошибочка, пусть создатель посмотрит",
	onlyZip:                    "Я понимаю только ZIP архивы с диагностиками, прости",
	fileDownloaded:             "Файл скачала, изучаю...",
	cannotParseVersion:         "Не смогла понять версию приложения или найти диагностику. Где-то ошибочка, пусть создатель посмотрит",
	startDeploy:                "Начинаю разворачивать %s",
	imageFail:                  "Не смогла собрать докер образ. Где-то ошибочка, пусть создатель посмотрит",
	imageSuccess:               "Успешно собрала докер-образ, начинаю разворачивать контейнер",
	containerFail:              "Не смогла запустить контейнер. Где-то ошибочка, пусть создатель посмотрит",
	containerSuccess:           "Контейнер поднялся. Жду пока ЕК оживёт, чтобы приступить к заливке диагностики",
	containerCheckError:        "Не могу проверить состояние контейнера, что-то не так. Позови создателя",
	containerIsDead:            "Во время старта «Единого клиента» произошла ошибка. Позови создателя",
	containerExists:            "Нашла существующий контейнер с таким именем, удаляю...",
	cdiAlive:                   "Единый клиент жив! Начинаю заливать диагностику",
	cdiStartingWait:            "Ещё жду... немного терпения",
	cdiTimeout:                 "Единый клиент так и не поднялся, надо разбираться. Пусть создатель посмотрит.",
	taskFailed:                 "Стенд развернут здесь http://%v:%v/cdi/ui/, но задача не отработала, что-то пошло не так: %s",
	allDone:                    "Все готово! Любуйся http://%v:%v/cdi/ui/. Не забудь удалить контейнер, а то стенд всего один.",
	notifyForDelete:            "Может уже можно удалить контейнер и освободить стенд?",
}

// if bot receive the callback message:
// * try to stop container by name and deactivate the session
// * if there is an error in docker API, we need to see the result manually
func (as *activeSession) handleCallbackQuery(update tgbotapi.Update) {
	tgUser := newTelegramUser(update.CallbackQuery.From.String(), int64(update.CallbackQuery.From.ID))
	switch update.CallbackQuery.Data {
	case "takePlace":
		place, alreadyInQueue := as.q.takePlace(tgUser)
		message := doloresMessages.addedToQueue
		if alreadyInQueue {
			message = doloresMessages.alreadyInQueue
			log.Println("user already in queue")
		}
		_, err := as.bot.Send(newMessageWithButton(
			int64(update.CallbackQuery.From.ID),
			fmt.Sprintf(message, place),
			"Покинуть очередь",
			"exitQueue",
		))
		if err != nil {
			log.Println("ERROR: ", err)
		}
		log.Println("user added to queue")
		return
	case "exitQueue":
		err := as.q.exit(tgUser)
		message := doloresMessages.exitQueue
		if err != nil {
			message = doloresMessages.notInQueue
		}
		_, err = as.bot.Send(newMessage(int64(update.CallbackQuery.From.ID), message))
		if err != nil {
			log.Println("ERROR: ", err)
		}
		return
	case "goNext":
		as.deactivate()
		_, err := as.bot.Send(newMessage(int64(update.CallbackQuery.From.ID), "Передал очередь дальше"))
		if err != nil {
			log.Println("ERROR: ", err)
		}
	case "cleanUp":
		as.deactivate()
		as.cleanUp()
		_, err := as.bot.Send(newMessage(int64(update.CallbackQuery.From.ID), "Сбросил состояние"))
		if err != nil {
			log.Println("ERROR: ", err)
		}
	default:
		log.Printf("try to stop and delete active container from user %s\n", update.CallbackQuery.From.String())
		containerName := update.CallbackQuery.Data
		_, err := as.bot.Send(newMessage(int64(update.CallbackQuery.From.ID), doloresMessages.tryToStop))
		if err != nil {
			log.Println("ERROR: ", err)
		}
		err = as.docker.StopAndRemoveContainer(containerName)
		if err != nil {
			log.Println(err)
			_, err := as.bot.Send(newMessage(int64(update.CallbackQuery.From.ID), doloresMessages.somethingWrong))
			if err != nil {
				log.Println("ERROR: ", err)
			}
			return
		}
		_, err = as.bot.Send(newMessage(int64(update.CallbackQuery.From.ID), doloresMessages.sessionSuccessfullyDeleted))
		if err != nil {
			log.Println("ERROR: ", err)
		}
		as.deactivate()
		return
	}
}

// if Dolores is busy (has ACTIVE status)
func (as *activeSession) handleBusy(update tgbotapi.Update) {
	_, err := as.bot.Send(
		newMessageWithButton(
			update.Message.Chat.ID,
			fmt.Sprintf(doloresMessages.busy, as.getUser(), as.getCustomer(), as.getTimeFrom()),
			"Встать в очередь",
			"takePlace",
		),
	)
	if err != nil {
		log.Println("ERROR: ", err)
	}
	log.Printf("user %+v want to start building\n", update.Message.From)
}

// if bot receive a file
// * check wheather it has .zip extension
// * if the file is bigger then 20mb – fail
// * in case of fail – deactivate session
func (as *activeSession) handleZipFile(update tgbotapi.Update) error {
	as.activate()
	as.setActiveUser(update)
	log.Printf("open session for the user %s\n", update.Message.From.String())
	url, err := as.bot.GetFileDirectURL(update.Message.Document.FileID)
	if err != nil {
		log.Println(err)
		_, err := as.bot.Send(newMessage(update.Message.Chat.ID, doloresMessages.cannotGetDownloadLink))
		if err != nil {
			log.Println("ERROR: ", err)
		}
		as.deactivate()
		return err
	}

	as.setDiagZipPath(update.Message.Document.FileName)

	if !strings.HasSuffix(as.diagZipPath, ".zip") {
		_, err := as.bot.Send(newMessage(update.Message.Chat.ID, doloresMessages.onlyZip))
		if err != nil {
			log.Println("ERROR: ", err)
		}
		log.Printf("user %+v sent not a zip file\n", update.Message.From)
		as.deactivate()
		return fmt.Errorf("not a zip file")
	}

	err = downloadFile(as.diagZipPath, url)
	if err != nil {
		switch {
		case strings.Contains("file is too big", err.Error()):
			_, err := as.bot.Send(newMessage(update.Message.Chat.ID, doloresMessages.tooBigFile))
			if err != nil {
				log.Println("ERROR: ", err)
			}
		default:
			log.Println(err)
			_, err := as.bot.Send(newMessage(update.Message.Chat.ID, doloresMessages.cannotDownload))
			if err != nil {
				log.Println("ERROR: ", err)
			}
		}
		as.deactivate()
		return err
	}
	return nil
}

// parse versions and sql.party.xls from zip
// * if any error – fail and deactivate session
func (as *activeSession) parseVersions(update tgbotapi.Update) error {
	_, err := as.bot.Send(newMessage(update.Message.Chat.ID, doloresMessages.fileDownloaded))
	if err != nil {
		log.Println("ERROR: ", err)
	}

	versions, err := parseZipFile(as.diagZipPath)
	if err != nil {
		log.Println(err)
		_, err := as.bot.Send(newMessage(update.Message.Chat.ID, doloresMessages.cannotParseVersion))
		if err != nil {
			log.Println("ERROR: ", err)
		}
		as.deactivate()
		return err
	}
	as.setVersions(versions)
	as.setCustomer(versions.CustomerName, versions.FactorTagVersion, update.Message.Chat.ID)
	return nil
}

// build the image from parsed versions
func (as *activeSession) buildImage(update tgbotapi.Update) error {
	_, err := as.bot.Send(
		newMessage(
			update.Message.Chat.ID,
			fmt.Sprintf(doloresMessages.startDeploy, as.getVersionsString()),
		),
	)
	if err != nil {
		log.Println("ERROR: ", err)
	}
	tags := []string{as.getCustomer()}
	log.Printf("start to build image %v\n", as.getCustomer())
	dockerfile := "Dockerfile"

	err = as.docker.BuildImage(dockerfile, tags, makeArgs(as.getVersions()), filesToIncludeToContext)
	if err != nil {
		log.Println("ERROR: ", err)
	}
	if err != nil {
		log.Println(err)
		_, err := as.bot.Send(newMessage(update.Message.Chat.ID, doloresMessages.imageFail))
		if err != nil {
			log.Println("ERROR: ", err)
		}
		as.deactivate()
		return err
	}
	_, err = as.bot.Send(newMessage(update.Message.Chat.ID, doloresMessages.imageSuccess))
	if err != nil {
		log.Println("ERROR: ", err)
	}
	return nil
}

// start container
// * if there is a conflict with existing container by name
// try to delete and try one more time
func (as *activeSession) runContainer(update tgbotapi.Update) error {
	err := as.docker.RunContainer(as.getCustomer(), as.getCustomer(), ports, volumeBinds, []string{})
	if err != nil {
		log.Println(err)
		// if conflict try to delete and repeat
		if strings.Contains("is already in use by container", err.Error()) {
			_, err := as.bot.Send(newMessage(update.Message.Chat.ID, doloresMessages.containerExists))
			if err != nil {
				log.Println("ERROR: ", err)
			}
			err = as.docker.StopAndRemoveContainer(as.getCustomer())
			if err != nil {
				log.Println(err)
				_, err = as.bot.Send(newMessage(update.Message.Chat.ID, doloresMessages.containerFail))
				if err != nil {
					log.Println("ERROR: ", err)
				}
				as.deactivate()
			}
			err = as.docker.RunContainer(as.getCustomer(), as.getCustomer(), ports, volumeBinds, []string{})

			if err != nil {
				log.Println(err)
				_, err := as.bot.Send(newMessage(update.Message.Chat.ID, doloresMessages.containerFail))
				as.deactivate()
				return err
			}
		} else {
			log.Println(err)
			_, err := as.bot.Send(newMessage(update.Message.Chat.ID, doloresMessages.containerFail))
			if err != nil {
				log.Println("ERROR: ", err)
			}
			as.deactivate()
			return err
		}
	}
	_, err = as.bot.Send(newMessageWithButton(update.Message.Chat.ID, doloresMessages.containerSuccess, "Удалить контейнер", as.getCustomer()))
	if err != nil {
		log.Println("ERROR: ", err)
	}
	return nil
}

// wait cdi to start:
// * check every 30 second the login page to be alive
// * if container stops – exit
// * if after 10 minutes CDI will not start – exit
func (as *activeSession) waitCDIToStart(update tgbotapi.Update) error {
	iter := 0
	for {
		iter++
		// check if container is running
		ok, err := as.docker.CheckRunningContainer(as.getCustomer())
		if err != nil {
			log.Println(err)
			_, err := as.bot.Send(newMessage(update.Message.Chat.ID, doloresMessages.containerCheckError))
			if err != nil {
				log.Println("ERROR: ", err)
			}
			return err
		}
		if !ok {
			_, err := as.bot.Send(newMessage(update.Message.Chat.ID, doloresMessages.containerIsDead))
			if err != nil {
				log.Println("ERROR: ", err)
			}
			return fmt.Errorf("problem to run the container")
		}
		resp, err := http.Get(fmt.Sprintf("http://localhost:%v/cdi/ui", cdiPort))
		if err == nil {
			if resp.StatusCode == 200 {
				_, err := as.bot.Send(newMessage(update.Message.Chat.ID, doloresMessages.cdiAlive))
				if err != nil {
					log.Println("ERROR: ", err)
				}
				break
			} else {
				log.Println(resp.StatusCode)
			}
		} else {
			log.Println(err)
		}
		if iter%6 == 0 {
			_, err := as.bot.Send(newMessage(update.Message.Chat.ID, doloresMessages.cdiStartingWait))
			if err != nil {
				log.Println("ERROR: ", err)
			}
		}
		if iter == 30 {
			// err := as.docker.stopAndRemoveContainer(as.getCustomer())
			_, err := as.bot.Send(newMessage(update.Message.Chat.ID, doloresMessages.cdiTimeout))
			if err != nil {
				log.Println("ERROR: ", err)
			}
			if err != nil {
				log.Println(err)
			}
			// as.deactivate()
			return fmt.Errorf("timed out")
		}
		time.Sleep(30 * time.Second)
	}
	return nil
}

// run task chain and return error if any fails
func (as *activeSession) runTasks(update tgbotapi.Update) error {
	for _, task := range taskChain {
		status, ok := as.cdi.runTaskAndWait(task.taskName, task.taskParams)
		if !ok {
			log.Println(status)
			_, err := as.bot.Send(
				newMessageWithButton(update.Message.Chat.ID,
					fmt.Sprintf(doloresMessages.taskFailed, serverIP, cdiPort, status), "Удалить контейнер", as.getCustomer()))
			if err != nil {
				log.Println("ERROR: ", err)
			}
			return fmt.Errorf("error in running task: %v", status)
		}
		_, err := as.bot.Send(newMessage(update.Message.Chat.ID, fmt.Sprintf("%s: %s", task.message, status)))
		if err != nil {
			log.Println("ERROR: ", err)
		}

	}
	return nil
}

func (as *activeSession) notifyForDelete() {
	for {
		time.Sleep(2 * time.Hour)
		if !as.isActive(0) {
			return
		}
		_, err := as.bot.Send(newMessageWithButton(as.user.id, doloresMessages.notifyForDelete, "Удалить контейнер", as.getCustomer()))
		if err != nil {
			log.Println("ERROR: ", err)
		}
	}
}

// main message handler
func (as *activeSession) handleMessage(update tgbotapi.Update) {
	// if it is callback to stop container
	if update.CallbackQuery != nil {
		as.handleCallbackQuery(update)
		return
	}

	// skip if no message
	if update.Message == nil {
		return
	}

	if update.Message.Text == "дай мне суперсилу" {
		_, err := as.bot.Send(newMessageWithButton(
			update.Message.Chat.ID,
			"Держи и повелевай мной",
			"Передать очередь дальше",
			"cleanUp",
		))
		if err != nil {
			log.Println("ERROR: ", err)
		}
		return
	}

	// download file
	if as.isActive(update.Message.Chat.ID) {
		as.handleBusy(update)
		return
	} else {
		// cleanup running containers
		err := as.docker.KillRunningContainers(as.getCustomer())
		if err != nil {
			_, err := as.bot.Send(newMessage(update.Message.Chat.ID, doloresMessages.busyWrong))
			if err != nil {
				log.Println("ERROR: ", err)
			}
			log.Printf("cannot stop running containers: %v\n", err)
			return
		}
	}

	// skip if no document
	if update.Message.Document == nil {
		_, err := as.bot.Send(newMessage(update.Message.Chat.ID, doloresMessages.hello))
		if err != nil {
			log.Println("ERROR: ", err)
		}
		log.Printf("user %s sent not a document\n", update.Message.From.String())
		return
	}

	// handle zip file
	err := as.handleZipFile(update)
	if err != nil {
		return
	}

	// parse versions and sql.party.xls
	err = as.parseVersions(update)
	if err != nil {
		return
	}

	// build image
	err = as.buildImage(update)
	if err != nil {
		return
	}

	// run container
	err = as.runContainer(update)
	if err != nil {
		return
	}

	// wait to start cdi
	err = as.waitCDIToStart(update)
	if err != nil {
		return
	}

	// run tasks
	err = as.runTasks(update)
	if err != nil {
		return
	}

	// final
	_, err = as.bot.Send(
		newMessageWithButton(update.Message.Chat.ID,
			fmt.Sprintf(doloresMessages.allDone, serverIP, cdiPort), "Удалить контейнер", as.getCustomer()))
	if err != nil {
		log.Println("ERROR: ", err)
	}

	go as.notifyForDelete()
}
