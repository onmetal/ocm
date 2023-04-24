#!/usr/bin/env bash

set -e

SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"
repo_root_dir="${SCRIPT_DIR}/../"

for component_dir in "${repo_root_dir}"/components/*/; do
  component_name="$(basename "${component_dir}")"
  echo "Generating component descriptor for '${component_name}'"

  component_descriptor_file="${component_dir}/component-descriptor.yaml"
  version=$(yq e '.component.version' "${component_descriptor_file}")

  echo "Found ${version} for component '${component_name}'"

  # download the image vector if available
  url="https://raw.githubusercontent.com/onmetal/${component_name}/main/charts/images.yaml?ref=${version}"
  component_chart_dir="${repo_root_dir}/gen/${component_name}/charts"
  component_image_vector_file="${component_chart_dir}/images.yaml"
  mkdir -p "${component_chart_dir}"

  echo "Downloading image vector"
  http_status_code=$(curl -s -o "${component_image_vector_file}" -w '%{http_code}' "$url")
  if [[ "$http_status_code" -eq 200 ]]; then
      echo "File downloaded successfully."
  else
      # Remove the partially downloaded file, if any
      rm -f "$component_image_vector_file"
      # Print the error message
      echo "Error: Unable to download the file. HTTP status code: $http_status_code"
      echo "Looks like this component does not have an image vector"
  fi

  echo "Enriching component descriptor from '${component_name}'"
  descriptor_out_dir="${repo_root_dir}/gen/${component_name}/"
  mkdir -p "${descriptor_out_dir}"
  descriptor_out_file="${descriptor_out_dir}/component-descriptor.yaml"
  cp "${component_descriptor_file}" "${descriptor_out_file}"

  if [[ -f "${component_image_vector_file}" ]]; then
    if [[ -z "${GENERIC_DEPENDENCIES}" ]]; then
      GENERIC_DEPENDENCIES="hyperkube,kube-apiserver,kube-controller-manager,kube-scheduler,kube-proxy"
    fi

    if [[ -z "${COMPONENT_CLI_ARGS}" ]]; then
      COMPONENT_CLI_ARGS="
      --comp-desc "${descriptor_out_file}" \
      --image-vector "${component_image_vector_file}" \
      --component-prefixes "${COMPONENT_PREFIXES}" \
      --generic-dependencies "${GENERIC_DEPENDENCIES}" \
      "
    fi

    # translates all images defined the images.yaml into component descriptor resources.
    # For detailed documentation see https://github.com/gardener/component-cli/blob/main/docs/reference/components-cli_image-vector_add.md
    "${repo_root_dir}/bin/component-cli" image-vector add ${COMPONENT_CLI_ARGS}
  fi

  "${repo_root_dir}/bin/component-cli" ctf add -f "${descriptor_out_dir}" "${descriptor_out_dir}/ctf"
done
