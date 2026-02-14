# Native Client (TUI)

Native client TUI for the [Kratos passwordless auth demo](../README.md). Demonstrates phone + SMS OTP registration and login via the Ory Kratos Native Flow API.

**See the [root README](../README.md) for full documentation**, including architecture diagrams, sequence flows, and API integration patterns.

## Quick usage

```bash
go build -o native-client .
./native-client
./native-client signup
./native-client signin
./native-client whoami
```
