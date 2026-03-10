
# Claude.ps1 - Startup script for Windows

# Set environment variables for permissions
$env:NIBOT_ENABLE_SKILLS = "1"
$env:NIBOT_POLICY_ALLOW_FS_WRITE = "true"
$env:NIBOT_POLICY_ALLOW_RUNTIME_EXEC = "true"
$env:NIBOT_POLICY_ALLOW_SKILL_EXEC = "true"
$env:NIBOT_POLICY_ALLOW_SKILL_INSTALL = "true"
$env:NIBOT_POLICY_ALLOW_MEMORY = "true"

# Determine the directory of the script
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path

# Run the bot
go run "$ScriptDir\cmd\web"
