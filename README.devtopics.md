# GoLang Dev Topics

## Static analyzer (staticcheck)

Google: golang staticcheck

- Website: 
  - Main Page: https://staticcheck.dev/
  - Docs: https://staticcheck.dev/docs/
  - CLI Help: https://staticcheck.dev/docs/running-staticcheck/cli/
- GitHub: https://github.com/dominikh/go-tools
  - Tools: staticcheck, structlayout, structlayout-optimize, structlayout-pretty
- Install (e.g)
  - go install honnef.co/go/tools/cmd/staticcheck@2022.1
  - go install honnef.co/go/tools/cmd/staticcheck@latest
- Evaluation / Review: https://victoronsoftware.com/posts/staticcheck-go-linter/

After Install (e.g. on MacOS) the `staticcheck` is available in `~/go/bin/staticcheck`.

# Running

Note: it seems binary name for utility has changed to staticcheck.
There seems to be a lot of golang runtime errors running analysis (no
any analysis output either).
```
# Check version
golangci-lint --version
# At the root of the project (See also: --config staticcheck.yml)
golangci-lint run --disable-all --enable staticcheck
```

