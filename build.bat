:xx
@echo off
rem go get -u github.com/jteeuwen/go-bindata/...
go-bindata -nomemcopy -o assets.go assets
go fmt
cls
rem go run paktool.go assets.go -c=unpack -f=resources.pak
go run paktool.go assets.go -c=repack -f=resources.json
rem go run paktool.go assets.go -c=lang_unpack -f=zh-CN.pak
rem go run paktool.go assets.go -c=lang_repack -f=zh-CN.json
pause
rem goto xx

echo build windows 386
set GOOS=windows
set GOARCH=386
go build -ldflags "-s -w" -o release/windows/386/pak_tools.exe

echo build windows amd64
set GOOS=windows
set GOARCH=amd64
go build -ldflags "-s -w" -o release/windows/amd64/pak_tools.exe

echo build linux 386
set GOOS=linux
set GOARCH=386
go build -ldflags "-s -w" -o release/linux/386/pak_tools

echo build linux amd64
set GOOS=linux
set GOARCH=amd64
go build -ldflags "-s -w" -o release/linux/amd64/pak_tools


echo build darwin amd64
set GOOS=darwin
set GOARCH=amd64
go build -ldflags "-s -w" -o release/darwin/amd64/pak_tools

pause
goto xx