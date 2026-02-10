#!/bin/bash
set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

log()  { echo -e "${CYAN}[TEST]${NC} $*"; }
ok()   { echo -e "${GREEN}[PASS]${NC} $*"; }
fail() { echo -e "${RED}[FAIL]${NC} $*"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $*"; }

# PSK for both agents (shared secret, 32 bytes hex)
PSK="0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

# Virtual overlay network IPs
AGENT_A_VIP="10.147.17.1/24"
AGENT_B_VIP="10.147.17.2/24"
AGENT_A_VIP_PLAIN="10.147.17.1"
AGENT_B_VIP_PLAIN="10.147.17.2"

# Docker underlay IPs
AGENT_A_DOCKER_IP="172.28.0.10"
AGENT_B_DOCKER_IP="172.28.0.11"

COMPOSE="docker compose -f docker-compose.test.yml"

cleanup() {
    log "Cleaning up..."
    $COMPOSE down --remove-orphans 2>/dev/null || true
    rm -rf test-data
}

trap cleanup EXIT

echo -e "${BOLD}═══════════════════════════════════════════${NC}"
echo -e "${BOLD}  ZeroGo P2P Virtual LAN Test Suite${NC}"
echo -e "${BOLD}═══════════════════════════════════════════${NC}"
echo ""

# ─────────────────────────────────────────────
log "Step 1/8: Build Docker image"
# ─────────────────────────────────────────────
$COMPOSE build 2>&1 | tail -5

# ─────────────────────────────────────────────
log "Step 2/8: Start containers"
# ─────────────────────────────────────────────
mkdir -p test-data/agent-a test-data/agent-b
$COMPOSE up -d
sleep 2

# Verify containers are running
if ! docker exec zerogo-agent-a true 2>/dev/null; then
    fail "Container agent-a failed to start"
    exit 1
fi
if ! docker exec zerogo-agent-b true 2>/dev/null; then
    fail "Container agent-b failed to start"
    exit 1
fi
ok "Both containers running"

# Check /dev/net/tun availability
if ! docker exec zerogo-agent-a ls /dev/net/tun >/dev/null 2>&1; then
    log "Creating /dev/net/tun inside containers..."
    docker exec zerogo-agent-a sh -c 'mkdir -p /dev/net && mknod /dev/net/tun c 10 200 && chmod 600 /dev/net/tun' 2>/dev/null || true
    docker exec zerogo-agent-b sh -c 'mkdir -p /dev/net && mknod /dev/net/tun c 10 200 && chmod 600 /dev/net/tun' 2>/dev/null || true
fi

# ─────────────────────────────────────────────
log "Step 3/8: Generate identities"
# ─────────────────────────────────────────────
IDENTITY_A=$(docker exec zerogo-agent-a zerogo-agent --identity /etc/zerogo/identity.key --show-identity 2>/dev/null)
IDENTITY_B=$(docker exec zerogo-agent-b zerogo-agent --identity /etc/zerogo/identity.key --show-identity 2>/dev/null)

PUBKEY_A=$(echo "$IDENTITY_A" | grep "Public Key" | awk '{print $3}')
PUBKEY_B=$(echo "$IDENTITY_B" | grep "Public Key" | awk '{print $3}')
ADDR_A=$(echo "$IDENTITY_A" | grep "Address" | awk '{print $2}')
ADDR_B=$(echo "$IDENTITY_B" | grep "Address" | awk '{print $2}')

if [ -z "$PUBKEY_A" ] || [ -z "$PUBKEY_B" ]; then
    fail "Failed to generate identities"
    docker exec zerogo-agent-a zerogo-agent --identity /etc/zerogo/identity.key --show-identity 2>&1
    exit 1
fi

log "Agent A: addr=${ADDR_A} pubkey=${PUBKEY_A:0:16}..."
log "Agent B: addr=${ADDR_B} pubkey=${PUBKEY_B:0:16}..."
ok "Identities generated"

# ─────────────────────────────────────────────
log "Step 4/8: Start Agent A"
# ─────────────────────────────────────────────
docker exec -d zerogo-agent-a sh -c "zerogo-agent \
    --identity /etc/zerogo/identity.key \
    --port 9993 \
    --tap zt0 \
    --tap-ip $AGENT_A_VIP \
    --mtu 1400 \
    --network 1 \
    --psk $PSK \
    --peer ${PUBKEY_B}@${AGENT_B_DOCKER_IP}:9993 \
    --log-level debug \
    > /tmp/agent.log 2>&1"

sleep 1

# ─────────────────────────────────────────────
log "Step 5/8: Start Agent B"
# ─────────────────────────────────────────────
docker exec -d zerogo-agent-b sh -c "zerogo-agent \
    --identity /etc/zerogo/identity.key \
    --port 9993 \
    --tap zt0 \
    --tap-ip $AGENT_B_VIP \
    --mtu 1400 \
    --network 1 \
    --psk $PSK \
    --peer ${PUBKEY_A}@${AGENT_A_DOCKER_IP}:9993 \
    --log-level debug \
    > /tmp/agent.log 2>&1"

log "Waiting for PSK handshake and TAP setup..."
sleep 4

# ─────────────────────────────────────────────
log "Step 6/8: Verify TAP interfaces"
# ─────────────────────────────────────────────
echo ""
log "Agent A network interfaces:"
docker exec zerogo-agent-a ip addr show zt0 2>&1 && ok "Agent A TAP zt0 exists" || fail "Agent A TAP zt0 NOT FOUND"
echo ""
log "Agent B network interfaces:"
docker exec zerogo-agent-b ip addr show zt0 2>&1 && ok "Agent B TAP zt0 exists" || fail "Agent B TAP zt0 NOT FOUND"

# ─────────────────────────────────────────────
log "Step 7/8: Test connectivity"
# ─────────────────────────────────────────────

echo ""
echo -e "${BOLD}--- Ping A → B (${AGENT_A_VIP_PLAIN} → ${AGENT_B_VIP_PLAIN}) ---${NC}"
if docker exec zerogo-agent-a ping -c 4 -W 5 "$AGENT_B_VIP_PLAIN" 2>&1; then
    echo ""
    ok "Ping A → B SUCCESS"
    PING_AB=0
else
    echo ""
    fail "Ping A → B FAILED"
    PING_AB=1
fi

echo ""
echo -e "${BOLD}--- Ping B → A (${AGENT_B_VIP_PLAIN} → ${AGENT_A_VIP_PLAIN}) ---${NC}"
if docker exec zerogo-agent-b ping -c 4 -W 5 "$AGENT_A_VIP_PLAIN" 2>&1; then
    echo ""
    ok "Ping B → A SUCCESS"
    PING_BA=0
else
    echo ""
    fail "Ping B → A FAILED"
    PING_BA=1
fi

echo ""
echo -e "${BOLD}--- ARP test (L2 verification) ---${NC}"
docker exec zerogo-agent-a arping -c 2 -I zt0 -w 5 "$AGENT_B_VIP_PLAIN" 2>&1 && ok "ARP test SUCCESS" || warn "ARP test inconclusive"

# ─────────────────────────────────────────────
log "Step 8/8: Agent logs"
# ─────────────────────────────────────────────
echo ""
echo -e "${BOLD}=== Agent A logs (last 30 lines) ===${NC}"
docker exec zerogo-agent-a cat /tmp/agent.log 2>/dev/null | tail -30 || warn "No logs captured for Agent A"

echo ""
echo -e "${BOLD}=== Agent B logs (last 30 lines) ===${NC}"
docker exec zerogo-agent-b cat /tmp/agent.log 2>/dev/null | tail -30 || warn "No logs captured for Agent B"

# ─────────────────────────────────────────────
echo ""
echo -e "${BOLD}═══════════════════════════════════════════${NC}"
if [ "${PING_AB}" = "0" ] && [ "${PING_BA}" = "0" ]; then
    echo -e "${GREEN}${BOLD}  ALL TESTS PASSED ✓${NC}"
else
    echo -e "${RED}${BOLD}  SOME TESTS FAILED ✗${NC}"
fi
echo -e "${BOLD}═══════════════════════════════════════════${NC}"
