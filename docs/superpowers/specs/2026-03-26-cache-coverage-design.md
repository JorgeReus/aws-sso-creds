# Cache Coverage Design

## Goal

Raise coverage for `github.com/JorgeReus/aws-sso-creds/internal/pkg/cache` above 80% by improving and extending the package tests, with a preference for table-driven tests where they make the coverage targets clearer and easier to maintain.

## Scope

This design covers:

- refactoring focused tests in `internal/pkg/cache/cache_test.go` into table-driven form where appropriate
- adding a small number of new edge-case tests for uncovered branches in `internal/pkg/cache/cache.go`
- verifying package coverage exceeds 80%

This design does not cover:

- broad test refactors outside `internal/pkg/cache/cache_test.go`
- production code changes unless a narrow testability issue forces them
- raising coverage significantly beyond what is needed for the user’s stated target

## Current State

The cache package currently has passing tests and package coverage at 79.3%.

The most relevant uncovered or partially covered areas are:

- malformed JSON handling in `GetSSOClientCreds`
- malformed JSON handling in `GetSSOToken`
- some save or file-handling edge paths

The existing tests are mostly individually structured. Some of them already share setup patterns that fit naturally into table-driven organization.

## Chosen Approach

Keep the implementation minimal and test-focused:

1. convert the behavior-oriented credential and token retrieval tests into table-driven groups where inputs and expected outputs vary by case
2. add only the missing cases needed to cover real branches that are currently untested
3. stop once package coverage is above 80%

This keeps the change small while still leaving the cache tests in a better shape than they are now.

## Alternatives Considered

### 1. Recommended: minimal table-driven expansion around retrieval logic

Pros:

- smallest code change
- directly targets the coverage gap
- improves test readability without over-abstracting

Cons:

- does not fully normalize every test in the file

### 2. Convert the entire file to one large table-driven suite

Pros:

- maximal structural consistency

Cons:

- more churn than needed
- risks making different behaviors harder to read by forcing them into one pattern

### 3. Add only new tests and leave existing structure unchanged

Pros:

- fastest path to the coverage target

Cons:

- misses the requested test-shape improvement
- leaves the file with mixed patterns that are less intentional

## Test Design

### Table-Driven Groups

Use separate table-driven tests for the two main retrieval functions:

- `GetSSOClientCreds`
- `GetSSOToken`

Each table should vary:

- file contents or saved fixture state
- expiration state
- validation result for tokens when applicable
- expected return value and error behavior

This keeps each table centered on one function and avoids a single oversized matrix.

### New Coverage Cases

Add targeted cases for:

- invalid JSON in the client credentials cache file
- invalid JSON in the token cache file

If coverage is still below 80% after those cases and the table-driven refactor, add one more small scenario for a real uncovered branch, preferably a save or file-handling error path that can be triggered cleanly in a temp directory.

### Boundaries

- Prefer temp-dir-backed tests over mocks when file IO behavior is the thing being exercised.
- Reuse existing function-level seams like `validateToken` overrides where needed.
- Do not rewrite unrelated tests into tables if that reduces clarity.

## Error Handling

Tests should assert the exact behavioral contract that matters:

- whether an error is returned
- whether the returned cache object is nil or populated
- whether expired files are truncated when that is part of the function behavior

The tests should avoid brittle assertions on irrelevant implementation details.

## Verification Strategy

Validation for this change consists of:

- running `go test ./internal/pkg/cache`
- running `go test ./internal/pkg/cache -coverprofile=/tmp/cache.cover`
- confirming `go tool cover -func=/tmp/cache.cover` reports package statement coverage above 80%

## Files Expected To Change

- `internal/pkg/cache/cache_test.go`

Production files should remain unchanged unless a narrow, justified testability issue is discovered during implementation.
