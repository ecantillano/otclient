# Launcher contract evals

`cases.json` is the release-contract corpus. It covers strict semantic versions,
cross-platform path safety, protocol 1525, stable/test environment binding,
endpoint flags, and trusted release-note hosts.

Run the periodic lane with:

```sh
go test -tags=eval ./evals
```

Adding a release field or accepting a new path form requires a positive fixture
and at least one adjacent negative fixture. These evals are deterministic and do
not contact the network.
