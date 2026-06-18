const httpMethodOptions = [
    { value: "GET", label: "GET" },
    { value: "HEAD", label: "HEAD" },
    { value: "POST", label: "POST" },
    { value: "PUT", label: "PUT" },
    { value: "PATCH", label: "PATCH" },
    { value: "DELETE", label: "DELETE" },
    { value: "OPTIONS", label: "OPTIONS" },
];

export function httpStepForm(propsArg = {}) {
    const props = store({
        step: null,
        error: null,
        triggerType: "",
        triggerCollectionRef: "",
    });

    const watchers = app.utils.extendStore(props, propsArg);

    return t.div(
        {
            className: "grid automation-http-step-form",
            onunmount: () => {
                watchers.forEach((w) => w?.unwatch());
            },
        },
        t.div(
            { className: "col-12" },
            t.div(
                { className: "flex gap-5 flex-wrap m-b-xs" },
                t.button(
                    {
                        type: "button",
                        className: "btn sm secondary",
                        onclick: () => openCurlImportModal(props.step),
                    },
                    t.i({ className: "ti ti-terminal", ariaHidden: true }),
                    t.span({ className: "txt" }, "Import cURL"),
                ),
            ),
        ),
        t.div(
            { className: "col-md-3" },
            t.div(
                { className: "field" },
                t.label({ htmlFor: `${props.step.__id}_method` }, "Method"),
                app.components.select({
                    id: `${props.step.__id}_method`,
                    value: () => props.step.method,
                    options: httpMethodOptions,
                    onchange: (selected) => {
                        props.step.method = selected?.[0]?.value || "GET";
                    },
                }),
            ),
        ),
        t.div(
            { className: "col-md-9" },
            t.div(
                { className: "field" },
                t.label({ htmlFor: `${props.step.__id}_url` }, "URL"),
                app.components.automationInput({
                    id: `${props.step.__id}_url`,
                    singleLine: true,
                    placeholder: "https://example.com/hooks",
                    value: () => props.step.url,
                    triggerType: () => props.triggerType,
                    triggerCollectionRef: () => props.triggerCollectionRef,
                    oninput: (value) => (props.step.url = value),
                }),
            ),
        ),
        t.div(
            { className: "col-md-4" },
            t.div(
                { className: "field" },
                t.label({ htmlFor: `${props.step.__id}_timeout` }, "Timeout (seconds)"),
                t.input({
                    id: `${props.step.__id}_timeout`,
                    type: "number",
                    min: 1,
                    step: 1,
                    placeholder: "30",
                    value: () => props.step.timeoutText,
                    oninput: (e) => (props.step.timeoutText = e.target.value),
                }),
            ),
            t.div({ className: "field-help" }, "Leave empty to use the default safe client timeout."),
        ),
        t.div(
            { className: "col-lg-12" },
            t.div(
                { className: "field" },
                t.label({ htmlFor: `${props.step.__id}_headers` }, "Headers JSON"),
                app.components.automationInput({
                    id: `${props.step.__id}_headers`,
                    className: "txt-code",
                    language: "js",
                    value: () => props.step.headersText,
                    triggerType: () => props.triggerType,
                    triggerCollectionRef: () => props.triggerCollectionRef,
                    placeholder: `{\n  "X-Automation": "{{trigger.type}}"\n}`,
                    oninput: (value) => (props.step.headersText = value),
                }),
            ),
            t.div({ className: "field-help" }, "Headers must be a JSON object."),
        ),
        t.div(
            { className: "col-lg-12" },
            t.div(
                { className: "field" },
                t.label({ htmlFor: `${props.step.__id}_body` }, "Body"),
                app.components.automationInput({
                    id: `${props.step.__id}_body`,
                    className: "txt-code",
                    language: "js",
                    value: () => props.step.bodyText,
                    triggerType: () => props.triggerType,
                    triggerCollectionRef: () => props.triggerCollectionRef,
                    placeholder: `{\n  "automation": "{{automation.name}}",\n  "recordId": "{{record.id}}"\n}`,
                    oninput: (value) => (props.step.bodyText = value),
                }),
            ),
            t.div(
                { className: "field-help" },
                "Body may be plain text or JSON. Template placeholders are resolved recursively.",
            ),
        ),
    );
}

function openCurlImportModal(step) {
    const modal = curlImportModal(step);

    document.body.appendChild(modal);
    app.modals.open(modal);
}

function curlImportModal(step) {
    const uniqueId = "curl_import_" + app.utils.randomString();
    const data = store({
        command: "",
        error: "",
    });

    let modal;

    function importCommand() {
        data.error = "";

        try {
            const parsed = parseCurlCommand(data.command);

            step.method = parsed.method;
            step.url = parsed.url;
            step.headersText = stringifyImportHeaders(parsed.headers);
            step.bodyText = parsed.bodyText;

            app.toasts.success("Imported cURL request.");
            app.modals.close(modal);
        } catch (err) {
            data.error = err?.message || "Failed to import cURL command.";
        }
    }

    modal = t.div(
        {
            pbEvent: "automationCurlImportModal",
            className: "modal popup md automation-curl-import-modal",
            onafterclose: (el) => el?.remove(),
        },
        t.header(
            { className: "modal-header" },
            t.h5(null, "Import cURL"),
        ),
        t.form(
            {
                id: uniqueId,
                className: "modal-content",
                onsubmit: (e) => {
                    e.preventDefault();
                    importCommand();
                },
            },
            t.div(
                { className: "field" },
                t.label({ htmlFor: `${uniqueId}_command` }, "cURL command"),
                t.textarea({
                    id: `${uniqueId}_command`,
                    rows: 10,
                    spellcheck: false,
                    autofocus: true,
                    className: "txt-code",
                    placeholder:
                        "curl -X POST https://example.com/hooks \\\n  -H 'Content-Type: application/json' \\\n  -d '{\"ok\":true}'",
                    value: () => data.command,
                    oninput: (e) => {
                        data.command = e.target.value;
                        data.error = "";
                    },
                }),
            ),
            () =>
                data.error
                    ? t.div(
                        { className: "alert danger m-t-sm" },
                        data.error,
                    )
                    : null,
        ),
        t.footer(
            { className: "modal-footer" },
            t.button(
                {
                    type: "button",
                    className: "btn transparent m-r-auto",
                    onclick: () => app.modals.close(modal),
                },
                t.span({ className: "txt" }, "Cancel"),
            ),
            t.button(
                {
                    "html-form": uniqueId,
                    type: "submit",
                    className: "btn",
                    disabled: () => !data.command.trim(),
                },
                t.i({ className: "ti ti-file-import", ariaHidden: true }),
                t.span({ className: "txt" }, "Import"),
            ),
        ),
    );

    return modal;
}

export function parseCurlCommand(command) {
    const tokens = tokenizeCurlCommand(command);
    const curlIndex = tokens.findIndex((token) => isCurlToken(token));

    if (curlIndex < 0) {
        throw new Error("Paste a command that starts with curl.");
    }

    const state = {
        explicitMethod: "",
        url: "",
        headers: {},
        bodyParts: [],
        queryParts: [],
        useGet: false,
        jsonMode: false,
        formMode: false,
    };
    const args = tokens.slice(curlIndex + 1);

    for (let i = 0; i < args.length; i++) {
        const token = args[i];

        if (!token) {
            continue;
        }

        if (token == "--") {
            if (!state.url && args[i + 1]) {
                state.url = args[i + 1];
            }
            break;
        }

        if (token.startsWith("--")) {
            i = readLongCurlOption(args, i, state);
            continue;
        }

        if (token.startsWith("-") && token != "-") {
            i = readShortCurlOption(args, i, state);
            continue;
        }

        if (!state.url) {
            state.url = token;
        }
    }

    if (!state.url) {
        throw new Error("The cURL command is missing a URL.");
    }

    if (state.queryParts.length) {
        state.url = appendQueryString(state.url, state.queryParts.join("&"));
    }

    if (state.useGet && state.bodyParts.length) {
        state.url = appendQueryString(state.url, state.bodyParts.join("&"));
        state.bodyParts = [];
    }

    if (state.formMode) {
        if (state.bodyParts.some((part) => /(^|=)@/.test(part))) {
            throw new Error("File upload form fields cannot be imported into this HTTP step.");
        }
        if (state.bodyParts.length) {
            setHeaderIfMissing(state.headers, "Content-Type", "application/x-www-form-urlencoded");
        }
    }

    if (state.jsonMode) {
        setHeaderIfMissing(state.headers, "Content-Type", "application/json");
        setHeaderIfMissing(state.headers, "Accept", "application/json");
    } else if (state.bodyParts.length && !state.formMode) {
        setHeaderIfMissing(state.headers, "Content-Type", "application/x-www-form-urlencoded");
    }

    return {
        method: normalizeCurlMethod(
            state.explicitMethod || (state.bodyParts.length ? "POST" : "GET"),
        ),
        url: state.url,
        headers: state.headers,
        bodyText: state.bodyParts.join("&"),
    };
}

function tokenizeCurlCommand(command) {
    const input = String(command || "").trim();
    const tokens = [];
    let token = "";
    let quote = "";
    let hasToken = false;

    for (let i = 0; i < input.length; i++) {
        const ch = input[i];

        if (quote) {
            if (quote == "'") {
                if (ch == "'") {
                    quote = "";
                } else {
                    token += ch;
                }
                continue;
            }

            if (quote == "\"") {
                if (ch == "\"") {
                    quote = "";
                } else if (ch == "\\" && i + 1 < input.length) {
                    token += input[++i];
                } else {
                    token += ch;
                }
                continue;
            }

            if (quote == "ansi") {
                if (ch == "'") {
                    quote = "";
                } else if (ch == "\\" && i + 1 < input.length) {
                    token += decodeAnsiEscape(input[++i]);
                } else {
                    token += ch;
                }
                continue;
            }
        }

        if (/\s/.test(ch)) {
            if (hasToken) {
                tokens.push(token);
                token = "";
                hasToken = false;
            }
            continue;
        }

        if (ch == "'") {
            quote = "'";
            hasToken = true;
            continue;
        }

        if (ch == "\"") {
            quote = "\"";
            hasToken = true;
            continue;
        }

        if (ch == "$" && input[i + 1] == "'") {
            quote = "ansi";
            hasToken = true;
            i++;
            continue;
        }

        if (ch == "\\" && i + 1 < input.length) {
            const next = input[i + 1];
            if (next == "\n") {
                i++;
                continue;
            }
            if (next == "\r" && input[i + 2] == "\n") {
                i += 2;
                continue;
            }

            token += next;
            hasToken = true;
            i++;
            continue;
        }

        token += ch;
        hasToken = true;
    }

    if (quote) {
        throw new Error("The cURL command has an unclosed quote.");
    }

    if (hasToken) {
        tokens.push(token);
    }

    return tokens;
}

function decodeAnsiEscape(ch) {
    switch (ch) {
        case "n":
            return "\n";
        case "r":
            return "\r";
        case "t":
            return "\t";
        default:
            return ch;
    }
}

function isCurlToken(token) {
    const normalized = String(token || "").toLowerCase();

    return normalized == "curl" || normalized == "curl.exe" || normalized.endsWith("/curl");
}

function readLongCurlOption(args, index, state) {
    const token = args[index];
    const eqIndex = token.indexOf("=");
    const name = eqIndex >= 0 ? token.slice(0, eqIndex) : token;
    let value = eqIndex >= 0 ? token.slice(eqIndex + 1) : undefined;
    const readValue = () => {
        if (value !== undefined) {
            return value;
        }

        index++;
        return args[index] || "";
    };

    switch (name) {
        case "--request":
        case "--request-method":
            state.explicitMethod = readValue();
            break;
        case "--url":
            state.url = readValue();
            break;
        case "--header":
            addHeader(state.headers, readValue());
            break;
        case "--user":
            addBasicAuthHeader(state.headers, readValue());
            break;
        case "--data":
        case "--data-raw":
        case "--data-ascii":
        case "--data-binary":
            state.bodyParts.push(readValue());
            break;
        case "--data-urlencode":
            state.bodyParts.push(readValue());
            break;
        case "--url-query":
            state.queryParts.push(readValue());
            break;
        case "--json":
            state.jsonMode = true;
            state.bodyParts.push(readValue());
            break;
        case "--form":
        case "--form-string":
            state.formMode = true;
            state.bodyParts.push(readValue());
            break;
        case "--get":
            state.useGet = true;
            break;
        case "--head":
            state.explicitMethod = "HEAD";
            break;
        case "--user-agent":
            setHeaderIfMissing(state.headers, "User-Agent", readValue());
            break;
        case "--referer":
            setHeaderIfMissing(state.headers, "Referer", readValue());
            break;
        case "--cookie":
            setHeaderIfMissing(state.headers, "Cookie", readValue());
            break;
        default:
            if (longCurlOptionTakesValue(name) && value === undefined) {
                index++;
            }
            break;
    }

    return index;
}

function readShortCurlOption(args, index, state) {
    const token = args[index];
    const flag = token.slice(0, 2);
    let inlineValue = token.length > 2 ? token.slice(2) : undefined;
    const readValue = () => {
        if (inlineValue !== undefined) {
            return inlineValue;
        }

        index++;
        return args[index] || "";
    };

    switch (flag) {
        case "-X":
            state.explicitMethod = readValue();
            break;
        case "-H":
            addHeader(state.headers, readValue());
            break;
        case "-u":
            addBasicAuthHeader(state.headers, readValue());
            break;
        case "-d":
            state.bodyParts.push(readValue());
            break;
        case "-F":
            state.formMode = true;
            state.bodyParts.push(readValue());
            break;
        case "-G":
            state.useGet = true;
            break;
        case "-I":
            state.explicitMethod = "HEAD";
            break;
        case "-A":
            setHeaderIfMissing(state.headers, "User-Agent", readValue());
            break;
        case "-e":
            setHeaderIfMissing(state.headers, "Referer", readValue());
            break;
        case "-b":
            setHeaderIfMissing(state.headers, "Cookie", readValue());
            break;
        default:
            if (shortCurlOptionTakesValue(flag) && inlineValue === undefined) {
                index++;
            }
            break;
    }

    return index;
}

function addHeader(headers, rawHeader) {
    const value = String(rawHeader || "");
    const colonIndex = value.indexOf(":");

    if (colonIndex < 0) {
        return;
    }

    const name = value.slice(0, colonIndex).trim();
    if (!name) {
        return;
    }

    headers[name] = value.slice(colonIndex + 1).trim();
}

function addBasicAuthHeader(headers, credentials) {
    if (!credentials || findHeaderName(headers, "Authorization")) {
        return;
    }

    const encoder = globalThis.btoa || ((value) => Buffer.from(value, "utf8").toString("base64"));
    headers.Authorization = "Basic " + encoder(credentials);
}

function setHeaderIfMissing(headers, name, value) {
    if (!findHeaderName(headers, name)) {
        headers[name] = value;
    }
}

function findHeaderName(headers, name) {
    const lowerName = name.toLowerCase();

    return Object.keys(headers).find((key) => key.toLowerCase() == lowerName);
}

function appendQueryString(url, query) {
    if (!query) {
        return url;
    }

    const separator = url.includes("?") ? (/[?&]$/.test(url) ? "" : "&") : "?";

    return url + separator + query;
}

function normalizeCurlMethod(method) {
    return String(method || "GET").trim().toUpperCase() || "GET";
}

function stringifyImportHeaders(headers) {
    return Object.keys(headers || {}).length ? JSON.stringify(headers, null, 4) : "{}";
}

function longCurlOptionTakesValue(name) {
    return [
        "--abstract-unix-socket",
        "--cacert",
        "--capath",
        "--cert",
        "--cert-type",
        "--ciphers",
        "--connect-timeout",
        "--connect-to",
        "--dns-interface",
        "--dns-ipv4-addr",
        "--dns-ipv6-addr",
        "--dns-servers",
        "--engine",
        "--expect100-timeout",
        "--interface",
        "--key",
        "--key-type",
        "--limit-rate",
        "--local-port",
        "--max-filesize",
        "--max-redirs",
        "--max-time",
        "--oauth2-bearer",
        "--output",
        "--pass",
        "--proxy",
        "--proxy-header",
        "--proxy-user",
        "--rate",
        "--request-target",
        "--resolve",
        "--retry",
        "--retry-delay",
        "--retry-max-time",
        "--socks5",
        "--socks5-hostname",
        "--unix-socket",
        "--upload-file",
    ].includes(name);
}

function shortCurlOptionTakesValue(flag) {
    return ["-o", "-O", "-x", "-U", "-y", "-Y", "-m", "-r", "-E", "-K"].includes(flag);
}
