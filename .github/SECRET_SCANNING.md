# Secret Scanning with Gitleaks

This repository uses [Gitleaks](https://github.com/gitleaks/gitleaks) to prevent secrets (API keys, passwords, private keys, tokens) from being committed to the codebase.

## How It Works

### Automated CI Scanning
- **Runs on:** All pull requests and pushes to `main` and `release-*` branches
- **What it scans:** Only new commits in your PR (not the entire git history)
- **Speed:** ~4 mins for full repository scan
- **Action:** Blocks PR merge if secrets are detected

### What Gitleaks Detects
- API keys (AWS, GitHub, GitLab, Slack, etc.)
- Private keys (RSA, SSH, PGP, TLS)
- Database credentials and connection strings
- OAuth and JWT tokens
- Generic secrets (password=, api_key=, etc.)
- High entropy strings (randomized secrets)

### What It Ignores
- Test fixtures in `test/` directories
- Vendor code in `vendor/`
- Example files in `examples/`
- Mock/placeholder credentials
- Variable names like `password` or `apiKey`

## Running Locally

### Installation

**macOS (Homebrew):**
```bash
brew install gitleaks
```

**Linux:**
```bash
# Docker/Podman
docker pull ghcr.io/gitleaks/gitleaks:latest

# Or download binary
wget https://github.com/gitleaks/gitleaks/releases/latest/download/gitleaks_linux_x64.tar.gz
tar -xzf gitleaks_linux_x64.tar.gz
sudo mv gitleaks /usr/local/bin/
```

### Scan Before Committing

**Scan staged changes (recommended):**
```bash
gitleaks protect --staged --verbose
```

**Scan entire working directory:**
```bash
gitleaks detect --source . --config .gitleaks.toml --verbose
```

**Scan specific file:**
```bash
gitleaks detect --source path/to/file.go --no-git
```

## Handling Detections

### If Gitleaks Flags Your Commit

**1. Is it a real secret?**

If YES:
- **Remove the secret immediately**
- Use environment variables instead: `os.Getenv("API_KEY")`
- Store secrets in Kubernetes Secrets, Vault, or similar
- Rotate/revoke the exposed secret if it was already pushed

If NO (false positive):
- Continue to step 2

**2. For legitimate test fixtures or examples:**

Add to `.gitleaks.toml` allowlist:

```toml
[allowlist]
paths = [
    '''path/to/test/file\.go$''',
]

# OR for specific values
regexes = [
    '''specific-test-value-to-ignore''',
]
```

**3. For one-time overrides (use sparingly):**

Add inline comment in your code:
```go
password := "test-password" // gitleaks:allow
```

## Configuration

The `.gitleaks.toml` file controls what gets scanned and ignored:

- **Excluded paths:** `test/`, `vendor/`, `examples/`, `*.md`
- **Excluded patterns:** Test credentials, base64 test values, common examples
- **Rules:** Extends default gitleaks ruleset

To modify exclusions, edit `.gitleaks.toml` and test:
```bash
gitleaks detect --source . --config .gitleaks.toml --verbose
```

## Best Practices

### DO:
- ✅ Use environment variables for secrets
- ✅ Use Kubernetes Secrets or external secret management
- ✅ Run `gitleaks protect --staged` before committing sensitive changes
- ✅ Use placeholder values in examples: `YOUR_API_KEY_HERE`

### DON'T:
- ❌ Commit real credentials, even temporarily
- ❌ Use `--no-verify` to bypass the check
- ❌ Add broad exclusions to `.gitleaks.toml` without review
- ❌ Assume deleted secrets are safe (git history remembers)

## Troubleshooting

### CI fails but local scan passes
```bash
# Ensure you're using the config file
gitleaks detect --source . --config .gitleaks.toml --no-git

# Check which gitleaks version CI uses
grep 'gitleaks-action@' .github/workflows/secret-scan.yml
```

### Too many false positives
1. Review the findings carefully
2. Update `.gitleaks.toml` with specific exclusions
3. Test the config change locally
4. Submit the config update in your PR

### Need to scan git history
```bash
# Scan all commits (WARNING: can be slow on large repos)
gitleaks detect --source . --verbose

# Scan specific commit range
gitleaks detect --log-opts="main..HEAD"
```

## Additional Resources

- [Gitleaks Documentation](https://github.com/gitleaks/gitleaks)
- [Gitleaks Configuration Reference](https://github.com/gitleaks/gitleaks#configuration)
- [GitHub Secret Scanning](https://docs.github.com/en/code-security/secret-scanning)

## Questions?

For issues with secret scanning:
1. Check this guide first
2. Review `.gitleaks.toml` configuration
3. Ask in your PR or open an issue

---

**Remember:** It's easier to prevent secrets from being committed than to clean them up from git history!
