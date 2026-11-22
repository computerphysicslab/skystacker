# Multi-provider audit benchmark — SkyStacker

**Date:** 2026-11-22 · **Toolchain:** `myTools/DocSpider/serializer.py` +
`myTools/AI/AI-API/ai-api.py` · **Subject:** SkyStacker v0.6.1 full code +
canonical docs.

## Method

1. A staging tree was built with all root `*.go`, the first-party modules
   (`goPureRaw`, `goTiff`, `goDiagnostics`) and the key Markdown docs;
   `catalogs/` (29 MB) and the local datasets (2.8 GB) were excluded.
2. `serializer.py` produced a **1,887,719-byte / 53,481-line** full
   serialization (Go source was not previously extracted: `.go`, `.mod`,
   `.sum`, `.sh` were added to `TEXT_EXTS` in `DocSpider.py` for this run).
3. A deep-audit prompt (9 mandatory sections, incl. ≥12 new roadmap items
   with IDs from 111, effort and falsifiable acceptance criteria) was
   prepended. The full prompt was **1,891,309 bytes**.
4. The full prompt was rejected by every provider for context/TPM limits, so
   four tiered bundles were generated with the same serializer:

   | Bundle | Bytes | Content |
   |---|---:|---|
   | A | 753,318 | all non-`main.go` Go sources + all docs + `main.go` function map |
   | B | 309,756 | first-party module sources + key docs + function map |
   | C | 296,214 | key docs + function map + 15 selected root helper `.go` files |
   | D | 88,245 | doc extracts + condensed function map |
   | E (mini) | 15,650 | pending-roadmap table + spec core + scope note |

5. Each provider was invoked with `-p <provider> --strict --temperature 0.3`,
   `--max-output-tokens 16000` (6,000 for Ollama run 3), sequential rotation
   disabled, and wall-clock time measured externally. Failed attempts were
   retried with a smaller bundle; only the successful attempt is scored below.

## Results (successful runs only)

| Provider | Model | Bundle | Prompt bytes | Latency | Output bytes | Lines | Words | Quality (0–10) |
|---|---|---|---:|---:|---:|---:|---:|---:|
| gemini | gemini-3.1-flash-lite | B | 313,346 | **8.79 s** | 6,798 | 86 | 977 | 6.5 |
| groq | openai/gpt-oss-120b | E (mini) | 16,347 | 5.49 s | 7,465 | 72 | 1,140 | 5.5 |
| grok | grok-4.7 | A | 756,908 | 525.77 s | **35,262** | **241** | **5,093** | **9.0** |
| openrouter | meta-llama/llama-3.3-70b-instruct | C | 299,804 | 90.19 s | 7,352 | 67 | 1,068 | 4.0 |
| ollama | qwen3:8b | E (mini) | 16,347 | 1,190.11 s | 3,896 | 80 | 482 | 2.0 |

Failed attempts (not scored): full prompt rejected by gemini (HTTP 429,
free-tier 250k input-token/min quota), groq (HTTP 413 context), grok
(641k tokens > 500k context), openrouter (521k > 131k context), ollama
(900 s timeout); second attempt: groq HTTP 413 TPM limit 8,000 tokens/min
with bundle C, ollama 900 s timeout with bundle D.

## Quality rubric

Completeness of the 9 requested sections (0–2), specificity with file/line
references and falsifiable acceptance criteria (0–3), respect for project
constraints — no new deps, no SIMD/GPU/ML, no split of `package main` (0–2),
novelty/deduplication of roadmap items (0–2), format compliance (0–1).

- **grok (9.0):** the only audit that read the full satellite sources and the
  function map; all 9 sections present; 20 new items (IDs 111–130) with
  precise, falsifiable acceptance tests; explicitly excluded duplicates and
  rejected technologies; labelled unverified `main.go` claims as hypotheses.
  Main weakness: 8m46s latency and no access to the `main.go` body.
- **gemini (6.5):** fast, well-structured, 12 items (111–122), but several
  proposals violate explicit constraints (SIMD/AVX2, `mmap`, CR3 CRX decoder,
  Bayer drizzle, multi-night mosaic, full-sky blind solver duplicate) and
  several acceptance criteria are weak.
- **groq (5.5):** compact audit forced by an 8,000 TPM cap; 10 items
  (111–120) with mostly good acceptance criteria; item 112 proposes a `cmd`
  package that would split the root package (forbidden). No full code seen.
- **openrouter (4.0):** clean structure but generic, non-falsifiable roadmap
  items ("improve error handling", "optimize algorithms", "add UI"); no IDs,
  no acceptance criteria. Adds little beyond common sense.
- **ollama (2.0):** 19m50s on a 16 KB prompt, generic conversational answer,
  no requested section structure and no roadmap IDs. Not usable for this task
  on this host (CPU-only qwen3:8b).

## Conclusions

1. **grok is the clear winner for depth**, gemini for speed. The best workflow
   is grok for the deep pass + gemini for a quick cross-check.
2. The pre-flight `--check` table is necessary but not sufficient: groq's
   8,000 TPM cap and each provider's context window must be checked before
   choosing a bundle. Always prepare tiered bundles.
3. Local Ollama (qwen3:8b) is not viable for code auditing here: ~20 minutes
   for a shallow answer. Reserve it for tiny classification tasks.
4. Voice/model rotation is not needed when using `--strict`; a rotation would
   have silently produced duplicate audits from the same provider.
5. The integrated findings are in `roadmap-update.md`.
