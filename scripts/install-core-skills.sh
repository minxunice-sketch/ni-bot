#!/bin/bash

# Ni Bot Core Skills Installation Script
# This script installs the three essential skills for Ni Bot

echo "🚀 Installing Ni Bot Core Skills..."

# Create skills directory if it doesn't exist
mkdir -p .trae/skills

# Define core skills array
CORE_SKILLS=("web-search" "web-crawler" "evomap")

# Check if skills are already installed
echo "📋 Checking existing skills..."
for skill in "${CORE_SKILLS[@]}"; do
    if [ -d ".trae/skills/$skill" ]; then
        echo "✅ $skill skill already installed"
    else
        echo "❌ $skill skill not found"
    fi
done

echo ""
echo "🎯 Installing missing core skills..."

# Install Web Search skill
if [ ! -d ".trae/skills/web-search" ]; then
    echo "📥 Installing Web Search skill..."
    mkdir -p .trae/skills/web-search
    cat > .trae/skills/web-search/SKILL.md << 'EOF'
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
EOF
    echo "✅ Web Search skill installed"
fi

# Install Web Crawler skill
if [ ! -d ".trae/skills/web-crawler" ]; then
    echo "📥 Installing Web Crawler skill..."
    mkdir -p .trae/skills/web-crawler
    cat > .trae/skills/web-crawler/SKILL.md << 'EOF'
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
EOF
    echo "✅ Web Crawler skill installed"
fi

# Install EvoMap skill
if [ ! -d ".trae/skills/evomap" ]; then
    echo "📥 Installing EvoMap skill..."
    mkdir -p .trae/skills/evomap
    cat > .trae/skills/evomap/SKILL.md << 'EOF'
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
EOF
    echo "✅ EvoMap skill installed"
fi

echo ""
echo "🎉 Core skills installation complete!"
echo ""
echo "📊 Installed Skills:"
ls -la .trae/skills/
echo ""
echo "💡 Usage Tips:"
echo "- Use '/search' for real-time information"
echo "- Use '/crawl' for deep content analysis"  
echo "- Use '/evolve' for continuous improvement"
echo ""
echo "🚀 Ni Bot is now equipped with essential capabilities!"

# Make the script executable
chmod +x "$0"