package main

import (
	"reflect"
	"testing"
)

func Test_sessionsQueue_getNext(t *testing.T) {
	tests := []struct {
		name       string
		queue      []*telegramUser
		want       *telegramUser
		queueAfter []*telegramUser
	}{
		{
			name: "positive",
			queue: []*telegramUser{
				{username: "1", id: 1},
				{username: "2", id: 2},
			},
			want: &telegramUser{username: "1", id: 1},
			queueAfter: []*telegramUser{
				{username: "2", id: 2},
			},
		},
		{
			name: "one in queue",
			queue: []*telegramUser{
				{username: "1", id: 1},
			},
			want:       &telegramUser{username: "1", id: 1},
			queueAfter: []*telegramUser{},
		},
		{
			name:       "no in queue",
			queue:      []*telegramUser{},
			want:       nil,
			queueAfter: []*telegramUser{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := &sessionsQueue{queue: tt.queue}
			got := q.getNext()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("sessionsQueue.getNext() = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(q.queue, tt.queueAfter) {
				t.Errorf("sessionsQueue.getNext() = %#v, want %#v", q.queue, tt.queueAfter)
			}
		})
	}
}

func Test_sessionsQueue_takePlace(t *testing.T) {
	tests := []struct {
		name      string
		queue     []*telegramUser
		user      *telegramUser
		wantPlace int
		wantExist bool
	}{
		{
			name: "positive",
			queue: []*telegramUser{
				{username: "1", id: 1},
			},
			user:      &telegramUser{username: "2", id: 1},
			wantPlace: 2,
			wantExist: false,
		},
		{
			name: "exist",
			queue: []*telegramUser{
				{username: "1", id: 1},
			},
			user:      &telegramUser{username: "1", id: 1},
			wantPlace: 1,
			wantExist: true,
		},
		{
			name:      "empty",
			queue:     []*telegramUser{},
			user:      &telegramUser{username: "1", id: 1},
			wantPlace: 1,
			wantExist: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := &sessionsQueue{queue: tt.queue}
			gotPlace, gotExist := q.takePlace(tt.user)
			if gotPlace != tt.wantPlace {
				t.Errorf("sessionsQueue.takePlace() got = %v, want %v", gotPlace, tt.wantPlace)
			}
			if gotExist != tt.wantExist {
				t.Errorf("sessionsQueue.takePlace() got1 = %v, want %v", gotExist, tt.wantExist)
			}
		})
	}
}

func Test_sessionsQueue_exit(t *testing.T) {
	tests := []struct {
		name       string
		queue      []*telegramUser
		user       *telegramUser
		wantErr    bool
		queueAfter []*telegramUser
	}{
		{
			name: "positive",
			queue: []*telegramUser{
				{username: "1", id: 1},
			},
			user:       &telegramUser{username: "1", id: 1},
			wantErr:    false,
			queueAfter: []*telegramUser{},
		},
		{
			name:       "empty",
			queue:      []*telegramUser{},
			user:       &telegramUser{username: "1", id: 1},
			wantErr:    true,
			queueAfter: []*telegramUser{},
		},
		{
			name: "not in queue",
			queue: []*telegramUser{
				{username: "1", id: 1},
			},
			user:    &telegramUser{username: "2", id: 1},
			wantErr: true,
			queueAfter: []*telegramUser{
				{username: "1", id: 1},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := &sessionsQueue{queue: tt.queue}
			err := q.exit(tt.user)
			if (err != nil) != tt.wantErr {
				t.Errorf("sessionsQueue.exit() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(q.queue, tt.queueAfter) {
				t.Errorf("sessionsQueue.exit() = %+v, want %v", q.queue, tt.queueAfter)
			}
		})
	}
}
