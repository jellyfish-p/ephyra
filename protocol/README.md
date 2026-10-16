# Ephyra judge protocol

This directory is the only source of the node control protocol. Judge implementations
must depend on a published version of this module; they must not copy and evolve a
private proto.

Generate Go code:

```text
./generate.ps1
```

or:

```text
protoc -I . --go_out=. --go_opt=paths=source_relative \
  --go-grpc_out=. --go-grpc_opt=paths=source_relative \
  judge/v1/judge.proto
```

Incompatible major versions are rejected at handshake. `protocol_version` `1` is
the first release of this package.
