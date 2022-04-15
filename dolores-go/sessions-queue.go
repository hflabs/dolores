package main

import (
	"fmt"
	"sync"
)

type sessionsQueue struct {
	sync.Mutex
	queue []*telegramUser
}

func newSessionsQueue() *sessionsQueue {
	return &sessionsQueue{queue: make([]*telegramUser, 0)}
}

func (q *sessionsQueue) getNext() *telegramUser {
	if len(q.queue) == 0 {
		return nil
	}
	nextUser := q.queue[0]
	q.queue = q.queue[1:]
	return nextUser
}

func (q *sessionsQueue) takePlace(user *telegramUser) (int, bool) {
	q.Lock()
	defer q.Unlock()
	if len(q.queue) == 0 {
		q.queue = append(q.queue, user)
		return 1, false
	}
	for userPlace, userInQueue := range q.queue {
		if userInQueue.username == user.username {
			return userPlace + 1, true
		}
	}
	q.queue = append(q.queue, user)
	return len(q.queue), false
}

func (q *sessionsQueue) exit(user *telegramUser) error {
	q.Lock()
	defer q.Unlock()
	for userPlace, userInQueue := range q.queue {
		if userInQueue.username == user.username {
			q.queue = append(q.queue[:userPlace], q.queue[userPlace+1:]...)
			return nil
		}
	}
	return fmt.Errorf("user is not in queue")
}
