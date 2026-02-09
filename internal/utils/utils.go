package utils

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/model-ci/apack/internal/log"
	"github.com/model-ci/apack/internal/types"
)

var validName = regexp.MustCompile(`^[a-z0-9]+(?:[._-][a-z0-9]+)*(?:/[a-z0-9]+(?:[._-][a-z0-9]+)*)*$`)

func ValidName(name string) bool {
	return validName.MatchString(name)
}

func ParseSize(s, unit string) (int, error) {
	sz := strings.TrimRight(s, "gGmMkK")
	if len(sz) == 0 {
		return -1, fmt.Errorf("%q:can't parse as num[gGmMkK]:%w", s, strconv.ErrSyntax)
	}
	amt, err := strconv.ParseUint(sz, 0, 0)
	if err != nil {
		return -1, err
	}
	if len(s) > len(sz) {
		unit = s[len(sz):]
	}
	switch unit {
	case "G", "g":
		return int(amt) << 30, nil
	case "M", "m":
		return int(amt) << 20, nil
	case "K", "k":
		return int(amt) << 10, nil
	case "":
		return int(amt), nil
	}
	return -1, fmt.Errorf("can not parse %q as num[gGmMkK]:%w", s, strconv.ErrSyntax)
}

func SafeGo(fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Logger.Error(fmt.Errorf("goroutine panic: %v\n", r), "goroutine panic")
			}
		}()
		fn()
	}()
}

func SafeJob(job func(), funcName string) func() {
	return func() {
		defer func() {
			if r := recover(); r != nil {
				log.Logger.Error(fmt.Errorf("panic reason: %v", r), "cron task recovered from panic", "funcName", funcName)
			}
		}()
		job()
	}
}

type WaitGroupWrapper struct {
	sync.WaitGroup
}

func (w *WaitGroupWrapper) SafeWrap(cb func(), cbName string) {
	w.Add(1)
	go func() {
		defer func() {
			w.Done() // 确保在 defer 中调用 Done
			if r := recover(); r != nil {
				log.Logger.Error(fmt.Errorf("goroutine panic: %v", r), "goroutine panic", "cbName", cbName)
			}
		}()
		cb()
	}()
}

type SequentialTask struct {
	Name     string
	TaskFunc func(ready chan<- struct{})
}

func (w *WaitGroupWrapper) StartSequentially(tasks []SequentialTask) {
	for _, task := range tasks {
		ready := make(chan struct{})

		w.Add(1)
		go func(t SequentialTask) {
			defer func() {
				w.Done()
				if r := recover(); r != nil {
					log.Logger.Error(fmt.Errorf("goroutine panic: %v", r), "goroutine panic", "taskName", t.Name)
				}
			}()

			t.TaskFunc(ready)
		}(task)

		<-ready
		log.Logger.Info("task initialized", "taskName", task.Name)
	}
}

func WriteError(w http.ResponseWriter, code, message string, statusCode int) {
	w.WriteHeader(statusCode)
	w.Header().Set("Content-Type", "application/json")

	resp := types.Response{Code: code, Message: []byte(message)}
	respBytes, err := resp.MarshalJSON()
	if err != nil {
		panic(err)
	}

	w.Write(respBytes)
}

func WriteStatusJSON(w http.ResponseWriter, hs *types.HealthStatus, statusCode int) {
	w.WriteHeader(statusCode)
	w.Header().Set("Content-Type", "application/json")

	response, err := hs.MarshalJSON()
	if err != nil {
		WriteError(w, "", "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Write(response)
}

func WriteJSON(w http.ResponseWriter, jm json.Marshaler, statusCode int) {
	w.WriteHeader(statusCode)
	w.Header().Set("Content-Type", "application/json")

	response, err := jm.MarshalJSON()
	if err != nil {
		WriteError(w, "", "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Write(response)
}
