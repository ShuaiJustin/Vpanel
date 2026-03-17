#!/bin/sh
set -e

APP_ROOT="/app"
DEFAULT_CONFIG_PATH="${APP_ROOT}/configs/config.yaml"
CONFIG_PATH="${VPANEL_CONFIG_PATH:-${DEFAULT_CONFIG_PATH}}"
CONFIG_DIR="$(dirname "${CONFIG_PATH}")"
DEFAULT_DATA_DIR="${APP_ROOT}/data"
DEFAULT_LOG_DIR="${APP_ROOT}/logs"
DEFAULT_XRAY_DIR="${APP_ROOT}/xray"
DATA_DIR="${VPANEL_DATA_DIR:-${DEFAULT_DATA_DIR}}"
LOG_DIR="${VPANEL_LOG_DIR:-${DEFAULT_LOG_DIR}}"
XRAY_DIR="${VPANEL_XRAY_DIR:-${DEFAULT_XRAY_DIR}}"
DEFAULT_DB_PATH="${DATA_DIR}/v.db"
DB_PATH="${V_DB_PATH:-${DEFAULT_DB_PATH}}"
ACME_HOME="${HOME}/.acme.sh"
ACME_SCRIPT="${ACME_HOME}/acme.sh"
ACME_INSTALLER="/tmp/acme.sh"
CONFIG_TEMPLATE_PATH="${APP_ROOT}/configs/config.yaml.example"
RUN_USER="vpanel"

log() {
    echo "$@"
}

ensure_writable_dir() {
    target="$1"
    mkdir -p "${target}"

    probe="${target}/.write-test.$$"
    if ! touch "${probe}" >/dev/null 2>&1; then
        log "ERROR: ${target} is not writable"
        exit 1
    fi

    rm -f "${probe}"
}

ensure_writable_file() {
    target="$1"
    parent="$(dirname "${target}")"

    ensure_writable_dir "${parent}"

    if [ -f "${target}" ] && [ ! -w "${target}" ]; then
        log "ERROR: ${target} is not writable"
        exit 1
    fi
}

fix_runtime_ownership() {
    if [ "$(id -u)" -ne 0 ]; then
        return
    fi

    mkdir -p "${CONFIG_DIR}" "${DATA_DIR}" "${LOG_DIR}" "${XRAY_DIR}" "$(dirname "${DB_PATH}")"

    chown -R "${RUN_USER}:${RUN_USER}" "${DATA_DIR}" "${LOG_DIR}" "${XRAY_DIR}" "$(dirname "${DB_PATH}")"

    if [ -e "${CONFIG_DIR}" ]; then
        chown -R "${RUN_USER}:${RUN_USER}" "${CONFIG_DIR}"
    fi
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
    fix_runtime_ownership

    ensure_writable_dir "${CONFIG_DIR}"
    ensure_writable_dir "${DATA_DIR}"
    ensure_writable_dir "${LOG_DIR}"
    ensure_writable_dir "${XRAY_DIR}"
    ensure_writable_file "${DB_PATH}"

    if [ ! -f "${CONFIG_PATH}" ]; then
        log "Creating default configuration..."
        cp "${CONFIG_TEMPLATE_PATH}" "${CONFIG_PATH}"
        if [ "$(id -u)" -eq 0 ]; then
            chown "${RUN_USER}:${RUN_USER}" "${CONFIG_PATH}"
        fi
    fi

    if [ ! -f "${DB_PATH}" ]; then
        log "Initializing database..."
        touch "${DB_PATH}"
        if [ "$(id -u)" -eq 0 ]; then
            chown "${RUN_USER}:${RUN_USER}" "${DB_PATH}"
        fi
    fi
}

print_config() {
    log "Configuration:"
    log "  Server Host: ${V_SERVER_HOST:-0.0.0.0}"
    log "  Server Port: ${V_SERVER_PORT:-8080}"
    log "  Server Mode: ${V_SERVER_MODE:-release}"
    log "  Log Level: ${V_LOG_LEVEL:-info}"
    log "  Config File: ${CONFIG_PATH}"
    log "  Database: ${DB_PATH}"
    log "  Data Dir: ${DATA_DIR}"
    log "  Log Dir: ${LOG_DIR}"
    log "  Xray Dir: ${XRAY_DIR}"
}

log "Starting V Panel..."
install_acme
validate_release_settings
prepare_runtime
print_config

if [ "$#" -ge 1 ] && [ "$1" = "${APP_ROOT}/v-panel" ]; then
    if [ "$#" -ge 2 ] && [ "${2}" = "-config" ]; then
        shift 2
    else
        shift 1
    fi
    set -- "${APP_ROOT}/v-panel" -config "${CONFIG_PATH}" "$@"
fi

if [ "$(id -u)" -eq 0 ]; then
    exec su-exec "${RUN_USER}" "$@"
fi

exec "$@"
