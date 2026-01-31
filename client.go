package main

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	go_crypto "github.com/pudongping/go-crypto"
)

// --- 逻辑函数 ---

func generateDailyKey(seed string) string {
	today := time.Now().Format("20060102")
	combined := seed + today
	hash := sha256.Sum256([]byte(combined))
	keyHex := hex.EncodeToString(hash[:])
	if len(keyHex) >= 32 {
		return keyHex[:32]
	}
	md5Hash := md5.Sum([]byte(keyHex))
	return keyHex[:24] + hex.EncodeToString(md5Hash[:])[:8]
}

var seed = "my-secret-seed-2024"
var key = generateDailyKey(seed)

func init() {
	fontPath := "./msyh.ttf"
	if _, err := os.Stat(fontPath); err == nil {
		os.Setenv("FYNE_FONT", fontPath)
	}
}

// --- 界面入口 ---

func main() {
	myApp := app.New()
	myApp.Settings().SetTheme(theme.LightTheme())
	myWindow := myApp.NewWindow("AES 加密聊天终端")
	myWindow.Resize(fyne.NewSize(600, 500))

	showConnectScreen(myWindow)

	myWindow.ShowAndRun()
}

// 1. 连接界面 (已修复布局宽度和语法错误)
func showConnectScreen(win fyne.Window) {
	ipInput := widget.NewEntry()
	ipInput.SetText("127.0.0.1")

	portInput := widget.NewEntry()
	portInput.SetText("8000")

	form := &widget.Form{
		Items: []*widget.FormItem{
			{Text: "服务器 IP", Widget: ipInput},
			{Text: "端口号", Widget: portInput},
		},
		SubmitText: "连接服务器",
		OnSubmit: func() {
			address := ipInput.Text + ":" + portInput.Text
			conn, err := net.DialTimeout("tcp", address, 5*time.Second)
			if err != nil {
				dialog.ShowError(fmt.Errorf("连接失败: %v", err), win)
				return
			}
			showChatScreen(win, conn)
		},
	}

	formContainer := container.NewVBox(
		widget.NewLabelWithStyle("AES 加密通讯登录", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		form,
	)

	// 这里强制给容器 450x200 的大小，解决“太短”的问题
	centeredBox := container.NewCenter(
		container.NewGridWrap(fyne.NewSize(450, 200), formContainer),
	)

	win.SetContent(centeredBox)
}

// 2. 聊天界面
func showChatScreen(win fyne.Window, conn net.Conn) {
	richText := widget.NewRichText()
	richText.Wrapping = fyne.TextWrapWord
	scroll := container.NewVScroll(richText)

	statusLabel := widget.NewLabel("状态: 已连接至 " + conn.RemoteAddr().String())

	appendLog := func(msg string) {
		timestamp := time.Now().Format("15:04:05")
		newText := fmt.Sprintf("[%s] %s", timestamp, msg)
		segment := &widget.TextSegment{
			Text: newText + "\n",
			Style: widget.RichTextStyle{
				ColorName: theme.ColorNameForeground,
			},
		}
		richText.Segments = append(richText.Segments, segment)
		richText.Refresh()
		scroll.ScrollToBottom()
	}

	go func() {
		defer conn.Close()
		buf := make([]byte, 2048)
		for {
			n, err := conn.Read(buf)
			if err != nil {
				appendLog("[系统] 连接断开")
				return
			}
			raw := string(buf[:n])
			idx := strings.Index(raw, ":")
			if idx != -1 {
				sender := raw[:idx]
				cipher := raw[idx+1:]
				plain, err := go_crypto.AESCBCDecrypt(cipher, key)
				if err == nil {
					appendLog(sender + ": " + plain)
				}
			}
		}
	}()

	inputWidget := widget.NewEntry()
	inputWidget.PlaceHolder = "在此输入消息..."

	sendAction := func() {
		text := strings.TrimSpace(inputWidget.Text)
		if text == "" {
			return
		}
		cipher, _ := go_crypto.AESCBCEncrypt(text, key)
		_, err := conn.Write([]byte(cipher))
		if err == nil {
			inputWidget.SetText("")
		} else {
			appendLog("[系统] 发送失败")
		}
	}

	sendBtn := widget.NewButtonWithIcon("发送", theme.MailSendIcon(), sendAction)
	inputWidget.OnSubmitted = func(s string) { sendAction() }

	exitBtn := widget.NewButton("断开", func() {
		conn.Close()
		showConnectScreen(win)
	})

	top := container.NewBorder(nil, nil, nil, exitBtn, statusLabel)
	bottom := container.NewBorder(nil, nil, nil, sendBtn, inputWidget)
	content := container.NewBorder(container.NewVBox(top, widget.NewSeparator()), bottom, nil, nil, scroll)

	win.SetContent(content)
	appendLog("[系统] 连接成功！可以开始聊天了")
}
