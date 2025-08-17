$ErrorActionPreference = "Stop"

try {
    Get-Command scoop -ErrorAction Stop | Out-Null
    Write-Host "Scoop already installed."
}
catch {
    Write-Host "Scoop not found — installing..."
    Set-ExecutionPolicy RemoteSigned -Scope CurrentUser -Force
    
    $scoopInstaller = Invoke-WebRequest -Uri "https://get.scoop.sh" -UseBasicParsing
    Invoke-Expression $scoopInstaller.Content
}

$scoopPath = $env:SCOOP
if (-not $scoopPath) {
    $scoopPath = "$env:USERPROFILE\scoop"
}

$msys2Path = Join-Path $scoopPath "apps\msys2"
if (-not (Test-Path $msys2Path)) {
    Write-Host "Installing MSYS2 (with MinGW64) via Scoop..."
    scoop install msys2
}
else {
    Write-Host "MSYS2 already installed."
}

Write-Host "Updating MSYS2..."
scoop update msys2

$msys2_shell = Join-Path $scoopPath "apps\msys2\current\usr\bin\bash.exe"

Write-Host "Installing MinGW64 toolchain and libraries..."

Start-Process -FilePath $msys2_shell -ArgumentList "-lc", "pacman --noconfirm -Syu" -Wait -NoNewWindow
Start-Process -FilePath $msys2_shell -ArgumentList "-lc", "pacman --noconfirm -S mingw-w64-x86_64-toolchain" -Wait -NoNewWindow
Start-Process -FilePath $msys2_shell -ArgumentList "-lc", "pacman --noconfirm -S mingw-w64-x86_64-curl mingw-w64-x86_64-pcre mingw-w64-x86_64-jansson" -Wait -NoNewWindow

Write-Host "Done."
