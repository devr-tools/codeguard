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
	"ci.workflow-cache-poisoning": {yamlExample(".github/workflows/release.yml", `on:
  push:
    tags: ["v*"]
permissions:
  contents: write
jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/setup-go@8f3c2b1a4d5e6f7890abc1234567890abc123456 # v5
        with:
          go-version-file: go.mod
      - run: make release
`, `on:
  push:
    tags: ["v*"]
permissions:
  contents: write
jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/setup-go@8f3c2b1a4d5e6f7890abc1234567890abc123456 # v5
        with:
          go-version-file: go.mod
          cache: false
      - run: make release
`)},
	"ci.workflow-credential-persistence": {yamlExample(".github/workflows/bundle.yml", `on: push
permissions:
  contents: read
jobs:
  bundle:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@8f3c2b1a4d5e6f7890abc1234567890abc123456 # v4
      - uses: actions/upload-artifact@8f3c2b1a4d5e6f7890abc1234567890abc123456 # v4
        with:
          name: source
          path: .
          include-hidden-files: true
`, `on: push
permissions:
  contents: read
jobs:
  bundle:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@8f3c2b1a4d5e6f7890abc1234567890abc123456 # v4
        with:
          persist-credentials: false
      - uses: actions/upload-artifact@8f3c2b1a4d5e6f7890abc1234567890abc123456 # v4
        with:
          name: source
          path: .
          include-hidden-files: true
`)},
	"ci.workflow-env-file-injection": {yamlExample(".github/workflows/report.yml", `on:
  workflow_run:
    workflows: [ci]
    types: [completed]
permissions:
  contents: read
jobs:
  report:
    runs-on: ubuntu-latest
    steps:
      - env:
          BRANCH: ${{ github.event.workflow_run.head_branch }}
        run: echo "BRANCH=$BRANCH" >> "$GITHUB_ENV"
`, `on:
  workflow_run:
    workflows: [ci]
    types: [completed]
permissions:
  contents: read
jobs:
  report:
    runs-on: ubuntu-latest
    steps:
      - id: branch
        env:
          BRANCH: ${{ github.event.workflow_run.head_branch }}
        run: echo "name=$BRANCH" >> "$GITHUB_OUTPUT"
`)},
	"ci.workflow-overprovisioned-secrets": {yamlExample(".github/workflows/deploy.yml", `on: push
permissions:
  contents: read
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - run: ./deploy.sh
        env:
          ALL_SECRETS: ${{ toJSON(secrets) }}
`, `on: push
permissions:
  contents: read
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - run: ./deploy.sh
        env:
          DEPLOY_TOKEN: ${{ secrets.DEPLOY_TOKEN }}
`)},
	"ci.workflow-unpinned-image": {yamlExample(".github/workflows/test.yml", `on: push
permissions:
  contents: read
jobs:
  test:
    runs-on: ubuntu-latest
    container: node:20
    steps:
      - run: npm test
`, `on: push
permissions:
  contents: read
jobs:
  test:
    runs-on: ubuntu-latest
    container: node:20@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef # 20
    steps:
      - run: npm test
`)},
	"ci.workflow-insecure-commands": {yamlExample(".github/workflows/legacy.yml", `on: push
permissions:
  contents: read
env:
  ACTIONS_ALLOW_UNSECURE_COMMANDS: true
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - run: echo "::set-env name=MODE::release"
`, `on: push
permissions:
  contents: read
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - run: echo "MODE=release" >> "$GITHUB_ENV"
`)},
	"ci.workflow-bot-conditions": {yamlExample(".github/workflows/automerge.yml", `on: pull_request_target
permissions:
  contents: read
jobs:
  automerge:
    runs-on: ubuntu-latest
    if: github.actor == 'dependabot[bot]'
    steps:
      - run: gh pr merge --auto --squash "$PR_URL"
        env:
          PR_URL: ${{ github.event.pull_request.html_url }}
`, `on: pull_request_target
permissions:
  contents: read
jobs:
  automerge:
    runs-on: ubuntu-latest
    if: github.event.pull_request.user.login == 'dependabot[bot]'
    steps:
      - run: gh pr merge --auto --squash "$PR_URL"
        env:
          PR_URL: ${{ github.event.pull_request.html_url }}
`)},
}
