---
title: الدليل العربي
lang: ar
translation_key: guide
permalink: /dalil/
rtl_tree:
  - name: src
    type: directory
    open: true
    comment: مجلد المصدر
    children:
      - name: components
        type: directory
        open: true
        children:
          - name: LanguagePicker.ts
            type: file
            highlight: true
      - name: إعدادات-config.toml
        type: file
        comment: اسم ملف يجمع العربية وLatin
  - name: README-العربية.md
    type: file
---

## قسم عربي مع `Starlyt CLI`

هذا نص عربي طويل يذكر المنتج Starlyt والإصدار 2026 والرقم 12345، ويربط إلى
<a href="https://example.test/docs/v2/?query=rtl#section"><bdi dir="ltr">https://example.test/docs/v2/?query=rtl#section</bdi></a> من دون عكس الأحرف أو الأرقام.
استخدم الملف `config.toml` ثم شغّل الأمر `starlyt build --baseurl /docs`.

### عنوان متداخل يجمع العربية و`inline-code`

يبقى ترتيب DOM وترتيب التركيز كما كُتبا، حتى عندما تنتقل مناطق التخطيط إلى
الجانبين المنطقيين المقابلين.

#### عنوان أعمق مع Latin API و42

<p class="rtl-icon-fixture">
{% include components/icon.html name="right-arrow" label="السابق" %}
{% include components/icon.html name="external" label="رابط خارجي" %}
{% include components/icon.html name="translate" label="ترجمة" %}
</p>
{% include components/callout.html type="danger" content="محتوى التنبيه العربي مع رابط https://example.test واسم Starlyt." %}

| المرحلة | الأمر | المسار | الإصدار | الحالة | ملاحظة طويلة |
| --- | --- | --- | --- | --- | --- |
| البناء | `jigyll build` | `/docs/ar/dalil/` | 2026.9 | ناجح | هذا عمود طويل يثبت أن الجدول العريض يمرر محليًا من دون توسيع الصفحة |
| النشر | `git push` | `https://example.test/repository` | v2.4.1 | جاهز | Starlyt-ABCDEFGHIJKLMNOPQRSTUVWXYZ-abcdefghijklmnopqrstuvwxyz-0123456789-unbroken-table-cell-validates-local-scrolling |

<img src="{{ '/assets/rtl-layout.svg' | relative_url }}" alt="رسم توضيحي متجاوب لاختبار RTL" width="960" height="360">

{% capture arabic_tab %}<p>لوحة عربية تحتوي الرقم 987 والملف <code>package.json</code>.</p>{% endcapture %}
{% capture latin_tab %}<p>Starlyt API response: <code>status=ready</code>.</p>{% endcapture %}
{% capture rtl_tabs %}{% include components/tab.html label="نظرة عامة" content=arabic_tab %}{% include components/tab.html label="Starlyt API" content=latin_tab %}{% endcapture %}
{% include components/tabs.html content=rtl_tabs %}

{% include components/file-tree.html items=page.rtl_tree %}

```text
const endpoint = "https://example.test/docs/v2";
const version = 2026;
# تعليق عربي محفوظ بترتيب المصدر
abcdefghijklmnopqrstuvwxyz-0123456789-ABCDEFGHIJKLMNOPQRSTUVWXYZ-this-line-scrolls-inside-the-code-frame
```
