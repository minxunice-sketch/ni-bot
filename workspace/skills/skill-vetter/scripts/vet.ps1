param(
  [Parameter(Mandatory = $true)]
  [string]$Repo
)

$ErrorActionPreference = "Stop"
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8

function New-GitHubHeaders {
  $h = @{
    "User-Agent" = "NiBot-Agent"
    "Accept"     = "application/vnd.github+json"
  }
  $tok = $env:GITHUB_TOKEN
  if (-not [string]::IsNullOrWhiteSpace($tok)) {
    $h["Authorization"] = "Bearer $tok"
  }
  return $h
}

function Get-GitHubJson([string]$Url) {
  return Invoke-RestMethod -Uri $Url -Headers (New-GitHubHeaders) -Method Get
}

function Get-GitHubTextFromContentApi([string]$Repo, [string]$Path) {
  $u = "https://api.github.com/repos/$Repo/contents/$Path"
  try {
    $obj = Invoke-RestMethod -Uri $u -Headers (New-GitHubHeaders) -Method Get
  } catch {
    return $null
  }
  if ($null -eq $obj -or [string]::IsNullOrWhiteSpace($obj.content)) {
    return $null
  }
  $b = [Convert]::FromBase64String(($obj.content -replace "\s", ""))
  return [System.Text.Encoding]::UTF8.GetString($b)
}

$repo = $Repo.Trim()

$meta = Get-GitHubJson "https://api.github.com/repos/$repo"
$contents = @()
try {
  $contents = Get-GitHubJson "https://api.github.com/repos/$repo/contents"
} catch {
  $contents = @()
}

$names = @()
foreach ($c in $contents) {
  if ($null -ne $c.name) { $names += ($c.name.ToString()) }
}

$readme = Get-GitHubTextFromContentApi $repo "README.md"
if ($null -eq $readme) { $readme = "" }
$readmeLower = $readme.ToLowerInvariant()

$score = 0
$signals = New-Object System.Collections.Generic.List[string]

if ($meta.archived -eq $true) { $score += 10; $signals.Add("repo archived") }
if ($null -eq $meta.license) { $score += 10; $signals.Add("no license metadata") }
if ($names -contains "Dockerfile") { $score += 8; $signals.Add("has Dockerfile") }
if ($names -contains ".github") { $score += 8; $signals.Add("has .github directory") }

if ($readmeLower -match "curl\s+[^|]*\|\s*(sh|bash)") { $score += 45; $signals.Add("README contains curl|sh pattern") }
if ($readmeLower -match "bash\s+<\(\s*curl") { $score += 45; $signals.Add("README contains bash <(curl ...)") }
if ($readmeLower -match "rm\s+-rf") { $score += 50; $signals.Add("README contains rm -rf") }
if ($readmeLower -match "\bsudo\b") { $score += 15; $signals.Add("README contains sudo") }

try {
  $created = [DateTimeOffset]::Parse($meta.created_at)
  $ageDays = ([DateTimeOffset]::UtcNow - $created).TotalDays
  if ($ageDays -lt 30) { $score += 10; $signals.Add("repo very new (<30d)") }
} catch {
}

$risk = "low"
if ($score -ge 80) { $risk = "high" }
elseif ($score -ge 40) { $risk = "medium" }

$out = [ordered]@{
  ok        = $true
  riskLevel = $risk
  score     = [int]$score
  signals   = $signals.ToArray()
}

Write-Output ($out | ConvertTo-Json -Compress)

