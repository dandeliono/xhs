package main

import (
	"flag"

	"github.com/dandeliono/xhs/configs"
	"github.com/sirupsen/logrus"
)

func main() {
	var (
		headless bool
		binPath  string // 浏览器二进制文件路径
	)
	flag.BoolVar(&headless, "headless", true, "是否无头模式")
	flag.StringVar(&binPath, "bin", "", "浏览器二进制文件路径")
	flag.Parse()

	configs.InitHeadless(headless)
	configs.SetBinPath(binPath)

	// 初始化服务
	xiaohongshuService := NewXiaohongshuService()

	// 创建并启动应用服务器
	appServer := NewAppServer(xiaohongshuService)
	if err := appServer.Start(":18060"); err != nil {
		logrus.Fatalf("failed to run server: %v", err)
	}
}
