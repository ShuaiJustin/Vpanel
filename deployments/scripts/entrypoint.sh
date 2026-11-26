#!/bin/sh
set -eu

APP_ROOT="/app"
RUN_USER="vpanel"
RUN_HOME="/home/${RUN_USER}"

DEFAULT_CONFIG_PATH="${APP_ROOT}/configs/config.yaml"
DEFAULT_DATA_DIR="${APP_ROOT}/data"
DEFAULT_LOG_DIR="${APP_ROOT}/logs"
DEFAULT_XRAY_DIR="${APP_ROOT}/xray"

CONFIG_PATH="${VPANEL_CONFIG_PATH:-${DEFAULT_CONFIG_PATH}}"
CONFIG_DIR="$(dirname "${CONFIG_PATH}")"
DATA_DIR="${VPANEL_DATA_DIR:-${DEFAULT_DATA_DIR}}"
LOG_DIR="${VPANEL_LOG_DIR:-${DEFAULT_LOG_DIR}}"
XRAY_DIR="${VPANEL_XRAY_DIR:-${DEFAULT_XRAY_DIR}}"
DB_PATH="${V_DB_PATH:-${DATA_DIR}/v.db}"
DB_DIR="$(dirname "${DB_PATH}")"

CONFIG_TEMPLATE_PATH="${APP_ROOT}/configs/config.yaml.example"
HOME="${HOME:-${RUN_HOME}}"
ACME_HOME="${HOME}/.acme.sh"
ACME_SCRIPT="${ACME_HOME}/acme.sh"
ACME_INSTALLER="/tmp/acme.sh"

log() {
    printf '%s\n' "$*"
}

is_root() {
    [ "$(id -u)" -eq 0 ]
}

ensure_dir() {
    target="$1"
    mkdir -p "${target}"
}

ensure_writable_dir() {
    target="$1"
    ensure_dir "${target}"

    probe="${target}/.write-test.$$"
    if ! touch "${probe}" >/dev/null 2>&1; then
        log "ERROR: ${target} is not writable"
        exit 1
    fi
    rm -f "${probe}"
}

ensure_writable_file() {
    target="$1"
    ensure_writable_dir "$(dirname "${target}")"

    if [ -f "${target}" ] && [ ! -w "${target}" ]; then
        log "ERROR: ${target} is not writable"
        exit 1
    fi
}

chown_if_root() {
    path="$1"
    if is_root && [ -e "${path}" ]; then
        chown -R "${RUN_USER}:${RUN_USER}" "${path}"
    fi
}

prepare_runtime_tree() {
    ensure_dir "${RUN_HOME}"
    ensure_dir "${CONFIG_DIR}"
    ensure_dir "${DATA_DIR}"
    ensure_dir "${LOG_DIR}"
    ensure_dir "${XRAY_DIR}"
    ensure_dir "${DB_DIR}"
    ensure_dir "${ACME_HOME}"

    chown_if_root "${RUN_HOME}"
    chown_if_root "${CONFIG_DIR}"
    chown_if_root "${DATA_DIR}"
    chown_if_root "${LOG_DIR}"
    chown_if_root "${XRAY_DIR}"
    chown_if_root "${DB_DIR}"
    chown_if_root "${ACME_HOME}"
}

bootstrap_default_config() {
    if [ -f "${CONFIG_PATH}" ]; then
        return
    fi

    log "Creating default configuration..."
    cp "${CONFIG_TEMPLATE_PATH}" "${CONFIG_PATH}"
    if is_root; then
        chown "${RUN_USER}:${RUN_USER}" "${CONFIG_PATH}"
    fi
}

bootstrap_database() {
    if [ -f "${DB_PATH}" ]; then
        return
    fi

    log "Initializing database..."
    touch "${DB_PATH}"
    if is_root; then
        chown "${RUN_USER}:${RUN_USER}" "${DB_PATH}"
    fi
}

install_acme() {
    if [ "${VPANEL_ACME_AUTO_INSTALL:-0}" = "0" ]; then
        log "Skipping acme.sh bootstrap because VPANEL_ACME_AUTO_INSTALL=0"
        return
    fi

    if [ -f "${ACME_SCRIPT}" ]; then
        return
    fi

    log "Installing acme.sh..."
    ensure_dir "${ACME_HOME}"

    if ! wget -q -O "${ACME_INSTALLER}" -t 2 -T 10 https://raw.githubusercontent.com/acmesh-official/acme.sh/master/acme.sh; then
        rm -f "${ACME_INSTALLER}"
        log "WARNING: acme.sh script download failed, will retry on first certificate request"
        return
    fi

    if [ -n "${ACME_EMAIL:-}" ]; then
        sh "${ACME_INSTALLER}" --install --home "${ACME_HOME}" --accountemail "${ACME_EMAIL}" >/tmp/acme-install.log 2>&1 || true
    else
        sh "${ACME_INSTALLER}" --install --home "${ACME_HOME}" >/tmp/acme-install.log 2>&1 || true
    fi

    rm -f "${ACME_INSTALLER}"

    if [ -f "${ACME_SCRIPT}" ]; then
        log "acme.sh installed successfully"
        "${ACME_SCRIPT}" --set-default-ca --server letsencrypt >/dev/null 2>&1 || true
        return
    fi

    log "WARNING: acme.sh installation failed, will retry on first certificate request"
    if [ -f /tmp/acme-install.log ]; then
        tail -n 20 /tmp/acme-install.log || true
    fi
}

validate_secret() {
    value="$1"
    min_len="$2"

    [ -n "${value}" ] || return 1

    length="$(printf '%s' "${value}" | wc -c | tr -d ' ')"
    [ "${length}" -ge "${min_len}" ]
}

validate_release_settings() {
    if [ "${V_SERVER_MODE:-release}" != "release" ]; then
        return
    fi

    log "Production mode detected, performing security checks..."

    case "${V_JWT_SECRET:-}" in
        ""|"CHANGE_ME_OR_AUTO_GENERATE_ON_FIRST_START"|"CHANGE_ME_OR_SYSTEM_WILL_REFUSE_TO_START"|"your-secure-jwt-secret-change-me"|"change-me-in-production")
            log "ERROR: JWT secret is not configured or still using a default value"
            exit 1
            ;;
    esac

    if ! validate_secret "${V_JWT_SECRET:-}" 32; then
        log "ERROR: JWT secret must be at least 32 characters"
        exit 1
    fi

    case "${V_ADMIN_PASS:-}" in
        ""|"CHANGE_ME_OR_AUTO_GENERATE_ON_FIRST_START"|"CHANGE_ME_OR_SYSTEM_WILL_REFUSE_TO_START"|"admin123"|"your-secure-admin-password")
            log "ERROR: admin password is not configured or still using a default value"
            exit 1
            ;;
    esac

    if ! validate_secret "${V_ADMIN_PASS:-}" 12; then
        log "ERROR: admin password must be at least 12 characters"
        exit 1
    fi

    log "Security checks passed"
}

prepare_runtime() {
    prepare_runtime_tree

    ensure_writable_dir "${CONFIG_DIR}"
    ensure_writable_dir "${DATA_DIR}"
    ensure_writable_dir "${LOG_DIR}"
    ensure_writable_dir "${XRAY_DIR}"
    ensure_writable_file "${DB_PATH}"

    bootstrap_default_config
    bootstrap_database
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

exec_command() {
    if is_root; then
        exec env HOME="${RUN_HOME}" su-exec "${RUN_USER}" "$@"
    fi

    exec "$@"
}

log "Starting V Panel..."
install_acme
validate_release_settings
prepare_runtime
print_config

if [ "$#" -eq 0 ]; then
    set -- "${APP_ROOT}/v-panel"
fi

if [ "$1" = "${APP_ROOT}/v-panel" ] || [ "$1" = "v-panel" ]; then
    if [ "$#" -ge 3 ] && [ "$2" = "-config" ]; then
        shift 3
    else
        shift 1
    fi
    set -- "${APP_ROOT}/v-panel" -config "${CONFIG_PATH}" "$@"
fi

exec_command "$@"
