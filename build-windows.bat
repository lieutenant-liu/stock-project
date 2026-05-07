@echo off
echo === Building Frontend ===
cd stock-frontend
call npm ci
call npm run build
xcopy /E /Y dist\* ..\stock-backend\web\
cd ..

echo === Building Go Binary ===
cd stock-backend
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0
go build -ldflags="-s -w" -o stock-terminal.exe .
cd ..

echo === Done! ===
echo Output: stock-backend\stock-terminal.exe
pause
