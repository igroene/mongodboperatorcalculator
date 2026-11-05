# Contributing

[Back to the README](README.md)

## Verification

Use Go 1.21 or newer. Before opening a pull request, format the code and run the checks:

```bash
gofmt -w src
go test ./...
go vet ./...
go build -o mongodboperatorcalculator ./src
git diff --check
```

Update the documentation when behavior changes. Keep the README focused on getting started; put reference material in the relevant guide under `docs/`.

## Documentation examples

`TestReadmeExamplesMatchOutput` checks the complete request/response pairs in [docs/examples.md](docs/examples.md) against real HTTP handler output, including error status codes. It also scans `README.md` and the other `docs/*.md` files for marked examples.

Each pair uses a unique name within its file, with markers directly before fenced code blocks:

    <!-- request:example-name -->
    ```json
    { ... }
    ```

    <!-- response:example-name status=422 -->
    ```json
    { ... }
    ```

The response status defaults to `200` when omitted. Use `ini` instead of `json` for human-output responses. Keep full responses in `docs/examples.md`; label any shorter excerpts elsewhere as excerpts, without the full-response markers.

After an intentional output change, regenerate the marked responses and review the diff:

```bash
go test ./src -run TestReadmeExamples -update-readme
git diff -- README.md docs
```

The existing `-update-readme` flag is retained for compatibility. It updates marked responses in all scanned documentation files, not just the README. Also review unmarked excerpts and the [operator mapping](docs/operator.md) if their values are affected.
