import EditorWorker from "monaco-editor/esm/vs/editor/editor.worker?worker";
import CssWorker from "monaco-editor/esm/vs/language/css/css.worker?worker";
import HtmlWorker from "monaco-editor/esm/vs/language/html/html.worker?worker";
import JsonWorker from "monaco-editor/esm/vs/language/json/json.worker?worker";
import TsWorker from "monaco-editor/esm/vs/language/typescript/ts.worker?worker";

window.app = window.app || {};
window.app.components = window.app.components || {};

let monacoLoadPromise;
let monacoTypeDefinitionsPromise;
let monacoTypeDefinitionsRegistered = false;

window.MonacoEnvironment = {
    ...(window.MonacoEnvironment || {}),
    getWorker: function(_, label) {
        switch (label) {
            case "json":
                return new JsonWorker();
            case "css":
            case "scss":
            case "less":
                return new CssWorker();
            case "html":
            case "handlebars":
            case "razor":
                return new HtmlWorker();
            case "typescript":
            case "javascript":
                return new TsWorker();
            default:
                return new EditorWorker();
        }
    },
};

/**
 * Lazy Monaco-powered code editor.
 *
 * @param  {Object} [propsArg]
 * @return {Element}
 */
window.app.components.monacoEditor = function(propsArg = {}) {
    const props = store({
        rid: undefined,
        id: undefined,
        hidden: undefined,
        inert: undefined,
        name: undefined,
        className: "",
        value: "",
        language: "javascript",
        placeholder: "",
        disabled: false,
        required: false,
        autocomplete: undefined,
        oninput: function(val) {},
        onfocus: function(val) {},
        onblur: function(val) {},
    });

    const watchers = app.utils.extendStore(props, propsArg, "autocomplete");

    let editor;
    let model;
    let valueWatcher;
    let disabledWatcher;
    let completionDisposable;
    let resizeObserver;
    let disposed = false;
    let syncingValue = false;

    const editorHost = t.div({
        className: "monaco-editor-host",
        textContent: "Loading editor...",
    });

    async function init() {
        try {
            const monaco = await loadMonaco();
            if (disposed || !editorHost.isConnected) {
                return;
            }

            editorHost.textContent = "";

            const language = normalizeLanguage(props.language);
            const uri = monaco.Uri.parse(`inmemory://pocketadmin/${props.id || app.utils.randomString()}.${language}`);
            model = monaco.editor.createModel(toString(props.value), language, uri);
            const theme = defineMonacoEditorTheme(monaco, editorHost);

            editor = monaco.editor.create(editorHost, {
                model: model,
                theme: theme,
                readOnly: !!props.disabled,
                automaticLayout: true,
                minimap: { enabled: false },
                scrollBeyondLastLine: false,
                wordWrap: "off",
                tabSize: 4,
                insertSpaces: true,
                lineNumbersMinChars: 3,
                renderLineHighlight: "line",
                fixedOverflowWidgets: false,
                fontFamily: getComputedStyle(document.documentElement).getPropertyValue("--monospaceFontFamily"),
                fontSize: 13,
            });

            editor.onDidChangeModelContent(() => {
                if (syncingValue) {
                    return;
                }

                props.value = model.getValue();
                props.oninput?.(props.value);
                editorHost.dispatchEvent(new CustomEvent("change", { detail: props.value }));
            });

            editor.onDidFocusEditorText(() => props.onfocus?.(props.value));
            editor.onDidBlurEditorText(() => props.onblur?.(props.value));

            completionDisposable = registerAutocomplete(monaco, model, props);

            valueWatcher?.unwatch();
            valueWatcher = watch(
                () => props.value,
                (value) => {
                    if (!model || model.getValue() == toString(value)) {
                        return;
                    }

                    syncingValue = true;
                    model.setValue(toString(value));
                    syncingValue = false;
                },
            );

            disabledWatcher?.unwatch();
            disabledWatcher = watch(
                () => props.disabled,
                (disabled) => editor?.updateOptions({ readOnly: !!disabled }),
            );

            resizeObserver = new ResizeObserver(() => editor?.layout());
            resizeObserver.observe(editorHost);
        } catch (err) {
            console.warn("failed to load Monaco editor:", err);
            mountFallback();
        }
    }

    function mountFallback() {
        if (disposed || !editorHost.isConnected || !app.components.codeEditor) {
            return;
        }

        editorHost.textContent = "";
        editorHost.appendChild(app.components.codeEditor(propsArg));
    }

    function dispose() {
        disposed = true;
        resizeObserver?.disconnect();
        completionDisposable?.dispose();
        valueWatcher?.unwatch();
        disabledWatcher?.unwatch();
        watchers.forEach((w) => w?.unwatch());
        editor?.dispose();
        model?.dispose();
    }

    return t.div(
        {
            rid: props.rid,
            id: () => props.id,
            inert: () => props.inert,
            hidden: () => props.hidden,
            "html-name": () => props.name,
            "html-required": () => props.required || undefined,
            className: () => `input monaco-editor-input ${props.className} ${props.disabled ? "disabled" : ""}`,
            onmount: init,
            onunmount: dispose,
        },
        editorHost,
    );
};

function loadMonaco() {
    if (window.monaco?.editor) {
        return configureMonacoTypeDefinitions(window.monaco).then(() => window.monaco);
    }

    if (!monacoLoadPromise) {
        monacoLoadPromise = import("monaco-editor").then(async (monaco) => {
            window.monaco = monaco;
            await configureMonacoTypeDefinitions(monaco);
            return monaco;
        });
    }

    return monacoLoadPromise;
}

function configureMonacoTypeDefinitions(monaco) {
    if (monacoTypeDefinitionsRegistered || !monaco?.languages?.typescript) {
        return Promise.resolve();
    }

    if (!monacoTypeDefinitionsPromise) {
        monacoTypeDefinitionsPromise = fetch(resolvePublicAssetURL("types.d.ts"), {
            cache: "force-cache",
        })
            .then((res) => {
                if (!res.ok) {
                    throw new Error(`Failed to load types.d.ts (${res.status})`);
                }
                return res.text();
            })
            .then((source) => {
                if (!source || monacoTypeDefinitionsRegistered) {
                    return;
                }

                const libPath = "file:///pocketbase/types.d.ts";
                monaco.languages.typescript.javascriptDefaults.addExtraLib(source, libPath);
                monaco.languages.typescript.typescriptDefaults.addExtraLib(source, libPath);
                monacoTypeDefinitionsRegistered = true;
            })
            .catch((err) => {
                console.warn("failed to load Monaco TypeScript definitions:", err);
            });
    }

    return monacoTypeDefinitionsPromise;
}

function resolvePublicAssetURL(path) {
    return new URL(path, document.baseURI).toString();
}

function defineMonacoEditorTheme(monaco, el) {
    const themeName = `pocketadmin-${app.utils.randomString()}`;
    const inputColor = cssVarColor(el, "--inputColor", "#ffffff");
    const inputFocusColor = cssVarColor(el, "--inputFocusColor", inputColor);
    const inputBorderColor = cssVarColor(el, "--inputBorderColor", inputFocusColor);
    const textColor = cssVarColor(el, "--surfaceTxtColor", "#111111");
    const hintColor = cssVarColor(el, "--surfaceTxtHintColor", textColor);
    const disabledColor = cssVarColor(el, "--surfaceTxtDisabledColor", hintColor);
    const selectionColor = cssVarColor(el, "--selectionColor", inputFocusColor);

    monaco.editor.defineTheme(themeName, {
        base: "vs",
        inherit: true,
        rules: [
            { token: "", foreground: "111111" },
            { token: "comment", foreground: "008000" },
            { token: "comment.doc", foreground: "008000" },
            { token: "keyword", foreground: "0000ff" },
            { token: "keyword.json", foreground: "0000ff" },
            { token: "identifier", foreground: "006dcc" },
            { token: "identifier.function", foreground: "795e26" },
            { token: "type.identifier", foreground: "001080" },
            { token: "number", foreground: "098658" },
            { token: "string", foreground: "a31515" },
            { token: "string.escape", foreground: "0451a5" },
            { token: "regexp", foreground: "811f3f" },
            { token: "delimiter", foreground: "111111" },
            { token: "delimiter.bracket", foreground: "0000ff" },
            { token: "delimiter.parenthesis", foreground: "111111" },
            { token: "operator", foreground: "0000ff" },
            { token: "predefined", foreground: "001080" },
            { token: "variable", foreground: "001080" },
            { token: "variable.predefined", foreground: "001080" },
            { token: "tag", foreground: "800000" },
            { token: "attribute.name", foreground: "ff0000" },
            { token: "attribute.value", foreground: "0000ff" },
        ],
        colors: {
            "editor.background": inputColor,
            "editor.foreground": textColor,
            "editorCursor.foreground": hintColor,
            "editorLineNumber.foreground": disabledColor,
            "editorLineNumber.activeForeground": textColor,
            "editorGutter.background": inputColor,
            "editor.lineHighlightBackground": withAlpha(inputFocusColor, "88"),
            "editor.selectionBackground": withAlpha(selectionColor, "66"),
            "editor.inactiveSelectionBackground": withAlpha(selectionColor, "33"),
            "editorIndentGuide.background1": inputBorderColor,
            "editorIndentGuide.activeBackground1": hintColor,
            "editorWidget.background": inputColor,
            "editorWidget.border": inputBorderColor,
            "editorHoverWidget.background": inputColor,
            "editorHoverWidget.border": inputBorderColor,
            "editorSuggestWidget.background": inputColor,
            "editorSuggestWidget.border": inputBorderColor,
            "editorSuggestWidget.foreground": textColor,
            "editorSuggestWidget.selectedBackground": inputFocusColor,
            "scrollbarSlider.background": withAlpha(hintColor, "33"),
            "scrollbarSlider.hoverBackground": withAlpha(hintColor, "55"),
            "scrollbarSlider.activeBackground": withAlpha(hintColor, "77"),
        },
    });

    return themeName;
}

function cssVarColor(el, name, fallback) {
    const raw = getComputedStyle(el).getPropertyValue(name).trim();
    return resolveCssColor(el, raw || fallback, fallback);
}

function resolveCssColor(parent, color, fallback) {
    const probe = document.createElement("span");
    probe.style.position = "absolute";
    probe.style.visibility = "hidden";
    probe.style.pointerEvents = "none";
    probe.style.color = color;
    parent.appendChild(probe);

    const resolved = getComputedStyle(probe).color;
    probe.remove();

    return rgbToHex(resolved) || fallback;
}

function rgbToHex(value) {
    const match = String(value || "").match(/^rgba?\((\d+),\s*(\d+),\s*(\d+)(?:,\s*([0-9.]+))?\)$/i);
    if (!match) {
        return "";
    }

    const r = Number(match[1]);
    const g = Number(match[2]);
    const b = Number(match[3]);
    const alpha = match[4] === undefined ? 1 : Number(match[4]);
    const result = [r, g, b].map((part) => clampColor(part).toString(16).padStart(2, "0")).join("");
    if (alpha >= 1) {
        return `#${result}`;
    }

    return `#${result}${clampColor(Math.round(alpha * 255)).toString(16).padStart(2, "0")}`;
}

function trimHex(value) {
    return String(value || "").replace(/^#/, "").slice(0, 6);
}

function withAlpha(value, alpha) {
    const hex = String(value || "").replace(/^#/, "").slice(0, 6);
    return `#${hex}${alpha}`;
}

function clampColor(value) {
    return Math.max(0, Math.min(255, Number(value) || 0));
}

function registerAutocomplete(monaco, model, props) {
    if (!props.autocomplete) {
        return null;
    }

    return monaco.languages.registerCompletionItemProvider(model.getLanguageId(), {
        triggerCharacters: [".", "_"],
        provideCompletionItems: (completionModel, position) => {
            if (completionModel !== model) {
                return { suggestions: [] };
            }

            const word = completionModel.getWordUntilPosition(position);
            const query = word?.word || "";
            const items = typeof props.autocomplete == "function"
                ? props.autocomplete(query) || []
                : filterAutocompleteItems(props.autocomplete, query);

            return {
                suggestions: items.map((item) => {
                    const value = item.value || item;
                    return {
                        label: item.label || value,
                        kind: monaco.languages.CompletionItemKind.Variable,
                        insertText: value,
                        range: {
                            startLineNumber: position.lineNumber,
                            endLineNumber: position.lineNumber,
                            startColumn: word.startColumn,
                            endColumn: word.endColumn,
                        },
                    };
                }),
            };
        },
    });
}

function filterAutocompleteItems(items, query) {
    const queryLower = String(query || "").toLowerCase();
    return (items || []).filter((item) => {
        const value = String(item?.value || item || "").toLowerCase();
        return value && (!queryLower || value.includes(queryLower));
    });
}

function normalizeLanguage(language) {
    switch (language) {
        case "js":
            return "javascript";
        case "ts":
            return "typescript";
        default:
            return language || "javascript";
    }
}

function toString(value) {
    return value === undefined || value === null ? "" : String(value);
}
