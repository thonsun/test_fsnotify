package main

import "testing"

func TestAddDockerWatch(t *testing.T) {
	stop := make(chan int)
	AddDockerWatch()
	<-stop
}
