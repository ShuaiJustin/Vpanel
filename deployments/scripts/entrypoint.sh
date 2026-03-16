#!/bin/sh
set -e

APP_ROOT="/app"
CONFIG_DIR="${APP_ROOT}/configs"
DATA_DIR="${APP_ROOT}/data"
LOG_DIR="${APP_ROOT}/logs"
XRAY_DIR="${APP_ROOT}/xray"
DEFAULT_DB_PATH="${DATA_DIR}/v.db"
DB_PATH="${V_DB_PATH:-${DEFAULT_DB_PATH}}"
ACME_HOME="${HOME}/.acme.sh"
ACME_SCRIPT="${ACME_HOME}/acme.sh"
ACME_INSTALLER="/tmp/acme.sh"

log() {
    echo "$@"
}

install_acme() {
    if [ "${VPANEL_ACME_AUTO_INSTALL:-1}" = "0" ]; then
        log "Skipping acme.sh bootstrap because VPANEL_ACME_AUTO_INSTALL=0"
        return
    fi

    if [ -f "${ACME_SCRIPT}" ]; then
        return
    fi

    log "Installing acme.sh..."
    mkdir -p "${ACME_HOME}"

    if ! wget -q -O "${ACME_INSTALLER}" -t 2 -T 10 https://raw.githubusercontent.com/acmesh-official/acme.sh/master/acme.sh; then
        rm -f "${ACME_INSTALLER}"
        log "⚠ acme.sh script download failed, will retry on first certificate request"
        return
    fi

    if [ -n "${ACME_EMAIL}" ]; then
        sh "${ACME_INSTALLER}" --install --home "${ACME_HOME}" --accountemail "${ACME_EMAIL}" >/tmp/acme-install.log 2>&1 || true
    else
        sh "${ACME_INSTALLER}" --install --home "${ACME_HOME}" >/tmp/acme-install.log 2>&1 || true
    fi

    rm -f "${ACME_INSTALLER}"

    if [ -f "${ACME_SCRIPT}" ]; then
        log "✓ acme.sh installed successfully"
        "${ACME_SCRIPT}" --set-default-ca --server letsencrypt >/dev/null 2>&1 || true
        return
    fi

    log "⚠ acme.sh installation failed, will retry on first certificate request"
    if [ -f /tmp/acme-install.log ]; then
        tail -n 20 /tmp/acme-install.log || true
    fi
}

validate_release_settings() {
    if [ "${V_SERVER_MODE}" != "release" ]; then
        return
    fi

    log "Production mode detected, performing security checks..."

    if [ -z "${V_JWT_SECRET}" ] || \
       [ "${V_JWT_SECRET}" = "CHANGE_ME_OR_AUTO_GENERATE_ON_FIRST_START" ] || \
       [ "${V_JWT_SECRET}" = "CHANGE_ME_OR_SYSTEM_WILL_REFUSE_TO_START" ] || \
       [ "${V_JWT_SECRET}" = "your-secure-jwt-secret-change-me" ] || \
       [ "${V_JWT_SECRET}" = "change-me-in-production" ]; then
        log "ERROR: JWT_SECRET is not configured or using default value!"
        log "Please set a secure JWT_SECRET in your .env file"
        log "Generate one with: openssl rand -base64 32"
        exit 1
    fi

    JWT_LEN=$(echo -n "${V_JWT_SECRET}" | wc -c | tr -d ' ')
    if [ "${JWT_LEN}" -lt 32 ]; then
        log "ERROR: JWT_SECRET is too short (${JWT_LEN} chars, minimum 32 required)"
        exit 1
    fi

    if [ -z "${V_ADMIN_PASS}" ] || \
       [ "${V_ADMIN_PASS}" = "CHANGE_ME_OR_AUTO_GENERATE_ON_FIRST_START" ] || \
       [ "${V_ADMIN_PASS}" = "CHANGE_ME_OR_SYSTEM_WILL_REFUSE_TO_START" ] || \
       [ "${V_ADMIN_PASS}" = "admin123" ] || \
       [ "${V_ADMIN_PASS}" = "your-secure-admin-password" ]; then
        log "ERROR: Admin password is not configured or using default value!"
        log "Please set a secure password in your .env file"
        exit 1
    fi

    PASS_LEN=$(echo -n "${V_ADMIN_PASS}" | wc -c | tr -d ' ')
    if [ "${PASS_LEN}" -lt 12 ]; then
        log "ERROR: Admin password is too short (${PASS_LEN} chars, minimum 12 required)"
        exit 1
    fi

    log "✓ Security checks passed"
}

prepare_runtime() {
    mkdir -p "${CONFIG_DIR}" "${DATA_DIR}" "${LOG_DIR}" "${XRAY_DIR}" "$(dirname "${DB_PATH}")"

    if [ ! -f "${CONFIG_DIR}/config.yaml" ]; then
        log "Creating default configuration..."
        cp "${CONFIG_DIR}/config.yaml.example" "${CONFIG_DIR}/config.yaml"
    fi

    if [ ! -f "${DB_PATH}" ]; then
        log "Initializing database..."
        touch "${DB_PATH}"
    fi
}

print_config() {
    log "Configuration:"
    log "  Server Host: ${V_SERVER_HOST:-0.0.0.0}"
    log "  Server Port: ${V_SERVER_PORT:-8080}"
    log "  Server Mode: ${V_SERVER_MODE:-release}"
    log "  Log Level: ${V_LOG_LEVEL:-info}"
    log "  Database: ${DB_PATH}"
}

log "Starting V Panel..."
install_acme
validate_release_settings
prepare_runtime
print_config

exec "$@"
