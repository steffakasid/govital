# General instructions
- be concise and clear in your responses.
- provide code snippets when necessary.
- use conventional commit messages for git commits.
- use renovate for dependency management.
- place the renovate configuration file into the root of the project.
- I like to have a dependency dashboard for renovate.
- github action should be updated via renovate (regex manager).

# Coding instructions
- use Golang for all code snippets and code generations. Unless specified, do not use any other programming language.
- follow best practices for Go code, including proper error handling, testing, and documentation.
- I prefer idiomatic Go code 
- use the `go fmt` tool to format your code.
- use net/http for HTTP server implementations.
- use spf13/viper for configuration management.
- use steffakasid/eslog for logging
- use 'cmd' package for command-line
- use 'pkg' package for reusable libraries.
- use 'internal' package for internal libraries.
- put main package into 'cmd' package.
- use Go 1.24.5
- use golangci-lint for linting.

# Testing instructions
- use testify assertions for unit tests.
- use mockery for mocking dependencies in tests.
- place the mockery configuration file into the root of the project.

# What I want you to do
- I want to create a new Go project.
- I want to create a go tool to scan all dependencies of a given Go project and check if those dependencies are actively maintained.
- The tool should also check if the use versions are up to date.
- The tool will be a command-line application. No web interface is needed.

# Build and deployment instructions
- I want to use go releaser for building my project.
- I want to use GitHub Actions for testing and building my project.
- I want to publish my project as a Docker image to Docker Hub.
- I want to use Helm for deploying my project to Kubernetes.
- put helm charts into 'charts' package.
- place the Dockerfile into 'build' package.
- We only need to build for Linux amd64 architecture.
- use Goreleaser for building and publishing the docker image.
- use Goreleaser github action