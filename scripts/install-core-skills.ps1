# Ni Bot Core Skills Installation Script (PowerShell)
# This script installs the three essential skills for Ni Bot

Write-Host "🚀 Installing Ni Bot Core Skills..." -ForegroundColor Green

# Create skills directory if it doesn't exist
$skillsDir = ".trae\skills"
if (!(Test-Path $skillsDir)) {
    New-Item -ItemType Directory -Path $skillsDir -Force | Out-Null
}

# Define core skills array
$coreSkills = @("web-search", "web-crawler", "evomap")

# Check if skills are already installed
Write-Host "📋 Checking existing skills..." -ForegroundColor Yellow
foreach ($skill in $coreSkills) {
    $skillPath = "$skillsDir\$skill"
    if (Test-Path $skillPath) {
        Write-Host "✅ $skill skill already installed" -ForegroundColor Green
    } else {
        Write-Host "❌ $skill skill not found" -ForegroundColor Red
    }
}

Write-Host ""
Write-Host "🎯 Installing missing core skills..." -ForegroundColor Yellow

# Install Web Search skill
$webSearchPath = "$skillsDir\web-search"
if (!(Test-Path $webSearchPath)) {
    Write-Host "📥 Installing Web Search skill..." -ForegroundColor Cyan
    New-Item -ItemType Directory -Path $webSearchPath -Force | Out-Null
    
    $webSearchContent = @"
---
name: "web-search"
description: "Performs web searches to find real-time information. Invoke when user needs current data, news, or information not in local knowledge base."
---

# Web Search Skill

This skill enables real-time web searching capabilities for the AI agent.

## Usage
- /search "query" - Perform web search
- /search -engine=google "query" - Use specific search engine
- /search -lang=zh "查询" - Search in specific language

## Supported Engines
- Google Search
- Bing Search  
- DuckDuckGo

## Features
- Real-time information access
- Multi-language support
- Search engine selection
- Result filtering and ranking
"@
    
    Set-Content -Path "$webSearchPath\SKILL.md" -Value $webSearchContent
    Write-Host "✅ Web Search skill installed" -ForegroundColor Green
}

# Install Web Crawler skill
$webCrawlerPath = "$skillsDir\web-crawler"
if (!(Test-Path $webCrawlerPath)) {
    Write-Host "📥 Installing Web Crawler skill..." -ForegroundColor Cyan
    New-Item -ItemType Directory -Path $webCrawlerPath -Force | Out-Null
    
    $webCrawlerContent = @"
---
name: "web-crawler"
description: "Crawls and extracts content from web pages for deep analysis. Invoke when user needs detailed content extraction, website scraping, or deep web page analysis."
---

# Web Crawler & Reader Skill

This skill provides advanced web content extraction capabilities.

## Usage
- /crawl "url" - Extract content from URL
- /crawl -depth=2 "url" - Crawl with specific depth
- /crawl -format=markdown "url" - Extract as markdown

## Content Types
- HTML pages and articles
- PDF documents
- Structured data (tables, lists)
- Images and media metadata

## Features
- Deep content extraction
- Multi-page crawling
- Content summarization
- Data structure parsing
"@
    
    Set-Content -Path "$webCrawlerPath\SKILL.md" -Value $webCrawlerContent
    Write-Host "✅ Web Crawler skill installed" -ForegroundColor Green
}

# Install EvoMap skill
$evomapPath = "$skillsDir\evomap"
if (!(Test-Path $evomapPath)) {
    Write-Host "📥 Installing EvoMap skill..." -ForegroundColor Cyan
    New-Item -ItemType Directory -Path $evomapPath -Force | Out-Null
    
    $evomapContent = @"
---
name: "evomap"
description: "Evolutionary mapping and strategy optimization for continuous improvement. Invoke when user needs adaptive learning, strategy evolution, or performance optimization over time."
---

# EvoMap & Evolver Skill

This skill provides evolutionary algorithm capabilities for continuous learning.

## Usage
- /evolve strategy strategy-name - Evolve specific strategy
- /optimize metric-name - Optimize performance metric
- /adapt parameter value - Adapt learning parameters

## Evolutionary Mechanisms
- Genetic algorithms
- Strategy optimization
- Performance tracking
- Adaptive learning

## Features
- Continuous improvement
- Multi-objective optimization
- Real-time adaptation
- Knowledge evolution
"@
    
    Set-Content -Path "$evomapPath\SKILL.md" -Value $evomapContent
    Write-Host "✅ EvoMap skill installed" -ForegroundColor Green
}

Write-Host ""
Write-Host "🎉 Core skills installation complete!" -ForegroundColor Green
Write-Host ""
Write-Host "📊 Installed Skills:" -ForegroundColor Yellow
Get-ChildItem $skillsDir | ForEach-Object { Write-Host "- $($_.Name)" -ForegroundColor White }
Write-Host ""
Write-Host "💡 Usage Tips:" -ForegroundColor Yellow
Write-Host "- Use '/search' for real-time information" -ForegroundColor White
Write-Host "- Use '/crawl' for deep content analysis" -ForegroundColor White  
Write-Host "- Use '/evolve' for continuous improvement" -ForegroundColor White
Write-Host ""
Write-Host "🚀 Ni Bot is now equipped with essential capabilities!" -ForegroundColor Green