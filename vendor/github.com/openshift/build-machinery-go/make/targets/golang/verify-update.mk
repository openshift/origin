include $(addprefix $(dir $(lastword $(MAKEFILE_LIST))), \
	../../lib/golang.mk \
)

go_files_count :=$(words $(GO_FILES))

verify-gofmt:
	$(info Running `$(GOFMT) $(GOFMT_FLAGS)` on $(go_files_count) file(s).)
	@TMP=$$( mktemp ); \
	$(GOFMT) $(GOFMT_FLAGS) $(GO_FILES) | tee $${TMP}; \
	if [ -s $${TMP} ]; then \
		echo "$@ failed - please run \`make update-gofmt\`"; \
		exit 1; \
	fi;
.PHONY: verify-gofmt

update-gofmt:
	$(info Running `$(GOFMT) $(GOFMT_FLAGS) -w` on $(go_files_count) file(s).)
	@$(GOFMT) $(GOFMT_FLAGS) -w $(GO_FILES)
.PHONY: update-gofmt


# FIXME: go vet needs to use $(GO_MOD_FLAGS) when this is fixed https://github.com/golang/go/issues/35955
# It will be enforced in CI by setting the env var there, so this remains to fix the dev experience
verify-govet:
	$(GO) vet $(GO_MOD_FLAGS) $(GO_PACKAGES)
.PHONY: verify-govet

verify-golint:
	$(GOLINT) $(GO_PACKAGES)
.PHONY: verify-golint
