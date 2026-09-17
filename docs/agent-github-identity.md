# Agent GitHub identity

The repository uses the private GitHub App `multi-agent-team-agent` as the independent identity for automated branches, commits, and pull requests. The installation is limited to `jinyule/multi-agent-team`.

The App receives only the repository permissions required by the development workflow:

- read Actions and Checks results;
- write repository contents, pull requests, and workflow files;
- read repository metadata, which GitHub grants to every App.

Installation access tokens are short-lived. The private key and generated tokens remain in the local application credentials directory and must never be committed, logged, or placed in pull-request content.

Agent-authored changes still pass the protected-branch checks. A human owner reviews the resulting pull request and makes the final merge decision.
