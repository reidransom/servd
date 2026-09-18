---
title: Tabs restoration fixture
permalink: /tabs-restoration/
---

This page verifies that a persistent synchronized selection restores on another page. Choose a package manager on the [main tabs fixture](../tabs/), then follow its Yarn-panel link here or navigate here directly.

{% capture npm_restore %}
The restored npm panel.
{% endcapture %}
{% capture yarn_restore %}
The restored Yarn panel with a [focusable return link](../tabs/).
{% endcapture %}
{% capture pnpm_restore %}
The restored pnpm panel.
{% endcapture %}
{% capture restore_tabs %}
{% include components/tab.html label="Yarn" content=yarn_restore %}
{% include components/tab.html label="npm" content=npm_restore %}
{% include components/tab.html label="pnpm" content=pnpm_restore %}
{% endcapture %}
{% include components/tabs.html content=restore_tabs sync="package-manager" persist=true %}
