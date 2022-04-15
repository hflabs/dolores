package main

import (
	"io"
	"log"
	"net/http"
	"os"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// Стандартный метод для написания телеграм-ботом сообщения
func newMessage(chatID int64, message string) tgbotapi.MessageConfig {
	return tgbotapi.NewMessage(chatID, message)
}

// Стандартный метод для написания телеграм-ботом сообщения, в котором есть некая кнопка. Дальше мы на каждой кнопке укажем её текст (text) и действие, которое надо выполнить при нажатии (action)
func newMessageWithButton(chatID int64, message, text, action string) tgbotapi.MessageConfig {
	msg := tgbotapi.NewMessage(chatID, message)
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				text,
				action,
			),
		),
	)
	return msg
}

// Метод для скачивания файла. Мы же открываем чат с Долорес и кидаем ей некий файл. Она должна его скачать
func downloadFile(filepath string, url string) error {
	// Get the data
	resp, err := http.Get(url) //nolint:gosec
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}

// Тут показываем скруктуру переменной taskToRun. Это будет массив задач, в котором мы передаем имя задачи, её параметры и сообщение бота
type taskToRun struct {
	taskName   string
	taskParams []*TaskParam
	message    string
}

// Создаем переменные и логируем их, полезно при отладке
func makeArgs(versions *applicationVersions) map[string]string {
	log.Printf("ARGS: %+v\n", versions)
	return map[string]string{
		"CUSTOMER_NAME":       versions.CustomerName,
		"CORE_REVISION":       versions.CoreRevision,
		"CUSTOMER_REVISION":   versions.CustomerRevision,
		"JDBC_USERNAME":       schemaName,
		"FACTOR_BUILD_FILTER": versions.FactorTagVersion,
	}
}

func convertMapToDockerArgs(in map[string]string) map[string]*string {
	out := make(map[string]*string)
	for key, value := range in {
		tmp := value
		out[key] = &tmp
	}
	return out
}
