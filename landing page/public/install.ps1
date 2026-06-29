# console.store installer (Windows) — irm consolestore.in/install.ps1 | iex
#   $env:CONSOLE_CHANNEL = "beta"|"alpha"   ;  $env:CONSOLE_ALPHA_CODE = "<code>"
$ErrorActionPreference = "Stop"
$base = if ($env:CONSOLE_BASE) { $env:CONSOLE_BASE } else { "https://consolestore.in" }
$channel = if ($env:CONSOLE_CHANNEL) { $env:CONSOLE_CHANNEL } else { "stable" }
$code = $env:CONSOLE_ALPHA_CODE

if ($channel -eq "alpha" -and -not $code) {
  throw "alpha channel is invite-only — set `$env:CONSOLE_ALPHA_CODE to your access code"
}

$arch = if ([Environment]::Is64BitOperatingSystem) {
  if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "amd64" }
} else { throw "unsupported 32-bit OS" }

$asset = "store_windows_$arch.exe"
$q = if ($channel -eq "alpha") { "?code=$code" } else { "" }

Write-Host "console.store channel $channel — fetching windows/$arch..." -ForegroundColor Cyan

$sum = (Invoke-WebRequest -UseBasicParsing "$base/$channel/checksum/$asset$q").Content.Trim()
$dir = Join-Path $env:LOCALAPPDATA "Programs\console.store"
New-Item -ItemType Directory -Force -Path $dir | Out-Null
$out = Join-Path $dir "store.exe"
$tmp = "$out.new"

Invoke-WebRequest -UseBasicParsing "$base/$channel/download/$asset$q" -OutFile $tmp
$got = (Get-FileHash -Algorithm SHA256 $tmp).Hash.ToLower()
if ($got -ne $sum.ToLower()) { Remove-Item $tmp; throw "checksum mismatch — refusing to install" }
Move-Item -Force $tmp $out

# Add install dir to the user PATH if absent.
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -notlike "*$dir*") {
  [Environment]::SetEnvironmentVariable("Path", "$userPath;$dir", "User")
  Write-Host "added $dir to your PATH (restart the terminal)" -ForegroundColor DarkGray
}
Write-Host "OK installed store -> $out" -ForegroundColor Green
Write-Host "run: store"
