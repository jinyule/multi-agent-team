"""Refuse publication until repository environment protection is verified."""
import os


def require_configured(value):
    if value != "true":
        raise ValueError(
            "publishing is disabled until development-release has required reviewers and the repository variable RELEASE_APPROVAL_CONFIGURED is exactly true"
        )


if __name__ == "__main__":
    try:
        require_configured(os.environ.get("RELEASE_APPROVAL_CONFIGURED"))
    except ValueError as error:
        raise SystemExit(str(error)) from error
