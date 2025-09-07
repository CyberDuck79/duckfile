# Duckfile Project Documentation
The specifications for the Duckfile project are located in the `docs/spec.md` file.
The presentation and onboarding materials can be found in the `README.md` file.
The json schemas for the Duckfile project can be found in the `docs` folder.

# Instructions for Agent
When asked to plan changes, please provide a summary of what you understand from the request, ask for clarification if you are unsure about any aspect of the request.
If the plan involves significant changes, do not give code snippets and instead provide a high-level overview of the changes.
When you perform a modification, please ensure that you test your changes thoroughly, explain each test case with a comment.
Additionally, make sure to update any relevant documentation to reflect your changes.
`internal/run/test_seams.go`, `cmd/duck/wizard.go`, `cmd/duck/init.go` and `cmd/duck/add.go` are excluded from code coverage reports.

# Git Guidelines
When working with Git, please follow these guidelines:
Write clear and concise commit messages that accurately describe the changes made.
The commit message should respect the conventional commit format, it is used to automate versioning and changelog generation.
Use github mcp for managing commit, pull requests and issues.
Ensure that your code passes all tests before submitting a pull request.
Rebase your branch onto the latest version of the main branch before merging.