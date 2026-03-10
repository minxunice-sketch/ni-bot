param(
    [string]$Query,
    [string]$Language = "PowerShell"
)
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$ErrorActionPreference = "Stop"

if ([string]::IsNullOrWhiteSpace($Query)) {
    Write-Error "Query parameter is required"
    exit 1
}

# Search GitHub API
$encoded = [uri]::EscapeDataString($Query)
$langPart = ""
if (-not [string]::IsNullOrWhiteSpace($Language) -and $Language -ne "All") {
    $langPart = "+language:$Language"
}
$url = "https://api.github.com/search/repositories?q=$encoded$langPart&sort=stars&order=desc"

try {
    # Use Invoke-RestMethod for JSON parsing
    $response = Invoke-RestMethod -Uri $url -Headers @{"User-Agent"="NiBot-Agent"}
    
    if ($response.total_count -eq 0) {
        Write-Output "No repositories found for query: $Query"
        exit 0
    }

    Write-Output "Found $($response.total_count) repositories (showing top 5):"
    foreach ($item in $response.items | Select-Object -First 5) {
        Write-Output "Name: $($item.name)"
        Write-Output "Description: $($item.description)"
        Write-Output "URL: $($item.clone_url)"
        Write-Output "Stars: $($item.stargazers_count)"
        Write-Output "---"
    }
} catch {
    Write-Error "GitHub search failed: $_"
    exit 1
}