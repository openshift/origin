This file provides guidance to AI agents when working with code in this repository.

This is the OpenShift API repository - the canonical location of OpenShift API type definitions and serialization code. It contains:

- API type definitions for OpenShift-specific resources (Custom Resource Definitions)
- FeatureGate management system for controlling API availability across cluster profiles
- Generated CRD manifests and validation schemas
- Integration test suite for API validation

## Key Architecture Components

### FeatureGate System
The FeatureGate system (`features/features.go`) controls API availability across different cluster profiles (Hypershift, SelfManaged) and feature sets (Default, TechPreview, DevPreview). Each API feature is gated behind a FeatureGate that can be enabled/disabled per cluster profile and feature set.

### API Structure
APIs are organized by group and version (e.g., `route/v1`, `config/v1`). Each API group contains:
- `types.go` - Go type definitions
- `zz_generated.*` files - Generated code (deepcopy, CRDs, etc.)
- `tests/` directories - Integration test definitions
- CRD manifest files

## Common Development Commands

### Building
```bash
make build              # Build render and write-available-featuresets binaries
make clean              # Clean build artifacts
```

### Code Generation
```bash
make update             # Alias for update-codegen-crds
```

#### Targeted Code Generation
When working on a specific API group/version, you can regenerate only the affected CRDs instead of all CRDs:

```bash
# Regenerate CRDs for a specific API group/version
make update-codegen-crds API_GROUP_VERSIONS=operator.openshift.io/v1alpha1
make update-codegen-crds API_GROUP_VERSIONS=config.openshift.io/v1
make update-codegen-crds API_GROUP_VERSIONS=route.openshift.io/v1

# Multiple API groups can be specified with comma separation
make update-codegen-crds API_GROUP_VERSIONS=operator.openshift.io/v1alpha1,config.openshift.io/v1
```

This is more efficient than running `make update` (which regenerates all CRDs) when you're only working on specific API groups.

### Testing
```bash
make test-unit          # Run unit tests
make integration        # Run integration tests (in tests/ directory)
go test -v ./...        # Run tests for specific packages

# Run integration tests for specific API groups
make -C config/v1 test  # Run tests for config/v1 API group
make -C route/v1 test   # Run tests for route/v1 API group
make -C operator/v1 test # Run tests for operator/v1 API group
```

### Validation and Verification
```bash
make verify             # Run all verification checks
make verify-scripts     # Verify generated code is up to date
make verify-codegen-crds # Verify CRD generation is current
make lint               # Run golangci-lint (only on changes from master)
make lint-fix           # Auto-fix linting issues where possible
```

## Adding New APIs

All APIs should start as tech preview.
New fields on stable APIs should be introduced behind a feature gate `+openshift:enable:FeatureGate=MyFeatureGate`.


### For New Stable APIs (v1)
1. Create the API type with proper kubebuilder annotations
2. Include required markers like `+openshift:compatibility-gen:level=1`
3. Add validation tests in `<group>/<version>/tests/<crd-name>/`
4. Run `make update-codegen-crds` to generate CRDs

### For New TechPreview APIs (v1alpha1)
1. First add a FeatureGate in `features/features.go`
2. Create the API type with `+openshift:enable:FeatureGate=MyFeatureGate`
3. Add corresponding test files
4. Run generation commands

### Adding FeatureGates
Add to `features/features.go` using the builder pattern:
```go
FeatureGateMyFeatureName = newFeatureGate("MyFeatureName").
    reportProblemsToJiraComponent("my-jira-component").
    contactPerson("my-team-lead").
    productScope(ocpSpecific).
    enableIn(configv1.TechPreviewNoUpgrade).
    mustRegister()
```

## Testing Framework

The repository includes a comprehensive integration test suite in `tests/`. Test suites are defined in `*.testsuite.yaml` files alongside API definitions and support:
- `onCreate` tests for validation during resource creation
- `onUpdate` tests for update-specific validations and immutability
- Status subresource testing
- Validation ratcheting tests using `initialCRDPatches`

Use `tests/hack/gen-minimal-test.sh $FOLDER $VERSION` to generate test suite templates.

## Container-based Development
```bash
make verify-with-container    # Run verification in container
make generate-with-container  # Run code generation in container
```

Uses `podman` by default, set `RUNTIME=docker` or `USE_DOCKER=1` to use Docker instead.

## Custom Claude Code Commands

### API Review
```
/api-review <pr-url>
```
Runs comprehensive API review for OpenShift API changes in a GitHub PR:
- Executes `make lint` to check for kube-api-linter issues
- Validates that all API fields are properly documented
- Ensures optional fields explain behavior when not present
- Confirms validation rules and kubebuilder markers are documented in field comments

#### Documentation Requirements
All kubebuilder validation markers must be documented in the field's comment. For example:

**Good:**
```go
// internalDNSRecords is an optional field that determines whether we deploy
// with internal records enabled for api, api-int, and ingress.
// Valid values are "Enabled" and "Disabled".
// When set to Enabled, in cluster DNS resolution will be enabled for the api, api-int, and ingress endpoints.
// When set to Disabled, in cluster DNS resolution will be disabled and an external DNS solution must be provided for these endpoints.
// +optional
// +kubebuilder:validation:Enum=Enabled;Disabled
InternalDNSRecords InternalDNSRecordsType `json:"internalDNSRecords"`
```

**Bad:**
```go
// internalDNSRecords determines whether we deploy with internal records enabled for
// api, api-int, and ingress.
// +optional  // ❌ Optional nature not documented in comment
// +kubebuilder:validation:Enum=Enabled;Disabled  // ❌ Valid values not documented
InternalDNSRecords InternalDNSRecordsType `json:"internalDNSRecords"`
```

#### Systematic Validation Marker Documentation Checklist

**MANDATORY**: For each field with validation markers, verify the comment documents ALL of the following that apply:

**Field Optionality:**
- [ ] `+optional` - explain behavior when field is omitted
- [ ] `+required` - explain that the field is required

**String/Array Length Constraints:**
- [ ] `+kubebuilder:validation:MinLength` and `+kubebuilder:validation:MaxLength` - document character length constraints
- [ ] `+kubebuilder:validation:MinItems` and `+kubebuilder:validation:MaxItems` - document item count ranges

**Value Constraints:**
- [ ] `+kubebuilder:validation:Enum` - list all valid enum values and their meanings
- [ ] `+kubebuilder:validation:Pattern` - explain the pattern requirement in human-readable terms
- [ ] `+kubebuilder:validation:Minimum` and `+kubebuilder:validation:Maximum` - document numeric ranges

**Advanced Validation:**
- [ ] `+kubebuilder:validation:XValidation` - explain cross-field validation rules in detail
- [ ] Any custom validation logic - document the validation behavior

#### API Review Process

**CRITICAL PROCESS**: Follow this exact order to ensure comprehensive validation:

1. **Linting Check**: Run `make lint` and fix all kubeapilinter errors first
2. **Extract Validation Markers**: Use systematic search to find all markers
3. **Systematic Documentation Review**: For each marker found, verify corresponding documentation exists
4. **Optional Fields Review**: Ensure every `+optional` field explains omitted behavior
5. **Cross-field Validation**: Verify any documented field relationships have corresponding `XValidation` rules

**FAILURE CONDITIONS**: The review MUST fail if any of these are found:
- Any validation marker without corresponding documentation
- Any `+optional` field without omitted behavior explanation
- Any documented field constraint without enforcement via validation rules
- Any `make lint` failures

The comment must explicitly state:
- When a field is optional (for `+kubebuilder:validation:Optional` or `+optional`)
- Valid enum values (for `+kubebuilder:validation:Enum`)
- Validation constraints (for min/max, patterns, etc.)
- Default behavior when field is omitted
- Any interactions with other fields, commonly implemented with `+kubebuilder:validation:XValidation`

**CRITICAL**: When API documentation states field relationships or constraints (e.g., "cannot be used together with field X", "mutually exclusive with field Y"), these relationships MUST be enforced with appropriate validation rules. Use `+kubebuilder:validation:XValidation` with CEL expressions for cross-field constraints. Documentation without enforcement is insufficient and will fail review.

Example: `/api-review https://github.com/openshift/api/pull/1234`
