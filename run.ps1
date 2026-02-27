param(
  [string]$GoBin = "",
  [switch]$EnableSkills,
  [switch]$EnableExec
)

$ErrorActionPreference = "Stop"

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
  if ($GoBin -ne "" -and (Test-Path $GoBin)) {
    $env:Path += ";$GoBin"
  } elseif (Test-Path "C:\Program Files\Go\bin") {
    $env:Path += ";C:\Program Files\Go\bin"
  } elseif (Test-Path "C:\Progra~1\Go\bin") {
    $env:Path += ";C:\Progra~1\Go\bin"
  }
}

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
  Write-Host "Go not found. Install Go from https://go.dev/dl/ or pass -GoBin `"C:\Program Files\Go\bin`"." -ForegroundColor Red
  exit 1
}

go version

if ($EnableSkills) {
  $env:NIBOT_ENABLE_SKILLS = "1"
}
if ($EnableExec) {
  $env:NIBOT_ENABLE_EXEC = "1"
}

go run .\cmd\nibot

