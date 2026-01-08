# SSH Command Executor

A Go utility for executing commands on multiple SSH hosts using playbooks.

## Features

- Execute commands on multiple hosts in parallel with concurrency control
- Organize commands into reusable playbooks
- Support for password and key-based SSH authentication
- Secure credential handling with encryption
- Configurable timeouts and connection parameters
- Output capture for all command executions

## Installation

```bash
# Clone the repository
git clone https://github.com/DenisFri/SshCommandExecutor.git
cd SshCommandExecutor

# Using Makefile (recommended)
make build         # Builds all executables to bin/ and copies config files
make test          # Runs tests
make deps          # Updates dependencies with go mod tidy
make clean         # Removes the bin directory and cleans Go artifacts
make install       # Installs executables to $GOPATH/bin

# Or build manually
go build -o bin/ssh-executor ./cmd/main.go
go build -o bin/credential-tool ./cmd/credential/main.go
mkdir -p bin/config
cp -r config/* bin/config/
```

## Running the Application

After building, you can run the applications from the bin directory:

```bash
# Navigate to the bin directory
cd bin

# Encrypt credentials
./credential-tool -encrypt -input=../credentials.txt -output=config/credentials.enc

# Run the SSH executor
./ssh-executor -playbooks=config/playbooks.yaml -hosts=config/hosts.yaml
```

## Configuration

### Host Configuration

Define your hosts in `config/hosts.yaml`:

```yaml
hosts:
  - hostname: "host1.example.com"
    playbook: "basic_info"
  - hostname: "host2.example.com"
    playbook: "user_info"
```

### Playbook Configuration

Define your command playbooks in `config/playbooks.yaml`:

```yaml
playbooks:
  - name: "basic_info"
    commands:
      - "uname -a"
      - "df -h"
  - name: "user_info"
    commands:
      - "whoami"
      - "uptime"
```

### Credentials Configuration

Create a credentials file and encrypt it using the included credential tool:

1. Create a plain text file with your credentials:
```
SSH_EXECUTOR_USER=myuser
SSH_EXECUTOR_PASSWORD=mypassword
SSH_KEY_PATH=~/.ssh/id_rsa
SSH_KEY_PASSPHRASE=keypassphrase
```

2. Use the credential tool to encrypt it:
```bash
# Set the encryption password (must be 32 characters for AES-256)
export CREDENTIALS_PASSWORD="your-secret-password-32-characters"

# Encrypt the credentials file
./bin/credential-tool -encrypt -input=credentials.txt -output=config/credentials.enc
```

## Usage

```bash
# Set the decryption password as an environment variable
export CREDENTIALS_PASSWORD="your-secure-password"

# Run with default settings
./bin/ssh-executor

# Run with custom configuration
./bin/ssh-executor -playbooks=/path/to/playbooks.yaml -hosts=/path/to/hosts.yaml -concurrency=5 -timeout=10m
```

### Command-line Options

- `-playbooks`: Path to playbooks YAML file (default: "config/playbooks.yaml")
- `-hosts`: Path to hosts YAML file (default: "config/hosts.yaml")
- `-credentials`: Path to encrypted credentials file (default: "config/credentials.enc")
- `-output`: Output directory for command results (default: "output")
- `-concurrency`: Maximum number of concurrent SSH connections (default: 10)
- `-timeout`: Global execution timeout (default: 5m)

## Security Considerations

- Credentials are encrypted at rest and only decrypted in memory during execution
- Always use SSH key-based authentication in production environments
- Consider implementing host key verification for added security
- Avoid storing SSH passwords in configuration files when possible

## License

[MIT License](LICENSE)
