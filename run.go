package main

import (
	log "github.com/sirupsen/logrus"
	"mydocker/cgroups"
	"mydocker/cgroups/subsystems"
	"mydocker/container"
	"mydocker/network"
	"os"
	"strings"
)

func Run(tty bool, commandArray []string, res *subsystems.ResourceConfig, volume string, containerName string,
	imageName string, envSlice []string, nw string, portMapping []string) {

	if containerName == "" {
		containerName = container.RandStringBytes(6)
	}

	// 创建父进程以及管道写入句柄
	parent, writePipe := container.NewParentProcess(tty, volume, containerName, imageName, envSlice)

	if err := parent.Start(); err != nil {
		log.Error(err)
	}

	containerInfo, err := container.RecordContainerInfo(parent.Process.Pid, commandArray, containerName,
		volume, portMapping)
	if err != nil {
		log.Errorf("Record container info error %v", err)
		return
	}

	if nw != "" {
		// config container network
		network.Init()
		if err := network.Connect(nw, containerInfo); err != nil {
			// this is where is wrong
			log.Errorf("error connect to network: %v", err)
			return
		}
	}

	cGroupManager := cgroups.NewCGroupManager("mydocker-cgroup")
	defer cGroupManager.Destroy()

	if err := cGroupManager.Set(res); err != nil {
		log.Errorf("cGroup %v set error.", cGroupManager.Path)
	}

	if err := cGroupManager.Apply(parent.Process.Pid); err != nil {
		log.Errorf("cGroup %v set error.", cGroupManager.Path)
	}

	sendInitCommand(commandArray, writePipe)

	if tty {
		if err := parent.Wait(); err != nil {
			log.Fatalf("Process wait error: ", err)
		}
		container.DeleteWorkSpace(containerName, volume)
	}
}

func sendInitCommand(commandArray []string, writePipe *os.File) {
	command := strings.Join(commandArray, " ")
	log.Info("command all is:", command)
	// 通过管道写入句柄，写入执行命令到管道，等待子进程从管道中读取父进程传入的命令
	if _, err := writePipe.WriteString(command); err != nil {
		log.Fatal("pipe write error:", err)
	}
	if err := writePipe.Close(); err != nil {
		log.Fatal("close pipe error:", err)
	}
}
