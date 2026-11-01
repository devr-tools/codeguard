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
}
