# Workspace Scan Example

This folder contains a customer-facing multi-repository workspace input file for `gco11y-size`.

Run from the repository root:

```sh
go run ./cmd/gco11y-size scan \
  --input example/example.json \
  --out example/workspace.html \
  --json example/workspace.json
```

The generated `workspace.html` and `workspace.json` files are ignored by Git so the example can be re-run locally without changing the working tree.
