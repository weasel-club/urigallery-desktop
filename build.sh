#!/bin/sh
GOOS=windows GOARCH=amd64 go build -ldflags "-s -w -H windowsgui" -o .out/urigallery.exe
go build -ldflags "-s -w" -o .out/urigallery

# add icon with winres
winres -platform windows -icon icon.ico -o .out/urigallery.exe

# compress with upx
upx .out/urigallery.exe
upx .out/urigallery
