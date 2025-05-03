package ui

import (
	_ "embed"
	"log"

	"github.com/getlantern/systray"
	"github.com/sqweek/dialog"
	"golang.design/x/clipboard"
)

//go:embed icon.ico
var icon []byte

type EventHandler func(event Event)

type UI struct{}

type Event any

type EventQuit struct{}

type EventGetOTP struct{}

func New() *UI {
	if err := clipboard.Init(); err != nil {
		log.Fatal(err)
	}
	return &UI{}
}

func (u *UI) ShowMessage(title string, message string) {
	dialog.Message("%s", message).Title(title).Info()
}

func (u *UI) OpenDirectory(title string, startDir string) (string, error) {
	return dialog.Directory().SetStartDir(startDir).Title(title).Browse()
}

func (u *UI) WriteClipboard(text string) {
	clipboard.Write(clipboard.FmtText, []byte(text))
}

func (u *UI) Start(eventHandler EventHandler) {
	systray.Run(func() {
		systray.SetIcon(icon)
		systray.SetTooltip("UriGallery Desktop")
		otp := systray.AddMenuItem("OTP 복사", "OTP 복사")
		quit := systray.AddMenuItem("종료", "종료")
		for {
			select {
			case <-otp.ClickedCh:
				eventHandler(&EventGetOTP{})
			case <-quit.ClickedCh:
				eventHandler(&EventQuit{})
			}
		}
	}, nil)
}
