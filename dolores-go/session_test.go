package main

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

// Автотесты на класс session_test.go
// Проверяет, как система ставит в очередь и деактивирует сессию

type testBotSender struct{}

func (tbs *testBotSender) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	fmt.Printf("Message to send: %#v\n", c)
	return tgbotapi.Message{}, nil
}
func (tbs *testBotSender) GetFileDirectURL(fileID string) (string, error) { return "", nil }

var testBot = &testBotSender{}

type testDockerRunner struct{}

func (tdr *testDockerRunner) BuildImage(dockerfile string, tags []string, args map[string]string, includeToContext []string) error {
	return nil
}
func (tdr *testDockerRunner) RunContainer(imageName string, containerName string, portsToExpose []string, volumeBinds []string, inputEnv []string) error {
	return nil
}
func (tdr *testDockerRunner) StopAndRemoveContainer(containername string) error { return nil }
func (tdr *testDockerRunner) CheckRunningContainer(containerName string) (bool, error) {
	return false, nil
}
func (tdr *testDockerRunner) KillRunningContainers(containerNameToDelete string) error { return nil }

var testDocker = &testDockerRunner{}

type fields struct {
	user   *telegramUser
	status sessionStatus
	q      *sessionsQueue
}

func newASFromFields(fields fields) *activeSession {
	return &activeSession{
		user:                 fields.user,
		status:               fields.status,
		bot:                  testBot,
		docker:               testDocker,
		q:                    fields.q,
		waitInPendingSeconds: 1,
	}
}

func Test_activeSession_handleCallbackQuery(t *testing.T) {

	tests := []struct {
		name   string
		before fields
		update tgbotapi.Update
		after  fields
	}{
		{
			name: "takePlace",
			before: fields{
				user:   newTelegramUser("1", 1),
				status: ACTIVE,
				q: &sessionsQueue{queue: []*telegramUser{
					{username: "2", id: 2},
					{username: "3", id: 3},
				}},
			},
			update: tgbotapi.Update{
				CallbackQuery: &tgbotapi.CallbackQuery{
					Data: "takePlace",
					From: &tgbotapi.User{
						UserName: "4",
						ID:       4,
					},
				},
			},
			after: fields{
				user:   newTelegramUser("1", 1),
				status: ACTIVE,
				q: &sessionsQueue{queue: []*telegramUser{
					{username: "2", id: 2},
					{username: "3", id: 3},
					{username: "4", id: 4},
				}},
			},
		},
		{
			name: "already in queue",
			before: fields{
				user:   newTelegramUser("1", 1),
				status: ACTIVE,
				q: &sessionsQueue{queue: []*telegramUser{
					{username: "2", id: 2},
					{username: "3", id: 3},
				}},
			},
			update: tgbotapi.Update{
				CallbackQuery: &tgbotapi.CallbackQuery{
					Data: "takePlace",
					From: &tgbotapi.User{
						UserName: "2",
						ID:       2,
					},
				},
			},
			after: fields{
				user:   newTelegramUser("1", 1),
				status: ACTIVE,
				q: &sessionsQueue{queue: []*telegramUser{
					{username: "2", id: 2},
					{username: "3", id: 3},
				}},
			},
		},
		{
			name: "exit queue",
			before: fields{
				user:   newTelegramUser("1", 1),
				status: ACTIVE,
				q: &sessionsQueue{queue: []*telegramUser{
					{username: "2", id: 2},
					{username: "3", id: 3},
				}},
			},
			update: tgbotapi.Update{
				CallbackQuery: &tgbotapi.CallbackQuery{
					Data: "exitQueue",
					From: &tgbotapi.User{
						UserName: "2",
						ID:       2,
					},
				},
			},
			after: fields{
				user:   newTelegramUser("1", 1),
				status: ACTIVE,
				q: &sessionsQueue{queue: []*telegramUser{
					{username: "3", id: 3},
				}},
			},
		},
		{
			name: "deactivate",
			before: fields{
				user:   newTelegramUser("1", 1),
				status: ACTIVE,
				q: &sessionsQueue{queue: []*telegramUser{
					{username: "2", id: 2},
					{username: "3", id: 3},
				}},
			},
			update: tgbotapi.Update{
				CallbackQuery: &tgbotapi.CallbackQuery{
					Data: "deactivate",
					From: &tgbotapi.User{
						UserName: "1",
						ID:       1,
					},
				},
			},
			after: fields{
				user:   nil,
				status: DISACTIVE,
				q:      &sessionsQueue{queue: []*telegramUser{}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			as := newASFromFields(tt.before)
			as.handleCallbackQuery(tt.update)
			want := newASFromFields(tt.after)
			if tt.name == "deactivate" {
				time.Sleep(3 * time.Second)
			}
			if !reflect.DeepEqual(as, want) {
				t.Errorf("as.CallbackHandler() = %v, want %v", as, want)
			}

		})
	}
}
