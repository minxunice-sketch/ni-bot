# Short alias for Claude.ps1
# Usage: ./cc.ps1

$ErrorActionPreference = "Stop"
$scriptPath = Split-Path -Parent $MyInvocation.MyCommand.Definition
& "$scriptPath\Claude.ps1" @args
