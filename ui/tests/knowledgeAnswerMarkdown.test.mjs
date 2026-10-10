import assert from "node:assert/strict";
import test from "node:test";

import {
    parseInlineMarkdown,
    parseKnowledgeMarkdown,
    renderKnowledgeMarkdown,
} from "../src/support/knowledgeAnswerMarkdown.js";

function tokensOfType(tokens, type) {
    const matches = [];
    for (const token of tokens) {
        if (token.type == type) {
            matches.push(token);
        }
        if (token.children) {
            matches.push(...tokensOfType(token.children, type));
        }
    }
    return matches;
}

test("parses common knowledge-answer blocks", () => {
    const blocks = parseKnowledgeMarkdown(
        "## Summary\n\n- **First** item [1]\n- Second item\n\n> Source-backed answer\n\n```go\nfmt.Println(\"ok\")\n```",
    );

    assert.deepEqual(blocks.map((block) => block.type), ["heading", "list", "quote", "code"]);
    assert.equal(blocks[1].items.length, 2);
    assert.equal(tokensOfType(blocks[1].items[0], "strong").length, 1);
    assert.equal(blocks[3].language, "go");
});

test("keeps citations and incomplete streaming markers readable", () => {
    const tokens = parseInlineMarkdown("Answer [1], then **still streaming");

    assert.deepEqual(tokens, [{ type: "text", value: "Answer [1], then **still streaming" }]);
});

test("allows safe links and does not create unsafe link tokens", () => {
    const safe = parseInlineMarkdown("[Docs](https://example.com/docs)");
    const unsafe = parseInlineMarkdown("[Run](javascript:alert(1)) <script>alert(1)</script>");

    assert.equal(tokensOfType(safe, "link")[0].href, "https://example.com/docs");
    assert.equal(tokensOfType(unsafe, "link").length, 0);
    assert.equal(
        unsafe.map((token) => token.value || "").join(""),
        "[Run](javascript:alert(1)) <script>alert(1)</script>",
    );
});

test("renders formatted elements while escaping raw HTML", () => {
    globalThis.t = new Proxy({}, {
        get: (_target, tag) => (attrs, ...children) => {
            if (attrs && (typeof attrs != "object" || Array.isArray(attrs))) {
                children.unshift(attrs);
                attrs = null;
            }
            return { tag, attrs, children: children.flat(Infinity) };
        },
    });

    const html = renderKnowledgeMarkdown("- **First** item [1]\n- <script>alert(1)</script>")
        .map(serializeNode)
        .join("");

    assert.equal(
        html,
        "<ul><li><strong>First</strong> item [1]</li><li>&lt;script&gt;alert(1)&lt;/script&gt;</li></ul>",
    );
});

function serializeNode(node) {
    if (typeof node != "object" || node === null) {
        return escapeHtml(node);
    }
    const attributes = Object.entries(node.attrs || {})
        .filter(([, value]) => value !== undefined)
        .map(([name, value]) => ` ${name}="${escapeHtml(value)}"`)
        .join("");
    return `<${node.tag}${attributes}>${node.children.map(serializeNode).join("")}</${node.tag}>`;
}

function escapeHtml(value) {
    return String(value)
        .replaceAll("&", "&amp;")
        .replaceAll("\"", "&quot;")
        .replaceAll("<", "&lt;")
        .replaceAll(">", "&gt;");
}
