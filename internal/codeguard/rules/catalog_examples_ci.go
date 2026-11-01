package rules

import "github.com/devr-tools/codeguard/internal/codeguard/core"

// ciWorkflowExamples cover the GitHub Actions workflow-security rules. The
// good snippets pin actions to a full commit SHA so they do not trade one
// finding for ci.mutable-deployment-reference.
var ciWorkflowExamples = map[string][]core.RuleExample{
	"ci.workflow-script-injection": {yamlExample(".github/workflows/triage.yml", `on: issues
permissions:
  contents: read
jobs:
  triage:
    runs-on: ubuntu-latest
    steps:
      - run: echo "${{ github.event.issue.title }}"
`, `on: issues
permissions:
  contents: read
jobs:
  triage:
    runs-on: ubuntu-latest
    steps:
      - env:
          TITLE: ${{ github.event.issue.title }}
        run: echo "$TITLE"
`)},
	"ci.workflow-dangerous-trigger": {yamlExample(".github/workflows/pr.yml", `on: pull_request_target
permissions:
  contents: read
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@8f3c2b1a4d5e6f7890abc1234567890abc123456
        with:
          ref: ${{ github.event.pull_request.head.sha }}
      - run: make test
`, `on: pull_request
permissions:
  contents: read
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@8f3c2b1a4d5e6f7890abc1234567890abc123456
        with:
          ref: ${{ github.event.pull_request.head.sha }}
      - run: make test
`)},
	"ci.workflow-write-all-permissions": {yamlExample(".github/workflows/release.yml", `on: push
permissions: write-all
jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - run: make release
`, `on: push
permissions:
  contents: write
jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - run: make release
`)},
	"ci.workflow-missing-permissions": {yamlExample(".github/workflows/build.yml", `on: push
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - run: make
`, `on: push
permissions:
  contents: read
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - run: make
`)},
	"ci.workflow-remote-script-pipe": {yamlExample(".github/workflows/setup.yml", `on: push
permissions:
  contents: read
jobs:
  setup:
    runs-on: ubuntu-latest
    steps:
      - run: curl -fsSL https://example.com/install.sh | bash
`, `on: push
permissions:
  contents: read
jobs:
  setup:
    runs-on: ubuntu-latest
    steps:
      - run: |
          curl -fsSLo install.sh https://example.com/install.sh
          echo "<expected-sha256>  install.sh" | sha256sum -c -
          bash install.sh
`)},
	"ci.workflow-fork-trigger-secrets": {yamlExample(".github/workflows/deploy.yml", `on: pull_request_target
permissions:
  contents: read
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - run: ./deploy.sh
        env:
          TOKEN: ${{ secrets.DEPLOY_TOKEN }}
`, `on:
  push:
    branches: [main]
permissions:
  contents: read
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - run: ./deploy.sh
        env:
          TOKEN: ${{ secrets.DEPLOY_TOKEN }}
`)},
}
