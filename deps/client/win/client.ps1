$ErrorActionPreference = "Stop"

if (-not (Get-Command scoop -ErrorAction SilentlyContinue)) {
    Write-Host "Scoop not found — installing..."
    Set-ExecutionPolicy RemoteSigned -Scope CurrentUser -Force
    Invoke-RestMethod get.scoop.sh | Invoke-Expression
}
else {
    Write-Host "Scoop already installed."
}

if (-not (Test-Path "$env:SCOOP\apps\msys2")) {
    Write-Host "Installing MSYS2 (with MinGW64) via Scoop..."
    scoop install msys2
}
else {
    Write-Host "MSYS2 already installed."
}

Write-Host "Updating MSYS2..."
scoop update msys2

$msys2_shell = "$env:SCOOP\apps\msys2\current\usr\bin\bash.exe"

Write-Host "Installing MinGW64 toolchain and libraries..."

& $msys2_shell -lc "pacman --noconfirm -Syu"
& $msys2_shell -lc "pacman --noconfirm -S mingw-w64-x86_64-toolchain"
& $msys2_shell -lc "pacman --noconfirm -S mingw-w64-x86_64-curl mingw-w64-x86_64-pcre mingw-w64-x86_64-jansson"

Write-Host "Done."
