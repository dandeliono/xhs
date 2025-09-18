package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"image/png"
	"os"
	"strings"
	"time"

	"github.com/dandeliono/xhs/browser"
	"github.com/dandeliono/xhs/cookies"
	"github.com/dandeliono/xhs/xiaohongshu"
	"github.com/go-rod/rod"
	"github.com/liyue201/goqr"
	qrterminal "github.com/mdp/qrterminal/v3"
	"github.com/sirupsen/logrus"
)

func main() {
	var (
		binPath string // 浏览器二进制文件路径
	)
	flag.StringVar(&binPath, "bin", "", "浏览器二进制文件路径")
	flag.Parse()

	b := browser.NewBrowser(true, browser.WithBinPath(binPath))
	defer b.Close()

	page := b.NewPage()
	defer page.Close()

	action := xiaohongshu.NewLogin(page)

	status, err := action.CheckLoginStatus(context.Background())
	if err != nil {
		logrus.Fatalf("failed to check login status: %v", err)
	}

	logrus.Infof("当前登录状态: %v", status)

	if status {
		return
	}

	logrus.Info("检测到未登录，将通过二维码完成登录流程")

	if err := loginWithQRCode(context.Background(), action, page); err != nil {
		logrus.Fatalf("登录失败: %v", err)
	}

	logrus.Info("登录成功！")
}

func saveCookies(page *rod.Page) error {
	cks, err := page.Browser().GetCookies()
	if err != nil {
		return err
	}

	data, err := json.Marshal(cks)
	if err != nil {
		return err
	}

	cookieLoader := cookies.NewLoadCookie(cookies.GetCookiesFilePath())
	return cookieLoader.SaveCookies(data)
}

func loginWithQRCode(ctx context.Context, action *xiaohongshu.LoginAction, page *rod.Page) error {
	const (
		maxAttempts    = 5
		waitPerAttempt = 90 * time.Second
	)

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		logrus.Infof("获取登录二维码（第 %d/%d 次）", attempt, maxAttempts)

		imageData, err := action.FetchLoginQRCode(ctx)
		if err != nil {
			return fmt.Errorf("获取二维码失败: %w", err)
		}

		payload, err := decodeQRCodePayload(imageData)
		if err != nil {
			return fmt.Errorf("解析二维码内容失败: %w", err)
		}

		fmt.Printf("\n请使用小红书 App 扫描下方二维码完成登录（尝试 %d/%d）：\n\n", attempt, maxAttempts)
		renderQRCode(payload)
		fmt.Printf("\n二维码链接: %s\n\n", payload)

		logrus.Info("等待扫码登录确认...")
		success, err := action.WaitForLogin(ctx, waitPerAttempt)
		if err != nil {
			return fmt.Errorf("等待登录结果失败: %w", err)
		}

		if success {
			if err := saveCookies(page); err != nil {
				return fmt.Errorf("保存 Cookies 失败: %w", err)
			}
			return nil
		}

		logrus.Warn("二维码可能已过期或尚未确认，将重新获取新的二维码...")
	}

	return fmt.Errorf("登录超时，未能通过二维码完成登录")
}

func decodeQRCodePayload(raw string) (string, error) {
	if raw == "" {
		return "", fmt.Errorf("二维码数据为空")
	}

	idx := strings.Index(raw, ",")
	if idx >= 0 {
		raw = raw[idx+1:]
	}

	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return "", fmt.Errorf("二维码 base64 解码失败: %w", err)
	}

	img, err := png.Decode(bytes.NewReader(decoded))
	if err != nil {
		return "", fmt.Errorf("二维码图片解码失败: %w", err)
	}

	codes, err := goqr.Recognize(img)
	if err != nil {
		return "", fmt.Errorf("二维码识别失败: %w", err)
	}

	if len(codes) == 0 {
		return "", fmt.Errorf("未能识别到二维码内容")
	}

	return string(codes[0].Payload), nil
}

func renderQRCode(content string) {
	config := qrterminal.Config{
		Level:     qrterminal.M,
		Writer:    os.Stdout,
		QuietZone: 1,
	}

	qrterminal.GenerateWithConfig(content, config)
}
