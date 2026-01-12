# CI/CD Documentation

## Overview

funcfinder uses GitHub Actions for continuous integration and deployment. The CI/CD pipeline automatically builds, tests, and releases the toolkit across multiple platforms.

## Workflows

### 1. CI Workflow (`ci.yml`)

**Triggers:**
- Push to `main`, `master`, or `develop` branches
- Pull requests to these branches

**Jobs:**

#### Test Job
- **Matrix:** Tests on Go 1.21, 1.22, and 1.23
- **Steps:**
  1. Checkout code
  2. Set up Go environment
  3. Cache Go modules
  4. Run tests with race detector
  5. Generate coverage report (84.8% coverage!)
  6. Upload coverage to Codecov
  7. Display coverage in GitHub summary

#### Build Job (Linux)
- Builds all 4 binaries using `build.sh`
- Tests each binary with `--version`
- Uploads artifacts for 7 days
- **Artifacts:** `binaries-linux`

#### Build Job (Windows)
- Builds using `build.ps1`
- Tests Windows executables
- **Artifacts:** `binaries-windows`

#### Build Job (macOS)
- Builds for macOS
- Tests binaries
- **Artifacts:** `binaries-macos`

**Coverage Reporting:**
- Uploads to Codecov automatically
- Displays total coverage in PR comments
- Shows coverage summary in GitHub Actions UI

---

### 2. Release Workflow (`release.yml`)

**Triggers:**
- Push of tags matching `v*` (e.g., `v1.4.0`, `v1.5.0`)

**Cross-Platform Builds:**
- Linux AMD64
- Linux ARM64
- Windows AMD64
- macOS AMD64 (Intel)
- macOS ARM64 (M1/M2)

**Steps:**
1. Checkout code
2. Set up Go 1.23
3. Build binaries for all platforms (5 × 4 = 20 binaries)
4. Create platform-specific archives:
   - `.tar.gz` for Linux/macOS
   - `.zip` for Windows
5. Generate SHA256 checksums
6. Create GitHub Release with:
   - All binary archives
   - Checksums file
   - Installation instructions
   - Changelog reference

**Release Assets:**
```
funcfinder-1.4.0-linux-amd64.tar.gz
funcfinder-1.4.0-linux-arm64.tar.gz
funcfinder-1.4.0-windows-amd64.zip
funcfinder-1.4.0-darwin-amd64.tar.gz
funcfinder-1.4.0-darwin-arm64.tar.gz
checksums.txt
```

---

### 3. CodeQL Workflow (`codeql.yml`)

**Triggers:**
- Push to `main`/`master`
- Pull requests
- Weekly schedule (Sunday at midnight)

**Purpose:**
- Security vulnerability scanning
- Code quality analysis
- Dependency analysis

---

## Makefile Automation

The `Makefile` provides convenient commands for local development:

### Building

```bash
make build          # Build all binaries
make build-all      # Build for all platforms
make install        # Install to /usr/local/bin
make uninstall      # Remove from /usr/local/bin
```

### Testing

```bash
make test           # Run all tests
make test-coverage  # Run with coverage report
make coverage       # Generate HTML coverage report
make bench          # Run benchmarks
```

### Code Quality

```bash
make fmt            # Format code with gofmt
make vet            # Run go vet
make lint           # Run golangci-lint
make check          # Run all checks (fmt, vet, test)
```

### Development

```bash
make run            # Run funcfinder on itself
make analyze        # Run all tools on codebase
make watch          # Watch for changes and rebuild
```

### Maintenance

```bash
make clean          # Remove binaries and artifacts
make deps           # Download dependencies
make tidy           # Tidy go.mod
make update         # Update dependencies
```

### Release

```bash
make release VERSION=1.5.0  # Prepare new release
make version                # Show current version
```

---

## Creating a New Release

### Method 1: Using Makefile (Recommended)

```bash
# 1. Update CHANGELOG.md with new version details
vim CHANGELOG.md

# 2. Create release (updates version strings and creates git tag)
make release VERSION=1.5.0

# 3. Push tag to trigger release workflow
git push origin v1.5.0

# 4. GitHub Actions will automatically:
#    - Build binaries for all platforms
#    - Create GitHub Release
#    - Upload all artifacts
```

### Method 2: Manual

```bash
# 1. Update version in all main.go files
sed -i 's/Version = "1.4.0"/Version = "1.5.0"/' cmd/*/main.go

# 2. Update CHANGELOG.md

# 3. Commit changes
git add .
git commit -m "Release v1.5.0"

# 4. Create and push tag
git tag -a v1.5.0 -m "Release v1.5.0"
git push origin v1.5.0
```

---

## Monitoring CI/CD

### GitHub Actions UI

View workflow runs at:
```
https://github.com/YOUR_USERNAME/funcfinder/actions
```

### CI Status Badges

Add to README.md:
```markdown
[![CI](https://github.com/YOUR_USERNAME/funcfinder/workflows/CI/badge.svg)](https://github.com/YOUR_USERNAME/funcfinder/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/YOUR_USERNAME/funcfinder/branch/main/graph/badge.svg)](https://codecov.io/gh/YOUR_USERNAME/funcfinder)
[![Go Report Card](https://goreportcard.com/badge/github.com/YOUR_USERNAME/funcfinder)](https://goreportcard.com/report/github.com/YOUR_USERNAME/funcfinder)
```

---

## Troubleshooting

### Build Failures

**Problem:** Tests fail on specific Go version
**Solution:** Check test compatibility, update go.mod if needed

**Problem:** Windows build fails
**Solution:** Verify `build.ps1` syntax, check PowerShell compatibility

### Release Issues

**Problem:** Release workflow doesn't trigger
**Solution:** Ensure tag format matches `v*` pattern (e.g., `v1.5.0` not `1.5.0`)

**Problem:** Missing binaries in release
**Solution:** Check build logs for GOOS/GOARCH errors

### Coverage Issues

**Problem:** Coverage upload fails
**Solution:** Check Codecov token in repository secrets

---

## Best Practices

1. **Always run tests locally before pushing:**
   ```bash
   make check
   ```

2. **Test cross-platform builds before release:**
   ```bash
   make build-all
   ```

3. **Update CHANGELOG.md for every release**

4. **Use semantic versioning:**
   - MAJOR.MINOR.PATCH (e.g., 1.4.0)
   - MAJOR: Breaking changes
   - MINOR: New features (backward compatible)
   - PATCH: Bug fixes

5. **Tag releases with `v` prefix:**
   ```bash
   git tag v1.4.0  # ✅ Correct
   git tag 1.4.0   # ❌ Wrong - won't trigger release
   ```

---

## Performance Metrics

### Build Times (Approximate)

| Platform | Time |
|----------|------|
| Linux AMD64 | ~30s |
| Linux ARM64 | ~35s |
| Windows AMD64 | ~40s |
| macOS AMD64 | ~45s |
| macOS ARM64 | ~45s |
| **Total** | **~3 minutes** |

### Test Suite

- **Duration:** ~5 seconds
- **Tests:** 60+
- **Coverage:** 84.8%
- **Parallel:** Yes (race detector enabled)

---

## Security

### CodeQL Analysis

- Runs weekly and on every PR
- Scans for:
  - SQL injection
  - Command injection
  - Path traversal
  - Insecure dependencies
  - Code quality issues

### Dependency Management

- Dependabot enabled (if configured)
- Regular security updates
- Go modules with checksums

---

## Future Improvements

- [ ] Add integration tests
- [ ] Docker container builds
- [ ] Homebrew formula automation
- [ ] Windows Chocolatey package
- [ ] Linux package repositories (deb/rpm)
- [ ] Performance benchmarking in CI
- [ ] Automated changelog generation
