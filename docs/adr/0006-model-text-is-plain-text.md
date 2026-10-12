# Model Text Is Plain Text, Escaped Whole

`render` escapes every `<`, `>` and `&` in the model's text (reasons, names,
owners, domain-model names) and every Markdown table separator in a cell.
Patterns in code spans escape only the separator and the backtick, and
Mermaid labels use Mermaid's entity codes.

Pinned by `TestADR_0006_ModelTextIsEscapedWhole` in `internal/text`.

## Context

`anarch.md` is meant to be committed and read on a code host, and a model can
be copied from another repository. A reason holding `<img onerror=…>` would
then run in a reader's browser. modelith's ADR-0014 faces the same risk, but
its prose fields are Markdown, so it escapes only the raw-HTML nodes a parser
finds. anarch.json's text fields are plain sentences, never Markdown, so
nothing in them needs to survive as markup.

## Decision

**Escape the three HTML-significant bytes everywhere, with no parser.** A
rule that escapes everything cannot miss a construct a parser would have
called markup.

## Consequences

A reason cannot use Markdown emphasis or links; it renders as written.
