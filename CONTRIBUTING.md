# Contributing to CloudVault

Thank you for your interest in contributing to CloudVault! 🎉

We welcome contributions from everyone, whether you're fixing a typo, adding a feature, or helping with documentation.

## 🚀 Getting Started

### Prerequisites

- **Go 1.21+** - [Download](https://go.dev/dl/)
- **Node.js 18+** - [Download](https://nodejs.org/)
- **Docker** - [Download](https://docs.docker.com/get-docker/)
- **kubectl** - [Install](https://kubernetes.io/docs/tasks/tools/)
- **Git** - [Download](https://git-scm.com/downloads)

### Development Setup

1. **Fork the repository**
   
   Click the "Fork" button on GitHub.

2. **Clone your fork**
   
   ```bash
   git clone https://github.com/YOUR_USERNAME/cloudvault.git
   cd cloudvault
   ```

3. **Add upstream remote**
   
   ```bash
   git remote add upstream https://github.com/cloudvault-io/cloudvault.git
   ```

4. **Install dependencies**
   
   ```bash
   make deps
   ```

5. **Create a branch**
   
   ```bash
   git checkout -b feature/my-awesome-feature
   ```

6. **Make your changes**
   
   Write code, add tests, update docs!

7. **Test your changes**
   
   ```bash
   make test
   make fmt
   make lint
   ```

8. **Commit your changes**
   
   ```bash
   git add .
   git commit -m "Add my awesome feature"
   ```

9. **Push to your fork**
   
   ```bash
   git push origin feature/my-awesome-feature
   ```

10. **Open a Pull Request**
    
    Go to GitHub and click "New Pull Request"

## 🎯 What to Contribute

We're actively looking for contributions in these areas:

### High Priority
- 🐛 **Bug fixes** - Found something broken? Fix it!
- 📝 **Documentation** - Improve guides, add examples
- 🧪 **Tests** - Increase test coverage
- 🌐 **Cloud provider support** - Enhance AWS/GCP/Azure integrations

### Medium Priority
- 🎨 **Web UI** - Help build the React dashboard
- 📊 **Visualizations** - Charts, graphs, cost breakdowns
- 🔧 **CLI improvements** - New commands, better UX
- 🤖 **ML models** - Cost prediction algorithms

### Future Work
- 🔄 **Automation** - Storage lifecycle management
- 🌍 **Multi-cluster** - Aggregate costs across clusters
- 📦 **Integrations** - Prometheus, Grafana, Slack, etc.

## 📋 Code Guidelines

### Go Style

We follow standard Go conventions:

- Use `gofmt` for formatting (run `make fmt`)
- Follow [Effective Go](https://golang.org/doc/effective_go.html)
- Add comments for exported functions
- Keep functions small and focused
- Use meaningful variable names

**Example:**
```go
// CalculateMonthlyCost calculates the monthly cost for a PVC based on size and storage class.
// It returns the cost in USD.
func CalculateMonthlyCost(sizeBytes int64, storageClass string) float64 {
    sizeGB := float64(sizeBytes) / (1024 * 1024 * 1024)
    pricePerGB := getPricing(storageClass)
    return sizeGB * pricePerGB
}
```

### Error Handling

- Always handle errors, don't ignore them
- Wrap errors with context using `fmt.Errorf()`
- Return errors instead of panicking (unless truly fatal)

**Example:**
```go
pvcs, err := clientset.CoreV1().PersistentVolumeClaims("").List(ctx, metav1.ListOptions{})
if err != nil {
    return nil, fmt.Errorf("failed to list PVCs: %w", err)
}
```

### Testing

- Add tests for new features
- Aim for 70%+ code coverage
- Use table-driven tests where appropriate

**Example:**
```go
func TestCalculateCost(t *testing.T) {
    tests := []struct {
        name       string
        sizeBytes  int64
        wantCost   float64
    }{
        {"100GB gp3", 100 * 1024 * 1024 * 1024, 8.0},
        {"50GB gp2", 50 * 1024 * 1024 * 1024, 5.0},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := CalculateCost(tt.sizeBytes)
            if got != tt.wantCost {
                t.Errorf("got %.2f, want %.2f", got, tt.wantCost)
            }
        })
    }
}
```

### Commit Messages

Use clear, descriptive commit messages:

**Good:**
```
Add cost calculation for Azure storage classes
Fix bug in zombie volume detection
Update README with installation instructions
```

**Bad:**
```
fix stuff
wip
updates
```

**Format:**
```
<type>: <short summary>

<optional detailed description>

<optional footer>
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `test`: Adding or updating tests
- `refactor`: Code refactoring
- `chore`: Build process, dependencies, etc.

## 🔄 Pull Request Process

1. **Update documentation** if you changed APIs or added features
2. **Add tests** for new functionality
3. **Run all tests** and ensure they pass: `make test`
4. **Run linting**: `make lint`
5. **Update CHANGELOG.md** if significant change
6. **Fill out the PR template** with details
7. **Request review** from maintainers

### PR Checklist

- [ ] Tests pass locally
- [ ] Code is formatted (`make fmt`)
- [ ] Linting passes (`make lint`)
- [ ] Documentation updated
- [ ] Commits are clear and atomic
- [ ] PR description explains the change

### PR Review Process

1. Maintainers will review within 2-3 business days
2. Address any feedback or requested changes
3. Once approved, a maintainer will merge
4. Your contribution will be in the next release!

## 🐛 Bug Reports

Found a bug? Please open an issue with:

**Title:** Clear, concise description

**Description:**
```markdown
## Expected Behavior
What you expected to happen

## Actual Behavior
What actually happened

## Steps to Reproduce
1. Run command X
2. See output Y
3. Error occurs

## Environment
- CloudVault version: v0.1.0
- Kubernetes version: v1.28
- Cloud provider: AWS
- OS: macOS 14.0
```

## 💡 Feature Requests

Have an idea? We'd love to hear it!

Open an issue with:
- **What** you want to build
- **Why** it's valuable
- **How** you envision it working

Or start a [Discussion](https://github.com/cloudvault-io/cloudvault/discussions) to brainstorm!

## 🎓 Learning Resources

New to Kubernetes cost optimization? Check out:

- [Kubernetes Documentation](https://kubernetes.io/docs/)
- [OpenCost Documentation](https://opencost.io/docs/)
- [CNCF FinOps Resources](https://www.cncf.io/blog/finops/)
- [Effective Go](https://golang.org/doc/effective_go.html)

## 📞 Getting Help

- **Questions?** Open a [Discussion](https://github.com/cloudvault-io/cloudvault/discussions)
- **Bug?** Open an [Issue](https://github.com/cloudvault-io/cloudvault/issues)
- **Chat?** Join our Discord (coming soon!)

## 🏆 Recognition

All contributors will be:
- Listed in our [CONTRIBUTORS.md](CONTRIBUTORS.md) file
- Mentioned in release notes
- Eligible for CloudVault swag (coming soon!)

## 📜 Code of Conduct

We follow the [CNCF Code of Conduct](https://github.com/cncf/foundation/blob/master/code-of-conduct.md).

**TL;DR:** Be kind, be respectful, be professional.

## 📝 License

By contributing, you agree that your contributions will be licensed under the [Apache License 2.0](LICENSE).

---

**Thank you for making CloudVault better! 🙏**

Every contribution, no matter how small, makes a difference.

Happy coding! 🚀
