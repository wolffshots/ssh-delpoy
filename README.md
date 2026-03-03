# ssh-deploy

A minimal SSH command service built with [charmbracelet/wish](https://github.com/charmbracelet/wish) for GitHub Actions-triggered deployments.

It only permits these remote actions:

- `deploy` (internally runs `docker compose pull` then `docker compose up -d`)
- `ps` (runs `docker compose ps`)
- `logs <service>` (runs `docker compose logs --tail <LOGS_TAIL> <service>`)

For compatibility with existing callers, it also accepts exact command forms:

- `docker compose pull && docker compose up`
- `docker compose ps`
- `docker compose logs <service>`

Anything else is rejected.

## Security model

- SSH public-key authentication only, via `SSH_AUTHORIZED_KEYS_PATH`.
- No shell execution (`sh -c`) is used.
- Only strict allowlisted commands are parsed and mapped to fixed `docker compose` invocations.
- `logs` requires a service in `ALLOWED_LOG_SERVICES`.
- Optional strict host key verification in GitHub Actions workflow via pinned `known_hosts`.

## Environment variables

| Variable | Default | Description |
|---|---|---|
| `SSH_LISTEN_HOST` | `0.0.0.0` | Interface to bind |
| `SSH_LISTEN_PORT` | `2222` | SSH port |
| `SSH_AUTHORIZED_KEYS_PATH` | `/app/data/authorized_keys` | Authorized keys file |
| `SSH_HOST_KEY_PATH` | `/app/data/ssh_host_ed25519` | SSH host key path (auto-generated if absent) |
| `DEPLOY_COMPOSE_PROJECT_DIR` | `/srv/target` | Directory where `docker compose` runs |
| `DEPLOY_COMPOSE_FILE` | `docker-compose.yml` | Compose file used inside `DEPLOY_COMPOSE_PROJECT_DIR` |
| `TARGET_COMPOSE_PROJECT_DIR` | `.` | Host path mounted at `/srv/target` in container |
| `TARGET_COMPOSE_FILE` | `docker-compose.yml` | Injected into `DEPLOY_COMPOSE_FILE` in `docker-compose.yml` |
| `ALLOWED_LOG_SERVICES` | *(empty)* | Comma-separated allowlist for `logs` |
| `LOGS_TAIL` | `300` | Tail line count for `logs` |
| `COMMAND_TIMEOUT` | `10m` | Max time per remote action |
| `SSH_IDLE_TIMEOUT` | `2m` | SSH idle timeout |
| `SSH_MAX_TIMEOUT` | `15m` | Max SSH connection duration |

Avoid setting generic `COMPOSE_FILE` / `COMPOSE_PROJECT_DIR` in `.env` for this stack; those names are reserved by Docker Compose and can cause command resolution issues.

## Run in Docker

1. Create `data/authorized_keys` and add deploy keys.
2. Point `TARGET_COMPOSE_PROJECT_DIR` to the stack you want to control.
3. Start:

```bash
docker compose up --build -d
```

Example local test:

```bash
ssh -p 2222 -i ~/.ssh/deploy_key deploy@localhost deploy
ssh -p 2222 -i ~/.ssh/deploy_key deploy@localhost ps
ssh -p 2222 -i ~/.ssh/deploy_key deploy@localhost "logs api"
```

## GitHub Actions workflow

Workflow template: `.github/workflows/ssh-deploy.yml`

Required secrets:

- `DEPLOY_SSH_PRIVATE_KEY`: private key used by Actions
- `DEPLOY_SSH_KNOWN_HOSTS_LINE`: pinned known_hosts line (required when strict checking enabled)

Suggested repository variables:

- `DEPLOY_SSH_HOST`
- `DEPLOY_SSH_PORT`
- `DEPLOY_SSH_USER`
- `DEPLOY_HOSTNAME_CHECK` (`true` or `false`)

When `DEPLOY_HOSTNAME_CHECK=true`, the workflow uses:

- `StrictHostKeyChecking=yes`
- `UserKnownHostsFile=~/.ssh/known_hosts`

This enforces hostname/host-key verification and fails on mismatches.

## Using from another app (minimum workflow footprint)

If another repository already built and pushed a new image, the deploy part can be a single SSH command to this service:

```yaml
- name: Deploy via ssh-deploy
	env:
		DEPLOY_SSH_HOST: ${{ vars.DEPLOY_SSH_HOST }}
		DEPLOY_SSH_PORT: ${{ vars.DEPLOY_SSH_PORT || '2222' }}
		DEPLOY_SSH_USER: ${{ vars.DEPLOY_SSH_USER || 'deploy' }}
		DEPLOY_SSH_PRIVATE_KEY: ${{ secrets.DEPLOY_SSH_PRIVATE_KEY }}
		DEPLOY_SSH_KNOWN_HOSTS_LINE: ${{ secrets.DEPLOY_SSH_KNOWN_HOSTS_LINE }}
	run: |
		set -euo pipefail
		mkdir -p "$HOME/.ssh"
		chmod 700 "$HOME/.ssh"
		printf '%s\n' "$DEPLOY_SSH_PRIVATE_KEY" > "$HOME/.ssh/deploy_key"
		chmod 600 "$HOME/.ssh/deploy_key"
		printf '%s\n' "$DEPLOY_SSH_KNOWN_HOSTS_LINE" > "$HOME/.ssh/known_hosts"
		chmod 600 "$HOME/.ssh/known_hosts"

		ssh \
			-o StrictHostKeyChecking=yes \
			-o UserKnownHostsFile="$HOME/.ssh/known_hosts" \
			-i "$HOME/.ssh/deploy_key" \
			-p "$DEPLOY_SSH_PORT" \
			"$DEPLOY_SSH_USER@$DEPLOY_SSH_HOST" \
			deploy
```

That `deploy` command executes `docker compose pull` then `docker compose up -d` on the target stack through `ssh-deploy`.

## One-time setup to get usable

1. Generate a dedicated keypair for GitHub Actions:

```bash
ssh-keygen -t ed25519 -C "github-actions-deploy" -f ./gha_deploy_key -N ""
```

2. On the deploy host (where `ssh-deploy` runs), append the public key to `data/authorized_keys`:

```bash
cat ./gha_deploy_key.pub >> /path/to/ssh-deploy/data/authorized_keys
chmod 600 /path/to/ssh-deploy/data/authorized_keys
```

3. Start `ssh-deploy` in the same compose environment as the target stack (or with access to that stack path and docker socket):

```bash
cd /path/to/ssh-deploy
TARGET_COMPOSE_PROJECT_DIR=/path/to/target/stack docker compose up -d --build
```

4. Capture the SSH host key line for strict verification and store it as a secret in the app repo:

```bash
ssh-keyscan -p 2222 your.deploy.host
```

Save one output line as `DEPLOY_SSH_KNOWN_HOSTS_LINE`.

5. In the app repository that will trigger deploys, set:

- Repository variables: `DEPLOY_SSH_HOST`, `DEPLOY_SSH_PORT`, `DEPLOY_SSH_USER`
- Repository secrets: `DEPLOY_SSH_PRIVATE_KEY` (contents of `gha_deploy_key`), `DEPLOY_SSH_KNOWN_HOSTS_LINE`

6. In that app workflow, run your image build/push first, then run the minimal deploy step above.

After this, each successful image build can trigger a single remote `deploy` call.

## Important risk note

The provided Docker setup mounts `/var/run/docker.sock` so the service can run `docker compose` on the host. This gives effectively root-equivalent control over the host Docker daemon. Restrict network access and only use trusted SSH keys.
