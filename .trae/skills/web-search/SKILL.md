---
name: "web-search"
description: "Performs web searches to find real-time information. Invoke when user needs current data, news, or information not in local knowledge base."
---

# Web Search Skill

This skill enables real-time web searching capabilities for the AI agent, allowing it to access current information, news, and data from the internet.

## Capabilities

- **Google Search**: Perform web searches using Google's search engine
- **Bing Search**: Alternative search engine support
- **DuckDuckGo**: Privacy-focused search option
- **Real-time Results**: Get current information and news
- **Multi-language Support**: Search in various languages

## Usage

### Basic Search
```
/search "latest AI developments 2025"
/search "current weather in Beijing"
```

### Advanced Search
```
/search -engine=google "machine learning trends"
/search -lang=zh "人工智能新闻"
/search -site=github.com "react components"
```

### Search Options
- `-engine`: Specify search engine (google, bing, duckduckgo)
- `-lang`: Language code for results
- `-site`: Limit to specific website
- `-limit`: Number of results to return

## Examples

1. **Get current news**
   ```
   /search "breaking technology news"
   ```

2. **Research specific topic**
   ```
   /search "quantum computing advancements 2025"
   ```

3. **Find technical information**
   ```
   /search -site=stackoverflow.com "python async await"
   ```

## Integration

This skill integrates with the agent's knowledge system, allowing it to:
- Supplement existing knowledge with current information
- Verify facts and data accuracy
- Provide real-time updates and news
- Access specialized information not in training data

## Best Practices

- Use for time-sensitive information
- Verify critical information from multiple sources
- Respect copyright and usage policies
- Cache results for repeated queries