#!/usr/bin/env bash
# Self-contained bootstrap, also sourced by setup.sh. Never read approvals from stdin.
set -euo pipefail

dependency_version_at_least() {
  local actual="$1" minimum="$2" a b c x y z
  [[ "$actual" =~ ^[0-9]+\.[0-9]+(\.[0-9]+)?$ && "$minimum" =~ ^[0-9]+\.[0-9]+(\.[0-9]+)?$ ]] || return 1
  IFS=. read -r a b c <<< "$actual"
  IFS=. read -r x y z <<< "$minimum"
  a=$((10#$a)); b=$((10#$b)); c=$((10#${c:-0}))
  x=$((10#$x)); y=$((10#$y)); z=$((10#${z:-0}))
  (( a > x || (a == x && b > y) || (a == x && b == y && c >= z) ))
}

dependency_confirm() {
  printf '\nAlteração proposta: %s\n' "$1" >&2
  if ! { exec 3<>/dev/tty; } 2>/dev/null; then
    echo 'Sem terminal interativo: nenhuma alteração autorizada. Execute em um terminal.' >&2
    return 1
  fi
  local answer=''
  printf 'Digite SIM para autorizar; qualquer outra resposta cancela: ' >&3
  IFS= read -r answer <&3 || answer=''
  exec 3>&-
  if [[ "$answer" != SIM ]]; then
    echo 'Alteração recusada. Instalação interrompida.' >&2
    return 1
  fi
}

dependency_detect_platform() {
  dependency_os=$(uname -s)
  case "$dependency_os" in
    Darwin) ;;
    Linux)
      [[ -r /etc/os-release ]] || { echo 'Linux sem identificação da distribuição.' >&2; return 1; }
      dependency_distribution=$( . /etc/os-release; printf '%s' "$ID" )
      case "$dependency_distribution" in ubuntu|debian) ;; *) echo 'Instalação automática suporta macOS, Ubuntu e Debian.' >&2; return 1;; esac
      local release minimum
      release=$( . /etc/os-release; printf '%s' "$VERSION_ID" )
      [[ "$release" == *.* ]] || release="$release.0"
      if [[ "$dependency_distribution" == ubuntu ]]; then minimum=22.04; else minimum=12.0; fi
      dependency_version_at_least "$release" "$minimum" || { echo "Versão não suportada: $dependency_distribution $release (mínimo $minimum). O sistema operacional não será atualizado pelo instalador." >&2; return 1; } ;;
    *) echo 'Use macOS, Ubuntu ou Debian (inclusive WSL2 com Docker disponível).' >&2; return 1 ;;
  esac
}

dependency_version() {
  local name="$1" raw=''
  if [[ "$dependency_os" == Darwin && ( "$name" == git || "$name" == cc ) ]]; then
    xcode-select -p >/dev/null 2>&1 || return 1
  fi
  case "$name" in
    go) raw=$(GOTOOLCHAIN=local go version 2>/dev/null) || return 1; [[ "$raw" != *rc* && "$raw" != *beta* && "$raw" != *devel* ]] || return 1 ;;
    docker) raw=$(docker --version 2>/dev/null) || return 1 ;;
    compose) raw=$(docker compose version --short 2>/dev/null) || return 1 ;;
    cc) raw=$("${CC:-cc}" --version 2>/dev/null) || return 1 ;;
    uuidgen)
      raw=$(uuidgen 2>/dev/null) || return 1
      [[ "$raw" =~ ^[[:xdigit:]]{8}-[[:xdigit:]]{4}-[[:xdigit:]]{4}-[[:xdigit:]]{4}-[[:xdigit:]]{12}$ ]] || return 1
      echo 1.0; return ;;
    *) raw=$("$name" --version 2>/dev/null) || return 1 ;;
  esac
  if [[ "$raw" =~ ([0-9]+)\.([0-9]+)(\.([0-9]+))? ]]; then
    printf '%s.%s.%s\n' "${BASH_REMATCH[1]}" "${BASH_REMATCH[2]}" "${BASH_REMATCH[4]:-0}"
  else return 1; fi
}

# Reuse already-installed Homebrew tools in future shells, without editing profiles.
dependency_activate_installed_tools() {
  [[ "$dependency_os" == Darwin ]] || return 0
  local prefix item tool package minimum current
  if ! command -v brew >/dev/null; then
    for prefix in /opt/homebrew/bin /usr/local/bin; do
      if [[ -x "$prefix/brew" ]]; then export PATH="$prefix:$PATH"; break; fi
    done
  fi
  if command -v brew >/dev/null; then
    for item in curl:curl:7.68.0 git:git:2.23.0 jq:jq:1.6.0 make:make:3.81.0 uuidgen:util-linux:1.0.0; do
      tool=${item%%:*}; item=${item#*:}; package=${item%%:*}; minimum=${item#*:}
      current=$(dependency_version "$tool") || current=''
      if ! dependency_version_at_least "$current" "$minimum"; then
        prefix=$(brew --prefix "$package" 2>/dev/null) || continue
        if [[ -d "$prefix/bin" ]]; then export PATH="$prefix/bin:$prefix/libexec/gnubin:$PATH"; fi
      fi
    done
  fi
  if ! command -v docker >/dev/null && [[ -x /Applications/Docker.app/Contents/Resources/bin/docker ]]; then
    export PATH="/Applications/Docker.app/Contents/Resources/bin:$PATH"
  fi
  current=$(dependency_version cc) || current=''
  if ! dependency_version_at_least "$current" 10.0.0 && command -v brew >/dev/null; then
    prefix=$(brew --prefix llvm 2>/dev/null) || prefix=''
    if [[ -x "$prefix/bin/clang" ]]; then export CC="$prefix/bin/clang"; fi
  fi
  hash -r
}

dependency_activate_go() {
  local required="$1" current managed
  current=$(dependency_version go) || current=0.0.0
  dependency_version_at_least "$current" "$required" && return 0
  managed="$HOME/.local/share/wagering/toolchains/go$required/bin"
  if [[ -x "$managed/go" ]]; then export PATH="$managed:$PATH"; hash -r; fi
}

dependency_sudo() {
  if [[ "$EUID" == 0 ]]; then "$@"; else sudo "$@"; fi
}

dependency_command_line_tools() {
  xcode-select -p >/dev/null 2>&1 && return 0
  dependency_confirm 'Instalar Apple Command Line Tools (compilador e ferramentas Git) pelo instalador do macOS.' || return 1
  xcode-select --install || return 1
  echo 'Conclua a instalação na janela do macOS. Aguardando até 20 minutos...'
  local attempt
  for ((attempt=0; attempt<240; attempt++)); do
    xcode-select -p >/dev/null 2>&1 && return 0
    sleep 5
  done
  echo 'Command Line Tools ainda indisponível. Conclua a instalação e execute novamente.' >&2
  return 1
}

dependency_homebrew() {
  if ! command -v brew >/dev/null; then
    dependency_command_line_tools || return 1
    dependency_confirm 'Instalar Homebrew pelo instalador oficial de brew.sh; poderá solicitar sudo. Nenhum perfil de shell será editado por este script.' || return 1
    local installer
    installer=$(mktemp "${TMPDIR:-/tmp}/wagering-homebrew.XXXXXX") || return 1
    if ! /usr/bin/curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh -o "$installer"; then rm -f -- "$installer"; return 1; fi
    if ! /bin/bash "$installer" </dev/tty; then rm -f -- "$installer"; return 1; fi
    rm -f -- "$installer"
    if [[ -x /opt/homebrew/bin/brew ]]; then export PATH="/opt/homebrew/bin:$PATH";
    elif [[ -x /usr/local/bin/brew ]]; then export PATH="/usr/local/bin:$PATH"; fi
  fi
  command -v brew >/dev/null || return 1
}

dependency_package() {
  local name="$1" minimum="$2" current="$3" package="$1" action=install requirement="versão >= $2"
  [[ "$name" != uuidgen ]] || requirement="geração de UUID válido"
  if [[ "$dependency_os" == Darwin ]]; then
    if [[ "$name" == cc ]]; then
      if ! xcode-select -p >/dev/null 2>&1; then dependency_command_line_tools; return; fi
      package=llvm
    fi
    dependency_homebrew || return 1
    case "$name" in uuidgen) package=util-linux ;; make) package=make ;; esac
    brew list --versions "$package" >/dev/null 2>&1 && action=upgrade
    dependency_confirm "$name: versão atual ${current:-ausente}; requisito $requirement. Executar brew $action $package e suas dependências. Fonte: Homebrew." || return 1
    HOMEBREW_NO_AUTO_UPDATE=1 brew "$action" "$package" || return 1
    local prefix
    prefix=$(brew --prefix "$package") || return 1
    export PATH="$prefix/bin:$prefix/libexec/gnubin:$PATH"
    if [[ "$name" == cc ]]; then export CC="$prefix/bin/clang"; fi
  else
    case "$name" in cc) package=build-essential ;; uuidgen) package=uuid-runtime ;; esac
    dependency_confirm "$name: versão atual ${current:-ausente}; requisito $requirement. Atualizar índices APT e instalar/atualizar $package pelos repositórios da distribuição, usando sudo quando necessário." || return 1
    dependency_sudo apt-get update || return 1
    dependency_sudo apt-get install -y "$package" || return 1
  fi
  hash -r
}

dependency_install_go() (
  local required="$1" current="$2" system arch archive checksum directory target
  system=$(uname -s | tr '[:upper:]' '[:lower:]')
  case "$(uname -m)" in arm64|aarch64) arch=arm64 ;; x86_64|amd64) arch=amd64 ;; *) echo 'Arquitetura Go não suportada.' >&2; return 1;; esac
  target="$HOME/.local/share/wagering/toolchains/go$required"
  dependency_confirm "Go: versão atual ${current:-ausente}; instalar Go $required em $target, com download de go.dev e validação SHA-256. O Go do sistema e seus perfis de shell serão preservados." || return 1
  [[ ! -e "$target" ]] || { echo "Destino já existe: $target. Não será sobrescrito." >&2; return 1; }
  directory=$(mktemp -d "${TMPDIR:-/tmp}/wagering-go.XXXXXX") || return 1
  trap 'rm -rf -- "$directory"' EXIT
  archive="go$required.$system-$arch.tar.gz"
  curl -fsSL 'https://go.dev/dl/?mode=json&include=all' -o "$directory/releases.json" || return 1
  checksum=$(jq -er --arg file "$archive" '[.[].files[] | select(.filename == $file) | .sha256][0] // error("versão/plataforma indisponível em go.dev")' "$directory/releases.json") || return 1
  [[ "$checksum" =~ ^[a-f0-9]{64}$ ]] || return 1
  curl -fsSL "https://go.dev/dl/$archive" -o "$directory/$archive" || return 1
  local actual
  if command -v sha256sum >/dev/null; then actual=$(sha256sum "$directory/$archive");
  else actual=$(shasum -a 256 "$directory/$archive"); fi
  [[ "${actual%% *}" == "$checksum" ]] || { echo 'SHA-256 inválido; instalação cancelada.' >&2; return 1; }
  tar -xzf "$directory/$archive" -C "$directory" || return 1
  "$directory/go/bin/go" version || return 1
  mkdir -p "$(dirname "$target")" || return 1
  mv "$directory/go" "$target" || return 1
)

dependency_install_docker() {
  local current="$1"
  if [[ "$dependency_os" == Darwin ]]; then
    dependency_homebrew || return 1
    local action=install
    brew list --cask docker-desktop >/dev/null 2>&1 && action=upgrade
    dependency_confirm "Docker/Compose ausente ou incompatível ($current). Executar brew $action --cask docker-desktop. A atualização pode interromper containers; o Docker Desktop possui termos próprios." || return 1
    HOMEBREW_NO_AUTO_UPDATE=1 brew "$action" --cask docker-desktop || return 1
    export PATH="/Applications/Docker.app/Contents/Resources/bin:$PATH"
  else
    dependency_confirm "Docker/Compose ausente ou incompatível ($current). Configurar o repositório oficial download.docker.com e instalar/atualizar docker-ce, docker-ce-cli, containerd.io, docker-buildx-plugin e docker-compose-plugin via sudo/APT. Pode reiniciar o serviço e containers. Pacotes conflitantes não serão removidos automaticamente." || return 1
    local directory codename architecture
    directory=$(mktemp -d "${TMPDIR:-/tmp}/wagering-docker.XXXXXX") || return 1
    codename=$( . /etc/os-release; printf '%s' "${UBUNTU_CODENAME:-$VERSION_CODENAME}" )
    architecture=$(dpkg --print-architecture) || return 1
    dependency_sudo apt-get update || return 1
    dependency_sudo apt-get install -y ca-certificates curl || return 1
    # Reuse an existing official repository instead of adding conflicting Signed-By entries.
    if ! grep -Rqs "download.docker.com/linux/$dependency_distribution" /etc/apt/sources.list /etc/apt/sources.list.d; then
      curl -fsSL "https://download.docker.com/linux/$dependency_distribution/gpg" -o "$directory/docker.asc" || return 1
      printf 'Types: deb\nURIs: https://download.docker.com/linux/%s\nSuites: %s\nComponents: stable\nArchitectures: %s\nSigned-By: /etc/apt/keyrings/wagering-docker.asc\n' "$dependency_distribution" "$codename" "$architecture" > "$directory/docker.sources"
      dependency_sudo install -m 0755 -d /etc/apt/keyrings || return 1
      dependency_sudo install -m 0644 "$directory/docker.asc" /etc/apt/keyrings/wagering-docker.asc || return 1
      dependency_sudo install -m 0644 "$directory/docker.sources" /etc/apt/sources.list.d/wagering-docker.sources || return 1
    fi
    rm -rf -- "$directory"
    dependency_sudo apt-get update || return 1
    dependency_sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin || return 1
  fi
  hash -r
}

dependency_ensure() {
  local name="$1" minimum="$2" mode="$3" current=''
  current=$(dependency_version "$name") || current=''
  if dependency_version_at_least "$current" "$minimum"; then
    if [[ "$name" == uuidgen ]]; then echo 'OK: uuidgen (UUID válido)';
    else printf 'OK: %s %s (mínimo %s)\n' "$name" "$current" "$minimum"; fi
    return 0
  fi
  if [[ "$name" == uuidgen ]]; then echo 'PENDENTE: uuidgen ausente ou saída inválida';
  else printf 'PENDENTE: %s — atual %s; mínimo %s\n' "$name" "${current:-ausente/incompatível}" "$minimum"; fi
  [[ "$mode" != check ]] || return 1
  case "$name" in
    go) dependency_install_go "$minimum" "$current" || return 1; dependency_activate_go "$minimum" ;;
    docker|compose) dependency_install_docker "${current:-ausente}" || return 1 ;;
    *) dependency_package "$name" "$minimum" "$current" || return 1 ;;
  esac
  current=$(dependency_version "$name") || current=''
  dependency_version_at_least "$current" "$minimum" || { echo "$name continua incompatível; não é seguro continuar. Confira a versão e o PATH." >&2; return 1; }
  if [[ "$name" == uuidgen ]]; then echo 'VALIDADO: uuidgen (UUID válido)';
  else printf 'VALIDADO: %s %s\n' "$name" "$current"; fi
}

dependency_docker_ready() {
  local mode="$1" server context
  if ! docker info >/dev/null 2>&1; then
    [[ "$mode" != check ]] || { echo 'PENDENTE: Docker daemon inacessível.' >&2; return 1; }
    context=$(docker context show 2>/dev/null) || context=default
    case "$context" in default|desktop-linux) ;; *) echo "Docker inacessível no contexto $context; inicie ou corrija esse contexto e execute novamente." >&2; return 1;; esac
    if [[ "$dependency_os" == Darwin ]]; then
      dependency_confirm 'Iniciar Docker Desktop e aguardar o daemon (até 120 segundos).' || return 1
      open -a Docker || return 1
    else
      dependency_confirm 'Iniciar o serviço Docker com sudo systemctl start docker.' || return 1
      dependency_sudo systemctl start docker || return 1
    fi
    local attempt
    for ((attempt=0; attempt<60; attempt++)); do
      docker info >/dev/null 2>&1 && break
      sleep 2
    done
    docker info >/dev/null 2>&1 || { echo 'Docker continua inacessível. Verifique o contexto e a permissão no socket; o instalador não altera grupos ou permissões do socket.' >&2; return 1; }
  fi
  server=$(docker version --format '{{.Server.Version}}') || return 1
  server=${server%%-*}
  if ! dependency_version_at_least "$server" 24.0.0; then
    echo "Docker Server $server é incompatível (mínimo 24.0.0)." >&2
    [[ "$mode" != check ]] || return 1
    dependency_install_docker "$server (servidor)" || return 1
    server=$(docker version --format '{{.Server.Version}}') || return 1
    dependency_version_at_least "${server%%-*}" 24.0.0 || return 1
  fi
  printf 'OK: Docker Server %s\n' "$server"
}

dependency_check_all() {
  local required_go="$1" mode="${2:-install}" failed=0 item name minimum
  dependency_detect_platform || return 1
  dependency_activate_installed_tools
  dependency_activate_go "$required_go"
  for item in curl:7.68.0 git:2.23.0 jq:1.6.0 make:3.81.0 cc:10.0.0 uuidgen:1.0.0 "go:$required_go" docker:24.0.0 compose:2.20.0; do
    name=${item%%:*}; minimum=${item#*:}
    if [[ "$mode" == check ]]; then
      dependency_ensure "$name" "$minimum" check || failed=1
    else
      dependency_ensure "$name" "$minimum" install || return 1
    fi
  done
  if [[ "$failed" == 0 ]]; then dependency_docker_ready "$mode" || return 1; fi
  return "$failed"
}

main() {
  local destination="$PWD/wagering" ref=feature/ledger start=true mode=install
  while (($#)); do
    case "$1" in
      --dir|--ref)
        [[ $# -ge 2 && -n "$2" ]] || { echo "Missing value for $1" >&2; return 2; }
        if [[ "$1" == --dir ]]; then destination="$2"; else ref="$2"; fi
        shift 2 ;;
      --no-start) start=false; shift ;;
      --check) mode=check; shift ;;
      --help|-h)
        echo 'Usage: install.sh [--dir PATH] [--ref REF] [--no-start] [--check]'
        echo 'Checks dependency versions; asks for SIM in /dev/tty before installing or upgrading.'
        echo 'Supports macOS, Ubuntu and Debian. --check never installs, clones or starts services.'
        return ;;
      *) echo "Unknown option: $1" >&2; return 2 ;;
    esac
  done
  if [[ "$mode" == check ]]; then dependency_check_all 1.27.1 check; return; fi
  if [[ -e "$destination" || -L "$destination" ]]; then
    echo "Destination already exists: $destination. Use its scripts/setup.sh or choose --dir." >&2
    return 1
  fi
  dependency_detect_platform || return 1
  dependency_activate_installed_tools
  dependency_ensure curl 7.68.0 install || return 1
  dependency_ensure git 2.23.0 install || return 1
  git clone --single-branch --branch "$ref" -- https://github.com/JimSP/wagering.git "$destination"
  if [[ "$start" == true ]]; then bash "$destination/scripts/setup.sh";
  else bash "$destination/scripts/setup.sh" --no-start; fi
}
if [[ -z "${BASH_SOURCE[0]:-}" || "${BASH_SOURCE[0]}" == "$0" ]]; then main "$@"; fi
