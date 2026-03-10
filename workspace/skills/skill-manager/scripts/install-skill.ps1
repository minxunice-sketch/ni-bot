param(
    [string]$Url,
    [string]$Name
)
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($Url)) {
    Write-Error "Url parameter is required"
    exit 1
}

# Infer name from URL if not provided
if ([string]::IsNullOrWhiteSpace($Name)) {
    $Name = ($Url -split "/")[-1] -replace "\.git$", ""
}

$skillDir = Join-Path "workspace/skills" $Name

if (Test-Path $skillDir) {
    Write-Error "Skill '$Name' already exists at $skillDir"
    exit 1
}

Write-Output "Installing skill '$Name' from $Url..."

try {
    # Use git clone
    git clone $Url $skillDir
    if ($LASTEXITCODE -eq 0) {
        Write-Output "Successfully installed skill '$Name'."
        Write-Output "Please reload the agent to use the new skill."
    } else {
        Write-Error "Git clone failed."
        exit 1
    }
} catch {
    Write-Error "Installation failed: $_"
    exit 1
}