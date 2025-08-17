@echo off
setlocal enabledelayedexpansion

where scoop >nul 2>&1
if %errorlevel% neq 0 (
    powershell -Command "Set-ExecutionPolicy RemoteSigned -Scope CurrentUser -Force"
    powershell -Command "$scoopInstaller = Invoke-WebRequest -Uri 'https://get.scoop.sh' -UseBasicParsing; Invoke-Expression $scoopInstaller.Content"
    call refreshenv.exe 2>nul || (
        for /f "tokens=2*" %%a in ('reg query "HKCU\Environment" /v PATH 2^>nul') do set "USERPATH=%%b"
        if defined USERPATH set "PATH=%PATH%;%USERPATH%"
        set "PATH=%PATH%;%USERPROFILE%\scoop\shims"
    )
)

if not defined SCOOP set "SCOOP=%USERPROFILE%\scoop"
if not exist "%SCOOP%\apps\msys2" (
    scoop install msys2
)

set "MSYS2_SHELL=%SCOOP%\apps\msys2\current\usr\bin\bash.exe"
"%MSYS2_SHELL%" -lc "pacman --noconfirm -Syu"
"%MSYS2_SHELL%" -lc "pacman --noconfirm -S mingw-w64-x86_64-toolchain"
"%MSYS2_SHELL%" -lc "pacman --noconfirm -S mingw-w64-x86_64-curl mingw-w64-x86_64-pcre mingw-w64-x86_64-jansson"

set "MINGW64_BIN=%SCOOP%\apps\msys2\current\mingw64\bin"

for /f "tokens=2*" %%a in ('reg query "HKCU\Environment" /v PATH 2^>nul') do set "CURRENT_PATH=%%b"

echo %CURRENT_PATH% | find /i "%MINGW64_BIN%" >nul
if %errorlevel% neq 0 (
    if defined CURRENT_PATH (
        set "NEW_PATH=%CURRENT_PATH%;%MINGW64_BIN%"
    ) else (
        set "NEW_PATH=%MINGW64_BIN%"
    )
    reg add "HKCU\Environment" /v PATH /t REG_EXPAND_SZ /d "%NEW_PATH%" /f >nul
    set "PATH=%PATH%;%MINGW64_BIN%"
)

echo "Done."
pause
