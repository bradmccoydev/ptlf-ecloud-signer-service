# Contributing

## Development Workflow

1. Create a feature branch from `main`
2. Implement changes with tests
3. Run `make verify` (fmt, vet, lint, test)
4. Push and create a Pull Request
5. CI pipeline runs automatically
6. Get code review approval
7. Merge to `main`

## Code Standards

- Follow standard Go conventions and `gofmt`
- All exported functions must have documentation comments
- New features require property-based tests where applicable
- Minimum 100 iterations for property tests
- Use structured logging (zap) — no `fmt.Println` or `log.*`
- Use interfaces for external dependencies (testability)

## Commit Messages

Follow conventional commits:

```
feat: add webhook retry logic
fix: handle nil scan report gracefully
docs: update configuration reference
test: add property test for gate monotonicity
chore: update cosign dependency to v2.6.4
```

## Pull Request Checklist

- [ ] Code compiles (`go build ./...`)
- [ ] All tests pass (`make test`)
- [ ] No vet issues (`go vet ./...`)
- [ ] New code has tests (unit or property-based)
- [ ] Documentation updated if applicable
- [ ] No secrets or credentials committed

## Release Process

Releases are handled automatically by CI:
1. Push/merge to `main` triggers build
2. Multi-arch Docker image built and pushed
3. Image signed with cosign
4. Helm chart values updated with new tag
