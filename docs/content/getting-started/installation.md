---
title: "Installation"
description: "Install nps from a release, with go install, or from source."
weight: 20
---

## Prebuilt binaries

Every [release](https://github.com/tamnd/nps-cli/releases) carries archives for Linux, macOS,
and Windows on amd64 and arm64, plus deb, rpm, and apk packages for Linux.
Download, unpack, put `nps` on your `PATH`, done. The `checksums.txt`
on each release is signed with keyless [cosign](https://docs.sigstore.dev/) if
you want to verify before running.

## With Go

```bash
go install github.com/tamnd/nps-cli/cmd/nps@latest
```

That puts `nps` in `$(go env GOPATH)/bin`, which is `~/go/bin` unless
you moved it. Make sure that directory is on your `PATH`.

## From source

```bash
git clone https://github.com/tamnd/nps-cli
cd nps-cli
make build        # produces ./bin/nps
./bin/nps version
```

## Container image

```bash
docker run --rm ghcr.io/tamnd/nps:latest --help
```

## Checking the install

```bash
nps version
```

prints the version and exits.
