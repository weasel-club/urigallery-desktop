package main

import (
	_ "embed"
	"log"
	"os"
	"time"
	"urigallery/app"
	"urigallery/peer"
	"urigallery/signal"
	"urigallery/ui"
	"urigallery/util"
)

func selectDirectory(u *ui.UI) string {
	picturesDir, err := u.OpenDirectory("Select VRChat Pictures Directory", "")
	if err != nil {
		u.ShowMessage("Error", err.Error())
		os.Exit(1)
	}
	return picturesDir
}

func getPicturesDir(u *ui.UI) string {
	picturesDir, err := util.GetPicturesDir()
	if err != nil {
		return selectDirectory(u)
	}
	return picturesDir
}

func ensurePicturesDir(u *ui.UI, dir string) string {
	if dir == "" {
		dir = getPicturesDir(u)
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		dir = selectDirectory(u)
		return ensurePicturesDir(u, dir)
	}

	return dir
}

func saveSettings(settings Settings) {
	if err := settings.Save(); err != nil {
		log.Fatal(err)
	}
}

func main() {
	settings := LoadOrDefaultSettings(util.GetPath(".settings.toml"))
	u := ui.New()

	picturesDir := ensurePicturesDir(u, settings.PicturesDir)
	settings.PicturesDir = picturesDir
	saveSettings(settings)

	// Login
	signalClient := signal.NewClient()
	signalClient.SetToken(settings.Token)
	token, err := signalClient.Login()
	if err != nil {
		log.Fatal(err)
	}
	settings.Token = token
	saveSettings(settings)

	log.Printf("Logged in as %s", signalClient.UID())
	go u.Start(func(event ui.Event) {
		switch event.(type) {
		case *ui.EventGetOTP:
			otp, err := signalClient.CreateOTP()
			if err != nil {
				log.Fatal(err)
			}
			u.WriteClipboard(otp)
			u.ShowMessage("OTP", "OTP가 클립보드에 복사되었습니다.")
		case *ui.EventQuit:
			signalClient.CloseSession()
			os.Exit(0)
		}
	})

	a := app.NewApp()
	a.AddHandler("listImages", func(r *app.Request) (*app.Response, error) {
		return app.ListImages(r, picturesDir)
	})
	a.AddHandler("downloadImage", func(r *app.Request) (*app.Response, error) {
		return app.DownloadImage(r, picturesDir)
	})

	channelCount := 0
	sleepTime := 1 * time.Second

	for {
		err = signalClient.OpenSession(func(channel *peer.Channel) {
			channelCount++
			log.Printf("Channel %s is opened (total: %d)", channel.ID, channelCount)
			channel.OnMessage(func(message *peer.Message) {
				a.HandleMessage(channel, message)
			})
			channel.OnClose(func() {
				channelCount--
				log.Printf("Channel %s is closed (total: %d)", channel.ID, channelCount)
			})
		})

		if err != nil {
			log.Printf("Error in session: %s, retrying in %s", err, sleepTime)
			time.Sleep(sleepTime)
			sleepTime *= 2
			if sleepTime > 15*time.Second {
				sleepTime = 15 * time.Second
			}
		}
	}
}
