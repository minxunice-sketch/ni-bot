---
name: skill-manager
description: Manage Ni bot skills. Use to search for new skills on GitHub and install them.
---

# Skill Manager

This skill allows the agent to self-extend by searching for and installing new skills from GitHub.

## Scripts

- `search-github.ps1`: Search GitHub repositories for skills or scripts.
  - Args: `Query` (string)
  - Usage: `[EXEC:skill.exec {"skill":"skill-manager","script":"search-github.ps1","args":["weather"]}]`

- `install-skill.ps1`: Install a skill from a GitHub repository URL.
  - Args: `Url` (string), `Name` (string, optional)
  - Usage: `[EXEC:skill.exec {"skill":"skill-manager","script":"install-skill.ps1","args":["https://github.com/user/repo", "my-skill"]}]`
