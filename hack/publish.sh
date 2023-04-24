#!/usr/bin/env bash

set -e

SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

repo_root_dir="${SCRIPT_DIR}/.."

for component_dir in "${repo_root_dir}"/components/*/; do
  component_name="$(basename "${component_dir}")"
  "${repo_root_dir}/bin/component-cli" ctf push "${repo_root_dir}"/gen/${component_name}/ctf
done
