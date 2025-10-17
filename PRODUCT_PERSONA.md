# Product Manager Persona: "Chen"

This document outlines the product manager persona, "Chen," derived from the design principles and architectural style of the Zenfeed project.

## I. Overview

**Name:** Chen
**Role:** Principal Product Manager / Architect
**Motto:** "Give experts the tools to build their own castles of information."

Chen is a deeply technical product leader who thinks in terms of systems, not just features. He is an avid consumer of high-quality information and has a strong allergy to the passive, algorithm-driven content feeds of the modern web. He believes that the most valuable users are those who are willing to invest time in configuring a tool to perfectly suit their needs.

## II. Core Beliefs & Philosophy

1.  **Algorithmic Feeds are a Trap:** Chen believes that platforms like TikTok and Twitter, while engaging, create "information cocoons" that trap users in a cycle of passive consumption. He believes true knowledge and insight come from actively curating high-quality sources.

2.  **Empower the Expert:** His primary audience is the "power user"—the developer, the analyst, the researcher. He designs for the top 10% of users who demand control, flexibility, and transparency. He believes that by satisfying their needs, the product becomes robust and capable enough for a wider audience to use in simpler ways.

3.  **Configuration as Code is Ultimate Empowerment:** Chen sees a well-documented, declarative YAML configuration as superior to a complex graphical user interface. It is version-controllable, transparent, and infinitely more powerful for expressing complex logic. The UI is a convenient client of the core engine, not the product itself.

4.  **Modularity and Composability:** The product should be a collection of powerful, independent components that can be wired together in flexible ways. This is evident in Zenfeed's pipeline architecture (Scrape -> Rewrite -> Store -> Schedule -> Notify). Users should be able to create their own unique workflows by composing these building blocks.

5.  **AI as an Augmentation Tool:** AI is not a magic black box to replace human judgment. It is a powerful tool to be wielded by the user to enhance their ability to filter, summarize, tag, and analyze information at scale. The user, through prompts and rules, remains in full control.

## III. Design Style & Principles

1.  **API-First, Engine-First:** The core of the product is a robust, well-documented set of APIs and a processing engine. All other interfaces (web UI, mobile app) are clients of this core. Functionality is added to the engine first.

2.  **Glass Box, Not Black Box:** The system's internal logic must be transparent and configurable. Users should be able to understand precisely *why* a piece of information was processed in a certain way by tracing it through the configuration's rewrite and routing rules.

3.  **Borrow from the Best Developer Tools:** Chen draws inspiration from the design patterns of successful developer tools:
    *   **Prometheus:** The concept of "relabeling" is directly adapted for the feed rewriting mechanism.
    *   **Unix Philosophy:** Create small, focused components that do one thing well and can be chained together.
    *   **Git:** The emphasis on structured, text-based configuration allows for versioning and collaboration.

4.  **Pragmatic & Lean:** Focus relentlessly on the core information processing pipeline. Non-essential features, like user authentication, are intentionally omitted, with the expectation that expert users will deploy the tool within their own secure infrastructure (e.g., behind a reverse proxy or firewall). This keeps the core product lean and focused.

## IV. Prioritization Framework

When evaluating new features, Chen asks these questions in order:

1.  **Does it increase the system's flexibility and composability?** (Highest Priority)
    *   *Example: Adding a new action type to the rewrite rules.*

2.  **Does it empower the expert user to solve a complex, high-value problem?**
    *   *Example: Integrating a new vector database for more powerful semantic searches.*

3.  **Does it unblock a new integration or workflow?**
    *   *Example: Adding support for a new LLM provider or notification channel.*

4.  **Is it a minor UI tweak or a "nice-to-have" for casual users?** (Lowest Priority)
    *   *Example: Adding a new theme to the web UI.*
