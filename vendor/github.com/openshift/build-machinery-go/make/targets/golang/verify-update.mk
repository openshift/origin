include $(addprefix $(dir $(lastword $(MAKEFILE_LIST))), \
	../../lib/golang.mk \
)

go_files_count :=$(words $(GO_FILES))
chunk_size :=1000

verify-gofmt:
	$(info Running `$(GOFMT) $(GOFMT_FLAGS)` on $(go_files_count) file(s).)
	@TMP=$$( mktemp ); \
	find . -name '*.go' -not -path '*/vendor/*' -not -path '*/_output/*' -print | xargs -n $(chunk_size) $(GOFMT) $(GOFMT_FLAGS) | tee $${TMP}; \
	if [ -s $${TMP} ]; then \
		echo "$@ failed - please run \`make update-gofmt\`"; \
		exit 1; \
	fi;
.PHONY: verify-gofmt

update-gofmt:
	$(info Running `$(GOFMT) $(GOFMT_FLAGS) -w` on $(go_files_count) file(s).)
	@find . -name '*.go' -not -path '*/vendor/*' -not -path '*/_output/*' -print | xargs -n $(chunk_size) $(GOFMT) $(GOFMT_FLAGS) -w
.PHONY: update-gofmt


# FIXME: go vet needs to use $(GO_MOD_FLAGS) when this is fixed https://github.com/golang/go/issues/35955
# It will be enforced in CI by setting the env var there, so this remains to fix the dev experience
verify-govet:
	$(GO) vet $(GO_MOD_FLAGS) $(GO_PACKAGES)
.PHONY: verify-govet

verify-golint:
	$(GOLINT) $(GO_PACKAGES)
.PHONY: verify-golint
