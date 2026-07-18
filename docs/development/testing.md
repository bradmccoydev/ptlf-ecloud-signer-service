# Testing

## Test Strategy

The signer service uses two complementary testing approaches:

1. **Property-Based Tests** — Validate universal correctness properties across random inputs
2. **Unit Tests** — Validate specific examples and edge cases

## Running Tests

```bash
# All tests
make test

# Unit tests only
make test-unit

# Property-based tests only
make test-property

# With verbose output
go test -v ./... -count=1

# With race detector
go test -race ./... -count=1
```

## Property-Based Tests

Built with [gopter](https://github.com/leanovate/gopter), each property test runs a minimum of 100 iterations with random inputs.

| Property | File | Validates |
|----------|------|-----------|
| 1. Severity gate monotonicity | `service/gate/evaluator_property_test.go` | Pass implies all vulns < High; any >= High implies fail |
| 2. Severity parsing round-trip | `service/gate/severity_property_test.go` | ParseSeverity → String produces original value |
| 3. Webhook secret validation | `handler/webhook/secret_property_test.go` | Invalid secrets rejected, valid accepted |
| 4. Signing idempotency | `service/signing/service_property_test.go` | Already-signed artifacts produce no new signatures |
| 5. Payload extraction completeness | `handler/webhook/extract_property_test.go` | Valid payloads extract; invalid payloads error |
| 6. Gate decision reason labelling | `service/gate/reason_property_test.go` | Reasons always in valid set |
| 7. Reconciliation skip logic | `service/reconciliation/reconciler_property_test.go` | Signed artifacts are never re-processed |
| 8. Annotation completeness | `pkg/sigstore/signer_property_test.go` | Required annotation keys always present |

## Unit Test Coverage

Key packages with unit tests:

- `internal/config/` — Config loading, validation, env parsing
- `internal/handler/webhook/` — All HTTP response paths (401, 400, 422, 200)
- `internal/handler/health/` — Health, readiness, shutdown flag, dependency checks
- `internal/pkg/harbor/` — Client retry, timeout, signature checking
- `internal/pkg/sigstore/` — Signer flow, Fulcio/Rekor retry, token rejection
- `internal/pkg/workerpool/` — Job execution, queue overflow, shutdown
- `internal/service/reconciliation/` — Skip-if-running, timeout, sweep summary

## Writing New Tests

### Property Tests

```go
func TestPropertyMyProperty(t *testing.T) {
    parameters := gopter.DefaultTestParameters()
    parameters.MinSuccessfulTests = 100

    properties := gopter.NewProperties(parameters)

    properties.Property("description", prop.ForAll(
        func(input string) bool {
            // Property assertion
            return true
        },
        gen.AlphaString(),
    ))

    properties.TestingRun(t)
}
```

### Unit Tests

Follow standard Go testing patterns with table-driven tests where appropriate:

```go
func TestMyFunction(t *testing.T) {
    tests := []struct{
        name string
        input string
        want  string
    }{
        {"case 1", "input", "expected"},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := MyFunction(tt.input)
            if got != tt.want {
                t.Errorf("got %q, want %q", got, tt.want)
            }
        })
    }
}
```
