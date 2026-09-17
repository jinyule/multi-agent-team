.PHONY: build test coverage service-e2e desktop-build desktop-test e2e quick format-check test-rules workflow-check check ci package release-verify hooks run

TEAM_QA_DIR ?= /tmp/multi-agent-team-qa
export TEAM_QA_DIR

build:
	go build -o bin/teamd ./cmd/teamd

test:
	python3 scripts/run-go-tests.py test

coverage:
	python3 scripts/run-go-tests.py coverage
	python3 scripts/quality.py coverage "$(TEAM_QA_DIR)/go-coverage.out" --minimum 80

service-e2e: build
	TEAM_E2E_DAEMON="$(CURDIR)/bin/teamd" python3 scripts/run-go-tests.py e2e

desktop-build:
	npm --prefix desktop run build

desktop-test: build desktop-build
	python3 scripts/run-desktop-e2e.py

e2e: service-e2e desktop-test

format-check:
	python3 scripts/quality.py repo
	cd desktop && npm exec -- prettier --check src e2e main.cjs preload.cjs index.html package.json tsconfig.json vite.config.ts ../scripts/*.mjs ../scripts/tests/*.mjs ../.github ../.prettierrc.json

workflow-check:
	node scripts/workflow-policy.mjs

quick: format-check workflow-check
	go vet ./...
	cd desktop && npm exec -- tsc --noEmit
	node --check desktop/main.cjs
	node --check desktop/preload.cjs

test-rules:
	python3 -m unittest discover -s scripts/tests -v
	node --test scripts/tests/*.test.mjs

check: quick test-rules coverage e2e

ci: check

package:
	python3 scripts/release.py pack

release-verify:
	python3 scripts/release.py verify --allow-dirty

hooks:
	python3 scripts/install-hooks.py

run: build
	./bin/teamd
