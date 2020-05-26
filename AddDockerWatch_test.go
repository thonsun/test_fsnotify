package main

import (
	"github.com/fsnotify/fsnotify"
	"github.com/test_fsnotigy/log"
	"testing"
)

// no such file or directory /etc/ImageMagick-6 阻断迭代

func TestAddDockerWatch(t *testing.T) {
	log.SetLogLevel(log.DEBUG)
	Watcher, err = fsnotify.NewWatcher()
	if err != nil {
		log.Error("fsnotify new Watcher error: %v",err)
		return
	}
	defer Watcher.Close()
	stop := make(chan int)
	AddWatcher()
	AddDockerWatch()
	<-stop
}
