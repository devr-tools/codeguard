package rules

import "github.com/devr-tools/codeguard/internal/codeguard/core"

var ciWorkflowFixTemplates = map[string]core.FixTemplate{
	"ci.workflow-script-injection": {
		Kind: deterministic,
		Text: "Move the untrusted expression into env: and reference the variable, quoted, from the script.\n\nBefore:\n- run: echo \"${{ github.event.pull_request.title }}\"\n\nAfter:\n- env:\n    PR_TITLE: ${{ github.event.pull_request.title }}\n  run: echo \"$PR_TITLE\"\n\n# actions/github-script: read process.env.PR_TITLE instead of interpolating into script:",
	},
	"ci.workflow-dangerous-trigger": {
		Kind: guided,
		Text: "Do not run pull request code in a privileged workflow. Split it: an unprivileged pull_request workflow builds and tests, and the privileged one only consumes its results.\n\nBefore:\non: pull_request_target\njobs:\n  test:\n    steps:\n      - uses: actions/checkout@<sha>\n        with:\n          ref: ${{ github.event.pull_request.head.sha }}\n      - run: make test\n\nAfter:\non: pull_request\npermissions:\n  contents: read\njobs:\n  test:\n    steps:\n      - uses: actions/checkout@<sha>\n      - run: make test",
	},
	"ci.workflow-write-all-permissions": {
		Kind: guided,
		Text: "Replace write-all with the scopes the workflow needs.\n\nBefore:\npermissions: write-all\n\nAfter:\npermissions:\n  contents: read\njobs:\n  release:\n    permissions:\n      contents: write",
	},
	"ci.workflow-missing-permissions": {
		Kind: deterministic,
		Text: "Declare a read-only default for the token and widen per job.\n\nBefore:\non: push\njobs:\n  build:\n    runs-on: ubuntu-latest\n\nAfter:\non: push\npermissions:\n  contents: read\njobs:\n  build:\n    runs-on: ubuntu-latest",
	},
	"ci.workflow-remote-script-pipe": {
		Kind: guided,
		Text: "Download to a file, verify it against a pinned checksum, then run it.\n\nBefore:\n- run: curl -fsSL https://example.com/install.sh | bash\n\nAfter:\n- run: |\n    curl -fsSLo install.sh https://example.com/v1.2.3/install.sh\n    echo \"<sha256>  install.sh\" | sha256sum -c -\n    bash install.sh",
	},
	"ci.workflow-fork-trigger-secrets": {
		Kind: guided,
		Text: "Keep secrets out of jobs any fork can trigger, or require approval first.\n\nBefore:\non: pull_request_target\njobs:\n  label:\n    steps:\n      - run: ./notify.sh\n        env:\n          TOKEN: ${{ secrets.DEPLOY_TOKEN }}\n\nAfter:\non: pull_request_target\njobs:\n  label:\n    environment: external-pr   # requires reviewer approval\n    steps:\n      - run: ./notify.sh\n        env:\n          TOKEN: ${{ secrets.DEPLOY_TOKEN }}",
	},
	"ci.workflow-cache-poisoning": {
		Kind: deterministic,
		Text: "Turn the cache off in the release job.\n\nBefore:\n- uses: actions/setup-go@<sha> # v5\n  with:\n    go-version-file: go.mod\n\nAfter:\n- uses: actions/setup-go@<sha> # v5\n  with:\n    go-version-file: go.mod\n    cache: false",
	},
	"ci.workflow-credential-persistence": {
		Kind: deterministic,
		Text: "Stop checkout from writing the token into .git/config.\n\nBefore:\n- uses: actions/checkout@<sha> # v4\n\nAfter:\n- uses: actions/checkout@<sha> # v4\n  with:\n    persist-credentials: false",
	},
	"ci.workflow-env-file-injection": {
		Kind: guided,
		Text: "Pass the value through an output instead of the environment file, or validate it first.\n\nBefore:\n- run: echo \"BRANCH=${{ github.event.workflow_run.head_branch }}\" >> \"$GITHUB_ENV\"\n\nAfter:\n- id: branch\n  env:\n    BRANCH: ${{ github.event.workflow_run.head_branch }}\n  run: |\n    [[ \"$BRANCH\" =~ ^[A-Za-z0-9._/-]+$ ]] || exit 1\n    echo \"name=$BRANCH\" >> \"$GITHUB_OUTPUT\"",
	},
	"ci.workflow-overprovisioned-secrets": {
		Kind: guided,
		Text: "Name the secrets the step needs.\n\nBefore:\n- env:\n    ALL_SECRETS: ${{ toJSON(secrets) }}\n  run: ./deploy.sh\n\nAfter:\n- env:\n    DEPLOY_TOKEN: ${{ secrets.DEPLOY_TOKEN }}\n  run: ./deploy.sh",
	},
	"ci.workflow-unpinned-image": {
		Kind: deterministic,
		Text: "Pin the image by digest and keep the tag as a comment.\n\nBefore:\ncontainer: node:20\n\nAfter:\ncontainer: node:20@sha256:<digest> # 20",
	},
	"ci.workflow-insecure-commands": {
		Kind: deterministic,
		Text: "Remove the opt-in and use environment files.\n\nBefore:\nenv:\n  ACTIONS_ALLOW_UNSECURE_COMMANDS: true\nsteps:\n  - run: echo \"::set-env name=MODE::release\"\n\nAfter:\nsteps:\n  - run: echo \"MODE=release\" >> \"$GITHUB_ENV\"",
	},
	"ci.workflow-bot-conditions": {
		Kind: deterministic,
		Text: "Check the pull request author rather than the latest actor.\n\nBefore:\nif: github.actor == 'dependabot[bot]'\n\nAfter:\nif: github.event.pull_request.user.login == 'dependabot[bot]'",
	},
}
