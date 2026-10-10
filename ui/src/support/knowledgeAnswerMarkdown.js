const unorderedListPattern = /^ {0,3}[-+*]\s+(.+)$/;
const orderedListPattern = /^ {0,3}(\d+)[.)]\s+(.+)$/;

export function parseKnowledgeMarkdown(value) {
    const lines = String(value || "").replace(/\r\n?/g, "\n").split("\n");
    const blocks = [];
    let index = 0;

    while (index < lines.length) {
        const line = lines[index];
        if (!line.trim()) {
            index++;
            continue;
        }

        const fence = line.match(/^ {0,3}```\s*([\w+-]*)\s*$/);
        if (fence) {
            const code = [];
            index++;
            while (index < lines.length && !/^ {0,3}```\s*$/.test(lines[index])) {
                code.push(lines[index]);
                index++;
            }
            if (index < lines.length) {
                index++;
            }
            blocks.push({
                type: "code",
                language: fence[1].toLowerCase(),
                value: code.join("\n"),
            });
            continue;
        }

        const heading = line.match(/^ {0,3}(#{1,6})\s+(.+?)\s*#*\s*$/);
        if (heading) {
            blocks.push({
                type: "heading",
                level: heading[1].length,
                children: parseInlineMarkdown(heading[2]),
            });
            index++;
            continue;
        }

        if (/^ {0,3}(?:-{3,}|_{3,}|\*{3,})\s*$/.test(line)) {
            blocks.push({ type: "rule" });
            index++;
            continue;
        }

        if (/^ {0,3}>/.test(line)) {
            const quote = [];
            while (index < lines.length) {
                const match = lines[index].match(/^ {0,3}>\s?(.*)$/);
                if (!match) {
                    break;
                }
                quote.push(match[1]);
                index++;
            }
            blocks.push({ type: "quote", children: parseInlineMarkdown(quote.join("\n")) });
            continue;
        }

        const unorderedItem = line.match(unorderedListPattern);
        const orderedItem = line.match(orderedListPattern);
        if (unorderedItem || orderedItem) {
            const ordered = !!orderedItem;
            const items = [];
            const start = ordered ? Number(orderedItem[1]) : 1;

            while (index < lines.length) {
                const match = lines[index].match(ordered ? orderedListPattern : unorderedListPattern);
                if (match) {
                    items.push(match[ordered ? 2 : 1]);
                    index++;
                    continue;
                }
                if (items.length && /^\s{2,}\S/.test(lines[index])) {
                    items[items.length - 1] += `\n${lines[index].trim()}`;
                    index++;
                    continue;
                }
                break;
            }

            blocks.push({
                type: "list",
                ordered,
                start,
                items: items.map((item) => parseInlineMarkdown(item)),
            });
            continue;
        }

        const paragraph = [line.trim()];
        index++;
        while (index < lines.length && lines[index].trim() && !startsBlock(lines[index])) {
            paragraph.push(lines[index].trim());
            index++;
        }
        blocks.push({ type: "paragraph", children: parseInlineMarkdown(paragraph.join("\n")) });
    }

    return blocks;
}

export function parseInlineMarkdown(value, depth = 0) {
    const input = String(value || "");
    if (!input || depth >= 8) {
        return input ? [{ type: "text", value: input }] : [];
    }

    const tokens = [];
    let index = 0;
    const appendText = (text) => {
        if (!text) {
            return;
        }
        const previous = tokens[tokens.length - 1];
        if (previous?.type == "text") {
            previous.value += text;
        } else {
            tokens.push({ type: "text", value: text });
        }
    };

    while (index < input.length) {
        if (input[index] == "\\" && index + 1 < input.length) {
            appendText(input[index + 1]);
            index += 2;
            continue;
        }

        if (input[index] == "\n") {
            tokens.push({ type: "break" });
            index++;
            continue;
        }

        if (input[index] == "`") {
            let markerLength = 1;
            while (input[index + markerLength] == "`") {
                markerLength++;
            }
            const marker = "`".repeat(markerLength);
            const closing = findUnescaped(input, marker, index + markerLength);
            if (closing >= 0) {
                tokens.push({
                    type: "code",
                    value: input.slice(index + markerLength, closing).replace(/\n/g, " "),
                });
                index = closing + markerLength;
                continue;
            }
        }

        if (input[index] == "[") {
            const labelEnd = findUnescaped(input, "](", index + 1);
            const hrefEnd = labelEnd >= 0 ? findUnescaped(input, ")", labelEnd + 2) : -1;
            if (labelEnd >= 0 && hrefEnd >= 0) {
                const href = input.slice(labelEnd + 2, hrefEnd).trim();
                if (isSafeHref(href)) {
                    tokens.push({
                        type: "link",
                        href,
                        children: parseInlineMarkdown(input.slice(index + 1, labelEnd), depth + 1),
                    });
                    index = hrefEnd + 1;
                    continue;
                }
                appendText(input.slice(index, hrefEnd + 1));
                index = hrefEnd + 1;
                continue;
            }
        }

        if (input[index] == "<") {
            const closing = input.indexOf(">", index + 1);
            const href = closing >= 0 ? input.slice(index + 1, closing).trim() : "";
            if (closing >= 0 && isSafeHref(href)) {
                tokens.push({ type: "link", href, children: [{ type: "text", value: href }] });
                index = closing + 1;
                continue;
            }
        }

        const pairedMarker = input.slice(index, index + 2);
        if (["**", "__", "~~"].includes(pairedMarker)) {
            const closing = findUnescaped(input, pairedMarker, index + 2);
            if (closing > index + 2) {
                tokens.push({
                    type: pairedMarker == "~~" ? "delete" : "strong",
                    children: parseInlineMarkdown(input.slice(index + 2, closing), depth + 1),
                });
                index = closing + 2;
                continue;
            }
        }

        if (input[index] == "*" || input[index] == "_") {
            const marker = input[index];
            const closing = findUnescaped(input, marker, index + 1);
            if (closing > index + 1) {
                tokens.push({
                    type: "emphasis",
                    children: parseInlineMarkdown(input.slice(index + 1, closing), depth + 1),
                });
                index = closing + 1;
                continue;
            }
        }

        appendText(input[index]);
        index++;
    }

    return tokens;
}

export function renderKnowledgeMarkdown(value) {
    return parseKnowledgeMarkdown(value).map(renderBlock);
}

function startsBlock(line) {
    return /^ {0,3}(?:```|#{1,6}\s|>|(?:[-+*]\s+)|(?:\d+[.)]\s+)|(?:-{3,}|_{3,}|\*{3,})\s*$)/.test(line);
}

function findUnescaped(input, marker, start) {
    let index = input.indexOf(marker, start);
    while (index >= 0) {
        let slashCount = 0;
        for (let cursor = index - 1; cursor >= 0 && input[cursor] == "\\"; cursor--) {
            slashCount++;
        }
        if (slashCount % 2 == 0) {
            return index;
        }
        index = input.indexOf(marker, index + marker.length);
    }
    return -1;
}

function isSafeHref(value) {
    return /^(?:https?:\/\/|mailto:)/i.test(value);
}

function renderBlock(block) {
    switch (block.type) {
        case "heading": {
            const children = renderInlineTokens(block.children);
            switch (block.level) {
                case 1:
                    return t.h1(null, ...children);
                case 2:
                    return t.h2(null, ...children);
                case 3:
                    return t.h3(null, ...children);
                case 4:
                    return t.h4(null, ...children);
                case 5:
                    return t.h5(null, ...children);
                default:
                    return t.h6(null, ...children);
            }
        }
        case "list": {
            const children = block.items.map((item) => t.li(null, ...renderInlineTokens(item)));
            return block.ordered
                ? t.ol({ start: block.start == 1 ? undefined : block.start }, ...children)
                : t.ul(null, ...children);
        }
        case "quote":
            return t.blockquote(null, ...renderInlineTokens(block.children));
        case "code":
            return t.pre(
                null,
                t.code(
                    { className: block.language ? `language-${block.language}` : undefined },
                    block.value,
                ),
            );
        case "rule":
            return t.hr();
        default:
            return t.p(null, ...renderInlineTokens(block.children));
    }
}

function renderInlineTokens(tokens) {
    return tokens.map((token) => {
        switch (token.type) {
            case "break":
                return t.br();
            case "code":
                return t.code(null, token.value);
            case "strong":
                return t.strong(null, ...renderInlineTokens(token.children));
            case "emphasis":
                return t.em(null, ...renderInlineTokens(token.children));
            case "delete":
                return t.del(null, ...renderInlineTokens(token.children));
            case "link":
                return t.a(
                    {
                        href: token.href,
                        target: /^https?:\/\//i.test(token.href) ? "_blank" : undefined,
                        rel: /^https?:\/\//i.test(token.href) ? "noopener noreferrer" : undefined,
                    },
                    ...renderInlineTokens(token.children),
                );
            default:
                return token.value;
        }
    });
}
