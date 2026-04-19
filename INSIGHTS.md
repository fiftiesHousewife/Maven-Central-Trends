# Insights: Maven Central in the Age of AI

Analysis of ~75,000 Maven Central namespaces (2-8 levels deep) tracked from April 2022 to March 2026, spanning the introduction of GitHub Copilot, GPT-4, Claude, Cursor, and other AI coding tools.

## The headline numbers

Maven Central group creation has **accelerated sharply** since mid-2024, but the story differs depending on what you count:

| Era | Monthly avg (all) | Monthly avg (truly new) | Extensions |
|-----|:-:|:-:|:-:|
| Pre-GPT-4 (Apr 2022 - Feb 2023) | 436 | 165 | 271 |
| GPT-4 year (Mar 2023 - Feb 2024) | 432 | 167 | 265 |
| Claude 3 + Cursor (Mar 2024 - Feb 2025) | 458 | 176 | 282 |
| Claude 4+ era (Mar 2025 - Mar 2026) | **613** | **247** | **366** |

![New Groups Per Month](docs/insights-all-groups.png)

Two signals emerge:

1. **Existing projects are more productive.** Extensions of existing namespaces — established organisations adding subgroups — grew from 271/month to 366/month, a **35% increase**. These are teams like Google, Apache, and Eclipse shipping more sub-projects.

2. **New entrants are surging.** Truly new namespaces — first-time publishers to Maven Central — grew from 165/month to 247/month, a **50% increase**.

### When did the acceleration actually happen?

The monthly truly-new counts tell a nuanced story:

| Period | Monthly truly new | Change |
|--------|:-:|:-:|
| Jun 2023 | 148 | baseline |
| Dec 2023 | 161 | +9% |
| Jun 2024 | 145 | flat |
| Dec 2024 | 208 | **+40%** |
| Jun 2025 | 267 | **+80%** |
| Dec 2025 | 230 | +55% |
| Mar 2026 | 489 | **+230%** |

**2024 was not a step change.** Despite the launch of Claude 3 (March), GPT-4o (May), Claude 3.5 Sonnet (June), and Cursor (August), the monthly new namespace count stayed flat through mid-2024 (~145-165/month). The real acceleration began in late 2024 and exploded in 2025-2026.

This lag makes sense: AI coding tools don't instantly create Maven Central publishers. Developers first use them for existing projects, then build confidence, then start new projects, then figure out Maven publishing. The 6-12 month delay between tool availability and publishing surge is consistent with this adoption curve.

It's also possible the growth has nothing to do with AI tools at all — it could reflect JitPack's growing adoption, changes to Maven Central publishing requirements, or other ecosystem shifts. **Correlation is not causation**, and we cannot attribute the growth to AI tools from this data alone.

## Where the new groups come from

The fastest-growing prefixes in the last 12 months tell the story:

| Prefix | New subgroups | What it is |
|--------|:-:|-----------|
| `io.github.*` | 3,074 | JitPack — GitHub repos auto-published as Maven artifacts |
| `io.gitee.*` | 81 | Chinese equivalent of JitPack |
| `com.github.*` | 56 | Older JitPack convention |
| `org.machanism.*` | 42 | Single project with deep namespace |
| `org.finos.*` | 24 | Financial open-source foundation |
| `io.quarkiverse.*` | 22 | Quarkus community extensions |
| `org.eclipse.*` | 20 | Eclipse Foundation projects |
| `org.apache.*` | 18 | Apache Software Foundation |

**JitPack dominates.** `io.github.*` alone accounts for 3,074 of the new subgroups — that's individual developers publishing their GitHub repos as Maven packages with zero setup. This is the clearest democratisation signal: the barrier to publishing on Maven Central has effectively dropped to "push to GitHub."

## The one-and-done problem

Not all new groups represent sustained projects. The **one-and-done rate** — groups that published exactly one version and never updated — is rising:

| Era | Groups | One-and-done | Rate |
|-----|:-:|:-:|:-:|
| Pre-GPT-4 | 4,810 | 1,236 | 25.7% |
| GPT-4 year | 5,106 | 1,358 | 26.6% |
| Claude 3 + Cursor | 5,490 | 1,667 | **30.4%** |
| Claude 4+ | 8,689 | 2,829 | **32.6%** |

The one-and-done rate has risen from 26% to 33%. This tracks with the democratisation thesis: more people are publishing, but a higher fraction publish once and walk away. These could be:
- AI-generated experimental projects
- Homework/tutorial artifacts
- Auto-published repos that were never intended as libraries
- Projects that moved to a different package manager

## Are groups getting smaller?

| Era | Avg artifacts | Avg versions | Groups (12+ months old) | Avg versions (age-controlled) |
|-----|:-:|:-:|:-:|:-:|
| Pre-GPT-4 | 7.6 | 12.0 | 4,810 | 12.0 |
| GPT-4 year | 8.6 | 11.1 | 5,106 | 11.2 |
| Claude 3 + Cursor | 9.7 | 8.4 | 5,985 | 8.4 |
| Claude 4+ | 9.3 | 6.6 | (too recent) | — |

Average artifacts per group is slightly increasing (~8 to ~10), but **average versions is falling**. 

**Important caveat on versions**: The Claude 4+ era shows only 6.6 average versions, but these groups are less than 12 months old — they simply haven't had time to publish many versions yet. However, even when controlling for age (comparing only groups 12+ months old), post-2024 groups average 8.4 versions vs 12.0 for pre-GPT-4 groups of similar age. **The decline is real, not just an artefact of recency.** Possible explanations: more experimental/one-off projects, faster iteration meaning fewer but larger releases, or a genuine shift in maintenance patterns.

## License trends

![License Distribution](docs/insights-licenses.png)

| License | Pre-AI tools | AI era | Shift |
|---------|:-:|:-:|:-:|
| Apache-2.0 | 57% | 57% | Stable |
| MIT | 16% | 24% | **+8pp** |
| non-standard | 18% | 11% | -7pp |
| GPL-3.0 | 3% | 2% | Stable |

MIT is gaining share at the expense of non-standard/custom licenses. The Java ecosystem has traditionally favoured Apache-2.0, but newer (often solo) developers default to MIT — the license most AI tools suggest.

## Source transparency

![Source Repo Presence](docs/insights-repos.png)

Source repository linkage has remained remarkably stable at **~96%** across all eras. Almost every Maven Central group links to a GitHub repo. This is partly a JitPack effect (the repo IS the source), partly because modern build tools (Maven Central Publisher, Gradle) require or encourage POM metadata including SCM links.

## Security

Of ~75,000 OSV-enriched groups, **702 (0.9%) have known CVEs**. The batch OSV API queries all artifacts per group, catching vulnerabilities that single-artifact scans miss. CVEs are concentrated in widely-used packages:

| Group | CVEs | Severity |
|-------|:-:|---------|
| `com.fasterxml.jackson.core` | 69 | CRITICAL |
| `org.apache.struts` | 60 | CRITICAL |
| `org.springframework` | 16+ | HIGH |
| `io.netty` | 12 | CRITICAL |
| `org.bouncycastle` | 9 | MODERATE |

An open question: are AI-era groups more or less likely to depend on vulnerable packages? This requires dependency graph analysis beyond what we currently track.

## Open questions

1. **Solo committer rate** — once GitHub enrichment completes, what % of new groups are single-developer projects? Is this the "democratisation" signal we expect?
2. **JitPack vs traditional publishing** — is the growth real library creation, or just GitHub repos being auto-published?
3. **Quality metrics** — for the top 100 most-depended-on groups, do commit frequency, issue resolution, and code size differ between pre-AI and AI-era groups?
4. **Dependency patterns** — do AI-era groups have more or fewer dependencies? Shallower or deeper dependency trees?

## Methodology

- **Data source**: repo1.maven.org for enumeration, deps.dev for version history and metadata, OSV for CVEs, Sonatype Central Portal for popularity
- **Coverage**: ~75,000 groups across 26 top-level prefixes, scanned to 8 levels deep
- **Time range**: April 2022 — March 2026 (48 months)
- **"Truly new" definition**: A group whose 2-level parent (e.g. `com.example`) was first published in the same month — meaning the whole namespace is new, not a subgroup of an established project
- **"One-and-done" definition**: A group whose primary artifact has total_versions <= 1 from deps.dev
- **Outlier handling**: Groups with 500+ artifacts (e.g. org.mvnpm) are capped in trend calculations

---

*Data collected by [Maven Central Trends](README.md). Interactive charts at `http://localhost:8080`. Last updated April 2026.*
