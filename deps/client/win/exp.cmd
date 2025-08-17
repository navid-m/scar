@echo off
setlocal

where scoop >nul 2>&1
if %errorlevel% neq 0 (
    powershell -Command "Set-ExecutionPolicy RemoteSigned -Scope CurrentUser -Force"
    powershell -Command "$scoopInstaller = Invoke-WebRequest -Uri 'https://get.scoop.sh' -UseBasicParsing; Invoke-Expression $scoopInstaller.Content"
)

if not defined SCOOP set "SCOOP=%USERPROFILE%\scoop"

if not exist "%SCOOP%\apps\msys2" (
    scoop install msys2
)

set "MSYS2_SHELL=%SCOOP%\apps\msys2\current\usr\bin\bash.exe"

"%MSYS2_SHELL%" -lc "pacman --noconfirm -Syuu"
"%MSYS2_SHELL%" -lc "pacman --noconfirm -S mingw-w64-x86_64-toolchain"
"%MSYS2_SHELL%" -lc "pacman --noconfirm -S mingw-w64-x86_64-curl mingw-w64-x86_64-pcre mingw-w64-x86_64-jansson"

set "MINGW64_BIN=%SCOOP%\apps\msys2\current\mingw64\bin"

echo %PATH% | find /i "%MINGW64_BIN%" >nul
if %errorlevel% neq 0 (
    setx PATH "%PATH%;%MINGW64_BIN%"
    set "PATH=%PATH%;%MINGW64_BIN%"
)

echo GCC should now be available:
gcc --version

echo Done.
pause
