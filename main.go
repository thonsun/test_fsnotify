package main

import (
	"errors"
	"fmt"
	"github.com/test_fsnotigy/config"
	"github.com/test_fsnotigy/log"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"github.com/fsnotify/fsnotify"
	"time"
)
var(
	Watcher *fsnotify.Watcher
	err error
)

func main() {
	log.SetLogLevel(log.DEBUG)
	Watcher, err = fsnotify.NewWatcher()
	if err != nil {
		log.Error("fsnotify new Watcher error: %v",err)
		return
	}
	defer Watcher.Close()

}

// 根据uid 获取user
func getFileUser(path string) (string, error) {
	uidStr := fmt.Sprintf("%d",GetFileUID(path))
	dat, err := ioutil.ReadFile("/etc/passwd")
	if err != nil {
		return "", err
	}
	userList := strings.Split(string(dat), "\n")
	for _, info := range userList[0 : len(userList)-1] {// 去掉最后一个\n 空元素
		// fmt.Println(info)
		s := strings.SplitN(info, ":", -1)
		if len(s) >= 3 && s[2] == uidStr {
			// fmt.Println(s[0])
			return s[0], nil
		}
	}
	return "", errors.New("error get fileOwner")
}
func GetFileUID(filename string) (uid uint64) {
	fileinfo, err := os.Stat(filename)
	if err != nil {
		log.Debug("get fileinfo error:%#v",err)
	}
	uid_num := reflect.ValueOf(fileinfo.Sys()).Elem().FieldByName("Uid").Uint()
	return uid_num
}

func AddWatcher() {
	var pathList []string
	for _,path := range config.MonitorPath{
		if path == "%web%"{
			//TODO:web 目录的监控
		}
		// 找出要监控的目录:宿主主机
		if strings.HasPrefix(path,"/"){
			pathList = append(pathList,path)
			if strings.HasSuffix(path,"*"){
				iterationWatcher([]string{strings.Replace(path,"*","",1)}, Watcher,pathList)
			}else {
				Watcher.Add(path) // 以文件夹为监控watcher
				log.Debug("add new wather: [%v]",strings.ToLower(path))
			}
		}else {
			log.Debug("error file monitor config! %v",path)
		}
	}
}

func AddDockerWatch() {
	ticker := time.NewTicker(time.Second * 10)
	status := 0
	go func() {
		docker := map[string]int{}
		for _ = range ticker.C{
			// 看是否有 新启动docker 与 容器的退出 => 缓存表
			// docker 的copy-on-write
			// /var/lib/docker/overlay2/f553c1fceb7ba14cc8cda6e7f4aba27493c11f3c34cfa05d44ce5c70d97233d8(包含init)/diff
			status = (status + 1)%100  // 当前存活的主机
			dirs, err := ioutil.ReadDir("/var/lib/docker/overlay2/")
			if err != nil {
				log.Error("open docker overlay2 error:%v",err)
			}
			// 遍历查找新启动docker 加入watch  与 更新存活 docker status
			for _, dir := range dirs {
				if dir.IsDir() {
					dirname := dir.Name()
					if ok := strings.Contains(dirname,"-init");ok{
						dockerlayer := strings.Split(dirname,"-")[0]
						if _,ok := docker[dockerlayer];ok {
							docker[dockerlayer] = status // 更新存活状态
						}else {
							// 新启动docker,加入watcher
							// TODO：在monitro path 前加上docker diff层
							var pathList []string
							for _,path := range config.MonitorPath{
								if path == "%web%"{
									//TODO:web 目录的监控
								}
								// 找出要监控的目录:宿主主机
								if strings.HasPrefix(path,"/"){
									pathList = append(pathList,path)
									if strings.HasSuffix(path,"*"){
										iterationWatcher([]string{strings.Replace(path,"*","",1)}, Watcher,pathList)
									}else {
										docker_path := fmt.Sprintf("/var/lib/docker/overlay2/%v/merged%v",dockerlayer,path)
										Watcher.Add(docker_path) // 以文件夹为监控watcher
										log.Debug("add new wather: [%v]",strings.ToLower(docker_path))
									}
								}
							}
						}
					}
				}
			}
			for dockerlayer,s := range docker{
				if s != status {
					// 不存在的docker 容器，删除watch
					var pathList []string
					for _,path := range config.MonitorPath{
						if path == "%web%"{
							//TODO:web 目录的监控
						}
						// 找出要监控的目录:宿主主机
						if strings.HasPrefix(path,"/"){
							pathList = append(pathList,path)
							if strings.HasSuffix(path,"*"){
								iterationWatcher([]string{strings.Replace(path,"*","",1)}, Watcher,pathList)
							}else {
								docker_path := fmt.Sprintf("/var/lib/docker/overlay2/%v/merged%v",dockerlayer,path)
								Watcher.Remove(docker_path) // 以文件夹为监控watcher
								log.Debug("remove wather: [%v]",strings.ToLower(docker_path))
							}
						}
					}
				}
			}
		}
	}()
}

func StartFileMonitor(resultChan chan map[string]string) {
	log.Debug("%s","Start File Monitor and config the Watcher once...")

	var resultdata map[string]string
	for {
		select {
		case event := <-Watcher.Events:
			resultdata = make(map[string]string)
			if len(event.Name) == 0{
				continue
			}
			resultdata["source"] = "file"
			resultdata["action"] = event.Op.String()
			resultdata["path"] = event.Name
			resultdata["user"] = ""

			f, err := os.Stat(event.Name)
			if err == nil && !f.IsDir(){
				if user, err := getFileUser(event.Name);err == nil {
					resultdata["user"] = user
				}
			}

			resultChan <- resultdata
			log.Debug("[+]Watcher new event: %v",resultdata)
		case err := <- Watcher.Errors:
			log.Error("error: %v", err)
		}
	}
}

func iterationWatcher(monList []string, watcher *fsnotify.Watcher, pathList []string)  {
	for _,p := range monList{
		filepath.Walk(p, func(path string, f os.FileInfo, err error) error {
			if err != nil {
				log.Error("file walk error: %v",err)
				return err
			}
			if f.IsDir(){
				pathList = append(pathList,path)
				err = watcher.Add(strings.ToLower(path))
				log.Debug("add new wather: %v",strings.ToLower(path))
				if err != nil{
					log.Error("add file watcher error: %v %v",err,path)
				}
			}
			return err
		})
	}
}

