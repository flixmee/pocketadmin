window.app = window.app || {};
window.app.components = window.app.components || {};

const monacoEditorMainURL = "https://cdnjs.cloudflare.com/ajax/libs/monaco-editor/0.53.0/min/vs/editor/editor.main.js";
const monacoBaseURL = monacoEditorMainURL.replace(/\/editor\/editor\.main\.js$/, "");
const monacoLoaderURL = monacoBaseURL + "/loader.js";

let monacoLoadPromise;

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

            editor = monaco.editor.create(editorHost, {
                model: model,
                readOnly: !!props.disabled,
                automaticLayout: true,
                minimap: { enabled: false },
                scrollBeyondLastLine: false,
                wordWrap: "off",
                tabSize: 4,
                insertSpaces: true,
                lineNumbersMinChars: 3,
                renderLineHighlight: "line",
                fixedOverflowWidgets: true,
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
        return Promise.resolve(window.monaco);
    }

    if (monacoLoadPromise) {
        return monacoLoadPromise;
    }

    monacoLoadPromise = loadScript("lazy-monaco-loader-js", monacoLoaderURL).then(() => {
        window.MonacoEnvironment = window.MonacoEnvironment || {
            getWorkerUrl: function() {
                const workerSource =
                    `self.MonacoEnvironment = { baseUrl: "${monacoBaseURL}/" }; importScripts("${monacoBaseURL}/base/worker/workerMain.js");`;
                return `data:text/javascript;charset=utf-8,${encodeURIComponent(workerSource)}`;
            },
        };

        return new Promise((resolve, reject) => {
            if (!window.require?.config) {
                reject(new Error("Monaco AMD loader is unavailable."));
                return;
            }

            window.require.config({
                paths: {
                    vs: monacoBaseURL,
                },
            });

            window.require(
                ["vs/editor/editor.main"],
                () => resolve(window.monaco),
                reject,
            );
        });
    });

    return monacoLoadPromise;
}

function loadScript(id, src) {
    const existing = document.getElementById(id);
    if (existing) {
        return new Promise((resolve, reject) => {
            existing.addEventListener("load", resolve, { once: true });
            existing.addEventListener("error", reject, { once: true });
        });
    }

    return new Promise((resolve, reject) => {
        const script = t.script({
            id: id,
            src: src,
            onload: resolve,
            onerror: reject,
        });
        const shablonScript = document.head.querySelector("#shablon-script");

        if (shablonScript) {
            shablonScript.after(script);
        } else {
            document.head.appendChild(script);
        }
    });
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
