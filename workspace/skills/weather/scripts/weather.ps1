param(
  [string]$City = "Beijing"
)

[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$ProgressPreference = "SilentlyContinue"

$encoded = [uri]::EscapeDataString($City)
# Use concatenation to avoid variable expansion issues with ?
# Prefer JSON (j1) for detailed data that the LLM can process
$uri = "https://wttr.in/" + $encoded + "?format=j1"

# Try curl.exe first (preferred)
$curlAvailable = Get-Command curl.exe -ErrorAction SilentlyContinue
if ($curlAvailable) {
    # Quote the URI to prevent shell parsing issues with special characters
    $raw = curl.exe -sL "$uri"
    if (-not [string]::IsNullOrWhiteSpace($raw) -and $raw.Trim().StartsWith("{")) {
        Write-Output $raw
        exit 0
    }
}

# Fallback to Invoke-WebRequest for JSON
try {
    # Force User-Agent to curl to ensure correct response
    $raw = (Invoke-WebRequest -Uri $uri -UseBasicParsing -UserAgent "curl/7.68.0").Content
    if (-not [string]::IsNullOrWhiteSpace($raw) -and $raw.Trim().StartsWith("{")) {
        Write-Output $raw
        exit 0
    }
} catch {
    # ignore
}

# If JSON fails, fallback to simple text format (format=4)
$uri = "https://wttr.in/" + $encoded + "?format=4"
try {
     $raw = (Invoke-WebRequest -Uri $uri -UseBasicParsing -UserAgent "curl/7.68.0").Content
     if (-not [string]::IsNullOrWhiteSpace($raw)) {
         Write-Output $raw.Trim()
         exit 0
     }
} catch {
     # ignore
}

Write-Output ("{0}: (no response)" -f $City)

