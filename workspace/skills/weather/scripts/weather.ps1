param(
  [string]$City = "Beijing"
)

[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$ProgressPreference = "SilentlyContinue"

$encoded = [uri]::EscapeDataString($City)
$uri = "https://wttr.in/$encoded?format=3"
$raw = curl.exe -s $uri
if ([string]::IsNullOrWhiteSpace($raw)) {
  Write-Output ("{0}: (no response)" -f $City)
  exit 0
}

$line = ($raw -split "(`r`n|`n|`r)" | Where-Object { -not [string]::IsNullOrWhiteSpace($_) } | Select-Object -First 1)
if ($line -notmatch [regex]::Escape($City)) {
  Write-Output ("{0}: {1}" -f $City, $line)
  exit 0
}
Write-Output $line

