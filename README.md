# ocm

`ocm` is a GitHub project that streamlines the process of generating and managing component artifacts. 
The project uses a Makefile to automate the process and generates component descriptors in the `/gen/<component-name>` 
folder. Each component has its dedicated subfolder under the `/components` directory.

## Introduction

The `ocm` project simplifies the handling of component artifacts by automating the generation and management of 
component descriptors and their corresponding resources using a Makefile.

## Prerequisites

- [Go](https://golang.org/dl/) (version 1.17 or later)
- [Docker](https://www.docker.com/) (required for building container images and logging into the OCI registry)
- [Make](https://www.gnu.org/software/make/) (for executing the Makefile)

## Installation

```bash
git clone https://github.com/onmetal/ocm.git
```

Change to the `ocm` directory:

```bash
cd ocm
```

## Usage

1. Perform a Docker login to access the OCI registry:

```bash
docker login <oci-registry-url>
```

2. Generate component descriptors for each component:

```bash
make component-descriptor 
```

The generated component descriptors will be located in the `/gen/<component-name>` folder.

3. Publish the generated component descriptors to the registry:

```bash
make publish-component-descriptor
```

## Contributing

Contributions are welcome! If you'd like to contribute to OCM, please follow these steps:

1. Fork the repository.
2. Create a new branch for your feature or bugfix.
3. Make your changes and commit them to your branch.
4. Submit a pull request with a clear and concise description of your changes.

For more detailed information, please refer to the [contributing guidelines](CONTRIBUTING.md).

## License

OCM is licensed under the [Apache License, Version 2.0](LICENSE). 
For more details, please see the [license file](LICENSE) in the repository.
