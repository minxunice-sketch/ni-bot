---
name: "web-crawler"
description: "Crawls and extracts content from web pages for deep analysis. Invoke when user needs detailed content extraction, website scraping, or deep web page analysis."
---

# Web Crawler & Reader Skill

This skill provides advanced web content extraction capabilities, allowing the AI agent to crawl websites, extract structured content, and perform deep analysis of web pages.

## Capabilities

- **Web Page Crawling**: Navigate and extract content from web pages
- **Content Extraction**: Parse HTML and extract text, images, links, and metadata
- **PDF Processing**: Extract text from PDF documents
- **Structured Data**: Parse tables, lists, and structured content
- **Multi-page Crawling**: Follow links and crawl entire websites
- **Content Analysis**: Summarize and analyze extracted content

## Usage

### Basic Content Extraction
```
/crawl "https://example.com/article"
/crawl -depth=2 "https://news.site.com"
```

### Advanced Extraction
```
/crawl -format=markdown "https://blog.com/post"
/crawl -elements=text,links "https://docs.site.com"
/crawl -pdf "https://site.com/document.pdf"
```

### Extraction Options
- `-depth`: Crawling depth (1-5)
- `-format`: Output format (text, markdown, html)
- `-elements`: Specific elements to extract
- `-pdf`: Process PDF documents
- `-timeout`: Request timeout in seconds

## Examples

1. **Extract article content**
   ```
   /crawl "https://medium.com/ai-article"
   ```

2. **Crawl documentation site**
   ```
   /crawl -depth=3 "https://docs.python.org"
   ```

3. **Extract specific content**
   ```
   /crawl -elements=tables,code "https://github.com/docs"
   ```

4. **Process research paper**
   ```
   /crawl -pdf "https://arxiv.org/pdf/paper.pdf"
   ```

## Content Types Supported

- **Articles & Blogs**: Full text extraction with formatting
- **Documentation**: Code examples, API docs, tutorials
- **Research Papers**: Academic papers and publications
- **News Sites**: News articles and updates
- **E-commerce**: Product information and reviews
- **Forums**: Discussion threads and comments

## Advanced Features

### Content Analysis
- Automatic summarization of long articles
- Key point extraction and highlighting
- Sentiment analysis of text content
- Topic modeling and categorization

### Data Extraction
- Table data extraction to CSV/JSON
- Image metadata and alt text extraction
- Link graph and site structure analysis
- Metadata extraction (authors, dates, tags)

## Best Practices

- Respect `robots.txt` and website terms of service
- Use appropriate delays between requests
- Cache results to avoid repeated crawling
- Handle different character encodings properly
- Manage sessions and cookies for authenticated content

## Integration

This skill works with:
- Web Search skill for finding content to crawl
- Database skills for storing extracted data
- Analysis skills for processing extracted content
- Export skills for saving results in various formats