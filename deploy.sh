#!/usr/bin/env bash
set -euo pipefail

# ─── Colors & Symbols ────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
BOLD='\033[1m'
DIM='\033[2m'
RESET='\033[0m'

TICK="✓"
CROSS="✗"
SPINNER_FRAMES=(⠋ ⠙ ⠹ ⠸ ⠼ ⠴ ⠦ ⠧ ⠇ ⠏)

TOTAL_STEPS=6
CURRENT_STEP=0
LOG_FILE="/tmp/deploy-desent-$$.log"
DEPLOY_KEY=~/.ssh/desent_deploy

# ─── Cleanup ─────────────────────────────────────────────────────────
cleanup() {
    tput cnorm 2>/dev/null || true
    rm -f /tmp/deploy-compose-$$ /tmp/deploy-step-$$ /tmp/deploy-rc-$$ 2>/dev/null || true
}
trap cleanup EXIT

# ─── Terminal UX ─────────────────────────────────────────────────────
banner() {
    echo ""
    printf "${BOLD}${CYAN}"
    cat <<'ART'
     _                      _
  __| | ___  ___  ___ _ __ | |_
 / _` |/ _ \/ __|/ _ \ '_ \| __|
| (_| |  __/\__ \  __/ | | | |_
 \__,_|\___||___/\___|_| |_|\__|

ART
    printf "${RESET}"
    printf "${DIM}  Decentralized Streaming Platform — Deploy${RESET}\n"
    echo ""
}

fatal() {
    tput cnorm 2>/dev/null || true
    printf "\n  ${RED}${BOLD}Fatal:${RESET} %s\n\n" "$1"
    exit 1
}

# ─── Step runner with live collapsing output ─────────────────────────
#
# Shows a scrolling window of the last N output lines while running.
# When the step finishes, erases the log and collapses to one status line.
#
step() {
    local msg="$1"
    shift
    CURRENT_STEP=$((CURRENT_STEP + 1))

    local tmpout="/tmp/deploy-step-$$"
    local rcfile="/tmp/deploy-rc-$$"
    local max_visible=8
    local trunc_width
    trunc_width=$(( $(tput cols 2>/dev/null || echo 80) - 6 ))

    > "$tmpout"
    rm -f "$rcfile"

    # Print initial header + empty line for output area
    printf "  ${DIM}[%d/%d]${RESET} ${CYAN}%s${RESET} %s\n" "$CURRENT_STEP" "$TOTAL_STEPS" "${SPINNER_FRAMES[0]}" "$msg"

    # Run command in background, save exit code
    ( set +e; "$@" >> "$tmpout" 2>&1; echo "$?" > "$rcfile" ) &
    local cmd_pid=$!

    tput civis 2>/dev/null || true

    local prev_lines=0
    local spin_idx=0

    while kill -0 "$cmd_pid" 2>/dev/null; do
        local spin="${SPINNER_FRAMES[$spin_idx]}"
        spin_idx=$(( (spin_idx + 1) % ${#SPINNER_FRAMES[@]} ))

        # Erase header + previously drawn output lines
        local erase=$((prev_lines + 1))
        printf "\033[%dA\033[J" "$erase"

        # Redraw header with animated spinner
        printf "  ${DIM}[%d/%d]${RESET} ${CYAN}%s${RESET} %s\n" "$CURRENT_STEP" "$TOTAL_STEPS" "$spin" "$msg"

        # Draw last N lines from output
        prev_lines=0
        if [[ -s "$tmpout" ]]; then
            while IFS= read -r line; do
                printf "    ${DIM}%.*s${RESET}\n" "$trunc_width" "$line"
                prev_lines=$((prev_lines + 1))
            done < <(tail -n "$max_visible" "$tmpout")
        fi

        sleep 0.1
    done
    wait "$cmd_pid" 2>/dev/null || true

    # Erase everything (header + output)
    printf "\033[%dA\033[J" $((prev_lines + 1))

    tput cnorm 2>/dev/null || true

    local rc
    rc=$(cat "$rcfile" 2>/dev/null || echo 1)

    if [[ "$rc" -eq 0 ]]; then
        printf "  ${DIM}[%d/%d]${RESET} ${GREEN}${TICK}${RESET} %s\n" "$CURRENT_STEP" "$TOTAL_STEPS" "$msg"
    else
        printf "  ${DIM}[%d/%d]${RESET} ${RED}${CROSS}${RESET} %s\n" "$CURRENT_STEP" "$TOTAL_STEPS" "$msg"
        echo ""
        printf "  ${RED}${BOLD}Error:${RESET} Step failed. Last 20 lines:\n\n"
        tail -20 "$tmpout" | sed 's/^/    /'
        echo ""
        printf "  ${DIM}Full log: %s${RESET}\n" "$LOG_FILE"
        cat "$tmpout" >> "$LOG_FILE"
        exit 1
    fi

    cat "$tmpout" >> "$LOG_FILE"
    rm -f "$tmpout" "$rcfile"
}

# ─── Configuration ───────────────────────────────────────────────────
GITHUB_REPO="Desent-tech/desent"
DOCKER_USER="cracklybody"

# ─── Prerequisite checks ────────────────────────────────────────────
check_deps() {
    if ! command -v curl &>/dev/null; then
        printf "  ${RED}${BOLD}Missing dependency:${RESET} curl\n\n"
        exit 1
    fi
}

# ─── Remote helpers ──────────────────────────────────────────────────
remote() {
    $SSH_CMD "$@"
}

remote_scp() {
    scp -i "$DEPLOY_KEY" -o StrictHostKeyChecking=no -o LogLevel=ERROR -o BatchMode=yes "$@"
}

# ─── Step implementations ────────────────────────────────────────────

DOCKER_OK=false

do_check_docker() {
    remote bash -s <<'SCRIPT'
if ! command -v docker &>/dev/null; then
    echo "NOT_INSTALLED"
    exit 0
fi

echo "Found: $(docker --version)"

# Check Docker API version — Traefik v3 needs >= 1.40 (Docker 19.03+)
api_ver=$(docker version --format '{{.Server.APIVersion}}' 2>/dev/null || echo "0.0")
echo "API version: $api_ver"

major=$(echo "$api_ver" | cut -d. -f1)
minor=$(echo "$api_ver" | cut -d. -f2)

if [[ "$major" -gt 1 ]] || { [[ "$major" -eq 1 ]] && [[ "$minor" -ge 40 ]]; }; then
    echo "DOCKER_OK"
else
    echo "DOCKER_OLD"
fi
SCRIPT
}

do_install_docker() {
    if [[ "$DOCKER_OK" == "true" ]]; then
        echo "Docker is up to date, skipping install."
        return 0
    fi

    remote bash -s <<'SCRIPT'
set -e
export DEBIAN_FRONTEND=noninteractive

echo "Stopping Docker if running..."
systemctl stop docker 2>/dev/null || true
systemctl stop docker.socket 2>/dev/null || true

echo "Purging ALL old Docker packages..."
apt-get purge -y docker docker-engine docker.io containerd runc \
    docker-ce docker-ce-cli containerd.io docker-buildx-plugin \
    docker-compose-plugin 2>/dev/null || true
apt-get autoremove -y 2>/dev/null || true

echo "Installing Docker via official script..."
curl -fsSL https://get.docker.com | sh

echo "Starting Docker..."
systemctl enable docker
systemctl start docker

# Wait for daemon to be ready
sleep 2

# Verify
echo "Installed: $(docker --version)"
api_ver=$(docker version --format '{{.Server.APIVersion}}' 2>/dev/null || echo "unknown")
echo "API version: $api_ver"

# Sanity check
docker info >/dev/null 2>&1 || { echo "ERROR: docker info failed"; exit 1; }
echo "Docker is ready."
SCRIPT
}

do_resolve_version() {
    echo "Fetching latest release tag from GitHub..."

    # Use GitHub API to get the latest tag (sorted by semver)
    DEPLOY_VERSION=$(curl -sf "https://api.github.com/repos/${GITHUB_REPO}/tags?per_page=1" \
        | grep '"name"' | head -1 | sed 's/.*"name": *"//;s/".*//')

    if [[ -z "$DEPLOY_VERSION" ]]; then
        echo "ERROR: Could not fetch tags from GitHub."
        echo "Falling back to 'latest'."
        DEPLOY_VERSION="latest"
    else
        # Strip 'v' prefix for Docker tag
        DEPLOY_VERSION="${DEPLOY_VERSION#v}"
        echo "Latest release: ${DEPLOY_VERSION}"
    fi

    SERVER_IMAGE="${DOCKER_USER}/desent-server:${DEPLOY_VERSION}"
    WEB_IMAGE="${DOCKER_USER}/desent-web:${DEPLOY_VERSION}"

    echo "Server image: ${SERVER_IMAGE}"
    echo "Web image:    ${WEB_IMAGE}"
}

do_pull_images() {
    remote bash -s <<SCRIPT
set -e
echo "Pulling ${SERVER_IMAGE}..."
docker pull "${SERVER_IMAGE}"

echo "Pulling ${WEB_IMAGE}..."
docker pull "${WEB_IMAGE}"

echo "Images pulled."
SCRIPT
}

do_generate_config() {
    local jwt_secret
    jwt_secret=$(openssl rand -hex 32)

    cat > /tmp/deploy-compose-$$ <<YAML
services:
  traefik:
    image: traefik:v3.6.10
    command:
      - --providers.docker=true
      - --providers.docker.exposedbydefault=false
      - --entrypoints.web.address=:80
    ports:
      - "80:80"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
    restart: unless-stopped

  server:
    image: ${SERVER_IMAGE}
    labels:
      - traefik.enable=true
      - "traefik.http.routers.server.rule=PathPrefix(\`/api\`) || PathPrefix(\`/ws\`) || PathPrefix(\`/live\`) || PathPrefix(\`/health\`) || PathPrefix(\`/static\`)"
      - traefik.http.routers.server.entrypoints=web
      - traefik.http.services.server.loadbalancer.server.port=8080
    ports:
      - "1935:1935"
    environment:
      - JWT_SECRET=${jwt_secret}
      - HLS_CACHE_MB=512
      - STREAM_KEY=live
      - DB_PATH=/data/desent.db
      - HLS_DIR=/tmp/hls
      - HTTP_ADDR=:8080
      - RTMP_ADDR=0.0.0.0:1935
    volumes:
      - db-data:/data
    restart: unless-stopped

  web:
    image: ${WEB_IMAGE}
    labels:
      - traefik.enable=true
      - "traefik.http.routers.web.rule=PathPrefix(\`/\`)"
      - traefik.http.routers.web.entrypoints=web
      - traefik.http.routers.web.priority=1
      - traefik.http.services.web.loadbalancer.server.port=3000
    depends_on:
      - server
    restart: unless-stopped

volumes:
  db-data:
YAML

    echo "Creating /opt/desent on server..."
    remote mkdir -p /opt/desent

    echo "Uploading docker-compose.yml..."
    remote_scp /tmp/deploy-compose-$$ "${SSH_TARGET}:/opt/desent/docker-compose.yml"

    rm -f /tmp/deploy-compose-$$
    echo "Config deployed."
}

do_deploy() {
    remote bash -s <<'SCRIPT'
cd /opt/desent

echo "Stopping old containers..."
docker compose down 2>/dev/null || true

echo "Starting containers..."
docker compose up -d

echo "Containers running:"
docker compose ps
SCRIPT
}

do_verify() {
    local max_attempts=20
    local attempt=0

    echo "Waiting for services to start..."
    while [[ $attempt -lt $max_attempts ]]; do
        # Try via Traefik (port 80)
        if remote curl -sf -o /dev/null "http://127.0.0.1/health" 2>/dev/null; then
            echo "Health check passed via Traefik!"
            remote bash -c 'cd /opt/desent && docker compose ps' 2>/dev/null || true
            return 0
        fi
        # Fallback: check server container directly
        if remote docker exec desent-server-1 wget -qO- "http://127.0.0.1:8080/health" 2>/dev/null | grep -q ok 2>/dev/null; then
            echo "Server is healthy (direct check)."
            echo "NOTE: Traefik may still be starting. Check http://${SERVER_IP} in a minute."
            remote bash -c 'cd /opt/desent && docker compose ps' 2>/dev/null || true
            return 0
        fi
        attempt=$((attempt + 1))
        echo "  waiting... ($attempt/$max_attempts)"
        sleep 3
    done

    echo ""
    echo "Health check failed. Container logs:"
    echo "─────────────────────────────────────"
    remote bash -c 'cd /opt/desent && docker compose logs --tail=30' 2>/dev/null || true
    return 1
}

# ─── Main ────────────────────────────────────────────────────────────
main() {
    banner
    check_deps

    # Prompt for SSH target
    printf "  ${BOLD}How do you SSH into the server?${RESET}\n"
    printf "  ${DIM}Example: ssh root@1.2.3.4${RESET}\n\n"

    read -rp "  > " SSH_INPUT
    [[ -z "$SSH_INPUT" ]] && fatal "SSH command is required"

    # Normalize: add "ssh" prefix if user just typed "root@1.2.3.4"
    if [[ "$SSH_INPUT" != ssh\ * ]]; then
        SSH_INPUT="ssh $SSH_INPUT"
    fi

    # Extract user@host (last argument) for scp and display
    SSH_TARGET="${SSH_INPUT##* }"
    SERVER_IP="${SSH_TARGET#*@}"

    # ── Setup deploy key (no passphrase, fully automated) ──
    echo ""
    if [[ ! -f "$DEPLOY_KEY" ]]; then
        printf "  ${DIM}Generating deploy key (no passphrase)...${RESET}\n"
        ssh-keygen -t ed25519 -f "$DEPLOY_KEY" -N "" -q -C "desent-deploy"
        printf "  ${GREEN}${TICK}${RESET} Created %s\n" "$DEPLOY_KEY"
    fi

    # Check if deploy key already works
    if ssh -i "$DEPLOY_KEY" -o BatchMode=yes -o StrictHostKeyChecking=no -o ConnectTimeout=5 -o LogLevel=ERROR \
        "$SSH_TARGET" true 2>/dev/null; then
        printf "  ${GREEN}${TICK}${RESET} Deploy key already on server\n"
    else
        # Copy deploy key using user's normal SSH (interactive — password/passphrase OK)
        printf "  ${DIM}Installing deploy key on server (you may need to enter password):${RESET}\n"
        local pubkey
        pubkey=$(cat "${DEPLOY_KEY}.pub")
        $SSH_INPUT -o StrictHostKeyChecking=no -o ConnectTimeout=10 \
            "mkdir -p ~/.ssh && chmod 700 ~/.ssh && echo '${pubkey}' >> ~/.ssh/authorized_keys && chmod 600 ~/.ssh/authorized_keys" \
            || fatal "Failed to install deploy key. Make sure you can run: $SSH_INPUT"
        printf "  ${GREEN}${TICK}${RESET} Deploy key installed\n"
    fi

    # From here on, use the deploy key only — no prompts, no passphrases
    SSH_CMD="ssh -i ${DEPLOY_KEY} -o StrictHostKeyChecking=no -o ConnectTimeout=10 -o LogLevel=ERROR -o BatchMode=yes ${SSH_TARGET}"

    # Verify it works
    if ! $SSH_CMD true 2>/dev/null; then
        fatal "Deploy key not working. Try: rm ${DEPLOY_KEY}* && ./deploy.sh"
    fi

    printf "\n  ${DIM}Deploying to ${BOLD}%s${RESET}\n\n" "$SSH_TARGET"

    # Step 1: Check Docker on remote
    step "Checking Docker on server"  do_check_docker

    # Parse check result to decide if install is needed
    if grep -q "DOCKER_OK" "$LOG_FILE" 2>/dev/null; then
        DOCKER_OK=true
    fi

    # Step 2: Install/upgrade Docker if needed
    if [[ "$DOCKER_OK" == "true" ]]; then
        # Skip but still show as completed
        CURRENT_STEP=$((CURRENT_STEP + 1))
        printf "  ${DIM}[%d/%d]${RESET} ${GREEN}${TICK}${RESET} Installing Docker ${DIM}(skipped — already up to date)${RESET}\n" "$CURRENT_STEP" "$TOTAL_STEPS"
    else
        step "Installing Docker"      do_install_docker
    fi

    step "Resolving latest version"   do_resolve_version
    step "Pulling images on server"   do_pull_images
    step "Generating config"          do_generate_config
    step "Deploying containers"       do_deploy
    step "Verifying deployment"       do_verify

    # Done
    echo ""
    printf "  ${GREEN}${BOLD}${TICK} Deploy complete!${RESET}\n"
    echo ""
    printf "  ${BOLD}Web app:${RESET}     http://%s\n" "$SERVER_IP"
    printf "  ${BOLD}RTMP ingest:${RESET} rtmp://%s:1935/live/live\n" "$SERVER_IP"
    printf "  ${BOLD}Traefik:${RESET}     http://%s:8080\n" "$SERVER_IP"
    echo ""
    printf "  ${DIM}Stream with OBS → rtmp://%s:1935/live/live${RESET}\n" "$SERVER_IP"
    printf "  ${DIM}Watch at → http://%s${RESET}\n" "$SERVER_IP"
    echo ""
}

main "$@"
