# console.store installer (Windows) — irm consolestore.in/install.ps1 | iex
#   $env:CONSOLE_CHANNEL = "beta"|"alpha"   ;  $env:CONSOLE_ALPHA_CODE = "<code>"
$ErrorActionPreference = "Stop"
# Invoke-WebRequest renders a progress bar per-chunk that throttles downloads to
# a crawl (a ~10 MB binary can take a minute and look hung). Silence it.
$ProgressPreference = "SilentlyContinue"
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
$out = Join-Path $dir "console.exe"
$tmp = "$out.new"

Invoke-WebRequest -UseBasicParsing "$base/$channel/download/$asset$q" -OutFile $tmp
$got = (Get-FileHash -Algorithm SHA256 $tmp).Hash.ToLower()
if ($got -ne $sum.ToLower()) { Remove-Item $tmp; throw "checksum mismatch — refusing to install" }
Move-Item -Force $tmp $out

# Persist the channel marker so self-update tracks this channel — and, for alpha,
# carries the access code (without it the alpha manifest fetch is 403 and updates
# stop). Matches the path the binary reads (XDG_CONFIG_HOME or ~/.config).
$cfgBase = if ($env:XDG_CONFIG_HOME) { $env:XDG_CONFIG_HOME } else { Join-Path $env:USERPROFILE ".config" }
$cfgDir = Join-Path $cfgBase "console-store"
New-Item -ItemType Directory -Force -Path $cfgDir | Out-Null
$marker = if ($channel -eq "alpha") { "{`"channel`":`"alpha`",`"alpha_code`":`"$code`"}" } else { "{`"channel`":`"$channel`"}" }
Set-Content -Path (Join-Path $cfgDir "channel.json") -Value $marker -NoNewline

# Add install dir to the persisted user PATH if absent (for future terminals).
$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
$pathWasAdded = $false
if ($userPath -notlike "*$dir*") {
  [Environment]::SetEnvironmentVariable("Path", "$userPath;$dir", "User")
  $pathWasAdded = $true
}
# Also update THIS session's PATH so `console` runs immediately — without it the
# new dir only reaches terminals opened AFTER the install (the #1 "command not
# found" confusion).
if (";$env:Path;" -notlike "*;$dir;*") {
  $env:Path = "$env:Path;$dir"
}
Write-Host "OK installed console -> $out" -ForegroundColor Green
if ($pathWasAdded) {
  Write-Host "added $dir to your PATH — usable now in this window; open a NEW terminal for future sessions." -ForegroundColor DarkGray
}
Write-Host "run: console"
# Wire console into local AI agents (MCP + skills). Best-effort + idempotent;
# CONSOLE_NO_AGENT_SETUP=1 opts out (handled inside the binary).
try { & $out agents install --quiet } catch { }
