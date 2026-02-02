package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
	"unicode"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	go_crypto "github.com/pudongping/go-crypto"
)

// --- 逻辑函数 ---

// processUserKey 将用户输入的任意文本转换为 32 字符长度的密钥
// 因为 AES 加密要求密钥长度必须固定（如 16/24/32 字节），不能直接用用户输入的 "123"
func processUserKey(input string) string {
	// 使用 SHA256 将输入变为固定的 64 字符 hex 串
	hash := sha256.Sum256([]byte(input))
	keyHex := hex.EncodeToString(hash[:])
	// 截取前 32 个字符作为密钥 (适配原代码逻辑)
	return keyHex[:32]
}

func init() {
	fontPath := "./msyh.ttf"
	if _, err := os.Stat(fontPath); err == nil {
		os.Setenv("FYNE_FONT", fontPath)
	}
}

func isPrintable(s string) bool {
	for _, r := range s {
		// unicode.IsPrint 会检查字符是否为可打印字符（包括空格、汉字、字母等）
		// 如果包含不可见的二进制乱码，它会返回 false
		if !unicode.IsPrint(r) && !unicode.IsGraphic(r) && r != '\n' && r != '\r' && r != '\t' {
			return false
		}
	}
	return true
}

// --- 界面入口 ---

func main() {
	myApp := app.New()
	myApp.Settings().SetTheme(theme.LightTheme())
	myWindow := myApp.NewWindow("AES 自定义密钥聊天")
	myWindow.Resize(fyne.NewSize(600, 500))

	showConnectScreen(myWindow)

	myWindow.ShowAndRun()
}

// 1. 连接界面 (增加了密钥输入框)
func showConnectScreen(win fyne.Window) {
	ipInput := widget.NewEntry()
	ipInput.SetText("127.0.0.1")

	portInput := widget.NewEntry()
	portInput.SetText("8000")

	// 新增：密钥输入框 (使用 PasswordEntry 隐藏输入内容)
	keyInput := widget.NewPasswordEntry()
	keyInput.PlaceHolder = "请输入约定的加密密码"
	keyInput.SetText("123456") // 默认值方便测试

	form := &widget.Form{
		Items: []*widget.FormItem{
			{Text: "服务器 IP", Widget: ipInput},
			{Text: "端口号", Widget: portInput},
			{Text: "加密密钥", Widget: keyInput}, // 将新输入框加入表单
		},
		SubmitText: "连接并设定密钥",
		OnSubmit: func() {
			if keyInput.Text == "" {
				dialog.ShowError(fmt.Errorf("密钥不能为空"), win)
				return
			}

			// 处理密钥：将用户输入的密码转为 AES 可用的 key
			finalKey := processUserKey(keyInput.Text)

			address := ipInput.Text + ":" + portInput.Text
			conn, err := net.DialTimeout("tcp", address, 5*time.Second)
			if err != nil {
				dialog.ShowError(fmt.Errorf("连接失败: %v", err), win)
				return
			}

			// 将生成的 key 传给聊天界面
			showChatScreen(win, conn, finalKey)
		},
	}

	formContainer := container.NewVBox(
		widget.NewLabelWithStyle("AES 加密通讯登录", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewSeparator(),
		form,
	)

	centeredBox := container.NewCenter(
		container.NewGridWrap(fyne.NewSize(450, 250), formContainer), // 稍微调高一点高度适应新输入框
	)

	win.SetContent(centeredBox)
}

// 2. 聊天界面 (接收 userKey 参数)
func showChatScreen(win fyne.Window, conn net.Conn, userKey string) {
	richText := widget.NewRichText()
	richText.Wrapping = fyne.TextWrapWord
	scroll := container.NewVScroll(richText)

	// 在标题栏显示部分密钥信息（可选，用于核对）
	statusLabel := widget.NewLabel(fmt.Sprintf("已连接 | 密钥指纹: %s...", userKey[:4]))

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
		defer func() {
			if r := recover(); r != nil {
				fmt.Println("捕获到异常，防止了闪退:", r)
			}
			conn.Close()
		}()
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
				// 使用传入的 userKey 进行解密
				plain, err := go_crypto.AESCBCDecrypt(cipher, userKey)
				if err == nil {
					if isPrintable(plain) {
						appendLog(sender + ": " + plain)
					} else {
						appendLog("[警告] 收到一条包含不可打印字符的消息 (可能密钥不匹配)")
					}
				} else {
					// 如果密钥不对，解密会失败，这里提示一下
					appendLog("[警告] 收到一条无法解密的消息 (可能密钥不匹配)")
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
		// 使用传入的 userKey 进行加密
		cipher, _ := go_crypto.AESCBCEncrypt(text, userKey)
		_, err := conn.Write([]byte(cipher))
		if err == nil {
			inputWidget.SetText("")
			// 本地回显（为了体验更好，可选）
			// appendLog("我: " + text)
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
	appendLog("[系统] 连接成功！双方必须使用相同密码才能正常通信。")
}
